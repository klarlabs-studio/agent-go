package notification

import (
	"context"
	"sync"

	"go.klarlabs.de/agent/domain/notification"
	"go.klarlabs.de/agent/infrastructure/logging"
)

// WebhookNotifierConfig configures the webhook notifier.
type WebhookNotifierConfig struct {
	// Endpoints are the webhook endpoints to notify.
	Endpoints []*notification.Endpoint
	// EnableBatching enables event batching.
	EnableBatching bool
	// BatcherConfig configures the batcher (if enabled).
	BatcherConfig BatcherConfig
	// SenderConfig configures the HTTP sender.
	SenderConfig SenderConfig
	// GlobalFilter is applied to all events before endpoint filters.
	GlobalFilter notification.EventFilter
}

// DefaultWebhookNotifierConfig returns sensible defaults.
func DefaultWebhookNotifierConfig() WebhookNotifierConfig {
	return WebhookNotifierConfig{
		EnableBatching: true,
		BatcherConfig:  DefaultBatcherConfig(),
		SenderConfig:   DefaultSenderConfig(),
	}
}

// WebhookNotifier sends notifications to configured webhook endpoints.
type WebhookNotifier struct {
	config    WebhookNotifierConfig
	endpoints []*notification.Endpoint
	sender    *Sender
	batcher   *Batcher
	closed    bool
	closedMu  sync.RWMutex
}

// NewWebhookNotifier creates a new webhook notifier.
func NewWebhookNotifier(config WebhookNotifierConfig) *WebhookNotifier {
	sender := NewSender(config.SenderConfig)

	notifier := &WebhookNotifier{
		config:    config,
		endpoints: config.Endpoints,
		sender:    sender,
	}

	if config.EnableBatching {
		batcherConfig := config.BatcherConfig
		batcherConfig.OnBatch = func(ctx context.Context, events []*notification.Event) error {
			return notifier.sendToAllEndpoints(ctx, events)
		}
		notifier.batcher = NewBatcher(batcherConfig)
	}

	return notifier
}

// Notify sends a single event to all configured endpoints.
func (w *WebhookNotifier) Notify(ctx context.Context, event *notification.Event) error {
	w.closedMu.RLock()
	if w.closed {
		w.closedMu.RUnlock()
		return notification.ErrNotifierClosed
	}
	w.closedMu.RUnlock()

	// Apply global filter
	if w.config.GlobalFilter != nil && !w.config.GlobalFilter(event) {
		return nil // Silently skip filtered events
	}

	if w.batcher != nil {
		return w.batcher.Add(ctx, event)
	}

	return w.sendToAllEndpoints(ctx, []*notification.Event{event})
}

// NotifyBatch sends multiple events to all configured endpoints.
func (w *WebhookNotifier) NotifyBatch(ctx context.Context, events []*notification.Event) error {
	w.closedMu.RLock()
	if w.closed {
		w.closedMu.RUnlock()
		return notification.ErrNotifierClosed
	}
	w.closedMu.RUnlock()

	// Apply global filter
	filtered := make([]*notification.Event, 0, len(events))
	for _, event := range events {
		if w.config.GlobalFilter == nil || w.config.GlobalFilter(event) {
			filtered = append(filtered, event)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	if w.batcher != nil {
		for _, event := range filtered {
			if err := w.batcher.Add(ctx, event); err != nil {
				return err
			}
		}
		return nil
	}

	return w.sendToAllEndpoints(ctx, filtered)
}

// Close flushes pending events and closes the notifier.
func (w *WebhookNotifier) Close() error {
	w.closedMu.Lock()
	w.closed = true
	w.closedMu.Unlock()

	if w.batcher != nil {
		return w.batcher.Close(context.Background())
	}
	return nil
}

// Flush immediately sends any pending batched events.
func (w *WebhookNotifier) Flush(ctx context.Context) error {
	if w.batcher != nil {
		return w.batcher.Flush(ctx)
	}
	return nil
}

// AddEndpoint adds a new endpoint to the notifier.
func (w *WebhookNotifier) AddEndpoint(endpoint *notification.Endpoint) {
	w.endpoints = append(w.endpoints, endpoint)
}

// RemoveEndpoint removes an endpoint by URL.
func (w *WebhookNotifier) RemoveEndpoint(url string) {
	filtered := make([]*notification.Endpoint, 0, len(w.endpoints))
	for _, ep := range w.endpoints {
		if ep.URL != url {
			filtered = append(filtered, ep)
		}
	}
	w.endpoints = filtered
}

// Endpoints returns the configured endpoints.
func (w *WebhookNotifier) Endpoints() []*notification.Endpoint {
	return w.endpoints
}

// sendToAllEndpoints sends events to all enabled endpoints.
func (w *WebhookNotifier) sendToAllEndpoints(ctx context.Context, events []*notification.Event) error {
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for _, endpoint := range w.endpoints {
		if !endpoint.Enabled {
			continue
		}

		// Filter events for this endpoint
		endpointEvents := events
		if endpoint.Filter != nil {
			endpointEvents = make([]*notification.Event, 0)
			for _, event := range events {
				if endpoint.Filter(event) {
					endpointEvents = append(endpointEvents, event)
				}
			}
		}

		if len(endpointEvents) == 0 {
			continue
		}

		wg.Add(1)
		go func(ep *notification.Endpoint, evts []*notification.Event) {
			defer wg.Done()

			err := w.sender.SendBatch(ctx, ep, evts)
			if err != nil {
				logging.Error().
					Add(logging.Str("endpoint", ep.URL)).
					Add(logging.Str("endpoint_name", ep.Name)).
					Add(logging.Int("event_count", len(evts))).
					Add(logging.ErrorField(err)).
					Msg("webhook delivery failed")

				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			} else {
				logging.Debug().
					Add(logging.Str("endpoint", ep.URL)).
					Add(logging.Int("event_count", len(evts))).
					Msg("webhook delivered")
			}
		}(endpoint, endpointEvents)
	}

	wg.Wait()
	return firstErr
}
