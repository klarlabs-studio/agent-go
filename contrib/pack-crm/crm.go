// Package crm provides CRM integration tools for agent-go.
//
// The pack uses an interface-based approach, allowing any CRM platform
// (Salesforce, HubSpot, Pipedrive, custom, etc.) to be plugged in.
package crm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// CRMPlatform provides CRM capabilities.
type CRMPlatform interface {
	CreateContact(ctx context.Context, contact Contact) (*ContactResult, error)
	UpdateContact(ctx context.Context, contactID string, fields map[string]any) (*ContactResult, error)
	SearchContacts(ctx context.Context, query SearchQuery) (*SearchResult, error)
	LogActivity(ctx context.Context, activity Activity) (*ActivityResult, error)
	UpdateDeal(ctx context.Context, dealID string, fields map[string]any) (*DealResult, error)
	SyncContacts(ctx context.Context, opts SyncOptions) (*SyncResult, error)
}

// Contact represents a CRM contact.
type Contact struct {
	FirstName  string            `json:"first_name"`
	LastName   string            `json:"last_name"`
	Email      string            `json:"email,omitempty"`
	Phone      string            `json:"phone,omitempty"`
	Company    string            `json:"company,omitempty"`
	Title      string            `json:"title,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Properties map[string]any    `json:"properties,omitempty"`
}

// ContactResult contains contact operation output.
type ContactResult struct {
	ID        string         `json:"id"`
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	Email     string         `json:"email,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
}

// SearchQuery configures a CRM search.
type SearchQuery struct {
	Query   string            `json:"query,omitempty"`
	Filters map[string]string `json:"filters,omitempty"`
	Limit   int               `json:"limit,omitempty"`
	Offset  int               `json:"offset,omitempty"`
}

// SearchResult contains search output.
type SearchResult struct {
	Contacts []ContactResult `json:"contacts"`
	Total    int             `json:"total"`
	HasMore  bool            `json:"has_more"`
}

// Activity represents a CRM activity log entry.
type Activity struct {
	ContactID string         `json:"contact_id"`
	Type      string         `json:"type"` // "call", "email", "meeting", "note", "task"
	Subject   string         `json:"subject"`
	Body      string         `json:"body,omitempty"`
	DealID    string         `json:"deal_id,omitempty"`
	DueDate   string         `json:"due_date,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

// ActivityResult contains activity log output.
type ActivityResult struct {
	ID        string `json:"id"`
	ContactID string `json:"contact_id"`
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
}

// DealResult contains deal operation output.
type DealResult struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Stage     string  `json:"stage"`
	Amount    float64 `json:"amount,omitempty"`
	Currency  string  `json:"currency,omitempty"`
	UpdatedAt string  `json:"updated_at"`
}

// SyncOptions configures contact synchronization.
type SyncOptions struct {
	Source    string `json:"source"` // external system identifier
	Direction string `json:"direction,omitempty"` // "import", "export", "bidirectional"
	Since     string `json:"since,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

// SyncResult contains synchronization output.
type SyncResult struct {
	Created  int `json:"created"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
	Errors   int `json:"errors"`
}

// Config holds CRM pack configuration.
type Config struct {
	Platform CRMPlatform
}

// Pack returns the CRM integration tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &crmPack{cfg: cfg}

	return pack.NewBuilder("crm").
		WithDescription("CRM integration tools: create_contact, log_activity, update_deal, search, sync_contacts").
		WithVersion("1.0.0").
		AddTools(
			p.createContactTool(), p.updateContactTool(), p.searchContactsTool(),
			p.logActivityTool(), p.updateDealTool(), p.syncContactsTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type crmPack struct{ cfg Config }

func (p *crmPack) createContactTool() tool.Tool {
	return tool.NewBuilder("crm_create_contact").
		WithDescription("Create a new CRM contact").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in Contact
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.FirstName == "" && in.LastName == "" {
				return tool.Result{}, fmt.Errorf("first_name or last_name is required")
			}
			result, err := p.cfg.Platform.CreateContact(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("create contact failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *crmPack) updateContactTool() tool.Tool {
	return tool.NewBuilder("crm_update_contact").
		WithDescription("Update an existing CRM contact").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ContactID string         `json:"contact_id"`
				Fields    map[string]any `json:"fields"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ContactID == "" {
				return tool.Result{}, fmt.Errorf("contact_id is required")
			}
			if len(in.Fields) == 0 {
				return tool.Result{}, fmt.Errorf("at least one field to update is required")
			}
			result, err := p.cfg.Platform.UpdateContact(ctx, in.ContactID, in.Fields)
			if err != nil {
				return tool.Result{}, fmt.Errorf("update contact failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *crmPack) searchContactsTool() tool.Tool {
	return tool.NewBuilder("crm_search_contacts").
		WithDescription("Search CRM contacts by query or filters").
		ReadOnly().Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in SearchQuery
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Query == "" && len(in.Filters) == 0 {
				return tool.Result{}, fmt.Errorf("query or filters are required")
			}
			if in.Limit == 0 {
				in.Limit = 25
			}
			result, err := p.cfg.Platform.SearchContacts(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("search contacts failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *crmPack) logActivityTool() tool.Tool {
	return tool.NewBuilder("crm_log_activity").
		WithDescription("Log an activity against a CRM contact").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in Activity
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ContactID == "" {
				return tool.Result{}, fmt.Errorf("contact_id is required")
			}
			if in.Type == "" {
				return tool.Result{}, fmt.Errorf("activity type is required")
			}
			if in.Subject == "" {
				return tool.Result{}, fmt.Errorf("subject is required")
			}
			result, err := p.cfg.Platform.LogActivity(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("log activity failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *crmPack) updateDealTool() tool.Tool {
	return tool.NewBuilder("crm_update_deal").
		WithDescription("Update a CRM deal's stage, amount, or properties").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				DealID string         `json:"deal_id"`
				Fields map[string]any `json:"fields"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.DealID == "" {
				return tool.Result{}, fmt.Errorf("deal_id is required")
			}
			if len(in.Fields) == 0 {
				return tool.Result{}, fmt.Errorf("at least one field to update is required")
			}
			result, err := p.cfg.Platform.UpdateDeal(ctx, in.DealID, in.Fields)
			if err != nil {
				return tool.Result{}, fmt.Errorf("update deal failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *crmPack) syncContactsTool() tool.Tool {
	return tool.NewBuilder("crm_sync_contacts").
		WithDescription("Synchronize contacts with an external system").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in SyncOptions
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Source == "" {
				return tool.Result{}, fmt.Errorf("source is required")
			}
			if in.Direction == "" {
				in.Direction = "import"
			}
			result, err := p.cfg.Platform.SyncContacts(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("sync contacts failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
