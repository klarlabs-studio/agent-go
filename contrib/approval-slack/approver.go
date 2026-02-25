// Package approvalslack provides Slack-based approval integration for agent-go.
//
// This package implements the policy.Approver interface to enable human approval
// of agent actions via Slack. When an agent requests approval for a destructive
// or high-risk action, a Slack message is sent to a configured channel with
// approve/deny buttons.
//
// # Usage
//
//	approver := approvalslack.New(approvalslack.Config{
//		Token:     os.Getenv("SLACK_BOT_TOKEN"),
//		ChannelID: "C0123456789",
//		Timeout:   5 * time.Minute,
//	})
//
//	engine, err := api.New(
//		api.WithApprover(approver),
//	)
//
// # Slack App Setup
//
// To use this integration, you need a Slack App with:
//   - Bot Token Scopes: chat:write, reactions:write
//   - Interactive Components enabled with a Request URL
//   - Event Subscriptions (optional) for real-time responses
//
// The Request URL should point to the HandleInteraction endpoint.
package approvalslack

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
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/policy"
)

// Common errors for Slack approval operations.
var (
	ErrMissingToken   = errors.New("missing Slack token")
	ErrMissingChannel = errors.New("missing channel ID")
	ErrTimeout        = errors.New("approval timed out")
	ErrDenied         = errors.New("approval denied")
	ErrSlackAPIError  = errors.New("Slack API error")
)

// Config configures the Slack approver.
type Config struct {
	// Token is the Slack Bot User OAuth Token.
	Token string

	// ChannelID is the default channel for approval requests.
	ChannelID string

	// Timeout is how long to wait for approval.
	Timeout time.Duration

	// MentionUsers is a list of user IDs to mention in approval requests.
	MentionUsers []string

	// MentionGroups is a list of user group IDs to mention.
	MentionGroups []string

	// SigningSecret is used to verify Slack request signatures.
	SigningSecret string

	// BaseURL overrides the Slack API URL (for testing).
	BaseURL string
}

// Approver implements policy.Approver via Slack.
type Approver struct {
	config   Config
	pending  map[string]*pendingApproval
	mu       sync.Mutex
	client   *http.Client
}

// pendingApproval tracks an in-flight approval request.
type pendingApproval struct {
	request  policy.ApprovalRequest
	response chan approvalResult
	ts       string // Slack message timestamp
}

// approvalResult is the internal result of an approval.
type approvalResult struct {
	approved bool
	approver string
	reason   string
}

