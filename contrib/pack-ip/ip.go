// Package ip provides IP address utilities for agents.
package ip

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the IP address tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("ip").
		WithDescription("IP address utilities").
		AddTools(
			parseTool(),
			validateTool(),
			isPrivateTool(),
			isLoopbackTool(),
			versionTool(),
			toIntTool(),
			fromIntTool(),
			cidrParseTool(),
			cidrContainsTool(),
			cidrRangeTool(),
			compareTool(),
			reverseDNSTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("ip_parse").
		WithDescription("Parse an IP address").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP string `json:"ip"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip := net.ParseIP(params.IP)
			if ip == nil {
				result := map[string]any{
					"valid": false,
					"error": "invalid IP address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var version int
			if ip.To4() != nil {
				version = 4
			} else {
				version = 6
			}

			result := map[string]any{
				"valid":          true,
				"ip":             ip.String(),
				"version":        version,
				"is_loopback":    ip.IsLoopback(),
				"is_private":     ip.IsPrivate(),
				"is_global":      ip.IsGlobalUnicast(),
				"is_multicast":   ip.IsMulticast(),
				"is_unspecified": ip.IsUnspecified(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("ip_validate").
		WithDescription("Validate an IP address").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP          string `json:"ip"`
				RequireIPv4 bool   `json:"require_ipv4,omitempty"`
				RequireIPv6 bool   `json:"require_ipv6,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip := net.ParseIP(params.IP)
			if ip == nil {
				result := map[string]any{
					"valid":  false,
					"reason": "invalid IP address format",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			isIPv4 := ip.To4() != nil

			if params.RequireIPv4 && !isIPv4 {
				result := map[string]any{
					"valid":  false,
					"reason": "not an IPv4 address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			if params.RequireIPv6 && isIPv4 {
				result := map[string]any{
					"valid":  false,
					"reason": "not an IPv6 address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"valid":   true,
				"version": 4,
			}
			if !isIPv4 {
				result["version"] = 6
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isPrivateTool() tool.Tool {
	return tool.NewBuilder("ip_is_private").
		WithDescription("Check if IP is private").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP string `json:"ip"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip := net.ParseIP(params.IP)
			if ip == nil {
				result := map[string]any{
					"valid": false,
					"error": "invalid IP address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"ip":         params.IP,
				"is_private": ip.IsPrivate(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isLoopbackTool() tool.Tool {
	return tool.NewBuilder("ip_is_loopback").
		WithDescription("Check if IP is loopback").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP string `json:"ip"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip := net.ParseIP(params.IP)
			if ip == nil {
				result := map[string]any{
					"valid": false,
					"error": "invalid IP address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"ip":          params.IP,
				"is_loopback": ip.IsLoopback(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func versionTool() tool.Tool {
	return tool.NewBuilder("ip_version").
		WithDescription("Get IP version (4 or 6)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP string `json:"ip"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip := net.ParseIP(params.IP)
			if ip == nil {
				result := map[string]any{
					"valid": false,
					"error": "invalid IP address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			version := 6
			if ip.To4() != nil {
				version = 4
			}

			result := map[string]any{
				"ip":      params.IP,
				"version": version,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func toIntTool() tool.Tool {
	return tool.NewBuilder("ip_to_int").
		WithDescription("Convert IPv4 to integer").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP string `json:"ip"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip := net.ParseIP(params.IP)
			if ip == nil {
				result := map[string]any{
					"valid": false,
					"error": "invalid IP address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			ip4 := ip.To4()
			if ip4 == nil {
				result := map[string]any{
					"valid": false,
					"error": "only IPv4 addresses can be converted to integer",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			intVal := binary.BigEndian.Uint32(ip4)

			result := map[string]any{
				"ip":      params.IP,
				"integer": intVal,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fromIntTool() tool.Tool {
	return tool.NewBuilder("ip_from_int").
		WithDescription("Convert integer to IPv4").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Integer uint32 `json:"integer"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip := make(net.IP, 4)
			binary.BigEndian.PutUint32(ip, params.Integer)

			result := map[string]any{
				"integer": params.Integer,
				"ip":      ip.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func cidrParseTool() tool.Tool {
	return tool.NewBuilder("ip_cidr_parse").
		WithDescription("Parse CIDR notation").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				CIDR string `json:"cidr"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip, ipNet, err := net.ParseCIDR(params.CIDR)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			ones, bits := ipNet.Mask.Size()

			result := map[string]any{
				"valid":         true,
				"ip":            ip.String(),
				"network":       ipNet.IP.String(),
				"mask":          net.IP(ipNet.Mask).String(),
				"prefix_length": ones,
				"total_bits":    bits,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func cidrContainsTool() tool.Tool {
	return tool.NewBuilder("ip_cidr_contains").
		WithDescription("Check if CIDR contains IP").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				CIDR string `json:"cidr"`
				IP   string `json:"ip"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			_, ipNet, err := net.ParseCIDR(params.CIDR)
			if err != nil {
				result := map[string]any{
					"error": "invalid CIDR: " + err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			ip := net.ParseIP(params.IP)
			if ip == nil {
				result := map[string]any{
					"error": "invalid IP address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			contains := ipNet.Contains(ip)

			result := map[string]any{
				"cidr":     params.CIDR,
				"ip":       params.IP,
				"contains": contains,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func cidrRangeTool() tool.Tool {
	return tool.NewBuilder("ip_cidr_range").
		WithDescription("Get IP range for CIDR").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				CIDR string `json:"cidr"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			_, ipNet, err := net.ParseCIDR(params.CIDR)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			ones, bits := ipNet.Mask.Size()
			hostBits := bits - ones
			numHosts := uint64(1) << hostBits

			// Calculate first and last IP
			firstIP := ipNet.IP
			lastIP := make(net.IP, len(firstIP))
			for i := range firstIP {
				lastIP[i] = firstIP[i] | ^ipNet.Mask[i]
			}

			result := map[string]any{
				"cidr":      params.CIDR,
				"first_ip":  firstIP.String(),
				"last_ip":   lastIP.String(),
				"num_hosts": numHosts,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compareTool() tool.Tool {
	return tool.NewBuilder("ip_compare").
		WithDescription("Compare two IP addresses").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP1 string `json:"ip1"`
				IP2 string `json:"ip2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip1 := net.ParseIP(params.IP1)
			ip2 := net.ParseIP(params.IP2)

			if ip1 == nil {
				result := map[string]any{
					"error": "invalid first IP address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			if ip2 == nil {
				result := map[string]any{
					"error": "invalid second IP address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			cmp := strings.Compare(ip1.String(), ip2.String())
			equal := ip1.Equal(ip2)

			result := map[string]any{
				"ip1":   params.IP1,
				"ip2":   params.IP2,
				"equal": equal,
				"cmp":   cmp,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func reverseDNSTool() tool.Tool {
	return tool.NewBuilder("ip_reverse_dns").
		WithDescription("Get reverse DNS notation").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP string `json:"ip"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip := net.ParseIP(params.IP)
			if ip == nil {
				result := map[string]any{
					"valid": false,
					"error": "invalid IP address",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var reverseDNS string
			if ip4 := ip.To4(); ip4 != nil {
				// IPv4 reverse DNS
				parts := make([]string, 4)
				for i := 0; i < 4; i++ {
					parts[3-i] = net.IP{ip4[i]}.String()
				}
				reverseDNS = strings.Join(parts, ".") + ".in-addr.arpa"
			} else {
				// IPv6 reverse DNS
				ip16 := ip.To16()
				var parts []string
				for i := len(ip16) - 1; i >= 0; i-- {
					parts = append(parts, string("0123456789abcdef"[ip16[i]&0x0f]))
					parts = append(parts, string("0123456789abcdef"[ip16[i]>>4]))
				}
				reverseDNS = strings.Join(parts, ".") + ".ip6.arpa"
			}

			result := map[string]any{
				"ip":          params.IP,
				"reverse_dns": reverseDNS,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
