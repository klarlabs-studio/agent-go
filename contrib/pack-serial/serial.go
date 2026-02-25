// Package serial provides serial communication tools for agent-go.
//
// The pack uses an interface-based approach, allowing any serial port
// implementation to be plugged in.
package serial

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// SerialPort provides serial communication capabilities.
type SerialPort interface {
	ListPorts(ctx context.Context) ([]PortInfo, error)
	Open(ctx context.Context, port string, cfg PortConfig) (string, error) // returns handle ID
	Close(ctx context.Context, handle string) error
	Read(ctx context.Context, handle string, opts ReadOptions) (*ReadResult, error)
	Write(ctx context.Context, handle string, data []byte) (*WriteResult, error)
	Configure(ctx context.Context, handle string, cfg PortConfig) error
}

// PortInfo describes an available serial port.
type PortInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	VendorID    string `json:"vendor_id,omitempty"`
	ProductID   string `json:"product_id,omitempty"`
	SerialNum   string `json:"serial_number,omitempty"`
	IsUSB       bool   `json:"is_usb,omitempty"`
}

// PortConfig holds serial port configuration.
type PortConfig struct {
	BaudRate int    `json:"baud_rate,omitempty"`
	DataBits int    `json:"data_bits,omitempty"`
	StopBits int    `json:"stop_bits,omitempty"`
	Parity   string `json:"parity,omitempty"` // "none", "odd", "even"
	Timeout  int    `json:"timeout_ms,omitempty"`
}

// ReadOptions configures serial reads.
type ReadOptions struct {
	Size      int    `json:"size,omitempty"`
	Delimiter string `json:"delimiter,omitempty"` // read until delimiter
	Timeout   int    `json:"timeout_ms,omitempty"`
	Encoding  string `json:"encoding,omitempty"` // "utf8", "hex", "base64"
}

// ReadResult contains data read from the serial port.
type ReadResult struct {
	Data     string `json:"data"`
	BytesRead int   `json:"bytes_read"`
	Encoding string `json:"encoding"`
}

// WriteResult contains serial write status.
type WriteResult struct {
	BytesWritten int `json:"bytes_written"`
}

// Config holds serial pack configuration.
type Config struct {
	Port SerialPort
}

// Pack returns the serial communication tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &serialPack{cfg: cfg}

	return pack.NewBuilder("serial").
		WithDescription("Serial communication tools: list_ports, open, close, read, write, configure").
		WithVersion("1.0.0").
		AddTools(
			p.listPortsTool(), p.openTool(), p.closeTool(),
			p.readTool(), p.writeTool(), p.configureTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type serialPack struct{ cfg Config }

func (p *serialPack) listPortsTool() tool.Tool {
	return tool.NewBuilder("serial_list_ports").
		WithDescription("List available serial ports").
		ReadOnly().Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			ports, err := p.cfg.Port.ListPorts(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("list ports failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"count": len(ports), "ports": ports})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *serialPack) openTool() tool.Tool {
	return tool.NewBuilder("serial_open").
		WithDescription("Open a serial port connection").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Port     string `json:"port"`
				BaudRate int    `json:"baud_rate,omitempty"`
				DataBits int    `json:"data_bits,omitempty"`
				StopBits int    `json:"stop_bits,omitempty"`
				Parity   string `json:"parity,omitempty"`
				Timeout  int    `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Port == "" {
				return tool.Result{}, fmt.Errorf("port is required")
			}
			if in.BaudRate == 0 {
				in.BaudRate = 9600
			}
			if in.DataBits == 0 {
				in.DataBits = 8
			}
			if in.StopBits == 0 {
				in.StopBits = 1
			}
			if in.Parity == "" {
				in.Parity = "none"
			}
			handle, err := p.cfg.Port.Open(ctx, in.Port, PortConfig{
				BaudRate: in.BaudRate, DataBits: in.DataBits,
				StopBits: in.StopBits, Parity: in.Parity, Timeout: in.Timeout,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("open failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"handle": handle, "port": in.Port})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *serialPack) closeTool() tool.Tool {
	return tool.NewBuilder("serial_close").
		WithDescription("Close a serial port connection").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Handle string `json:"handle"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Handle == "" {
				return tool.Result{}, fmt.Errorf("handle is required")
			}
			err := p.cfg.Port.Close(ctx, in.Handle)
			if err != nil {
				return tool.Result{}, fmt.Errorf("close failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"handle": in.Handle, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *serialPack) readTool() tool.Tool {
	return tool.NewBuilder("serial_read").
		WithDescription("Read data from a serial port").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Handle    string `json:"handle"`
				Size      int    `json:"size,omitempty"`
				Delimiter string `json:"delimiter,omitempty"`
				Timeout   int    `json:"timeout_ms,omitempty"`
				Encoding  string `json:"encoding,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Handle == "" {
				return tool.Result{}, fmt.Errorf("handle is required")
			}
			if in.Encoding == "" {
				in.Encoding = "utf8"
			}
			result, err := p.cfg.Port.Read(ctx, in.Handle, ReadOptions{
				Size: in.Size, Delimiter: in.Delimiter,
				Timeout: in.Timeout, Encoding: in.Encoding,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("read failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *serialPack) writeTool() tool.Tool {
	return tool.NewBuilder("serial_write").
		WithDescription("Write data to a serial port").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Handle   string `json:"handle"`
				Data     string `json:"data"`
				Encoding string `json:"encoding,omitempty"` // "utf8", "hex", "base64"
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Handle == "" {
				return tool.Result{}, fmt.Errorf("handle is required")
			}
			if in.Data == "" {
				return tool.Result{}, fmt.Errorf("data is required")
			}
			result, err := p.cfg.Port.Write(ctx, in.Handle, []byte(in.Data))
			if err != nil {
				return tool.Result{}, fmt.Errorf("write failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *serialPack) configureTool() tool.Tool {
	return tool.NewBuilder("serial_configure").
		WithDescription("Configure serial port settings").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Handle   string `json:"handle"`
				BaudRate int    `json:"baud_rate,omitempty"`
				DataBits int    `json:"data_bits,omitempty"`
				StopBits int    `json:"stop_bits,omitempty"`
				Parity   string `json:"parity,omitempty"`
				Timeout  int    `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Handle == "" {
				return tool.Result{}, fmt.Errorf("handle is required")
			}
			err := p.cfg.Port.Configure(ctx, in.Handle, PortConfig{
				BaudRate: in.BaudRate, DataBits: in.DataBits,
				StopBits: in.StopBits, Parity: in.Parity, Timeout: in.Timeout,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("configure failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"handle": in.Handle, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
