package kafka

import (
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/infrastructure/distributed/queue"
)

func TestDefaultQueueConfig(t *testing.T) {
	cfg := DefaultQueueConfig()
	if cfg.Topic != "agent-tasks" {
		t.Errorf("Topic: got %s, want agent-tasks", cfg.Topic)
	}
	if cfg.GroupID != "agent-workers" {
		t.Errorf("GroupID: got %s, want agent-workers", cfg.GroupID)
	}
	if len(cfg.Brokers) != 1 || cfg.Brokers[0] != "localhost:9092" {
		t.Errorf("Brokers: got %v", cfg.Brokers)
	}
}

func TestNewQueue_NoBrokers(t *testing.T) {
	_, err := NewQueue(QueueConfig{})
	if err == nil {
		t.Error("expected error for empty brokers")
	}
}

func TestNewEventStore_NoBrokers(t *testing.T) {
	_, err := NewEventStore(EventStoreConfig{})
	if err == nil {
		t.Error("expected error for empty brokers")
	}
}

func TestQueueInterfaceCompliance(t *testing.T) {
	var _ queue.Queue = (*Queue)(nil)
}

func TestEventStoreInterfaceCompliance(t *testing.T) {
	var _ event.Store = (*EventStore)(nil)
}
