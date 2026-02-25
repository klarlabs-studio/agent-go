// Package calendar provides calendar management tools for agent-go.
//
// The pack uses an interface-based approach, allowing any calendar provider
// (Google Calendar, Outlook, CalDAV, etc.) to be plugged in.
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// CalendarProvider provides calendar operations.
type CalendarProvider interface {
	// CreateEvent creates a new calendar event.
	CreateEvent(ctx context.Context, event Event) (*Event, error)

	// GetEvent retrieves an event by ID.
	GetEvent(ctx context.Context, calendarID, eventID string) (*Event, error)

	// ListEvents lists events in a time range.
	ListEvents(ctx context.Context, calendarID string, start, end time.Time, opts ListOptions) ([]Event, error)

	// UpdateEvent updates an existing event.
	UpdateEvent(ctx context.Context, event Event) (*Event, error)

	// CancelEvent cancels an event.
	CancelEvent(ctx context.Context, calendarID, eventID string, notify bool) error

	// FindAvailability finds free time slots for participants.
	FindAvailability(ctx context.Context, participants []string, start, end time.Time, duration time.Duration) ([]TimeSlot, error)

	// ListCalendars returns available calendars.
	ListCalendars(ctx context.Context) ([]CalendarInfo, error)
}

// Event represents a calendar event.
type Event struct {
	ID           string            `json:"id,omitempty"`
	CalendarID   string            `json:"calendar_id,omitempty"`
	Title        string            `json:"title"`
	Description  string            `json:"description,omitempty"`
	Location     string            `json:"location,omitempty"`
	Start        time.Time         `json:"start"`
	End          time.Time         `json:"end"`
	AllDay       bool              `json:"all_day,omitempty"`
	Attendees    []Attendee        `json:"attendees,omitempty"`
	Recurrence   string            `json:"recurrence,omitempty"`
	Status       string            `json:"status,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Attendee represents an event participant.
type Attendee struct {
	Email    string `json:"email"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status,omitempty"` // "accepted", "declined", "tentative", "pending"
	Optional bool   `json:"optional,omitempty"`
}

// TimeSlot represents a free time period.
type TimeSlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// CalendarInfo describes a calendar.
type CalendarInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
}

// ListOptions configures event listing.
type ListOptions struct {
	MaxResults int    `json:"max_results,omitempty"`
	Query      string `json:"query,omitempty"`
	OrderBy    string `json:"order_by,omitempty"`
}

// Config holds calendar pack configuration.
type Config struct {
	// Provider is the calendar provider (required).
	Provider CalendarProvider

	// DefaultCalendarID is the default calendar to use.
	DefaultCalendarID string

	// DefaultTimezone is the default timezone for events.
	DefaultTimezone string
}

