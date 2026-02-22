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
	"context"
	"encoding/json"
	"errors"
	"net/http"
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

	// TODO: Implement actual Slack API call
	_ = ctx
	_ = msg

	return "placeholder-timestamp", nil
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
func (a *Approver) updateMessageExpired(_ context.Context, _ string) error {
	// TODO: Implement message update
	return nil
}

// HandleInteraction processes Slack interactive component callbacks.
// This should be mounted at the Interactive Components Request URL.
func (a *Approver) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Verify request signature using signing secret

	// Parse the payload
	payload := r.FormValue("payload")
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
