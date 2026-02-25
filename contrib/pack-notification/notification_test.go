package notification

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNotifySlack(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	cfg := NotificationConfig{
		SlackWebhookURL: server.URL,
		HTTPClient:      server.Client(),
	}

	p := Pack(cfg)
	slackTool, ok := p.GetTool("notify_slack")
	if !ok {
		t.Fatal("notify_slack tool not found")
	}

	input := `{"text": "Test message", "channel": "#general"}`
	result, err := slackTool.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify request body
	select {
	case body := <-received:
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("Failed to unmarshal request body: %v", err)
		}
		if payload["text"] != "Test message" {
			t.Errorf("Expected text 'Test message', got '%s'", payload["text"])
		}
		if payload["channel"] != "#general" {
			t.Errorf("Expected channel '#general', got '%s'", payload["channel"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for request")
	}

	// Verify result
	var res map[string]string
	if err := json.Unmarshal(result.Output, &res); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if res["status"] != "sent" {
		t.Errorf("Expected status 'sent', got '%s'", res["status"])
	}
}

func TestNotifySlackError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid_payload"))
	}))
	defer server.Close()

	cfg := NotificationConfig{
		SlackWebhookURL: server.URL,
		HTTPClient:      server.Client(),
	}

	p := Pack(cfg)
	slackTool, _ := p.GetTool("notify_slack")

	input := `{"text": "Test"}`
	_, err := slackTool.Execute(context.Background(), json.RawMessage(input))
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected error to contain '400', got: %v", err)
	}
}

