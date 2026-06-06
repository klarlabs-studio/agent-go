package api_test

import (
	"testing"
	"time"

	"go.klarlabs.de/agent/infrastructure/storage/memory"
	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewPatternStore(t *testing.T) {
	t.Parallel()

	t.Run("creates in-memory pattern store", func(t *testing.T) {
		t.Parallel()

		store := api.NewPatternStore()

		if store == nil {
			t.Fatal("NewPatternStore() returned nil")
		}
	})
}

func TestNewSequenceDetector(t *testing.T) {
	t.Parallel()

	t.Run("creates sequence detector", func(t *testing.T) {
		t.Parallel()

		eventStore := memory.NewEventStore()
		runStore := memory.NewRunStore()

		detector := api.NewSequenceDetector(eventStore, runStore)

		if detector == nil {
			t.Fatal("NewSequenceDetector() returned nil")
		}
	})
}

func TestNewFailureDetector(t *testing.T) {
	t.Parallel()

	t.Run("creates failure detector", func(t *testing.T) {
		t.Parallel()

		eventStore := memory.NewEventStore()
		runStore := memory.NewRunStore()

		detector := api.NewFailureDetector(eventStore, runStore)

		if detector == nil {
			t.Fatal("NewFailureDetector() returned nil")
		}
	})
}

func TestNewPerformanceDetector(t *testing.T) {
	t.Parallel()

	t.Run("creates performance detector without options", func(t *testing.T) {
		t.Parallel()

		eventStore := memory.NewEventStore()
		runStore := memory.NewRunStore()

		detector := api.NewPerformanceDetector(eventStore, runStore)

		if detector == nil {
			t.Fatal("NewPerformanceDetector() returned nil")
		}
	})

	t.Run("creates performance detector with slow tool threshold", func(t *testing.T) {
		t.Parallel()

		eventStore := memory.NewEventStore()
		runStore := memory.NewRunStore()

		detector := api.NewPerformanceDetector(
			eventStore,
			runStore,
			api.WithSlowToolThreshold(5*time.Second),
		)

		if detector == nil {
			t.Fatal("NewPerformanceDetector() with slow tool threshold returned nil")
		}
	})

	t.Run("creates performance detector with long run threshold", func(t *testing.T) {
		t.Parallel()

		eventStore := memory.NewEventStore()
		runStore := memory.NewRunStore()

		detector := api.NewPerformanceDetector(
			eventStore,
			runStore,
			api.WithLongRunThreshold(30*time.Second),
		)

		if detector == nil {
			t.Fatal("NewPerformanceDetector() with long run threshold returned nil")
		}
	})

	t.Run("creates performance detector with multiple options", func(t *testing.T) {
		t.Parallel()

		eventStore := memory.NewEventStore()
		runStore := memory.NewRunStore()

		detector := api.NewPerformanceDetector(
			eventStore,
			runStore,
			api.WithSlowToolThreshold(5*time.Second),
			api.WithLongRunThreshold(30*time.Second),
		)

		if detector == nil {
			t.Fatal("NewPerformanceDetector() with multiple options returned nil")
		}
	})
}

func TestWithSlowToolThreshold(t *testing.T) {
	t.Parallel()

	t.Run("creates slow tool threshold option", func(t *testing.T) {
		t.Parallel()

		opt := api.WithSlowToolThreshold(10 * time.Second)

		if opt == nil {
			t.Fatal("WithSlowToolThreshold() returned nil")
		}
	})
}

func TestWithLongRunThreshold(t *testing.T) {
	t.Parallel()

	t.Run("creates long run threshold option", func(t *testing.T) {
		t.Parallel()

		opt := api.WithLongRunThreshold(60 * time.Second)

		if opt == nil {
			t.Fatal("WithLongRunThreshold() returned nil")
		}
	})
}

func TestNewCompositeDetector(t *testing.T) {
	t.Parallel()

	t.Run("creates composite detector with multiple detectors", func(t *testing.T) {
		t.Parallel()

		eventStore := memory.NewEventStore()
		runStore := memory.NewRunStore()

		seq := api.NewSequenceDetector(eventStore, runStore)
		fail := api.NewFailureDetector(eventStore, runStore)
		perf := api.NewPerformanceDetector(eventStore, runStore)

		composite := api.NewCompositeDetector(seq, fail, perf)

		if composite == nil {
			t.Fatal("NewCompositeDetector() returned nil")
		}
	})

	t.Run("creates composite detector with single detector", func(t *testing.T) {
		t.Parallel()

		eventStore := memory.NewEventStore()
		runStore := memory.NewRunStore()

		seq := api.NewSequenceDetector(eventStore, runStore)

		composite := api.NewCompositeDetector(seq)

		if composite == nil {
			t.Fatal("NewCompositeDetector() with single detector returned nil")
		}
	})

	t.Run("creates empty composite detector", func(t *testing.T) {
		t.Parallel()

		composite := api.NewCompositeDetector()

		if composite == nil {
			t.Fatal("NewCompositeDetector() with no detectors returned nil")
		}
	})
}

func TestPatternTypeConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		got      api.PatternType
		expected string
	}{
		{"PatternTypeToolSequence", api.PatternTypeToolSequence, "tool_sequence"},
		{"PatternTypeStateLoop", api.PatternTypeStateLoop, "state_loop"},
		{"PatternTypeToolAffinity", api.PatternTypeToolAffinity, "tool_affinity"},
		{"PatternTypeRecurringFailure", api.PatternTypeRecurringFailure, "recurring_failure"},
		{"PatternTypeToolFailure", api.PatternTypeToolFailure, "tool_failure"},
		{"PatternTypeBudgetExhaustion", api.PatternTypeBudgetExhaustion, "budget_exhaustion"},
		{"PatternTypeSlowTool", api.PatternTypeSlowTool, "slow_tool"},
		{"PatternTypeLongRuns", api.PatternTypeLongRuns, "long_runs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if string(tt.got) != tt.expected {
				t.Errorf("PatternType = %s, want %s", tt.got, tt.expected)
			}
		})
	}
}
