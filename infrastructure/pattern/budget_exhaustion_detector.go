// Package pattern provides pattern detection implementations.
package pattern

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/pattern"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

// BudgetExhaustionDetector provides detailed analysis of budget exhaustion patterns.
// This detector extends beyond basic detection to provide:
// - Per-budget analysis
// - Trend detection (increasing/decreasing exhaustion rates)
// - Early warning for runs approaching budget limits
// - Recommendations for budget adjustments
type BudgetExhaustionDetector struct {
	eventStore     event.Store
	runStore       run.Store
	minOccurrences int
	warningRatio   float64 // Warn when budget usage exceeds this ratio (e.g., 0.8 = 80%)
}

// BudgetExhaustionOption configures the budget exhaustion detector.
type BudgetExhaustionOption func(*BudgetExhaustionDetector)

// WithBudgetMinOccurrences sets the minimum occurrences for pattern detection.
func WithBudgetMinOccurrences(n int) BudgetExhaustionOption {
	return func(d *BudgetExhaustionDetector) {
		d.minOccurrences = n
	}
}

// WithWarningRatio sets the budget usage ratio that triggers warnings.
func WithWarningRatio(ratio float64) BudgetExhaustionOption {
	return func(d *BudgetExhaustionDetector) {
		d.warningRatio = ratio
	}
}

// NewBudgetExhaustionDetector creates a new budget exhaustion detector.
func NewBudgetExhaustionDetector(eventStore event.Store, runStore run.Store, opts ...BudgetExhaustionOption) *BudgetExhaustionDetector {
	d := &BudgetExhaustionDetector{
		eventStore:     eventStore,
		runStore:       runStore,
		minOccurrences: 2,
		warningRatio:   0.8,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// budgetExhaustionEvent represents a budget exhaustion occurrence.
type budgetExhaustionEvent struct {
	runID      string
	timestamp  time.Time
	budgetName string
}

// budgetUsageStats tracks usage statistics for a specific budget.
type budgetUsageStats struct {
	name            string
	exhaustionCount int
	nearMissCount   int // Runs that used > warningRatio but didn't exhaust
	avgUsageRatio   float64
	totalRuns       int
	events          []budgetExhaustionEvent
}

// Detect finds budget exhaustion patterns across runs.
func (d *BudgetExhaustionDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
	runs, err := d.getRuns(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Collect budget events by budget name
	budgetStats := make(map[string]*budgetUsageStats)

	for _, r := range runs {
		events, err := d.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		d.processRunEvents(r.ID, events, budgetStats)
	}

	// Generate patterns from collected stats
	return d.generatePatterns(budgetStats, opts)
}

func (d *BudgetExhaustionDetector) getRuns(ctx context.Context, opts pattern.DetectionOptions) ([]*agent.Run, error) {
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
		return filtered, nil
	}

	return runs, nil
}

func (d *BudgetExhaustionDetector) processRunEvents(runID string, events []event.Event, stats map[string]*budgetUsageStats) {
	// Track budget consumption to estimate usage ratios
	budgetUsage := make(map[string]int) // budget name -> total consumed

	for _, e := range events {
		switch e.Type {
		case event.TypeBudgetExhausted:
			var payload event.BudgetExhaustedPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				d.recordExhaustion(stats, budgetExhaustionEvent{
					runID:      runID,
					timestamp:  e.Timestamp,
					budgetName: payload.BudgetName,
				})
			}

		case event.TypeBudgetConsumed:
			// Track consumption for usage analysis
			var payload event.BudgetConsumedPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				budgetUsage[payload.BudgetName] += payload.Amount
				// Use remaining to estimate usage ratio
				if payload.Remaining >= 0 {
					total := budgetUsage[payload.BudgetName] + payload.Remaining
					if total > 0 {
						ratio := float64(budgetUsage[payload.BudgetName]) / float64(total)
						d.updateUsageStats(stats, payload.BudgetName, ratio, runID)
					}
				}
			}
		}
	}
}