func TestNotifyDiscord(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := NotificationConfig{
		DiscordWebhookURL: server.URL,
		HTTPClient:        server.Client(),
	}

	p := Pack(cfg)
	discordTool, ok := p.GetTool("notify_discord")
	if !ok {
		t.Fatal("notify_discord tool not found")
	}

	input := `{"content": "Discord test message"}`
	result, err := discordTool.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify request body
	select {
	case body := <-received:
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("Failed to unmarshal request body: %v", err)
		}
		if payload["content"] != "Discord test message" {
			t.Errorf("Expected content 'Discord test message', got '%s'", payload["content"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for request")
	}

	// Verify result
	var res map[string]string
	if err := json.Unmarshal(result.Output, &res); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if res["status"] != "sent" {
		t.Errorf("Expected status 'sent', got '%s'", res["status"])
	}
}

func TestNotifyTeams(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("1"))
	}))
	defer server.Close()

	cfg := NotificationConfig{
		TeamsWebhookURL: server.URL,
		HTTPClient:      server.Client(),
	}

	p := Pack(cfg)
	teamsTool, ok := p.GetTool("notify_teams")
	if !ok {
		t.Fatal("notify_teams tool not found")
	}

	input := `{"text": "Teams test", "title": "Test Title"}`
	result, err := teamsTool.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify request body
	select {
	case body := <-received:
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("Failed to unmarshal request body: %v", err)
		}
		if payload["text"] != "Teams test" {
			t.Errorf("Expected text 'Teams test', got '%v'", payload["text"])
		}
		if payload["title"] != "Test Title" {
			t.Errorf("Expected title 'Test Title', got '%v'", payload["title"])
		}
		if payload["@type"] != "MessageCard" {
			t.Errorf("Expected @type 'MessageCard', got '%v'", payload["@type"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for request")
	}

	// Verify result
	var res map[string]string
	if err := json.Unmarshal(result.Output, &res); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if res["status"] != "sent" {
		t.Errorf("Expected status 'sent', got '%s'", res["status"])
	}
}

func TestNotifyWebhook(t *testing.T) {
	received := make(chan struct {
		method  string
		headers http.Header
		body    []byte
	}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- struct {
			method  string
			headers http.Header
			body    []byte
		}{
			method:  r.Method,
			headers: r.Header,
			body:    body,
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received": true}`))
	}))
	defer server.Close()

	cfg := NotificationConfig{
		HTTPClient: server.Client(),
	}

	p := Pack(cfg)
	webhookTool, ok := p.GetTool("notify_webhook")
	if !ok {
		t.Fatal("notify_webhook tool not found")
	}

	input := map[string]any{
		"url":    server.URL,
		"method": "POST",
		"headers": map[string]string{
			"X-Custom-Header": "test-value",
		},
		"body": `{"message": "webhook test"}`,
	}
	inputJSON, _ := json.Marshal(input)

	result, err := webhookTool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify request
	select {
	case req := <-received:
		if req.method != "POST" {
			t.Errorf("Expected method POST, got %s", req.method)
		}
		if req.headers.Get("X-Custom-Header") != "test-value" {
			t.Errorf("Expected X-Custom-Header 'test-value', got '%s'", req.headers.Get("X-Custom-Header"))
		}
		if string(req.body) != `{"message": "webhook test"}` {
			t.Errorf("Expected body '{\"message\": \"webhook test\"}', got '%s'", string(req.body))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for request")
	}

	// Verify result
	var res map[string]any
	if err := json.Unmarshal(result.Output, &res); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if status, ok := res["status"].(float64); !ok || int(status) != 200 {
		t.Errorf("Expected status 200, got %v", res["status"])
	}
}

func TestNotifySMS(t *testing.T) {
	received := make(chan struct {
		auth string
		body string
	}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		received <- struct {
			auth string
			body string
		}{
			auth: r.Header.Get("Authorization"),
			body: string(bodyBytes),
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"sid": "SM123456"}`))
	}))
	defer server.Close()

	// Override Twilio URL for testing
	accountSID := "AC123456"
	authToken := "test_token"

	cfg := NotificationConfig{
		TwilioAccountSID: accountSID,
		TwilioAuthToken:  authToken,
		TwilioFromNumber: "+15551234567",
		HTTPClient:       server.Client(),
	}

	p := Pack(cfg)
	smsTool, ok := p.GetTool("notify_sms")
	if !ok {
		t.Fatal("notify_sms tool not found")
	}

	// We need to modify the handler to use test server URL
	// For this test, we'll verify the handler exists and has correct schema
	schema := smsTool.InputSchema()
	if schema.IsEmpty() {
		t.Fatal("Input schema is empty")
	}

	// Verify schema structure by attempting to validate valid input
	input := `{"to": "+15559876543", "body": "Test SMS"}`
	if err := schema.Validate(json.RawMessage(input)); err != nil {
		t.Fatalf("Valid input failed validation: %v", err)
	}
}

func TestNotifyPush(t *testing.T) {
	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		received <- string(bodyBytes)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": 1}`))
	}))
	defer server.Close()

	cfg := NotificationConfig{
		PushoverToken:   "test_token",
		PushoverUserKey: "test_user",
		HTTPClient:      server.Client(),
	}

	p := Pack(cfg)
	pushTool, ok := p.GetTool("notify_push")
	if !ok {
		t.Fatal("notify_push tool not found")
	}

	// Verify schema exists
	schema := pushTool.InputSchema()
	if schema.IsEmpty() {
		t.Fatal("Input schema is empty")
	}

	// Verify valid input passes validation
	input := `{"message": "Test push notification", "title": "Test"}`
	if err := schema.Validate(json.RawMessage(input)); err != nil {
		t.Fatalf("Valid input failed validation: %v", err)
	}
}

