// Package notification provides notification tools for agent-go.
//
// This pack includes tools for sending notifications:
//   - notify_slack: Send a Slack message
//   - notify_discord: Send a Discord message
//   - notify_teams: Send a Microsoft Teams message
//   - notify_webhook: Send a generic webhook notification
//   - notify_sms: Send an SMS message
//   - notify_push: Send a push notification
//   - notify_pagerduty: Create a PagerDuty incident
//
// Supports templating and variable substitution in messages.
package notification

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// NotificationConfig holds configuration for notification tools.
type NotificationConfig struct {
	// SlackWebhookURL is the Slack incoming webhook URL.
	SlackWebhookURL string

	// DiscordWebhookURL is the Discord webhook URL.
	DiscordWebhookURL string

	// TeamsWebhookURL is the Microsoft Teams webhook URL.
	TeamsWebhookURL string

	// TwilioAccountSID is the Twilio account SID for SMS.
	TwilioAccountSID string
	// TwilioAuthToken is the Twilio auth token.
	TwilioAuthToken string
	// TwilioFromNumber is the Twilio sender phone number.
	TwilioFromNumber string

	// PushoverToken is the Pushover API token.
	PushoverToken string
	// PushoverUserKey is the Pushover user key.
	PushoverUserKey string

	// PagerDutyRoutingKey is the PagerDuty Events API v2 routing key.
	PagerDutyRoutingKey string

	// HTTPClient is the HTTP client to use (default: http.DefaultClient with 30s timeout).
	HTTPClient *http.Client
}

// Input schema types
type slackInput struct {
	Channel string `json:"channel,omitempty"`
	Text    string `json:"text"`
}

type discordInput struct {
	Content string `json:"content"`
}

type teamsInput struct {
	Title string `json:"title,omitempty"`
	Text  string `json:"text"`
}

type webhookInput struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
}

type smsInput struct {
	To   string `json:"to"`
	Body string `json:"body"`
}

type pushInput struct {
	Title   string `json:"title,omitempty"`
	Message string `json:"message"`
}

type pagerdutyInput struct {
	Summary  string `json:"summary"`
	Severity string `json:"severity"`
	Source   string `json:"source,omitempty"`
}

// Pack returns the notification tools pack.
func Pack(cfg NotificationConfig) *pack.Pack {
	// Ensure we have an HTTP client with reasonable timeout
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return pack.NewBuilder("notification").
		WithDescription("Notification tools for alerts and messaging").
		WithVersion("0.1.0").
		AddTools(
			notifySlack(cfg),
			notifyDiscord(cfg),
			notifyTeams(cfg),
			notifyWebhook(cfg),
			notifySMS(cfg),
			notifyPush(cfg),
			notifyPagerDuty(cfg),
		).
		AllowInState(agent.StateAct, "notify_slack", "notify_discord", "notify_teams", "notify_webhook", "notify_sms", "notify_push", "notify_pagerduty").
		Build()
}

func notifySlack(cfg NotificationConfig) tool.Tool {
	return tool.NewBuilder("notify_slack").
		WithDescription("Send a message to a Slack channel").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"text": {
					"type": "string",
					"description": "Message text to send"
				},
				"channel": {
					"type": "string",
					"description": "Optional channel override (e.g., #general)"
				}
			},
			"required": ["text"]
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in slackInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if cfg.SlackWebhookURL == "" {
				return tool.Result{}, fmt.Errorf("slack webhook URL not configured")
			}

			payload := map[string]string{"text": in.Text}
			if in.Channel != "" {
				payload["channel"] = in.Channel
			}

			body, _ := json.Marshal(payload)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.SlackWebhookURL, bytes.NewReader(body))
			if err != nil {
				return tool.Result{}, err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := cfg.HTTPClient.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				respBody, _ := io.ReadAll(resp.Body)
				return tool.Result{}, fmt.Errorf("slack returned %d: %s", resp.StatusCode, string(respBody))
			}

			result, _ := json.Marshal(map[string]string{"status": "sent"})
			return tool.Result{Output: result}, nil
		}).
		MustBuild()
}

