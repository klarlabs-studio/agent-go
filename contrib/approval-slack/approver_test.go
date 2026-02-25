package approvalslack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/stretchr/testify/require"
)

func TestSendApprovalMessage(t *testing.T) {
	tests := []struct {
		name           string
		request        policy.ApprovalRequest
		mockResponse   map[string]any
		mockStatusCode int
		expectError    bool
		expectTS       string
	}{
		{
			name: "successful approval request",
			request: policy.ApprovalRequest{
				RunID:     "run-1",
				ToolName:  "delete_file",
				Reason:    "destructive action",
				RiskLevel: "high",
				Input:     []byte(`{"path":"/tmp/test.txt"}`),
				Timestamp: time.Now(),
			},
			mockResponse: map[string]any{
				"ok": true,
				"ts": "1234567890.123456",
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectTS:       "1234567890.123456",
		},
		{
			name: "slack api error",
			request: policy.ApprovalRequest{
				RunID:    "run-2",
				ToolName: "delete_file",
			},
			mockResponse: map[string]any{
				"ok":    false,
				"error": "channel_not_found",
			},
			mockStatusCode: http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock Slack API
			slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/chat.postMessage", r.URL.Path)
				require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				require.Equal(t, "application/json; charset=utf-8", r.Header.Get("Content-Type"))

				// Verify the request body
				var msg slackMessage
				err := json.NewDecoder(r.Body).Decode(&msg)
				require.NoError(t, err)
				require.Equal(t, "C123", msg.Channel)
				require.Equal(t, tt.request.ToolName, msg.Text[len("Approval Required: "):])
				require.NotEmpty(t, msg.Blocks)

				w.WriteHeader(tt.mockStatusCode)
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer slackServer.Close()

			approver, err := New(Config{
				Token:     "test-token",
				ChannelID: "C123",
				BaseURL:   slackServer.URL,
			})
			require.NoError(t, err)

			ts, err := approver.sendApprovalMessage(context.Background(), tt.request)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectTS, ts)
			}
		})
	}
}

