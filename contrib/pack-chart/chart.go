// Package chart provides data visualization tools for agents.
// It enables generation of various chart types as SVG output.
package chart

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the chart tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("chart").
		WithDescription("Data visualization and charting tools").
		AddTools(
			barChartTool(),
			lineChartTool(),
			pieChartTool(),
			scatterPlotTool(),
			histogramChartTool(),
			areaChartTool(),
			heatmapTool(),
			boxPlotTool(),
			sparklineTool(),
			gaugeChartTool(),
			radarChartTool(),
			funnelChartTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// SVGChart represents a generated chart
type SVGChart struct {
	SVG    string `json:"svg"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Title  string `json:"title,omitempty"`
}

func barChartTool() tool.Tool {
	return tool.NewBuilder("chart_bar").
		WithDescription("Generate a bar chart SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Labels     []string  `json:"labels"`
				Values     []float64 `json:"values"`
				Title      string    `json:"title,omitempty"`
				Width      int       `json:"width,omitempty"`
				Height     int       `json:"height,omitempty"`
				Color      string    `json:"color,omitempty"`
				Horizontal bool      `json:"horizontal,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 600
			}
			if params.Height == 0 {
				params.Height = 400
			}
			if params.Color == "" {
				params.Color = "#4a90d9"
			}

			svg := generateBarChart(params.Labels, params.Values, params.Title, params.Width, params.Height, params.Color, params.Horizontal)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lineChartTool() tool.Tool {
	return tool.NewBuilder("chart_line").
		WithDescription("Generate a line chart SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Labels []string    `json:"labels"`
				Series [][]float64 `json:"series"`
				Names  []string    `json:"names,omitempty"`
				Title  string      `json:"title,omitempty"`
				Width  int         `json:"width,omitempty"`
				Height int         `json:"height,omitempty"`
				Colors []string    `json:"colors,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 600
			}
			if params.Height == 0 {
				params.Height = 400
			}
			if len(params.Colors) == 0 {
				params.Colors = defaultColors()
			}

			svg := generateLineChart(params.Labels, params.Series, params.Names, params.Title, params.Width, params.Height, params.Colors)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pieChartTool() tool.Tool {
	return tool.NewBuilder("chart_pie").
		WithDescription("Generate a pie chart SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Labels []string  `json:"labels"`
				Values []float64 `json:"values"`
				Title  string    `json:"title,omitempty"`
				Width  int       `json:"width,omitempty"`
				Height int       `json:"height,omitempty"`
				Colors []string  `json:"colors,omitempty"`
				Donut  bool      `json:"donut,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 400
			}
			if params.Height == 0 {
				params.Height = 400
			}
			if len(params.Colors) == 0 {
				params.Colors = defaultColors()
			}

			svg := generatePieChart(params.Labels, params.Values, params.Title, params.Width, params.Height, params.Colors, params.Donut)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func scatterPlotTool() tool.Tool {
	return tool.NewBuilder("chart_scatter").
		WithDescription("Generate a scatter plot SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				X      []float64 `json:"x"`
				Y      []float64 `json:"y"`
				Title  string    `json:"title,omitempty"`
				XLabel string    `json:"x_label,omitempty"`
				YLabel string    `json:"y_label,omitempty"`
				Width  int       `json:"width,omitempty"`
				Height int       `json:"height,omitempty"`
				Color  string    `json:"color,omitempty"`
				Size   int       `json:"size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 600
			}
			if params.Height == 0 {
				params.Height = 400
			}
			if params.Color == "" {
				params.Color = "#4a90d9"
			}
			if params.Size == 0 {
				params.Size = 5
			}

			svg := generateScatterPlot(params.X, params.Y, params.Title, params.XLabel, params.YLabel, params.Width, params.Height, params.Color, params.Size)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func histogramChartTool() tool.Tool {
	return tool.NewBuilder("chart_histogram").
		WithDescription("Generate a histogram SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
				Bins   int       `json:"bins,omitempty"`
				Title  string    `json:"title,omitempty"`
				Width  int       `json:"width,omitempty"`
				Height int       `json:"height,omitempty"`
				Color  string    `json:"color,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 600
			}
			if params.Height == 0 {
				params.Height = 400
			}
			if params.Bins == 0 {
				params.Bins = 10
			}
			if params.Color == "" {
				params.Color = "#4a90d9"
			}

			svg := generateHistogram(params.Values, params.Bins, params.Title, params.Width, params.Height, params.Color)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func areaChartTool() tool.Tool {
	return tool.NewBuilder("chart_area").
		WithDescription("Generate an area chart SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Labels []string    `json:"labels"`
				Series [][]float64 `json:"series"`
				Names  []string    `json:"names,omitempty"`
				Title  string      `json:"title,omitempty"`
				Width  int         `json:"width,omitempty"`
				Height int         `json:"height,omitempty"`
				Colors []string    `json:"colors,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 600
			}
			if params.Height == 0 {
				params.Height = 400
			}
			if len(params.Colors) == 0 {
				params.Colors = defaultColors()
			}

			svg := generateAreaChart(params.Labels, params.Series, params.Names, params.Title, params.Width, params.Height, params.Colors)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func heatmapTool() tool.Tool {
	return tool.NewBuilder("chart_heatmap").
		WithDescription("Generate a heatmap SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data       [][]float64 `json:"data"`
				RowLabels  []string    `json:"row_labels,omitempty"`
				ColLabels  []string    `json:"col_labels,omitempty"`
				Title      string      `json:"title,omitempty"`
				Width      int         `json:"width,omitempty"`
				Height     int         `json:"height,omitempty"`
				ColorScale string      `json:"color_scale,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 600
			}
			if params.Height == 0 {
				params.Height = 400
			}

			svg := generateHeatmap(params.Data, params.RowLabels, params.ColLabels, params.Title, params.Width, params.Height, params.ColorScale)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func boxPlotTool() tool.Tool {
	return tool.NewBuilder("chart_box").
		WithDescription("Generate a box plot SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data   [][]float64 `json:"data"`
				Labels []string    `json:"labels,omitempty"`
				Title  string      `json:"title,omitempty"`
				Width  int         `json:"width,omitempty"`
				Height int         `json:"height,omitempty"`
				Color  string      `json:"color,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 600
			}
			if params.Height == 0 {
				params.Height = 400
			}
			if params.Color == "" {
				params.Color = "#4a90d9"
			}

			svg := generateBoxPlot(params.Data, params.Labels, params.Title, params.Width, params.Height, params.Color)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sparklineTool() tool.Tool {
	return tool.NewBuilder("chart_sparkline").
		WithDescription("Generate a compact sparkline SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
				Width  int       `json:"width,omitempty"`
				Height int       `json:"height,omitempty"`
				Color  string    `json:"color,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 100
			}
			if params.Height == 0 {
				params.Height = 30
			}
			if params.Color == "" {
				params.Color = "#4a90d9"
			}

			svg := generateSparkline(params.Values, params.Width, params.Height, params.Color)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func gaugeChartTool() tool.Tool {
	return tool.NewBuilder("chart_gauge").
		WithDescription("Generate a gauge chart SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value    float64 `json:"value"`
				Min      float64 `json:"min,omitempty"`
				Max      float64 `json:"max,omitempty"`
				Title    string  `json:"title,omitempty"`
				Width    int     `json:"width,omitempty"`
				Height   int     `json:"height,omitempty"`
				Color    string  `json:"color,omitempty"`
				ShowText bool    `json:"show_text,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 200
			}
			if params.Height == 0 {
				params.Height = 150
			}
			if params.Max == 0 {
				params.Max = 100
			}
			if params.Color == "" {
				params.Color = "#4a90d9"
			}

			svg := generateGauge(params.Value, params.Min, params.Max, params.Title, params.Width, params.Height, params.Color, params.ShowText)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func radarChartTool() tool.Tool {
	return tool.NewBuilder("chart_radar").
		WithDescription("Generate a radar/spider chart SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Labels []string    `json:"labels"`
				Values [][]float64 `json:"values"`
				Names  []string    `json:"names,omitempty"`
				Title  string      `json:"title,omitempty"`
				Width  int         `json:"width,omitempty"`
				Height int         `json:"height,omitempty"`
				Colors []string    `json:"colors,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 400
			}
			if params.Height == 0 {
				params.Height = 400
			}
			if len(params.Colors) == 0 {
				params.Colors = defaultColors()
			}

			svg := generateRadarChart(params.Labels, params.Values, params.Names, params.Title, params.Width, params.Height, params.Colors)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func funnelChartTool() tool.Tool {
	return tool.NewBuilder("chart_funnel").
		WithDescription("Generate a funnel chart SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Labels []string  `json:"labels"`
				Values []float64 `json:"values"`
				Title  string    `json:"title,omitempty"`
				Width  int       `json:"width,omitempty"`
				Height int       `json:"height,omitempty"`
				Colors []string  `json:"colors,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Width == 0 {
				params.Width = 400
			}
			if params.Height == 0 {
				params.Height = 300
			}
			if len(params.Colors) == 0 {
				params.Colors = defaultColors()
			}

			svg := generateFunnelChart(params.Labels, params.Values, params.Title, params.Width, params.Height, params.Colors)

			result := SVGChart{SVG: svg, Width: params.Width, Height: params.Height, Title: params.Title}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Helper functions for SVG generation

func defaultColors() []string {
	return []string{"#4a90d9", "#e74c3c", "#2ecc71", "#f39c12", "#9b59b6", "#1abc9c", "#34495e", "#e91e63"}
}

func generateBarChart(labels []string, values []float64, title string, width, height int, color string, horizontal bool) string {
	var sb strings.Builder

	padding := 60
	chartWidth := width - 2*padding
	chartHeight := height - 2*padding

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	if title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold">%s</text>`, width/2, title))
	}

	if len(values) == 0 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	maxVal := values[0]
	for _, v := range values[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	barCount := len(values)
	if horizontal {
		barHeight := chartHeight / barCount
		gap := barHeight / 5
		actualBarHeight := barHeight - gap

		for i, v := range values {
			barWidth := int(float64(chartWidth) * v / maxVal)
			y := padding + i*barHeight + gap/2
			sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`, padding, y, barWidth, actualBarHeight, color))
			if i < len(labels) {
				sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" text-anchor="end" font-size="12">%s</text>`, padding-5, y+actualBarHeight/2+4, labels[i]))
			}
		}
	} else {
		barWidth := chartWidth / barCount
		gap := barWidth / 5
		actualBarWidth := barWidth - gap

		for i, v := range values {
			barHeight := int(float64(chartHeight) * v / maxVal)
			x := padding + i*barWidth + gap/2
			y := padding + chartHeight - barHeight
			sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`, x, y, actualBarWidth, barHeight, color))
			if i < len(labels) {
				sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" text-anchor="middle" font-size="10">%s</text>`, x+actualBarWidth/2, height-padding+15, labels[i]))
			}
		}
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

func generateLineChart(labels []string, series [][]float64, _ []string, title string, width, height int, colors []string) string {
	var sb strings.Builder

	padding := 60
	chartWidth := width - 2*padding
	chartHeight := height - 2*padding

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	if title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold">%s</text>`, width/2, title))
	}

	if len(series) == 0 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	maxVal := 0.0
	for _, s := range series {
		for _, v := range s {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	for si, s := range series {
		if len(s) == 0 {
			continue
		}
		color := colors[si%len(colors)]

		var points []string
		divisor := len(s) - 1
		if divisor < 1 {
			divisor = 1
		}
		for i, v := range s {
			x := padding + i*chartWidth/divisor
			y := padding + chartHeight - int(float64(chartHeight)*v/maxVal)
			points = append(points, fmt.Sprintf("%d,%d", x, y))
		}

		sb.WriteString(fmt.Sprintf(`<polyline points="%s" fill="none" stroke="%s" stroke-width="2"/>`, strings.Join(points, " "), color))

		for i, v := range s {
			x := padding + i*chartWidth/divisor
			y := padding + chartHeight - int(float64(chartHeight)*v/maxVal)
			sb.WriteString(fmt.Sprintf(`<circle cx="%d" cy="%d" r="4" fill="%s"/>`, x, y, color))
		}
	}

	if len(labels) > 1 {
		divisor := len(labels) - 1
		for i, label := range labels {
			x := padding + i*chartWidth/divisor
			sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" text-anchor="middle" font-size="10">%s</text>`, x, height-padding+15, label))
		}
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

func generatePieChart(labels []string, values []float64, title string, width, height int, colors []string, donut bool) string {
	var sb strings.Builder

	cx := width / 2
	cy := height / 2
	radius := minInt(width, height)/2 - 40
	innerRadius := 0
	if donut {
		innerRadius = radius / 2
	}

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	if title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold">%s</text>`, width/2, title))
	}

	total := 0.0
	for _, v := range values {
		total += v
	}
	if total == 0 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	startAngle := -90.0
	for i, v := range values {
		if v <= 0 {
			continue
		}
		angle := 360 * v / total
		endAngle := startAngle + angle
		color := colors[i%len(colors)]

		path := describeArc(float64(cx), float64(cy), float64(radius), float64(innerRadius), startAngle, endAngle)
		sb.WriteString(fmt.Sprintf(`<path d="%s" fill="%s" stroke="white" stroke-width="2"/>`, path, color))

		startAngle = endAngle
	}

	_ = labels // Labels can be added as legend if needed

	sb.WriteString(`</svg>`)
	return sb.String()
}

func describeArc(cx, cy, outerR, innerR, startAngle, endAngle float64) string {
	startRad := startAngle * math.Pi / 180
	endRad := endAngle * math.Pi / 180

	x1 := cx + outerR*math.Cos(startRad)
	y1 := cy + outerR*math.Sin(startRad)
	x2 := cx + outerR*math.Cos(endRad)
	y2 := cy + outerR*math.Sin(endRad)

	largeArc := 0
	if endAngle-startAngle > 180 {
		largeArc = 1
	}

	if innerR > 0 {
		x3 := cx + innerR*math.Cos(endRad)
		y3 := cy + innerR*math.Sin(endRad)
		x4 := cx + innerR*math.Cos(startRad)
		y4 := cy + innerR*math.Sin(startRad)

		return fmt.Sprintf("M %f %f A %f %f 0 %d 1 %f %f L %f %f A %f %f 0 %d 0 %f %f Z",
			x1, y1, outerR, outerR, largeArc, x2, y2, x3, y3, innerR, innerR, largeArc, x4, y4)
	}

	return fmt.Sprintf("M %f %f A %f %f 0 %d 1 %f %f L %f %f Z",
		x1, y1, outerR, outerR, largeArc, x2, y2, cx, cy)
}

func generateScatterPlot(x, y []float64, title, xLabel, yLabel string, width, height int, color string, size int) string {
	var sb strings.Builder

	padding := 60
	chartWidth := width - 2*padding
	chartHeight := height - 2*padding

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	if title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold">%s</text>`, width/2, title))
	}

	if len(x) == 0 || len(y) == 0 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	minX, maxX := x[0], x[0]
	minY, maxY := y[0], y[0]
	for _, v := range x {
		if v < minX {
			minX = v
		}
		if v > maxX {
			maxX = v
		}
	}
	for _, v := range y {
		if v < minY {
			minY = v
		}
		if v > maxY {
			maxY = v
		}
	}

	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX == 0 {
		rangeX = 1
	}
	if rangeY == 0 {
		rangeY = 1
	}

	for i := range x {
		if i >= len(y) {
			break
		}
		px := padding + int(float64(chartWidth)*(x[i]-minX)/rangeX)
		py := padding + chartHeight - int(float64(chartHeight)*(y[i]-minY)/rangeY)
		sb.WriteString(fmt.Sprintf(`<circle cx="%d" cy="%d" r="%d" fill="%s" fill-opacity="0.7"/>`, px, py, size, color))
	}

	if xLabel != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" text-anchor="middle" font-size="12">%s</text>`, width/2, height-10, xLabel))
	}
	if yLabel != "" {
		sb.WriteString(fmt.Sprintf(`<text x="15" y="%d" text-anchor="middle" font-size="12" transform="rotate(-90, 15, %d)">%s</text>`, height/2, height/2, yLabel))
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

func generateHistogram(values []float64, bins int, title string, width, height int, color string) string {
	if len(values) == 0 {
		return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d"><rect width="%d" height="%d" fill="white"/></svg>`, width, height, width, height)
	}

	minVal, maxVal := values[0], values[0]
	for _, v := range values[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	binWidth := (maxVal - minVal) / float64(bins)
	if binWidth == 0 {
		binWidth = 1
	}

	counts := make([]int, bins)
	for _, v := range values {
		bin := int((v - minVal) / binWidth)
		if bin >= bins {
			bin = bins - 1
		}
		if bin < 0 {
			bin = 0
		}
		counts[bin]++
	}

	floatCounts := make([]float64, len(counts))
	labels := make([]string, len(counts))
	for i, c := range counts {
		floatCounts[i] = float64(c)
		labels[i] = fmt.Sprintf("%.1f", minVal+float64(i)*binWidth)
	}

	return generateBarChart(labels, floatCounts, title, width, height, color, false)
}

func generateAreaChart(labels []string, series [][]float64, _ []string, title string, width, height int, colors []string) string {
	var sb strings.Builder

	padding := 60
	chartWidth := width - 2*padding
	chartHeight := height - 2*padding

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	if title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold">%s</text>`, width/2, title))
	}

	if len(series) == 0 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	maxVal := 0.0
	for _, s := range series {
		for _, v := range s {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	baseline := padding + chartHeight
	_ = labels

	for si := len(series) - 1; si >= 0; si-- {
		s := series[si]
		if len(s) == 0 {
			continue
		}
		color := colors[si%len(colors)]

		divisor := len(s) - 1
		if divisor < 1 {
			divisor = 1
		}

		var pathPoints []string
		pathPoints = append(pathPoints, fmt.Sprintf("M %d %d", padding, baseline))

		for i, v := range s {
			x := padding + i*chartWidth/divisor
			y := padding + chartHeight - int(float64(chartHeight)*v/maxVal)
			pathPoints = append(pathPoints, fmt.Sprintf("L %d %d", x, y))
		}

		pathPoints = append(pathPoints, fmt.Sprintf("L %d %d Z", padding+chartWidth, baseline))

		sb.WriteString(fmt.Sprintf(`<path d="%s" fill="%s" fill-opacity="0.5" stroke="%s" stroke-width="2"/>`, strings.Join(pathPoints, " "), color, color))
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

func generateHeatmap(data [][]float64, _, _ []string, title string, width, height int, colorScale string) string {
	var sb strings.Builder

	padding := 60
	chartWidth := width - 2*padding
	chartHeight := height - 2*padding

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	if title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold">%s</text>`, width/2, title))
	}

	if len(data) == 0 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	rows := len(data)
	cols := 0
	for _, row := range data {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	cellWidth := chartWidth / cols
	cellHeight := chartHeight / rows

	minVal, maxVal := data[0][0], data[0][0]
	for _, row := range data {
		for _, v := range row {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
	}
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
	}

	for i, row := range data {
		for j, v := range row {
			intensity := (v - minVal) / valRange
			color := getHeatmapColor(intensity, colorScale)
			x := padding + j*cellWidth
			y := padding + i*cellHeight
			sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`, x, y, cellWidth, cellHeight, color))
		}
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

func getHeatmapColor(intensity float64, scale string) string {
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}

	switch scale {
	case "hot":
		r := minInt(255, int(255*intensity*2))
		g := maxInt(0, minInt(255, int(255*(intensity*2-1))))
		b := 0
		return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
	case "cool":
		r := int(255 * (1 - intensity))
		g := int(255 * (1 - intensity))
		b := 255
		return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
	default:
		r := int(68 + 187*intensity)
		g := int(1 + 180*intensity - 100*intensity*intensity)
		b := int(84 + 100*(1-intensity))
		return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
	}
}

func generateBoxPlot(data [][]float64, labels []string, title string, width, height int, color string) string {
	var sb strings.Builder

	padding := 60
	chartWidth := width - 2*padding
	chartHeight := height - 2*padding

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	if title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold">%s</text>`, width/2, title))
	}

	if len(data) == 0 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	globalMin, globalMax := data[0][0], data[0][0]
	for _, d := range data {
		for _, v := range d {
			if v < globalMin {
				globalMin = v
			}
			if v > globalMax {
				globalMax = v
			}
		}
	}
	valRange := globalMax - globalMin
	if valRange == 0 {
		valRange = 1
	}

	boxWidth := chartWidth / len(data)
	actualBoxWidth := boxWidth * 2 / 3

	for i, d := range data {
		if len(d) == 0 {
			continue
		}

		sorted := make([]float64, len(d))
		copy(sorted, d)
		sortFloat64s(sorted)

		q1 := percentile(sorted, 25)
		q2 := percentile(sorted, 50)
		q3 := percentile(sorted, 75)
		minVal := sorted[0]
		maxVal := sorted[len(sorted)-1]

		cx := padding + i*boxWidth + boxWidth/2

		y1 := padding + chartHeight - int(float64(chartHeight)*(q1-globalMin)/valRange)
		y2 := padding + chartHeight - int(float64(chartHeight)*(q2-globalMin)/valRange)
		y3 := padding + chartHeight - int(float64(chartHeight)*(q3-globalMin)/valRange)
		yMin := padding + chartHeight - int(float64(chartHeight)*(minVal-globalMin)/valRange)
		yMax := padding + chartHeight - int(float64(chartHeight)*(maxVal-globalMin)/valRange)

		sb.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s"/>`, cx, yMin, cx, y1, color))
		sb.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s"/>`, cx, y3, cx, yMax, color))

		sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s" fill-opacity="0.5" stroke="%s"/>`, cx-actualBoxWidth/2, y3, actualBoxWidth, y1-y3, color, color))

		sb.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2"/>`, cx-actualBoxWidth/2, y2, cx+actualBoxWidth/2, y2, color))

		capWidth := actualBoxWidth / 2
		sb.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s"/>`, cx-capWidth/2, yMin, cx+capWidth/2, yMin, color))
		sb.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s"/>`, cx-capWidth/2, yMax, cx+capWidth/2, yMax, color))

		if i < len(labels) {
			sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" text-anchor="middle" font-size="10">%s</text>`, cx, height-padding+15, labels[i]))
		}
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

func generateSparkline(values []float64, width, height int, color string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))

	if len(values) < 2 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	minVal, maxVal := values[0], values[0]
	for _, v := range values[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
	}

	var points []string
	for i, v := range values {
		x := i * width / (len(values) - 1)
		y := height - int(float64(height)*(v-minVal)/valRange)
		points = append(points, fmt.Sprintf("%d,%d", x, y))
	}

	sb.WriteString(fmt.Sprintf(`<polyline points="%s" fill="none" stroke="%s" stroke-width="1.5"/>`, strings.Join(points, " "), color))
	sb.WriteString(`</svg>`)
	return sb.String()
}

func generateGauge(value, minVal, maxVal float64, title string, width, height int, color string, showText bool) string {
	var sb strings.Builder

	cx := width / 2
	cy := height - 20
	radius := minInt(width/2-10, height-30)

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	sb.WriteString(fmt.Sprintf(`<path d="M %d %d A %d %d 0 0 1 %d %d" fill="none" stroke="#e0e0e0" stroke-width="10"/>`,
		cx-radius, cy, radius, radius, cx+radius, cy))

	pct := (value - minVal) / (maxVal - minVal)
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	angle := 180 * pct
	endX := float64(cx) + float64(radius)*math.Cos(math.Pi*(1-pct))
	endY := float64(cy) - float64(radius)*math.Sin(math.Pi*pct)

	largeArc := 0
	if angle > 90 {
		largeArc = 1
	}

	sb.WriteString(fmt.Sprintf(`<path d="M %d %d A %d %d 0 %d 1 %.1f %.1f" fill="none" stroke="%s" stroke-width="10"/>`,
		cx-radius, cy, radius, radius, largeArc, endX, endY, color))

	if showText || title != "" {
		if title != "" {
			sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" text-anchor="middle" font-size="12">%s</text>`, cx, cy-radius/2, title))
		}
		if showText {
			sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" text-anchor="middle" font-size="16" font-weight="bold">%.1f</text>`, cx, cy, value))
		}
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

func generateRadarChart(labels []string, values [][]float64, _ []string, title string, width, height int, colors []string) string {
	var sb strings.Builder

	cx := width / 2
	cy := height / 2
	radius := minInt(width, height)/2 - 50

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	if title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold">%s</text>`, width/2, title))
	}

	n := len(labels)
	if n < 3 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	angleStep := 2 * math.Pi / float64(n)

	for level := 1; level <= 5; level++ {
		r := float64(radius) * float64(level) / 5
		var points []string
		for i := 0; i < n; i++ {
			angle := -math.Pi/2 + float64(i)*angleStep
			x := float64(cx) + r*math.Cos(angle)
			y := float64(cy) + r*math.Sin(angle)
			points = append(points, fmt.Sprintf("%.1f,%.1f", x, y))
		}
		sb.WriteString(fmt.Sprintf(`<polygon points="%s" fill="none" stroke="#ddd"/>`, strings.Join(points, " ")))
	}

	for i := 0; i < n; i++ {
		angle := -math.Pi/2 + float64(i)*angleStep
		x := float64(cx) + float64(radius)*math.Cos(angle)
		y := float64(cy) + float64(radius)*math.Sin(angle)
		sb.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#ddd"/>`, cx, cy, x, y))

		labelX := float64(cx) + float64(radius+15)*math.Cos(angle)
		labelY := float64(cy) + float64(radius+15)*math.Sin(angle)
		anchor := "middle"
		if angle < -math.Pi/4 && angle > -3*math.Pi/4 {
			anchor = "middle"
		} else if angle >= -math.Pi/4 || angle <= -3*math.Pi/4 {
			if math.Cos(angle) > 0 {
				anchor = "start"
			} else {
				anchor = "end"
			}
		}
		sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" text-anchor="%s" font-size="10">%s</text>`, labelX, labelY+4, anchor, labels[i]))
	}

	for si, vals := range values {
		if len(vals) != n {
			continue
		}
		color := colors[si%len(colors)]

		var points []string
		for i, v := range vals {
			angle := -math.Pi/2 + float64(i)*angleStep
			r := float64(radius) * v / 100
			x := float64(cx) + r*math.Cos(angle)
			y := float64(cy) + r*math.Sin(angle)
			points = append(points, fmt.Sprintf("%.1f,%.1f", x, y))
		}
		sb.WriteString(fmt.Sprintf(`<polygon points="%s" fill="%s" fill-opacity="0.3" stroke="%s" stroke-width="2"/>`, strings.Join(points, " "), color, color))
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

func generateFunnelChart(labels []string, values []float64, title string, width, height int, colors []string) string {
	var sb strings.Builder

	padding := 40
	chartWidth := width - 2*padding
	chartHeight := height - 2*padding

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width, height))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, width, height))

	if title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" text-anchor="middle" font-size="16" font-weight="bold">%s</text>`, width/2, title))
	}

	if len(values) == 0 {
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	maxVal := values[0]
	for _, v := range values[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	n := len(values)
	sectionHeight := chartHeight / n
	centerX := width / 2

	for i, v := range values {
		color := colors[i%len(colors)]
		widthRatio := v / maxVal
		sectionWidth := int(float64(chartWidth) * widthRatio)

		y := padding + i*sectionHeight
		x1 := centerX - sectionWidth/2
		x2 := centerX + sectionWidth/2

		nextWidth := sectionWidth
		if i < n-1 {
			nextWidth = int(float64(chartWidth) * values[i+1] / maxVal)
		}
		x3 := centerX + nextWidth/2
		x4 := centerX - nextWidth/2

		sb.WriteString(fmt.Sprintf(`<polygon points="%d,%d %d,%d %d,%d %d,%d" fill="%s" stroke="white"/>`,
			x1, y, x2, y, x3, y+sectionHeight, x4, y+sectionHeight, color))

		if i < len(labels) {
			sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" text-anchor="middle" font-size="11" fill="white">%s</text>`,
				centerX, y+sectionHeight/2+4, labels[i]))
		}
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

func sortFloat64s(a []float64) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p / 100.0) * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	return sorted[lower] + (idx-float64(lower))*(sorted[upper]-sorted[lower])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
