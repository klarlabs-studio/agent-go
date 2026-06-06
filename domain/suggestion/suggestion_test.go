package suggestion

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pattern"
)

// Suggestion Creation Tests

func TestNewSuggestion_CreatesValidSuggestion(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test title", "test description")

	if s.ID == "" {
		t.Error("expected non-empty ID")
	}
	if s.Type != SuggestionTypeAddEligibility {
		t.Errorf("expected type %s, got %s", SuggestionTypeAddEligibility, s.Type)
	}
	if s.Title != "test title" {
		t.Errorf("expected title 'test title', got %s", s.Title)
	}
	if s.Description != "test description" {
		t.Errorf("expected description 'test description', got %s", s.Description)
	}
	if s.Status != SuggestionStatusPending {
		t.Errorf("expected status Pending, got %s", s.Status)
	}
	if s.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if len(s.PatternIDs) != 0 {
		t.Errorf("expected empty PatternIDs, got %d", len(s.PatternIDs))
	}
	if s.Metadata == nil {
		t.Error("expected Metadata to be initialized")
	}
}

func TestNewSuggestion_GeneratesUniqueIDs(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")
		if ids[s.ID] {
			t.Errorf("duplicate ID generated: %s", s.ID)
		}
		ids[s.ID] = true
	}
}

func TestNewSuggestion_AllTypes(t *testing.T) {
	types := []SuggestionType{
		SuggestionTypeAddEligibility,
		SuggestionTypeRemoveEligibility,
		SuggestionTypeAddTransition,
		SuggestionTypeRemoveTransition,
		SuggestionTypeIncreaseBudget,
		SuggestionTypeDecreaseBudget,
		SuggestionTypeRequireApproval,
		SuggestionTypeRemoveApproval,
	}

	for _, typ := range types {
		s := NewSuggestion(typ, "test", "test")
		if s.Type != typ {
			t.Errorf("expected type %s, got %s", typ, s.Type)
		}
	}
}

// Status Transition Tests

