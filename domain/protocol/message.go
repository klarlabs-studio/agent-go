// Package protocol defines the agent-to-agent communication protocol.
//
// Messages are the fundamental unit of inter-agent communication. Each message
// has an envelope (routing, correlation, timing) and a typed payload. The protocol
// supports request-reply, fire-and-forget, and broadcast patterns.
//
// Example:
//
//	msg := protocol.NewRequest("research-agent", "find data",
//	    json.RawMessage(`{"query":"revenue Q4"}`),
//	    protocol.WithTimeout(30*time.Second),
//	)
//	reply, err := router.Send(ctx, msg)
package protocol

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MessageType classifies the communication pattern.
type MessageType string

const (
	// TypeRequest expects a reply from the receiver.
	TypeRequest MessageType = "request"
	// TypeReply is a response to a request.
	TypeReply MessageType = "reply"
	// TypeNotify is fire-and-forget (no reply expected).
	TypeNotify MessageType = "notify"
	// TypeBroadcast is sent to all agents matching a capability.
	TypeBroadcast MessageType = "broadcast"
	// TypeError is an error response to a request.
	TypeError MessageType = "error"
)

// Message is the envelope for inter-agent communication.
type Message struct {
	// ID is the unique message identifier.
	ID string `json:"id"`

	// CorrelationID links related messages (request + reply share this).
	CorrelationID string `json:"correlation_id"`

	// Type is the message pattern (request, reply, notify, broadcast).
	Type MessageType `json:"type"`

	// Sender is the agent name or ID that sent this message.
	Sender string `json:"sender"`

	// Receiver is the target agent name or ID.
	// Empty for broadcasts (routed by capability).
	Receiver string `json:"receiver,omitempty"`

	// TaskID links this message to a task context for shared state.
	TaskID string `json:"task_id,omitempty"`

	// RunID is the sender's current run ID for tracing.
	RunID string `json:"run_id,omitempty"`

	// Action describes what the sender wants (e.g., "research", "summarize").
	Action string `json:"action"`

	// Payload is the message-specific data.
	Payload json.RawMessage `json:"payload,omitempty"`

	// Metadata holds optional key-value pairs (priority, tags, etc.).
	Metadata map[string]string `json:"metadata,omitempty"`

	// Timeout is the deadline for a reply (only for requests).
	Timeout time.Duration `json:"timeout,omitempty"`

	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`

	// ReplyTo overrides where replies should be sent (for routing).
	ReplyTo string `json:"reply_to,omitempty"`
}

// MessageOption configures a message.
type MessageOption func(*Message)

// WithTimeout sets the reply deadline.
func WithTimeout(d time.Duration) MessageOption {
	return func(m *Message) { m.Timeout = d }
}

// WithTaskID links the message to a task context.
func WithTaskID(id string) MessageOption {
	return func(m *Message) { m.TaskID = id }
}

// WithRunID sets the sender's run ID.
func WithRunID(id string) MessageOption {
	return func(m *Message) { m.RunID = id }
}

// WithMetadata adds metadata key-value pairs.
func WithMetadata(key, value string) MessageOption {
	return func(m *Message) {
		if m.Metadata == nil {
			m.Metadata = make(map[string]string)
		}
		m.Metadata[key] = value
	}
}

// WithReplyTo sets the reply routing address.
func WithReplyTo(addr string) MessageOption {
	return func(m *Message) { m.ReplyTo = addr }
}

// NewRequest creates a request message expecting a reply.
func NewRequest(receiver, action string, payload json.RawMessage, opts ...MessageOption) Message {
	return newMessage(TypeRequest, receiver, action, payload, opts...)
}

// NewReply creates a reply to a request message.
func NewReply(request Message, payload json.RawMessage) Message {
	return Message{
		ID:            uuid.New().String(),
		CorrelationID: request.CorrelationID,
		Type:          TypeReply,
		Sender:        request.Receiver,
		Receiver:      request.Sender,
		TaskID:        request.TaskID,
		Action:        request.Action,
		Payload:       payload,
		Timestamp:     time.Now(),
	}
}

// NewErrorReply creates an error reply to a request message.
func NewErrorReply(request Message, errMsg string) Message {
	payload, _ := json.Marshal(map[string]string{"error": errMsg})
	msg := NewReply(request, payload)
	msg.Type = TypeError
	return msg
}

// NewNotify creates a fire-and-forget message.
func NewNotify(receiver, action string, payload json.RawMessage, opts ...MessageOption) Message {
	return newMessage(TypeNotify, receiver, action, payload, opts...)
}

// NewBroadcast creates a broadcast message routed by capability.
func NewBroadcast(action string, payload json.RawMessage, opts ...MessageOption) Message {
	return newMessage(TypeBroadcast, "", action, payload, opts...)
}

func newMessage(msgType MessageType, receiver, action string, payload json.RawMessage, opts ...MessageOption) Message {
	id := uuid.New().String()
	msg := Message{
		ID:            id,
		CorrelationID: id, // same as ID for initiating messages
		Type:          msgType,
		Receiver:      receiver,
		Action:        action,
		Payload:       payload,
		Timestamp:     time.Now(),
	}
	for _, opt := range opts {
		opt(&msg)
	}
	return msg
}

// IsRequest returns true if this message expects a reply.
func (m Message) IsRequest() bool { return m.Type == TypeRequest }

// IsReply returns true if this message is a reply.
func (m Message) IsReply() bool { return m.Type == TypeReply || m.Type == TypeError }

// IsError returns true if this message is an error reply.
func (m Message) IsError() bool { return m.Type == TypeError }