func TestUpdateMessageExpired(t *testing.T) {
	tests := []struct {
		name           string
		timestamp      string
		mockResponse   map[string]any
		mockStatusCode int
		expectError    bool
	}{
		{
			name:      "successful update",
			timestamp: "1234567890.123456",
			mockResponse: map[string]any{
				"ok": true,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:      "slack api error",
			timestamp: "1234567890.123456",
			mockResponse: map[string]any{
				"ok":    false,
				"error": "message_not_found",
			},
			mockStatusCode: http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock Slack API
			slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/chat.update", r.URL.Path)
				require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

				// Verify the request body
				var msg slackMessage
				err := json.NewDecoder(r.Body).Decode(&msg)
				require.NoError(t, err)
				require.Equal(t, "C123", msg.Channel)
				require.Equal(t, tt.timestamp, msg.TS)
				require.Equal(t, "Approval Request Expired", msg.Text)

				w.WriteHeader(tt.mockStatusCode)
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer slackServer.Close()

			approver, err := New(Config{
				Token:     "test-token",
				ChannelID: "C123",
				BaseURL:   slackServer.URL,
			})
			require.NoError(t, err)

			err = approver.updateMessageExpired(context.Background(), tt.timestamp)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	signingSecret := "test-signing-secret"

	tests := []struct {
		name        string
		timestamp   string
		body        string
		signature   string
		expectError bool
	}{
		{
			name:        "valid signature",
			timestamp:   strconv.FormatInt(time.Now().Unix(), 10),
			body:        "test-body",
			signature:   "", // will be computed
			expectError: false,
		},
		{
			name:        "invalid signature",
			timestamp:   strconv.FormatInt(time.Now().Unix(), 10),
			body:        "test-body",
			signature:   "v0=invalid",
			expectError: true,
		},
		{
			name:        "timestamp too old",
			timestamp:   strconv.FormatInt(time.Now().Unix()-400, 10),
			body:        "test-body",
			signature:   "", // will be computed
			expectError: true,
		},
		{
			name:        "invalid timestamp format",
			timestamp:   "invalid",
			body:        "test-body",
			signature:   "v0=test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			approver, err := New(Config{
				Token:         "test-token",
				ChannelID:     "C123",
				SigningSecret: signingSecret,
			})
			require.NoError(t, err)

			// Compute valid signature if not explicitly set to invalid
			signature := tt.signature
			if signature == "" && tt.name == "valid signature" {
				sigBaseString := "v0:" + tt.timestamp + ":" + tt.body
				mac := hmac.New(sha256.New, []byte(signingSecret))
				mac.Write([]byte(sigBaseString))
				signature = "v0=" + hex.EncodeToString(mac.Sum(nil))
			} else if signature == "" {
				// For other tests, compute the signature even though it may fail timestamp check
				sigBaseString := "v0:" + tt.timestamp + ":" + tt.body
				mac := hmac.New(sha256.New, []byte(signingSecret))
				mac.Write([]byte(sigBaseString))
				signature = "v0=" + hex.EncodeToString(mac.Sum(nil))
			}

			err = approver.verifySignature(tt.timestamp, tt.body, signature)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHandleInteraction(t *testing.T) {
	tests := []struct {
		name               string
		method             string
		payload            map[string]any
		withSigningSecret  bool
		validSignature     bool
		expectStatusCode   int
		setupPendingAction bool
		expectApproved     bool
	}{
		{
			name:   "invalid method",
			method: http.MethodGet,
			payload: map[string]any{
				"type": "block_actions",
				"user": map[string]string{"id": "U123", "name": "test_user"},
				"actions": []map[string]string{
					{"action_id": "approve_run-1", "value": "run-1"},
				},
			},
			expectStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:   "approve action",
			method: http.MethodPost,
			payload: map[string]any{
				"type": "block_actions",
				"user": map[string]string{"id": "U123", "name": "test_user"},
				"actions": []map[string]string{
					{"action_id": "approve_run-1", "value": "run-1"},
				},
			},
			expectStatusCode:   http.StatusOK,
			setupPendingAction: true,
			expectApproved:     true,
		},
		{
			name:   "deny action",
			method: http.MethodPost,
			payload: map[string]any{
				"type": "block_actions",
				"user": map[string]string{"id": "U123", "name": "test_user"},
				"actions": []map[string]string{
					{"action_id": "deny_run-2", "value": "run-2"},
				},
			},
			expectStatusCode:   http.StatusOK,
			setupPendingAction: true,
			expectApproved:     false,
		},
		{
			name:   "missing payload",
			method: http.MethodPost,
			payload: map[string]any{
				"type": "block_actions",
			},
			expectStatusCode: http.StatusBadRequest,
		},
		{
			name:              "valid signature",
			method:            http.MethodPost,
			withSigningSecret: true,
			validSignature:    true,
			payload: map[string]any{
				"type": "block_actions",
				"user": map[string]string{"id": "U123", "name": "test_user"},
				"actions": []map[string]string{
					{"action_id": "approve_run-3", "value": "run-3"},
				},
			},
			expectStatusCode:   http.StatusOK,
			setupPendingAction: true,
			expectApproved:     true,
		},
		{
			name:              "invalid signature",
			method:            http.MethodPost,
			withSigningSecret: true,
			validSignature:    false,
			payload: map[string]any{
				"type": "block_actions",
				"user": map[string]string{"id": "U123", "name": "test_user"},
				"actions": []map[string]string{
					{"action_id": "approve_run-4", "value": "run-4"},
				},
			},
			expectStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Token:     "test-token",
				ChannelID: "C123",
			}
			if tt.withSigningSecret {
				config.SigningSecret = "test-signing-secret"
			}

			approver, err := New(config)
			require.NoError(t, err)

			// Setup pending approval if needed
			var resultChan chan approvalResult
			if tt.setupPendingAction {
				resultChan = make(chan approvalResult, 1)
				runID := ""
				if actions, ok := tt.payload["actions"].([]map[string]string); ok && len(actions) > 0 {
					runID = actions[0]["value"]
				}
				approver.pending[runID] = &pendingApproval{
					response: resultChan,
				}
			}

			// Create the request
			payloadJSON, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			// Prepare form body
			formBody := ""
			if tt.name != "missing payload" {
				formBody = url.Values{"payload": []string{string(payloadJSON)}}.Encode()
			}

			req := httptest.NewRequest(tt.method, "/slack/interaction", nil)
			if tt.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Body = io.NopCloser(bytes.NewReader([]byte(formBody)))
			}

			// Add signature headers if needed
			if tt.withSigningSecret {
				timestamp := strconv.FormatInt(time.Now().Unix(), 10)
				req.Header.Set("X-Slack-Request-Timestamp", timestamp)

				if tt.validSignature {
					sigBaseString := "v0:" + timestamp + ":" + formBody
					mac := hmac.New(sha256.New, []byte(config.SigningSecret))
					mac.Write([]byte(sigBaseString))
					signature := "v0=" + hex.EncodeToString(mac.Sum(nil))
					req.Header.Set("X-Slack-Signature", signature)
				} else {
					req.Header.Set("X-Slack-Signature", "v0=invalid")
				}
			}

			// Handle the request
			w := httptest.NewRecorder()
			approver.HandleInteraction(w, req)

			// Check status code
			require.Equal(t, tt.expectStatusCode, w.Code)

			// Check result if action was processed
			if tt.setupPendingAction && tt.expectStatusCode == http.StatusOK {
				select {
				case result := <-resultChan:
					require.Equal(t, tt.expectApproved, result.approved)
					require.Contains(t, result.approver, "U123")
					require.Contains(t, result.approver, "test_user")
				case <-time.After(100 * time.Millisecond):
					t.Fatal("timeout waiting for result")
				}
			}
		})
	}
}

func TestApproveIntegration(t *testing.T) {
	// Mock Slack API
	messageSent := false
	messageUpdated := false

	slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat.postMessage" {
			messageSent = true
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"ts": "1234567890.123456",
			})
		} else if r.URL.Path == "/chat.update" {
			messageUpdated = true
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
			})
		}
	}))
	defer slackServer.Close()

	approver, err := New(Config{
		Token:     "test-token",
		ChannelID: "C123",
		BaseURL:   slackServer.URL,
		Timeout:   500 * time.Millisecond,
	})
	require.NoError(t, err)

	// Test timeout scenario
	ctx := context.Background()
	req := policy.ApprovalRequest{
		RunID:     "run-timeout",
		ToolName:  "delete_file",
		Reason:    "destructive action",
		RiskLevel: "high",
		Input:     []byte(`{"path":"/tmp/test.txt"}`),
		Timestamp: time.Now(),
	}

	resp, err := approver.Approve(ctx, req)
	require.Error(t, err)
	require.Equal(t, ErrTimeout, err)
	require.False(t, resp.Approved)
	require.True(t, messageSent)
	require.True(t, messageUpdated)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError error
	}{
		{
			name: "valid config",
			config: Config{
				Token:     "test-token",
				ChannelID: "C123",
			},
			expectError: nil,
		},
		{
			name: "missing token",
			config: Config{
				ChannelID: "C123",
			},
			expectError: ErrMissingToken,
		},
		{
			name: "missing channel",
			config: Config{
				Token: "test-token",
			},
			expectError: ErrMissingChannel,
		},
		{
			name: "default timeout set",
			config: Config{
				Token:     "test-token",
				ChannelID: "C123",
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
				require.Equal(t, "https://slack.com/api", approver.config.BaseURL)
			}
		})
	}
}

