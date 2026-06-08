// Package websocket provides WebSocket client tools for agents.
package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// ConnectionPool manages WebSocket connections.
type ConnectionPool struct {
	mu    sync.RWMutex
	conns map[string]*websocket.Conn
}

var pool = &ConnectionPool{
	conns: make(map[string]*websocket.Conn),
}

// Pack returns the WebSocket tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("websocket").
		WithDescription("WebSocket client tools").
		AddTools(
			connectTool(),
			disconnectTool(),
			sendTool(),
			receiveTool(),
			sendJSONTool(),
			receiveJSONTool(),
			pingTool(),
			listTool(),
			closeAllTool(),
			sendReceiveTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func connectTool() tool.Tool {
	return tool.NewBuilder("ws_connect").
		WithDescription("Connect to a WebSocket server").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL     string            `json:"url"`
				ID      string            `json:"id,omitempty"`
				Headers map[string]string `json:"headers,omitempty"`
				Timeout int               `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			id := params.ID
			if id == "" {
				id = params.URL
			}

			timeout := time.Duration(params.Timeout) * time.Millisecond
			if timeout <= 0 {
				timeout = 30 * time.Second
			}

			dialer := websocket.Dialer{
				HandshakeTimeout: timeout,
			}

			header := http.Header{}
			for k, v := range params.Headers {
				header.Set(k, v)
			}

			conn, resp, err := dialer.DialContext(ctx, params.URL, header)
			if err != nil {
				return tool.Result{}, err
			}

			pool.mu.Lock()
			if existing, ok := pool.conns[id]; ok {
				_ = existing.Close()
			}
			pool.conns[id] = conn
			pool.mu.Unlock()

			result := map[string]any{
				"id":          id,
				"url":         params.URL,
				"connected":   true,
				"status_code": resp.StatusCode,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func disconnectTool() tool.Tool {
	return tool.NewBuilder("ws_disconnect").
		WithDescription("Disconnect from a WebSocket server").
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
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				_ = conn.Close()
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

func sendTool() tool.Tool {
	return tool.NewBuilder("ws_send").
		WithDescription("Send a text message through WebSocket").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID      string `json:"id"`
				Message string `json:"message"`
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

			err := conn.WriteMessage(websocket.TextMessage, []byte(params.Message))
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"id":   params.ID,
				"sent": true,
				"size": len(params.Message),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func receiveTool() tool.Tool {
	return tool.NewBuilder("ws_receive").
		WithDescription("Receive a message from WebSocket").
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
				timeout = 30 * time.Second
			}

			_ = conn.SetReadDeadline(time.Now().Add(timeout))
			messageType, message, err := conn.ReadMessage()
			_ = conn.SetReadDeadline(time.Time{})

			if err != nil {
				return tool.Result{}, err
			}

			msgType := "unknown"
			switch messageType {
			case websocket.TextMessage:
				msgType = "text"
			case websocket.BinaryMessage:
				msgType = "binary"
			}

			result := map[string]any{
				"id":      params.ID,
				"message": string(message),
				"type":    msgType,
				"size":    len(message),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sendJSONTool() tool.Tool {
	return tool.NewBuilder("ws_send_json").
		WithDescription("Send a JSON message through WebSocket").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID   string         `json:"id"`
				Data map[string]any `json:"data"`
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

			err := conn.WriteJSON(params.Data)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"id":   params.ID,
				"sent": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func receiveJSONTool() tool.Tool {
	return tool.NewBuilder("ws_receive_json").
		WithDescription("Receive a JSON message from WebSocket").
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
				timeout = 30 * time.Second
			}

			_ = conn.SetReadDeadline(time.Now().Add(timeout))
			var data map[string]any
			err := conn.ReadJSON(&data)
			_ = conn.SetReadDeadline(time.Time{})

			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"id":   params.ID,
				"data": data,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pingTool() tool.Tool {
	return tool.NewBuilder("ws_ping").
		WithDescription("Send a ping to WebSocket server").
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

			start := time.Now()
			pongReceived := make(chan bool, 1)

			conn.SetPongHandler(func(string) error {
				pongReceived <- true
				return nil
			})

			err := conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return tool.Result{}, err
			}

			select {
			case <-pongReceived:
				latency := time.Since(start)
				result := map[string]any{
					"id":         params.ID,
					"pong":       true,
					"latency_ms": latency.Milliseconds(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			case <-time.After(timeout):
				result := map[string]any{
					"id":      params.ID,
					"pong":    false,
					"timeout": true,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("ws_list").
		WithDescription("List active WebSocket connections").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pool.mu.RLock()
			var connections []string
			for id := range pool.conns {
				connections = append(connections, id)
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
	return tool.NewBuilder("ws_close_all").
		WithDescription("Close all WebSocket connections").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pool.mu.Lock()
			count := len(pool.conns)
			for id, conn := range pool.conns {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				_ = conn.Close()
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

func sendReceiveTool() tool.Tool {
	return tool.NewBuilder("ws_send_receive").
		WithDescription("Send a message and wait for response").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID      string `json:"id"`
				Message string `json:"message"`
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
				timeout = 30 * time.Second
			}

			start := time.Now()

			// Send
			err := conn.WriteMessage(websocket.TextMessage, []byte(params.Message))
			if err != nil {
				return tool.Result{}, err
			}

			// Receive
			_ = conn.SetReadDeadline(time.Now().Add(timeout))
			_, response, err := conn.ReadMessage()
			_ = conn.SetReadDeadline(time.Time{})

			if err != nil {
				return tool.Result{}, err
			}

			latency := time.Since(start)

			result := map[string]any{
				"id":         params.ID,
				"sent":       params.Message,
				"received":   string(response),
				"latency_ms": latency.Milliseconds(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
