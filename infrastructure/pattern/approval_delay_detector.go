package pattern

import (
	"context"
	"fmt"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/run"
)

// ApprovalDelayDetector detects human approval bottlenecks.
type ApprovalDelayDetector struct {
	eventStore     event.Store
	runStore       run.Store
	delayThreshold time.Duration // Minimum delay to consider a bottleneck
	minOccurrences int           // Minimum occurrences to report
}

// ApprovalDelayOption configures the approval delay detector.
type ApprovalDelayOption func(*ApprovalDelayDetector)

// WithDelayThreshold sets the delay threshold.
func WithDelayThreshold(d time.Duration) ApprovalDelayOption {
	return func(det *ApprovalDelayDetector) {
		det.delayThreshold = d
	}
}

// WithApprovalMinOccurrences sets minimum occurrences.
func WithApprovalMinOccurrences(n int) ApprovalDelayOption {
	return func(d *ApprovalDelayDetector) {
		d.minOccurrences = n
	}
}

// NewApprovalDelayDetector creates a new approval delay detector.
func NewApprovalDelayDetector(eventStore event.Store, runStore run.Store, opts ...ApprovalDelayOption) *ApprovalDelayDetector {
	d := &ApprovalDelayDetector{
		eventStore:     eventStore,
		runStore:       runStore,
		delayThreshold: 5 * time.Minute,
		minOccurrences: 2,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Detect finds approval bottlenecks.
func (d *ApprovalDelayDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
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

	// Track approval delays by tool
	type approvalKey struct {
		toolName string
		state    agent.State
	}
	type approvalStats struct {
		waitTimes []time.Duration
		runIDs    map[string]bool
		approved  int
		denied    int
		pending   int
		maxWait   time.Duration
	}
	statsByKey := make(map[approvalKey]*approvalStats)

	for _, r := range runs {
		events, err := d.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		// Track pending approval requests
		pendingRequests := make(map[string]approvalRequest) // toolName -> request

		for _, e := range events {
			switch e.Type {
			case event.TypeApprovalRequested:
				var payload event.ApprovalRequestedPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					pendingRequests[payload.ToolName] = approvalRequest{
						timestamp: e.Timestamp,
						toolName:  payload.ToolName,
					}

					// Get current state from run
					key := approvalKey{toolName: payload.ToolName, state: r.CurrentState}
					if _, ok := statsByKey[key]; !ok {
						statsByKey[key] = &approvalStats{
							waitTimes: make([]time.Duration, 0),
							runIDs:    make(map[string]bool),
						}
					}
				}

			case event.TypeApprovalGranted:
				var payload event.ApprovalResultPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					if req, ok := pendingRequests[payload.ToolName]; ok {
						waitTime := e.Timestamp.Sub(req.timestamp)
						key := approvalKey{toolName: payload.ToolName, state: r.CurrentState}

						if stats, ok := statsByKey[key]; ok {
							stats.waitTimes = append(stats.waitTimes, waitTime)
							stats.runIDs[r.ID] = true
							stats.approved++
							if waitTime > stats.maxWait {
								stats.maxWait = waitTime
							}
						}
						delete(pendingRequests, payload.ToolName)
					}
				}

			case event.TypeApprovalDenied:
				var payload event.ApprovalResultPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					if req, ok := pendingRequests[payload.ToolName]; ok {
						waitTime := e.Timestamp.Sub(req.timestamp)
						key := approvalKey{toolName: payload.ToolName, state: r.CurrentState}

						if stats, ok := statsByKey[key]; ok {
							stats.waitTimes = append(stats.waitTimes, waitTime)
							stats.runIDs[r.ID] = true
							stats.denied++
							if waitTime > stats.maxWait {
								stats.maxWait = waitTime
							}
						}
						delete(pendingRequests, payload.ToolName)
					}
				}
			}
		}

		// Count still-pending approvals
		for toolName := range pendingRequests {
			key := approvalKey{toolName: toolName, state: r.CurrentState}
			if stats, ok := statsByKey[key]; ok {
				stats.pending++
				stats.runIDs[r.ID] = true
			}
		}
	}

	// Create patterns for significant delays
	var patterns []pattern.Pattern
	for key, stats := range statsByKey {
		totalApprovals := stats.approved + stats.denied
		if len(stats.waitTimes) < d.minOccurrences {
			continue
		}

		// Calculate average wait time
		var totalWait time.Duration
		delayCount := 0
		for _, wt := range stats.waitTimes {
			totalWait += wt
			if wt >= d.delayThreshold {
				delayCount++
			}
		}

		if delayCount == 0 {
			continue // No significant delays
		}

		avgWait := totalWait / time.Duration(len(stats.waitTimes))
		approvalRate := 0.0
		if totalApprovals > 0 {
			approvalRate = float64(stats.approved) / float64(totalApprovals)
		}

		// Calculate confidence
		confidence := calculateApprovalDelayConfidence(delayCount, len(stats.waitTimes), avgWait, d.delayThreshold)
		if opts.MinConfidence > 0 && confidence < opts.MinConfidence {
			continue
		}

		p := pattern.NewPattern(
			pattern.PatternTypeApprovalDelay,
			fmt.Sprintf("Approval Bottleneck: %s", key.toolName),
			fmt.Sprintf("Tool %s in state %s has avg wait time %v (%d delays, %d pending)",
				key.toolName, key.state, avgWait.Round(time.Second), delayCount, stats.pending),
		)
		p.Confidence = confidence
		p.Frequency = delayCount

		data := pattern.ApprovalDelayData{
			ToolName:        key.toolName,
			State:           key.state,
			AverageWaitTime: avgWait,
			MaxWaitTime:     stats.maxWait,
			PendingCount:    stats.pending,
			TotalApprovals:  totalApprovals,
			ApprovalRate:    approvalRate,
		}
		if err := p.SetData(data); err != nil {
			continue
		}

		// Add evidence
		for runID := range stats.runIDs {
			if err := p.AddEvidence(runID, map[string]any{
				"tool_name": key.toolName,
				"state":     key.state,
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
func (d *ApprovalDelayDetector) Types() []pattern.PatternType {
	return []pattern.PatternType{pattern.PatternTypeApprovalDelay}
}

type approvalRequest struct {
	timestamp time.Time
	toolName  string
}

func calculateApprovalDelayConfidence(delayCount, totalCount int, avgWait, threshold time.Duration) float64 {
	// Base confidence on delay frequency
	delayRate := float64(delayCount) / float64(totalCount)
	rateConfidence := 0.4 + delayRate*0.4

	// Bonus for wait time significantly exceeding threshold
	waitRatio := float64(avgWait) / float64(threshold)
	waitBonus := 0.0
	if waitRatio > 2 {
		waitBonus = 0.15
	} else if waitRatio > 1.5 {
		waitBonus = 0.1
	}

	confidence := rateConfidence + waitBonus
	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return confidence
}

// Ensure ApprovalDelayDetector implements Detector
var _ pattern.Detector = (*ApprovalDelayDetector)(nil)
