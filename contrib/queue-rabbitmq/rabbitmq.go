// Package rabbitmq provides a RabbitMQ-backed implementation of the distributed queue.
//
// Tasks are published to a durable queue with JSON serialization. Consumer
// acknowledgements map to the Queue interface's Acknowledge/Reject pattern.
// Dead-letter exchange handles failed tasks.
//
// Usage:
//
//	q, err := rabbitmq.NewQueue(rabbitmq.Config{URL: "amqp://guest:guest@localhost:5672/"})
//	defer q.Close()
//	q.Enqueue(ctx, task)
//	task, _ := q.Dequeue(ctx)
package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"go.klarlabs.de/agent/infrastructure/distributed/queue"
)

// Config configures the RabbitMQ queue.
type Config struct {
	// URL is the AMQP connection string.
	URL string

	// QueueName is the name of the RabbitMQ queue. Defaults to "agent-tasks".
	QueueName string

	// DeadLetterExchange is the exchange for rejected messages. Defaults to "agent-dlx".
	DeadLetterExchange string

	// PrefetchCount limits unacknowledged messages per consumer. Defaults to 1.
	PrefetchCount int

	// Durable makes the queue survive broker restarts. Defaults to true.
	Durable bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		URL:                "amqp://guest:guest@localhost:5672/",
		QueueName:          "agent-tasks",
		DeadLetterExchange: "agent-dlx",
		PrefetchCount:      1,
		Durable:            true,
	}
}

// Queue implements queue.Queue backed by RabbitMQ.
type Queue struct {
	config Config
	conn   *amqp.Connection
	ch     *amqp.Channel

	mu          sync.Mutex
	deliveries  map[string]amqp.Delivery // taskID -> delivery for ack/reject
	consumerTag string
}

// NewQueue creates a RabbitMQ queue with the given config.
func NewQueue(cfg Config) (*Queue, error) {
	if cfg.QueueName == "" {
		cfg.QueueName = "agent-tasks"
	}
	if cfg.DeadLetterExchange == "" {
		cfg.DeadLetterExchange = "agent-dlx"
	}
	if cfg.PrefetchCount == 0 {
		cfg.PrefetchCount = 1
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq: channel: %w", err)
	}

	// Declare dead letter exchange
	if err := ch.ExchangeDeclare(cfg.DeadLetterExchange, "fanout", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq: declare DLX: %w", err)
	}

	// Declare queue with dead letter routing
	args := amqp.Table{
		"x-dead-letter-exchange": cfg.DeadLetterExchange,
	}
	_, err = ch.QueueDeclare(cfg.QueueName, cfg.Durable, false, false, false, args)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq: declare queue: %w", err)
	}

	if err := ch.Qos(cfg.PrefetchCount, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq: qos: %w", err)
	}

	return &Queue{
		config:     cfg,
		conn:       conn,
		ch:         ch,
		deliveries: make(map[string]amqp.Delivery),
	}, nil
}

// Enqueue publishes a task to the RabbitMQ queue.
func (q *Queue) Enqueue(ctx context.Context, task queue.Task) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("rabbitmq: marshal: %w", err)
	}

	return q.ch.PublishWithContext(ctx, "", q.config.QueueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    task.ID,
		Timestamp:    time.Now(),
		Body:         body,
	})
}

// Dequeue consumes the next task from the queue.
func (q *Queue) Dequeue(ctx context.Context) (*queue.Task, error) {
	q.mu.Lock()
	if q.consumerTag == "" {
		tag := fmt.Sprintf("agent-%d", time.Now().UnixNano())
		q.consumerTag = tag
		q.mu.Unlock()

		// Start consuming
		msgs, err := q.ch.Consume(q.config.QueueName, tag, false, false, false, false, nil)
		if err != nil {
			return nil, fmt.Errorf("rabbitmq: consume: %w", err)
		}

		// Wait for a message
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case d, ok := <-msgs:
			if !ok {
				return nil, errors.New("rabbitmq: channel closed")
			}

			var task queue.Task
			if err := json.Unmarshal(d.Body, &task); err != nil {
				_ = d.Nack(false, false)
				return nil, fmt.Errorf("rabbitmq: unmarshal: %w", err)
			}

			q.mu.Lock()
			q.deliveries[task.ID] = d
			q.mu.Unlock()

			return &task, nil
		}
	}
	q.mu.Unlock()

	// Already consuming — this simple implementation only supports one consumer
	return nil, errors.New("rabbitmq: already consuming, call Acknowledge/Reject first")
}

// Acknowledge marks a task as completed.
func (q *Queue) Acknowledge(_ context.Context, taskID string, _ queue.TaskResult) error {
	q.mu.Lock()
	d, ok := q.deliveries[taskID]
	if ok {
		delete(q.deliveries, taskID)
	}
	q.mu.Unlock()

	if !ok {
		return fmt.Errorf("rabbitmq: no delivery for task %s", taskID)
	}
	return d.Ack(false)
}

// Reject marks a task as failed. If requeue is true, the task is requeued.
func (q *Queue) Reject(_ context.Context, taskID string, _ string, requeue bool) error {
	q.mu.Lock()
	d, ok := q.deliveries[taskID]
	if ok {
		delete(q.deliveries, taskID)
	}
	q.mu.Unlock()

	if !ok {
		return fmt.Errorf("rabbitmq: no delivery for task %s", taskID)
	}
	return d.Nack(false, requeue)
}

// Peek is not natively supported by RabbitMQ — returns nil.
func (q *Queue) Peek(_ context.Context) (*queue.Task, error) {
	msg, ok, err := q.ch.Get(q.config.QueueName, false)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: get: %w", err)
	}
	if !ok {
		return nil, nil
	}

	var task queue.Task
	if err := json.Unmarshal(msg.Body, &task); err != nil {
		_ = msg.Nack(false, true)
		return nil, fmt.Errorf("rabbitmq: unmarshal: %w", err)
	}

	// Re-queue since Peek shouldn't consume
	_ = msg.Nack(false, true)
	return &task, nil
}

// Size returns the approximate queue length.
func (q *Queue) Size(_ context.Context) (int, error) {
	qi, err := q.ch.QueueDeclarePassive(q.config.QueueName, q.config.Durable, false, false, false, nil)
	if err != nil {
		return 0, fmt.Errorf("rabbitmq: inspect: %w", err)
	}
	return qi.Messages, nil
}

// Close releases RabbitMQ resources.
func (q *Queue) Close() error {
	var errs []error
	if q.ch != nil {
		errs = append(errs, q.ch.Close())
	}
	if q.conn != nil {
		errs = append(errs, q.conn.Close())
	}
	return errors.Join(errs...)
}

// Ensure Queue implements queue.Queue.
var _ queue.Queue = (*Queue)(nil)
