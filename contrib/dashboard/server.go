// Package dashboard provides a web-based monitoring dashboard for agent-go.
//
// The dashboard enables real-time monitoring of agent runs, including:
//   - Active run list with status indicators
//   - Real-time event streaming via WebSockets
//   - Run history and search
//   - Evidence and decision visualization
//   - Budget and constraint status
//
// # Usage
//
//	srv := dashboard.New(dashboard.Config{
//		EventStore: myEventStore,
//		RunStore:   myRunStore,
//		Address:    ":8080",
//	})
//
//	if err := srv.Start(); err != nil {
//		log.Fatal(err)
//	}
package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

//go:embed static/*
var staticFiles embed.FS

// Config configures the dashboard server.
type Config struct {
	// EventStore provides access to agent events.
	EventStore event.Store

	// RunStore provides access to run state.
	RunStore run.Store

	// Address is the HTTP listen address (default ":8080").
	Address string

	// BasePath is the URL path prefix (default "/").
	BasePath string

	// StaticDir is the path to static assets (optional).
	// If empty, embedded assets are used.
	StaticDir string

	// EnableCORS enables Cross-Origin Resource Sharing.
	EnableCORS bool

	// ReadTimeout is the HTTP read timeout.
	ReadTimeout time.Duration

	// WriteTimeout is the HTTP write timeout.
	WriteTimeout time.Duration
}

// Server is the dashboard HTTP server.
type Server struct {
	config     Config
	httpServer *http.Server
	mux        *http.ServeMux
	clients    map[string]map[chan event.Event]struct{} // runID -> clients
	mu         sync.RWMutex
}

// New creates a new dashboard server.
func New(cfg Config) *Server {
	if cfg.Address == "" {
		cfg.Address = ":8080"
	}
	if cfg.BasePath == "" {
		cfg.BasePath = "/"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}

	s := &Server{
		config:  cfg,
		mux:     http.NewServeMux(),
		clients: make(map[string]map[chan event.Event]struct{}),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the HTTP routes.
func (s *Server) setupRoutes() {
	base := s.config.BasePath

	// API routes
	s.mux.HandleFunc(base+"api/runs", s.handleListRuns)
	s.mux.HandleFunc(base+"api/runs/", s.handleGetRun)
	s.mux.HandleFunc(base+"api/runs/events", s.handleRunEvents)
	s.mux.HandleFunc(base+"api/health", s.handleHealth)

	// WebSocket route for real-time updates
	s.mux.HandleFunc(base+"ws/", s.handleWebSocket)

	// Static assets
	s.mux.HandleFunc(base, s.handleIndex)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         s.config.Address,
		Handler:      s.withMiddleware(s.mux),
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	return s.httpServer.ListenAndServe()
}

// StartTLS starts the HTTPS server.
func (s *Server) StartTLS(certFile, keyFile string) error {
	s.httpServer = &http.Server{
		Addr:         s.config.Address,
		Handler:      s.withMiddleware(s.mux),
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// withMiddleware wraps the handler with common middleware.
func (s *Server) withMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		if s.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")

		handler.ServeHTTP(w, r)
	})
}

// handleIndex serves the main dashboard page from embedded static files.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != s.config.BasePath && r.URL.Path != s.config.BasePath+"index.html" {
		// Try serving from embedded static files
		sub, err := fs.Sub(staticFiles, "static")
		if err == nil {
			http.FileServer(http.FS(sub)).ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
		return
	}
	// Serve index.html from embedded static files
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		// Fallback to legacy inline HTML
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// handleListRuns returns a list of all runs.
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.config.RunStore == nil {
		s.writeJSON(w, []RunSummary{})
		return
	}

	filter := run.ListFilter{
		Limit:      100,
		OrderBy:    run.OrderByStartTime,
		Descending: true,
	}

	// Parse optional query params: ?status=running&limit=50
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = []agent.RunStatus{agent.RunStatus(status)}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}

	runs, err := s.config.RunStore.List(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summaries := make([]RunSummary, 0, len(runs))
	for _, agentRun := range runs {
		summaries = append(summaries, RunSummary{
			ID:        agentRun.ID,
			Goal:      agentRun.Goal,
			State:     string(agentRun.CurrentState),
			Status:    string(agentRun.Status),
			StartTime: agentRun.StartTime,
			EndTime:   agentRun.EndTime,
		})
	}

	s.writeJSON(w, summaries)
}

// handleGetRun returns details for a specific run.
func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract run ID from path: /api/runs/{id}
	runID := strings.TrimPrefix(r.URL.Path, s.config.BasePath+"api/runs/")
	if runID == "" {
		http.Error(w, "Missing run ID", http.StatusBadRequest)
		return
	}

	if s.config.RunStore == nil {
		http.Error(w, "No run store configured", http.StatusServiceUnavailable)
		return
	}

	agentRun, err := s.config.RunStore.Get(r.Context(), runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	detail := RunDetail{
		RunSummary: RunSummary{
			ID:        agentRun.ID,
			Goal:      agentRun.Goal,
			State:     string(agentRun.CurrentState),
			Status:    string(agentRun.Status),
			StartTime: agentRun.StartTime,
			EndTime:   agentRun.EndTime,
		},
		Evidence: agentRun.Evidence,
		Vars:     agentRun.Vars,
	}

	// Load events if event store available
	if s.config.EventStore != nil {
		events, err := s.config.EventStore.LoadEvents(r.Context(), runID)
		if err == nil {
			detail.Events = events
		}
	}

	s.writeJSON(w, detail)
}

