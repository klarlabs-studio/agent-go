// Package network provides network diagnostic tools for agent-go.
//
// The pack uses an interface-based approach, allowing any network utility
// implementation to be plugged in.
package network

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// NetworkUtil provides network diagnostic operations.
type NetworkUtil interface {
	Ping(ctx context.Context, host string, count int) (*PingResult, error)
	Traceroute(ctx context.Context, host string, maxHops int) (*TracerouteResult, error)
	DNSLookup(ctx context.Context, host, recordType string) (*DNSResult, error)
	PortScan(ctx context.Context, host string, ports []int) ([]PortResult, error)
	Whois(ctx context.Context, domain string) (string, error)
	HTTPCheck(ctx context.Context, url string, opts HTTPCheckOptions) (*HTTPCheckResult, error)
}

// PingResult contains ping results.
type PingResult struct {
	Host       string  `json:"host"`
	Packets    int     `json:"packets_sent"`
	Received   int     `json:"packets_received"`
	Loss       float64 `json:"packet_loss_pct"`
	MinMS      float64 `json:"min_ms"`
	AvgMS      float64 `json:"avg_ms"`
	MaxMS      float64 `json:"max_ms"`
}

// TracerouteResult contains traceroute results.
type TracerouteResult struct {
	Host string `json:"host"`
	Hops []Hop  `json:"hops"`
}

// Hop represents a single traceroute hop.
type Hop struct {
	Number  int     `json:"number"`
	Address string  `json:"address"`
	Host    string  `json:"host,omitempty"`
	RTT     float64 `json:"rtt_ms"`
}

// DNSResult contains DNS lookup results.
type DNSResult struct {
	Host    string      `json:"host"`
	Type    string      `json:"type"`
	Records []DNSRecord `json:"records"`
}

// DNSRecord represents a DNS record.
type DNSRecord struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   int    `json:"ttl,omitempty"`
}

// PortResult represents a port scan result.
type PortResult struct {
	Port    int    `json:"port"`
	State   string `json:"state"` // "open", "closed", "filtered"
	Service string `json:"service,omitempty"`
}

// HTTPCheckOptions configures HTTP checks.
type HTTPCheckOptions struct {
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout int               `json:"timeout_seconds,omitempty"`
}

// HTTPCheckResult contains HTTP check results.
type HTTPCheckResult struct {
	URL        string            `json:"url"`
	Status     int               `json:"status"`
	LatencyMS  int64             `json:"latency_ms"`
	Headers    map[string]string `json:"headers,omitempty"`
	TLSVersion string           `json:"tls_version,omitempty"`
	TLSExpiry  string           `json:"tls_expiry,omitempty"`
}

// Config holds network pack configuration.
type Config struct {
	Util NetworkUtil
}

// Pack returns the network diagnostic tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &networkPack{cfg: cfg}

	return pack.NewBuilder("network").
		WithDescription("Network diagnostic tools: ping, traceroute, DNS lookup, port scan, whois, HTTP check").
		WithVersion("1.0.0").
		AddTools(
			p.pingTool(), p.tracerouteTool(), p.dnsLookupTool(),
			p.portScanTool(), p.whoisTool(), p.httpCheckTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type networkPack struct{ cfg Config }

func (p *networkPack) pingTool() tool.Tool {
	return tool.NewBuilder("network_ping").
		WithDescription("Ping a host to check connectivity and latency").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Host  string `json:"host"`
				Count int    `json:"count,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Host == "" {
				return tool.Result{}, fmt.Errorf("host is required")
			}
			if in.Count == 0 {
				in.Count = 4
			}
			result, err := p.cfg.Util.Ping(ctx, in.Host, in.Count)
			if err != nil {
				return tool.Result{}, fmt.Errorf("ping failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *networkPack) tracerouteTool() tool.Tool {
	return tool.NewBuilder("network_traceroute").
		WithDescription("Trace the route to a host").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Host    string `json:"host"`
				MaxHops int    `json:"max_hops,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Host == "" {
				return tool.Result{}, fmt.Errorf("host is required")
			}
			if in.MaxHops == 0 {
				in.MaxHops = 30
			}
			result, err := p.cfg.Util.Traceroute(ctx, in.Host, in.MaxHops)
			if err != nil {
				return tool.Result{}, fmt.Errorf("traceroute failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *networkPack) dnsLookupTool() tool.Tool {
	return tool.NewBuilder("network_dns_lookup").
		WithDescription("Perform a DNS lookup for a host").
		ReadOnly().Idempotent().Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Host       string `json:"host"`
				RecordType string `json:"record_type,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Host == "" {
				return tool.Result{}, fmt.Errorf("host is required")
			}
			if in.RecordType == "" {
				in.RecordType = "A"
			}
			result, err := p.cfg.Util.DNSLookup(ctx, in.Host, in.RecordType)
			if err != nil {
				return tool.Result{}, fmt.Errorf("dns lookup failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *networkPack) portScanTool() tool.Tool {
	return tool.NewBuilder("network_port_scan").
		WithDescription("Scan ports on a host").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Host  string `json:"host"`
				Ports []int  `json:"ports,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Host == "" {
				return tool.Result{}, fmt.Errorf("host is required")
			}
			if len(in.Ports) == 0 {
				in.Ports = []int{22, 80, 443, 3306, 5432, 6379, 8080, 8443}
			}
			results, err := p.cfg.Util.PortScan(ctx, in.Host, in.Ports)
			if err != nil {
				return tool.Result{}, fmt.Errorf("port scan failed: %w", err)
			}
			open := 0
			for _, r := range results {
				if r.State == "open" {
					open++
				}
			}
			output, _ := json.Marshal(map[string]any{
				"host": in.Host, "scanned": len(results), "open": open, "results": results,
			})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *networkPack) whoisTool() tool.Tool {
	return tool.NewBuilder("network_whois").
		WithDescription("Look up domain registration information").
		ReadOnly().Idempotent().Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Domain string `json:"domain"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Domain == "" {
				return tool.Result{}, fmt.Errorf("domain is required")
			}
			info, err := p.cfg.Util.Whois(ctx, in.Domain)
			if err != nil {
				return tool.Result{}, fmt.Errorf("whois failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"domain": in.Domain, "info": info})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *networkPack) httpCheckTool() tool.Tool {
	return tool.NewBuilder("network_http_check").
		WithDescription("Check HTTP endpoint availability and TLS status").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				URL     string            `json:"url"`
				Method  string            `json:"method,omitempty"`
				Headers map[string]string `json:"headers,omitempty"`
				Timeout int               `json:"timeout_seconds,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.URL == "" {
				return tool.Result{}, fmt.Errorf("url is required")
			}
			result, err := p.cfg.Util.HTTPCheck(ctx, in.URL, HTTPCheckOptions{
				Method: in.Method, Headers: in.Headers, Timeout: in.Timeout,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("http check failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
