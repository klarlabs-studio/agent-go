// Package api provides the public API for the agent-go library.
// This file provides notification-related exports.
package api

import (
	domainnotif "go.klarlabs.de/agent/domain/notification"
	infranotif "go.klarlabs.de/agent/infrastructure/notification"
)

// Re-export domain notification types.
type (
	// Event represents a notification event to be sent to webhooks.
	Event = domainnotif.Event
	// EventType represents the type of notification event.
	EventType = domainnotif.EventType
	// EventFilter is a function that determines whether an event should be sent.
	EventFilter = domainnotif.EventFilter
	// Endpoint represents a webhook endpoint configuration.
	Endpoint = domainnotif.Endpoint
	// Notifier is the interface for sending notification events.
	Notifier = domainnotif.Notifier

	// Payload types for various event types.
	RunStartedPayload      = domainnotif.RunStartedPayload
	RunCompletedPayload    = domainnotif.RunCompletedPayload
	RunFailedPayload       = domainnotif.RunFailedPayload
	StateChangedPayload    = domainnotif.StateChangedPayload
	ToolStartedPayload     = domainnotif.ToolStartedPayload
	ToolCompletedPayload   = domainnotif.ToolCompletedPayload
	ToolFailedPayload      = domainnotif.ToolFailedPayload
	ApprovalNeededPayload  = domainnotif.ApprovalNeededPayload
	BudgetWarningPayload   = domainnotif.BudgetWarningPayload
	BudgetExhaustedPayload = domainnotif.BudgetExhaustedPayload
)

// Re-export infrastructure notification types.
type (
	// WebhookNotifier sends notifications to configured webhook endpoints.
	WebhookNotifier = infranotif.WebhookNotifier
	// WebhookNotifierConfig configures the webhook notifier.
	WebhookNotifierConfig = infranotif.WebhookNotifierConfig
	// SenderConfig configures the HTTP sender.
	SenderConfig = infranotif.SenderConfig
	// BatcherConfig configures the event batcher.
	BatcherConfig = infranotif.BatcherConfig
	// Signer handles payload signing for webhook requests.
	Signer = infranotif.Signer
)

// Event type constants.
const (
	EventRunStarted      = domainnotif.EventRunStarted
	EventRunCompleted    = domainnotif.EventRunCompleted
	EventRunFailed       = domainnotif.EventRunFailed
	EventRunPaused       = domainnotif.EventRunPaused
	EventStateChanged    = domainnotif.EventStateChanged
	EventToolStarted     = domainnotif.EventToolStarted
	EventToolCompleted   = domainnotif.EventToolCompleted
	EventToolFailed      = domainnotif.EventToolFailed
	EventApprovalNeeded  = domainnotif.EventApprovalNeeded
	EventBudgetWarning   = domainnotif.EventBudgetWarning
	EventBudgetExhausted = domainnotif.EventBudgetExhausted
)

// Notification errors.
var (
	// ErrEndpointUnavailable indicates the webhook endpoint is not reachable.
	ErrEndpointUnavailable = domainnotif.ErrEndpointUnavailable
	// ErrEndpointRejected indicates the endpoint rejected the notification.
	ErrEndpointRejected = domainnotif.ErrEndpointRejected
	// ErrNotifierClosed indicates the notifier has been closed.
	ErrNotifierClosed = domainnotif.ErrNotifierClosed
	// ErrInvalidEndpoint indicates the endpoint configuration is invalid.
	ErrInvalidEndpoint = domainnotif.ErrInvalidEndpoint
	// ErrBatchTooLarge indicates the batch exceeds the maximum size.
	ErrBatchTooLarge = domainnotif.ErrBatchTooLarge
	// ErrEventFilteredOut indicates the event was filtered out.
	ErrEventFilteredOut = domainnotif.ErrEventFilteredOut
	// ErrSigningFailed indicates payload signing failed.
	ErrSigningFailed = domainnotif.ErrSigningFailed
)

// NewEvent creates a new notification event.
func NewEvent(id string, eventType EventType, runID string, payload any) (*Event, error) {
	return domainnotif.NewEvent(id, eventType, runID, payload)
}

// NewWebhookNotifier creates a new webhook notifier.
func NewWebhookNotifier(config WebhookNotifierConfig) *WebhookNotifier {
	return infranotif.NewWebhookNotifier(config)
}

// DefaultWebhookNotifierConfig returns sensible default configuration for the webhook notifier.
func DefaultWebhookNotifierConfig() WebhookNotifierConfig {
	return infranotif.DefaultWebhookNotifierConfig()
}

// DefaultSenderConfig returns sensible default configuration for the HTTP sender.
func DefaultSenderConfig() SenderConfig {
	return infranotif.DefaultSenderConfig()
}

// DefaultBatcherConfig returns sensible default configuration for the event batcher.
func DefaultBatcherConfig() BatcherConfig {
	return infranotif.DefaultBatcherConfig()
}

// NewSigner creates a new payload signer.
func NewSigner() *Signer {
	return infranotif.NewSigner()
}

// Filter helper functions.

// FilterByType creates a filter that only allows events of the specified types.
func FilterByType(types ...EventType) EventFilter {
	return domainnotif.FilterByType(types...)
}

// FilterByRunID creates a filter that only allows events for the specified run ID.
func FilterByRunID(runID string) EventFilter {
	return domainnotif.FilterByRunID(runID)
}

// CombineFilters creates a filter that requires all provided filters to pass.
func CombineFilters(filters ...EventFilter) EventFilter {
	return domainnotif.CombineFilters(filters...)
}

// Note: To integrate notifications with the engine, create a WebhookNotifier
// and call its Notify methods from your application code when agent events occur.
// Example:
//
//   notifier := api.NewWebhookNotifier(api.DefaultWebhookNotifierConfig())
//   defer notifier.Close()
//
//   // After a run starts:
//   event, _ := api.NewEvent("evt-1", api.EventRunStarted, run.ID(), api.RunStartedPayload{Goal: goal})
//   notifier.Notify(ctx, event)
//
// Full engine integration is planned for a future release.