func TestAccept_TransitionFromPending(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")

	err := s.Accept("proposal-123", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Status != SuggestionStatusAccepted {
		t.Errorf("expected status Accepted, got %s", s.Status)
	}
	if s.ProposalID != "proposal-123" {
		t.Errorf("expected ProposalID 'proposal-123', got %s", s.ProposalID)
	}
	if s.StatusChangedBy != "alice" {
		t.Errorf("expected StatusChangedBy 'alice', got %s", s.StatusChangedBy)
	}
	if s.StatusChangedAt.IsZero() {
		t.Error("expected StatusChangedAt to be set")
	}
}

func TestAccept_FailsFromNonPendingStatus(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")
	s.Accept("proposal-1", "alice") // Now accepted

	err := s.Accept("proposal-2", "bob")
	if err != ErrInvalidStatusTransition {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

func TestReject_TransitionFromPending(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")

	err := s.Reject("bob", "Not needed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Status != SuggestionStatusRejected {
		t.Errorf("expected status Rejected, got %s", s.Status)
	}
	if s.StatusChangedBy != "bob" {
		t.Errorf("expected StatusChangedBy 'bob', got %s", s.StatusChangedBy)
	}
	if s.Metadata["rejection_reason"] != "Not needed" {
		t.Errorf("expected rejection reason, got %v", s.Metadata["rejection_reason"])
	}
}

func TestReject_FailsFromNonPendingStatus(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")
	s.Reject("alice", "reason")

	err := s.Reject("bob", "another reason")
	if err != ErrInvalidStatusTransition {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

func TestSupersede_TransitionFromPending(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")

	err := s.Supersede("new-suggestion-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Status != SuggestionStatusSuperseded {
		t.Errorf("expected status Superseded, got %s", s.Status)
	}
	if s.Metadata["superseded_by"] != "new-suggestion-456" {
		t.Errorf("expected superseded_by metadata, got %v", s.Metadata["superseded_by"])
	}
}

func TestSupersede_FailsFromNonPendingStatus(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")
	s.Accept("proposal-1", "alice")

	err := s.Supersede("new-suggestion")
	if err != ErrInvalidStatusTransition {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

// Change Data Tests

func TestSetChangeData_SerializesData(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")

	data := EligibilityChangeData{
		State:    agent.StateExplore,
		ToolName: "read_file",
		Add:      true,
	}
	err := s.SetChangeData(data)
	if err != nil {
		t.Fatalf("failed to set change data: %v", err)
	}

	if s.ChangeData == nil {
		t.Error("expected ChangeData to be set")
	}
}

func TestGetChangeData_DeserializesData(t *testing.T) {
	s := NewSuggestion(SuggestionTypeIncreaseBudget, "test", "test")

	original := BudgetChangeData{
		BudgetName: "tool_calls",
		OldValue:   100,
		NewValue:   200,
	}
	s.SetChangeData(original)

	var retrieved BudgetChangeData
	err := s.GetChangeData(&retrieved)
	if err != nil {
		t.Fatalf("failed to get change data: %v", err)
	}

	if retrieved.BudgetName != "tool_calls" {
		t.Errorf("expected BudgetName 'tool_calls', got %s", retrieved.BudgetName)
	}
	if retrieved.OldValue != 100 {
		t.Errorf("expected OldValue 100, got %d", retrieved.OldValue)
	}
	if retrieved.NewValue != 200 {
		t.Errorf("expected NewValue 200, got %d", retrieved.NewValue)
	}
}

func TestGetChangeData_HandlesNilData(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")

	var data EligibilityChangeData
	err := s.GetChangeData(&data)
	if err != nil {
		t.Errorf("expected no error for nil data, got %v", err)
	}
}

func TestSetChangeData_HandlesUnserializableData(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")

	err := s.SetChangeData(make(chan int))
	if err == nil {
		t.Error("expected error for unserializable data")
	}
}

// Significance Tests

func TestIsSignificant_ReturnsTrueWhenAboveThreshold(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")
	s.Confidence = 0.8

	if !s.IsSignificant(0.7) {
		t.Error("expected suggestion to be significant")
	}
}

func TestIsSignificant_ReturnsFalseWhenBelowThreshold(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")
	s.Confidence = 0.5

	if s.IsSignificant(0.7) {
		t.Error("expected suggestion to not be significant")
	}
}

func TestIsSignificant_BoundaryCondition(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")
	s.Confidence = 0.7

	if !s.IsSignificant(0.7) {
		t.Error("expected suggestion at exact threshold to be significant")
	}
}

// Pattern ID Tests

func TestAddPatternID_AddsUniqueID(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")

	s.AddPatternID("pattern-1")
	s.AddPatternID("pattern-2")

	if len(s.PatternIDs) != 2 {
		t.Errorf("expected 2 pattern IDs, got %d", len(s.PatternIDs))
	}
}

func TestAddPatternID_DeduplicatesIDs(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")

	s.AddPatternID("pattern-1")
	s.AddPatternID("pattern-1") // Duplicate
	s.AddPatternID("pattern-2")

	if len(s.PatternIDs) != 2 {
		t.Errorf("expected 2 unique pattern IDs, got %d", len(s.PatternIDs))
	}
}

// Generation Options Tests

func TestDefaultGenerationOptions(t *testing.T) {
	opts := DefaultGenerationOptions()

	if opts.MinConfidence != 0.6 {
		t.Errorf("expected MinConfidence 0.6, got %f", opts.MinConfidence)
	}
	if opts.MinFrequency != 3 {
		t.Errorf("expected MinFrequency 3, got %d", opts.MinFrequency)
	}
	if opts.MaxSuggestions != 50 {
		t.Errorf("expected MaxSuggestions 50, got %d", opts.MaxSuggestions)
	}
	if opts.IncludeHighImpact {
		t.Error("expected IncludeHighImpact to be false by default")
	}
}

// GeneratorFunc Tests

func TestGeneratorFunc_ImplementsGenerator(t *testing.T) {
	called := false
	expectedSuggestions := []Suggestion{
		*NewSuggestion(SuggestionTypeAddEligibility, "test", "test"),
	}

	generator := GeneratorFunc(func(ctx context.Context, patterns []pattern.Pattern) ([]Suggestion, error) {
		called = true
		return expectedSuggestions, nil
	})

	ctx := context.Background()
	suggestions, err := generator.Generate(ctx, nil)

	if !called {
		t.Error("expected generator function to be called")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(suggestions) != 1 {
		t.Errorf("expected 1 suggestion, got %d", len(suggestions))
	}
}

func TestGeneratorFunc_TypesReturnsNil(t *testing.T) {
	generator := GeneratorFunc(func(ctx context.Context, patterns []pattern.Pattern) ([]Suggestion, error) {
		return nil, nil
	})

	types := generator.Types()
	if types != nil {
		t.Errorf("expected nil Types(), got %v", types)
	}
}

// ListFilter Tests

func TestListFilter_DefaultValues(t *testing.T) {
	filter := ListFilter{}

	if filter.MinConfidence != 0 {
		t.Error("expected zero MinConfidence by default")
	}
	if filter.Limit != 0 {
		t.Error("expected zero Limit by default")
	}
	if len(filter.Types) != 0 {
		t.Error("expected empty Types by default")
	}
	if len(filter.Status) != 0 {
		t.Error("expected empty Status by default")
	}
}

func TestListFilter_TypeFiltering(t *testing.T) {
	filter := ListFilter{
		Types: []SuggestionType{SuggestionTypeAddEligibility, SuggestionTypeIncreaseBudget},
	}

	if len(filter.Types) != 2 {
		t.Errorf("expected 2 types, got %d", len(filter.Types))
	}
}

func TestListFilter_StatusFiltering(t *testing.T) {
	filter := ListFilter{
		Status: []SuggestionStatus{SuggestionStatusPending, SuggestionStatusAccepted},
	}

	if len(filter.Status) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(filter.Status))
	}
}

func TestListFilter_ImpactFiltering(t *testing.T) {
	filter := ListFilter{
		Impact: []ImpactLevel{ImpactLevelHigh, ImpactLevelMedium},
	}

	if len(filter.Impact) != 2 {
		t.Errorf("expected 2 impact levels, got %d", len(filter.Impact))
	}
}

func TestListFilter_Ordering(t *testing.T) {
	orderBys := []OrderBy{
		OrderByCreatedAt,
		OrderByConfidence,
		OrderByImpact,
		OrderByStatus,
	}

	for _, order := range orderBys {
		filter := ListFilter{OrderBy: order, Descending: true}
		if filter.OrderBy != order {
			t.Errorf("expected OrderBy %s, got %s", order, filter.OrderBy)
		}
	}
}

// Policy Change Type Tests

func TestPolicyChange_Serialization(t *testing.T) {
	change := PolicyChange{
		Type:   PolicyChangeTypeEligibility,
		Target: "read_file",
		From:   false,
		To:     true,
	}

	jsonBytes, err := json.Marshal(change)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled PolicyChange
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Type != PolicyChangeTypeEligibility {
		t.Errorf("expected type Eligibility, got %s", unmarshaled.Type)
	}
	if unmarshaled.Target != "read_file" {
		t.Errorf("expected target 'read_file', got %s", unmarshaled.Target)
	}
}

func TestEligibilityChangeData_Serialization(t *testing.T) {
	data := EligibilityChangeData{
		State:    agent.StateExplore,
		ToolName: "read_file",
		Add:      true,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled EligibilityChangeData
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.State != agent.StateExplore {
		t.Errorf("expected state Explore, got %s", unmarshaled.State)
	}
}

func TestTransitionChangeData_Serialization(t *testing.T) {
	data := TransitionChangeData{
		FromState: agent.StateExplore,
		ToState:   agent.StateDecide,
		Add:       true,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled TransitionChangeData
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.FromState != agent.StateExplore {
		t.Errorf("expected FromState Explore, got %s", unmarshaled.FromState)
	}
	if unmarshaled.ToState != agent.StateDecide {
		t.Errorf("expected ToState Decide, got %s", unmarshaled.ToState)
	}
}

func TestApprovalChangeData_Serialization(t *testing.T) {
	data := ApprovalChangeData{
		ToolName: "delete_file",
		Add:      true,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled ApprovalChangeData
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.ToolName != "delete_file" {
		t.Errorf("expected ToolName 'delete_file', got %s", unmarshaled.ToolName)
	}
	if !unmarshaled.Add {
		t.Error("expected Add to be true")
	}
}

// Summary Tests

func TestSummary_Structure(t *testing.T) {
	summary := Summary{
		TotalSuggestions:  100,
		AverageConfidence: 0.75,
		ByType:            make(map[SuggestionType]int64),
		ByStatus:          make(map[SuggestionStatus]int64),
		ByImpact:          make(map[ImpactLevel]int64),
	}

	summary.ByType[SuggestionTypeAddEligibility] = 50
	summary.ByStatus[SuggestionStatusPending] = 70
	summary.ByImpact[ImpactLevelMedium] = 40

	if summary.TotalSuggestions != 100 {
		t.Errorf("expected TotalSuggestions 100, got %d", summary.TotalSuggestions)
	}
	if summary.ByType[SuggestionTypeAddEligibility] != 50 {
		t.Error("expected ByType count to be set")
	}
}

// Impact Level Tests

func TestImpactLevels(t *testing.T) {
	levels := []ImpactLevel{ImpactLevelLow, ImpactLevelMedium, ImpactLevelHigh}

	if len(levels) != 3 {
		t.Errorf("expected 3 impact levels, got %d", len(levels))
	}

	// Verify each level has a string value
	for _, level := range levels {
		if string(level) == "" {
			t.Error("impact level should have non-empty string value")
		}
	}
}

// Error Sentinel Tests

func TestErrorSentinels_Defined(t *testing.T) {
	if ErrSuggestionNotFound == nil {
		t.Error("ErrSuggestionNotFound should not be nil")
	}
	if ErrSuggestionExists == nil {
		t.Error("ErrSuggestionExists should not be nil")
	}
	if ErrInvalidSuggestion == nil {
		t.Error("ErrInvalidSuggestion should not be nil")
	}
	if ErrInvalidSuggestionType == nil {
		t.Error("ErrInvalidSuggestionType should not be nil")
	}
	if ErrInvalidStatusTransition == nil {
		t.Error("ErrInvalidStatusTransition should not be nil")
	}
	if ErrGenerationFailed == nil {
		t.Error("ErrGenerationFailed should not be nil")
	}
	if ErrNoPatterns == nil {
		t.Error("ErrNoPatterns should not be nil")
	}
}

func TestErrorSentinels_HaveMessages(t *testing.T) {
	errors := []error{
		ErrSuggestionNotFound,
		ErrSuggestionExists,
		ErrInvalidSuggestion,
		ErrInvalidSuggestionType,
		ErrInvalidStatusTransition,
		ErrGenerationFailed,
		ErrNoPatterns,
	}

	for _, err := range errors {
		if err.Error() == "" {
			t.Errorf("error %v should have a message", err)
		}
	}
}

// Time-related Tests

func TestStatusChangedAt_Updated(t *testing.T) {
	s := NewSuggestion(SuggestionTypeAddEligibility, "test", "test")
	initialTime := s.CreatedAt

	time.Sleep(1 * time.Millisecond)
	s.Accept("proposal-1", "alice")

	if !s.StatusChangedAt.After(initialTime) {
		t.Error("expected StatusChangedAt to be after CreatedAt")
	}
}
