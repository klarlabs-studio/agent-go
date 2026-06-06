package api_test

import (
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewInspector(t *testing.T) {
	t.Parallel()

	t.Run("creates inspector with all exporters", func(t *testing.T) {
		t.Parallel()

		// Create stores
		runStore := memory.NewRunStore()
		eventStore := memory.NewEventStore()

		// Create exporters
		runExporter := api.NewRunExporter(runStore, eventStore)
		eligibility := api.NewToolEligibility()
		transitions := api.DefaultTransitions()
		stateMachineExporter := api.NewStateMachineExporter(eligibility, transitions)
		metricsExporter := api.NewMetricsExporter(runStore, eventStore)

		// Create inspector
		inspector := api.NewInspector(runExporter, stateMachineExporter, metricsExporter)

		if inspector == nil {
			t.Fatal("NewInspector() returned nil")
		}
	})
}

func TestNewRunExporter(t *testing.T) {
	t.Parallel()

	t.Run("creates run exporter", func(t *testing.T) {
		t.Parallel()

		runStore := memory.NewRunStore()
		eventStore := memory.NewEventStore()

		exporter := api.NewRunExporter(runStore, eventStore)

		if exporter == nil {
			t.Fatal("NewRunExporter() returned nil")
		}
	})
}

func TestNewStateMachineExporter(t *testing.T) {
	t.Parallel()

	t.Run("creates state machine exporter with eligibility and transitions", func(t *testing.T) {
		t.Parallel()

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")
		eligibility.Allow(agent.StateAct, "write_file")

		transitions := api.DefaultTransitions()

		exporter := api.NewStateMachineExporter(eligibility, transitions)

		if exporter == nil {
			t.Fatal("NewStateMachineExporter() returned nil")
		}
	})

	t.Run("creates state machine exporter with custom transitions", func(t *testing.T) {
		t.Parallel()

		eligibility := api.NewToolEligibility()

		transitions := policy.NewStateTransitions()
		transitions.Allow(agent.StateIntake, agent.StateExplore)
		transitions.Allow(agent.StateExplore, agent.StateDecide)

		exporter := api.NewStateMachineExporter(eligibility, transitions)

		if exporter == nil {
			t.Fatal("NewStateMachineExporter() returned nil")
		}
	})
}

func TestNewMetricsExporter(t *testing.T) {
	t.Parallel()

	t.Run("creates metrics exporter", func(t *testing.T) {
		t.Parallel()

		runStore := memory.NewRunStore()
		eventStore := memory.NewEventStore()

		exporter := api.NewMetricsExporter(runStore, eventStore)

		if exporter == nil {
			t.Fatal("NewMetricsExporter() returned nil")
		}
	})
}

func TestExportFormatConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   api.ExportFormat
		expected string
	}{
		{"FormatJSON", api.FormatJSON, "json"},
		{"FormatDOT", api.FormatDOT, "dot"},
		{"FormatMermaid", api.FormatMermaid, "mermaid"},
		{"FormatTimeline", api.FormatTimeline, "timeline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if string(tt.format) != tt.expected {
				t.Errorf("ExportFormat = %s, want %s", tt.format, tt.expected)
			}
		})
	}
}
