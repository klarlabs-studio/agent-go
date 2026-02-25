package notification_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/felixgeelhaar/agent-go/contrib/pack-notification"
)

// ExamplePack demonstrates basic usage of the notification pack.
func ExamplePack() {
	// Create a mock Slack server for demonstration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	// Configure notification services
	cfg := notification.NotificationConfig{
		SlackWebhookURL: server.URL,
		HTTPClient:      server.Client(),
	}

	// Create the pack
	pack := notification.Pack(cfg)

	// Get a notification tool
	slackTool, ok := pack.GetTool("notify_slack")
	if !ok {
		log.Fatal("notify_slack tool not found")
	}

	// Prepare input
	input := json.RawMessage(`{
		"text": "Agent task completed successfully",
		"channel": "#agent-alerts"
	}`)

	// Execute the tool
	result, err := slackTool.Execute(context.Background(), input)
	if err != nil {
		log.Fatalf("Failed to send notification: %v", err)
	}

	fmt.Printf("Notification sent: %s\n", string(result.Output))
	// Output: Notification sent: {"status":"sent"}
}

// ExampleNotificationConfig demonstrates configuration options.
func ExampleNotificationConfig() {
	cfg := notification.NotificationConfig{
		// Slack configuration
		SlackWebhookURL: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",

		// Discord configuration
		DiscordWebhookURL: "https://discord.com/api/webhooks/YOUR/WEBHOOK",

		// Teams configuration
		TeamsWebhookURL: "https://outlook.office.com/webhook/YOUR/WEBHOOK/URL",

		// Twilio SMS configuration
		TwilioAccountSID: "ACxxxxxxxxxxxx",
		TwilioAuthToken:  "your_auth_token",
		TwilioFromNumber: "+15551234567",

		// Pushover configuration
		PushoverToken:   "your_application_token",
		PushoverUserKey: "your_user_key",

		// PagerDuty configuration
		PagerDutyRoutingKey: "your_routing_key",

		// Optional: custom HTTP client
		HTTPClient: &http.Client{
			Timeout: 30 * 1e9, // 30 seconds
		},
	}

	pack := notification.Pack(cfg)
	fmt.Printf("Pack contains %d notification tools\n", len(pack.Tools))
	// Output: Pack contains 7 notification tools
}

// ExamplePack_multipleServices demonstrates sending notifications to multiple services.
func ExamplePack_multipleServices() {
	// Create mock servers
	slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer slackServer.Close()

	discordServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discordServer.Close()

	// Configure multiple services
	cfg := notification.NotificationConfig{
		SlackWebhookURL:   slackServer.URL,
		DiscordWebhookURL: discordServer.URL,
		HTTPClient:        slackServer.Client(),
	}

	pack := notification.Pack(cfg)

	// Send to Slack
	slackTool, _ := pack.GetTool("notify_slack")
	slackInput := json.RawMessage(`{"text": "System alert"}`)
	_, err := slackTool.Execute(context.Background(), slackInput)
	if err != nil {
		log.Printf("Slack notification failed: %v", err)
	} else {
		fmt.Println("Slack notification sent")
	}

	// Send to Discord
	discordTool, _ := pack.GetTool("notify_discord")
	discordInput := json.RawMessage(`{"content": "System alert"}`)
	_, err = discordTool.Execute(context.Background(), discordInput)
	if err != nil {
		log.Printf("Discord notification failed: %v", err)
	} else {
		fmt.Println("Discord notification sent")
	}

	// Output:
	// Slack notification sent
	// Discord notification sent
}

// ExamplePack_webhookTool demonstrates using the generic webhook tool.
func ExamplePack_webhookTool() {
	// Create a mock webhook endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received": true}`))
	}))
	defer server.Close()

	cfg := notification.NotificationConfig{
		HTTPClient: server.Client(),
	}

	pack := notification.Pack(cfg)
	webhookTool, _ := pack.GetTool("notify_webhook")

	// Send to custom webhook with headers
	input := map[string]any{
		"url":    server.URL,
		"method": "POST",
		"headers": map[string]string{
			"Authorization": "Bearer token123",
			"Content-Type":  "application/json",
		},
		"body": `{"event": "task_complete", "status": "success"}`,
	}
	inputJSON, _ := json.Marshal(input)

	result, err := webhookTool.Execute(context.Background(), inputJSON)
	if err != nil {
		log.Fatalf("Webhook failed: %v", err)
	}

	var response map[string]any
	json.Unmarshal(result.Output, &response)
	fmt.Printf("Webhook response status: %v\n", response["status"])
	// Output: Webhook response status: 200
}

// ExamplePack_pagerDuty demonstrates PagerDuty incident configuration.
//
// Note: This example shows configuration only. PagerDuty uses a production API endpoint,
// so actual execution requires valid credentials.
func ExamplePack_pagerDuty() {
	cfg := notification.NotificationConfig{
		PagerDutyRoutingKey: "your_routing_key_here",
	}

	pack := notification.Pack(cfg)
	pdTool, ok := pack.GetTool("notify_pagerduty")

	if ok {
		fmt.Printf("PagerDuty tool configured: %s\n", pdTool.Name())
		fmt.Printf("Risk level: %v\n", pdTool.Annotations().RiskLevel)
	}

	// Example input format:
	// {
	//   "summary": "Database connection pool exhausted",
	//   "severity": "critical",
	//   "source": "monitoring-agent"
	// }

	// Output:
	// PagerDuty tool configured: notify_pagerduty
	// Risk level: medium
}
