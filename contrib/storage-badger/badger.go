// Package badger provides BadgerDB-backed implementations of agent-go storage interfaces.
//
// BadgerDB is an embeddable, persistent, and fast key-value database written in pure Go.
// It provides LSM tree-based storage with ACID transactions and is suitable for
// high-performance, single-node deployments.
//
// # Usage
//
//	db, err := badger.Open(badger.DefaultOptions("/path/to/db"))
//	if err != nil {
//		return err
//	}
//	defer db.Close()
//
//	cache := storagebadger.NewCache(db)
//	eventStore := storagebadger.NewEventStore(db)
package badger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"go.klarlabs.de/agent/domain/cache"
	"go.klarlabs.de/agent/domain/event"
)

// DB is a type alias for BadgerDB.
type DB = badger.DB

// Cache is a BadgerDB-backed implementation of cache.Cache.
// It provides high-performance key-value caching with optional TTL support.
type Cache struct {
	db *DB
}

// NewCache creates a new BadgerDB cache with the given database.
// The caller is responsible for managing the database lifecycle.
func NewCache(db *DB) *Cache {
	return &Cache{db: db}
}

// prefixKey adds the cache prefix to keys.
func (c *Cache) prefixKey(key string) string {
	return "cache:" + key
}

// Get retrieves a cached value by key.
// Returns the value, whether it was found, and any error.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	var value []byte
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(c.prefixKey(key)))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			value = append([]byte{}, val...)
			return nil
		})
	})

	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, false, nil
		}
		return nil, false, c.wrapError(err)
	}

	return value, true, nil
}

// Set stores a value with the given key and options.
// TTL is supported natively by BadgerDB entries.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if key == "" {
		return cache.ErrInvalidKey
	}

	return c.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(c.prefixKey(key)), value)
		if opts.TTL > 0 {
			e = e.WithTTL(opts.TTL)
		}
		return txn.SetEntry(e)
	})
}

// Delete removes a cached entry by key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return c.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(c.prefixKey(key)))
	})
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	err := c.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(c.prefixKey(key)))
		return err
	})

	if err == nil {
		return true, nil
	}
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	return false, c.wrapError(err)
}

// Clear removes all entries from the cache.
// Note: This operation can be expensive for large datasets.
func (c *Cache) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return c.db.DropPrefix([]byte("cache:"))
}

// Close closes the underlying database.
func (c *Cache) Close() error {
	return c.db.Close()
}

// wrapError wraps BadgerDB errors with domain errors.
func (c *Cache) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(cache.ErrOperationTimeout, err)
	}

	return errors.Join(cache.ErrConnectionFailed, err)
}

// EventStore is a BadgerDB-backed implementation of event.Store.
// It provides event sourcing capabilities with ordered key storage.
type EventStore struct {
	db          *DB
	subscribers map[string][]chan event.Event
	mu          sync.RWMutex
}

// NewEventStore creates a new BadgerDB event store with the given database.
// The caller is responsible for managing the database lifecycle.
func NewEventStore(db *DB) *EventStore {
	return &EventStore{
		db:          db,
		subscribers: make(map[string][]chan event.Event),
	}
}

// eventKey generates the key for an event: events:{runID}:{sequence:016d}
func (s *EventStore) eventKey(runID string, sequence uint64) string {
	return fmt.Sprintf("events:%s:%016d", runID, sequence)
}

// eventPrefix generates the prefix for all events in a run: events:{runID}:
func (s *EventStore) eventPrefix(runID string) string {
	return fmt.Sprintf("events:%s:", runID)
}

