package run

import (
	"errors"
	"testing"
)

func TestListFilter_Defaults(t *testing.T) {
	var f ListFilter
	if f.Limit != 0 || f.Offset != 0 || f.GoalPattern != "" {
		t.Error("expected zero-value ListFilter")
	}
	if f.ParentRunID != "" || f.TaskID != "" {
		t.Error("expected empty hierarchy fields")
	}
}

func TestListFilter_WithPagination(t *testing.T) {
	f := ListFilter{Limit: 20, Offset: 40, OrderBy: OrderByStartTime, Descending: true}
	if f.Limit != 20 || f.Offset != 40 {
		t.Error("pagination not set")
	}
}

func TestOrderByConstants(t *testing.T) {
	tests := []struct {
		name  string
		order OrderBy
	}{
		{"StartTime", OrderByStartTime},
		{"EndTime", OrderByEndTime},
		{"ID", OrderByID},
		{"Status", OrderByStatus},
	}
	for _, tt := range tests {
		if tt.order == "" {
			t.Errorf("%s: empty OrderBy constant", tt.name)
		}
	}
}

func TestErrorSentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrRunNotFound", ErrRunNotFound},
		{"ErrRunExists", ErrRunExists},
		{"ErrInvalidRunID", ErrInvalidRunID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error sentinel is nil")
			}
			wrapped := errors.Join(tt.err, errors.New("detail"))
			if !errors.Is(wrapped, tt.err) {
				t.Error("errors.Is failed on wrapped error")
			}
		})
	}
}

func TestInterfaces(t *testing.T) {
	var _ Store = nil
	var _ SummaryProvider = nil
}
