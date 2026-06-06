// Package jira provides Jira integration tools for agent-go.
//
// Tools include issue management, project tracking, sprints, and search.
package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	jira "github.com/felixgeelhaar/jirasdk"
	"github.com/felixgeelhaar/jirasdk/core/agile"
	"github.com/felixgeelhaar/jirasdk/core/issue"
	"github.com/felixgeelhaar/jirasdk/core/search"
	"github.com/felixgeelhaar/jirasdk/core/user"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Config holds Jira connection settings.
type Config struct {
	BaseURL  string
	Username string
	APIToken string
	Timeout  time.Duration
}

// Pack returns the Jira tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &jiraPack{cfg: cfg}

	return pack.NewBuilder("jira").
		WithDescription("Jira integration tools for issue tracking, projects, and sprints").
		WithVersion("2.0.0").
		AddTools(
			p.getIssueTool(),
			p.createIssueTool(),
			p.updateIssueTool(),
			p.transitionIssueTool(),
			p.addCommentTool(),
			p.getCommentsTool(),
			p.assignIssueTool(),
			p.linkIssuesTool(),
			p.getProjectTool(),
			p.listProjectsTool(),
			p.searchIssuesTool(),
			p.getSprintTool(),
			p.listSprintsTool(),
			p.getSprintIssuesTool(),
			p.moveToSprintTool(),
			p.getUserTool(),
			p.searchUsersTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type jiraPack struct {
	cfg    Config
	client *jira.Client
}

func (p *jiraPack) getClient() (*jira.Client, error) {
	if p.client != nil {
		return p.client, nil
	}

	timeout := p.cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	client, err := jira.NewClient(
		jira.WithBaseURL(p.cfg.BaseURL),
		jira.WithAPIToken(p.cfg.Username, p.cfg.APIToken),
		jira.WithTimeout(timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira client: %w", err)
	}

	p.client = client
	return p.client, nil
}

// ============================================================================
// Issue Tools
// ============================================================================

func (p *jiraPack) getIssueTool() tool.Tool {
	return tool.NewBuilder("jira_get_issue").
		WithDescription("Get a Jira issue by key").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key    string   `json:"key"`
				Fields []string `json:"fields,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			var opts *issue.GetOptions
			if len(in.Fields) > 0 {
				opts = &issue.GetOptions{Fields: in.Fields}
			}

			iss, err := client.Issue.Get(ctx, in.Key, opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get issue: %w", err)
			}

			output, _ := json.Marshal(formatIssue(iss))
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) createIssueTool() tool.Tool {
	return tool.NewBuilder("jira_create_issue").
		WithDescription("Create a new Jira issue").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Project     string   `json:"project"`
				IssueType   string   `json:"issue_type"`
				Summary     string   `json:"summary"`
				Description string   `json:"description,omitempty"`
				Priority    string   `json:"priority,omitempty"`
				Labels      []string `json:"labels,omitempty"`
				Components  []string `json:"components,omitempty"`
				Assignee    string   `json:"assignee,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			fields := &issue.IssueFields{
				Summary:     in.Summary,
				Description: in.Description,
				Project:     &issue.Project{Key: in.Project},
				IssueType:   &issue.IssueType{Name: in.IssueType},
			}

			if in.Priority != "" {
				fields.Priority = &issue.Priority{Name: in.Priority}
			}
			if len(in.Labels) > 0 {
				fields.Labels = in.Labels
			}
			if len(in.Components) > 0 {
				components := make([]*issue.Component, len(in.Components))
				for i, c := range in.Components {
					components[i] = &issue.Component{Name: c}
				}
				fields.Components = components
			}
			if in.Assignee != "" {
				fields.Assignee = &issue.User{AccountID: in.Assignee}
			}

			created, err := client.Issue.Create(ctx, &issue.CreateInput{
				Fields: fields,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create issue: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"key":  created.Key,
				"id":   created.ID,
				"self": created.Self,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) updateIssueTool() tool.Tool {
	return tool.NewBuilder("jira_update_issue").
		WithDescription("Update a Jira issue").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key         string   `json:"key"`
				Summary     string   `json:"summary,omitempty"`
				Description string   `json:"description,omitempty"`
				Priority    string   `json:"priority,omitempty"`
				Labels      []string `json:"labels,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			fields := make(map[string]interface{})

			if in.Summary != "" {
				fields["summary"] = in.Summary
			}
			if in.Description != "" {
				fields["description"] = in.Description
			}
			if in.Priority != "" {
				fields["priority"] = map[string]string{"name": in.Priority}
			}
			if len(in.Labels) > 0 {
				fields["labels"] = in.Labels
			}

			err = client.Issue.Update(ctx, in.Key, &issue.UpdateInput{
				Fields: fields,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to update issue: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"key":     in.Key,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) transitionIssueTool() tool.Tool {
	return tool.NewBuilder("jira_transition_issue").
		WithDescription("Transition a Jira issue to a new status").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key        string `json:"key"`
				Transition string `json:"transition"` // transition name or ID
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			// Get available transitions
			transitions, err := client.Workflow.GetTransitions(ctx, in.Key, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get transitions: %w", err)
			}

			// Find matching transition
			var transitionID string
			for _, t := range transitions {
				if t.Name == in.Transition || t.ID == in.Transition {
					transitionID = t.ID
					break
				}
			}

			if transitionID == "" {
				available := make([]string, len(transitions))
				for i, t := range transitions {
					available[i] = t.Name
				}
				return tool.Result{}, fmt.Errorf("transition not found, available: %v", available)
			}

			err = client.Issue.DoTransition(ctx, in.Key, &issue.TransitionInput{
				Transition: &issue.Transition{ID: transitionID},
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to transition: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"key":        in.Key,
				"transition": in.Transition,
				"success":    true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) addCommentTool() tool.Tool {
	return tool.NewBuilder("jira_add_comment").
		WithDescription("Add a comment to a Jira issue").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key  string `json:"key"`
				Body string `json:"body"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			comment, err := client.Issue.AddComment(ctx, in.Key, &issue.AddCommentInput{
				Body: in.Body,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to add comment: %w", err)
			}

			result := map[string]any{
				"id": comment.ID,
			}
			if comment.Created != nil {
				result["created"] = comment.Created
			}
			if comment.Author != nil {
				result["author"] = comment.Author.DisplayName
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) getCommentsTool() tool.Tool {
	return tool.NewBuilder("jira_get_comments").
		WithDescription("Get comments on a Jira issue").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			commentList, err := client.Issue.ListComments(ctx, in.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get comments: %w", err)
			}

			comments := make([]map[string]any, 0, len(commentList))
			for _, c := range commentList {
				entry := map[string]any{
					"id":   c.ID,
					"body": c.Body,
				}
				if c.Author != nil {
					entry["author"] = c.Author.DisplayName
				}
				if c.Created != nil {
					entry["created"] = c.Created
				}
				if c.Updated != nil {
					entry["updated"] = c.Updated
				}
				comments = append(comments, entry)
			}

			output, _ := json.Marshal(map[string]any{
				"count":    len(comments),
				"comments": comments,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) assignIssueTool() tool.Tool {
	return tool.NewBuilder("jira_assign_issue").
		WithDescription("Assign a Jira issue to a user").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key      string `json:"key"`
				Assignee string `json:"assignee"` // account ID or empty to unassign
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			// Use Update to change assignee
			fields := map[string]interface{}{
				"assignee": map[string]string{"accountId": in.Assignee},
			}
			if in.Assignee == "" {
				fields["assignee"] = nil
			}

			err = client.Issue.Update(ctx, in.Key, &issue.UpdateInput{
				Fields: fields,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to assign: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"key":      in.Key,
				"assignee": in.Assignee,
				"success":  true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) linkIssuesTool() tool.Tool {
	return tool.NewBuilder("jira_link_issues").
		WithDescription("Create a link between two Jira issues").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				InwardKey  string `json:"inward_key"`
				OutwardKey string `json:"outward_key"`
				LinkType   string `json:"link_type"` // e.g., "Blocks", "Relates"
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			err = client.Issue.CreateIssueLink(ctx, &issue.CreateIssueLinkInput{
				Type:         &issue.IssueLinkType{Name: in.LinkType},
				InwardIssue:  &issue.IssueRef{Key: in.InwardKey},
				OutwardIssue: &issue.IssueRef{Key: in.OutwardKey},
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to link issues: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Project Tools
// ============================================================================

func (p *jiraPack) getProjectTool() tool.Tool {
	return tool.NewBuilder("jira_get_project").
		WithDescription("Get a Jira project by key").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			proj, err := client.Project.Get(ctx, in.Key, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get project: %w", err)
			}

			issueTypes := make([]string, len(proj.IssueTypes))
			for i, it := range proj.IssueTypes {
				issueTypes[i] = it.Name
			}

			result := map[string]any{
				"key":         proj.Key,
				"name":        proj.Name,
				"description": proj.Description,
				"issue_types": issueTypes,
				"self":        proj.Self,
			}
			if proj.Lead != nil {
				result["lead"] = proj.Lead.DisplayName
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) listProjectsTool() tool.Tool {
	return tool.NewBuilder("jira_list_projects").
		WithDescription("List Jira projects").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			projects, err := client.Project.List(ctx, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list projects: %w", err)
			}

			result := make([]map[string]any, len(projects))
			for i, proj := range projects {
				result[i] = map[string]any{
					"key":  proj.Key,
					"name": proj.Name,
					"id":   proj.ID,
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count":    len(result),
				"projects": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Search Tools
// ============================================================================

func (p *jiraPack) searchIssuesTool() tool.Tool {
	return tool.NewBuilder("jira_search_issues").
		WithDescription("Search for Jira issues using JQL").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				JQL        string   `json:"jql"`
				MaxResults int      `json:"max_results,omitempty"`
				Fields     []string `json:"fields,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MaxResults == 0 {
				in.MaxResults = 50
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			opts := &search.SearchOptions{
				JQL:        in.JQL,
				MaxResults: in.MaxResults,
			}
			if len(in.Fields) > 0 {
				opts.Fields = in.Fields
			}

			searchResult, err := client.Search.Search(ctx, opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to search: %w", err)
			}

			result := make([]map[string]any, len(searchResult.Issues))
			for i, iss := range searchResult.Issues {
				result[i] = formatIssue(iss)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(result),
				"total":  searchResult.Total,
				"issues": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Sprint Tools
// ============================================================================

func (p *jiraPack) getSprintTool() tool.Tool {
	return tool.NewBuilder("jira_get_sprint").
		WithDescription("Get sprint issues by sprint name using JQL").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				SprintName string `json:"sprint_name"`
				MaxResults int    `json:"max_results,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MaxResults == 0 {
				in.MaxResults = 50
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			// Use JQL to find issues in the sprint
			jql := fmt.Sprintf("sprint = \"%s\"", in.SprintName)
			searchResult, err := client.Search.Search(ctx, &search.SearchOptions{
				JQL:        jql,
				MaxResults: in.MaxResults,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get sprint issues: %w", err)
			}

			result := make([]map[string]any, len(searchResult.Issues))
			for i, iss := range searchResult.Issues {
				result[i] = formatIssue(iss)
			}

			output, _ := json.Marshal(map[string]any{
				"sprint": in.SprintName,
				"count":  len(result),
				"issues": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) listSprintsTool() tool.Tool {
	return tool.NewBuilder("jira_list_sprints").
		WithDescription("List sprints for a board").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				BoardID int64 `json:"board_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			sprints, err := client.Agile.GetBoardSprints(ctx, in.BoardID, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list sprints: %w", err)
			}

			result := make([]map[string]any, len(sprints))
			for i, s := range sprints {
				result[i] = map[string]any{
					"id":         s.ID,
					"name":       s.Name,
					"state":      s.State,
					"start_date": s.StartDate,
					"end_date":   s.EndDate,
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count":   len(result),
				"sprints": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) getSprintIssuesTool() tool.Tool {
	return tool.NewBuilder("jira_get_sprint_issues").
		WithDescription("Get issues in an active sprint for a board").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				BoardID    int64 `json:"board_id"`
				MaxResults int   `json:"max_results,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MaxResults == 0 {
				in.MaxResults = 50
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			// Use JQL to find issues in active sprint for the board
			jql := "sprint in openSprints()"
			searchResult, err := client.Search.Search(ctx, &search.SearchOptions{
				JQL:        jql,
				MaxResults: in.MaxResults,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get sprint issues: %w", err)
			}

			result := make([]map[string]any, len(searchResult.Issues))
			for i, iss := range searchResult.Issues {
				result[i] = formatIssue(iss)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(result),
				"issues": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) moveToSprintTool() tool.Tool {
	return tool.NewBuilder("jira_move_to_sprint").
		WithDescription("Move issues to a sprint").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				SprintID int64    `json:"sprint_id"`
				Issues   []string `json:"issues"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			err = client.Agile.MoveIssuesToSprint(ctx, in.SprintID, &agile.MoveIssuesToSprintInput{
				Issues: in.Issues,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to move issues to sprint: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
				"count":   len(in.Issues),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// User Tools
// ============================================================================

func (p *jiraPack) getUserTool() tool.Tool {
	return tool.NewBuilder("jira_get_user").
		WithDescription("Get a Jira user by account ID").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				AccountID string `json:"account_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			u, err := client.User.Get(ctx, in.AccountID, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get user: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"account_id":   u.AccountID,
				"display_name": u.DisplayName,
				"email":        u.EmailAddress,
				"active":       u.Active,
				"timezone":     u.TimeZone,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) searchUsersTool() tool.Tool {
	return tool.NewBuilder("jira_search_users").
		WithDescription("Search for Jira users").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query      string `json:"query"`
				MaxResults int    `json:"max_results,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MaxResults == 0 {
				in.MaxResults = 50
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			users, err := client.User.Search(ctx, &user.SearchOptions{
				Query:      in.Query,
				MaxResults: in.MaxResults,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to search users: %w", err)
			}

			result := make([]map[string]any, len(users))
			for i, u := range users {
				result[i] = map[string]any{
					"account_id":   u.AccountID,
					"display_name": u.DisplayName,
					"email":        u.EmailAddress,
					"active":       u.Active,
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count": len(result),
				"users": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Helpers
// ============================================================================

func formatIssue(iss *issue.Issue) map[string]any {
	result := map[string]any{
		"key":  iss.Key,
		"id":   iss.ID,
		"self": iss.Self,
	}

	if iss.Fields != nil {
		result["summary"] = iss.Fields.Summary
		result["description"] = iss.Fields.Description

		if iss.Fields.Created != nil {
			result["created"] = iss.Fields.Created
		}
		if iss.Fields.Updated != nil {
			result["updated"] = iss.Fields.Updated
		}
		if iss.Fields.Status != nil {
			result["status"] = iss.Fields.Status.Name
		}
		if iss.Fields.Priority != nil {
			result["priority"] = iss.Fields.Priority.Name
		}
		if iss.Fields.IssueType != nil {
			result["issue_type"] = iss.Fields.IssueType.Name
		}
		if iss.Fields.Assignee != nil {
			result["assignee"] = iss.Fields.Assignee.DisplayName
		}
		if iss.Fields.Reporter != nil {
			result["reporter"] = iss.Fields.Reporter.DisplayName
		}
		if len(iss.Fields.Labels) > 0 {
			result["labels"] = iss.Fields.Labels
		}
		if iss.Fields.Project != nil {
			result["project"] = iss.Fields.Project.Key
		}
	}

	return result
}
