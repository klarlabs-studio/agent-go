// Package sse provides Server-Sent Events client tools for agents.
package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// ConnectionPool manages SSE connections.
type ConnectionPool struct {
	mu    sync.RWMutex
	conns map[string]*sseConnection
}

type sseConnection struct {
	url     string
	cancel  context.CancelFunc
	events  []sseEvent
	mu      sync.Mutex
	running bool
	lastID  string
}

type sseEvent struct {
	ID    string    `json:"id,omitempty"`
	Event string    `json:"event,omitempty"`
	Data  string    `json:"data"`
	Time  time.Time `json:"time"`
}

var pool = &ConnectionPool{
	conns: make(map[string]*sseConnection),
}

// Pack returns the SSE tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("sse").
		WithDescription("Server-Sent Events client tools").
		AddTools(
			connectTool(),
			disconnectTool(),
			readTool(),
			readAllTool(),
			waitForTool(),
			clearTool(),
			statusTool(),
			listTool(),
			closeAllTool(),
			parseTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func connectTool() tool.Tool {
	return tool.NewBuilder("sse_connect").
		WithDescription("Connect to an SSE endpoint").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL         string            `json:"url"`
				ID          string            `json:"id,omitempty"`
				Headers     map[string]string `json:"headers,omitempty"`
				LastEventID string            `json:"last_event_id,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			id := params.ID
			if id == "" {
				id = params.URL
			}

			// Create context with cancel
			connCtx, cancel := context.WithCancel(context.Background())

			conn := &sseConnection{
				url:     params.URL,
				cancel:  cancel,
				events:  []sseEvent{},
				running: true,
				lastID:  params.LastEventID,
			}

			// Store connection
			pool.mu.Lock()
			if existing, ok := pool.conns[id]; ok {
				existing.cancel()
			}
			pool.conns[id] = conn
			pool.mu.Unlock()

			// Start reading in background
			go func() {
				client := &http.Client{Timeout: 0} // No timeout for SSE

				req, err := http.NewRequestWithContext(connCtx, "GET", params.URL, nil)
				if err != nil {
					return
				}

				req.Header.Set("Accept", "text/event-stream")
				req.Header.Set("Cache-Control", "no-cache")
				req.Header.Set("Connection", "keep-alive")

				if params.LastEventID != "" {
					req.Header.Set("Last-Event-ID", params.LastEventID)
				}

				for k, v := range params.Headers {
					req.Header.Set(k, v)
				}

				resp, err := client.Do(req)
				if err != nil {
					conn.mu.Lock()
					conn.running = false
					conn.mu.Unlock()
					return
				}
				defer resp.Body.Close()

				reader := bufio.NewReader(resp.Body)
				var currentEvent sseEvent

				for {
					select {
					case <-connCtx.Done():
						conn.mu.Lock()
						conn.running = false
						conn.mu.Unlock()
						return
					default:
					}

					line, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							continue
						}
						conn.mu.Lock()
						conn.running = false
						conn.mu.Unlock()
						return
					}

					line = strings.TrimSpace(line)

					if line == "" {
						// End of event
						if currentEvent.Data != "" {
							currentEvent.Time = time.Now()
							conn.mu.Lock()
							conn.events = append(conn.events, currentEvent)
							if currentEvent.ID != "" {
								conn.lastID = currentEvent.ID
							}
							conn.mu.Unlock()
						}
						currentEvent = sseEvent{}
						continue
					}

					if strings.HasPrefix(line, ":") {
						// Comment, ignore
						continue
					}

					switch {
					case strings.HasPrefix(line, "data:"):
						data := strings.TrimPrefix(line, "data:")
						data = strings.TrimSpace(data)
						if currentEvent.Data != "" {
							currentEvent.Data += "\n" + data
						} else {
							currentEvent.Data = data
						}
					case strings.HasPrefix(line, "event:"):
						currentEvent.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
					case strings.HasPrefix(line, "id:"):
						currentEvent.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
					}
				}
			}()

			result := map[string]any{
				"id":        id,
				"url":       params.URL,
				"connected": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func disconnectTool() tool.Tool {
	return tool.NewBuilder("sse_disconnect").
		WithDescription("Disconnect from an SSE endpoint").
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
				conn.cancel()
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

func readTool() tool.Tool {
	return tool.NewBuilder("sse_read").
		WithDescription("Read next event from SSE connection").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID      string `json:"id"`
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

			deadline := time.Now().Add(timeout)

			for time.Now().Before(deadline) {
				conn.mu.Lock()
				if len(conn.events) > 0 {
					event := conn.events[0]
					conn.events = conn.events[1:]
					conn.mu.Unlock()

					result := map[string]any{
						"id":    params.ID,
						"event": event,
						"found": true,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
				conn.mu.Unlock()

				time.Sleep(50 * time.Millisecond)
			}

			result := map[string]any{
				"id":      params.ID,
				"found":   false,
				"timeout": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func readAllTool() tool.Tool {
	return tool.NewBuilder("sse_read_all").
		WithDescription("Read all buffered events from SSE connection").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID    string `json:"id"`
				Limit int    `json:"limit,omitempty"`
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

			conn.mu.Lock()
			events := make([]sseEvent, len(conn.events))
			copy(events, conn.events)

			limit := params.Limit
			if limit > 0 && limit < len(events) {
				events = events[:limit]
				conn.events = conn.events[limit:]
			} else {
				conn.events = []sseEvent{}
			}
			conn.mu.Unlock()

			result := map[string]any{
				"id":     params.ID,
				"events": events,
				"count":  len(events),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func waitForTool() tool.Tool {
	return tool.NewBuilder("sse_wait_for").
		WithDescription("Wait for an event matching criteria").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID        string `json:"id"`
				EventType string `json:"event_type,omitempty"`
				Contains  string `json:"data_contains,omitempty"`
				Timeout   int    `json:"timeout_ms,omitempty"`
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

			deadline := time.Now().Add(timeout)

			for time.Now().Before(deadline) {
				conn.mu.Lock()
				for i, event := range conn.events {
					match := true
					if params.EventType != "" && event.Event != params.EventType {
						match = false
					}
					if params.Contains != "" && !strings.Contains(event.Data, params.Contains) {
						match = false
					}

					if match {
						// Remove this event
						conn.events = append(conn.events[:i], conn.events[i+1:]...)
						conn.mu.Unlock()

						result := map[string]any{
							"id":    params.ID,
							"event": event,
							"found": true,
						}
						output, _ := json.Marshal(result)
						return tool.Result{Output: output}, nil
					}
				}
				conn.mu.Unlock()

				time.Sleep(50 * time.Millisecond)
			}

			result := map[string]any{
				"id":      params.ID,
				"found":   false,
				"timeout": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func clearTool() tool.Tool {
	return tool.NewBuilder("sse_clear").
		WithDescription("Clear buffered events").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
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

			conn.mu.Lock()
			cleared := len(conn.events)
			conn.events = []sseEvent{}
			conn.mu.Unlock()

			result := map[string]any{
				"id":      params.ID,
				"cleared": cleared,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func statusTool() tool.Tool {
	return tool.NewBuilder("sse_status").
		WithDescription("Get SSE connection status").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
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

			conn.mu.Lock()
			result := map[string]any{
				"id":            params.ID,
				"url":           conn.url,
				"running":       conn.running,
				"buffered":      len(conn.events),
				"last_event_id": conn.lastID,
			}
			conn.mu.Unlock()

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("sse_list").
		WithDescription("List active SSE connections").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pool.mu.RLock()
			var connections []map[string]any
			for id, conn := range pool.conns {
				conn.mu.Lock()
				connections = append(connections, map[string]any{
					"id":       id,
					"url":      conn.url,
					"running":  conn.running,
					"buffered": len(conn.events),
				})
				conn.mu.Unlock()
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
	return tool.NewBuilder("sse_close_all").
		WithDescription("Close all SSE connections").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pool.mu.Lock()
			count := len(pool.conns)
			for id, conn := range pool.conns {
				conn.cancel()
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

func parseTool() tool.Tool {
	return tool.NewBuilder("sse_parse").
		WithDescription("Parse SSE message format").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var events []sseEvent
			var currentEvent sseEvent

			lines := strings.Split(params.Message, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)

				if line == "" {
					if currentEvent.Data != "" {
						events = append(events, currentEvent)
					}
					currentEvent = sseEvent{}
					continue
				}

				if strings.HasPrefix(line, ":") {
					continue
				}

				switch {
				case strings.HasPrefix(line, "data:"):
					data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
					if currentEvent.Data != "" {
						currentEvent.Data += "\n" + data
					} else {
						currentEvent.Data = data
					}
				case strings.HasPrefix(line, "event:"):
					currentEvent.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				case strings.HasPrefix(line, "id:"):
					currentEvent.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
				}
			}

			if currentEvent.Data != "" {
				events = append(events, currentEvent)
			}

			result := map[string]any{
				"events": events,
				"count":  len(events),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
