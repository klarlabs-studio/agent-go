package api_test

import (
	"testing"

	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewEvent(t *testing.T) {
	t.Parallel()

	evt, err := api.NewEvent("evt-1", api.EventRunStarted, "run-1", api.RunStartedPayload{Goal: "test"})
	if err != nil {
		t.Fatalf("NewEvent() error = %v", err)
	}
	if evt == nil {
		t.Fatal("NewEvent() returned nil")
	}
}

func TestNewWebhookNotifier(t *testing.T) {
	t.Parallel()
	notifier := api.NewWebhookNotifier(api.DefaultWebhookNotifierConfig())
	if notifier == nil {
		t.Fatal("NewWebhookNotifier() returned nil")
	}
}

func TestDefaultWebhookNotifierConfig(t *testing.T) {
	t.Parallel()
	cfg := api.DefaultWebhookNotifierConfig()
	// Just verify it doesn't panic and returns something usable
	_ = cfg
}

func TestDefaultSenderConfig(t *testing.T) {
	t.Parallel()
	cfg := api.DefaultSenderConfig()
	_ = cfg
}

func TestDefaultBatcherConfig(t *testing.T) {
	t.Parallel()
	cfg := api.DefaultBatcherConfig()
	_ = cfg
}

func TestNewSigner(t *testing.T) {
	t.Parallel()
	signer := api.NewSigner()
	if signer == nil {
		t.Fatal("NewSigner() returned nil")
	}
}

func TestFilterByType(t *testing.T) {
	t.Parallel()
	filter := api.FilterByType(api.EventRunStarted, api.EventRunCompleted)
	if filter == nil {
		t.Fatal("FilterByType() returned nil")
	}
}

func TestFilterByRunID(t *testing.T) {
	t.Parallel()
	filter := api.FilterByRunID("run-123")
	if filter == nil {
		t.Fatal("FilterByRunID() returned nil")
	}
}

func TestCombineFilters(t *testing.T) {
	t.Parallel()
	f1 := api.FilterByType(api.EventRunStarted)
	f2 := api.FilterByRunID("run-1")
	combined := api.CombineFilters(f1, f2)
	if combined == nil {
		t.Fatal("CombineFilters() returned nil")
	}
}
