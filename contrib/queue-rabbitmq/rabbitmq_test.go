package rabbitmq

import (
	"testing"

	"go.klarlabs.de/agent/infrastructure/distributed/queue"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.QueueName != "agent-tasks" {
		t.Errorf("QueueName: got %s, want agent-tasks", cfg.QueueName)
	}
	if cfg.PrefetchCount != 1 {
		t.Errorf("PrefetchCount: got %d, want 1", cfg.PrefetchCount)
	}
	if !cfg.Durable {
		t.Error("expected Durable=true")
	}
}

func TestInterfaceCompliance(t *testing.T) {
	var _ queue.Queue = (*Queue)(nil)
}

func TestNewQueue_InvalidURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, err := NewQueue(Config{URL: "amqp://invalid:5672/"})
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}
