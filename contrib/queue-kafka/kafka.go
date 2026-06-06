// Package kafka provides a Kafka-backed implementation of the distributed queue
// and event store.
//
// Tasks are serialized as JSON and published to a Kafka topic. Consumer group
// offsets provide at-least-once delivery with explicit commit on Acknowledge.
// The event store implementation uses a dedicated topic per run ID prefix,
// with events ordered by partition offset.
//
// Usage:
//
//	q, err := kafka.NewQueue(kafka.QueueConfig{
//	    Brokers: []string{"localhost:9092"},
//	    Topic:   "agent-tasks",
//	    GroupID: "agent-workers",
//	})
//	defer q.Close()
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/infrastructure/distributed/queue"
)

// QueueConfig configures the Kafka queue.
type QueueConfig struct {
	// Brokers is the list of Kafka broker addresses.
	Brokers []string

	// Topic is the Kafka topic for tasks. Defaults to "agent-tasks".
	Topic string

	// GroupID is the consumer group ID. Defaults to "agent-workers".
	GroupID string

	// MaxWait is the maximum time to wait for new messages. Defaults to 1s.
	MaxWait time.Duration
}

// DefaultQueueConfig returns sensible defaults.
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "agent-tasks",
		GroupID: "agent-workers",
		MaxWait: time.Second,
	}
}

// Queue implements queue.Queue backed by Kafka.
type Queue struct {
	writer *kafkago.Writer
	reader *kafkago.Reader
	config QueueConfig

	mu      sync.Mutex
	pending map[string]kafkago.Message // taskID -> message for commit tracking
}

// NewQueue creates a Kafka queue.
func NewQueue(cfg QueueConfig) (*Queue, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka: at least one broker is required")
	}
	if cfg.Topic == "" {
		cfg.Topic = "agent-tasks"
	}
	if cfg.GroupID == "" {
		cfg.GroupID = "agent-workers"
	}
	if cfg.MaxWait == 0 {
		cfg.MaxWait = time.Second
	}

	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireAll,
		BatchTimeout: 10 * time.Millisecond,
	}

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.GroupID,
		MaxWait:  cfg.MaxWait,
		MinBytes: 1,
		MaxBytes: 10e6, // 10MB
	})

	return &Queue{
		writer:  writer,
		reader:  reader,
		config:  cfg,
		pending: make(map[string]kafkago.Message),
	}, nil
}

// Enqueue publishes a task to the Kafka topic.
func (q *Queue) Enqueue(ctx context.Context, task queue.Task) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("kafka: marshal: %w", err)
	}

	return q.writer.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(task.ID),
		Value: body,
		Time:  time.Now(),
	})
}

// Dequeue reads the next task from the Kafka topic.
func (q *Queue) Dequeue(ctx context.Context) (*queue.Task, error) {
	msg, err := q.reader.ReadMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("kafka: read: %w", err)
	}

	var task queue.Task
	if err := json.Unmarshal(msg.Value, &task); err != nil {
		return nil, fmt.Errorf("kafka: unmarshal: %w", err)
	}

	q.mu.Lock()
	q.pending[task.ID] = msg
	q.mu.Unlock()

	return &task, nil
}

// Acknowledge commits the offset for a completed task.
func (q *Queue) Acknowledge(ctx context.Context, taskID string, _ queue.TaskResult) error {
	q.mu.Lock()
	msg, ok := q.pending[taskID]
	if ok {
		delete(q.pending, taskID)
	}
	q.mu.Unlock()

	if !ok {
		return fmt.Errorf("kafka: no pending message for task %s", taskID)
	}

	return q.reader.CommitMessages(ctx, msg)
}

// Reject handles a failed task. Kafka doesn't support requeue natively —
// if requeue is true, the task is re-published to the topic.
func (q *Queue) Reject(ctx context.Context, taskID string, _ string, requeue bool) error {
	q.mu.Lock()
	msg, ok := q.pending[taskID]
	if ok {
		delete(q.pending, taskID)
	}
	q.mu.Unlock()

	if !ok {
		return fmt.Errorf("kafka: no pending message for task %s", taskID)
	}

	// Commit the offset regardless (message was read)
	if err := q.reader.CommitMessages(ctx, msg); err != nil {
		return err
	}

	// Re-publish if requeue requested
	if requeue {
		return q.writer.WriteMessages(ctx, kafkago.Message{
			Key:   msg.Key,
			Value: msg.Value,
			Time:  time.Now(),
		})
	}

	return nil
}

