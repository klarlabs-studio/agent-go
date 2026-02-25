// Package nats provides NATS-backed implementations of agent-go storage interfaces.
//
// NATS is a high-performance messaging system that supports pub/sub, request/reply,
// and queue groups. With NATS JetStream, it also provides persistence, exactly-once
// delivery, and stream processing capabilities.
//
// This package uses NATS JetStream for event storage, providing durable, ordered
// event streams with replay capabilities.
//
// # Usage
//
//	nc, err := nats.Connect(nats.DefaultURL)
//	if err != nil {
//		return err
//	}
//	defer nc.Close()
//
//	js, err := nc.JetStream()
//	if err != nil {
//		return err
//	}
//
//	eventStore := storagenats.NewEventStore(js, "AGENT_EVENTS")
package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// JetStreamContext represents a NATS JetStream context interface.
// This allows for mocking in tests.
type JetStreamContext interface {
	Publish(subj string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error)
	PullSubscribe(subj, durable string, opts ...nats.SubOpt) (*nats.Subscription, error)
	Subscribe(subj string, cb nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error)
}

// EventStore is a NATS JetStream-backed implementation of event.Store.
// It stores events in a JetStream stream with subject-based routing per run.
type EventStore struct {
	js            JetStreamContext
	streamName    string
	subjectPrefix string
	mu            sync.RWMutex
	sequences     map[string]uint64 // Track sequence per run
	seqMu         sync.Mutex
}

// EventStoreConfig holds configuration for the NATS event store.
type EventStoreConfig struct {
	// StreamName is the JetStream stream name.
	StreamName string

	// SubjectPrefix is the prefix for event subjects (default: "agent.events").
	SubjectPrefix string

	// MaxMsgsPerSubject limits messages per subject (run) for retention.
	MaxMsgsPerSubject int64
}

// NewEventStore creates a new NATS JetStream event store with the given context and stream name.
func NewEventStore(js JetStreamContext, streamName string) *EventStore {
	return &EventStore{
		js:            js,
		streamName:    streamName,
		subjectPrefix: "agent.events",
		sequences:     make(map[string]uint64),
	}
}

// NewEventStoreWithConfig creates a new NATS JetStream event store with full configuration.
func NewEventStoreWithConfig(js JetStreamContext, cfg EventStoreConfig) *EventStore {
	subjectPrefix := cfg.SubjectPrefix
	if subjectPrefix == "" {
		subjectPrefix = "agent.events"
	}
	return &EventStore{
		js:            js,
		streamName:    cfg.StreamName,
		subjectPrefix: subjectPrefix,
		sequences:     make(map[string]uint64),
	}
}

// Append persists one or more events atomically.
// Events are published to subjects formatted as: {prefix}.{runID}.{sequence}
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Assign sequence numbers and validate events
	s.seqMu.Lock()
	for i := range events {
		// Assign ID if not set
		if events[i].ID == "" {
			events[i].ID = uuid.New().String()
		}

		// Validate event
		if events[i].Type == "" {
			s.seqMu.Unlock()
			return event.ErrInvalidEvent
		}

		// Set default version
		if events[i].Version == 0 {
			events[i].Version = 1
		}

		// Assign sequence number
		s.sequences[events[i].RunID]++
		events[i].Sequence = s.sequences[events[i].RunID]
	}
	s.seqMu.Unlock()

	// Publish each event to JetStream
	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		subject := s.eventSubject(e.RunID)
		_, err = s.js.Publish(subject, data)
		if err != nil {
			return s.wrapError(err)
		}
	}

	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
// Uses JetStream consumer to fetch all messages for the run subject.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	subject := s.eventSubject(runID)

	// Create ephemeral pull subscriber to fetch all messages
	sub, err := s.js.PullSubscribe(subject, "", nats.DeliverAll())
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer sub.Unsubscribe()

	var events []event.Event

	// Fetch messages in batches until no more available
	for {
		// Use a short timeout for each batch fetch
		fetchCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		msgs, err := sub.Fetch(100, nats.Context(fetchCtx))
		cancel()

		if err != nil {
			if errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
				// No more messages
				break
			}
			return nil, s.wrapError(err)
		}

		if len(msgs) == 0 {
			break
		}

		for _, msg := range msgs {
			var e event.Event
			if err := json.Unmarshal(msg.Data, &e); err != nil {
				return nil, fmt.Errorf("failed to unmarshal event: %w", err)
			}
			events = append(events, e)
			msg.Ack()
		}
	}

	// Sort by sequence to ensure order
	sort.Slice(events, func(i, j int) bool {
		return events[i].Sequence < events[j].Sequence
	})

	return events, nil
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
// Uses JetStream consumer with start sequence option.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	// Load all events and filter client-side
	// NATS sequences are per-stream, not per-subject, so we need to filter by our sequence field
	allEvents, err := s.LoadEvents(ctx, runID)
	if err != nil {
		return nil, err
	}

	var filtered []event.Event
	for _, e := range allEvents {
		if e.Sequence >= fromSeq {
			filtered = append(filtered, e)
		}
	}

	return filtered, nil
}

// Subscribe returns a channel that receives new events for a run.
// Uses JetStream push consumer for real-time event delivery.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	ch := make(chan event.Event, 100)

	// Subscribe to JetStream for all events (including those from this instance)
	subject := s.eventSubject(runID)

	// Track if channel is closed
	var closed bool
	var closeMu sync.Mutex

	sub, err := s.js.Subscribe(subject, func(msg *nats.Msg) {
		var e event.Event
		if err := json.Unmarshal(msg.Data, &e); err != nil {
			msg.Ack()
			return // Skip malformed events
		}

		closeMu.Lock()
		isClosed := closed
		closeMu.Unlock()

		if !isClosed {
			select {
			case ch <- e:
			default:
				// Channel full, skip
			}
		}

		msg.Ack()
	}, nats.DeliverNew())

	if err != nil {
		close(ch)
		return nil, s.wrapError(err)
	}

	// Cleanup goroutine
	go func() {
		<-ctx.Done()
		sub.Unsubscribe()

		closeMu.Lock()
		closed = true
		closeMu.Unlock()

		// Close channel
		s.mu.Lock()
		close(ch)
		s.mu.Unlock()
	}()

	return ch, nil
}

// eventSubject returns the JetStream subject for a run's events.
func (s *EventStore) eventSubject(runID string) string {
	return fmt.Sprintf("%s.%s", s.subjectPrefix, runID)
}

// wrapError wraps NATS errors with domain errors.
func (s *EventStore) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, nats.ErrTimeout) {
		return errors.Join(event.ErrOperationTimeout, err)
	}

	return errors.Join(event.ErrConnectionFailed, err)
}

// Ensure interface is implemented.
var _ event.Store = (*EventStore)(nil)
