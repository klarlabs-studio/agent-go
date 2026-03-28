package approvalwebhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError error
	}{
		{
			name: "valid config",
			config: Config{
				WebhookURL: "https://example.com/approval",
			},
			expectError: nil,
		},
		{
			name:        "missing webhook URL",
			config:      Config{},
			expectError: ErrMissingWebhookURL,
		},
		{
			name: "defaults applied",
			config: Config{
				WebhookURL: "https://example.com/approval",
			},
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			approver, err := New(tt.config)

			if tt.expectError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectError, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, approver)
				require.Equal(t, 5*time.Minute, approver.config.Timeout)
				require.Equal(t, "/approval/callback", approver.config.CallbackPath)
			}
		})
	}
}

func TestSendWebhookRequest(t *testing.T) {
	tests := []struct {
		name            string
		secret          string
		statusCode      int
		expectError     bool
		verifySignature bool
	}{
		{
			name:       "successful POST without signature",
			statusCode: http.StatusOK,
		},
		{
			name:            "successful POST with signature",
			secret:          "test-secret",
			statusCode:      http.StatusOK,
			verifySignature: true,
		},
		{
			name:        "webhook returns error status",
			statusCode:  http.StatusInternalServerError,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody []byte
			var receivedSig string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "application/json", r.Header.Get("Content-Type"))

				receivedSig = r.Header.Get("X-Signature")

				var err error
				receivedBody, err = io.ReadAll(r.Body)
				require.NoError(t, err)

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			approver, err := New(Config{
				WebhookURL: server.URL,
				Secret:     tt.secret,
			})
			require.NoError(t, err)

			req := policy.ApprovalRequest{
				RunID:     "run-1",
				ToolName:  "delete_file",
				Reason:    "destructive action",
				RiskLevel: "high",
				Input:     json.RawMessage(`{"path":"/tmp/test"}`),
				Timestamp: time.Now(),
			}

			err = approver.sendWebhookRequest(context.Background(), req)

			if tt.expectError {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrWebhookFailed)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, receivedBody)

				// Verify the body is a valid ApprovalRequest
				var parsed policy.ApprovalRequest
				require.NoError(t, json.Unmarshal(receivedBody, &parsed))
				require.Equal(t, "run-1", parsed.RunID)
				require.Equal(t, "delete_file", parsed.ToolName)
			}

			if tt.verifySignature {
				require.NotEmpty(t, receivedSig)
				require.True(t, verifyHMAC(receivedBody, receivedSig, tt.secret))
			}
		})
	}
}

func TestCallbackHandler(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		body             string
		secret           string
		signature        string
		setupPending     string
		expectStatusCode int
		expectApproved   bool
	}{
		{
			name:             "invalid method",
			method:           http.MethodGet,
			expectStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:             "invalid JSON body",
			method:           http.MethodPost,
			body:             "not json",
			expectStatusCode: http.StatusBadRequest,
		},
		{
			name:             "missing run_id",
			method:           http.MethodPost,
			body:             `{"approved":true}`,
			expectStatusCode: http.StatusBadRequest,
		},
		{
			name:             "unknown run ID",
			method:           http.MethodPost,
			body:             `{"run_id":"unknown","approved":true}`,
			expectStatusCode: http.StatusNotFound,
		},
		{
			name:             "successful approval",
			method:           http.MethodPost,
			body:             `{"run_id":"run-1","approved":true,"approver":"admin","reason":"looks good"}`,
			setupPending:     "run-1",
			expectStatusCode: http.StatusOK,
			expectApproved:   true,
		},
		{
			name:             "successful denial",
			method:           http.MethodPost,
			body:             `{"run_id":"run-2","approved":false,"reason":"too risky"}`,
			setupPending:     "run-2",
			expectStatusCode: http.StatusOK,
			expectApproved:   false,
		},
		{
			name:             "valid signature",
			method:           http.MethodPost,
			body:             `{"run_id":"run-3","approved":true}`,
			secret:           "test-secret",
			signature:        "", // will be computed
			setupPending:     "run-3",
			expectStatusCode: http.StatusOK,
			expectApproved:   true,
		},
		{
			name:             "invalid signature",
			method:           http.MethodPost,
			body:             `{"run_id":"run-4","approved":true}`,
			secret:           "test-secret",
			signature:        "bad-signature",
			setupPending:     "run-4",
			expectStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			approver, err := New(Config{
				WebhookURL: "https://example.com/webhook",
				Secret:     tt.secret,
			})
			require.NoError(t, err)

			// Setup pending approval if specified
			var resultChan chan callbackResult
			if tt.setupPending != "" {
				resultChan = make(chan callbackResult, 1)
				approver.pending[tt.setupPending] = &pendingApproval{
					response: resultChan,
				}
			}

			// Compute valid signature if needed
			signature := tt.signature
			if tt.secret != "" && signature == "" {
				signature = computeHMAC([]byte(tt.body), tt.secret)
			}

			// Build request
			var bodyReader io.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, "/approval/callback", bodyReader)
			if tt.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			if tt.secret != "" {
				req.Header.Set("X-Signature", signature)
			}

			w := httptest.NewRecorder()
			approver.CallbackHandler().ServeHTTP(w, req)

			require.Equal(t, tt.expectStatusCode, w.Code)

			// Verify result if pending was set up and callback succeeded
			if tt.setupPending != "" && tt.expectStatusCode == http.StatusOK {
				select {
				case result := <-resultChan:
					require.Equal(t, tt.expectApproved, result.approved)
				case <-time.After(100 * time.Millisecond):
					t.Fatal("timeout waiting for callback result")
				}
			}
		})
	}
}