// Append persists one or more events atomically.
// Events are assigned sequence numbers in order of appearance.
// Keys are structured as: events/{runID}/{sequence} for ordered iteration.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if len(events) == 0 {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	err := s.db.Update(func(txn *badger.Txn) error {
		// Get current max sequence for each run
		sequences := make(map[string]uint64)
		for _, e := range events {
			if _, ok := sequences[e.RunID]; !ok {
				// Find max sequence for this run by iterating backwards
				prefix := []byte(s.eventPrefix(e.RunID))
				opts := badger.DefaultIteratorOptions
				opts.Reverse = true
				opts.PrefetchValues = false
				it := txn.NewIterator(opts)
				defer it.Close()

				it.Seek(append(prefix, 0xFF)) // Seek to end of prefix range
				if it.ValidForPrefix(prefix) {
					// Parse sequence from last key
					key := string(it.Item().Key())
					var maxSeq uint64
					_, err := fmt.Sscanf(key, "events:"+e.RunID+":%016d", &maxSeq)
					if err == nil {
						sequences[e.RunID] = maxSeq
					}
				}
			}
		}

		// Insert events with sequence numbers
		for i := range events {
			// Assign ID if not set
			if events[i].ID == "" {
				events[i].ID = uuid.New().String()
			}

			// Validate event
			if events[i].Type == "" {
				return event.ErrInvalidEvent
			}

			// Set default version
			if events[i].Version == 0 {
				events[i].Version = 1
			}

			// Assign sequence number
			sequences[events[i].RunID]++
			events[i].Sequence = sequences[events[i].RunID]

			// Marshal event to JSON
			data, err := json.Marshal(events[i])
			if err != nil {
				return errors.Join(event.ErrInvalidEvent, err)
			}

			// Store event
			key := []byte(s.eventKey(events[i].RunID, events[i].Sequence))
			if err := txn.Set(key, data); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		// Don't wrap domain errors, only infrastructure errors
		if errors.Is(err, event.ErrInvalidEvent) {
			return err
		}
		return s.wrapError(err)
	}

	// Notify subscribers after successful commit
	s.notifySubscribers(events)

	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
// Uses BadgerDB's key ordering for efficient sequential reads.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var result []event.Event

	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(s.eventPrefix(runID))
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var e event.Event
				if err := json.Unmarshal(val, &e); err != nil {
					return errors.Join(event.ErrInvalidEvent, err)
				}
				result = append(result, e)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, s.wrapError(err)
	}

	return result, nil
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
// This enables incremental replay from a known checkpoint.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var result []event.Event

	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(s.eventPrefix(runID))
		seekKey := []byte(s.eventKey(runID, fromSeq))
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(seekKey); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var e event.Event
				if err := json.Unmarshal(val, &e); err != nil {
					return errors.Join(event.ErrInvalidEvent, err)
				}
				result = append(result, e)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, s.wrapError(err)
	}

	return result, nil
}

// Subscribe returns a channel that receives new events for a run.
// The channel is closed when the context is cancelled or the run completes.
// Note: BadgerDB does not have native pub/sub, so this uses polling or
// Subscribe callbacks if available.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan event.Event, 100)
	s.subscribers[runID] = append(s.subscribers[runID], ch)

	// Start cleanup goroutine
	go func() {
		<-ctx.Done()
		s.unsubscribe(runID, ch)
	}()

	return ch, nil
}

// Close closes the underlying database.
func (s *EventStore) Close() error {
	return s.db.Close()
}

// notifySubscribers sends events to all subscribers.
func (s *EventStore) notifySubscribers(events []event.Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range events {
		if subs, ok := s.subscribers[e.RunID]; ok {
			for _, ch := range subs {
				select {
				case ch <- e:
				default:
					// Channel full, skip
				}
			}
		}
	}
}

// unsubscribe removes a subscriber channel.
func (s *EventStore) unsubscribe(runID string, ch chan event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs := s.subscribers[runID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[runID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}

	if len(s.subscribers[runID]) == 0 {
		delete(s.subscribers, runID)
	}
}

// wrapError wraps BadgerDB errors with domain errors.
func (s *EventStore) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(event.ErrOperationTimeout, err)
	}

	return errors.Join(event.ErrConnectionFailed, err)
}

// Ensure interfaces are implemented.
var (
	_ cache.Cache = (*Cache)(nil)
	_ event.Store = (*EventStore)(nil)
)