// handleRunEvents returns events for a run.
func (s *Server) handleRunEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse run_id query param
	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		http.Error(w, "Missing run_id parameter", http.StatusBadRequest)
		return
	}

	if s.config.EventStore == nil {
		s.writeJSON(w, []event.Event{})
		return
	}

	events, err := s.config.EventStore.LoadEvents(r.Context(), runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, events)
}

// handleWebSocket handles Server-Sent Events connections for real-time updates.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract run ID from path: /ws/{runID}
	runID := strings.TrimPrefix(r.URL.Path, s.config.BasePath+"ws/")
	if runID == "" {
		http.Error(w, "Missing run ID", http.StatusBadRequest)
		return
	}

	if s.config.EventStore == nil {
		http.Error(w, "No event store configured", http.StatusServiceUnavailable)
		return
	}

	// Use Server-Sent Events (no external deps needed)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Subscribe to events
	ch, err := s.config.EventStore.Subscribe(r.Context(), runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	health := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
	}
	s.writeJSON(w, health)
}

// writeJSON writes a JSON response.
func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// RunSummary is a condensed run representation for listing.
type RunSummary struct {
	ID        string    `json:"id"`
	Goal      string    `json:"goal"`
	State     string    `json:"state"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

// RunDetail is a full run representation with events.
type RunDetail struct {
	RunSummary
	Evidence []agent.Evidence `json:"evidence"`
	Events   []event.Event    `json:"events"`
	Vars     map[string]any   `json:"vars"`
}

// HealthStatus represents server health.
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version,omitempty"`
}

