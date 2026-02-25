// Package mqtt provides IoT messaging tools for agent-go.
//
// The pack uses an interface-based approach, allowing any MQTT client
// implementation (Eclipse Paho, etc.) to be plugged in.
package mqtt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// MQTTClient provides MQTT messaging capabilities.
type MQTTClient interface {
	Connect(ctx context.Context, broker string, opts ConnectOptions) (string, error) // returns connection ID
	Disconnect(ctx context.Context, connID string) error
	Publish(ctx context.Context, connID string, msg PublishMessage) error
	Subscribe(ctx context.Context, connID string, topic string, qos int) error
	Unsubscribe(ctx context.Context, connID string, topic string) error
	ListTopics(ctx context.Context, connID string, filter string) ([]TopicInfo, error)
	Receive(ctx context.Context, connID string, topic string, opts ReceiveOptions) ([]Message, error)
}

// ConnectOptions configures broker connections.
type ConnectOptions struct {
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	TLS       bool   `json:"tls,omitempty"`
	KeepAlive int    `json:"keep_alive_seconds,omitempty"`
	CleanSession bool `json:"clean_session,omitempty"`
}

// PublishMessage describes a message to publish.
type PublishMessage struct {
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
	QoS     int    `json:"qos,omitempty"`     // 0, 1, or 2
	Retain  bool   `json:"retain,omitempty"`
}

// TopicInfo describes an MQTT topic.
type TopicInfo struct {
	Name        string `json:"name"`
	Subscribers int    `json:"subscribers,omitempty"`
	Retained    bool   `json:"retained,omitempty"`
}

// ReceiveOptions configures message receiving.
type ReceiveOptions struct {
	Limit   int `json:"limit,omitempty"`
	Timeout int `json:"timeout_ms,omitempty"`
}

// Message represents a received MQTT message.
type Message struct {
	Topic     string `json:"topic"`
	Payload   string `json:"payload"`
	QoS       int    `json:"qos"`
	Retained  bool   `json:"retained"`
	Timestamp string `json:"timestamp,omitempty"`
}

// Config holds MQTT pack configuration.
type Config struct {
	Client MQTTClient
}