func notifyDiscord(cfg NotificationConfig) tool.Tool {
	return tool.NewBuilder("notify_discord").
		WithDescription("Send a message to a Discord channel").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"content": {
					"type": "string",
					"description": "Message content to send"
				}
			},
			"required": ["content"]
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in discordInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if cfg.DiscordWebhookURL == "" {
				return tool.Result{}, fmt.Errorf("discord webhook URL not configured")
			}

			payload := map[string]string{"content": in.Content}
			body, _ := json.Marshal(payload)

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.DiscordWebhookURL, bytes.NewReader(body))
			if err != nil {
				return tool.Result{}, err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := cfg.HTTPClient.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
				respBody, _ := io.ReadAll(resp.Body)
				return tool.Result{}, fmt.Errorf("discord returned %d: %s", resp.StatusCode, string(respBody))
			}

			result, _ := json.Marshal(map[string]string{"status": "sent"})
			return tool.Result{Output: result}, nil
		}).
		MustBuild()
}

func notifyTeams(cfg NotificationConfig) tool.Tool {
	return tool.NewBuilder("notify_teams").
		WithDescription("Send a message to a Microsoft Teams channel").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"text": {
					"type": "string",
					"description": "Message text to send"
				},
				"title": {
					"type": "string",
					"description": "Optional message title"
				}
			},
			"required": ["text"]
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in teamsInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if cfg.TeamsWebhookURL == "" {
				return tool.Result{}, fmt.Errorf("teams webhook URL not configured")
			}

			// Create Adaptive Card payload for Teams
			payload := map[string]any{
				"@type": "MessageCard",
				"text":  in.Text,
			}
			if in.Title != "" {
				payload["title"] = in.Title
			}

			body, _ := json.Marshal(payload)

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TeamsWebhookURL, bytes.NewReader(body))
			if err != nil {
				return tool.Result{}, err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := cfg.HTTPClient.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				respBody, _ := io.ReadAll(resp.Body)
				return tool.Result{}, fmt.Errorf("teams returned %d: %s", resp.StatusCode, string(respBody))
			}

			result, _ := json.Marshal(map[string]string{"status": "sent"})
			return tool.Result{Output: result}, nil
		}).
		MustBuild()
}

func notifyWebhook(cfg NotificationConfig) tool.Tool {
	return tool.NewBuilder("notify_webhook").
		WithDescription("Send a notification via generic webhook").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "Webhook URL"
				},
				"method": {
					"type": "string",
					"description": "HTTP method (default: POST)",
					"enum": ["GET", "POST", "PUT", "PATCH"]
				},
				"headers": {
					"type": "object",
					"description": "Optional HTTP headers",
					"additionalProperties": {
						"type": "string"
					}
				},
				"body": {
					"type": "string",
					"description": "Request body"
				}
			},
			"required": ["url", "body"]
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in webhookInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			method := in.Method
			if method == "" {
				method = http.MethodPost
			}

			req, err := http.NewRequestWithContext(ctx, method, in.URL, strings.NewReader(in.Body))
			if err != nil {
				return tool.Result{}, err
			}

			// Set default Content-Type if not provided
			if in.Headers == nil || in.Headers["Content-Type"] == "" {
				req.Header.Set("Content-Type", "application/json")
			}

			// Apply custom headers
			for k, v := range in.Headers {
				req.Header.Set(k, v)
			}

			resp, err := cfg.HTTPClient.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			result, _ := json.Marshal(map[string]any{
				"status":      resp.StatusCode,
				"status_text": resp.Status,
				"body":        string(respBody),
			})
			return tool.Result{Output: result}, nil
		}).
		MustBuild()
}

