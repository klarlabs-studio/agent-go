// Package metrics provides metric collection tools for agents.
package metrics

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// MetricStore manages metrics.
type MetricStore struct {
	mu       sync.RWMutex
	counters map[string]*counter
	gauges   map[string]*gauge
	histos   map[string]*histogram
}

type counter struct {
	Value     int64             `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type gauge struct {
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type histogram struct {
	Values    []float64         `json:"values"`
	Labels    map[string]string `json:"labels,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

var store = &MetricStore{
	counters: make(map[string]*counter),
	gauges:   make(map[string]*gauge),
	histos:   make(map[string]*histogram),
}

// Pack returns the metrics tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("metrics").
		WithDescription("Metric collection tools").
		AddTools(
			counterIncTool(),
			counterGetTool(),
			gaugeSetTool(),
			gaugeGetTool(),
			histoRecordTool(),
			histoStatsTool(),
			listTool(),
			deleteTool(),
			resetTool(),
			exportTool(),
			summaryTool(),
			timingTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func counterIncTool() tool.Tool {
	return tool.NewBuilder("metrics_counter_inc").
		WithDescription("Increment a counter metric").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name   string            `json:"name"`
				Value  int64             `json:"value,omitempty"`
				Labels map[string]string `json:"labels,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			inc := params.Value
			if inc == 0 {
				inc = 1
			}

			store.mu.Lock()
			c, ok := store.counters[params.Name]
			if !ok {
				c = &counter{
					Labels: params.Labels,
				}
				store.counters[params.Name] = c
			}
			c.Value += inc
			c.UpdatedAt = time.Now()
			value := c.Value
			store.mu.Unlock()

			result := map[string]any{
				"name":  params.Name,
				"value": value,
				"inc":   inc,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func counterGetTool() tool.Tool {
	return tool.NewBuilder("metrics_counter_get").
		WithDescription("Get counter value").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			c, ok := store.counters[params.Name]
			store.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"name":  params.Name,
					"found": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"name":       params.Name,
				"found":      true,
				"value":      c.Value,
				"labels":     c.Labels,
				"updated_at": c.UpdatedAt,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func gaugeSetTool() tool.Tool {
	return tool.NewBuilder("metrics_gauge_set").
		WithDescription("Set a gauge metric").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name   string            `json:"name"`
				Value  float64           `json:"value"`
				Labels map[string]string `json:"labels,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			g, ok := store.gauges[params.Name]
			if !ok {
				g = &gauge{
					Labels: params.Labels,
				}
				store.gauges[params.Name] = g
			}
			g.Value = params.Value
			g.UpdatedAt = time.Now()
			store.mu.Unlock()

			result := map[string]any{
				"name":  params.Name,
				"value": params.Value,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func gaugeGetTool() tool.Tool {
	return tool.NewBuilder("metrics_gauge_get").
		WithDescription("Get gauge value").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			g, ok := store.gauges[params.Name]
			store.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"name":  params.Name,
					"found": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"name":       params.Name,
				"found":      true,
				"value":      g.Value,
				"labels":     g.Labels,
				"updated_at": g.UpdatedAt,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func histoRecordTool() tool.Tool {
	return tool.NewBuilder("metrics_histo_record").
		WithDescription("Record a histogram value").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name   string            `json:"name"`
				Value  float64           `json:"value"`
				Labels map[string]string `json:"labels,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			h, ok := store.histos[params.Name]
			if !ok {
				h = &histogram{
					Values: make([]float64, 0),
					Labels: params.Labels,
				}
				store.histos[params.Name] = h
			}
			h.Values = append(h.Values, params.Value)
			h.UpdatedAt = time.Now()
			count := len(h.Values)
			store.mu.Unlock()

			result := map[string]any{
				"name":  params.Name,
				"value": params.Value,
				"count": count,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func histoStatsTool() tool.Tool {
	return tool.NewBuilder("metrics_histo_stats").
		WithDescription("Get histogram statistics").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name        string    `json:"name"`
				Percentiles []float64 `json:"percentiles,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			h, ok := store.histos[params.Name]
			if !ok {
				store.mu.RUnlock()
				result := map[string]any{
					"name":  params.Name,
					"found": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			values := make([]float64, len(h.Values))
			copy(values, h.Values)
			store.mu.RUnlock()

			if len(values) == 0 {
				result := map[string]any{
					"name":  params.Name,
					"found": true,
					"count": 0,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Calculate statistics
			sort.Float64s(values)
			count := len(values)

			var sum float64
			for _, v := range values {
				sum += v
			}
			mean := sum / float64(count)

			var variance float64
			for _, v := range values {
				variance += (v - mean) * (v - mean)
			}
			variance /= float64(count)
			stddev := math.Sqrt(variance)

			min := values[0]
			max := values[count-1]

			// Calculate percentiles
			percentiles := params.Percentiles
			if len(percentiles) == 0 {
				percentiles = []float64{50, 90, 95, 99}
			}

			pctValues := make(map[string]float64)
			for _, p := range percentiles {
				idx := int(float64(count-1) * p / 100)
				if idx >= count {
					idx = count - 1
				}
				pctValues[json.Number(json.Number(string(rune('0'+int(p)/10)))).String()] = values[idx]
			}

			result := map[string]any{
				"name":        params.Name,
				"found":       true,
				"count":       count,
				"sum":         sum,
				"mean":        mean,
				"stddev":      stddev,
				"min":         min,
				"max":         max,
				"percentiles": pctValues,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("metrics_list").
		WithDescription("List all metrics").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Type string `json:"type,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			var metrics []map[string]any

			if params.Type == "" || params.Type == "counter" {
				for name, c := range store.counters {
					metrics = append(metrics, map[string]any{
						"name":  name,
						"type":  "counter",
						"value": c.Value,
					})
				}
			}

			if params.Type == "" || params.Type == "gauge" {
				for name, g := range store.gauges {
					metrics = append(metrics, map[string]any{
						"name":  name,
						"type":  "gauge",
						"value": g.Value,
					})
				}
			}

			if params.Type == "" || params.Type == "histogram" {
				for name, h := range store.histos {
					metrics = append(metrics, map[string]any{
						"name":  name,
						"type":  "histogram",
						"count": len(h.Values),
					})
				}
			}
			store.mu.RUnlock()

			result := map[string]any{
				"metrics": metrics,
				"count":   len(metrics),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deleteTool() tool.Tool {
	return tool.NewBuilder("metrics_delete").
		WithDescription("Delete a metric").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name string `json:"name"`
				Type string `json:"type,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			deleted := false

			if params.Type == "" || params.Type == "counter" {
				if _, ok := store.counters[params.Name]; ok {
					delete(store.counters, params.Name)
					deleted = true
				}
			}

			if params.Type == "" || params.Type == "gauge" {
				if _, ok := store.gauges[params.Name]; ok {
					delete(store.gauges, params.Name)
					deleted = true
				}
			}

			if params.Type == "" || params.Type == "histogram" {
				if _, ok := store.histos[params.Name]; ok {
					delete(store.histos, params.Name)
					deleted = true
				}
			}
			store.mu.Unlock()

			result := map[string]any{
				"name":    params.Name,
				"deleted": deleted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func resetTool() tool.Tool {
	return tool.NewBuilder("metrics_reset").
		WithDescription("Reset all metrics").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			store.mu.Lock()
			counters := len(store.counters)
			gauges := len(store.gauges)
			histos := len(store.histos)

			store.counters = make(map[string]*counter)
			store.gauges = make(map[string]*gauge)
			store.histos = make(map[string]*histogram)
			store.mu.Unlock()

			result := map[string]any{
				"counters_cleared":   counters,
				"gauges_cleared":     gauges,
				"histograms_cleared": histos,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func exportTool() tool.Tool {
	return tool.NewBuilder("metrics_export").
		WithDescription("Export metrics in various formats").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Format string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			format := params.Format
			if format == "" {
				format = "json"
			}

			store.mu.RLock()
			data := map[string]any{
				"counters":   store.counters,
				"gauges":     store.gauges,
				"histograms": store.histos,
				"timestamp":  time.Now(),
			}
			store.mu.RUnlock()

			result := map[string]any{
				"format": format,
				"data":   data,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func summaryTool() tool.Tool {
	return tool.NewBuilder("metrics_summary").
		WithDescription("Get metrics summary").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			store.mu.RLock()
			numCounters := len(store.counters)
			numGauges := len(store.gauges)
			numHistos := len(store.histos)

			var totalHistoValues int
			for _, h := range store.histos {
				totalHistoValues += len(h.Values)
			}
			store.mu.RUnlock()

			result := map[string]any{
				"counters":           numCounters,
				"gauges":             numGauges,
				"histograms":         numHistos,
				"total_metrics":      numCounters + numGauges + numHistos,
				"total_histo_values": totalHistoValues,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func timingTool() tool.Tool {
	return tool.NewBuilder("metrics_timing").
		WithDescription("Record a timing metric").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name     string            `json:"name"`
				Duration float64           `json:"duration_ms"`
				Labels   map[string]string `json:"labels,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Record as histogram
			store.mu.Lock()
			h, ok := store.histos[params.Name]
			if !ok {
				h = &histogram{
					Values: make([]float64, 0),
					Labels: params.Labels,
				}
				store.histos[params.Name] = h
			}
			h.Values = append(h.Values, params.Duration)
			h.UpdatedAt = time.Now()
			count := len(h.Values)
			store.mu.Unlock()

			result := map[string]any{
				"name":        params.Name,
				"duration_ms": params.Duration,
				"count":       count,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
