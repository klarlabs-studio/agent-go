package memory_test

import (
	"context"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/proposal"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

func TestNewProposalStore(t *testing.T) {
	t.Parallel()

	store := memory.NewProposalStore()
	if store == nil {
		t.Fatal("NewProposalStore() returned nil")
	}
}

func TestProposalStore_Save(t *testing.T) {
	t.Parallel()

	t.Run("saves valid proposal", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		p := &proposal.Proposal{
			ID:          "prop-1",
			Title:       "Test Proposal",
			Description: "Test Description",
			Status:      proposal.ProposalStatusDraft,
			CreatedAt:   time.Now(),
			CreatedBy:   "user-1",
		}

		err := store.Save(ctx, p)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		p := &proposal.Proposal{
			ID:    "",
			Title: "Test Proposal",
		}

		err := store.Save(ctx, p)
		if err != proposal.ErrInvalidProposal {
			t.Errorf("Save() error = %v, want ErrInvalidProposal", err)
		}
	})

	t.Run("returns error for duplicate ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		p := &proposal.Proposal{
			ID:    "prop-1",
			Title: "Test Proposal",
		}

		store.Save(ctx, p)
		err := store.Save(ctx, p)
		if err != proposal.ErrProposalExists {
			t.Errorf("Save() error = %v, want ErrProposalExists", err)
		}
	})

	t.Run("stores deep copy of slices and maps", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		p := &proposal.Proposal{
			ID:       "prop-1",
			Changes:  []proposal.PolicyChange{{Target: "original"}},
			Metadata: map[string]any{"key": "original"},
		}

		store.Save(ctx, p)

		// Modify original
		p.Changes[0].Target = "modified"
		p.Metadata["key"] = "modified"

		result, _ := store.Get(ctx, "prop-1")
		if result.Changes[0].Target != "original" {
			t.Error("Save() should store a deep copy of Changes")
		}
		if result.Metadata["key"] != "original" {
			t.Error("Save() should store a deep copy of Metadata")
		}
	})
}

func TestProposalStore_Get(t *testing.T) {
	t.Parallel()

	t.Run("gets existing proposal", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		p := &proposal.Proposal{
			ID:        "prop-1",
			Title:     "Test Proposal",
			CreatedBy: "user-1",
		}

		store.Save(ctx, p)

		result, err := store.Get(ctx, "prop-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if result.ID != "prop-1" {
			t.Errorf("Get() ID = %s, want prop-1", result.ID)
		}
		if result.CreatedBy != "user-1" {
			t.Errorf("Get() CreatedBy = %s, want user-1", result.CreatedBy)
		}
	})

	t.Run("returns error for non-existent proposal", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		_, err := store.Get(ctx, "nonexistent")
		if err != proposal.ErrProposalNotFound {
			t.Errorf("Get() error = %v, want ErrProposalNotFound", err)
		}
	})

	t.Run("returns copy not reference", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		p := &proposal.Proposal{
			ID:    "prop-1",
			Title: "Original",
		}
		store.Save(ctx, p)

		result, _ := store.Get(ctx, "prop-1")
		result.Title = "Modified"

		result2, _ := store.Get(ctx, "prop-1")
		if result2.Title != "Original" {
			t.Error("Get() should return a copy, not reference")
		}
	})
}