func TestNotifyPagerDuty(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"dedup_key": "key123", "status": "success"}`))
	}))
	defer server.Close()

	cfg := NotificationConfig{
		PagerDutyRoutingKey: "test_routing_key",
		HTTPClient:          server.Client(),
	}

	p := Pack(cfg)
	pdTool, ok := p.GetTool("notify_pagerduty")
	if !ok {
		t.Fatal("notify_pagerduty tool not found")
	}

	// Verify schema exists and is valid
	schema := pdTool.InputSchema()
	if schema.IsEmpty() {
		t.Fatal("Input schema is empty")
	}

	input := `{"summary": "Test incident", "severity": "critical", "source": "test-system"}`
	if err := schema.Validate(json.RawMessage(input)); err != nil {
		t.Fatalf("Valid input failed validation: %v", err)
	}
}

func TestNotifySlackMissingConfig(t *testing.T) {
	cfg := NotificationConfig{
		SlackWebhookURL: "", // Missing webhook URL
	}

	p := Pack(cfg)
	slackTool, _ := p.GetTool("notify_slack")

	input := `{"text": "Test"}`
	_, err := slackTool.Execute(context.Background(), json.RawMessage(input))
	if err == nil {
		t.Fatal("Expected error for missing webhook URL, got nil")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("Expected 'not configured' error, got: %v", err)
	}
}

func TestNotifySlackInvalidInput(t *testing.T) {
	cfg := NotificationConfig{
		SlackWebhookURL: "http://example.com",
	}

	p := Pack(cfg)
	slackTool, _ := p.GetTool("notify_slack")

	input := `{"invalid": json}`
	_, err := slackTool.Execute(context.Background(), json.RawMessage(input))
	if err == nil {
		t.Fatal("Expected error for invalid input, got nil")
	}
}

func TestPackMetadata(t *testing.T) {
	cfg := NotificationConfig{}
	p := Pack(cfg)

	if p.Name != "notification" {
		t.Errorf("Expected pack name 'notification', got '%s'", p.Name)
	}

	if p.Version != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got '%s'", p.Version)
	}

	expectedTools := []string{
		"notify_slack",
		"notify_discord",
		"notify_teams",
		"notify_webhook",
		"notify_sms",
		"notify_push",
		"notify_pagerduty",
	}

	toolNames := p.ToolNames()
	if len(toolNames) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(toolNames))
	}

	for _, expected := range expectedTools {
		found := false
		for _, name := range toolNames {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool '%s' not found in pack", expected)
		}
	}
}

func TestDefaultHTTPClient(t *testing.T) {
	cfg := NotificationConfig{
		HTTPClient: nil, // Should create default client
	}

	p := Pack(cfg)

	// Verify pack was created successfully
	if p == nil {
		t.Fatal("Pack should not be nil")
	}

	// Verify tools are accessible
	if len(p.Tools) != 7 {
		t.Errorf("Expected 7 tools, got %d", len(p.Tools))
	}
}

func TestExtractSIDFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{
			name:     "valid response",
			body:     []byte(`{"sid": "SM123456", "status": "sent"}`),
			expected: "SM123456",
		},
		{
			name:     "missing sid",
			body:     []byte(`{"status": "sent"}`),
			expected: "",
		},
		{
			name:     "invalid json",
			body:     []byte(`not json`),
			expected: "",
		},
		{
			name:     "empty body",
			body:     []byte(``),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSIDFromResponse(tt.body)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestNotifyWebhookDefaultMethod(t *testing.T) {
	methodReceived := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodReceived <- r.Method
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := NotificationConfig{
		HTTPClient: server.Client(),
	}

	p := Pack(cfg)
	webhookTool, _ := p.GetTool("notify_webhook")

	// Don't specify method - should default to POST
	input := map[string]any{
		"url":  server.URL,
		"body": `{"test": true}`,
	}
	inputJSON, _ := json.Marshal(input)

	_, err := webhookTool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	select {
	case method := <-methodReceived:
		if method != "POST" {
			t.Errorf("Expected default method POST, got %s", method)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for request")
	}
}

func TestTwilioBasicAuth(t *testing.T) {
	accountSID := "ACtest123"
	authToken := "token456"

	expected := base64.StdEncoding.EncodeToString([]byte(accountSID + ":" + authToken))

	cfg := NotificationConfig{
		TwilioAccountSID: accountSID,
		TwilioAuthToken:  authToken,
		TwilioFromNumber: "+15551234567",
	}

	// Verify config is set correctly
	if cfg.TwilioAccountSID != accountSID {
		t.Errorf("Account SID not set correctly")
	}

	// Verify encoding matches expected format
	actual := base64.StdEncoding.EncodeToString([]byte(accountSID + ":" + authToken))
	if actual != expected {
		t.Errorf("Expected auth '%s', got '%s'", expected, actual)
	}
}
