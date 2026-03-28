package protocol

import "context"

// Router dispatches messages between agents.
type Router interface {
	// Send delivers a message and waits for a reply (for requests).
	// For notify/broadcast messages, returns immediately with nil reply.
	Send(ctx context.Context, msg Message) (*Message, error)

	// Register adds an agent to the router with its descriptor.
	Register(descriptor AgentDescriptor, handler Handler) error

	// Unregister removes an agent from the router.
	Unregister(agentName string) error

	// Discover returns agents matching the given capability.
	Discover(capability string) []AgentDescriptor

	// DiscoverAction returns agents that handle the given action.
	DiscoverAction(action string) []AgentDescriptor
}

// Handler processes incoming messages for an agent.
type Handler interface {
	// HandleMessage processes an incoming message and returns an optional reply.
	HandleMessage(ctx context.Context, msg Message) (*Message, error)
}

// HandlerFunc adapts a function to the Handler interface.
type HandlerFunc func(ctx context.Context, msg Message) (*Message, error)

// HandleMessage implements Handler.
func (f HandlerFunc) HandleMessage(ctx context.Context, msg Message) (*Message, error) {
	return f(ctx, msg)
}
