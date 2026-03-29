// Package grpc provides gRPC tools for agents.
package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ConnectionPool manages gRPC connections.
type ConnectionPool struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn
}

var pool = &ConnectionPool{
	conns: make(map[string]*grpc.ClientConn),
}

// Pack returns the gRPC tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("grpc").
		WithDescription("gRPC client tools").
		AddTools(
			connectTool(),
			disconnectTool(),
			callTool(),
			listTool(),
			closeAllTool(),
			parseProtoTool(),
			buildRequestTool(),
			parseResponseTool(),
			healthCheckTool(),
			metadataTool(),
			statusCodeTool(),
			describeTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func connectTool() tool.Tool {
	return tool.NewBuilder("grpc_connect").
		WithDescription("Connect to a gRPC server").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Address string `json:"address"`
				ID      string `json:"id,omitempty"`
				TLS     bool   `json:"tls,omitempty"`
				Timeout int    `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			id := params.ID
			if id == "" {
				id = params.Address
			}

			timeout := time.Duration(params.Timeout) * time.Millisecond
			if timeout <= 0 {
				timeout = 30 * time.Second
			}

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			opts := []grpc.DialOption{
				grpc.WithBlock(),
			}

			if !params.TLS {
				opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
			}

			conn, err := grpc.DialContext(ctx, params.Address, opts...)
			if err != nil {
				return tool.Result{}, err
			}

			pool.mu.Lock()
			if existing, ok := pool.conns[id]; ok {
				_ = existing.Close() // #nosec G104 -- best-effort close
			}
			pool.conns[id] = conn
			pool.mu.Unlock()

			result := map[string]any{
				"id":        id,
				"address":   params.Address,
				"connected": true,
				"tls":       params.TLS,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func disconnectTool() tool.Tool {
	return tool.NewBuilder("grpc_disconnect").
		WithDescription("Disconnect from a gRPC server").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pool.mu.Lock()
			conn, ok := pool.conns[params.ID]
			if ok {
				_ = conn.Close() // #nosec G104 -- best-effort close
				delete(pool.conns, params.ID)
			}
			pool.mu.Unlock()

			result := map[string]any{
				"id":           params.ID,
				"disconnected": ok,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func callTool() tool.Tool {
	return tool.NewBuilder("grpc_call").
		WithDescription("Call a gRPC method").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID       string            `json:"id"`
				Method   string            `json:"method"` // /package.Service/Method
				Request  map[string]any    `json:"request,omitempty"`
				Metadata map[string]string `json:"metadata,omitempty"`
				Timeout  int               `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pool.mu.RLock()
			conn, ok := pool.conns[params.ID]
			pool.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "connection not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			timeout := time.Duration(params.Timeout) * time.Millisecond
			if timeout <= 0 {
				timeout = 30 * time.Second
			}

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Add metadata
			if len(params.Metadata) > 0 {
				md := metadata.New(params.Metadata)
				ctx = metadata.NewOutgoingContext(ctx, md)
			}

			// Build request JSON
			reqJSON, _ := json.Marshal(params.Request)

			// Generic unary call
			var header, trailer metadata.MD
			respBytes := make([]byte, 0)

			err := conn.Invoke(ctx, params.Method, reqJSON, &respBytes,
				grpc.Header(&header),
				grpc.Trailer(&trailer),
			)

			if err != nil {
				st, _ := status.FromError(err)
				result := map[string]any{
					"error":   err.Error(),
					"code":    st.Code().String(),
					"message": st.Message(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var response map[string]any
			_ = json.Unmarshal(respBytes, &response) // #nosec G104 -- best-effort parse, empty response is acceptable

			result := map[string]any{
				"id":       params.ID,
				"method":   params.Method,
				"response": response,
				"success":  true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("grpc_list").
		WithDescription("List active gRPC connections").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pool.mu.RLock()
			var connections []map[string]any
			for id, conn := range pool.conns {
				connections = append(connections, map[string]any{
					"id":    id,
					"state": conn.GetState().String(),
				})
			}
			pool.mu.RUnlock()

			result := map[string]any{
				"connections": connections,
				"count":       len(connections),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func closeAllTool() tool.Tool {
	return tool.NewBuilder("grpc_close_all").
		WithDescription("Close all gRPC connections").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pool.mu.Lock()
			count := len(pool.conns)
			for id, conn := range pool.conns {
				_ = conn.Close() // #nosec G104 -- best-effort close
				delete(pool.conns, id)
			}
			pool.mu.Unlock()

			result := map[string]any{
				"closed": count,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseProtoTool() tool.Tool {
	return tool.NewBuilder("grpc_parse_proto").
		WithDescription("Parse proto file content").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Extract package
			pkgPattern := regexp.MustCompile(`package\s+(\w+(?:\.\w+)*)\s*;`)
			pkgMatch := pkgPattern.FindStringSubmatch(params.Content)
			pkg := ""
			if len(pkgMatch) > 1 {
				pkg = pkgMatch[1]
			}

			// Extract services
			servicePattern := regexp.MustCompile(`service\s+(\w+)\s*\{([^}]*)\}`)
			serviceMatches := servicePattern.FindAllStringSubmatch(params.Content, -1)

			var services []map[string]any
			for _, match := range serviceMatches {
				serviceName := match[1]
				serviceBody := match[2]

				// Extract methods
				methodPattern := regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*(\w+)\s*\)\s*returns\s*\(\s*(\w+)\s*\)`)
				methodMatches := methodPattern.FindAllStringSubmatch(serviceBody, -1)

				var methods []map[string]string
				for _, m := range methodMatches {
					methods = append(methods, map[string]string{
						"name":     m[1],
						"input":    m[2],
						"output":   m[3],
						"fullName": fmt.Sprintf("/%s.%s/%s", pkg, serviceName, m[1]),
					})
				}

				services = append(services, map[string]any{
					"name":    serviceName,
					"methods": methods,
				})
			}

			// Extract messages
			messagePattern := regexp.MustCompile(`message\s+(\w+)\s*\{([^}]*)\}`)
			messageMatches := messagePattern.FindAllStringSubmatch(params.Content, -1)

			var messages []map[string]any
			for _, match := range messageMatches {
				msgName := match[1]
				msgBody := match[2]

				// Extract fields
				fieldPattern := regexp.MustCompile(`(\w+)\s+(\w+)\s*=\s*(\d+)\s*;`)
				fieldMatches := fieldPattern.FindAllStringSubmatch(msgBody, -1)

				var fields []map[string]any
				for _, f := range fieldMatches {
					fields = append(fields, map[string]any{
						"type":   f[1],
						"name":   f[2],
						"number": f[3],
					})
				}

				messages = append(messages, map[string]any{
					"name":   msgName,
					"fields": fields,
				})
			}

			result := map[string]any{
				"package":  pkg,
				"services": services,
				"messages": messages,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func buildRequestTool() tool.Tool {
	return tool.NewBuilder("grpc_build_request").
		WithDescription("Build a gRPC request from JSON").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				MessageType string         `json:"message_type"`
				Data        map[string]any `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Build JSON representation
			dataJSON, err := json.Marshal(params.Data)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"message_type": params.MessageType,
				"json":         string(dataJSON),
				"fields":       len(params.Data),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseResponseTool() tool.Tool {
	return tool.NewBuilder("grpc_parse_response").
		WithDescription("Parse gRPC response").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data   json.RawMessage `json:"data"`
				Format string          `json:"format,omitempty"` // json, pretty
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var parsed map[string]any
			if err := json.Unmarshal(params.Data, &parsed); err != nil {
				return tool.Result{}, err
			}

			var formatted string
			if params.Format == "pretty" {
				prettyJSON, _ := json.MarshalIndent(parsed, "", "  ")
				formatted = string(prettyJSON)
			} else {
				formattedJSON, _ := json.Marshal(parsed)
				formatted = string(formattedJSON)
			}

			result := map[string]any{
				"parsed":    parsed,
				"formatted": formatted,
				"fields":    len(parsed),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func healthCheckTool() tool.Tool {
	return tool.NewBuilder("grpc_health_check").
		WithDescription("Check gRPC server health").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID      string `json:"id"`
				Service string `json:"service,omitempty"`
				Timeout int    `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pool.mu.RLock()
			conn, ok := pool.conns[params.ID]
			pool.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "connection not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			timeout := time.Duration(params.Timeout) * time.Millisecond
			if timeout <= 0 {
				timeout = 5 * time.Second
			}

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Standard health check method
			method := "/grpc.health.v1.Health/Check"
			request := map[string]string{"service": params.Service}
			reqJSON, _ := json.Marshal(request)

			var respBytes []byte
			err := conn.Invoke(ctx, method, reqJSON, &respBytes)

			if err != nil {
				st, _ := status.FromError(err)
				result := map[string]any{
					"id":      params.ID,
					"healthy": false,
					"error":   st.Message(),
					"code":    st.Code().String(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"id":      params.ID,
				"healthy": true,
				"service": params.Service,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func metadataTool() tool.Tool {
	return tool.NewBuilder("grpc_metadata").
		WithDescription("Build gRPC metadata").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Headers map[string]string   `json:"headers,omitempty"`
				Pairs   []map[string]string `json:"pairs,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			md := make(map[string][]string)

			for k, v := range params.Headers {
				md[strings.ToLower(k)] = []string{v}
			}

			for _, pair := range params.Pairs {
				for k, v := range pair {
					key := strings.ToLower(k)
					md[key] = append(md[key], v)
				}
			}

			result := map[string]any{
				"metadata": md,
				"count":    len(md),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func statusCodeTool() tool.Tool {
	return tool.NewBuilder("grpc_status_codes").
		WithDescription("List gRPC status codes").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			codes := []map[string]any{
				{"code": 0, "name": "OK", "description": "Success"},
				{"code": 1, "name": "CANCELLED", "description": "Operation cancelled"},
				{"code": 2, "name": "UNKNOWN", "description": "Unknown error"},
				{"code": 3, "name": "INVALID_ARGUMENT", "description": "Invalid argument"},
				{"code": 4, "name": "DEADLINE_EXCEEDED", "description": "Deadline exceeded"},
				{"code": 5, "name": "NOT_FOUND", "description": "Not found"},
				{"code": 6, "name": "ALREADY_EXISTS", "description": "Already exists"},
				{"code": 7, "name": "PERMISSION_DENIED", "description": "Permission denied"},
				{"code": 8, "name": "RESOURCE_EXHAUSTED", "description": "Resource exhausted"},
				{"code": 9, "name": "FAILED_PRECONDITION", "description": "Failed precondition"},
				{"code": 10, "name": "ABORTED", "description": "Operation aborted"},
				{"code": 11, "name": "OUT_OF_RANGE", "description": "Out of range"},
				{"code": 12, "name": "UNIMPLEMENTED", "description": "Not implemented"},
				{"code": 13, "name": "INTERNAL", "description": "Internal error"},
				{"code": 14, "name": "UNAVAILABLE", "description": "Service unavailable"},
				{"code": 15, "name": "DATA_LOSS", "description": "Data loss"},
				{"code": 16, "name": "UNAUTHENTICATED", "description": "Unauthenticated"},
			}

			result := map[string]any{
				"codes": codes,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func describeTool() tool.Tool {
	return tool.NewBuilder("grpc_describe").
		WithDescription("Describe a protobuf message type").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				TypeName string `json:"type_name"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Parse type name for basic info
			parts := strings.Split(params.TypeName, ".")
			simpleName := parts[len(parts)-1]
			pkgName := ""
			if len(parts) > 1 {
				pkgName = strings.Join(parts[:len(parts)-1], ".")
			}

			result := map[string]any{
				"type_name":   params.TypeName,
				"simple_name": simpleName,
				"package":     pkgName,
				"note":        "Use grpc_parse_proto with proto file content for detailed field information",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
