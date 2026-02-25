// Package linear provides Linear project management integration tools for agent-go.
//
// The pack uses an interface-based approach, allowing the Linear GraphQL API
// or any compatible project tracker to be plugged in.
package linear

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// LinearClient provides Linear project management operations.
type LinearClient interface {
	// CreateIssue creates a new issue.
	CreateIssue(ctx context.Context, issue IssueInput) (*Issue, error)

	// UpdateIssue updates an existing issue.
	UpdateIssue(ctx context.Context, issueID string, update IssueUpdate) (*Issue, error)

	// GetIssue retrieves an issue by ID or identifier.
	GetIssue(ctx context.Context, issueID string) (*Issue, error)

	// SearchIssues searches for issues.
	SearchIssues(ctx context.Context, query string, opts SearchOptions) ([]Issue, error)

	// AddComment adds a comment to an issue.
	AddComment(ctx context.Context, issueID, body string) (*Comment, error)

	// GetCycle retrieves the current or specified cycle.
	GetCycle(ctx context.Context, teamID string, cycleNumber int) (*Cycle, error)

	// ListTeams lists available teams.
	ListTeams(ctx context.Context) ([]Team, error)
}

// IssueInput defines fields for creating an issue.
type IssueInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	TeamID      string   `json:"team_id"`
	AssigneeID  string   `json:"assignee_id,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	StateID     string   `json:"state_id,omitempty"`
	LabelIDs    []string `json:"label_ids,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
	CycleID     string   `json:"cycle_id,omitempty"`
	Estimate    int      `json:"estimate,omitempty"`
}

// IssueUpdate defines fields for updating an issue.
type IssueUpdate struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	AssigneeID  *string  `json:"assignee_id,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	StateID     *string  `json:"state_id,omitempty"`
	LabelIDs    []string `json:"label_ids,omitempty"`
	Estimate    *int     `json:"estimate,omitempty"`
}

// Issue represents a Linear issue.
type Issue struct {
	ID          string   `json:"id"`
	Identifier  string   `json:"identifier"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	State       string   `json:"state,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	Team        string   `json:"team,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Estimate    int      `json:"estimate,omitempty"`
	URL         string   `json:"url,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

// Comment represents an issue comment.
type Comment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	User      string `json:"user,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// Cycle represents a development cycle/sprint.
type Cycle struct {
	ID        string  `json:"id"`
	Number    int     `json:"number"`
	Name      string  `json:"name,omitempty"`
	StartsAt  string  `json:"starts_at,omitempty"`
	EndsAt    string  `json:"ends_at,omitempty"`
	Progress  float64 `json:"progress,omitempty"`
	Issues    int     `json:"issues,omitempty"`
	Completed int     `json:"completed,omitempty"`
}

// Team represents a Linear team.
type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

// SearchOptions configures issue search.
type SearchOptions struct {
	TeamID     string `json:"team_id,omitempty"`
	AssigneeID string `json:"assignee_id,omitempty"`
	State      string `json:"state,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

// Config holds Linear pack configuration.
type Config struct {
	// Client is the Linear client (required).
	Client LinearClient

	// DefaultTeamID is the default team for operations.
	DefaultTeamID string
}

// Pack returns the Linear integration tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &linearPack{cfg: cfg}

	return pack.NewBuilder("linear").
		WithDescription("Linear project management tools: issues, comments, cycles, search").
		WithVersion("1.0.0").
		AddTools(
			p.createIssueTool(),
			p.updateIssueTool(),
			p.getIssueTool(),
			p.searchIssuesTool(),
			p.addCommentTool(),
			p.getCycleTool(),
			p.listTeamsTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type linearPack struct {
	cfg Config
}

func (p *linearPack) createIssueTool() tool.Tool {
	return tool.NewBuilder("linear_create_issue").
		WithDescription("Create a new Linear issue").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in IssueInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Title == "" {
				return tool.Result{}, fmt.Errorf("title is required")
			}
			if in.TeamID == "" {
				in.TeamID = p.cfg.DefaultTeamID
			}
			if in.TeamID == "" {
				return tool.Result{}, fmt.Errorf("team_id is required")
			}

			issue, err := p.cfg.Client.CreateIssue(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("create issue failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"issue":   issue,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *linearPack) updateIssueTool() tool.Tool {
	return tool.NewBuilder("linear_update_issue").
		WithDescription("Update an existing Linear issue").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				IssueID string      `json:"issue_id"`
				Update  IssueUpdate `json:"update"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.IssueID == "" {
				return tool.Result{}, fmt.Errorf("issue_id is required")
			}

			issue, err := p.cfg.Client.UpdateIssue(ctx, in.IssueID, in.Update)
			if err != nil {
				return tool.Result{}, fmt.Errorf("update issue failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"issue":   issue,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *linearPack) getIssueTool() tool.Tool {
	return tool.NewBuilder("linear_get_issue").
		WithDescription("Get a Linear issue by ID or identifier").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				IssueID string `json:"issue_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.IssueID == "" {
				return tool.Result{}, fmt.Errorf("issue_id is required")
			}

			issue, err := p.cfg.Client.GetIssue(ctx, in.IssueID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("get issue failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"issue": issue,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *linearPack) searchIssuesTool() tool.Tool {
	return tool.NewBuilder("linear_search_issues").
		WithDescription("Search for Linear issues").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query      string `json:"query"`
				TeamID     string `json:"team_id,omitempty"`
				AssigneeID string `json:"assignee_id,omitempty"`
				State      string `json:"state,omitempty"`
				MaxResults int    `json:"max_results,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Query == "" {
				return tool.Result{}, fmt.Errorf("query is required")
			}
			if in.MaxResults == 0 {
				in.MaxResults = 20
			}

			issues, err := p.cfg.Client.SearchIssues(ctx, in.Query, SearchOptions{
				TeamID:     in.TeamID,
				AssigneeID: in.AssigneeID,
				State:      in.State,
				MaxResults: in.MaxResults,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("search issues failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(issues),
				"issues": issues,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *linearPack) addCommentTool() tool.Tool {
	return tool.NewBuilder("linear_add_comment").
		WithDescription("Add a comment to a Linear issue").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				IssueID string `json:"issue_id"`
				Body    string `json:"body"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.IssueID == "" {
				return tool.Result{}, fmt.Errorf("issue_id is required")
			}
			if in.Body == "" {
				return tool.Result{}, fmt.Errorf("body is required")
			}

			comment, err := p.cfg.Client.AddComment(ctx, in.IssueID, in.Body)
			if err != nil {
				return tool.Result{}, fmt.Errorf("add comment failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"comment": comment,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *linearPack) getCycleTool() tool.Tool {
	return tool.NewBuilder("linear_get_cycle").
		WithDescription("Get the current or specified development cycle").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				TeamID      string `json:"team_id,omitempty"`
				CycleNumber int    `json:"cycle_number,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			teamID := in.TeamID
			if teamID == "" {
				teamID = p.cfg.DefaultTeamID
			}
			if teamID == "" {
				return tool.Result{}, fmt.Errorf("team_id is required")
			}

			cycle, err := p.cfg.Client.GetCycle(ctx, teamID, in.CycleNumber)
			if err != nil {
				return tool.Result{}, fmt.Errorf("get cycle failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"cycle": cycle,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *linearPack) listTeamsTool() tool.Tool {
	return tool.NewBuilder("linear_list_teams").
		WithDescription("List available Linear teams").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			teams, err := p.cfg.Client.ListTeams(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("list teams failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count": len(teams),
				"teams": teams,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