// indexHTML is the embedded dashboard HTML page.
const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Agent Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: #f5f5f5;
            color: #333;
            line-height: 1.6;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
        }
        header {
            background: #fff;
            padding: 20px;
            margin-bottom: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 {
            font-size: 24px;
            margin-bottom: 10px;
        }
        .status {
            color: #666;
            font-size: 14px;
        }
        .runs-list {
            background: #fff;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .runs-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
        }
        .runs-header h2 {
            font-size: 20px;
        }
        .refresh-btn {
            background: #007bff;
            color: #fff;
            border: none;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }
        .refresh-btn:hover {
            background: #0056b3;
        }
        .run-item {
            border-bottom: 1px solid #eee;
            padding: 15px 0;
            cursor: pointer;
            transition: background 0.2s;
        }
        .run-item:hover {
            background: #f9f9f9;
        }
        .run-item:last-child {
            border-bottom: none;
        }
        .run-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 8px;
        }
        .run-id {
            font-weight: 600;
            color: #007bff;
        }
        .run-status {
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 600;
        }
        .status-running { background: #ffc107; color: #000; }
        .status-completed { background: #28a745; color: #fff; }
        .status-failed { background: #dc3545; color: #fff; }
        .status-pending { background: #6c757d; color: #fff; }
        .status-paused { background: #17a2b8; color: #fff; }
        .run-goal {
            margin-bottom: 8px;
            color: #555;
        }
        .run-meta {
            display: flex;
            gap: 20px;
            font-size: 13px;
            color: #666;
        }
        .run-detail {
            background: #fff;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .detail-section {
            margin-bottom: 20px;
        }
        .detail-section h3 {
            font-size: 16px;
            margin-bottom: 10px;
            border-bottom: 2px solid #007bff;
            padding-bottom: 5px;
        }
        .evidence-item, .event-item {
            background: #f9f9f9;
            padding: 10px;
            margin-bottom: 10px;
            border-radius: 4px;
            font-size: 14px;
        }
        .back-btn {
            background: #6c757d;
            color: #fff;
            border: none;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
            margin-bottom: 20px;
        }
        .back-btn:hover {
            background: #5a6268;
        }
        .loading {
            text-align: center;
            padding: 40px;
            color: #666;
        }
        .error {
            background: #f8d7da;
            color: #721c24;
            padding: 15px;
            border-radius: 4px;
            margin-bottom: 20px;
        }
        pre {
            background: #f4f4f4;
            padding: 10px;
            border-radius: 4px;
            overflow-x: auto;
            font-size: 13px;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Agent Dashboard</h1>
            <div class="status" id="status">Loading...</div>
        </header>

        <div id="app"></div>
    </div>

    <script>
        const API_BASE = window.location.pathname.endsWith('/')
            ? window.location.pathname
            : window.location.pathname + '/';

        let currentView = 'list';
        let currentRunID = null;

        // Fetch health status
        async function checkHealth() {
            try {
                const response = await fetch(API_BASE + 'api/health');
                const health = await response.json();
                document.getElementById('status').textContent =
                    'Status: ' + health.status + ' | ' + new Date(health.timestamp).toLocaleString();
            } catch (err) {
                document.getElementById('status').textContent = 'Status: Offline';
            }
        }

        // Fetch runs list
        async function fetchRuns() {
            try {
                const response = await fetch(API_BASE + 'api/runs');
                const runs = await response.json();
                return runs;
            } catch (err) {
                console.error('Failed to fetch runs:', err);
                return [];
            }
        }

        // Fetch run detail
        async function fetchRunDetail(runID) {
            try {
                const response = await fetch(API_BASE + 'api/runs/' + runID);
                const detail = await response.json();
                return detail;
            } catch (err) {
                console.error('Failed to fetch run detail:', err);
                return null;
            }
        }

        // Render runs list
        function renderRunsList(runs) {
            const html = '<div class="runs-list">' +
                '<div class="runs-header">' +
                    '<h2>Runs</h2>' +
                    '<button class="refresh-btn" onclick="loadRuns()">Refresh</button>' +
                '</div>' +
                (runs.length === 0
                    ? '<p class="loading">No runs found</p>'
                    : runs.map(run =>
                        '<div class="run-item" onclick="showRunDetail(\'' + run.id + '\')">' +
                            '<div class="run-header">' +
                                '<span class="run-id">' + run.id + '</span>' +
                                '<span class="run-status status-' + run.status + '">' + run.status + '</span>' +
                            '</div>' +
                            '<div class="run-goal">' + run.goal + '</div>' +
                            '<div class="run-meta">' +
                                '<span>State: ' + run.state + '</span>' +
                                '<span>Started: ' + new Date(run.start_time).toLocaleString() + '</span>' +
                                (run.end_time ? '<span>Ended: ' + new Date(run.end_time).toLocaleString() + '</span>' : '') +
                            '</div>' +
                        '</div>'
                    ).join('')) +
                '</div>';

            document.getElementById('app').innerHTML = html;
        }

        // Render run detail
        function renderRunDetail(detail) {
            const html = '<button class="back-btn" onclick="showRunsList()">← Back to List</button>' +
                '<div class="run-detail">' +
                    '<h2>' + detail.id + '</h2>' +
                    '<div class="run-meta">' +
                        '<span class="run-status status-' + detail.status + '">' + detail.status + '</span>' +
                        '<span>State: ' + detail.state + '</span>' +
                    '</div>' +
                    '<div class="detail-section">' +
                        '<h3>Goal</h3>' +
                        '<p>' + detail.goal + '</p>' +
                    '</div>' +
                    (detail.evidence && detail.evidence.length > 0
                        ? '<div class="detail-section">' +
                            '<h3>Evidence (' + detail.evidence.length + ')</h3>' +
                            detail.evidence.map(e =>
                                '<div class="evidence-item">' +
                                    '<strong>' + e.source + '</strong>: ' + e.content.substring(0, 200) +
                                    (e.content.length > 200 ? '...' : '') +
                                '</div>'
                            ).join('') +
                          '</div>'
                        : '') +
                    (detail.events && detail.events.length > 0
                        ? '<div class="detail-section">' +
                            '<h3>Events (' + detail.events.length + ')</h3>' +
                            detail.events.slice(-10).reverse().map(e =>
                                '<div class="event-item">' +
                                    '<strong>' + e.type + '</strong> at ' + new Date(e.timestamp).toLocaleString() +
                                    '<pre>' + JSON.stringify(e.payload, null, 2) + '</pre>' +
                                '</div>'
                            ).join('') +
                          '</div>'
                        : '') +
                    (detail.vars && Object.keys(detail.vars).length > 0
                        ? '<div class="detail-section">' +
                            '<h3>Variables</h3>' +
                            '<pre>' + JSON.stringify(detail.vars, null, 2) + '</pre>' +
                          '</div>'
                        : '') +
                '</div>';

            document.getElementById('app').innerHTML = html;
        }

        // Show runs list
        async function showRunsList() {
            currentView = 'list';
            currentRunID = null;
            await loadRuns();
        }

        // Show run detail
        async function showRunDetail(runID) {
            currentView = 'detail';
            currentRunID = runID;
            document.getElementById('app').innerHTML = '<p class="loading">Loading...</p>';

            const detail = await fetchRunDetail(runID);
            if (detail) {
                renderRunDetail(detail);
            } else {
                document.getElementById('app').innerHTML =
                    '<div class="error">Failed to load run details</div>' +
                    '<button class="back-btn" onclick="showRunsList()">← Back to List</button>';
            }
        }

        // Load runs
        async function loadRuns() {
            document.getElementById('app').innerHTML = '<p class="loading">Loading runs...</p>';
            const runs = await fetchRuns();
            renderRunsList(runs);
        }

        // Initialize
        checkHealth();
        loadRuns();

        // Auto-refresh health every 30 seconds
        setInterval(checkHealth, 30000);

        // Auto-refresh runs list every 10 seconds if on list view
        setInterval(() => {
            if (currentView === 'list') {
                loadRuns();
            }
        }, 10000);
    </script>
</body>
</html>
`
