package mongodb

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.klarlabs.de/agent/domain/event"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// eventDocument is the MongoDB document representation of an event.
type eventDocument struct {
	ID        string    `bson:"_id"`
	RunID     string    `bson:"run_id"`
	Type      string    `bson:"type"`
	Sequence  uint64    `bson:"sequence"`
	Payload   []byte    `bson:"payload"`
	Timestamp time.Time `bson:"timestamp"`
	Version   int       `bson:"version,omitempty"`
}

// EventStore is a MongoDB-backed implementation of event.Store.
type EventStore struct {
	collection   *mongo.Collection
	queryTimeout time.Duration

	// Subscription management
	mu          sync.RWMutex
	subscribers map[string][]chan event.Event
}

// NewEventStore creates a new MongoDB event store.
func NewEventStore(client *Client, collectionName string) *EventStore {
	if collectionName == "" {
		collectionName = "events"
	}
	return &EventStore{
		collection:   client.Collection(collectionName),
		queryTimeout: client.config.QueryTimeout,
		subscribers:  make(map[string][]chan event.Event),
	}
}

// Append persists one or more events atomically.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if len(events) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	// Group events by run ID to handle sequencing
	byRun := make(map[string][]event.Event)
	for _, e := range events {
		byRun[e.RunID] = append(byRun[e.RunID], e)
	}

	// Process each run's events
	for runID, runEvents := range byRun {
		// Get the current max sequence for this run
		maxSeq, err := s.getMaxSequence(ctx, runID)
		if err != nil {
			return err
		}

		docs := make([]interface{}, len(runEvents))
		for i := range runEvents {
			e := runEvents[i]

			// Assign ID if not set
			if e.ID == "" {
				e.ID = uuid.New().String()
			}

			// Assign sequence number
			maxSeq++
			e.Sequence = maxSeq

			// Validate event
			if e.Type == "" {
				return event.ErrInvalidEvent
			}

			docs[i] = s.toDocument(&e)
			runEvents[i] = e
		}

		// Insert all events for this run
		_, err = s.collection.InsertMany(ctx, docs)
		if err != nil {
			return s.wrapError(err)
		}

		// Notify subscribers
		s.notifySubscribers(runID, runEvents)
	}

	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	return s.LoadEventsFrom(ctx, runID, 0)
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	filter := bson.M{"run_id": runID}
	if fromSeq > 0 {
		filter["sequence"] = bson.M{"$gte": fromSeq}
	}

	opts := options.Find().SetSort(bson.D{{Key: "sequence", Value: 1}})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var events []event.Event
	for cursor.Next(ctx) {
		var doc eventDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, s.wrapError(err)
		}
		events = append(events, s.fromDocument(&doc))
	}

	if err := cursor.Err(); err != nil {
		return nil, s.wrapError(err)
	}

	return events, nil
}

// Subscribe returns a channel that receives new events for a run.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
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

// Query retrieves events matching the given options.
func (s *EventStore) Query(ctx context.Context, runID string, opts event.QueryOptions) ([]event.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	filter := bson.M{"run_id": runID}

	// Filter by types
	if len(opts.Types) > 0 {
		types := make([]string, len(opts.Types))
		for i, t := range opts.Types {
			types[i] = string(t)
		}
		filter["type"] = bson.M{"$in": types}
	}

	// Filter by time range
	if opts.FromTime > 0 || opts.ToTime > 0 {
		timeFilter := bson.M{}
		if opts.FromTime > 0 {
			timeFilter["$gte"] = time.Unix(opts.FromTime, 0)
		}
		if opts.ToTime > 0 {
			timeFilter["$lte"] = time.Unix(opts.ToTime, 0)
		}
		filter["timestamp"] = timeFilter
	}

	findOpts := options.Find().SetSort(bson.D{{Key: "sequence", Value: 1}})

	if opts.Offset > 0 {
		findOpts.SetSkip(int64(opts.Offset))
	}

	if opts.Limit > 0 {
		findOpts.SetLimit(int64(opts.Limit))
	}

	cursor, err := s.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var events []event.Event
	for cursor.Next(ctx) {
		var doc eventDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, s.wrapError(err)
		}
		events = append(events, s.fromDocument(&doc))
	}

	if err := cursor.Err(); err != nil {
		return nil, s.wrapError(err)
	}

	return events, nil
}

