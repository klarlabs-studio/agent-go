package application_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/agent/application"
	"go.klarlabs.de/agent/domain/pattern"
)

// mockDetector implements pattern.Detector for testing.
type mockDetector struct {
	detectFn func(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error)
	typesFn  func() []pattern.PatternType
}

func (m *mockDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
	if m.detectFn != nil {
		return m.detectFn(ctx, opts)
	}
	return []pattern.Pattern{}, nil
}

func (m *mockDetector) Types() []pattern.PatternType {
	if m.typesFn != nil {
		return m.typesFn()
	}
	return []pattern.PatternType{}
}

// mockPatternStore implements pattern.Store for testing.
type mockPatternStore struct {
	saveFn   func(ctx context.Context, p *pattern.Pattern) error
	getFn    func(ctx context.Context, id string) (*pattern.Pattern, error)
	listFn   func(ctx context.Context, filter pattern.ListFilter) ([]*pattern.Pattern, error)
	deleteFn func(ctx context.Context, id string) error
	updateFn func(ctx context.Context, p *pattern.Pattern) error
}

func (m *mockPatternStore) Save(ctx context.Context, p *pattern.Pattern) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, p)
	}
	return nil
}

func (m *mockPatternStore) Get(ctx context.Context, id string) (*pattern.Pattern, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, pattern.ErrPatternNotFound
}

func (m *mockPatternStore) List(ctx context.Context, filter pattern.ListFilter) ([]*pattern.Pattern, error) {
	if m.listFn != nil {
		return m.listFn(ctx, filter)
	}
	return []*pattern.Pattern{}, nil
}

func (m *mockPatternStore) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockPatternStore) Update(ctx context.Context, p *pattern.Pattern) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, p)
	}
	return nil
}

func TestNewDetectionService(t *testing.T) {
	t.Parallel()

	detector := &mockDetector{}
	store := &mockPatternStore{}

	service := application.NewDetectionService(detector, store)
	if service == nil {
		t.Error("NewDetectionService should return non-nil service")
	}
}

func TestDetectionService_DetectPatterns(t *testing.T) {
	t.Parallel()

	t.Run("without detector", func(t *testing.T) {
		t.Parallel()

		service := application.NewDetectionService(nil, nil)

		_, err := service.DetectPatterns(context.Background(), pattern.DetectionOptions{})
		if !errors.Is(err, pattern.ErrDetectionFailed) {
			t.Errorf("DetectPatterns() error = %v, want %v", err, pattern.ErrDetectionFailed)
		}
	})

	t.Run("detection fails", func(t *testing.T) {
		t.Parallel()

		detector := &mockDetector{
			detectFn: func(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
				return nil, errors.New("detection error")
			},
		}
		service := application.NewDetectionService(detector, nil)

		_, err := service.DetectPatterns(context.Background(), pattern.DetectionOptions{})
		if err == nil {
			t.Error("DetectPatterns() should return error")
		}
	})

	t.Run("success without store", func(t *testing.T) {
		t.Parallel()

		detector := &mockDetector{
			detectFn: func(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
				return []pattern.Pattern{
					{ID: "p1", Name: "Pattern 1"},
					{ID: "p2", Name: "Pattern 2"},
				}, nil
			},
		}
		service := application.NewDetectionService(detector, nil)

		patterns, err := service.DetectPatterns(context.Background(), pattern.DetectionOptions{})
		if err != nil {
			t.Fatalf("DetectPatterns() error = %v", err)
		}
		if len(patterns) != 2 {
			t.Errorf("len(patterns) = %d, want 2", len(patterns))
		}
	})

	t.Run("success with store", func(t *testing.T) {
		t.Parallel()

		savedCount := 0
		detector := &mockDetector{
			detectFn: func(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
				return []pattern.Pattern{
					{ID: "p1", Name: "Pattern 1"},
				}, nil
			},
		}
		store := &mockPatternStore{
			saveFn: func(ctx context.Context, p *pattern.Pattern) error {
				savedCount++
				return nil
			},
		}
		service := application.NewDetectionService(detector, store)

		patterns, err := service.DetectPatterns(context.Background(), pattern.DetectionOptions{})
		if err != nil {
			t.Fatalf("DetectPatterns() error = %v", err)
		}
		if len(patterns) != 1 {
			t.Errorf("len(patterns) = %d, want 1", len(patterns))
		}
		if savedCount != 1 {
			t.Errorf("savedCount = %d, want 1", savedCount)
		}
	})

	t.Run("store save fails then updates", func(t *testing.T) {
		t.Parallel()

		updateCalled := false
		detector := &mockDetector{
			detectFn: func(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
				return []pattern.Pattern{
					{ID: "p1", Name: "Pattern 1"},
				}, nil
			},
		}
		store := &mockPatternStore{
			saveFn: func(ctx context.Context, p *pattern.Pattern) error {
				return pattern.ErrPatternExists
			},
			updateFn: func(ctx context.Context, p *pattern.Pattern) error {
				updateCalled = true
				return nil
			},
		}
		service := application.NewDetectionService(detector, store)

		patterns, err := service.DetectPatterns(context.Background(), pattern.DetectionOptions{})
		if err != nil {
			t.Fatalf("DetectPatterns() error = %v", err)
		}
		if len(patterns) != 1 {
			t.Errorf("len(patterns) = %d, want 1", len(patterns))
		}
		if !updateCalled {
			t.Error("Update should have been called")
		}
	})
}