func TestProposalStore_List(t *testing.T) {
	t.Parallel()

	t.Run("lists all proposals", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &proposal.Proposal{ID: "p1", CreatedAt: now})
		store.Save(ctx, &proposal.Proposal{ID: "p2", CreatedAt: now.Add(time.Second)})
		store.Save(ctx, &proposal.Proposal{ID: "p3", CreatedAt: now.Add(2 * time.Second)})

		results, err := store.List(ctx, proposal.ListFilter{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 3 {
			t.Errorf("List() count = %d, want 3", len(results))
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1", Status: proposal.ProposalStatusDraft})
		store.Save(ctx, &proposal.Proposal{ID: "p2", Status: proposal.ProposalStatusPendingReview})
		store.Save(ctx, &proposal.Proposal{ID: "p3", Status: proposal.ProposalStatusDraft})

		results, err := store.List(ctx, proposal.ListFilter{
			Status: []proposal.ProposalStatus{proposal.ProposalStatusDraft},
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by creator", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1", CreatedBy: "user-1"})
		store.Save(ctx, &proposal.Proposal{ID: "p2", CreatedBy: "user-2"})
		store.Save(ctx, &proposal.Proposal{ID: "p3", CreatedBy: "user-1"})

		results, err := store.List(ctx, proposal.ListFilter{CreatedBy: "user-1"})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by suggestion ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1", SuggestionID: "sug-1"})
		store.Save(ctx, &proposal.Proposal{ID: "p2", SuggestionID: "sug-2"})
		store.Save(ctx, &proposal.Proposal{ID: "p3", SuggestionID: "sug-1"})

		results, err := store.List(ctx, proposal.ListFilter{SuggestionID: "sug-1"})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by time range", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &proposal.Proposal{ID: "p1", CreatedAt: now.Add(-2 * time.Hour)})
		store.Save(ctx, &proposal.Proposal{ID: "p2", CreatedAt: now.Add(-30 * time.Minute)})
		store.Save(ctx, &proposal.Proposal{ID: "p3", CreatedAt: now.Add(time.Hour)})

		results, err := store.List(ctx, proposal.ListFilter{
			FromTime: now.Add(-1 * time.Hour),
			ToTime:   now,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 1 {
			t.Errorf("List() count = %d, want 1", len(results))
		}
	})

	t.Run("applies offset and limit", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		for i := 0; i < 10; i++ {
			store.Save(ctx, &proposal.Proposal{ID: string(rune('a' + i))})
		}

		results, err := store.List(ctx, proposal.ListFilter{Offset: 3, Limit: 4})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 4 {
			t.Errorf("List() count = %d, want 4", len(results))
		}
	})

	t.Run("returns empty for large offset", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1"})

		results, err := store.List(ctx, proposal.ListFilter{Offset: 100})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("List() count = %d, want 0", len(results))
		}
	})

	t.Run("sorts by created at", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &proposal.Proposal{ID: "p3", CreatedAt: now.Add(2 * time.Hour)})
		store.Save(ctx, &proposal.Proposal{ID: "p1", CreatedAt: now})
		store.Save(ctx, &proposal.Proposal{ID: "p2", CreatedAt: now.Add(time.Hour)})

		results, err := store.List(ctx, proposal.ListFilter{OrderBy: proposal.OrderByCreatedAt})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].ID != "p1" || results[1].ID != "p2" || results[2].ID != "p3" {
			t.Error("List() did not sort by created at correctly")
		}
	})

	t.Run("sorts by submitted at", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		now := time.Now()
		t1, t2, t3 := now, now.Add(time.Hour), now.Add(2*time.Hour)
		store.Save(ctx, &proposal.Proposal{ID: "p3", SubmittedAt: &t3})
		store.Save(ctx, &proposal.Proposal{ID: "p1", SubmittedAt: &t1})
		store.Save(ctx, &proposal.Proposal{ID: "p2", SubmittedAt: &t2})

		results, err := store.List(ctx, proposal.ListFilter{OrderBy: proposal.OrderBySubmittedAt})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].ID != "p1" || results[1].ID != "p2" || results[2].ID != "p3" {
			t.Error("List() did not sort by submitted at correctly")
		}
	})

	t.Run("sorts by submitted at with nil values", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &proposal.Proposal{ID: "p1", SubmittedAt: nil})
		store.Save(ctx, &proposal.Proposal{ID: "p2", SubmittedAt: &now})
		store.Save(ctx, &proposal.Proposal{ID: "p3", SubmittedAt: nil})

		results, err := store.List(ctx, proposal.ListFilter{OrderBy: proposal.OrderBySubmittedAt})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		// Nil values should come first
		if len(results) != 3 {
			t.Errorf("List() count = %d, want 3", len(results))
		}
	})

	t.Run("sorts by approved at", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		now := time.Now()
		t1, t2, t3 := now, now.Add(time.Hour), now.Add(2*time.Hour)
		store.Save(ctx, &proposal.Proposal{ID: "p3", ApprovedAt: &t3})
		store.Save(ctx, &proposal.Proposal{ID: "p1", ApprovedAt: &t1})
		store.Save(ctx, &proposal.Proposal{ID: "p2", ApprovedAt: &t2})

		results, err := store.List(ctx, proposal.ListFilter{OrderBy: proposal.OrderByApprovedAt})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].ID != "p1" || results[1].ID != "p2" || results[2].ID != "p3" {
			t.Error("List() did not sort by approved at correctly")
		}
	})

	t.Run("sorts by status", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p4", Status: proposal.ProposalStatusApplied})
		store.Save(ctx, &proposal.Proposal{ID: "p1", Status: proposal.ProposalStatusDraft})
		store.Save(ctx, &proposal.Proposal{ID: "p3", Status: proposal.ProposalStatusApproved})
		store.Save(ctx, &proposal.Proposal{ID: "p2", Status: proposal.ProposalStatusPendingReview})

		results, err := store.List(ctx, proposal.ListFilter{OrderBy: proposal.OrderByStatus})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].Status != proposal.ProposalStatusDraft ||
			results[1].Status != proposal.ProposalStatusPendingReview ||
			results[2].Status != proposal.ProposalStatusApproved ||
			results[3].Status != proposal.ProposalStatusApplied {
			t.Error("List() did not sort by status correctly")
		}
	})

	t.Run("sorts descending", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &proposal.Proposal{ID: "p1", CreatedAt: now})
		store.Save(ctx, &proposal.Proposal{ID: "p2", CreatedAt: now.Add(time.Hour)})
		store.Save(ctx, &proposal.Proposal{ID: "p3", CreatedAt: now.Add(2 * time.Hour)})

		results, err := store.List(ctx, proposal.ListFilter{OrderBy: proposal.OrderByCreatedAt, Descending: true})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].ID != "p3" || results[1].ID != "p2" || results[2].ID != "p1" {
			t.Error("List() did not sort descending correctly")
		}
	})
}

