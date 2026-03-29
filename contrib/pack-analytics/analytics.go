// Package analytics provides product analytics tools for agent-go.
//
// The pack uses an interface-based approach, allowing any analytics platform
// (Mixpanel, Amplitude, PostHog, custom, etc.) to be plugged in.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// AnalyticsPlatform provides product analytics capabilities.
type AnalyticsPlatform interface {
	TrackEvent(ctx context.Context, event Event) error
	QueryMetrics(ctx context.Context, query MetricQuery) (*MetricResult, error)
	CreateSegment(ctx context.Context, segment Segment) (*SegmentResult, error)
	FunnelAnalysis(ctx context.Context, funnel FunnelQuery) (*FunnelResult, error)
}

// Event represents an analytics event.
type Event struct {
	Name       string         `json:"name"`
	UserID     string         `json:"user_id,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Timestamp  string         `json:"timestamp,omitempty"`
}

// MetricQuery configures a metrics query.
type MetricQuery struct {
	Metric    string            `json:"metric"` // event name or built-in metric
	GroupBy   []string          `json:"group_by,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"`
	StartDate string            `json:"start_date"`
	EndDate   string            `json:"end_date"`
	Interval  string            `json:"interval,omitempty"` // "hour", "day", "week", "month"
}

// MetricResult contains metrics query results.
type MetricResult struct {
	Metric     string      `json:"metric"`
	DataPoints []DataPoint `json:"data_points"`
	Total      float64     `json:"total"`
	Average    float64     `json:"average"`
}

// DataPoint represents a single metric data point.
type DataPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
	Group     string  `json:"group,omitempty"`
}

// Segment defines a user segment.
type Segment struct {
	Name       string             `json:"name"`
	Conditions []SegmentCondition `json:"conditions"`
}

// SegmentCondition describes a segment filter condition.
type SegmentCondition struct {
	Property string `json:"property"`
	Operator string `json:"operator"` // "eq", "neq", "gt", "lt", "contains", "in"
	Value    any    `json:"value"`
}

// SegmentResult contains segment creation output.
type SegmentResult struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"user_count"`
}

// FunnelQuery configures a funnel analysis.
type FunnelQuery struct {
	Name      string   `json:"name"`
	Steps     []string `json:"steps"` // ordered event names
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	GroupBy   string   `json:"group_by,omitempty"`
}

// FunnelResult contains funnel analysis output.
type FunnelResult struct {
	Name        string       `json:"name"`
	Steps       []FunnelStep `json:"steps"`
	OverallRate float64      `json:"overall_conversion_rate"`
}

// FunnelStep represents a step in the funnel.
type FunnelStep struct {
	Name           string  `json:"name"`
	Count          int     `json:"count"`
	ConversionRate float64 `json:"conversion_rate"`
	DropoffRate    float64 `json:"dropoff_rate"`
}

// Config holds analytics pack configuration.
type Config struct {
	Platform AnalyticsPlatform
}

// Pack returns the product analytics tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &analyticsPack{cfg: cfg}

	return pack.NewBuilder("analytics").
		WithDescription("Product analytics tools: track_event, query_metrics, create_segment, funnel_analysis").
		WithVersion("1.0.0").
		AddTools(
			p.trackEventTool(), p.queryMetricsTool(),
			p.createSegmentTool(), p.funnelAnalysisTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type analyticsPack struct{ cfg Config }

func (p *analyticsPack) trackEventTool() tool.Tool {
	return tool.NewBuilder("analytics_track_event").
		WithDescription("Track a product analytics event").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in Event
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Name == "" {
				return tool.Result{}, fmt.Errorf("event name is required")
			}
			err := p.cfg.Platform.TrackEvent(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("track event failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"event": in.Name, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *analyticsPack) queryMetricsTool() tool.Tool {
	return tool.NewBuilder("analytics_query_metrics").
		WithDescription("Query analytics metrics over a time range").
		ReadOnly().Idempotent().Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in MetricQuery
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Metric == "" {
				return tool.Result{}, fmt.Errorf("metric is required")
			}
			if in.StartDate == "" || in.EndDate == "" {
				return tool.Result{}, fmt.Errorf("start_date and end_date are required")
			}
			if in.Interval == "" {
				in.Interval = "day"
			}
			result, err := p.cfg.Platform.QueryMetrics(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("query metrics failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *analyticsPack) createSegmentTool() tool.Tool {
	return tool.NewBuilder("analytics_create_segment").
		WithDescription("Create a user segment based on conditions").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in Segment
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Name == "" {
				return tool.Result{}, fmt.Errorf("segment name is required")
			}
			if len(in.Conditions) == 0 {
				return tool.Result{}, fmt.Errorf("at least one condition is required")
			}
			result, err := p.cfg.Platform.CreateSegment(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("create segment failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *analyticsPack) funnelAnalysisTool() tool.Tool {
	return tool.NewBuilder("analytics_funnel_analysis").
		WithDescription("Analyze conversion funnel across ordered event steps").
		ReadOnly().Idempotent().Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in FunnelQuery
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if len(in.Steps) < 2 {
				return tool.Result{}, fmt.Errorf("at least 2 funnel steps are required")
			}
			if in.StartDate == "" || in.EndDate == "" {
				return tool.Result{}, fmt.Errorf("start_date and end_date are required")
			}
			result, err := p.cfg.Platform.FunnelAnalysis(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("funnel analysis failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
