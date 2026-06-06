package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/notification"
)

func TestSender_Send(t *testing.T) {
	var receivedBody []byte
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := SenderConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		UserAgent:  "test-agent/1.0",
	}

	sender := NewSender(config)

	endpoint := &notification.Endpoint{
		URL:     server.URL,
		Enabled: true,
	}

	event, err := notification.NewEvent("evt-1", notification.EventRunStarted, "run-123", notification.RunStartedPayload{
		Goal: "Test goal",
	})
	if err != nil {
		t.Fatalf("NewEvent() error = %v", err)
	}

	ctx := context.Background()
	err = sender.Send(ctx, endpoint, event)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Verify headers
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", receivedHeaders.Get("Content-Type"))
	}
	if receivedHeaders.Get("User-Agent") != "test-agent/1.0" {
		t.Errorf("User-Agent = %s, want test-agent/1.0", receivedHeaders.Get("User-Agent"))
	}

	// Verify body
	var events []*notification.Event
	if err := json.Unmarshal(receivedBody, &events); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("should have 1 event, got %d", len(events))
	}
}

func TestSender_SendWithSignature(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(DefaultSenderConfig())

	endpoint := &notification.Endpoint{
		URL:     server.URL,
		Enabled: true,
		Secret:  "test-secret",
	}

	event, err := notification.NewEvent("evt-1", notification.EventRunStarted, "run-123", notification.RunStartedPayload{
		Goal: "test goal",
	})
	if err != nil {
		t.Fatalf("NewEvent() error = %v", err)
	}

	ctx := context.Background()
	err = sender.Send(ctx, endpoint, event)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Verify signature headers are present
	if receivedHeaders.Get("X-Webhook-Signature") == "" {
		t.Error("X-Webhook-Signature header should be present")
	}
	if receivedHeaders.Get("X-Webhook-Timestamp") == "" {
		t.Error("X-Webhook-Timestamp header should be present")
	}
	if receivedHeaders.Get("X-Webhook-Signature-V2") == "" {
		t.Error("X-Webhook-Signature-V2 header should be present")
	}
}

func TestSender_SendWithCustomHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(DefaultSenderConfig())

	endpoint := &notification.Endpoint{
		URL:     server.URL,
		Enabled: true,
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer token123",
		},
	}

	event, err := notification.NewEvent("evt-1", notification.EventRunStarted, "run-123", notification.RunStartedPayload{
		Goal: "test goal",
	})
	if err != nil {
		t.Fatalf("NewEvent() error = %v", err)
	}

	ctx := context.Background()
	err = sender.Send(ctx, endpoint, event)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("X-Custom-Header = %s, want custom-value", receivedHeaders.Get("X-Custom-Header"))
	}
	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Errorf("Authorization = %s, want Bearer token123", receivedHeaders.Get("Authorization"))
	}
}

func TestSender_SendBatch(t *testing.T) {
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(DefaultSenderConfig())

	endpoint := &notification.Endpoint{
		URL:     server.URL,
		Enabled: true,
	}

	event1, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "goal1"})
	event2, _ := notification.NewEvent("evt-2", notification.EventRunCompleted, "run-1", notification.RunCompletedPayload{Steps: 10})
	event3, _ := notification.NewEvent("evt-3", notification.EventStateChanged, "run-1", notification.StateChangedPayload{FromState: "a", ToState: "b"})

	events := []*notification.Event{event1, event2, event3}

	ctx := context.Background()
	err := sender.SendBatch(ctx, endpoint, events)
	if err != nil {
		t.Fatalf("SendBatch() error = %v", err)
	}

	var received []*notification.Event
	if err := json.Unmarshal(receivedBody, &received); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(received) != 3 {
		t.Errorf("should have 3 events, got %d", len(received))
	}
}

