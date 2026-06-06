package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/notification"
)

func TestWebhookNotifier_Notify(t *testing.T) {
	var receivedEvents []*notification.Event
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var events []*notification.Event
		json.Unmarshal(body, &events)
		mu.Lock()
		receivedEvents = append(receivedEvents, events...)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := WebhookNotifierConfig{
		Endpoints: []*notification.Endpoint{
			{
				URL:     server.URL,
				Enabled: true,
			},
		},
		EnableBatching: false,
		SenderConfig:   DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)
	defer notifier.Close()

	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	ctx := context.Background()
	err := notifier.Notify(ctx, event)
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	mu.Lock()
	if len(receivedEvents) != 1 {
		t.Errorf("should have 1 event, got %d", len(receivedEvents))
	}
	mu.Unlock()
}

func TestWebhookNotifier_NotifyWithBatching(t *testing.T) {
	var batches []int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var events []*notification.Event
		json.Unmarshal(body, &events)
		mu.Lock()
		batches = append(batches, len(events))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := WebhookNotifierConfig{
		Endpoints: []*notification.Endpoint{
			{
				URL:     server.URL,
				Enabled: true,
			},
		},
		EnableBatching: true,
		BatcherConfig: BatcherConfig{
			MaxBatchSize: 3,
			MaxWait:      1 * time.Hour,
		},
		SenderConfig: DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)

	ctx := context.Background()

	// Add 5 events
	for i := 0; i < 5; i++ {
		event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})
		if err := notifier.Notify(ctx, event); err != nil {
			t.Fatalf("Notify() error = %v", err)
		}
	}

	// Should have auto-flushed one batch of 3
	mu.Lock()
	if len(batches) != 1 || batches[0] != 3 {
		t.Errorf("should have 1 batch of 3, got batches: %v", batches)
	}
	mu.Unlock()

	// Close should flush remaining
	notifier.Close()

	mu.Lock()
	if len(batches) != 2 || batches[1] != 2 {
		t.Errorf("close should flush remaining 2, got batches: %v", batches)
	}
	mu.Unlock()
}

func TestWebhookNotifier_NotifyBatch(t *testing.T) {
	var receivedEvents []*notification.Event
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var events []*notification.Event
		json.Unmarshal(body, &events)
		mu.Lock()
		receivedEvents = append(receivedEvents, events...)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := WebhookNotifierConfig{
		Endpoints: []*notification.Endpoint{
			{
				URL:     server.URL,
				Enabled: true,
			},
		},
		EnableBatching: false,
		SenderConfig:   DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)
	defer notifier.Close()

	events := []*notification.Event{}
	for i := 0; i < 5; i++ {
		event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})
		events = append(events, event)
	}

	ctx := context.Background()
	err := notifier.NotifyBatch(ctx, events)
	if err != nil {
		t.Fatalf("NotifyBatch() error = %v", err)
	}

	mu.Lock()
	if len(receivedEvents) != 5 {
		t.Errorf("should have 5 events, got %d", len(receivedEvents))
	}
	mu.Unlock()
}

func TestWebhookNotifier_GlobalFilter(t *testing.T) {
	var receivedCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var events []*notification.Event
		json.Unmarshal(body, &events)
		atomic.AddInt32(&receivedCount, int32(len(events)))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Filter to only allow RunStarted events
	config := WebhookNotifierConfig{
		Endpoints: []*notification.Endpoint{
			{
				URL:     server.URL,
				Enabled: true,
			},
		},
		EnableBatching: false,
		GlobalFilter: func(e *notification.Event) bool {
			return e.Type == notification.EventRunStarted
		},
		SenderConfig: DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)
	defer notifier.Close()

	ctx := context.Background()

	// Send different event types
	event1, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})
	event2, _ := notification.NewEvent("evt-2", notification.EventRunCompleted, "run-1", notification.RunCompletedPayload{Steps: 10})
	event3, _ := notification.NewEvent("evt-3", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test2"})

	notifier.Notify(ctx, event1) // Should pass
	notifier.Notify(ctx, event2) // Should be filtered
	notifier.Notify(ctx, event3) // Should pass

	if atomic.LoadInt32(&receivedCount) != 2 {
		t.Errorf("should have 2 events (filtered), got %d", atomic.LoadInt32(&receivedCount))
	}
}

func TestWebhookNotifier_EndpointFilter(t *testing.T) {
	var endpoint1Count, endpoint2Count int32

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var events []*notification.Event
		json.Unmarshal(body, &events)
		atomic.AddInt32(&endpoint1Count, int32(len(events)))
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var events []*notification.Event
		json.Unmarshal(body, &events)
		atomic.AddInt32(&endpoint2Count, int32(len(events)))
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	config := WebhookNotifierConfig{
		Endpoints: []*notification.Endpoint{
			{
				Name:    "all-events",
				URL:     server1.URL,
				Enabled: true,
			},
			{
				Name:    "failures-only",
				URL:     server2.URL,
				Enabled: true,
				Filter: func(e *notification.Event) bool {
					return e.Type == notification.EventRunFailed || e.Type == notification.EventToolFailed
				},
			},
		},
		EnableBatching: false,
		SenderConfig:   DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)
	defer notifier.Close()

	ctx := context.Background()

	event1, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})
	event2, _ := notification.NewEvent("evt-2", notification.EventRunFailed, "run-1", notification.RunFailedPayload{Error: "oops"})

	notifier.Notify(ctx, event1)
	notifier.Notify(ctx, event2)

	// Wait for async delivery
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&endpoint1Count) != 2 {
		t.Errorf("endpoint1 should have 2 events, got %d", atomic.LoadInt32(&endpoint1Count))
	}
	if atomic.LoadInt32(&endpoint2Count) != 1 {
		t.Errorf("endpoint2 should have 1 event (failures-only), got %d", atomic.LoadInt32(&endpoint2Count))
	}
}

