// Package time provides tools for date/time manipulation and timezone conversions.
package time

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

type timePack struct{}

// Pack creates a new time tools pack.
func Pack() *pack.Pack {
	p := &timePack{}

	return pack.NewBuilder("time").
		WithDescription("Tools for date/time manipulation and timezone conversions").
		WithVersion("1.0.0").
		AddTools(
			p.nowTool(),
			p.parseTool(),
			p.formatTool(),
			p.convertTimezoneTool(),
			p.addTool(),
			p.subtractTool(),
			p.diffTool(),
			p.compareTool(),
			p.startOfTool(),
			p.endOfTool(),
			p.isBeforeTool(),
			p.isAfterTool(),
			p.isBetweenTool(),
			p.unixTool(),
			p.fromUnixTool(),
			p.listTimzonesTool(),
			p.weekdayTool(),
			p.isLeapYearTool(),
			p.daysInMonthTool(),
			p.relativeTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// Common format layouts
var formatLayouts = map[string]string{
	"iso8601":    time.RFC3339,
	"rfc3339":    time.RFC3339,
	"rfc822":     time.RFC822,
	"rfc850":     time.RFC850,
	"rfc1123":    time.RFC1123,
	"unix_date":  time.UnixDate,
	"ruby_date":  time.RubyDate,
	"ansic":      time.ANSIC,
	"kitchen":    time.Kitchen,
	"stamp":      time.Stamp,
	"date":       "2006-01-02",
	"time":       "15:04:05",
	"datetime":   "2006-01-02 15:04:05",
	"date_us":    "01/02/2006",
	"date_eu":    "02/01/2006",
	"year_month": "2006-01",
	"month_day":  "01-02",
}

func getLayout(format string) string {
	if layout, ok := formatLayouts[strings.ToLower(format)]; ok {
		return layout
	}
	return format
}

// nowTool returns current time.
func (p *timePack) nowTool() tool.Tool {
	return tool.NewBuilder("time_now").
		WithDescription("Get the current date and time").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Timezone string `json:"timezone,omitempty"`
				Format   string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			now := time.Now()

			if params.Timezone != "" {
				loc, err := time.LoadLocation(params.Timezone)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid timezone: %w", err)
				}
				now = now.In(loc)
			}

			format := params.Format
			if format == "" {
				format = time.RFC3339
			} else {
				format = getLayout(format)
			}

			result := map[string]interface{}{
				"time":       now.Format(format),
				"unix":       now.Unix(),
				"unix_milli": now.UnixMilli(),
				"timezone":   now.Location().String(),
				"offset":     now.Format("-07:00"),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// parseTool parses a time string.
func (p *timePack) parseTool() tool.Tool {
	return tool.NewBuilder("time_parse").
		WithDescription("Parse a date/time string").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time     string `json:"time"`
				Format   string `json:"format,omitempty"`
				Timezone string `json:"timezone,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" {
				return tool.Result{}, fmt.Errorf("time is required")
			}

			var loc *time.Location = time.UTC
			if params.Timezone != "" {
				var err error
				loc, err = time.LoadLocation(params.Timezone)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid timezone: %w", err)
				}
			}

			var t time.Time
			var err error

			if params.Format != "" {
				layout := getLayout(params.Format)
				t, err = time.ParseInLocation(layout, params.Time, loc)
			} else {
				// Try common formats
				formats := []string{
					time.RFC3339,
					time.RFC3339Nano,
					"2006-01-02T15:04:05",
					"2006-01-02 15:04:05",
					"2006-01-02",
					"01/02/2006",
					"02/01/2006",
					"Jan 2, 2006",
					"January 2, 2006",
					time.RFC822,
					time.RFC1123,
				}
				for _, f := range formats {
					t, err = time.ParseInLocation(f, params.Time, loc)
					if err == nil {
						break
					}
				}
			}

			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
			}

			result := map[string]interface{}{
				"iso8601":     t.Format(time.RFC3339),
				"unix":        t.Unix(),
				"year":        t.Year(),
				"month":       int(t.Month()),
				"day":         t.Day(),
				"hour":        t.Hour(),
				"minute":      t.Minute(),
				"second":      t.Second(),
				"weekday":     t.Weekday().String(),
				"day_of_year": t.YearDay(),
				"timezone":    t.Location().String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// formatTool formats a time.
func (p *timePack) formatTool() tool.Tool {
	return tool.NewBuilder("time_format").
		WithDescription("Format a date/time in a specific format").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time   string `json:"time"`
				Format string `json:"format"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" || params.Format == "" {
				return tool.Result{}, fmt.Errorf("time and format are required")
			}

			t, err := time.Parse(time.RFC3339, params.Time)
			if err != nil {
				// Try other formats
				t, err = time.Parse("2006-01-02 15:04:05", params.Time)
				if err != nil {
					t, err = time.Parse("2006-01-02", params.Time)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
					}
				}
			}

			layout := getLayout(params.Format)
			formatted := t.Format(layout)

			result := map[string]interface{}{
				"formatted": formatted,
				"format":    params.Format,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// convertTimezoneTool converts between timezones.
func (p *timePack) convertTimezoneTool() tool.Tool {
	return tool.NewBuilder("time_convert_timezone").
		WithDescription("Convert a time between timezones").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time         string `json:"time"`
				FromTimezone string `json:"from_timezone,omitempty"`
				ToTimezone   string `json:"to_timezone"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" || params.ToTimezone == "" {
				return tool.Result{}, fmt.Errorf("time and to_timezone are required")
			}

			fromLoc := time.UTC
			if params.FromTimezone != "" {
				var err error
				fromLoc, err = time.LoadLocation(params.FromTimezone)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid from_timezone: %w", err)
				}
			}

			toLoc, err := time.LoadLocation(params.ToTimezone)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid to_timezone: %w", err)
			}

			t, err := time.ParseInLocation(time.RFC3339, params.Time, fromLoc)
			if err != nil {
				t, err = time.ParseInLocation("2006-01-02 15:04:05", params.Time, fromLoc)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
				}
			}

			converted := t.In(toLoc)

			result := map[string]interface{}{
				"original":      t.Format(time.RFC3339),
				"converted":     converted.Format(time.RFC3339),
				"from_timezone": fromLoc.String(),
				"to_timezone":   toLoc.String(),
				"offset_change": converted.Format("-07:00"),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// addTool adds duration to time.
func (p *timePack) addTool() tool.Tool {
	return tool.NewBuilder("time_add").
		WithDescription("Add duration to a time").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time    string `json:"time"`
				Years   int    `json:"years,omitempty"`
				Months  int    `json:"months,omitempty"`
				Days    int    `json:"days,omitempty"`
				Hours   int    `json:"hours,omitempty"`
				Minutes int    `json:"minutes,omitempty"`
				Seconds int    `json:"seconds,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" {
				return tool.Result{}, fmt.Errorf("time is required")
			}

			t, err := time.Parse(time.RFC3339, params.Time)
			if err != nil {
				t, err = time.Parse("2006-01-02 15:04:05", params.Time)
				if err != nil {
					t, err = time.Parse("2006-01-02", params.Time)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
					}
				}
			}

			// Add years, months, days
			t = t.AddDate(params.Years, params.Months, params.Days)

			// Add hours, minutes, seconds
			duration := time.Duration(params.Hours)*time.Hour +
				time.Duration(params.Minutes)*time.Minute +
				time.Duration(params.Seconds)*time.Second
			t = t.Add(duration)

			result := map[string]interface{}{
				"result": t.Format(time.RFC3339),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// subtractTool subtracts duration from time.
func (p *timePack) subtractTool() tool.Tool {
	return tool.NewBuilder("time_subtract").
		WithDescription("Subtract duration from a time").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time    string `json:"time"`
				Years   int    `json:"years,omitempty"`
				Months  int    `json:"months,omitempty"`
				Days    int    `json:"days,omitempty"`
				Hours   int    `json:"hours,omitempty"`
				Minutes int    `json:"minutes,omitempty"`
				Seconds int    `json:"seconds,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" {
				return tool.Result{}, fmt.Errorf("time is required")
			}

			t, err := time.Parse(time.RFC3339, params.Time)
			if err != nil {
				t, err = time.Parse("2006-01-02 15:04:05", params.Time)
				if err != nil {
					t, err = time.Parse("2006-01-02", params.Time)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
					}
				}
			}

			// Subtract years, months, days
			t = t.AddDate(-params.Years, -params.Months, -params.Days)

			// Subtract hours, minutes, seconds
			duration := time.Duration(params.Hours)*time.Hour +
				time.Duration(params.Minutes)*time.Minute +
				time.Duration(params.Seconds)*time.Second
			t = t.Add(-duration)

			result := map[string]interface{}{
				"result": t.Format(time.RFC3339),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// diffTool calculates difference between two times.
func (p *timePack) diffTool() tool.Tool {
	return tool.NewBuilder("time_diff").
		WithDescription("Calculate the difference between two times").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				From string `json:"from"`
				To   string `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.From == "" || params.To == "" {
				return tool.Result{}, fmt.Errorf("from and to are required")
			}

			parseTime := func(s string) (time.Time, error) {
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					t, err = time.Parse("2006-01-02 15:04:05", s)
					if err != nil {
						t, err = time.Parse("2006-01-02", s)
					}
				}
				return t, err
			}

			from, err := parseTime(params.From)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse from time: %w", err)
			}

			to, err := parseTime(params.To)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse to time: %w", err)
			}

			diff := to.Sub(from)

			totalSeconds := int64(diff.Seconds())
			totalMinutes := totalSeconds / 60
			totalHours := totalMinutes / 60
			totalDays := totalHours / 24

			result := map[string]interface{}{
				"total_seconds": totalSeconds,
				"total_minutes": totalMinutes,
				"total_hours":   totalHours,
				"total_days":    totalDays,
				"duration":      diff.String(),
				"is_negative":   diff < 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// compareTool compares two times.
func (p *timePack) compareTool() tool.Tool {
	return tool.NewBuilder("time_compare").
		WithDescription("Compare two times").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A string `json:"a"`
				B string `json:"b"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.A == "" || params.B == "" {
				return tool.Result{}, fmt.Errorf("a and b are required")
			}

			parseTime := func(s string) (time.Time, error) {
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					t, err = time.Parse("2006-01-02 15:04:05", s)
					if err != nil {
						t, err = time.Parse("2006-01-02", s)
					}
				}
				return t, err
			}

			a, err := parseTime(params.A)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse a: %w", err)
			}

			b, err := parseTime(params.B)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse b: %w", err)
			}

			var comparison string
			if a.Before(b) {
				comparison = "before"
			} else if a.After(b) {
				comparison = "after"
			} else {
				comparison = "equal"
			}

			result := map[string]interface{}{
				"comparison": comparison,
				"a_before_b": a.Before(b),
				"a_after_b":  a.After(b),
				"equal":      a.Equal(b),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// startOfTool gets start of period.
func (p *timePack) startOfTool() tool.Tool {
	return tool.NewBuilder("time_start_of").
		WithDescription("Get the start of a time period (day, week, month, year)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time   string `json:"time"`
				Period string `json:"period"` // day, week, month, year, hour, minute
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" || params.Period == "" {
				return tool.Result{}, fmt.Errorf("time and period are required")
			}

			t, err := time.Parse(time.RFC3339, params.Time)
			if err != nil {
				t, err = time.Parse("2006-01-02 15:04:05", params.Time)
				if err != nil {
					t, err = time.Parse("2006-01-02", params.Time)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
					}
				}
			}

			var startOf time.Time
			switch strings.ToLower(params.Period) {
			case "minute":
				startOf = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
			case "hour":
				startOf = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
			case "day":
				startOf = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			case "week":
				weekday := int(t.Weekday())
				startOf = time.Date(t.Year(), t.Month(), t.Day()-weekday, 0, 0, 0, 0, t.Location())
			case "month":
				startOf = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
			case "year":
				startOf = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
			default:
				return tool.Result{}, fmt.Errorf("unsupported period: %s", params.Period)
			}

			result := map[string]interface{}{
				"result": startOf.Format(time.RFC3339),
				"period": params.Period,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// endOfTool gets end of period.
func (p *timePack) endOfTool() tool.Tool {
	return tool.NewBuilder("time_end_of").
		WithDescription("Get the end of a time period (day, week, month, year)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time   string `json:"time"`
				Period string `json:"period"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" || params.Period == "" {
				return tool.Result{}, fmt.Errorf("time and period are required")
			}

			t, err := time.Parse(time.RFC3339, params.Time)
			if err != nil {
				t, err = time.Parse("2006-01-02 15:04:05", params.Time)
				if err != nil {
					t, err = time.Parse("2006-01-02", params.Time)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
					}
				}
			}

			var endOf time.Time
			switch strings.ToLower(params.Period) {
			case "minute":
				endOf = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 59, 999999999, t.Location())
			case "hour":
				endOf = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 59, 59, 999999999, t.Location())
			case "day":
				endOf = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
			case "week":
				weekday := int(t.Weekday())
				daysUntilSaturday := 6 - weekday
				endOf = time.Date(t.Year(), t.Month(), t.Day()+daysUntilSaturday, 23, 59, 59, 999999999, t.Location())
			case "month":
				endOf = time.Date(t.Year(), t.Month()+1, 0, 23, 59, 59, 999999999, t.Location())
			case "year":
				endOf = time.Date(t.Year(), 12, 31, 23, 59, 59, 999999999, t.Location())
			default:
				return tool.Result{}, fmt.Errorf("unsupported period: %s", params.Period)
			}

			result := map[string]interface{}{
				"result": endOf.Format(time.RFC3339),
				"period": params.Period,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// isBeforeTool checks if time is before another.
func (p *timePack) isBeforeTool() tool.Tool {
	return tool.NewBuilder("time_is_before").
		WithDescription("Check if a time is before another time").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time   string `json:"time"`
				Before string `json:"before"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			parseTime := func(s string) (time.Time, error) {
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					t, err = time.Parse("2006-01-02", s)
				}
				return t, err
			}

			t, err := parseTime(params.Time)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
			}

			before, err := parseTime(params.Before)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse before: %w", err)
			}

			result := map[string]interface{}{
				"is_before": t.Before(before),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// isAfterTool checks if time is after another.
func (p *timePack) isAfterTool() tool.Tool {
	return tool.NewBuilder("time_is_after").
		WithDescription("Check if a time is after another time").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time  string `json:"time"`
				After string `json:"after"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			parseTime := func(s string) (time.Time, error) {
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					t, err = time.Parse("2006-01-02", s)
				}
				return t, err
			}

			t, err := parseTime(params.Time)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
			}

			after, err := parseTime(params.After)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse after: %w", err)
			}

			result := map[string]interface{}{
				"is_after": t.After(after),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// isBetweenTool checks if time is between two times.
func (p *timePack) isBetweenTool() tool.Tool {
	return tool.NewBuilder("time_is_between").
		WithDescription("Check if a time is between two other times").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time  string `json:"time"`
				Start string `json:"start"`
				End   string `json:"end"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			parseTime := func(s string) (time.Time, error) {
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					t, err = time.Parse("2006-01-02", s)
				}
				return t, err
			}

			t, err := parseTime(params.Time)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
			}

			start, err := parseTime(params.Start)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse start: %w", err)
			}

			end, err := parseTime(params.End)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse end: %w", err)
			}

			isBetween := (t.Equal(start) || t.After(start)) && (t.Equal(end) || t.Before(end))

			result := map[string]interface{}{
				"is_between": isBetween,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// unixTool converts time to Unix timestamp.
func (p *timePack) unixTool() tool.Tool {
	return tool.NewBuilder("time_unix").
		WithDescription("Convert a time to Unix timestamp").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time string `json:"time"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" {
				return tool.Result{}, fmt.Errorf("time is required")
			}

			t, err := time.Parse(time.RFC3339, params.Time)
			if err != nil {
				t, err = time.Parse("2006-01-02 15:04:05", params.Time)
				if err != nil {
					t, err = time.Parse("2006-01-02", params.Time)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
					}
				}
			}

			result := map[string]interface{}{
				"unix":       t.Unix(),
				"unix_milli": t.UnixMilli(),
				"unix_nano":  t.UnixNano(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// fromUnixTool converts Unix timestamp to time.
func (p *timePack) fromUnixTool() tool.Tool {
	return tool.NewBuilder("time_from_unix").
		WithDescription("Convert a Unix timestamp to time").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Timestamp int64  `json:"timestamp"`
				Unit      string `json:"unit,omitempty"` // seconds, milliseconds, nanoseconds
				Timezone  string `json:"timezone,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			unit := strings.ToLower(params.Unit)
			if unit == "" {
				unit = "seconds"
			}

			var t time.Time
			switch unit {
			case "seconds", "s":
				t = time.Unix(params.Timestamp, 0)
			case "milliseconds", "ms":
				t = time.UnixMilli(params.Timestamp)
			case "nanoseconds", "ns":
				t = time.Unix(0, params.Timestamp)
			default:
				return tool.Result{}, fmt.Errorf("unsupported unit: %s", unit)
			}

			if params.Timezone != "" {
				loc, err := time.LoadLocation(params.Timezone)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid timezone: %w", err)
				}
				t = t.In(loc)
			}

			result := map[string]interface{}{
				"time":     t.Format(time.RFC3339),
				"timezone": t.Location().String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// listTimzonesTool lists common timezones.
func (p *timePack) listTimzonesTool() tool.Tool {
	return tool.NewBuilder("time_list_timezones").
		WithDescription("List common timezones").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			timezones := []map[string]interface{}{
				{"name": "UTC", "offset": "+00:00"},
				{"name": "America/New_York", "offset": "-05:00"},
				{"name": "America/Chicago", "offset": "-06:00"},
				{"name": "America/Denver", "offset": "-07:00"},
				{"name": "America/Los_Angeles", "offset": "-08:00"},
				{"name": "America/Sao_Paulo", "offset": "-03:00"},
				{"name": "Europe/London", "offset": "+00:00"},
				{"name": "Europe/Paris", "offset": "+01:00"},
				{"name": "Europe/Berlin", "offset": "+01:00"},
				{"name": "Europe/Moscow", "offset": "+03:00"},
				{"name": "Asia/Dubai", "offset": "+04:00"},
				{"name": "Asia/Kolkata", "offset": "+05:30"},
				{"name": "Asia/Shanghai", "offset": "+08:00"},
				{"name": "Asia/Tokyo", "offset": "+09:00"},
				{"name": "Australia/Sydney", "offset": "+11:00"},
				{"name": "Pacific/Auckland", "offset": "+13:00"},
			}

			result := map[string]interface{}{
				"timezones": timezones,
				"count":     len(timezones),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// weekdayTool gets weekday information.
func (p *timePack) weekdayTool() tool.Tool {
	return tool.NewBuilder("time_weekday").
		WithDescription("Get weekday information for a date").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time string `json:"time"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" {
				return tool.Result{}, fmt.Errorf("time is required")
			}

			t, err := time.Parse(time.RFC3339, params.Time)
			if err != nil {
				t, err = time.Parse("2006-01-02", params.Time)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
				}
			}

			weekday := t.Weekday()

			result := map[string]interface{}{
				"weekday":     weekday.String(),
				"weekday_num": int(weekday),
				"is_weekend":  weekday == time.Saturday || weekday == time.Sunday,
				"is_weekday":  weekday >= time.Monday && weekday <= time.Friday,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// isLeapYearTool checks if year is a leap year.
func (p *timePack) isLeapYearTool() tool.Tool {
	return tool.NewBuilder("time_is_leap_year").
		WithDescription("Check if a year is a leap year").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Year int `json:"year"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Year == 0 {
				return tool.Result{}, fmt.Errorf("year is required")
			}

			isLeap := params.Year%4 == 0 && (params.Year%100 != 0 || params.Year%400 == 0)

			result := map[string]interface{}{
				"is_leap_year": isLeap,
				"year":         params.Year,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// daysInMonthTool gets days in a month.
func (p *timePack) daysInMonthTool() tool.Tool {
	return tool.NewBuilder("time_days_in_month").
		WithDescription("Get the number of days in a month").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Year  int `json:"year"`
				Month int `json:"month"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Year == 0 || params.Month < 1 || params.Month > 12 {
				return tool.Result{}, fmt.Errorf("valid year and month (1-12) are required")
			}

			// Get the first day of the next month, then subtract one day
			t := time.Date(params.Year, time.Month(params.Month+1), 0, 0, 0, 0, 0, time.UTC)

			result := map[string]interface{}{
				"days":  t.Day(),
				"year":  params.Year,
				"month": params.Month,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// relativeTool formats time as relative string.
func (p *timePack) relativeTool() tool.Tool {
	return tool.NewBuilder("time_relative").
		WithDescription("Format time as relative string (e.g., '2 hours ago')").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Time string `json:"time"`
				From string `json:"from,omitempty"` // Reference time, defaults to now
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Time == "" {
				return tool.Result{}, fmt.Errorf("time is required")
			}

			t, err := time.Parse(time.RFC3339, params.Time)
			if err != nil {
				t, err = time.Parse("2006-01-02 15:04:05", params.Time)
				if err != nil {
					t, err = time.Parse("2006-01-02", params.Time)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to parse time: %w", err)
					}
				}
			}

			from := time.Now()
			if params.From != "" {
				from, err = time.Parse(time.RFC3339, params.From)
				if err != nil {
					from, err = time.Parse("2006-01-02", params.From)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to parse from: %w", err)
					}
				}
			}

			diff := from.Sub(t)
			isPast := diff > 0
			if diff < 0 {
				diff = -diff
			}

			var relative string
			switch {
			case diff < time.Minute:
				relative = "just now"
			case diff < time.Hour:
				mins := int(diff.Minutes())
				if isPast {
					relative = fmt.Sprintf("%d minute(s) ago", mins)
				} else {
					relative = fmt.Sprintf("in %d minute(s)", mins)
				}
			case diff < 24*time.Hour:
				hours := int(diff.Hours())
				if isPast {
					relative = fmt.Sprintf("%d hour(s) ago", hours)
				} else {
					relative = fmt.Sprintf("in %d hour(s)", hours)
				}
			case diff < 30*24*time.Hour:
				days := int(diff.Hours() / 24)
				if isPast {
					relative = fmt.Sprintf("%d day(s) ago", days)
				} else {
					relative = fmt.Sprintf("in %d day(s)", days)
				}
			case diff < 365*24*time.Hour:
				months := int(diff.Hours() / 24 / 30)
				if isPast {
					relative = fmt.Sprintf("%d month(s) ago", months)
				} else {
					relative = fmt.Sprintf("in %d month(s)", months)
				}
			default:
				years := int(diff.Hours() / 24 / 365)
				if isPast {
					relative = fmt.Sprintf("%d year(s) ago", years)
				} else {
					relative = fmt.Sprintf("in %d year(s)", years)
				}
			}

			result := map[string]interface{}{
				"relative": relative,
				"is_past":  isPast,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