func TestSender_SendInvalidEndpoint(t *testing.T) {
	sender := NewSender(DefaultSenderConfig())
	ctx := context.Background()

	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	tests := []struct {
		name     string
		endpoint *notification.Endpoint
	}{
		{
			name:     "nil endpoint",
			endpoint: nil,
		},
		{
			name:     "empty URL",
			endpoint: &notification.Endpoint{URL: "", Enabled: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sender.Send(ctx, tt.endpoint, event)
			if err != notification.ErrInvalidEndpoint {
				t.Errorf("Send() error = %v, want ErrInvalidEndpoint", err)
			}
		})
	}
}

func TestSender_ServerError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	config := SenderConfig{
		Timeout:                 5 * time.Second,
		MaxRetries:              3,
		RetryDelay:              10 * time.Millisecond,
		CircuitBreakerThreshold: 10, // High threshold to avoid circuit break
	}

	sender := NewSender(config)

	endpoint := &notification.Endpoint{
		URL:     server.URL,
		Enabled: true,
	}

	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	ctx := context.Background()
	err := sender.Send(ctx, endpoint, event)
	if err == nil {
		t.Error("Send() should return error for server error")
	}

	// Should have retried
	if atomic.LoadInt32(&attempts) < 2 {
		t.Errorf("should have retried, got %d attempts", atomic.LoadInt32(&attempts))
	}
}

func TestSender_ClientError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	config := SenderConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
	}

	sender := NewSender(config)

	endpoint := &notification.Endpoint{
		URL:     server.URL,
		Enabled: true,
	}

	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	ctx := context.Background()
	err := sender.Send(ctx, endpoint, event)
	if err == nil {
		t.Error("Send() should return error for client error")
	}

	// Should NOT retry on client errors (4xx)
	if atomic.LoadInt32(&attempts) > 1 {
		t.Errorf("should not retry on 4xx, got %d attempts", atomic.LoadInt32(&attempts))
	}
}

func TestSender_BreakerState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(DefaultSenderConfig())

	// Unknown endpoint
	state := sender.BreakerState("http://unknown")
	if state != "unknown" {
		t.Errorf("BreakerState() for unknown = %s, want unknown", state)
	}

	// After request
	endpoint := &notification.Endpoint{
		URL:     server.URL,
		Enabled: true,
	}

	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	ctx := context.Background()
	_ = sender.Send(ctx, endpoint, event)

	state = sender.BreakerState(server.URL)
	if state == "unknown" {
		t.Error("BreakerState() should not be unknown after request")
	}
}

func TestSender_DefaultConfig(t *testing.T) {
	config := DefaultSenderConfig()

	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", config.Timeout)
	}
	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.RetryDelay != 1*time.Second {
		t.Errorf("RetryDelay = %v, want 1s", config.RetryDelay)
	}
	if config.CircuitBreakerThreshold != 5 {
		t.Errorf("CircuitBreakerThreshold = %d, want 5", config.CircuitBreakerThreshold)
	}
	if config.CircuitBreakerTimeout != 30*time.Second {
		t.Errorf("CircuitBreakerTimeout = %v, want 30s", config.CircuitBreakerTimeout)
	}
	if config.UserAgent != "agent-go-webhook/1.0" {
		t.Errorf("UserAgent = %s, want agent-go-webhook/1.0", config.UserAgent)
	}
}

func TestSender_ConfigDefaults(t *testing.T) {
	// Empty config should get defaults
	config := SenderConfig{}
	sender := NewSender(config)

	// Verify by checking that it works (would fail with 0 values)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "agent-go-webhook/1.0" {
			t.Errorf("default User-Agent not applied")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint := &notification.Endpoint{
		URL:     server.URL,
		Enabled: true,
	}

	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	ctx := context.Background()
	err := sender.Send(ctx, endpoint, event)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestSender_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := SenderConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 1,
	}

	sender := NewSender(config)

	endpoint := &notification.Endpoint{
		URL:     server.URL,
		Enabled: true,
	}

	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, "run-1", notification.RunStartedPayload{Goal: "test"})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := sender.Send(ctx, endpoint, event)
	if err == nil {
		t.Error("Send() should return error on context cancellation")
	}
}