func TestWebhookNotifier_DisabledEndpoint(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := WebhookNotifierConfig{
		Endpoints: []*notification.Endpoint{
			{
				URL:     server.URL,
				Enabled: false, // Disabled
			},
		},
		EnableBatching: false,
		SenderConfig:   DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)
	defer notifier.Close()

	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	ctx := context.Background()
	err := notifier.Notify(ctx, event)
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	if atomic.LoadInt32(&requestCount) != 0 {
		t.Error("disabled endpoint should not receive requests")
	}
}

func TestWebhookNotifier_AddRemoveEndpoint(t *testing.T) {
	notifier := NewWebhookNotifier(DefaultWebhookNotifierConfig())
	defer notifier.Close()

	// Initial count
	if len(notifier.Endpoints()) != 0 {
		t.Errorf("should have 0 endpoints, got %d", len(notifier.Endpoints()))
	}

	// Add endpoints
	notifier.AddEndpoint(&notification.Endpoint{URL: "http://example1.com", Enabled: true})
	notifier.AddEndpoint(&notification.Endpoint{URL: "http://example2.com", Enabled: true})

	if len(notifier.Endpoints()) != 2 {
		t.Errorf("should have 2 endpoints, got %d", len(notifier.Endpoints()))
	}

	// Remove endpoint
	notifier.RemoveEndpoint("http://example1.com")

	if len(notifier.Endpoints()) != 1 {
		t.Errorf("should have 1 endpoint after remove, got %d", len(notifier.Endpoints()))
	}
	if notifier.Endpoints()[0].URL != "http://example2.com" {
		t.Errorf("remaining endpoint should be example2.com")
	}
}

func TestWebhookNotifier_Close(t *testing.T) {
	config := WebhookNotifierConfig{
		EnableBatching: false,
		SenderConfig:   DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)

	// Close
	err := notifier.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Notify after close should fail
	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	ctx := context.Background()
	err = notifier.Notify(ctx, event)
	if err != notification.ErrNotifierClosed {
		t.Errorf("Notify() after close should return ErrNotifierClosed, got %v", err)
	}

	// NotifyBatch after close should fail
	err = notifier.NotifyBatch(ctx, []*notification.Event{event})
	if err != notification.ErrNotifierClosed {
		t.Errorf("NotifyBatch() after close should return ErrNotifierClosed, got %v", err)
	}
}

func TestWebhookNotifier_Flush(t *testing.T) {
	var receivedCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var events []*notification.Event
		json.Unmarshal(body, &events)
		atomic.AddInt32(&receivedCount, int32(len(events)))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := WebhookNotifierConfig{
		Endpoints: []*notification.Endpoint{
			{
				URL:     server.URL,
				Enabled: true,
			},
		},
		EnableBatching: true,
		BatcherConfig: BatcherConfig{
			MaxBatchSize: 100,
			MaxWait:      1 * time.Hour, // Long wait
		},
		SenderConfig: DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)
	defer notifier.Close()

	ctx := context.Background()

	// Add events
	for i := 0; i < 5; i++ {
		event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})
		notifier.Notify(ctx, event)
	}

	// Should not have sent yet (waiting for batch)
	if atomic.LoadInt32(&receivedCount) != 0 {
		t.Errorf("should not have sent yet, got %d", atomic.LoadInt32(&receivedCount))
	}

	// Flush
	err := notifier.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	if atomic.LoadInt32(&receivedCount) != 5 {
		t.Errorf("should have 5 events after flush, got %d", atomic.LoadInt32(&receivedCount))
	}
}

func TestWebhookNotifier_DefaultConfig(t *testing.T) {
	config := DefaultWebhookNotifierConfig()

	if !config.EnableBatching {
		t.Error("EnableBatching should be true by default")
	}
	if config.BatcherConfig.MaxBatchSize != 100 {
		t.Errorf("BatcherConfig.MaxBatchSize = %d, want 100", config.BatcherConfig.MaxBatchSize)
	}
	if config.SenderConfig.Timeout != 30*time.Second {
		t.Errorf("SenderConfig.Timeout = %v, want 30s", config.SenderConfig.Timeout)
	}
}

func TestWebhookNotifier_MultipleEndpoints(t *testing.T) {
	var endpoint1Count, endpoint2Count int32

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&endpoint1Count, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&endpoint2Count, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	config := WebhookNotifierConfig{
		Endpoints: []*notification.Endpoint{
			{URL: server1.URL, Enabled: true},
			{URL: server2.URL, Enabled: true},
		},
		EnableBatching: false,
		SenderConfig:   DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)
	defer notifier.Close()

	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	ctx := context.Background()
	err := notifier.Notify(ctx, event)
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	// Wait for async delivery
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&endpoint1Count) != 1 {
		t.Errorf("endpoint1 should have 1 request, got %d", atomic.LoadInt32(&endpoint1Count))
	}
	if atomic.LoadInt32(&endpoint2Count) != 1 {
		t.Errorf("endpoint2 should have 1 request, got %d", atomic.LoadInt32(&endpoint2Count))
	}
}

func TestWebhookNotifier_FlushWithoutBatching(t *testing.T) {
	config := WebhookNotifierConfig{
		EnableBatching: false,
		SenderConfig:   DefaultSenderConfig(),
	}

	notifier := NewWebhookNotifier(config)
	defer notifier.Close()

	ctx := context.Background()
	err := notifier.Flush(ctx)
	if err != nil {
		t.Errorf("Flush() without batching should return nil, got %v", err)
	}
}