// Pack returns the calendar management tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &calendarPack{cfg: cfg}
	if p.cfg.DefaultTimezone == "" {
		p.cfg.DefaultTimezone = "UTC"
	}

	return pack.NewBuilder("calendar").
		WithDescription("Calendar management tools: create events, find availability, schedule meetings").
		WithVersion("1.0.0").
		AddTools(
			p.createEventTool(),
			p.listEventsTool(),
			p.getEventTool(),
			p.cancelEventTool(),
			p.findAvailabilityTool(),
			p.scheduleMeetingTool(),
			p.listCalendarsTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type calendarPack struct {
	cfg Config
}

func (p *calendarPack) calendarID(override string) string {
	if override != "" {
		return override
	}
	return p.cfg.DefaultCalendarID
}

func (p *calendarPack) createEventTool() tool.Tool {
	return tool.NewBuilder("calendar_create_event").
		WithDescription("Create a new calendar event").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				CalendarID  string     `json:"calendar_id,omitempty"`
				Title       string     `json:"title"`
				Description string     `json:"description,omitempty"`
				Location    string     `json:"location,omitempty"`
				Start       string     `json:"start"`
				End         string     `json:"end"`
				AllDay      bool       `json:"all_day,omitempty"`
				Attendees   []Attendee `json:"attendees,omitempty"`
				Recurrence  string     `json:"recurrence,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Title == "" {
				return tool.Result{}, fmt.Errorf("title is required")
			}
			if in.Start == "" {
				return tool.Result{}, fmt.Errorf("start time is required")
			}

			start, err := time.Parse(time.RFC3339, in.Start)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid start time: %w", err)
			}
			end := start.Add(time.Hour)
			if in.End != "" {
				end, err = time.Parse(time.RFC3339, in.End)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid end time: %w", err)
				}
			}

			event := Event{
				CalendarID:  p.calendarID(in.CalendarID),
				Title:       in.Title,
				Description: in.Description,
				Location:    in.Location,
				Start:       start,
				End:         end,
				AllDay:      in.AllDay,
				Attendees:   in.Attendees,
				Recurrence:  in.Recurrence,
			}

			created, err := p.cfg.Provider.CreateEvent(ctx, event)
			if err != nil {
				return tool.Result{}, fmt.Errorf("create event failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"event":   created,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *calendarPack) listEventsTool() tool.Tool {
	return tool.NewBuilder("calendar_list_events").
		WithDescription("List calendar events in a time range").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				CalendarID string `json:"calendar_id,omitempty"`
				Start      string `json:"start"`
				End        string `json:"end"`
				MaxResults int    `json:"max_results,omitempty"`
				Query      string `json:"query,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Start == "" || in.End == "" {
				return tool.Result{}, fmt.Errorf("start and end are required")
			}

			start, err := time.Parse(time.RFC3339, in.Start)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid start time: %w", err)
			}
			end, err := time.Parse(time.RFC3339, in.End)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid end time: %w", err)
			}

			events, err := p.cfg.Provider.ListEvents(ctx, p.calendarID(in.CalendarID), start, end, ListOptions{
				MaxResults: in.MaxResults,
				Query:      in.Query,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("list events failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(events),
				"events": events,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *calendarPack) getEventTool() tool.Tool {
	return tool.NewBuilder("calendar_get_event").
		WithDescription("Get a specific calendar event by ID").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				CalendarID string `json:"calendar_id,omitempty"`
				EventID    string `json:"event_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.EventID == "" {
				return tool.Result{}, fmt.Errorf("event_id is required")
			}

			event, err := p.cfg.Provider.GetEvent(ctx, p.calendarID(in.CalendarID), in.EventID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("get event failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"event": event,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *calendarPack) cancelEventTool() tool.Tool {
	return tool.NewBuilder("calendar_cancel_event").
		WithDescription("Cancel a calendar event").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				CalendarID string `json:"calendar_id,omitempty"`
				EventID    string `json:"event_id"`
				Notify     bool   `json:"notify,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.EventID == "" {
				return tool.Result{}, fmt.Errorf("event_id is required")
			}

			err := p.cfg.Provider.CancelEvent(ctx, p.calendarID(in.CalendarID), in.EventID, in.Notify)
			if err != nil {
				return tool.Result{}, fmt.Errorf("cancel event failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"event_id": in.EventID,
				"success":  true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *calendarPack) findAvailabilityTool() tool.Tool {
	return tool.NewBuilder("calendar_find_availability").
		WithDescription("Find free time slots for participants in a time range").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Participants []string `json:"participants"`
				Start        string   `json:"start"`
				End          string   `json:"end"`
				DurationMins int      `json:"duration_minutes,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if len(in.Participants) == 0 {
				return tool.Result{}, fmt.Errorf("participants is required")
			}
			if in.Start == "" || in.End == "" {
				return tool.Result{}, fmt.Errorf("start and end are required")
			}

			start, err := time.Parse(time.RFC3339, in.Start)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid start time: %w", err)
			}
			end, err := time.Parse(time.RFC3339, in.End)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid end time: %w", err)
			}

			dur := time.Duration(in.DurationMins) * time.Minute
			if dur == 0 {
				dur = 30 * time.Minute
			}

			slots, err := p.cfg.Provider.FindAvailability(ctx, in.Participants, start, end, dur)
			if err != nil {
				return tool.Result{}, fmt.Errorf("find availability failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"participants":     in.Participants,
				"duration_minutes": int(dur.Minutes()),
				"count":            len(slots),
				"slots":            slots,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *calendarPack) scheduleMeetingTool() tool.Tool {
	return tool.NewBuilder("calendar_schedule_meeting").
		WithDescription("Find availability and create a meeting for participants").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				CalendarID   string     `json:"calendar_id,omitempty"`
				Title        string     `json:"title"`
				Description  string     `json:"description,omitempty"`
				Attendees    []Attendee `json:"attendees"`
				Start        string     `json:"start"`
				End          string     `json:"end"`
				DurationMins int        `json:"duration_minutes,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Title == "" {
				return tool.Result{}, fmt.Errorf("title is required")
			}
			if len(in.Attendees) == 0 {
				return tool.Result{}, fmt.Errorf("attendees is required")
			}
			if in.Start == "" || in.End == "" {
				return tool.Result{}, fmt.Errorf("start and end are required")
			}

			start, err := time.Parse(time.RFC3339, in.Start)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid start time: %w", err)
			}
			end, err := time.Parse(time.RFC3339, in.End)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid end time: %w", err)
			}

			dur := time.Duration(in.DurationMins) * time.Minute
			if dur == 0 {
				dur = 30 * time.Minute
			}

			// Find availability first
			emails := make([]string, len(in.Attendees))
			for i, a := range in.Attendees {
				emails[i] = a.Email
			}

			slots, err := p.cfg.Provider.FindAvailability(ctx, emails, start, end, dur)
			if err != nil {
				return tool.Result{}, fmt.Errorf("find availability failed: %w", err)
			}
			if len(slots) == 0 {
				output, _ := json.Marshal(map[string]any{
					"success": false,
					"message": "no available time slots found",
				})
				return tool.Result{Output: output}, nil
			}

			// Use first available slot
			slot := slots[0]
			event := Event{
				CalendarID:  p.calendarID(in.CalendarID),
				Title:       in.Title,
				Description: in.Description,
				Start:       slot.Start,
				End:         slot.Start.Add(dur),
				Attendees:   in.Attendees,
			}

			created, err := p.cfg.Provider.CreateEvent(ctx, event)
			if err != nil {
				return tool.Result{}, fmt.Errorf("create meeting failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"event":   created,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *calendarPack) listCalendarsTool() tool.Tool {
	return tool.NewBuilder("calendar_list_calendars").
		WithDescription("List available calendars").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			calendars, err := p.cfg.Provider.ListCalendars(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("list calendars failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":     len(calendars),
				"calendars": calendars,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
