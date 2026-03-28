// Package approvalwebhook provides HTTP webhook-based approval integration for agent-go.
//
// This package implements the policy.Approver interface to enable human approval
// of agent actions via HTTP webhooks. When an agent requests approval for a destructive
// or high-risk action, an HTTP POST is sent to the configured webhook URL. The approver
// then waits for a callback on a local HTTP handler to receive the decision.
//
// # Usage
//
//	approver, err := approvalwebhook.New(approvalwebhook.Config{
//		WebhookURL:   "https://example.com/approval",
//		CallbackPath: "/approval/callback",
//		Timeout:      5 * time.Minute,
//		Secret:       "my-hmac-secret",
//	})
//
//	engine, err := api.New(
//		api.WithApprover(approver),
//	)
//
//	// Mount the callback handler on your HTTP server:
//	http.Handle("/approval/callback", approver.CallbackHandler())
//
// # Webhook Protocol
//
// The approver POSTs an ApprovalRequest JSON body to WebhookURL. The external service
// should respond with an HTTP POST to the callback path containing an ApprovalResponse
// JSON body, signed with HMAC-SHA256 using the shared secret.
//
// The callback request must include:
//   - Header X-Signature: HMAC-SHA256 hex digest of the request body
//   - Body: JSON-encoded policy.ApprovalResponse with run_id matching the request
package approvalwebhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/policy"
)

// Common errors for webhook approval operations.
var (
	ErrMissingWebhookURL = errors.New("missing webhook URL")
	ErrTimeout           = errors.New("approval timed out")
	ErrInvalidSignature  = errors.New("invalid HMAC signature")
	ErrWebhookFailed     = errors.New("webhook request failed")
	ErrUnknownRunID      = errors.New("unknown run ID in callback")
)

// Config configures the webhook approver.
type Config struct {
	// WebhookURL is the URL to POST approval requests to.
	WebhookURL string

	// CallbackPath is the HTTP path for receiving approval callbacks.
	// Defaults to "/approval/callback".
	CallbackPath string

	// Timeout is how long to wait for a callback response.
	// Defaults to 5 minutes.
	Timeout time.Duration

	// Secret is the shared HMAC-SHA256 secret for verifying callback signatures.
	// When empty, signature verification is skipped.
	Secret string

	// HTTPClient overrides the default HTTP client (for testing).
	HTTPClient *http.Client
}

// Approver implements policy.Approver via HTTP webhooks.
type Approver struct {
	config  Config
	pending map[string]*pendingApproval
	mu      sync.Mutex
	client  *http.Client
}

// pendingApproval tracks an in-flight approval request.
type pendingApproval struct {
	request  policy.ApprovalRequest
	response chan callbackResult
}

// callbackResult is the internal result of a callback.
type callbackResult struct {
	approved bool
	approver string
	reason   string
}

// New creates a new webhook approver.
func New(cfg Config) (*Approver, error) {
	if cfg.WebhookURL == "" {
		return nil, ErrMissingWebhookURL
	}
	if cfg.CallbackPath == "" {
		cfg.CallbackPath = "/approval/callback"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	return &Approver{
		config:  cfg,
		pending: make(map[string]*pendingApproval),
		client:  client,
	}, nil
}

// Approve sends an approval request via webhook and waits for a callback.
// This implements the policy.Approver interface.
func (a *Approver) Approve(ctx context.Context, req policy.ApprovalRequest) (policy.ApprovalResponse, error) {
	// Create response channel
	respChan := make(chan callbackResult, 1)

	// Track the pending approval
	a.mu.Lock()
	a.pending[req.RunID] = &pendingApproval{
		request:  req,
		response: respChan,
	}
	a.mu.Unlock()

	// Clean up when done
	defer func() {
		a.mu.Lock()
		delete(a.pending, req.RunID)
		a.mu.Unlock()
	}()

	// POST the approval request to the webhook URL
	if err := a.sendWebhookRequest(ctx, req); err != nil {
		return policy.ApprovalResponse{}, err
	}

	// Wait for callback response with timeout
	timeout := a.config.Timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < timeout {
			timeout = remaining
		}
	}

	select {
	case result := <-respChan:
		return policy.ApprovalResponse{
			Approved:  result.approved,
			Approver:  result.approver,
			Reason:    result.reason,
			Timestamp: time.Now(),
		}, nil
	case <-time.After(timeout):
		return policy.ApprovalResponse{
			Approved:  false,
			Reason:    "approval timed out",
			Timestamp: time.Now(),
		}, ErrTimeout
	case <-ctx.Done():
		return policy.ApprovalResponse{}, ctx.Err()
	}
}

// sendWebhookRequest POSTs the approval request to the configured webhook URL.
func (a *Approver) sendWebhookRequest(ctx context.Context, req policy.ApprovalRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal approval request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Sign the request if a secret is configured
	if a.config.Secret != "" {
		sig := computeHMAC(body, a.config.Secret)
		httpReq.Header.Set("X-Signature", sig)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrWebhookFailed, err)
	}
	defer resp.Body.Close()

	// Discard body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrWebhookFailed, resp.StatusCode)
	}

	return nil
}

// CallbackHandler returns an http.Handler that processes approval callbacks.
// Mount this handler at the configured CallbackPath on your HTTP server.
func (a *Approver) CallbackHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		// Verify HMAC signature if secret is configured
		if a.config.Secret != "" {
			sig := r.Header.Get("X-Signature")
			if !verifyHMAC(body, sig, a.config.Secret) {
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// Parse the callback response
		var callbackResp callbackPayload
		if err := json.Unmarshal(body, &callbackResp); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		if callbackResp.RunID == "" {
			http.Error(w, "Missing run_id", http.StatusBadRequest)
			return
		}

		// Look up the pending approval
		a.mu.Lock()
		pending, ok := a.pending[callbackResp.RunID]
		a.mu.Unlock()

		if !ok {
			http.Error(w, "Unknown run ID", http.StatusNotFound)
			return
		}

		// Deliver the result
		pending.response <- callbackResult{
			approved: callbackResp.Approved,
			approver: callbackResp.Approver,
			reason:   callbackResp.Reason,
		}

		w.WriteHeader(http.StatusOK)
	})
}

// callbackPayload is the expected JSON body of a callback request.
type callbackPayload struct {
	RunID    string `json:"run_id"`
	Approved bool   `json:"approved"`
	Approver string `json:"approver,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// computeHMAC computes the HMAC-SHA256 hex digest of data using the given secret.
func computeHMAC(data []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyHMAC verifies the HMAC-SHA256 hex digest of data.
func verifyHMAC(data []byte, signature, secret string) bool {
	expected := computeHMAC(data, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// Ensure Approver implements policy.Approver.
var _ policy.Approver = (*Approver)(nil)