// CountEvents returns the number of events for a run.
func (s *EventStore) CountEvents(ctx context.Context, runID string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	count, err := s.collection.CountDocuments(ctx, bson.M{"run_id": runID})
	if err != nil {
		return 0, s.wrapError(err)
	}

	return count, nil
}

// ListRuns returns all run IDs with events in the store.
func (s *EventStore) ListRuns(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	runs, err := s.collection.Distinct(ctx, "run_id", bson.M{})
	if err != nil {
		return nil, s.wrapError(err)
	}

	result := make([]string, len(runs))
	for i, r := range runs {
		result[i] = r.(string)
	}

	return result, nil
}

// DeleteRun removes all events for a specific run.
func (s *EventStore) DeleteRun(ctx context.Context, runID string) error {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	// Close subscriber channels for this run
	s.mu.Lock()
	if subs, ok := s.subscribers[runID]; ok {
		for _, ch := range subs {
			close(ch)
		}
		delete(s.subscribers, runID)
	}
	s.mu.Unlock()

	_, err := s.collection.DeleteMany(ctx, bson.M{"run_id": runID})
	if err != nil {
		return s.wrapError(err)
	}

	return nil
}

// getMaxSequence returns the current maximum sequence number for a run.
func (s *EventStore) getMaxSequence(ctx context.Context, runID string) (uint64, error) {
	opts := options.FindOne().
		SetSort(bson.D{{Key: "sequence", Value: -1}}).
		SetProjection(bson.M{"sequence": 1})

	var doc struct {
		Sequence uint64 `bson:"sequence"`
	}

	err := s.collection.FindOne(ctx, bson.M{"run_id": runID}, opts).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, nil
		}
		return 0, s.wrapError(err)
	}

	return doc.Sequence, nil
}

// notifySubscribers sends events to all subscribers for a run.
func (s *EventStore) notifySubscribers(runID string, events []event.Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if subs, ok := s.subscribers[runID]; ok {
		for _, sub := range subs {
			for _, e := range events {
				select {
				case sub <- e:
				default:
					// Channel full, skip (non-blocking)
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

	// Clean up empty subscriber lists
	if len(s.subscribers[runID]) == 0 {
		delete(s.subscribers, runID)
	}
}

// toDocument converts an Event to a MongoDB document.
func (s *EventStore) toDocument(e *event.Event) *eventDocument {
	return &eventDocument{
		ID:        e.ID,
		RunID:     e.RunID,
		Type:      string(e.Type),
		Sequence:  e.Sequence,
		Payload:   e.Payload,
		Timestamp: e.Timestamp,
		Version:   e.Version,
	}
}

// fromDocument converts a MongoDB document to an Event.
func (s *EventStore) fromDocument(doc *eventDocument) event.Event {
	return event.Event{
		ID:        doc.ID,
		RunID:     doc.RunID,
		Type:      event.Type(doc.Type),
		Sequence:  doc.Sequence,
		Payload:   doc.Payload,
		Timestamp: doc.Timestamp,
		Version:   doc.Version,
	}
}

// wrapError wraps MongoDB errors with domain errors.
func (s *EventStore) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(event.ErrOperationTimeout, err)
	}

	return errors.Join(event.ErrConnectionFailed, err)
}

// Ensure EventStore implements event.Store and event.Querier
var (
	_ event.Store   = (*EventStore)(nil)
	_ event.Querier = (*EventStore)(nil)
)
