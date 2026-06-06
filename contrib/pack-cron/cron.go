// Package cron provides cron expression utilities for agents.
package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the cron tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("cron").
		WithDescription("Cron expression utilities").
		AddTools(
			parseTool(),
			validateTool(),
			nextRunTool(),
			describeTool(),
			buildTool(),
			matchesTool(),
			betweenTool(),
			commonExprTool(),
			convertTool(),
			humanReadableTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// CronExpr represents a parsed cron expression
type CronExpr struct {
	Minute     string
	Hour       string
	DayOfMonth string
	Month      string
	DayOfWeek  string
	Second     string // Optional 6th field
}

func parseCron(expr string) (*CronExpr, error) {
	parts := strings.Fields(expr)
	if len(parts) < 5 || len(parts) > 6 {
		return nil, fmt.Errorf("invalid cron expression: expected 5 or 6 fields, got %d", len(parts))
	}

	c := &CronExpr{}
	if len(parts) == 6 {
		c.Second = parts[0]
		c.Minute = parts[1]
		c.Hour = parts[2]
		c.DayOfMonth = parts[3]
		c.Month = parts[4]
		c.DayOfWeek = parts[5]
	} else {
		c.Second = "0"
		c.Minute = parts[0]
		c.Hour = parts[1]
		c.DayOfMonth = parts[2]
		c.Month = parts[3]
		c.DayOfWeek = parts[4]
	}

	return c, nil
}

func parseTool() tool.Tool {
	return tool.NewBuilder("cron_parse").
		WithDescription("Parse cron expression into components").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Expression string `json:"expression"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			expr, err := parseCron(params.Expression)
			if err != nil {
				result := map[string]any{
					"error":      err.Error(),
					"expression": params.Expression,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"expression":   params.Expression,
				"minute":       expr.Minute,
				"hour":         expr.Hour,
				"day_of_month": expr.DayOfMonth,
				"month":        expr.Month,
				"day_of_week":  expr.DayOfWeek,
				"second":       expr.Second,
				"has_seconds":  len(strings.Fields(params.Expression)) == 6,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("cron_validate").
		WithDescription("Validate cron expression syntax").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Expression string `json:"expression"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var errors []string

			expr, err := parseCron(params.Expression)
			if err != nil {
				errors = append(errors, err.Error())
			} else {
				// Validate each field
				if err := validateField(expr.Minute, 0, 59, "minute"); err != nil {
					errors = append(errors, err.Error())
				}
				if err := validateField(expr.Hour, 0, 23, "hour"); err != nil {
					errors = append(errors, err.Error())
				}
				if err := validateField(expr.DayOfMonth, 1, 31, "day of month"); err != nil {
					errors = append(errors, err.Error())
				}
				if err := validateField(expr.Month, 1, 12, "month"); err != nil {
					errors = append(errors, err.Error())
				}
				if err := validateField(expr.DayOfWeek, 0, 6, "day of week"); err != nil {
					errors = append(errors, err.Error())
				}
				if expr.Second != "" && expr.Second != "0" {
					if err := validateField(expr.Second, 0, 59, "second"); err != nil {
						errors = append(errors, err.Error())
					}
				}
			}

			result := map[string]any{
				"expression": params.Expression,
				"valid":      len(errors) == 0,
			}
			if len(errors) > 0 {
				result["errors"] = errors
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateField(field string, minVal, maxVal int, name string) error {
	if field == "*" {
		return nil
	}

	// Handle step values like */5
	if strings.HasPrefix(field, "*/") {
		step := strings.TrimPrefix(field, "*/")
		n, err := strconv.Atoi(step)
		if err != nil || n < 1 {
			return fmt.Errorf("%s: invalid step value '%s'", name, step)
		}
		return nil
	}

	// Handle ranges like 1-5
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return fmt.Errorf("%s: invalid range '%s'", name, field)
		}
		start, err1 := strconv.Atoi(parts[0])
		end, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("%s: invalid range values in '%s'", name, field)
		}
		if start < minVal || end > maxVal || start > end {
			return fmt.Errorf("%s: range %d-%d out of bounds (%d-%d)", name, start, end, minVal, maxVal)
		}
		return nil
	}

	// Handle lists like 1,3,5
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		for _, p := range parts {
			if err := validateField(strings.TrimSpace(p), minVal, maxVal, name); err != nil {
				return err
			}
		}
		return nil
	}

	// Single value
	n, err := strconv.Atoi(field)
	if err != nil {
		return fmt.Errorf("%s: invalid value '%s'", name, field)
	}
	if n < minVal || n > maxVal {
		return fmt.Errorf("%s: value %d out of range (%d-%d)", name, n, minVal, maxVal)
	}

	return nil
}

