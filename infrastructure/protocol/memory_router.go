// Package protocol provides infrastructure implementations for the agent protocol.
package protocol

import (
	"context"
	"fmt"
	"sync"
	"time"

	domainprotocol "go.klarlabs.de/agent/domain/protocol"
)

// MemoryRouter is an in-process message router for agent communication.
// It routes messages between agents registered in the same process.
type MemoryRouter struct {
	mu     sync.RWMutex
	agents map[string]registeredAgent
	policy *domainprotocol.TrustPolicy
}

type registeredAgent struct {
	descriptor domainprotocol.AgentDescriptor
	handler    domainprotocol.Handler
}

// NewMemoryRouter creates a new in-process router with the given trust policy.
// If policy is nil, all agents are trusted by default.
func NewMemoryRouter(policy *domainprotocol.TrustPolicy) *MemoryRouter {
	if policy == nil {
		policy = domainprotocol.NewTrustPolicy(domainprotocol.TrustFull)
	}
	return &MemoryRouter{
		agents: make(map[string]registeredAgent),
		policy: policy,
	}
}

// Register adds an agent to the router.
func (r *MemoryRouter) Register(desc domainprotocol.AgentDescriptor, handler domainprotocol.Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[desc.Name]; exists {
		return fmt.Errorf("agent %q already registered", desc.Name)
	}

	r.agents[desc.Name] = registeredAgent{descriptor: desc, handler: handler}
	return nil
}

// Unregister removes an agent from the router.
func (r *MemoryRouter) Unregister(agentName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agentName]; !exists {
		return domainprotocol.ErrAgentNotFound
	}

	delete(r.agents, agentName)
	return nil
}

// Send delivers a message to the target agent.
// For requests, it waits for a reply with the configured timeout.
// For notifications and broadcasts, it returns immediately.
func (r *MemoryRouter) Send(ctx context.Context, msg domainprotocol.Message) (*domainprotocol.Message, error) {
	// Enforce trust policy
	if msg.Sender != "" && !r.policy.IsActionAllowed(msg.Sender, msg.Action) {
		return nil, fmt.Errorf("%w: %s cannot perform %s", domainprotocol.ErrActionDenied, msg.Sender, msg.Action)
	}

	switch msg.Type {
	case domainprotocol.TypeBroadcast:
		return nil, r.broadcast(ctx, msg)
	case domainprotocol.TypeNotify:
		return nil, r.deliver(ctx, msg)
	case domainprotocol.TypeRequest:
		return r.requestReply(ctx, msg)
	default:
		return nil, fmt.Errorf("unsupported message type: %s", msg.Type)
	}
}

// Discover returns agents with the given capability.
func (r *MemoryRouter) Discover(capability string) []domainprotocol.AgentDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domainprotocol.AgentDescriptor
	for _, reg := range r.agents {
		if reg.descriptor.HasCapability(capability) {
			result = append(result, reg.descriptor)
		}
	}
	return result
}

// DiscoverAction returns agents that handle the given action.
func (r *MemoryRouter) DiscoverAction(action string) []domainprotocol.AgentDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domainprotocol.AgentDescriptor
	for _, reg := range r.agents {
		if reg.descriptor.HandlesAction(action) {
			result = append(result, reg.descriptor)
		}
	}
	return result
}

func (r *MemoryRouter) requestReply(ctx context.Context, msg domainprotocol.Message) (*domainprotocol.Message, error) {
	if msg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, msg.Timeout)
		defer cancel()
	}

	r.mu.RLock()
	reg, ok := r.agents[msg.Receiver]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", domainprotocol.ErrAgentNotFound, msg.Receiver)
	}

	reply, err := reg.handler.HandleMessage(ctx, msg)
	if err != nil {
		return nil, err
	}

	return reply, nil
}

func (r *MemoryRouter) deliver(ctx context.Context, msg domainprotocol.Message) error {
	r.mu.RLock()
	reg, ok := r.agents[msg.Receiver]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", domainprotocol.ErrAgentNotFound, msg.Receiver)
	}

	_, err := reg.handler.HandleMessage(ctx, msg)
	return err
}

func (r *MemoryRouter) broadcast(ctx context.Context, msg domainprotocol.Message) error {
	r.mu.RLock()
	handlers := make([]domainprotocol.Handler, 0)
	for _, reg := range r.agents {
		if reg.descriptor.HandlesAction(msg.Action) {
			handlers = append(handlers, reg.handler)
		}
	}
	r.mu.RUnlock()

	var firstErr error
	for _, h := range handlers {
		if _, err := h.HandleMessage(ctx, msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// AgentCount returns the number of registered agents.
func (r *MemoryRouter) AgentCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// Ensure MemoryRouter implements Router.
var _ domainprotocol.Router = (*MemoryRouter)(nil)

// Suppress unused import warning for time.
var _ = time.Now
