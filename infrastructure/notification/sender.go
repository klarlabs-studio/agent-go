package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/felixgeelhaar/fortify/circuitbreaker"
	"github.com/felixgeelhaar/fortify/retry"

	"github.com/felixgeelhaar/agent-go/domain/notification"
)

// SenderConfig configures the HTTP sender.
type SenderConfig struct {
	// Timeout is the HTTP request timeout.
	Timeout time.Duration
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// RetryDelay is the initial delay between retries.
	RetryDelay time.Duration
	// CircuitBreakerThreshold is failures before opening circuit.
	CircuitBreakerThreshold int
	// CircuitBreakerTimeout is how long circuit stays open.
	CircuitBreakerTimeout time.Duration
	// UserAgent is the User-Agent header value.
	UserAgent string
}

// DefaultSenderConfig returns sensible default configuration.
func DefaultSenderConfig() SenderConfig {
	return SenderConfig{
		Timeout:                 30 * time.Second,
		MaxRetries:              3,
		RetryDelay:              1 * time.Second,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30 * time.Second,
		UserAgent:               "agent-go-webhook/1.0",
	}
}

// Sender handles HTTP delivery of webhook notifications.
type Sender struct {
	config   SenderConfig
	client   *http.Client
	signer   *Signer
	breakers map[string]circuitbreaker.CircuitBreaker[*http.Response]
	retrier  retry.Retry[*http.Response]
	mu       sync.RWMutex
}

// NewSender creates a new HTTP sender.
func NewSender(config SenderConfig) *Sender {
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.CircuitBreakerThreshold <= 0 {
		config.CircuitBreakerThreshold = 5
	}
	if config.CircuitBreakerTimeout <= 0 {
		config.CircuitBreakerTimeout = 30 * time.Second
	}
	if config.UserAgent == "" {
		config.UserAgent = "agent-go-webhook/1.0"
	}

	return &Sender{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		signer:   NewSigner(),
		breakers: make(map[string]circuitbreaker.CircuitBreaker[*http.Response]),
		retrier: retry.New[*http.Response](retry.Config{
			MaxAttempts:   config.MaxRetries,
			InitialDelay:  config.RetryDelay,
			BackoffPolicy: retry.BackoffExponential,
			Multiplier:    2.0,
			// Don't retry on client errors (4xx) - only server errors (5xx)
			NonRetryableErrors: []error{notification.ErrEndpointRejected},
		}),
	}
}

// Send sends an event to the specified endpoint.
func (s *Sender) Send(ctx context.Context, endpoint *notification.Endpoint, event *notification.Event) error {
	events := []*notification.Event{event}
	return s.SendBatch(ctx, endpoint, events)
}

// SendBatch sends multiple events to the specified endpoint.
func (s *Sender) SendBatch(ctx context.Context, endpoint *notification.Endpoint, events []*notification.Event) error {
	if endpoint == nil || endpoint.URL == "" {
		return notification.ErrInvalidEndpoint
	}

	// Get or create circuit breaker for this endpoint
	breaker := s.getBreaker(endpoint.URL)

	// Serialize events
	payload, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to serialize events: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", s.config.UserAgent)

	// Add custom headers
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	// Add signature headers if secret is configured
	if endpoint.Secret != "" {
		signedHeaders := s.signer.SignedHeaders(payload, endpoint.Secret, time.Now())
		for key, value := range signedHeaders {
			req.Header.Set(key, value)
		}
	}

	// Execute with circuit breaker and retry
	_, err = breaker.Execute(ctx, func(ctx context.Context) (*http.Response, error) {
		return s.retrier.Execute(ctx, func(ctx context.Context) (*http.Response, error) {
			resp, err := s.client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", notification.ErrEndpointUnavailable, err)
			}
			defer resp.Body.Close()

			// Read body for error messages
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

			// Check status code
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return resp, nil
			}

			if resp.StatusCode >= 500 {
				// Server error - should retry
				return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
			}

			// Client error - should not retry
			return nil, fmt.Errorf("%w: status %d: %s", notification.ErrEndpointRejected, resp.StatusCode, string(body))
		})
	})

	return err
}

// getBreaker returns the circuit breaker for an endpoint, creating one if needed.
func (s *Sender) getBreaker(url string) circuitbreaker.CircuitBreaker[*http.Response] {
	s.mu.RLock()
	breaker, exists := s.breakers[url]
	s.mu.RUnlock()

	if exists {
		return breaker
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists = s.breakers[url]; exists {
		return breaker
	}

	threshold := s.config.CircuitBreakerThreshold
	if threshold < 0 {
		threshold = 5
	}

	breaker = circuitbreaker.New[*http.Response](circuitbreaker.Config{
		MaxRequests: 10,
		Interval:    s.config.CircuitBreakerTimeout,
		Timeout:     s.config.CircuitBreakerTimeout,
		ReadyToTrip: func(counts circuitbreaker.Counts) bool {
			return counts.ConsecutiveFailures >= uint32(threshold) // #nosec G115 -- threshold is validated
		},
	})
	s.breakers[url] = breaker

	return breaker
}

// BreakerState returns the circuit breaker state for an endpoint.
func (s *Sender) BreakerState(url string) string {
	s.mu.RLock()
	breaker, exists := s.breakers[url]
	s.mu.RUnlock()

	if !exists {
		return "unknown"
	}

	return breaker.State().String()
}