func TestBuildMessageBlocks(t *testing.T) {
	approver, err := New(Config{
		Token:     "test-token",
		ChannelID: "C123",
	})
	require.NoError(t, err)

	req := policy.ApprovalRequest{
		RunID:     "run-1",
		ToolName:  "delete_file",
		Reason:    "destructive action",
		RiskLevel: "high",
		Input:     []byte(`{"path":"/tmp/test.txt"}`),
		Timestamp: time.Now(),
	}

	blocks := approver.buildMessageBlocks(req)

	require.NotEmpty(t, blocks)
	require.Equal(t, "header", blocks[0].Type)
	require.Equal(t, "Approval Required", blocks[0].Text.Text)

	// Check for tool name and risk level
	foundToolInfo := false
	for _, block := range blocks {
		if block.Type == "section" && len(block.Fields) > 0 {
			for _, field := range block.Fields {
				if field.Type == "mrkdwn" && field.Text == "*Tool:*\ndelete_file" {
					foundToolInfo = true
				}
			}
		}
	}
	require.True(t, foundToolInfo, "Tool information not found in blocks")

	// Check for action buttons
	foundActions := false
	for _, block := range blocks {
		if block.Type == "actions" {
			require.Len(t, block.Elements, 2)
			require.Equal(t, "button", block.Elements[0].Type)
			require.Equal(t, "Approve", block.Elements[0].Text.Text)
			require.Equal(t, "button", block.Elements[1].Type)
			require.Equal(t, "Deny", block.Elements[1].Text.Text)
			foundActions = true
		}
	}
	require.True(t, foundActions, "Action buttons not found in blocks")
}

func TestProcessAction(t *testing.T) {
	approver, err := New(Config{
		Token:     "test-token",
		ChannelID: "C123",
	})
	require.NoError(t, err)

	tests := []struct {
		name           string
		action         slackAction
		user           slackUser
		expectApproved bool
		expectReason   string
	}{
		{
			name: "approve action",
			action: slackAction{
				ActionID: "approve_run-1",
				Value:    "run-1",
			},
			user: slackUser{
				ID:   "U123",
				Name: "test_user",
			},
			expectApproved: true,
			expectReason:   "approved",
		},
		{
			name: "deny action",
			action: slackAction{
				ActionID: "deny_run-2",
				Value:    "run-2",
			},
			user: slackUser{
				ID:   "U456",
				Name: "another_user",
			},
			expectApproved: false,
			expectReason:   "denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultChan := make(chan approvalResult, 1)
			approver.pending[tt.action.Value] = &pendingApproval{
				response: resultChan,
			}

			approver.processAction(tt.action, tt.user)

			select {
			case result := <-resultChan:
				require.Equal(t, tt.expectApproved, result.approved)
				require.Equal(t, tt.expectReason, result.reason)
				require.Contains(t, result.approver, tt.user.ID)
				require.Contains(t, result.approver, tt.user.Name)
			case <-time.After(100 * time.Millisecond):
				t.Fatal("timeout waiting for result")
			}
		})
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    int64
		expected int64
	}{
		{input: 5, expected: 5},
		{input: -5, expected: 5},
		{input: 0, expected: 0},
		{input: 123456789, expected: 123456789},
		{input: -123456789, expected: 123456789},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("abs(%d)", tt.input), func(t *testing.T) {
			result := abs(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
