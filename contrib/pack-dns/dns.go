// Package dns provides DNS lookup and resolution tools for agents.
package dns

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the DNS tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("dns").
		WithDescription("DNS lookup and resolution tools").
		AddTools(
			lookupHostTool(),
			lookupIPTool(),
			lookupCNAMETool(),
			lookupMXTool(),
			lookupNSTool(),
			lookupTXTTool(),
			lookupSRVTool(),
			reverseLookupTool(),
			resolveAllTool(),
			checkAvailabilityTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func lookupHostTool() tool.Tool {
	return tool.NewBuilder("dns_lookup_host").
		WithDescription("Lookup IP addresses for a hostname").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Host    string `json:"host"`
				Timeout int    `json:"timeout,omitempty"` // seconds
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			addrs, err := resolver.LookupHost(ctx, params.Host)
			if err != nil {
				return tool.Result{}, err
			}

			var ipv4, ipv6 []string
			for _, addr := range addrs {
				ip := net.ParseIP(addr)
				if ip.To4() != nil {
					ipv4 = append(ipv4, addr)
				} else {
					ipv6 = append(ipv6, addr)
				}
			}

			result := map[string]any{
				"host":      params.Host,
				"addresses": addrs,
				"ipv4":      ipv4,
				"ipv6":      ipv6,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lookupIPTool() tool.Tool {
	return tool.NewBuilder("dns_lookup_ip").
		WithDescription("Lookup hostnames for an IP address").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP      string `json:"ip"`
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			names, err := resolver.LookupAddr(ctx, params.IP)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"ip":    params.IP,
				"names": names,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lookupCNAMETool() tool.Tool {
	return tool.NewBuilder("dns_lookup_cname").
		WithDescription("Lookup CNAME record for a hostname").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Host    string `json:"host"`
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			cname, err := resolver.LookupCNAME(ctx, params.Host)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"host":  params.Host,
				"cname": strings.TrimSuffix(cname, "."),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lookupMXTool() tool.Tool {
	return tool.NewBuilder("dns_lookup_mx").
		WithDescription("Lookup MX records for a domain").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Domain  string `json:"domain"`
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			mxs, err := resolver.LookupMX(ctx, params.Domain)
			if err != nil {
				return tool.Result{}, err
			}

			var records []map[string]any
			for _, mx := range mxs {
				records = append(records, map[string]any{
					"host": strings.TrimSuffix(mx.Host, "."),
					"pref": mx.Pref,
				})
			}

			result := map[string]any{
				"domain":  params.Domain,
				"records": records,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lookupNSTool() tool.Tool {
	return tool.NewBuilder("dns_lookup_ns").
		WithDescription("Lookup NS records for a domain").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Domain  string `json:"domain"`
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			nss, err := resolver.LookupNS(ctx, params.Domain)
			if err != nil {
				return tool.Result{}, err
			}

			var nameservers []string
			for _, ns := range nss {
				nameservers = append(nameservers, strings.TrimSuffix(ns.Host, "."))
			}

			result := map[string]any{
				"domain":      params.Domain,
				"nameservers": nameservers,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lookupTXTTool() tool.Tool {
	return tool.NewBuilder("dns_lookup_txt").
		WithDescription("Lookup TXT records for a domain").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Domain  string `json:"domain"`
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			txts, err := resolver.LookupTXT(ctx, params.Domain)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"domain":  params.Domain,
				"records": txts,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lookupSRVTool() tool.Tool {
	return tool.NewBuilder("dns_lookup_srv").
		WithDescription("Lookup SRV records").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Service string `json:"service"` // e.g., "xmpp-server"
				Proto   string `json:"proto"`   // e.g., "tcp"
				Domain  string `json:"domain"`
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			_, srvs, err := resolver.LookupSRV(ctx, params.Service, params.Proto, params.Domain)
			if err != nil {
				return tool.Result{}, err
			}

			var records []map[string]any
			for _, srv := range srvs {
				records = append(records, map[string]any{
					"target":   strings.TrimSuffix(srv.Target, "."),
					"port":     srv.Port,
					"priority": srv.Priority,
					"weight":   srv.Weight,
				})
			}

			result := map[string]any{
				"service": params.Service,
				"proto":   params.Proto,
				"domain":  params.Domain,
				"records": records,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func reverseLookupTool() tool.Tool {
	return tool.NewBuilder("dns_reverse_lookup").
		WithDescription("Perform reverse DNS lookup").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP      string `json:"ip"`
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			names, err := resolver.LookupAddr(ctx, params.IP)
			if err != nil {
				return tool.Result{}, err
			}

			// Clean up trailing dots
			var cleaned []string
			for _, name := range names {
				cleaned = append(cleaned, strings.TrimSuffix(name, "."))
			}

			result := map[string]any{
				"ip":        params.IP,
				"hostnames": cleaned,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func resolveAllTool() tool.Tool {
	return tool.NewBuilder("dns_resolve_all").
		WithDescription("Resolve all DNS records for a domain").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Domain  string `json:"domain"`
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 10 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			result := map[string]any{
				"domain": params.Domain,
			}

			// A/AAAA records
			if addrs, err := resolver.LookupHost(ctx, params.Domain); err == nil {
				var ipv4, ipv6 []string
				for _, addr := range addrs {
					ip := net.ParseIP(addr)
					if ip.To4() != nil {
						ipv4 = append(ipv4, addr)
					} else {
						ipv6 = append(ipv6, addr)
					}
				}
				result["a"] = ipv4
				result["aaaa"] = ipv6
			}

			// CNAME
			if cname, err := resolver.LookupCNAME(ctx, params.Domain); err == nil {
				result["cname"] = strings.TrimSuffix(cname, ".")
			}

			// MX
			if mxs, err := resolver.LookupMX(ctx, params.Domain); err == nil {
				var mxRecords []map[string]any
				for _, mx := range mxs {
					mxRecords = append(mxRecords, map[string]any{
						"host": strings.TrimSuffix(mx.Host, "."),
						"pref": mx.Pref,
					})
				}
				result["mx"] = mxRecords
			}

			// NS
			if nss, err := resolver.LookupNS(ctx, params.Domain); err == nil {
				var nameservers []string
				for _, ns := range nss {
					nameservers = append(nameservers, strings.TrimSuffix(ns.Host, "."))
				}
				result["ns"] = nameservers
			}

			// TXT
			if txts, err := resolver.LookupTXT(ctx, params.Domain); err == nil {
				result["txt"] = txts
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func checkAvailabilityTool() tool.Tool {
	return tool.NewBuilder("dns_check_availability").
		WithDescription("Check if a host is resolvable").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Host    string `json:"host"`
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			timeout := time.Duration(params.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			start := time.Now()
			addrs, err := resolver.LookupHost(ctx, params.Host)
			duration := time.Since(start)

			available := err == nil && len(addrs) > 0
			errorMsg := ""
			if err != nil {
				errorMsg = err.Error()
			}

			result := map[string]any{
				"host":        params.Host,
				"available":   available,
				"response_ms": duration.Milliseconds(),
				"num_ips":     len(addrs),
				"error":       errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
