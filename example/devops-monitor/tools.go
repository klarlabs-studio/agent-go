package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/tool"
	api "go.klarlabs.de/agent/interfaces/api"
)

// MockInfrastructure simulates a monitoring backend.
type MockInfrastructure struct {
	mu       sync.RWMutex
	services map[string]*ServiceState
	logs     map[string][]LogEntry
	alerts   []Alert
}

// ServiceState represents a service's current state.
type ServiceState struct {
	Name         string  `json:"name"`
	CPU          float64 `json:"cpu"`
	Memory       float64 `json:"memory"`
	ErrorsPerMin int     `json:"errors_per_min"`
	Status       string  `json:"status"`
	Restarts     int     `json:"restarts"`
}

// LogEntry represents a log line.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Count     int       `json:"count,omitempty"`
}

// Alert represents an alert notification.
type Alert struct {
	ID        string    `json:"id"`
	Service   string    `json:"service"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

// NewMockInfrastructure creates infrastructure with sample data.
func NewMockInfrastructure() *MockInfrastructure {
	return &MockInfrastructure{
		services: map[string]*ServiceState{
			"api-gateway": {
				Name:         "api-gateway",
				CPU:          23.0,
				Memory:       67.0,
				ErrorsPerMin: 847,
				Status:       "degraded",
				Restarts:     0,
			},
			"user-service": {
				Name:         "user-service",
				CPU:          45.0,
				Memory:       52.0,
				ErrorsPerMin: 2,
				Status:       "healthy",
				Restarts:     0,
			},
			"payment-service": {
				Name:         "payment-service",
				CPU:          31.0,
				Memory:       48.0,
				ErrorsPerMin: 0,
				Status:       "healthy",
				Restarts:     0,
			},
		},
		logs: map[string][]LogEntry{
			"api-gateway": {
				{Timestamp: time.Now().Add(-5 * time.Minute), Level: "error", Message: "connection pool exhausted", Count: 312},
				{Timestamp: time.Now().Add(-4 * time.Minute), Level: "error", Message: "upstream timeout", Count: 156},
				{Timestamp: time.Now().Add(-3 * time.Minute), Level: "warn", Message: "high latency detected", Count: 89},
				{Timestamp: time.Now().Add(-2 * time.Minute), Level: "error", Message: "connection pool exhausted", Count: 289},
			},
			"user-service": {
				{Timestamp: time.Now().Add(-10 * time.Minute), Level: "info", Message: "cache refreshed", Count: 1},
				{Timestamp: time.Now().Add(-5 * time.Minute), Level: "info", Message: "health check passed", Count: 5},
			},
		},
		alerts: []Alert{},
	}
}

// --- Tool Input/Output Types ---

type GetMetricsInput struct {
	Service string `json:"service"`
}

type GetMetricsOutput struct {
	CPU          string `json:"cpu"`
	Memory       string `json:"memory"`
	ErrorsPerMin int    `json:"errors"`
	Status       string `json:"status"`
}

type QueryLogsInput struct {
	Service string `json:"service"`
	Level   string `json:"level"`
	Limit   int    `json:"limit"`
}

type QueryLogsOutput struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
	Level   string `json:"level"`
}

type RestartServiceInput struct {
	Service  string `json:"service"`
	Graceful bool   `json:"graceful"`
}

type RestartServiceOutput struct {
	Status   string `json:"status"`
	Downtime string `json:"downtime"`
}

type SendAlertInput struct {
	Service  string `json:"service"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type SendAlertOutput struct {
	AlertID   string `json:"alert_id"`
	Delivered bool   `json:"delivered"`
}

// --- Tool Constructors ---

// NewGetMetricsTool creates a tool to fetch service metrics.
func NewGetMetricsTool(infra *MockInfrastructure) tool.Tool {
	return api.NewToolBuilder("get_metrics").
		WithDescription("Fetch service health metrics").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  false, // Metrics should be fresh
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"service": {"type": "string", "description": "Service name to get metrics for"}
			},
			"required": ["service"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"cpu": {"type": "string"},
				"memory": {"type": "string"},
				"errors": {"type": "integer"},
				"status": {"type": "string"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in GetMetricsInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			infra.mu.RLock()
			defer infra.mu.RUnlock()

			svc, ok := infra.services[in.Service]
			if !ok {
				return tool.Result{}, fmt.Errorf("service not found: %s", in.Service)
			}

			output := GetMetricsOutput{
				CPU:          fmt.Sprintf("%.0f%%", svc.CPU),
				Memory:       fmt.Sprintf("%.0f%%", svc.Memory),
				ErrorsPerMin: svc.ErrorsPerMin,
				Status:       svc.Status,
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewQueryLogsTool creates a tool to search application logs.
func NewQueryLogsTool(infra *MockInfrastructure) tool.Tool {
	return api.NewToolBuilder("query_logs").
		WithDescription("Search application logs").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  false,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"service": {"type": "string", "description": "Service name"},
				"level": {"type": "string", "description": "Log level (error, warn, info)"},
				"limit": {"type": "integer", "description": "Max entries to return"}
			},
			"required": ["service", "level"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"pattern": {"type": "string"},
				"count": {"type": "integer"},
				"level": {"type": "string"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in QueryLogsInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if in.Limit == 0 {
				in.Limit = 10
			}

			infra.mu.RLock()
			defer infra.mu.RUnlock()

			logs, ok := infra.logs[in.Service]
			if !ok {
				return tool.Result{}, fmt.Errorf("no logs for service: %s", in.Service)
			}

			// Filter by level and find most common pattern
			patternCounts := make(map[string]int)
			totalCount := 0
			for _, entry := range logs {
				if strings.EqualFold(entry.Level, in.Level) {
					patternCounts[entry.Message] += entry.Count
					totalCount += entry.Count
				}
			}

			// Find dominant pattern
			var topPattern string
			var topCount int
			for pattern, count := range patternCounts {
				if count > topCount {
					topPattern = pattern
					topCount = count
				}
			}

			output := QueryLogsOutput{
				Pattern: topPattern,
				Count:   totalCount,
				Level:   in.Level,
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewRestartServiceTool creates a tool to restart services.
func NewRestartServiceTool(infra *MockInfrastructure) tool.Tool {
	return api.NewToolBuilder("restart_service").
		WithDescription("Restart a service (requires approval)").
		WithAnnotations(api.Annotations{
			ReadOnly:    false,
			Destructive: true, // Requires approval
			Idempotent:  true, // Restarting twice is safe
			RiskLevel:   api.RiskHigh,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"service": {"type": "string", "description": "Service to restart"},
				"graceful": {"type": "boolean", "description": "Whether to drain connections first"}
			},
			"required": ["service"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"status": {"type": "string"},
				"downtime": {"type": "string"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in RestartServiceInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			infra.mu.Lock()
			defer infra.mu.Unlock()

			svc, ok := infra.services[in.Service]
			if !ok {
				return tool.Result{}, fmt.Errorf("service not found: %s", in.Service)
			}

			// Simulate restart
			downtime := "3.2s"
			if !in.Graceful {
				downtime = "0.5s"
			}

			// Reset service state after restart
			svc.Status = "healthy"
			svc.ErrorsPerMin = 2
			svc.CPU = 18.0
			svc.Memory = 45.0
			svc.Restarts++

			// Clear error logs
			infra.logs[in.Service] = []LogEntry{
				{Timestamp: time.Now(), Level: "info", Message: "service restarted", Count: 1},
			}

			output := RestartServiceOutput{
				Status:   "restarted",
				Downtime: downtime,
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewSendAlertTool creates a tool to send alerts.
func NewSendAlertTool(infra *MockInfrastructure) tool.Tool {
	return api.NewToolBuilder("send_alert").
		WithDescription("Send alert to on-call team").
		WithAnnotations(api.Annotations{
			ReadOnly:    false,
			Destructive: false,
			Idempotent:  true, // Duplicate alerts are deduplicated
			RiskLevel:   api.RiskMedium,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"service": {"type": "string", "description": "Service name"},
				"message": {"type": "string", "description": "Alert message"},
				"severity": {"type": "string", "description": "Alert severity (info, warning, critical)"}
			},
			"required": ["service", "message", "severity"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"alert_id": {"type": "string"},
				"delivered": {"type": "boolean"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in SendAlertInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			infra.mu.Lock()
			defer infra.mu.Unlock()

			alertID := fmt.Sprintf("ALT-%d", len(infra.alerts)+1001)
			alert := Alert{
				ID:        alertID,
				Service:   in.Service,
				Message:   in.Message,
				Severity:  in.Severity,
				Timestamp: time.Now(),
			}
			infra.alerts = append(infra.alerts, alert)

			output := SendAlertOutput{
				AlertID:   alertID,
				Delivered: true,
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}