// Pack returns the MQTT messaging tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &mqttPack{cfg: cfg}

	return pack.NewBuilder("mqtt").
		WithDescription("IoT messaging tools: connect, disconnect, publish, subscribe, list_topics, receive").
		WithVersion("1.0.0").
		AddTools(
			p.connectTool(), p.disconnectTool(), p.publishTool(),
			p.subscribeTool(), p.unsubscribeTool(), p.listTopicsTool(),
			p.receiveTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type mqttPack struct{ cfg Config }

func (p *mqttPack) connectTool() tool.Tool {
	return tool.NewBuilder("mqtt_connect").
		WithDescription("Connect to an MQTT broker").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Broker       string `json:"broker"`
				ClientID     string `json:"client_id,omitempty"`
				Username     string `json:"username,omitempty"`
				Password     string `json:"password,omitempty"`
				TLS          bool   `json:"tls,omitempty"`
				KeepAlive    int    `json:"keep_alive_seconds,omitempty"`
				CleanSession bool   `json:"clean_session,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Broker == "" {
				return tool.Result{}, fmt.Errorf("broker is required")
			}
			connID, err := p.cfg.Client.Connect(ctx, in.Broker, ConnectOptions{
				ClientID: in.ClientID, Username: in.Username, Password: in.Password,
				TLS: in.TLS, KeepAlive: in.KeepAlive, CleanSession: in.CleanSession,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("connect failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"connection_id": connID, "broker": in.Broker})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *mqttPack) disconnectTool() tool.Tool {
	return tool.NewBuilder("mqtt_disconnect").
		WithDescription("Disconnect from an MQTT broker").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ConnectionID string `json:"connection_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ConnectionID == "" {
				return tool.Result{}, fmt.Errorf("connection_id is required")
			}
			err := p.cfg.Client.Disconnect(ctx, in.ConnectionID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("disconnect failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"connection_id": in.ConnectionID, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *mqttPack) publishTool() tool.Tool {
	return tool.NewBuilder("mqtt_publish").
		WithDescription("Publish a message to an MQTT topic").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ConnectionID string `json:"connection_id"`
				Topic        string `json:"topic"`
				Payload      string `json:"payload"`
				QoS          int    `json:"qos,omitempty"`
				Retain       bool   `json:"retain,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ConnectionID == "" {
				return tool.Result{}, fmt.Errorf("connection_id is required")
			}
			if in.Topic == "" {
				return tool.Result{}, fmt.Errorf("topic is required")
			}
			err := p.cfg.Client.Publish(ctx, in.ConnectionID, PublishMessage{
				Topic: in.Topic, Payload: in.Payload, QoS: in.QoS, Retain: in.Retain,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("publish failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"topic": in.Topic, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *mqttPack) subscribeTool() tool.Tool {
	return tool.NewBuilder("mqtt_subscribe").
		WithDescription("Subscribe to an MQTT topic").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ConnectionID string `json:"connection_id"`
				Topic        string `json:"topic"`
				QoS          int    `json:"qos,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ConnectionID == "" {
				return tool.Result{}, fmt.Errorf("connection_id is required")
			}
			if in.Topic == "" {
				return tool.Result{}, fmt.Errorf("topic is required")
			}
			err := p.cfg.Client.Subscribe(ctx, in.ConnectionID, in.Topic, in.QoS)
			if err != nil {
				return tool.Result{}, fmt.Errorf("subscribe failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"topic": in.Topic, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *mqttPack) unsubscribeTool() tool.Tool {
	return tool.NewBuilder("mqtt_unsubscribe").
		WithDescription("Unsubscribe from an MQTT topic").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ConnectionID string `json:"connection_id"`
				Topic        string `json:"topic"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ConnectionID == "" {
				return tool.Result{}, fmt.Errorf("connection_id is required")
			}
			if in.Topic == "" {
				return tool.Result{}, fmt.Errorf("topic is required")
			}
			err := p.cfg.Client.Unsubscribe(ctx, in.ConnectionID, in.Topic)
			if err != nil {
				return tool.Result{}, fmt.Errorf("unsubscribe failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"topic": in.Topic, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *mqttPack) listTopicsTool() tool.Tool {
	return tool.NewBuilder("mqtt_list_topics").
		WithDescription("List MQTT topics matching a filter").
		ReadOnly().Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ConnectionID string `json:"connection_id"`
				Filter       string `json:"filter,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ConnectionID == "" {
				return tool.Result{}, fmt.Errorf("connection_id is required")
			}
			if in.Filter == "" {
				in.Filter = "#"
			}
			topics, err := p.cfg.Client.ListTopics(ctx, in.ConnectionID, in.Filter)
			if err != nil {
				return tool.Result{}, fmt.Errorf("list topics failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"count": len(topics), "topics": topics})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *mqttPack) receiveTool() tool.Tool {
	return tool.NewBuilder("mqtt_receive").
		WithDescription("Receive messages from a subscribed MQTT topic").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ConnectionID string `json:"connection_id"`
				Topic        string `json:"topic"`
				Limit        int    `json:"limit,omitempty"`
				Timeout      int    `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ConnectionID == "" {
				return tool.Result{}, fmt.Errorf("connection_id is required")
			}
			if in.Topic == "" {
				return tool.Result{}, fmt.Errorf("topic is required")
			}
			if in.Limit == 0 {
				in.Limit = 10
			}
			messages, err := p.cfg.Client.Receive(ctx, in.ConnectionID, in.Topic, ReceiveOptions{
				Limit: in.Limit, Timeout: in.Timeout,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("receive failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"count": len(messages), "messages": messages})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