func TestApproveIntegration_Approved(t *testing.T) {
	// Webhook server that immediately calls back with approval
	approver, err := New(Config{
		WebhookURL: "https://placeholder.test", // will be overridden
		Timeout:    2 * time.Second,
	})
	require.NoError(t, err)

	// Create the callback server
	callbackServer := httptest.NewServer(approver.CallbackHandler())
	defer callbackServer.Close()

	// Mock webhook endpoint that POSTs back to the callback
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the incoming approval request
		var req policy.ApprovalRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)

		// Simulate external approval by calling back
		go func() {
			callbackBody, _ := json.Marshal(callbackPayload{
				RunID:    req.RunID,
				Approved: true,
				Approver: "webhook-admin",
				Reason:   "approved via test",
			})
			resp, err := http.Post(callbackServer.URL, "application/json", strings.NewReader(string(callbackBody)))
			if err == nil {
				resp.Body.Close()
			}
		}()
	}))
	defer webhookServer.Close()

	// Override the webhook URL
	approver.config.WebhookURL = webhookServer.URL

	ctx := context.Background()
	req := policy.ApprovalRequest{
		RunID:     "run-integration",
		ToolName:  "delete_file",
		Reason:    "integration test",
		RiskLevel: "high",
		Input:     json.RawMessage(`{"path":"/tmp/test"}`),
		Timestamp: time.Now(),
	}

	resp, err := approver.Approve(ctx, req)
	require.NoError(t, err)
	require.True(t, resp.Approved)
	require.Equal(t, "webhook-admin", resp.Approver)
	require.Equal(t, "approved via test", resp.Reason)
}

func TestApproveIntegration_Timeout(t *testing.T) {
	// Webhook server that never calls back
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	approver, err := New(Config{
		WebhookURL: webhookServer.URL,
		Timeout:    100 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := policy.ApprovalRequest{
		RunID:     "run-timeout",
		ToolName:  "delete_file",
		Reason:    "timeout test",
		RiskLevel: "high",
		Timestamp: time.Now(),
	}

	resp, err := approver.Approve(ctx, req)
	require.Error(t, err)
	require.Equal(t, ErrTimeout, err)
	require.False(t, resp.Approved)
	require.Equal(t, "approval timed out", resp.Reason)
}

func TestApproveIntegration_ContextCancelled(t *testing.T) {
	// Webhook server that never calls back
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	approver, err := New(Config{
		WebhookURL: webhookServer.URL,
		Timeout:    10 * time.Second,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	req := policy.ApprovalRequest{
		RunID:     "run-cancel",
		ToolName:  "delete_file",
		Reason:    "cancel test",
		RiskLevel: "high",
		Timestamp: time.Now(),
	}

	_, err = approver.Approve(ctx, req)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestComputeAndVerifyHMAC(t *testing.T) {
	secret := "test-secret"
	data := []byte("test-data")

	sig := computeHMAC(data, secret)
	require.NotEmpty(t, sig)

	// Valid verification
	require.True(t, verifyHMAC(data, sig, secret))

	// Invalid signature
	require.False(t, verifyHMAC(data, "invalid", secret))

	// Wrong secret
	require.False(t, verifyHMAC(data, sig, "wrong-secret"))

	// Wrong data
	require.False(t, verifyHMAC([]byte("different"), sig, secret))
}