func TestDetectionService_GetPattern(t *testing.T) {
	t.Parallel()

	t.Run("without store", func(t *testing.T) {
		t.Parallel()

		service := application.NewDetectionService(nil, nil)

		_, err := service.GetPattern(context.Background(), "p1")
		if !errors.Is(err, pattern.ErrPatternNotFound) {
			t.Errorf("GetPattern() error = %v, want %v", err, pattern.ErrPatternNotFound)
		}
	})

	t.Run("pattern found", func(t *testing.T) {
		t.Parallel()

		store := &mockPatternStore{
			getFn: func(ctx context.Context, id string) (*pattern.Pattern, error) {
				return &pattern.Pattern{ID: id, Name: "Test Pattern"}, nil
			},
		}
		service := application.NewDetectionService(nil, store)

		p, err := service.GetPattern(context.Background(), "p1")
		if err != nil {
			t.Fatalf("GetPattern() error = %v", err)
		}
		if p.ID != "p1" {
			t.Errorf("Pattern.ID = %s, want p1", p.ID)
		}
	})
}

func TestDetectionService_ListPatterns(t *testing.T) {
	t.Parallel()

	t.Run("without store", func(t *testing.T) {
		t.Parallel()

		service := application.NewDetectionService(nil, nil)

		patterns, err := service.ListPatterns(context.Background(), pattern.ListFilter{})
		if err != nil {
			t.Fatalf("ListPatterns() error = %v", err)
		}
		if len(patterns) != 0 {
			t.Errorf("len(patterns) = %d, want 0", len(patterns))
		}
	})

	t.Run("with store", func(t *testing.T) {
		t.Parallel()

		store := &mockPatternStore{
			listFn: func(ctx context.Context, filter pattern.ListFilter) ([]*pattern.Pattern, error) {
				return []*pattern.Pattern{
					{ID: "p1"},
					{ID: "p2"},
				}, nil
			},
		}
		service := application.NewDetectionService(nil, store)

		patterns, err := service.ListPatterns(context.Background(), pattern.ListFilter{})
		if err != nil {
			t.Fatalf("ListPatterns() error = %v", err)
		}
		if len(patterns) != 2 {
			t.Errorf("len(patterns) = %d, want 2", len(patterns))
		}
	})
}

func TestDetectionService_DeletePattern(t *testing.T) {
	t.Parallel()

	t.Run("without store", func(t *testing.T) {
		t.Parallel()

		service := application.NewDetectionService(nil, nil)

		err := service.DeletePattern(context.Background(), "p1")
		if !errors.Is(err, pattern.ErrPatternNotFound) {
			t.Errorf("DeletePattern() error = %v, want %v", err, pattern.ErrPatternNotFound)
		}
	})

	t.Run("with store", func(t *testing.T) {
		t.Parallel()

		deleted := false
		store := &mockPatternStore{
			deleteFn: func(ctx context.Context, id string) error {
				deleted = true
				return nil
			},
		}
		service := application.NewDetectionService(nil, store)

		err := service.DeletePattern(context.Background(), "p1")
		if err != nil {
			t.Fatalf("DeletePattern() error = %v", err)
		}
		if !deleted {
			t.Error("Delete should have been called")
		}
	})
}

func TestDetectionService_GetSupportedPatternTypes(t *testing.T) {
	t.Parallel()

	t.Run("without detector", func(t *testing.T) {
		t.Parallel()

		service := application.NewDetectionService(nil, nil)

		types := service.GetSupportedPatternTypes()
		if len(types) != 0 {
			t.Errorf("len(types) = %d, want 0", len(types))
		}
	})

	t.Run("with detector", func(t *testing.T) {
		t.Parallel()

		detector := &mockDetector{
			typesFn: func() []pattern.PatternType {
				return []pattern.PatternType{"type1", "type2"}
			},
		}
		service := application.NewDetectionService(detector, nil)

		types := service.GetSupportedPatternTypes()
		if len(types) != 2 {
			t.Errorf("len(types) = %d, want 2", len(types))
		}
	})
}