// New creates a new Slack approver.
func New(cfg Config) (*Approver, error) {
	if cfg.Token == "" {
		return nil, ErrMissingToken
	}
	if cfg.ChannelID == "" {
		return nil, ErrMissingChannel
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://slack.com/api"
	}

	return &Approver{
		config:  cfg,
		pending: make(map[string]*pendingApproval),
		client:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Approve sends an approval request to Slack and waits for a response.
// This implements the policy.Approver interface.
func (a *Approver) Approve(ctx context.Context, req policy.ApprovalRequest) (policy.ApprovalResponse, error) {
	// Create response channel
	respChan := make(chan approvalResult, 1)

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

	// Send the Slack message
	ts, err := a.sendApprovalMessage(ctx, req)
	if err != nil {
		return policy.ApprovalResponse{}, err
	}

	// Update with message timestamp
	a.mu.Lock()
	if p, ok := a.pending[req.RunID]; ok {
		p.ts = ts
	}
	a.mu.Unlock()

	// Wait for response with timeout
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
		_ = a.updateMessageExpired(ctx, ts)
		return policy.ApprovalResponse{
			Approved:  false,
			Reason:    "approval timed out",
			Timestamp: time.Now(),
		}, ErrTimeout
	case <-ctx.Done():
		return policy.ApprovalResponse{}, ctx.Err()
	}
}

// sendApprovalMessage sends the approval request to Slack.
func (a *Approver) sendApprovalMessage(ctx context.Context, req policy.ApprovalRequest) (string, error) {
	// Build the message blocks
	blocks := a.buildMessageBlocks(req)

	msg := slackMessage{
		Channel: a.config.ChannelID,
		Text:    "Approval Required: " + req.ToolName,
		Blocks:  blocks,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal message: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.BaseURL+"/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpReq.Header.Set("Authorization", "Bearer "+a.config.Token)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return "", errors.Join(ErrSlackAPIError, err)
	}
	defer resp.Body.Close()

	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
		TS    string `json:"ts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&slackResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if !slackResp.OK {
		return "", fmt.Errorf("%w: %s", ErrSlackAPIError, slackResp.Error)
	}

	return slackResp.TS, nil
}

// buildMessageBlocks creates Slack block kit blocks for the approval message.
func (a *Approver) buildMessageBlocks(req policy.ApprovalRequest) []slackBlock {
	inputJSON, err := json.MarshalIndent(json.RawMessage(req.Input), "", "  ")
	if err != nil {
		inputJSON = []byte(req.Input)
	}

	return []slackBlock{
		{
			Type: "header",
			Text: &slackText{
				Type: "plain_text",
				Text: "Approval Required",
			},
		},
		{
			Type: "section",
			Fields: []slackText{
				{Type: "mrkdwn", Text: "*Tool:*\n" + req.ToolName},
				{Type: "mrkdwn", Text: "*Risk Level:*\n" + req.RiskLevel},
			},
		},
		{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: "*Reason:*\n" + req.Reason,
			},
		},
		{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: "*Input:*\n```" + string(inputJSON) + "```",
			},
		},
		{
			Type: "actions",
			Elements: []slackElement{
				{
					Type:     "button",
					Text:     slackText{Type: "plain_text", Text: "Approve"},
					Style:    "primary",
					ActionID: "approve_" + req.RunID,
					Value:    req.RunID,
				},
				{
					Type:     "button",
					Text:     slackText{Type: "plain_text", Text: "Deny"},
					Style:    "danger",
					ActionID: "deny_" + req.RunID,
					Value:    req.RunID,
				},
			},
		},
		{
			Type: "context",
			Elements: []slackElement{
				{
					Type:      "mrkdwn",
					PlainText: "Run ID: " + req.RunID + " | Requested at: " + req.Timestamp.Format(time.RFC822),
				},
			},
		},
	}
}

// updateMessageExpired updates the message to show it expired.
func (a *Approver) updateMessageExpired(ctx context.Context, ts string) error {
	msg := slackMessage{
		Channel: a.config.ChannelID,
		TS:      ts,
		Text:    "Approval Request Expired",
		Blocks: []slackBlock{
			{
				Type: "section",
				Text: &slackText{
					Type: "mrkdwn",
					Text: ":hourglass: *Approval Request Expired*\nThis request was not responded to in time.",
				},
			},
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.BaseURL+"/chat.update", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpReq.Header.Set("Authorization", "Bearer "+a.config.Token)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return errors.Join(ErrSlackAPIError, err)
	}
	defer resp.Body.Close()

	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&slackResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !slackResp.OK {
		return fmt.Errorf("%w: %s", ErrSlackAPIError, slackResp.Error)
	}

	return nil
}

// HandleInteraction processes Slack interactive component callbacks.
// This should be mounted at the Interactive Components Request URL.
func (a *Approver) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body for signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature if signing secret is configured
	if a.config.SigningSecret != "" {
		timestamp := r.Header.Get("X-Slack-Request-Timestamp")
		signature := r.Header.Get("X-Slack-Signature")

		if err := a.verifySignature(timestamp, string(body), signature); err != nil {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse the payload from form data
	// The body is form-encoded: payload=...
	values, err := url.ParseQuery(string(body))
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	payload := values.Get("payload")
	if payload == "" {
		http.Error(w, "Missing payload", http.StatusBadRequest)
		return
	}

	var interaction slackInteraction
	if err := json.Unmarshal([]byte(payload), &interaction); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Process the action
	for _, action := range interaction.Actions {
		a.processAction(action, interaction.User)
	}

	w.WriteHeader(http.StatusOK)
}

// verifySignature verifies the Slack request signature.
func (a *Approver) verifySignature(timestamp, body, expectedSig string) error {
	// Check timestamp freshness (5 min window)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	if abs(time.Now().Unix()-ts) > 300 {
		return fmt.Errorf("request too old")
	}

	// Compute HMAC-SHA256
	sigBaseString := "v0:" + timestamp + ":" + body
	mac := hmac.New(sha256.New, []byte(a.config.SigningSecret))
	mac.Write([]byte(sigBaseString))
	computed := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(computed), []byte(expectedSig)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// abs returns the absolute value of n.
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// processAction handles an approval or denial action.
func (a *Approver) processAction(action slackAction, user slackUser) {
	runID := action.Value
	approved := action.ActionID[:7] == "approve"

	a.mu.Lock()
	pending, ok := a.pending[runID]
	a.mu.Unlock()

	if !ok {
		return // Already handled or expired
	}

	reason := "approved"
	if !approved {
		reason = "denied"
	}

	pending.response <- approvalResult{
		approved: approved,
		approver: user.ID + " (" + user.Name + ")",
		reason:   reason,
	}
}

// Slack API types

type slackMessage struct {
	Channel string       `json:"channel"`
	Text    string       `json:"text"`
	Blocks  []slackBlock `json:"blocks"`
	TS      string       `json:"ts,omitempty"`
}

type slackBlock struct {
	Type     string         `json:"type"`
	Text     *slackText     `json:"text,omitempty"`
	Fields   []slackText    `json:"fields,omitempty"`
	Elements []slackElement `json:"elements,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackElement struct {
	Type      string    `json:"type"`
	Text      slackText `json:"text,omitempty"`
	Style     string    `json:"style,omitempty"`
	ActionID  string    `json:"action_id,omitempty"`
	Value     string    `json:"value,omitempty"`
	PlainText string    `json:"plain_text,omitempty"` // For context elements
}

type slackInteraction struct {
	Type    string        `json:"type"`
	User    slackUser     `json:"user"`
	Actions []slackAction `json:"actions"`
}

type slackUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type slackAction struct {
	ActionID string `json:"action_id"`
	Value    string `json:"value"`
}

// Ensure Approver implements policy.Approver.
var _ policy.Approver = (*Approver)(nil)
