// Package monitoring provides monitoring tools for agent-go.
//
// This pack includes tools for observability and monitoring:
//   - metrics_query: Query metrics from a time series database
//   - metrics_push: Push custom metrics
//   - alerts_list: List active alerts
//   - alerts_silence: Silence an alert
//   - logs_query: Query structured logs
//   - traces_query: Query distributed traces
//   - health_check: Check service health endpoints
//   - dashboard_get: Get dashboard configuration
//
// Supports Prometheus, Grafana, Datadog, CloudWatch, and OpenTelemetry.
package monitoring

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the monitoring tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("monitoring").
		WithDescription("Monitoring and observability tools").
		WithVersion("0.1.0").
		AddTools(
			metricsQuery(),
			metricsPush(),
			alertsList(),
			alertsSilence(),
			logsQuery(),
			tracesQuery(),
			healthCheck(),
			dashboardGet(),
		).
		AllowInState(agent.StateExplore, "metrics_query", "alerts_list", "logs_query", "traces_query", "health_check", "dashboard_get").
		AllowInState(agent.StateAct, "metrics_query", "metrics_push", "alerts_list", "alerts_silence", "logs_query", "traces_query", "health_check", "dashboard_get").
		AllowInState(agent.StateValidate, "metrics_query", "alerts_list", "health_check").
		Build()
}

func metricsQuery() tool.Tool {
	return tool.NewBuilder("metrics_query").
		WithDescription("Query metrics using PromQL or similar query language").
		ReadOnly().
		MustBuild()
}

func metricsPush() tool.Tool {
	return tool.NewBuilder("metrics_push").
		WithDescription("Push custom metrics to a monitoring system").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func alertsList() tool.Tool {
	return tool.NewBuilder("alerts_list").
		WithDescription("List active alerts and their status").
		ReadOnly().
		MustBuild()
}

func alertsSilence() tool.Tool {
	return tool.NewBuilder("alerts_silence").
		WithDescription("Silence an alert for a specified duration").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func logsQuery() tool.Tool {
	return tool.NewBuilder("logs_query").
		WithDescription("Query structured logs using LogQL or similar").
		ReadOnly().
		MustBuild()
}

func tracesQuery() tool.Tool {
	return tool.NewBuilder("traces_query").
		WithDescription("Query distributed traces by trace ID or filters").
		ReadOnly().
		MustBuild()
}

func healthCheck() tool.Tool {
	return tool.NewBuilder("health_check").
		WithDescription("Check health of services via HTTP endpoints").
		ReadOnly().
		MustBuild()
}

func dashboardGet() tool.Tool {
	return tool.NewBuilder("dashboard_get").
		WithDescription("Get dashboard configuration and panels").
		ReadOnly().
		Cacheable().
		MustBuild()
}
