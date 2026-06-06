package task

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
)

func TestNewContext(t *testing.T) {
	tc := NewContext("task-1", "run-root")
	if tc.ID != "task-1" {
		t.Errorf("ID: got %s, want task-1", tc.ID)
	}
	if tc.RootRunID != "run-root" {
		t.Errorf("RootRunID: got %s, want run-root", tc.RootRunID)
	}
}

func TestContext_Vars(t *testing.T) {
	tc := NewContext("t1", "r1")

	// Initially empty
	if _, ok := tc.GetVar("key"); ok {
		t.Error("expected var not found initially")
	}

	// Set and get
	tc.SetVar("key", "value")
	v, ok := tc.GetVar("key")
	if !ok || v != "value" {
		t.Errorf("GetVar: got %v, %v", v, ok)
	}

	// Overwrite
	tc.SetVar("key", "new")
	v, _ = tc.GetVar("key")
	if v != "new" {
		t.Errorf("overwrite: got %v, want new", v)
	}

	// Snapshot
	tc.SetVar("k2", 42)
	snap := tc.Vars()
	if len(snap) != 2 {
		t.Errorf("snapshot length: got %d, want 2", len(snap))
	}

	// Snapshot is a copy — mutation doesn't affect original
	snap["k3"] = "injected"
	if _, ok := tc.GetVar("k3"); ok {
		t.Error("snapshot mutation leaked to context")
	}
}

func TestContext_Evidence(t *testing.T) {
	tc := NewContext("t1", "r1")

	if len(tc.Evidence()) != 0 {
		t.Error("expected empty evidence initially")
	}

	e1 := agent.NewToolEvidence("tool1", json.RawMessage(`{"a":1}`))
	e2 := agent.NewToolEvidence("tool2", json.RawMessage(`{"b":2}`))
	tc.AddEvidence(e1)
	tc.AddEvidence(e2)

	evidence := tc.Evidence()
	if len(evidence) != 2 {
		t.Fatalf("evidence count: got %d, want 2", len(evidence))
	}
	if evidence[0].Source != "tool1" || evidence[1].Source != "tool2" {
		t.Error("evidence order or content incorrect")
	}

	// Copy safety
	evidence[0].Source = "mutated"
	if tc.Evidence()[0].Source == "mutated" {
		t.Error("evidence copy mutation leaked")
	}
}

func TestContext_ArtifactRefs(t *testing.T) {
	tc := NewContext("t1", "r1")

	if len(tc.ArtifactRefs()) != 0 {
		t.Error("expected empty artifacts initially")
	}

	tc.AddArtifactRef("art-1")
	tc.AddArtifactRef("art-2")

	refs := tc.ArtifactRefs()
	if len(refs) != 2 || refs[0] != "art-1" || refs[1] != "art-2" {
		t.Errorf("artifact refs: got %v", refs)
	}
}

func TestContext_ConcurrentAccess(t *testing.T) {
	tc := NewContext("t1", "r1")
	var wg sync.WaitGroup

	// 50 goroutines writing vars
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tc.SetVar("key", n)
			tc.GetVar("key")
			tc.Vars()
		}(i)
	}

	// 50 goroutines adding evidence
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tc.AddEvidence(agent.NewToolEvidence("tool", json.RawMessage(`{}`)))
			tc.Evidence()
		}(i)
	}

	// 50 goroutines adding artifacts
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tc.AddArtifactRef("art")
			tc.ArtifactRefs()
		}(i)
	}

	wg.Wait()

	if len(tc.Evidence()) != 50 {
		t.Errorf("evidence count after concurrent writes: got %d, want 50", len(tc.Evidence()))
	}
}

func TestRunIDContext(t *testing.T) {
	ctx := context.Background()

	// Not set
	if id := RunIDFromContext(ctx); id != "" {
		t.Errorf("expected empty, got %s", id)
	}

	// Set and read
	ctx = WithRunID(ctx, "run-123")
	if id := RunIDFromContext(ctx); id != "run-123" {
		t.Errorf("expected run-123, got %s", id)
	}
}
