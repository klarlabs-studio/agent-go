package pattern

import (
	"context"
	"fmt"
	"math"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/run"
)

// CostAnomalyDetector detects unusual cost spikes and trends.
type CostAnomalyDetector struct {
	eventStore         event.Store
	runStore           run.Store
	deviationThreshold float64  // Standard deviations to consider anomalous
	minSampleSize      int      // Minimum runs needed for baseline
	costTypes          []string // Cost types to track
}

// CostAnomalyOption configures the cost anomaly detector.
type CostAnomalyOption func(*CostAnomalyDetector)

// WithDeviationThreshold sets the deviation threshold.
func WithDeviationThreshold(d float64) CostAnomalyOption {
	return func(c *CostAnomalyDetector) {
		c.deviationThreshold = d
	}
}

// WithMinSampleSize sets the minimum sample size for baseline.
func WithMinSampleSize(n int) CostAnomalyOption {
	return func(c *CostAnomalyDetector) {
		c.minSampleSize = n
	}
}

// WithCostTypes sets the cost types to track.
func WithCostTypes(types []string) CostAnomalyOption {
	return func(c *CostAnomalyDetector) {
		c.costTypes = types
	}
}

// NewCostAnomalyDetector creates a new cost anomaly detector.
func NewCostAnomalyDetector(eventStore event.Store, runStore run.Store, opts ...CostAnomalyOption) *CostAnomalyDetector {
	d := &CostAnomalyDetector{
		eventStore:         eventStore,
		runStore:           runStore,
		deviationThreshold: 2.0, // 2 standard deviations
		minSampleSize:      5,
		costTypes:          []string{"tool_calls", "tokens", "api_calls"},
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Detect finds cost anomalies across runs.
func (d *CostAnomalyDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
	// Get runs matching filter
	runs, err := d.runStore.List(ctx, run.ListFilter{
		FromTime: opts.FromTime,
		ToTime:   opts.ToTime,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}

	// Filter by run IDs if specified
	if len(opts.RunIDs) > 0 {
		filtered := make([]*agent.Run, 0)
		runIDSet := make(map[string]bool)
		for _, id := range opts.RunIDs {
			runIDSet[id] = true
		}
		for _, r := range runs {
			if runIDSet[r.ID] {
				filtered = append(filtered, r)
			}
		}
		runs = filtered
	}

	if len(runs) < d.minSampleSize {
		return nil, nil // Not enough data for meaningful detection
	}

	// Calculate costs per run for each cost type
	costsByType := make(map[string][]runCost)
	for _, costType := range d.costTypes {
		costsByType[costType] = make([]runCost, 0)
	}

	for _, r := range runs {
		events, err := d.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		costs := calculateRunCosts(r.ID, events)
		for costType, cost := range costs {
			if _, ok := costsByType[costType]; ok {
				costsByType[costType] = append(costsByType[costType], runCost{
					runID: r.ID,
					cost:  cost,
				})
			}
		}
	}

	// Detect anomalies for each cost type
	var patterns []pattern.Pattern
	for costType, costs := range costsByType {
		if len(costs) < d.minSampleSize {
			continue
		}

		anomalies := d.detectAnomalies(costs)
		if len(anomalies) == 0 {
			continue
		}

		// Calculate statistics
		values := make([]float64, len(costs))
		for i, c := range costs {
			values[i] = c.cost
		}
		avgCost := mean(values)
		trend := calculateTrend(costs)

		// Calculate average anomaly cost
		var anomalyCostSum float64
		var maxDeviation float64
		for _, a := range anomalies {
			anomalyCostSum += a.cost
			if a.deviation > maxDeviation {
				maxDeviation = a.deviation
			}
		}
		avgAnomalyCost := anomalyCostSum / float64(len(anomalies))

		// Calculate confidence based on deviation strength
		confidence := calculateAnomalyConfidence(anomalies, d.deviationThreshold)
		if opts.MinConfidence > 0 && confidence < opts.MinConfidence {
			continue
		}

		p := pattern.NewPattern(
			pattern.PatternTypeCostAnomaly,
			fmt.Sprintf("Cost Anomaly: %s", costType),
			fmt.Sprintf("Detected %d anomalous runs with avg cost %.2f (baseline: %.2f), trend: %s",
				len(anomalies), avgAnomalyCost, avgCost, trend),
		)
		p.Confidence = confidence
		p.Frequency = len(anomalies)

		data := pattern.CostAnomalyData{
			CostType:     costType,
			AverageCost:  avgCost,
			AnomalyCost:  avgAnomalyCost,
			Deviation:    maxDeviation,
			AnomalyCount: len(anomalies),
			TrendDir:     trend,
		}
		if err := p.SetData(data); err != nil {
			continue
		}

		// Add evidence
		for _, a := range anomalies {
			if err := p.AddEvidence(a.runID, map[string]any{
				"cost":      a.cost,
				"deviation": a.deviation,
			}); err != nil {
				continue
			}
		}

		patterns = append(patterns, *p)

		if opts.Limit > 0 && len(patterns) >= opts.Limit {
			break
		}
	}

	return patterns, nil
}

// Types returns the pattern types this detector can find.
func (d *CostAnomalyDetector) Types() []pattern.PatternType {
	return []pattern.PatternType{pattern.PatternTypeCostAnomaly}
}

type runCost struct {
	runID string
	cost  float64
}

type anomaly struct {
	runID     string
	cost      float64
	deviation float64
}

func calculateRunCosts(runID string, events []event.Event) map[string]float64 {
	costs := make(map[string]float64)

	var toolCalls int
	var tokens int
	var apiCalls int

	for _, e := range events {
		switch e.Type {
		case event.TypeToolCalled:
			toolCalls++
		case event.TypeToolSucceeded:
			// Tool calls that succeeded count as API calls
			apiCalls++
		case event.TypeToolFailed:
			// Failed tools also count as API calls
			apiCalls++
		}
	}

	costs["tool_calls"] = float64(toolCalls)
	costs["tokens"] = float64(tokens)
	costs["api_calls"] = float64(apiCalls)

	return costs
}

func (d *CostAnomalyDetector) detectAnomalies(costs []runCost) []anomaly {
	if len(costs) < 3 {
		return nil
	}

	// Calculate mean and standard deviation
	values := make([]float64, len(costs))
	for i, c := range costs {
		values[i] = c.cost
	}

	avg := mean(values)
	std := stddev(values, avg)

	if std == 0 {
		return nil // No variance, no anomalies
	}

	// Find anomalies
	var anomalies []anomaly
	for _, c := range costs {
		deviation := (c.cost - avg) / std
		if math.Abs(deviation) >= d.deviationThreshold {
			anomalies = append(anomalies, anomaly{
				runID:     c.runID,
				cost:      c.cost,
				deviation: deviation,
			})
		}
	}

	return anomalies
}

func calculateTrend(costs []runCost) string {
	if len(costs) < 3 {
		return "stable"
	}

	// Simple linear regression to determine trend
	n := float64(len(costs))
	var sumX, sumY, sumXY, sumX2 float64

	for i, c := range costs {
		x := float64(i)
		y := c.cost
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

	// Determine trend based on slope relative to mean
	avgCost := sumY / n
	relativeSlope := slope / avgCost

	switch {
	case relativeSlope > 0.05:
		return "increasing"
	case relativeSlope < -0.05:
		return "decreasing"
	default:
		return "stable"
	}
}

func calculateAnomalyConfidence(anomalies []anomaly, threshold float64) float64 {
	if len(anomalies) == 0 {
		return 0
	}

	// Higher confidence for stronger deviations
	var totalDeviation float64
	for _, a := range anomalies {
		totalDeviation += math.Abs(a.deviation)
	}
	avgDeviation := totalDeviation / float64(len(anomalies))

	// Confidence scales with deviation strength
	confidence := 0.5 + (avgDeviation-threshold)*0.15
	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return confidence
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stddev(values []float64, avg float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var sumSquares float64
	for _, v := range values {
		diff := v - avg
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)-1))
}

// Ensure CostAnomalyDetector implements Detector
var _ pattern.Detector = (*CostAnomalyDetector)(nil)