// Peek is not efficiently supported by Kafka consumer groups.
func (q *Queue) Peek(_ context.Context) (*queue.Task, error) {
	return nil, nil // No-op: Kafka doesn't support peek with consumer groups
}

// Size is not supported by Kafka consumer groups.
func (q *Queue) Size(_ context.Context) (int, error) {
	return -1, nil // Kafka doesn't expose consumer lag as queue size easily
}

// Close releases Kafka resources.
func (q *Queue) Close() error {
	var errs []error
	if q.writer != nil {
		errs = append(errs, q.writer.Close())
	}
	if q.reader != nil {
		errs = append(errs, q.reader.Close())
	}
	return errors.Join(errs...)
}

// Ensure Queue implements queue.Queue.
var _ queue.Queue = (*Queue)(nil)

// --- Event Store ---

// EventStoreConfig configures the Kafka event store.
type EventStoreConfig struct {
	// Brokers is the list of Kafka broker addresses.
	Brokers []string

	// TopicPrefix is prepended to run IDs to form topic names. Defaults to "agent-events-".
	TopicPrefix string
}

// EventStore implements event.Store backed by Kafka topics.
// Each run ID maps to a separate Kafka topic for ordered event streams.
type EventStore struct {
	config  EventStoreConfig
	writers sync.Map // runID -> *kafkago.Writer
	mu      sync.Mutex
	subs    map[string][]chan event.Event // runID -> subscribers
}

// NewEventStore creates a Kafka event store.
func NewEventStore(cfg EventStoreConfig) (*EventStore, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka: at least one broker is required")
	}
	if cfg.TopicPrefix == "" {
		cfg.TopicPrefix = "agent-events-"
	}

	return &EventStore{
		config: cfg,
		subs:   make(map[string][]chan event.Event),
	}, nil
}

func (s *EventStore) writerFor(runID string) *kafkago.Writer {
	topic := s.config.TopicPrefix + runID
	if w, ok := s.writers.Load(runID); ok {
		return w.(*kafkago.Writer)
	}

	w := &kafkago.Writer{
		Addr:         kafkago.TCP(s.config.Brokers...),
		Topic:        topic,
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireAll,
		BatchTimeout: 10 * time.Millisecond,
		// Note: topic must be pre-created or auto.create.topics.enable=true on broker
	}
	s.writers.Store(runID, w)
	return w
}

// Append publishes events to the Kafka topic for the run.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if len(events) == 0 {
		return nil
	}

	runID := events[0].RunID
	writer := s.writerFor(runID)

	msgs := make([]kafkago.Message, len(events))
	for i, evt := range events {
		body, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("kafka: marshal event: %w", err)
		}
		msgs[i] = kafkago.Message{
			Key:   []byte(evt.RunID),
			Value: body,
			Time:  evt.Timestamp,
		}
	}

	if err := writer.WriteMessages(ctx, msgs...); err != nil {
		return fmt.Errorf("kafka: write events: %w", err)
	}

	// Notify subscribers
	s.mu.Lock()
	subs := s.subs[runID]
	s.mu.Unlock()

	for _, ch := range subs {
		for _, evt := range events {
			select {
			case ch <- evt:
			default: // non-blocking
			}
		}
	}

	return nil
}

// LoadEvents reads all events for a run from the Kafka topic.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	return s.LoadEventsFrom(ctx, runID, 0)
}

// LoadEventsFrom reads events starting from a sequence number.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, _ uint64) ([]event.Event, error) {
	topic := s.config.TopicPrefix + runID
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  s.config.Brokers,
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  500 * time.Millisecond,
	})
	defer func() { _ = reader.Close() }()

	var events []event.Event
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		msg, err := reader.ReadMessage(readCtx)
		if err != nil {
			break // timeout or end of topic
		}

		var evt event.Event
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			continue
		}
		events = append(events, evt)
	}

	return events, nil
}

// Subscribe returns a channel that receives new events for a run.
func (s *EventStore) Subscribe(_ context.Context, runID string) (<-chan event.Event, error) {
	ch := make(chan event.Event, 100)

	s.mu.Lock()
	s.subs[runID] = append(s.subs[runID], ch)
	s.mu.Unlock()

	return ch, nil
}

// Ensure EventStore implements event.Store.
var _ event.Store = (*EventStore)(nil)