func nextRunTool() tool.Tool {
	return tool.NewBuilder("cron_next_run").
		WithDescription("Calculate next run times for cron expression").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Expression string `json:"expression"`
				Count      int    `json:"count,omitempty"`
				After      string `json:"after,omitempty"` // RFC3339 timestamp
				Timezone   string `json:"timezone,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			expr, err := parseCron(params.Expression)
			if err != nil {
				result := map[string]any{"error": err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			count := params.Count
			if count <= 0 {
				count = 5
			}
			if count > 100 {
				count = 100
			}

			loc := time.UTC
			if params.Timezone != "" {
				var err error
				loc, err = time.LoadLocation(params.Timezone)
				if err != nil {
					result := map[string]any{"error": "invalid timezone: " + params.Timezone}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			start := time.Now().In(loc)
			if params.After != "" {
				parsed, err := time.Parse(time.RFC3339, params.After)
				if err == nil {
					start = parsed.In(loc)
				}
			}

			// Calculate next runs (simplified - just iterate minute by minute)
			var nextRuns []string
			current := start.Truncate(time.Minute).Add(time.Minute)

			for i := 0; i < 365*24*60 && len(nextRuns) < count; i++ {
				if matchesCron(current, expr) {
					nextRuns = append(nextRuns, current.Format(time.RFC3339))
				}
				current = current.Add(time.Minute)
			}

			result := map[string]any{
				"expression": params.Expression,
				"next_runs":  nextRuns,
				"count":      len(nextRuns),
				"timezone":   loc.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func matchesCron(t time.Time, expr *CronExpr) bool {
	return matchesField(t.Minute(), expr.Minute) &&
		matchesField(t.Hour(), expr.Hour) &&
		matchesField(t.Day(), expr.DayOfMonth) &&
		matchesField(int(t.Month()), expr.Month) &&
		matchesField(int(t.Weekday()), expr.DayOfWeek)
}

func matchesField(value int, field string) bool {
	if field == "*" {
		return true
	}

	// Handle step values
	if strings.HasPrefix(field, "*/") {
		step, _ := strconv.Atoi(strings.TrimPrefix(field, "*/"))
		return value%step == 0
	}

	// Handle ranges
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		start, _ := strconv.Atoi(parts[0])
		end, _ := strconv.Atoi(parts[1])
		return value >= start && value <= end
	}

	// Handle lists
	if strings.Contains(field, ",") {
		for _, p := range strings.Split(field, ",") {
			if matchesField(value, strings.TrimSpace(p)) {
				return true
			}
		}
		return false
	}

	// Single value
	n, _ := strconv.Atoi(field)
	return value == n
}

func describeTool() tool.Tool {
	return tool.NewBuilder("cron_describe").
		WithDescription("Describe cron expression in human-readable format").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Expression string `json:"expression"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			expr, err := parseCron(params.Expression)
			if err != nil {
				result := map[string]any{"error": err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			description := describeCron(expr)

			result := map[string]any{
				"expression":  params.Expression,
				"description": description,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func describeCron(expr *CronExpr) string {
	var parts []string

	// Describe minute
	if expr.Minute == "*" {
		parts = append(parts, "every minute")
	} else if strings.HasPrefix(expr.Minute, "*/") {
		step := strings.TrimPrefix(expr.Minute, "*/")
		parts = append(parts, "every "+step+" minutes")
	} else {
		parts = append(parts, "at minute "+expr.Minute)
	}

	// Describe hour
	if expr.Hour == "*" {
		parts = append(parts, "of every hour")
	} else if strings.HasPrefix(expr.Hour, "*/") {
		step := strings.TrimPrefix(expr.Hour, "*/")
		parts = append(parts, "every "+step+" hours")
	} else {
		parts = append(parts, "past hour "+expr.Hour)
	}

	// Describe day of month
	if expr.DayOfMonth != "*" {
		if strings.HasPrefix(expr.DayOfMonth, "*/") {
			step := strings.TrimPrefix(expr.DayOfMonth, "*/")
			parts = append(parts, "every "+step+" days")
		} else {
			parts = append(parts, "on day "+expr.DayOfMonth)
		}
	}

	// Describe month
	if expr.Month != "*" {
		months := []string{"", "January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December"}
		if n, err := strconv.Atoi(expr.Month); err == nil && n >= 1 && n <= 12 {
			parts = append(parts, "in "+months[n])
		} else {
			parts = append(parts, "in month "+expr.Month)
		}
	}

	// Describe day of week
	if expr.DayOfWeek != "*" {
		days := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
		if n, err := strconv.Atoi(expr.DayOfWeek); err == nil && n >= 0 && n <= 6 {
			parts = append(parts, "on "+days[n])
		} else {
			parts = append(parts, "on weekday "+expr.DayOfWeek)
		}
	}

	return strings.Join(parts, " ")
}

func buildTool() tool.Tool {
	return tool.NewBuilder("cron_build").
		WithDescription("Build cron expression from parameters").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Minute     string `json:"minute,omitempty"`
				Hour       string `json:"hour,omitempty"`
				DayOfMonth string `json:"day_of_month,omitempty"`
				Month      string `json:"month,omitempty"`
				DayOfWeek  string `json:"day_of_week,omitempty"`
				Second     string `json:"second,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Set defaults
			if params.Minute == "" {
				params.Minute = "*"
			}
			if params.Hour == "" {
				params.Hour = "*"
			}
			if params.DayOfMonth == "" {
				params.DayOfMonth = "*"
			}
			if params.Month == "" {
				params.Month = "*"
			}
			if params.DayOfWeek == "" {
				params.DayOfWeek = "*"
			}

			var expression string
			if params.Second != "" {
				expression = fmt.Sprintf("%s %s %s %s %s %s",
					params.Second, params.Minute, params.Hour,
					params.DayOfMonth, params.Month, params.DayOfWeek)
			} else {
				expression = fmt.Sprintf("%s %s %s %s %s",
					params.Minute, params.Hour,
					params.DayOfMonth, params.Month, params.DayOfWeek)
			}

			result := map[string]any{
				"expression":   expression,
				"minute":       params.Minute,
				"hour":         params.Hour,
				"day_of_month": params.DayOfMonth,
				"month":        params.Month,
				"day_of_week":  params.DayOfWeek,
			}
			if params.Second != "" {
				result["second"] = params.Second
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func matchesTool() tool.Tool {
	return tool.NewBuilder("cron_matches").
		WithDescription("Check if a time matches a cron expression").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Expression string `json:"expression"`
				Time       string `json:"time"` // RFC3339
				Timezone   string `json:"timezone,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			expr, err := parseCron(params.Expression)
			if err != nil {
				result := map[string]any{"error": err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			t, err := time.Parse(time.RFC3339, params.Time)
			if err != nil {
				result := map[string]any{"error": "invalid time format: " + err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			if params.Timezone != "" {
				loc, err := time.LoadLocation(params.Timezone)
				if err == nil {
					t = t.In(loc)
				}
			}

			matches := matchesCron(t, expr)

			result := map[string]any{
				"expression": params.Expression,
				"time":       t.Format(time.RFC3339),
				"matches":    matches,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func betweenTool() tool.Tool {
	return tool.NewBuilder("cron_between").
		WithDescription("Find all cron matches between two times").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Expression string `json:"expression"`
				Start      string `json:"start"` // RFC3339
				End        string `json:"end"`   // RFC3339
				Limit      int    `json:"limit,omitempty"`
				Timezone   string `json:"timezone,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			expr, err := parseCron(params.Expression)
			if err != nil {
				result := map[string]any{"error": err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			startTime, err := time.Parse(time.RFC3339, params.Start)
			if err != nil {
				result := map[string]any{"error": "invalid start time: " + err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			endTime, err := time.Parse(time.RFC3339, params.End)
			if err != nil {
				result := map[string]any{"error": "invalid end time: " + err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			loc := time.UTC
			if params.Timezone != "" {
				loc, _ = time.LoadLocation(params.Timezone)
			}
			startTime = startTime.In(loc)
			endTime = endTime.In(loc)

			limit := params.Limit
			if limit <= 0 {
				limit = 100
			}
			if limit > 1000 {
				limit = 1000
			}

			var matches []string
			current := startTime.Truncate(time.Minute)

			for current.Before(endTime) && len(matches) < limit {
				if matchesCron(current, expr) {
					matches = append(matches, current.Format(time.RFC3339))
				}
				current = current.Add(time.Minute)
			}

			result := map[string]any{
				"expression": params.Expression,
				"start":      startTime.Format(time.RFC3339),
				"end":        endTime.Format(time.RFC3339),
				"matches":    matches,
				"count":      len(matches),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func commonExprTool() tool.Tool {
	return tool.NewBuilder("cron_common").
		WithDescription("Get common cron expressions").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			common := map[string]string{
				"every_minute":       "* * * * *",
				"every_5_minutes":    "*/5 * * * *",
				"every_10_minutes":   "*/10 * * * *",
				"every_15_minutes":   "*/15 * * * *",
				"every_30_minutes":   "*/30 * * * *",
				"every_hour":         "0 * * * *",
				"every_2_hours":      "0 */2 * * *",
				"every_day_midnight": "0 0 * * *",
				"every_day_noon":     "0 12 * * *",
				"every_monday":       "0 0 * * 1",
				"every_weekday":      "0 0 * * 1-5",
				"every_weekend":      "0 0 * * 0,6",
				"every_month":        "0 0 1 * *",
				"every_quarter":      "0 0 1 1,4,7,10 *",
				"every_year":         "0 0 1 1 *",
			}

			result := map[string]any{
				"expressions": common,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func convertTool() tool.Tool {
	return tool.NewBuilder("cron_convert").
		WithDescription("Convert between cron formats").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Expression string `json:"expression"`
				From       string `json:"from,omitempty"` // standard, quartz, aws
				To         string `json:"to,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			expr, err := parseCron(params.Expression)
			if err != nil {
				result := map[string]any{"error": err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Standard 5-field
			standard := fmt.Sprintf("%s %s %s %s %s",
				expr.Minute, expr.Hour, expr.DayOfMonth, expr.Month, expr.DayOfWeek)

			// Quartz 6-field (with seconds)
			quartz := fmt.Sprintf("%s %s %s %s %s %s",
				expr.Second, expr.Minute, expr.Hour, expr.DayOfMonth, expr.Month, expr.DayOfWeek)

			// AWS CloudWatch Events (with year field)
			aws := fmt.Sprintf("cron(%s %s %s %s %s *)",
				expr.Minute, expr.Hour, expr.DayOfMonth, expr.Month, expr.DayOfWeek)

			result := map[string]any{
				"input":    params.Expression,
				"standard": standard,
				"quartz":   quartz,
				"aws":      aws,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func humanReadableTool() tool.Tool {
	return tool.NewBuilder("cron_from_text").
		WithDescription("Convert human-readable schedule to cron").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			text := strings.ToLower(strings.TrimSpace(params.Text))
			var expression, description string

			switch {
			case strings.Contains(text, "every minute"):
				expression = "* * * * *"
				description = "Every minute"
			case strings.Contains(text, "every hour"):
				expression = "0 * * * *"
				description = "Every hour at minute 0"
			case strings.Contains(text, "every day") && strings.Contains(text, "midnight"):
				expression = "0 0 * * *"
				description = "Every day at midnight"
			case strings.Contains(text, "every day") && strings.Contains(text, "noon"):
				expression = "0 12 * * *"
				description = "Every day at noon"
			case strings.Contains(text, "every monday"):
				expression = "0 0 * * 1"
				description = "Every Monday at midnight"
			case strings.Contains(text, "every weekday"):
				expression = "0 9 * * 1-5"
				description = "Every weekday at 9 AM"
			case strings.Contains(text, "every weekend"):
				expression = "0 9 * * 0,6"
				description = "Every weekend at 9 AM"
			case strings.Contains(text, "every month"):
				expression = "0 0 1 * *"
				description = "First day of every month at midnight"
			case strings.Contains(text, "every year"):
				expression = "0 0 1 1 *"
				description = "January 1st at midnight"
			default:
				result := map[string]any{
					"error": "could not parse schedule text",
					"text":  params.Text,
					"hint":  "Try: 'every minute', 'every hour', 'every day at midnight', 'every monday', 'every weekday', 'every weekend', 'every month', 'every year'",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"text":        params.Text,
				"expression":  expression,
				"description": description,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
