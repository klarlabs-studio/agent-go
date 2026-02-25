# Notification Pack

The notification pack provides tools for sending alerts and messages to various communication platforms.

## Tools

### notify_slack
Send a message to a Slack channel via incoming webhook.

**Input:**
```json
{
  "text": "Message text to send",
  "channel": "#general"  // Optional channel override
}
```

**Configuration:** `SlackWebhookURL`

### notify_discord
Send a message to a Discord channel via webhook.

**Input:**
```json
{
  "content": "Message content to send"
}
```

**Configuration:** `DiscordWebhookURL`

### notify_teams
Send a message to a Microsoft Teams channel via incoming webhook.

**Input:**
```json
{
  "text": "Message text to send",
  "title": "Optional message title"
}
```

**Configuration:** `TeamsWebhookURL`

### notify_webhook
Send a notification to a generic webhook endpoint.

**Input:**
```json
{
  "url": "https://example.com/webhook",
  "method": "POST",  // Optional, defaults to POST
  "headers": {       // Optional custom headers
    "Authorization": "Bearer token"
  },
  "body": "{\"message\": \"test\"}"
}
```

**Configuration:** None (URL provided in input)

### notify_sms
Send an SMS message via Twilio.

**Input:**
```json
{
  "to": "+15551234567",  // E.164 format
  "body": "SMS message body"
}
```

**Configuration:** `TwilioAccountSID`, `TwilioAuthToken`, `TwilioFromNumber`

### notify_push
Send a push notification via Pushover.

**Input:**
```json
{
  "message": "Notification message",
  "title": "Optional notification title"
}
```

**Configuration:** `PushoverToken`, `PushoverUserKey`

### notify_pagerduty
Create a PagerDuty incident via Events API v2.

**Input:**
```json
{
  "summary": "Incident summary",
  "severity": "critical",  // critical, error, warning, info
  "source": "optional-source-identifier"
}
```

**Configuration:** `PagerDutyRoutingKey`

## Usage

```go
import (
    "github.com/felixgeelhaar/agent-go/contrib/pack-notification"
    api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

// Configure notification services
cfg := notification.NotificationConfig{
    SlackWebhookURL:     "https://hooks.slack.com/services/...",
    DiscordWebhookURL:   "https://discord.com/api/webhooks/...",
    TeamsWebhookURL:     "https://outlook.office.com/webhook/...",

    TwilioAccountSID:    "ACxxxx",
    TwilioAuthToken:     "your_auth_token",
    TwilioFromNumber:    "+15551234567",

    PushoverToken:       "your_pushover_token",
    PushoverUserKey:     "your_pushover_user_key",

    PagerDutyRoutingKey: "your_routing_key",

    // Optional: provide custom HTTP client
    HTTPClient:          nil, // defaults to 30s timeout client
}

// Create the pack
notificationPack := notification.Pack(cfg)

// Register with engine
registry := api.NewToolRegistry()
for _, tool := range notificationPack.Tools {
    registry.Register(tool)
}

engine, err := api.New(
    api.WithRegistry(registry),
    api.WithPlanner(myPlanner),
    // ... other options
)
```

## Configuration

All notification tools require appropriate credentials to be configured in `NotificationConfig`. Tools will return an error if required credentials are missing.

### Obtaining Credentials

**Slack:** Create an incoming webhook at https://api.slack.com/messaging/webhooks

**Discord:** Create a webhook in your Discord server settings

**Teams:** Create an incoming webhook in your Teams channel

**Twilio:** Sign up at https://www.twilio.com/ and obtain Account SID, Auth Token, and phone number

**Pushover:** Create an application at https://pushover.net/apps/build

**PagerDuty:** Create an Events API v2 integration in PagerDuty and obtain the routing key

## Risk Levels

- **notify_slack, notify_discord, notify_teams, notify_webhook, notify_push:** RiskLow
- **notify_sms, notify_pagerduty:** RiskMedium (can trigger external alerts/costs)

## State Eligibility

By default, all notification tools are allowed only in the `Act` state, as they perform side effects (sending external notifications).

## Testing

The pack includes comprehensive tests using `httptest` to mock HTTP endpoints:

```bash
cd contrib/pack-notification
go test -race -v ./...
```

## Example

```go
// Send a Slack notification
input := json.RawMessage(`{
    "text": "Agent completed task successfully",
    "channel": "#agent-alerts"
}`)

tool, _ := notificationPack.GetTool("notify_slack")
result, err := tool.Execute(ctx, input)
if err != nil {
    log.Printf("Failed to send notification: %v", err)
}

// Create a PagerDuty incident
pdInput := json.RawMessage(`{
    "summary": "Critical system error detected",
    "severity": "critical",
    "source": "monitoring-agent"
}`)

pdTool, _ := notificationPack.GetTool("notify_pagerduty")
result, err = pdTool.Execute(ctx, pdInput)
```

## Architecture

The notification pack follows the agent-go domain model:

- **Configuration:** Centralized in `NotificationConfig` struct
- **HTTP Client:** Shared client with 30s timeout (configurable)
- **Error Handling:** Returns detailed errors with HTTP status codes
- **Resilience:** Compatible with fortify resilience patterns
- **State Constraints:** Restricted to Act state by default

## Dependencies

- Standard library only (`net/http`, `encoding/json`, `context`)
- No external HTTP libraries required
- Compatible with Go 1.21+