func notifySMS(cfg NotificationConfig) tool.Tool {
	return tool.NewBuilder("notify_sms").
		WithDescription("Send an SMS message via Twilio").
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"to": {
					"type": "string",
					"description": "Recipient phone number (E.164 format)"
				},
				"body": {
					"type": "string",
					"description": "SMS message body"
				}
			},
			"required": ["to", "body"]
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in smsInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if cfg.TwilioAccountSID == "" || cfg.TwilioAuthToken == "" || cfg.TwilioFromNumber == "" {
				return tool.Result{}, fmt.Errorf("twilio credentials not configured")
			}

			// Build Twilio API URL
			apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", cfg.TwilioAccountSID)

			// Prepare form data
			data := url.Values{}
			data.Set("To", in.To)
			data.Set("From", cfg.TwilioFromNumber)
			data.Set("Body", in.Body)

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
			if err != nil {
				return tool.Result{}, err
			}

			// Set Basic Auth
			auth := base64.StdEncoding.EncodeToString([]byte(cfg.TwilioAccountSID + ":" + cfg.TwilioAuthToken))
			req.Header.Set("Authorization", "Basic "+auth)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := cfg.HTTPClient.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				return tool.Result{}, fmt.Errorf("twilio returned %d: %s", resp.StatusCode, string(respBody))
			}

			result, _ := json.Marshal(map[string]any{
				"status": "sent",
				"sid":    extractSIDFromResponse(respBody),
			})
			return tool.Result{Output: result}, nil
		}).
		MustBuild()
}

func notifyPush(cfg NotificationConfig) tool.Tool {
	return tool.NewBuilder("notify_push").
		WithDescription("Send a push notification via Pushover").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"message": {
					"type": "string",
					"description": "Notification message"
				},
				"title": {
					"type": "string",
					"description": "Optional notification title"
				}
			},
			"required": ["message"]
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in pushInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if cfg.PushoverToken == "" || cfg.PushoverUserKey == "" {
				return tool.Result{}, fmt.Errorf("pushover credentials not configured")
			}

			// Prepare form data
			data := url.Values{}
			data.Set("token", cfg.PushoverToken)
			data.Set("user", cfg.PushoverUserKey)
			data.Set("message", in.Message)
			if in.Title != "" {
				data.Set("title", in.Title)
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.pushover.net/1/messages.json", strings.NewReader(data.Encode()))
			if err != nil {
				return tool.Result{}, err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := cfg.HTTPClient.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != http.StatusOK {
				return tool.Result{}, fmt.Errorf("pushover returned %d: %s", resp.StatusCode, string(respBody))
			}

			result, _ := json.Marshal(map[string]string{"status": "sent"})
			return tool.Result{Output: result}, nil
		}).
		MustBuild()
}

func notifyPagerDuty(cfg NotificationConfig) tool.Tool {
	return tool.NewBuilder("notify_pagerduty").
		WithDescription("Create a PagerDuty incident").
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"summary": {
					"type": "string",
					"description": "Incident summary"
				},
				"severity": {
					"type": "string",
					"description": "Incident severity",
					"enum": ["critical", "error", "warning", "info"]
				},
				"source": {
					"type": "string",
					"description": "Optional source identifier"
				}
			},
			"required": ["summary", "severity"]
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in pagerdutyInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if cfg.PagerDutyRoutingKey == "" {
				return tool.Result{}, fmt.Errorf("pagerduty routing key not configured")
			}

			source := in.Source
			if source == "" {
				source = "agent-go"
			}

			// Build PagerDuty Events API v2 payload
			payload := map[string]any{
				"routing_key":  cfg.PagerDutyRoutingKey,
				"event_action": "trigger",
				"payload": map[string]any{
					"summary":  in.Summary,
					"severity": in.Severity,
					"source":   source,
				},
			}

			body, _ := json.Marshal(payload)

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://events.pagerduty.com/v2/enqueue", bytes.NewReader(body))
			if err != nil {
				return tool.Result{}, err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := cfg.HTTPClient.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != http.StatusAccepted {
				return tool.Result{}, fmt.Errorf("pagerduty returned %d: %s", resp.StatusCode, string(respBody))
			}

			var pdResp map[string]any
			_ = json.Unmarshal(respBody, &pdResp)

			result, _ := json.Marshal(map[string]any{
				"status":    "triggered",
				"dedup_key": pdResp["dedup_key"],
			})
			return tool.Result{Output: result}, nil
		}).
		MustBuild()
}

// extractSIDFromResponse parses the Twilio response to extract the message SID.
func extractSIDFromResponse(body []byte) string {
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err == nil {
		if sid, ok := resp["sid"].(string); ok {
			return sid
		}
	}
	return ""
}
