// Package gpio provides hardware GPIO tools for agent-go.
//
// The pack uses an interface-based approach, allowing any GPIO library
// (sysfs, gpiod, pigpio, etc.) to be plugged in.
package gpio

import (
	"context"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// GPIOController provides hardware GPIO capabilities.
type GPIOController interface {
	ConfigurePin(ctx context.Context, pin int, cfg PinConfig) error
	ReadPin(ctx context.Context, pin int) (*PinState, error)
	WritePin(ctx context.Context, pin int, value int) error
	PWM(ctx context.Context, pin int, opts PWMOptions) error
	WatchPin(ctx context.Context, pin int, opts WatchOptions) ([]PinEvent, error)
	ListPins(ctx context.Context) ([]PinInfo, error)
}

// PinConfig configures a GPIO pin.
type PinConfig struct {
	Direction string `json:"direction"`      // "input", "output"
	Pull      string `json:"pull,omitempty"` // "up", "down", "none"
	Edge      string `json:"edge,omitempty"` // "rising", "falling", "both", "none"
}

// PinState represents the current state of a GPIO pin.
type PinState struct {
	Pin       int    `json:"pin"`
	Value     int    `json:"value"` // 0 or 1
	Direction string `json:"direction"`
}

// PWMOptions configures PWM output.
type PWMOptions struct {
	Frequency int     `json:"frequency_hz"`
	DutyCycle float64 `json:"duty_cycle"`            // 0.0 to 1.0
	Duration  int     `json:"duration_ms,omitempty"` // 0 = indefinite
}

// WatchOptions configures pin watching.
type WatchOptions struct {
	Edge    string `json:"edge,omitempty"` // "rising", "falling", "both"
	Timeout int    `json:"timeout_ms,omitempty"`
	Count   int    `json:"count,omitempty"` // max events to capture
}

// PinEvent represents a GPIO pin state change event.
type PinEvent struct {
	Pin       int    `json:"pin"`
	Value     int    `json:"value"`
	Edge      string `json:"edge"`
	Timestamp string `json:"timestamp"`
}

// PinInfo describes an available GPIO pin.
type PinInfo struct {
	Pin       int    `json:"pin"`
	Name      string `json:"name,omitempty"`
	Direction string `json:"direction"`
	Value     int    `json:"value"`
	InUse     bool   `json:"in_use"`
}

// Config holds GPIO pack configuration.
type Config struct {
	Controller GPIOController
}

// Pack returns the hardware GPIO tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &gpioPack{cfg: cfg}

	return pack.NewBuilder("gpio").
		WithDescription("Hardware GPIO tools: read_pin, write_pin, pwm, configure_pin, watch_pin, list_pins").
		WithVersion("1.0.0").
		AddTools(
			p.configurePinTool(), p.readPinTool(), p.writePinTool(),
			p.pwmTool(), p.watchPinTool(), p.listPinsTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type gpioPack struct{ cfg Config }

func (p *gpioPack) configurePinTool() tool.Tool {
	return tool.NewBuilder("gpio_configure_pin").
		WithDescription("Configure a GPIO pin's direction and pull resistor").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pin       int    `json:"pin"`
				Direction string `json:"direction"`
				Pull      string `json:"pull,omitempty"`
				Edge      string `json:"edge,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Direction == "" {
				return tool.Result{}, fmt.Errorf("direction is required (input or output)")
			}
			err := p.cfg.Controller.ConfigurePin(ctx, in.Pin, PinConfig{
				Direction: in.Direction, Pull: in.Pull, Edge: in.Edge,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("configure pin failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"pin": in.Pin, "direction": in.Direction, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *gpioPack) readPinTool() tool.Tool {
	return tool.NewBuilder("gpio_read_pin").
		WithDescription("Read the current value of a GPIO pin").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pin int `json:"pin"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			state, err := p.cfg.Controller.ReadPin(ctx, in.Pin)
			if err != nil {
				return tool.Result{}, fmt.Errorf("read pin failed: %w", err)
			}
			output, _ := json.Marshal(state)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *gpioPack) writePinTool() tool.Tool {
	return tool.NewBuilder("gpio_write_pin").
		WithDescription("Write a value to a GPIO output pin").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pin   int `json:"pin"`
				Value int `json:"value"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Value != 0 && in.Value != 1 {
				return tool.Result{}, fmt.Errorf("value must be 0 or 1")
			}
			err := p.cfg.Controller.WritePin(ctx, in.Pin, in.Value)
			if err != nil {
				return tool.Result{}, fmt.Errorf("write pin failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"pin": in.Pin, "value": in.Value, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *gpioPack) pwmTool() tool.Tool {
	return tool.NewBuilder("gpio_pwm").
		WithDescription("Set PWM output on a GPIO pin").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pin       int     `json:"pin"`
				Frequency int     `json:"frequency_hz"`
				DutyCycle float64 `json:"duty_cycle"`
				Duration  int     `json:"duration_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Frequency <= 0 {
				return tool.Result{}, fmt.Errorf("frequency_hz must be positive")
			}
			if in.DutyCycle < 0 || in.DutyCycle > 1 {
				return tool.Result{}, fmt.Errorf("duty_cycle must be between 0.0 and 1.0")
			}
			err := p.cfg.Controller.PWM(ctx, in.Pin, PWMOptions{
				Frequency: in.Frequency, DutyCycle: in.DutyCycle, Duration: in.Duration,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("pwm failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{
				"pin": in.Pin, "frequency_hz": in.Frequency,
				"duty_cycle": in.DutyCycle, "success": true,
			})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *gpioPack) watchPinTool() tool.Tool {
	return tool.NewBuilder("gpio_watch_pin").
		WithDescription("Watch a GPIO pin for state changes").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pin     int    `json:"pin"`
				Edge    string `json:"edge,omitempty"`
				Timeout int    `json:"timeout_ms,omitempty"`
				Count   int    `json:"count,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Edge == "" {
				in.Edge = "both"
			}
			if in.Count == 0 {
				in.Count = 10
			}
			events, err := p.cfg.Controller.WatchPin(ctx, in.Pin, WatchOptions{
				Edge: in.Edge, Timeout: in.Timeout, Count: in.Count,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("watch pin failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"pin": in.Pin, "events": events, "count": len(events)})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *gpioPack) listPinsTool() tool.Tool {
	return tool.NewBuilder("gpio_list_pins").
		WithDescription("List available GPIO pins and their current state").
		ReadOnly().Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pins, err := p.cfg.Controller.ListPins(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("list pins failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"count": len(pins), "pins": pins})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
