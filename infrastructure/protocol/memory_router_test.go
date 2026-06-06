package protocol

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domainprotocol "go.klarlabs.de/agent/domain/protocol"
)

func echoHandler() domainprotocol.HandlerFunc {
	return func(_ context.Context, msg domainprotocol.Message) (*domainprotocol.Message, error) {
		reply := domainprotocol.NewReply(msg, msg.Payload)
		return &reply, nil
	}
}

func TestMemoryRouter_RegisterAndDiscover(t *testing.T) {
	router := NewMemoryRouter(nil)

	desc := domainprotocol.AgentDescriptor{
		Name: "search-agent",
		Capabilities: []domainprotocol.Capability{
			{Name: "search", Actions: []string{"web-search", "db-search"}},
		},
		TrustLevel: domainprotocol.TrustFull,
	}

	if err := router.Register(desc, echoHandler()); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if router.AgentCount() != 1 {
		t.Errorf("AgentCount: got %d, want 1", router.AgentCount())
	}

	// Discover by capability
	found := router.Discover("search")
	if len(found) != 1 || found[0].Name != "search-agent" {
		t.Errorf("Discover: got %v", found)
	}

	// Discover by action
	found = router.DiscoverAction("web-search")
	if len(found) != 1 {
		t.Errorf("DiscoverAction: got %d, want 1", len(found))
	}

	// Not found
	found = router.Discover("code-review")
	if len(found) != 0 {
		t.Errorf("unexpected discovery for code-review")
	}
}

func TestMemoryRouter_DuplicateRegister(t *testing.T) {
	router := NewMemoryRouter(nil)
	desc := domainprotocol.AgentDescriptor{Name: "agent-a"}

	if err := router.Register(desc, echoHandler()); err != nil {
		t.Fatal(err)
	}
	if err := router.Register(desc, echoHandler()); err == nil {
		t.Fatal("expected error on duplicate register")
	}
}

func TestMemoryRouter_Unregister(t *testing.T) {
	router := NewMemoryRouter(nil)
	desc := domainprotocol.AgentDescriptor{Name: "agent-a"}
	_ = router.Register(desc, echoHandler())

	if err := router.Unregister("agent-a"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if router.AgentCount() != 0 {
		t.Error("expected 0 agents after unregister")
	}
	if err := router.Unregister("nonexistent"); err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestMemoryRouter_RequestReply(t *testing.T) {
	router := NewMemoryRouter(nil)

	desc := domainprotocol.AgentDescriptor{
		Name: "echo-agent",
		Capabilities: []domainprotocol.Capability{
			{Name: "echo", Actions: []string{"echo"}},
		},
	}
	_ = router.Register(desc, echoHandler())

	payload, _ := json.Marshal(map[string]string{"msg": "hello"})
	msg := domainprotocol.NewRequest("echo-agent", "echo", payload,
		domainprotocol.WithTimeout(5*time.Second),
	)
	msg.Sender = "caller"

	reply, err := router.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if reply == nil {
		t.Fatal("expected non-nil reply")
	}
	if reply.CorrelationID != msg.CorrelationID {
		t.Error("reply correlation ID mismatch")
	}
	if string(reply.Payload) != string(payload) {
		t.Errorf("payload: got %s, want %s", reply.Payload, payload)
	}
}

func TestMemoryRouter_Notify(t *testing.T) {
	received := make(chan domainprotocol.Message, 1)
	handler := domainprotocol.HandlerFunc(func(_ context.Context, msg domainprotocol.Message) (*domainprotocol.Message, error) {
		received <- msg
		return nil, nil
	})

	router := NewMemoryRouter(nil)
	desc := domainprotocol.AgentDescriptor{Name: "listener"}
	_ = router.Register(desc, handler)

	msg := domainprotocol.NewNotify("listener", "status-update", nil)
	msg.Sender = "sender"

	reply, err := router.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send notify: %v", err)
	}
	if reply != nil {
		t.Error("expected nil reply for notify")
	}

	select {
	case got := <-received:
		if got.Action != "status-update" {
			t.Errorf("action: got %s", got.Action)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

func TestMemoryRouter_TrustPolicyDeniesAction(t *testing.T) {
	policy := domainprotocol.NewTrustPolicy(domainprotocol.TrustNone)
	policy.SetPermission("untrusted", domainprotocol.Permission{
		DeniedActions: []string{"delete"},
	})

	router := NewMemoryRouter(policy)
	desc := domainprotocol.AgentDescriptor{Name: "target"}
	_ = router.Register(desc, echoHandler())

	msg := domainprotocol.NewRequest("target", "delete", nil)
	msg.Sender = "untrusted"

	_, err := router.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for denied action")
	}
}

func TestMemoryRouter_AgentNotFound(t *testing.T) {
	router := NewMemoryRouter(nil)

	msg := domainprotocol.NewRequest("nonexistent", "search", nil)
	_, err := router.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}