func (d *BudgetExhaustionDetector) recordExhaustion(stats map[string]*budgetUsageStats, evt budgetExhaustionEvent) {
	name := evt.budgetName
	if name == "" {
		name = "default"
	}

	if _, ok := stats[name]; !ok {
		stats[name] = &budgetUsageStats{
			name:   name,
			events: make([]budgetExhaustionEvent, 0),
		}
	}

	s := stats[name]
	s.exhaustionCount++
	s.totalRuns++
	s.events = append(s.events, evt)
}

func (d *BudgetExhaustionDetector) updateUsageStats(stats map[string]*budgetUsageStats, name string, ratio float64, runID string) {
	if name == "" {
		name = "default"
	}

	if _, ok := stats[name]; !ok {
		stats[name] = &budgetUsageStats{
			name:   name,
			events: make([]budgetExhaustionEvent, 0),
		}
	}

	s := stats[name]

	// Update average usage ratio
	s.avgUsageRatio = (s.avgUsageRatio*float64(s.totalRuns) + ratio) / float64(s.totalRuns+1)
	s.totalRuns++

	// Track near-misses (high usage but not exhausted)
	if ratio >= d.warningRatio && ratio < 1.0 {
		s.nearMissCount++
	}
}

func (d *BudgetExhaustionDetector) generatePatterns(stats map[string]*budgetUsageStats, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
	var patterns []pattern.Pattern

	for _, s := range stats {
		if s.exhaustionCount < d.minOccurrences {
			continue
		}

		confidence := d.calculateConfidence(s)
		if opts.MinConfidence > 0 && confidence < opts.MinConfidence {
			continue
		}

		p := d.createPattern(s, confidence)
		patterns = append(patterns, *p)

		if opts.Limit > 0 && len(patterns) >= opts.Limit {
			break
		}
	}

	// Sort by exhaustion count (most impactful first)
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Frequency > patterns[j].Frequency
	})

	return patterns, nil
}

func (d *BudgetExhaustionDetector) calculateConfidence(s *budgetUsageStats) float64 {
	if s.totalRuns == 0 {
		return 0
	}

	// Base confidence on exhaustion rate
	exhaustionRate := float64(s.exhaustionCount) / float64(s.totalRuns)

	// Boost confidence if there are also near-misses
	nearMissBonus := 0.0
	if s.nearMissCount > 0 {
		nearMissBonus = 0.1 * float64(s.nearMissCount) / float64(s.totalRuns)
	}

	// Boost confidence for more occurrences
	occurrenceBonus := float64(s.exhaustionCount) * 0.05
	if occurrenceBonus > 0.2 {
		occurrenceBonus = 0.2
	}

	confidence := 0.5 + exhaustionRate*0.3 + nearMissBonus + occurrenceBonus
	if confidence > 0.98 {
		confidence = 0.98
	}

	return confidence
}

func (d *BudgetExhaustionDetector) createPattern(s *budgetUsageStats, confidence float64) *pattern.Pattern {
	// Determine trend
	trend := d.detectTrend(s.events)

	description := fmt.Sprintf(
		"Budget '%s' exhausted %d times across %d runs (%.1f%% exhaustion rate)",
		s.name, s.exhaustionCount, s.totalRuns,
		float64(s.exhaustionCount)/float64(s.totalRuns)*100,
	)

	if trend != "stable" {
		description += fmt.Sprintf(", trend: %s", trend)
	}

	if s.nearMissCount > 0 {
		description += fmt.Sprintf(", %d near-misses", s.nearMissCount)
	}

	p := pattern.NewPattern(
		pattern.PatternTypeBudgetExhaustion,
		fmt.Sprintf("Budget Exhaustion: %s", s.name),
		description,
	)
	p.Confidence = confidence

	// Set extended data
	data := BudgetExhaustionExtendedData{
		BudgetName:      s.name,
		ExhaustionCount: s.exhaustionCount,
		NearMissCount:   s.nearMissCount,
		TotalRuns:       s.totalRuns,
		ExhaustionRate:  float64(s.exhaustionCount) / float64(s.totalRuns),
		AvgUsageRatio:   s.avgUsageRatio,
		Trend:           trend,
		Recommendation:  d.generateRecommendation(s, trend),
	}
	_ = p.SetData(data)

	// Add evidence from events
	for _, evt := range s.events {
		_ = p.AddEvidence(evt.runID, map[string]any{
			"timestamp":   evt.timestamp,
			"budget_name": evt.budgetName,
		})
	}

	return p
}