func TestProposalStore_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing proposal", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1"})

		err := store.Delete(ctx, "p1")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err = store.Get(ctx, "p1")
		if err != proposal.ErrProposalNotFound {
			t.Error("Proposal should be deleted")
		}
	})

	t.Run("returns error for non-existent proposal", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		err := store.Delete(ctx, "nonexistent")
		if err != proposal.ErrProposalNotFound {
			t.Errorf("Delete() error = %v, want ErrProposalNotFound", err)
		}
	})
}

func TestProposalStore_Update(t *testing.T) {
	t.Parallel()

	t.Run("updates existing proposal", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1", Title: "Original"})

		err := store.Update(ctx, &proposal.Proposal{ID: "p1", Title: "Updated"})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		result, _ := store.Get(ctx, "p1")
		if result.Title != "Updated" {
			t.Errorf("Update() Title = %s, want Updated", result.Title)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		err := store.Update(ctx, &proposal.Proposal{ID: ""})
		if err != proposal.ErrInvalidProposal {
			t.Errorf("Update() error = %v, want ErrInvalidProposal", err)
		}
	})

	t.Run("returns error for non-existent proposal", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		err := store.Update(ctx, &proposal.Proposal{ID: "nonexistent"})
		if err != proposal.ErrProposalNotFound {
			t.Errorf("Update() error = %v, want ErrProposalNotFound", err)
		}
	})
}

func TestProposalStore_Count(t *testing.T) {
	t.Parallel()

	t.Run("counts all proposals", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1"})
		store.Save(ctx, &proposal.Proposal{ID: "p2"})
		store.Save(ctx, &proposal.Proposal{ID: "p3"})

		count, err := store.Count(ctx, proposal.ListFilter{})
		if err != nil {
			t.Fatalf("Count() error = %v", err)
		}
		if count != 3 {
			t.Errorf("Count() = %d, want 3", count)
		}
	})

	t.Run("counts with filter", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1", Status: proposal.ProposalStatusDraft})
		store.Save(ctx, &proposal.Proposal{ID: "p2", Status: proposal.ProposalStatusPendingReview})
		store.Save(ctx, &proposal.Proposal{ID: "p3", Status: proposal.ProposalStatusDraft})

		count, err := store.Count(ctx, proposal.ListFilter{
			Status: []proposal.ProposalStatus{proposal.ProposalStatusDraft},
		})
		if err != nil {
			t.Fatalf("Count() error = %v", err)
		}
		if count != 2 {
			t.Errorf("Count() = %d, want 2", count)
		}
	})
}

func TestProposalStore_Summarize(t *testing.T) {
	t.Parallel()

	t.Run("summarizes proposals", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1", Status: proposal.ProposalStatusDraft})
		store.Save(ctx, &proposal.Proposal{ID: "p2", Status: proposal.ProposalStatusPendingReview})
		store.Save(ctx, &proposal.Proposal{ID: "p3", Status: proposal.ProposalStatusApplied})
		store.Save(ctx, &proposal.Proposal{ID: "p4", Status: proposal.ProposalStatusRolledBack})

		summary, err := store.Summarize(ctx, proposal.ListFilter{})
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if summary.TotalProposals != 4 {
			t.Errorf("Summarize() TotalProposals = %d, want 4", summary.TotalProposals)
		}
		if summary.ByStatus[proposal.ProposalStatusDraft] != 1 {
			t.Errorf("Summarize() ByStatus[Draft] = %d, want 1", summary.ByStatus[proposal.ProposalStatusDraft])
		}
		if summary.PendingReview != 1 {
			t.Errorf("Summarize() PendingReview = %d, want 1", summary.PendingReview)
		}
		if summary.AppliedCount != 1 {
			t.Errorf("Summarize() AppliedCount = %d, want 1", summary.AppliedCount)
		}
		if summary.RolledBackCount != 1 {
			t.Errorf("Summarize() RolledBackCount = %d, want 1", summary.RolledBackCount)
		}
	})

	t.Run("summarizes with filter", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		store.Save(ctx, &proposal.Proposal{ID: "p1", Status: proposal.ProposalStatusDraft, CreatedBy: "user-1"})
		store.Save(ctx, &proposal.Proposal{ID: "p2", Status: proposal.ProposalStatusDraft, CreatedBy: "user-2"})

		summary, err := store.Summarize(ctx, proposal.ListFilter{CreatedBy: "user-1"})
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if summary.TotalProposals != 1 {
			t.Errorf("Summarize() TotalProposals = %d, want 1", summary.TotalProposals)
		}
	})

	t.Run("handles empty store", func(t *testing.T) {
		t.Parallel()

		store := memory.NewProposalStore()
		ctx := context.Background()

		summary, err := store.Summarize(ctx, proposal.ListFilter{})
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if summary.TotalProposals != 0 {
			t.Errorf("Summarize() TotalProposals = %d, want 0", summary.TotalProposals)
		}
	})
}
