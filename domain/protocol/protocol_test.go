package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewRequest(t *testing.T) {
	payload, _ := json.Marshal(map[string]string{"query": "test"})
	msg := NewRequest("agent-b", "search", payload,
		WithTimeout(30*time.Second),
		WithTaskID("task-1"),
		WithRunID("run-1"),
		WithMetadata("priority", "high"),
	)

	if msg.Type != TypeRequest {
		t.Errorf("Type: got %s, want request", msg.Type)
	}
	if msg.Receiver != "agent-b" {
		t.Errorf("Receiver: got %s, want agent-b", msg.Receiver)
	}
	if msg.Action != "search" {
		t.Errorf("Action: got %s, want search", msg.Action)
	}
	if msg.Timeout != 30*time.Second {
		t.Errorf("Timeout: got %v, want 30s", msg.Timeout)
	}
	if msg.TaskID != "task-1" {
		t.Errorf("TaskID: got %s, want task-1", msg.TaskID)
	}
	if msg.ID == "" || msg.CorrelationID == "" {
		t.Error("expected non-empty ID and CorrelationID")
	}
	if msg.ID != msg.CorrelationID {
		t.Error("initiating message should have ID == CorrelationID")
	}
	if !msg.IsRequest() {
		t.Error("expected IsRequest() == true")
	}
	if msg.Metadata["priority"] != "high" {
		t.Error("metadata not set")
	}
}

func TestNewReply(t *testing.T) {
	req := NewRequest("agent-b", "search", nil)
	req.Sender = "agent-a"

	replyPayload, _ := json.Marshal(map[string]string{"result": "found"})
	reply := NewReply(req, replyPayload)

	if reply.Type != TypeReply {
		t.Errorf("Type: got %s, want reply", reply.Type)
	}
	if reply.CorrelationID != req.CorrelationID {
		t.Error("reply should share request's CorrelationID")
	}
	if reply.Receiver != req.Sender {
		t.Errorf("Receiver: got %s, want %s", reply.Receiver, req.Sender)
	}
	if reply.Sender != req.Receiver {
		t.Errorf("Sender: got %s, want %s", reply.Sender, req.Receiver)
	}
	if reply.IsRequest() {
		t.Error("reply should not be a request")
	}
	if !reply.IsReply() {
		t.Error("expected IsReply() == true")
	}
}

func TestNewErrorReply(t *testing.T) {
	req := NewRequest("agent-b", "search", nil)
	req.Sender = "agent-a"

	errReply := NewErrorReply(req, "something failed")

	if errReply.Type != TypeError {
		t.Errorf("Type: got %s, want error", errReply.Type)
	}
	if !errReply.IsError() {
		t.Error("expected IsError() == true")
	}
	if !errReply.IsReply() {
		t.Error("error reply should also be a reply")
	}
}

func TestNewNotify(t *testing.T) {
	msg := NewNotify("agent-b", "status-update", nil)
	if msg.Type != TypeNotify {
		t.Errorf("Type: got %s, want notify", msg.Type)
	}
	if msg.IsRequest() {
		t.Error("notify should not be a request")
	}
}

func TestNewBroadcast(t *testing.T) {
	msg := NewBroadcast("shutdown", nil)
	if msg.Type != TypeBroadcast {
		t.Errorf("Type: got %s, want broadcast", msg.Type)
	}
	if msg.Receiver != "" {
		t.Error("broadcast should have empty receiver")
	}
}

func TestAgentDescriptor_Capabilities(t *testing.T) {
	desc := AgentDescriptor{
		Name: "research-agent",
		Capabilities: []Capability{
			{Name: "search", Actions: []string{"web-search", "db-search"}},
			{Name: "summarize", Actions: []string{"summarize-text"}},
		},
		TrustLevel: TrustLimited,
	}

	if !desc.HasCapability("search") {
		t.Error("expected HasCapability(search) == true")
	}
	if desc.HasCapability("code-review") {
		t.Error("expected HasCapability(code-review) == false")
	}
	if !desc.HandlesAction("web-search") {
		t.Error("expected HandlesAction(web-search) == true")
	}
	if desc.HandlesAction("deploy") {
		t.Error("expected HandlesAction(deploy) == false")
	}
}

func TestTrustPolicy(t *testing.T) {
	policy := NewTrustPolicy(TrustReadOnly)

	// Default trust
	if policy.TrustFor("unknown") != TrustReadOnly {
		t.Error("expected default trust level")
	}

	// Set specific trust
	policy.SetTrust("trusted-agent", TrustFull)
	if policy.TrustFor("trusted-agent") != TrustFull {
		t.Error("expected full trust for trusted-agent")
	}

	// Permissions
	policy.SetPermission("limited-agent", Permission{
		AllowedActions: []string{"search", "read"},
		DeniedActions:  []string{"delete"},
		MaxBudget:      10,
	})

	if !policy.IsActionAllowed("limited-agent", "search") {
		t.Error("search should be allowed")
	}
	if policy.IsActionAllowed("limited-agent", "write") {
		t.Error("write should not be allowed (not in allowed list)")
	}
	if policy.IsActionAllowed("limited-agent", "delete") {
		t.Error("delete should be denied")
	}

	// Unknown agent gets no restrictions
	if !policy.IsActionAllowed("random-agent", "anything") {
		t.Error("unknown agent should have no restrictions by default")
	}
}

func TestTrustLevel_String(t *testing.T) {
	tests := []struct {
		level TrustLevel
		want  string
	}{
		{TrustNone, "none"},
		{TrustReadOnly, "read_only"},
		{TrustLimited, "limited"},
		{TrustFull, "full"},
		{TrustLevel(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("TrustLevel(%d).String() = %s, want %s", tt.level, got, tt.want)
		}
	}
}