func (d *BudgetExhaustionDetector) detectTrend(events []budgetExhaustionEvent) string {
	if len(events) < 3 {
		return "stable"
	}

	// Sort by timestamp
	sorted := make([]budgetExhaustionEvent, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].timestamp.Before(sorted[j].timestamp)
	})

	// Compare first half vs second half
	mid := len(sorted) / 2
	firstHalf := sorted[:mid]
	secondHalf := sorted[mid:]

	// Calculate average time between events in each half
	firstInterval := averageInterval(firstHalf)
	secondInterval := averageInterval(secondHalf)

	if firstInterval == 0 || secondInterval == 0 {
		return "stable"
	}

	// If events are happening more frequently, trend is increasing
	ratio := secondInterval.Seconds() / firstInterval.Seconds()
	if ratio < 0.7 {
		return "increasing" // Events happening more frequently
	} else if ratio > 1.3 {
		return "decreasing" // Events happening less frequently
	}

	return "stable"
}

func averageInterval(events []budgetExhaustionEvent) time.Duration {
	if len(events) < 2 {
		return 0
	}

	var totalInterval time.Duration
	for i := 1; i < len(events); i++ {
		totalInterval += events[i].timestamp.Sub(events[i-1].timestamp)
	}

	return totalInterval / time.Duration(len(events)-1)
}

func (d *BudgetExhaustionDetector) generateRecommendation(s *budgetUsageStats, trend string) string {
	if s.exhaustionCount == 0 {
		return "No action needed"
	}

	exhaustionRate := float64(s.exhaustionCount) / float64(s.totalRuns)

	switch {
	case exhaustionRate > 0.5:
		return fmt.Sprintf("Critical: Consider increasing '%s' budget by at least 50%% or reducing scope", s.name)
	case exhaustionRate > 0.25:
		return fmt.Sprintf("Warning: Consider increasing '%s' budget by 25-50%%", s.name)
	case trend == "increasing":
		return fmt.Sprintf("Monitor: '%s' exhaustion rate is increasing, may need budget adjustment soon", s.name)
	case s.nearMissCount > s.exhaustionCount:
		return fmt.Sprintf("Optimize: Many near-misses for '%s', consider reviewing task complexity", s.name)
	default:
		return fmt.Sprintf("Review: Occasional '%s' exhaustion, may be acceptable or require minor adjustment", s.name)
	}
}

// Types returns the pattern types this detector can find.
func (d *BudgetExhaustionDetector) Types() []pattern.PatternType {
	return []pattern.PatternType{
		pattern.PatternTypeBudgetExhaustion,
	}
}

// Ensure BudgetExhaustionDetector implements Detector
var _ pattern.Detector = (*BudgetExhaustionDetector)(nil)

// BudgetExhaustionExtendedData provides detailed budget exhaustion analysis.
type BudgetExhaustionExtendedData struct {
	// BudgetName is the name of the exhausted budget.
	BudgetName string `json:"budget_name"`

	// ExhaustionCount is how many times this budget was exhausted.
	ExhaustionCount int `json:"exhaustion_count"`

	// NearMissCount is runs that used > warning ratio but didn't exhaust.
	NearMissCount int `json:"near_miss_count"`

	// TotalRuns is the total number of runs analyzed.
	TotalRuns int `json:"total_runs"`

	// ExhaustionRate is the percentage of runs that exhausted the budget.
	ExhaustionRate float64 `json:"exhaustion_rate"`

	// AvgUsageRatio is the average budget usage ratio across all runs.
	AvgUsageRatio float64 `json:"avg_usage_ratio"`

	// Trend indicates if exhaustions are increasing, decreasing, or stable.
	Trend string `json:"trend"`

	// Recommendation provides actionable advice.
	Recommendation string `json:"recommendation"`
}
