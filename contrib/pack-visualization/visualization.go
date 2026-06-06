// Package visualization provides data visualization tools for agent-go.
//
// The pack uses an interface-based approach, allowing any charting or
// reporting engine to be plugged in.
package visualization

import (
	"context"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// ChartEngine creates charts and visualizations.
type ChartEngine interface {
	CreateChart(ctx context.Context, opts ChartOptions) (*ChartOutput, error)
	CreateHeatmap(ctx context.Context, opts HeatmapOptions) (*ChartOutput, error)
}

// ReportEngine generates reports and dashboards.
type ReportEngine interface {
	GenerateReport(ctx context.Context, opts ReportOptions) (*ReportOutput, error)
	ExportDashboard(ctx context.Context, opts DashboardOptions) (*DashboardOutput, error)
}

// DataSeries represents a data series for charting.
type DataSeries struct {
	Name   string    `json:"name"`
	X      []float64 `json:"x,omitempty"`
	Y      []float64 `json:"y"`
	Labels []string  `json:"labels,omitempty"`
	Color  string    `json:"color,omitempty"`
}

// ChartOptions configures chart creation.
type ChartOptions struct {
	Type   string       `json:"type"` // "bar", "line", "pie", "scatter", "area", "histogram"
	Title  string       `json:"title,omitempty"`
	XLabel string       `json:"x_label,omitempty"`
	YLabel string       `json:"y_label,omitempty"`
	Series []DataSeries `json:"series"`
	Width  int          `json:"width,omitempty"`
	Height int          `json:"height,omitempty"`
	Format string       `json:"format,omitempty"` // "png", "svg", "pdf"
}

// ChartOutput contains generated chart data.
type ChartOutput struct {
	Data   string `json:"data"` // base64 encoded
	Format string `json:"format"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// HeatmapOptions configures heatmap creation.
type HeatmapOptions struct {
	Title    string      `json:"title,omitempty"`
	XLabels  []string    `json:"x_labels"`
	YLabels  []string    `json:"y_labels"`
	Values   [][]float64 `json:"values"`
	ColorMap string      `json:"color_map,omitempty"` // "viridis", "plasma", "hot", "cool"
	Width    int         `json:"width,omitempty"`
	Height   int         `json:"height,omitempty"`
	Format   string      `json:"format,omitempty"`
}

// ReportOptions configures report generation.
type ReportOptions struct {
	Title    string          `json:"title"`
	Sections []ReportSection `json:"sections"`
	Format   string          `json:"format,omitempty"` // "html", "pdf", "markdown"
}

// ReportSection represents a section in a report.
type ReportSection struct {
	Title   string `json:"title"`
	Type    string `json:"type"` // "text", "chart", "table", "metric"
	Content any    `json:"content"`
}

// ReportOutput contains generated report data.
type ReportOutput struct {
	Data   string `json:"data"`
	Format string `json:"format"`
	Pages  int    `json:"pages,omitempty"`
}

// DashboardOptions configures dashboard export.
type DashboardOptions struct {
	Title  string           `json:"title"`
	Panels []DashboardPanel `json:"panels"`
	Format string           `json:"format,omitempty"` // "html", "pdf", "png"
	Width  int              `json:"width,omitempty"`
	Height int              `json:"height,omitempty"`
}

// DashboardPanel represents a panel in a dashboard.
type DashboardPanel struct {
	Title   string `json:"title"`
	Type    string `json:"type"` // "chart", "metric", "table", "text"
	Content any    `json:"content"`
	Row     int    `json:"row"`
	Col     int    `json:"col"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

// DashboardOutput contains exported dashboard data.
type DashboardOutput struct {
	Data   string `json:"data"`
	Format string `json:"format"`
	Panels int    `json:"panels"`
}

// Config holds visualization pack configuration.
type Config struct {
	Charts  ChartEngine
	Reports ReportEngine // optional
}

// Pack returns the data visualization tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &vizPack{cfg: cfg}

	tools := []tool.Tool{
		p.createChartTool(), p.createHeatmapTool(),
	}

	if cfg.Reports != nil {
		tools = append(tools, p.generateReportTool(), p.exportDashboardTool())
	}

	return pack.NewBuilder("visualization").
		WithDescription("Data visualization tools: create_chart, create_heatmap, generate_report, export_dashboard").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type vizPack struct{ cfg Config }

func (p *vizPack) createChartTool() tool.Tool {
	return tool.NewBuilder("viz_create_chart").
		WithDescription("Create a chart visualization from data series").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in ChartOptions
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Type == "" {
				return tool.Result{}, fmt.Errorf("chart type is required")
			}
			if len(in.Series) == 0 {
				return tool.Result{}, fmt.Errorf("at least one data series is required")
			}
			if in.Format == "" {
				in.Format = "png"
			}
			result, err := p.cfg.Charts.CreateChart(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("create chart failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *vizPack) createHeatmapTool() tool.Tool {
	return tool.NewBuilder("viz_create_heatmap").
		WithDescription("Create a heatmap visualization from a 2D data matrix").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in HeatmapOptions
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if len(in.XLabels) == 0 || len(in.YLabels) == 0 {
				return tool.Result{}, fmt.Errorf("x_labels and y_labels are required")
			}
			if len(in.Values) == 0 {
				return tool.Result{}, fmt.Errorf("values matrix is required")
			}
			if in.Format == "" {
				in.Format = "png"
			}
			result, err := p.cfg.Charts.CreateHeatmap(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("create heatmap failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *vizPack) generateReportTool() tool.Tool {
	return tool.NewBuilder("viz_generate_report").
		WithDescription("Generate a report with text, charts, and tables").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in ReportOptions
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Title == "" {
				return tool.Result{}, fmt.Errorf("title is required")
			}
			if len(in.Sections) == 0 {
				return tool.Result{}, fmt.Errorf("at least one section is required")
			}
			if in.Format == "" {
				in.Format = "html"
			}
			result, err := p.cfg.Reports.GenerateReport(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("generate report failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *vizPack) exportDashboardTool() tool.Tool {
	return tool.NewBuilder("viz_export_dashboard").
		WithDescription("Export a multi-panel dashboard visualization").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in DashboardOptions
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Title == "" {
				return tool.Result{}, fmt.Errorf("title is required")
			}
			if len(in.Panels) == 0 {
				return tool.Result{}, fmt.Errorf("at least one panel is required")
			}
			if in.Format == "" {
				in.Format = "html"
			}
			result, err := p.cfg.Reports.ExportDashboard(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("export dashboard failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
