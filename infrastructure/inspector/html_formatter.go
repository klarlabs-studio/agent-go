package inspector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"

	"go.klarlabs.de/agent/domain/inspector"
)

// HTMLFormatter formats data as an interactive HTML timeline.
type HTMLFormatter struct {
	title string
	theme string
}

// HTMLFormatterOption configures the HTML formatter.
type HTMLFormatterOption func(*HTMLFormatter)

// WithTitle sets the page title.
func WithTitle(title string) HTMLFormatterOption {
	return func(f *HTMLFormatter) {
		f.title = title
	}
}

// WithTheme sets the color theme (light or dark).
func WithTheme(theme string) HTMLFormatterOption {
	return func(f *HTMLFormatter) {
		f.theme = theme
	}
}

// NewHTMLFormatter creates a new HTML formatter.
func NewHTMLFormatter(opts ...HTMLFormatterOption) *HTMLFormatter {
	f := &HTMLFormatter{
		title: "Agent Run Timeline",
		theme: "light",
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Format formats the data as HTML.
func (f *HTMLFormatter) Format(data any) ([]byte, error) {
	// Convert data to JSON for embedding in HTML
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	tmpl, err := template.New("timeline").Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{
		"Title": f.title,
		"Theme": f.theme,
		"Data":  template.JS(jsonData),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

// FormatType returns the format type.
func (f *HTMLFormatter) FormatType() inspector.ExportFormat {
	return inspector.FormatHTML
}

// Ensure HTMLFormatter implements inspector.Formatter
var _ inspector.Formatter = (*HTMLFormatter)(nil)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        :root {
            --bg-color: {{if eq .Theme "dark"}}#1a1a2e{{else}}#ffffff{{end}};
            --text-color: {{if eq .Theme "dark"}}#eaeaea{{else}}#333333{{end}};
            --border-color: {{if eq .Theme "dark"}}#333366{{else}}#e0e0e0{{end}};
            --card-bg: {{if eq .Theme "dark"}}#16213e{{else}}#f8f9fa{{end}};
            --success-color: #28a745;
            --error-color: #dc3545;
            --warning-color: #ffc107;
            --info-color: #17a2b8;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            line-height: 1.6;
            padding: 20px;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { margin-bottom: 20px; font-size: 1.8em; }
        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 30px;
        }
        .summary-card {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 15px;
        }
        .summary-card h3 { font-size: 0.9em; opacity: 0.7; margin-bottom: 5px; }
        .summary-card .value { font-size: 1.5em; font-weight: bold; }
        .timeline {
            position: relative;
            padding-left: 30px;
        }
        .timeline::before {
            content: '';
            position: absolute;
            left: 10px;
            top: 0;
            bottom: 0;
            width: 2px;
            background: var(--border-color);
        }
        .timeline-item {
            position: relative;
            margin-bottom: 20px;
            padding: 15px;
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 8px;
        }
        .timeline-item::before {
            content: '';
            position: absolute;
            left: -24px;
            top: 20px;
            width: 10px;
            height: 10px;
            border-radius: 50%;
            background: var(--info-color);
        }
        .timeline-item.success::before { background: var(--success-color); }
        .timeline-item.error::before { background: var(--error-color); }
        .timeline-item.transition::before { background: var(--warning-color); }
        .timeline-item .header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 10px;
        }
        .timeline-item .type {
            font-weight: bold;
            font-size: 1.1em;
        }
        .timeline-item .time {
            font-size: 0.85em;
            opacity: 0.7;
        }
        .timeline-item .details {
            font-size: 0.9em;
            opacity: 0.8;
        }
        .badge {
            display: inline-block;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 0.8em;
            font-weight: bold;
        }
        .badge-state { background: var(--info-color); color: white; }
        .badge-success { background: var(--success-color); color: white; }
        .badge-error { background: var(--error-color); color: white; }
        .tabs {
            display: flex;
            border-bottom: 2px solid var(--border-color);
            margin-bottom: 20px;
        }
        .tab {
            padding: 10px 20px;
            cursor: pointer;
            border-bottom: 2px solid transparent;
            margin-bottom: -2px;
        }
        .tab.active { border-bottom-color: var(--info-color); font-weight: bold; }
        .tab-content { display: none; }
        .tab-content.active { display: block; }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 15px;
        }
        th, td {
            padding: 10px;
            text-align: left;
            border-bottom: 1px solid var(--border-color);
        }
        th { font-weight: bold; opacity: 0.7; }
    </style>
</head>
<body>
    <div class="container">
        <h1>{{.Title}}</h1>
        <div id="content"></div>
    </div>
    <script>
        const data = {{.Data}};

        function formatDuration(ns) {
            if (!ns) return '-';
            const ms = ns / 1000000;
            if (ms < 1000) return ms.toFixed(2) + 'ms';
            return (ms / 1000).toFixed(2) + 's';
        }

        function formatTime(ts) {
            if (!ts) return '-';
            return new Date(ts).toLocaleString();
        }

        function renderRunExport(data) {
            if (!data.run) return '<p>No run data available</p>';

            let html = '<div class="summary">';
            html += '<div class="summary-card"><h3>Run ID</h3><div class="value">' + data.run.id + '</div></div>';
            html += '<div class="summary-card"><h3>Status</h3><div class="value">' + data.run.status + '</div></div>';
            html += '<div class="summary-card"><h3>State</h3><div class="value">' + data.run.state + '</div></div>';
            if (data.metrics) {
                html += '<div class="summary-card"><h3>Tool Calls</h3><div class="value">' + data.metrics.tool_call_count + '</div></div>';
                html += '<div class="summary-card"><h3>Duration</h3><div class="value">' + formatDuration(data.metrics.total_duration) + '</div></div>';
            }
            html += '</div>';

            html += '<div class="tabs">';
            html += '<div class="tab active" data-tab="timeline">Timeline</div>';
            html += '<div class="tab" data-tab="tools">Tool Calls</div>';
            html += '<div class="tab" data-tab="transitions">Transitions</div>';
            html += '</div>';

            html += '<div id="timeline" class="tab-content active">' + renderTimeline(data.timeline || []) + '</div>';
            html += '<div id="tools" class="tab-content">' + renderToolCalls(data.tool_calls || []) + '</div>';
            html += '<div id="transitions" class="tab-content">' + renderTransitions(data.transitions || []) + '</div>';

            return html;
        }

        function renderTimeline(timeline) {
            if (!timeline.length) return '<p>No timeline events</p>';
            let html = '<div class="timeline">';
            timeline.forEach(entry => {
                let itemClass = 'timeline-item';
                if (entry.type.includes('Succeeded')) itemClass += ' success';
                else if (entry.type.includes('Failed')) itemClass += ' error';
                else if (entry.type.includes('Transition')) itemClass += ' transition';

                html += '<div class="' + itemClass + '">';
                html += '<div class="header">';
                html += '<span class="type">' + entry.label + '</span>';
                html += '<span class="time">' + formatTime(entry.timestamp) + '</span>';
                html += '</div>';
                if (entry.state) {
                    html += '<span class="badge badge-state">' + entry.state + '</span> ';
                }
                if (entry.duration) {
                    html += '<span class="details">Duration: ' + formatDuration(entry.duration) + '</span>';
                }
                html += '</div>';
            });
            html += '</div>';
            return html;
        }

        function renderToolCalls(toolCalls) {
            if (!toolCalls.length) return '<p>No tool calls</p>';
            let html = '<table><thead><tr><th>Tool</th><th>State</th><th>Duration</th><th>Status</th></tr></thead><tbody>';
            toolCalls.forEach(tc => {
                const statusClass = tc.success ? 'badge-success' : 'badge-error';
                const statusText = tc.success ? 'Success' : 'Failed';
                html += '<tr>';
                html += '<td>' + tc.name + '</td>';
                html += '<td>' + tc.state + '</td>';
                html += '<td>' + formatDuration(tc.duration) + '</td>';
                html += '<td><span class="badge ' + statusClass + '">' + statusText + '</span></td>';
                html += '</tr>';
            });
            html += '</tbody></table>';
            return html;
        }

        function renderTransitions(transitions) {
            if (!transitions.length) return '<p>No transitions</p>';
            let html = '<table><thead><tr><th>From</th><th>To</th><th>Reason</th><th>Duration</th></tr></thead><tbody>';
            transitions.forEach(t => {
                html += '<tr>';
                html += '<td>' + t.from + '</td>';
                html += '<td>' + t.to + '</td>';
                html += '<td>' + (t.reason || '-') + '</td>';
                html += '<td>' + formatDuration(t.duration) + '</td>';
                html += '</tr>';
            });
            html += '</tbody></table>';
            return html;
        }

        function renderStateMachine(data) {
            let html = '<div class="summary">';
            html += '<div class="summary-card"><h3>Initial State</h3><div class="value">' + data.initial + '</div></div>';
            html += '<div class="summary-card"><h3>States</h3><div class="value">' + (data.states ? data.states.length : 0) + '</div></div>';
            html += '<div class="summary-card"><h3>Transitions</h3><div class="value">' + (data.transitions ? data.transitions.length : 0) + '</div></div>';
            html += '</div>';

            if (data.states && data.states.length) {
                html += '<h2>States</h2><table><thead><tr><th>Name</th><th>Terminal</th><th>Side Effects</th></tr></thead><tbody>';
                data.states.forEach(s => {
                    html += '<tr><td>' + s.name + '</td>';
                    html += '<td>' + (s.is_terminal ? 'Yes' : 'No') + '</td>';
                    html += '<td>' + (s.allows_side_effects ? 'Yes' : 'No') + '</td></tr>';
                });
                html += '</tbody></table>';
            }
            return html;
        }

        function renderMetrics(data) {
            let html = '<div class="summary">';
            if (data.summary) {
                html += '<div class="summary-card"><h3>Total Runs</h3><div class="value">' + data.summary.total_runs + '</div></div>';
                html += '<div class="summary-card"><h3>Completed</h3><div class="value">' + data.summary.completed_runs + '</div></div>';
                html += '<div class="summary-card"><h3>Failed</h3><div class="value">' + data.summary.failed_runs + '</div></div>';
            }
            html += '</div>';

            if (data.tool_metrics && data.tool_metrics.length) {
                html += '<h2>Tool Metrics</h2><table><thead><tr><th>Tool</th><th>Calls</th><th>Success Rate</th><th>Avg Duration</th></tr></thead><tbody>';
                data.tool_metrics.forEach(tm => {
                    html += '<tr><td>' + tm.name + '</td>';
                    html += '<td>' + tm.call_count + '</td>';
                    html += '<td>' + (tm.success_rate * 100).toFixed(1) + '%</td>';
                    html += '<td>' + formatDuration(tm.average_duration) + '</td></tr>';
                });
                html += '</tbody></table>';
            }
            return html;
        }

        function render() {
            const content = document.getElementById('content');
            let html = '';

            if (data.run) {
                html = renderRunExport(data);
            } else if (data.states) {
                html = renderStateMachine(data);
            } else if (data.summary) {
                html = renderMetrics(data);
            } else {
                html = '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
            }

            content.innerHTML = html;

            // Tab switching
            document.querySelectorAll('.tab').forEach(tab => {
                tab.addEventListener('click', function() {
                    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
                    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
                    this.classList.add('active');
                    document.getElementById(this.dataset.tab).classList.add('active');
                });
            });
        }

        render();
    </script>
</body>
</html>`
