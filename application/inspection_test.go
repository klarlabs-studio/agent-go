package application_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/agent/application"
	"go.klarlabs.de/agent/domain/inspector"
)

// mockInspector implements inspector.Inspector for testing.
type mockInspector struct {
	exportRunFn          func(ctx context.Context, runID string, format inspector.ExportFormat) ([]byte, error)
	exportStateMachineFn func(ctx context.Context, format inspector.ExportFormat) ([]byte, error)
	exportMetricsFn      func(ctx context.Context, filter inspector.MetricsFilter, format inspector.ExportFormat) ([]byte, error)
}

func (m *mockInspector) ExportRun(ctx context.Context, runID string, format inspector.ExportFormat) ([]byte, error) {
	if m.exportRunFn != nil {
		return m.exportRunFn(ctx, runID, format)
	}
	return []byte(`{"run_id": "` + runID + `"}`), nil
}

func (m *mockInspector) ExportStateMachine(ctx context.Context, format inspector.ExportFormat) ([]byte, error) {
	if m.exportStateMachineFn != nil {
		return m.exportStateMachineFn(ctx, format)
	}
	return []byte(`digraph {}`), nil
}

func (m *mockInspector) ExportMetrics(ctx context.Context, filter inspector.MetricsFilter, format inspector.ExportFormat) ([]byte, error) {
	if m.exportMetricsFn != nil {
		return m.exportMetricsFn(ctx, filter, format)
	}
	return []byte(`{"metrics": []}`), nil
}

func TestNewInspectionService(t *testing.T) {
	t.Parallel()

	insp := &mockInspector{}
	service := application.NewInspectionService(insp)

	if service == nil {
		t.Error("NewInspectionService should return non-nil service")
	}
}

func TestInspectionService_ExportRun(t *testing.T) {
	t.Parallel()

	t.Run("with inspector", func(t *testing.T) {
		t.Parallel()

		insp := &mockInspector{
			exportRunFn: func(ctx context.Context, runID string, format inspector.ExportFormat) ([]byte, error) {
				return []byte(`{"id": "` + runID + `"}`), nil
			},
		}
		service := application.NewInspectionService(insp)

		data, err := service.ExportRun(context.Background(), "run-123", inspector.FormatJSON)
		if err != nil {
			t.Fatalf("ExportRun() error = %v", err)
		}
		if string(data) != `{"id": "run-123"}` {
			t.Errorf("ExportRun() = %s, want {\"id\": \"run-123\"}", data)
		}
	})

	t.Run("without inspector", func(t *testing.T) {
		t.Parallel()

		service := application.NewInspectionService(nil)

		_, err := service.ExportRun(context.Background(), "run-123", inspector.FormatJSON)
		if !errors.Is(err, inspector.ErrExportFailed) {
			t.Errorf("ExportRun() error = %v, want %v", err, inspector.ErrExportFailed)
		}
	})
}

func TestInspectionService_ExportStateMachine(t *testing.T) {
	t.Parallel()

	t.Run("with inspector", func(t *testing.T) {
		t.Parallel()

		insp := &mockInspector{
			exportStateMachineFn: func(ctx context.Context, format inspector.ExportFormat) ([]byte, error) {
				return []byte(`digraph { intake -> explore }`), nil
			},
		}
		service := application.NewInspectionService(insp)

		data, err := service.ExportStateMachine(context.Background(), inspector.FormatDOT)
		if err != nil {
			t.Fatalf("ExportStateMachine() error = %v", err)
		}
		if len(data) == 0 {
			t.Error("ExportStateMachine() should return data")
		}
	})

	t.Run("without inspector", func(t *testing.T) {
		t.Parallel()

		service := application.NewInspectionService(nil)

		_, err := service.ExportStateMachine(context.Background(), inspector.FormatDOT)
		if !errors.Is(err, inspector.ErrExportFailed) {
			t.Errorf("ExportStateMachine() error = %v, want %v", err, inspector.ErrExportFailed)
		}
	})
}

func TestInspectionService_ExportMetrics(t *testing.T) {
	t.Parallel()

	t.Run("with inspector", func(t *testing.T) {
		t.Parallel()

		insp := &mockInspector{
			exportMetricsFn: func(ctx context.Context, filter inspector.MetricsFilter, format inspector.ExportFormat) ([]byte, error) {
				return []byte(`{"total_runs": 10}`), nil
			},
		}
		service := application.NewInspectionService(insp)

		filter := inspector.MetricsFilter{}
		data, err := service.ExportMetrics(context.Background(), filter, inspector.FormatJSON)
		if err != nil {
			t.Fatalf("ExportMetrics() error = %v", err)
		}
		if len(data) == 0 {
			t.Error("ExportMetrics() should return data")
		}
	})

	t.Run("without inspector", func(t *testing.T) {
		t.Parallel()

		service := application.NewInspectionService(nil)

		filter := inspector.MetricsFilter{}
		_, err := service.ExportMetrics(context.Background(), filter, inspector.FormatJSON)
		if !errors.Is(err, inspector.ErrExportFailed) {
			t.Errorf("ExportMetrics() error = %v, want %v", err, inspector.ErrExportFailed)
		}
	})
}

func TestInspectionService_GetRunAsJSON(t *testing.T) {
	t.Parallel()

	insp := &mockInspector{}
	service := application.NewInspectionService(insp)

	data, err := service.GetRunAsJSON(context.Background(), "run-456")
	if err != nil {
		t.Fatalf("GetRunAsJSON() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("GetRunAsJSON() should return data")
	}
}

func TestInspectionService_GetStateMachineAsDOT(t *testing.T) {
	t.Parallel()

	insp := &mockInspector{}
	service := application.NewInspectionService(insp)

	data, err := service.GetStateMachineAsDOT(context.Background())
	if err != nil {
		t.Fatalf("GetStateMachineAsDOT() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("GetStateMachineAsDOT() should return data")
	}
}

func TestInspectionService_GetStateMachineAsMermaid(t *testing.T) {
	t.Parallel()

	insp := &mockInspector{
		exportStateMachineFn: func(ctx context.Context, format inspector.ExportFormat) ([]byte, error) {
			if format == inspector.FormatMermaid {
				return []byte(`stateDiagram-v2\n  intake --> explore`), nil
			}
			return nil, nil
		},
	}
	service := application.NewInspectionService(insp)

	data, err := service.GetStateMachineAsMermaid(context.Background())
	if err != nil {
		t.Fatalf("GetStateMachineAsMermaid() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("GetStateMachineAsMermaid() should return data")
	}
}

func TestInspectionService_GetMetricsAsJSON(t *testing.T) {
	t.Parallel()

	insp := &mockInspector{}
	service := application.NewInspectionService(insp)

	filter := inspector.MetricsFilter{}
	data, err := service.GetMetricsAsJSON(context.Background(), filter)
	if err != nil {
		t.Fatalf("GetMetricsAsJSON() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("GetMetricsAsJSON() should return data")
	}
}
