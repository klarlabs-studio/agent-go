// Package github provides GitHub integration tools for agent-go.
//
// Tools include repository management, pull requests, issues, actions,
// and code search capabilities.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/go-github/v68/github"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
	"golang.org/x/oauth2"
)

// Pack returns the GitHub tool pack.
func Pack(token string) *pack.Pack {
	p := &githubPack{token: token}

	return pack.NewBuilder("github").
		WithDescription("GitHub integration tools for repository management, PRs, issues, and actions").
		WithVersion("1.0.0").
		AddTools(
			p.getRepoTool(),
			p.listReposTool(),
			p.createIssueTool(),
			p.getIssueTool(),
			p.listIssuesTool(),
			p.updateIssueTool(),
			p.createPRTool(),
			p.getPRTool(),
			p.listPRsTool(),
			p.mergePRTool(),
			p.createReviewTool(),
			p.listWorkflowsTool(),
			p.triggerWorkflowTool(),
			p.getWorkflowRunTool(),
			p.searchCodeTool(),
			p.searchIssuesTool(),
			p.getFileTool(),
			p.createFileTool(),
			p.updateFileTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type githubPack struct {
	token  string
	client *github.Client
}

func (p *githubPack) getClient(ctx context.Context) *github.Client {
	if p.client != nil {
		return p.client
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: p.token})
	tc := oauth2.NewClient(ctx, ts)
	p.client = github.NewClient(tc)
	return p.client
}

// parseRepo splits "owner/repo" into owner and repo parts.
func parseRepo(fullName string) (owner, repo string, err error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo format, expected 'owner/repo': %s", fullName)
	}
	return parts[0], parts[1], nil
}

// ============================================================================
// Repository Tools
// ============================================================================

func (p *githubPack) getRepoTool() tool.Tool {
	return tool.NewBuilder("github_get_repo").
		WithDescription("Get repository information").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo string `json:"repo"` // owner/repo
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			client := p.getClient(ctx)
			r, _, err := client.Repositories.Get(ctx, owner, repo)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get repo: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"name":           r.GetName(),
				"full_name":      r.GetFullName(),
				"description":    r.GetDescription(),
				"private":        r.GetPrivate(),
				"default_branch": r.GetDefaultBranch(),
				"stars":          r.GetStargazersCount(),
				"forks":          r.GetForksCount(),
				"open_issues":    r.GetOpenIssuesCount(),
				"language":       r.GetLanguage(),
				"url":            r.GetHTMLURL(),
				"created_at":     r.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
				"updated_at":     r.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) listReposTool() tool.Tool {
	return tool.NewBuilder("github_list_repos").
		WithDescription("List repositories for a user or organization").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Owner   string `json:"owner"`
				Type    string `json:"type"`     // user, org, or empty for authenticated user
				PerPage int    `json:"per_page"` // default 30, max 100
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.PerPage == 0 {
				in.PerPage = 30
			}

			client := p.getClient(ctx)
			opts := &github.RepositoryListOptions{
				ListOptions: github.ListOptions{PerPage: in.PerPage},
			}

			var repos []*github.Repository
			var err error

			if in.Owner == "" {
				repos, _, err = client.Repositories.List(ctx, "", opts)
			} else if in.Type == "org" {
				repos, _, err = client.Repositories.ListByOrg(ctx, in.Owner, &github.RepositoryListByOrgOptions{
					ListOptions: opts.ListOptions,
				})
			} else {
				repos, _, err = client.Repositories.List(ctx, in.Owner, opts)
			}

			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list repos: %w", err)
			}

			result := make([]map[string]any, len(repos))
			for i, r := range repos {
				result[i] = map[string]any{
					"name":        r.GetName(),
					"full_name":   r.GetFullName(),
					"description": r.GetDescription(),
					"private":     r.GetPrivate(),
					"url":         r.GetHTMLURL(),
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count": len(result),
				"repos": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Issue Tools
// ============================================================================

func (p *githubPack) createIssueTool() tool.Tool {
	return tool.NewBuilder("github_create_issue").
		WithDescription("Create a new issue in a repository").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo      string   `json:"repo"` // owner/repo
				Title     string   `json:"title"`
				Body      string   `json:"body"`
				Labels    []string `json:"labels"`
				Assignees []string `json:"assignees"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			client := p.getClient(ctx)
			issue, _, err := client.Issues.Create(ctx, owner, repo, &github.IssueRequest{
				Title:     &in.Title,
				Body:      &in.Body,
				Labels:    &in.Labels,
				Assignees: &in.Assignees,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create issue: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"number":     issue.GetNumber(),
				"title":      issue.GetTitle(),
				"url":        issue.GetHTMLURL(),
				"state":      issue.GetState(),
				"created_at": issue.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) getIssueTool() tool.Tool {
	return tool.NewBuilder("github_get_issue").
		WithDescription("Get an issue by number").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo   string `json:"repo"` // owner/repo
				Number int    `json:"number"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			client := p.getClient(ctx)
			issue, _, err := client.Issues.Get(ctx, owner, repo, in.Number)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get issue: %w", err)
			}

			labels := make([]string, len(issue.Labels))
			for i, l := range issue.Labels {
				labels[i] = l.GetName()
			}

			output, _ := json.Marshal(map[string]any{
				"number":     issue.GetNumber(),
				"title":      issue.GetTitle(),
				"body":       issue.GetBody(),
				"state":      issue.GetState(),
				"labels":     labels,
				"user":       issue.GetUser().GetLogin(),
				"assignees":  getLogins(issue.Assignees),
				"url":        issue.GetHTMLURL(),
				"created_at": issue.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
				"updated_at": issue.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) listIssuesTool() tool.Tool {
	return tool.NewBuilder("github_list_issues").
		WithDescription("List issues in a repository").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo    string   `json:"repo"`  // owner/repo
				State   string   `json:"state"` // open, closed, all
				Labels  []string `json:"labels"`
				PerPage int      `json:"per_page"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			if in.State == "" {
				in.State = "open"
			}
			if in.PerPage == 0 {
				in.PerPage = 30
			}

			client := p.getClient(ctx)
			issues, _, err := client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
				State:       in.State,
				Labels:      in.Labels,
				ListOptions: github.ListOptions{PerPage: in.PerPage},
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list issues: %w", err)
			}

			result := make([]map[string]any, 0, len(issues))
			for _, issue := range issues {
				// Skip pull requests (they appear as issues too)
				if issue.PullRequestLinks != nil {
					continue
				}
				result = append(result, map[string]any{
					"number": issue.GetNumber(),
					"title":  issue.GetTitle(),
					"state":  issue.GetState(),
					"user":   issue.GetUser().GetLogin(),
					"url":    issue.GetHTMLURL(),
				})
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(result),
				"issues": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) updateIssueTool() tool.Tool {
	return tool.NewBuilder("github_update_issue").
		WithDescription("Update an issue").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo      string   `json:"repo"` // owner/repo
				Number    int      `json:"number"`
				Title     string   `json:"title,omitempty"`
				Body      string   `json:"body,omitempty"`
				State     string   `json:"state,omitempty"` // open, closed
				Labels    []string `json:"labels,omitempty"`
				Assignees []string `json:"assignees,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			req := &github.IssueRequest{}
			if in.Title != "" {
				req.Title = &in.Title
			}
			if in.Body != "" {
				req.Body = &in.Body
			}
			if in.State != "" {
				req.State = &in.State
			}
			if len(in.Labels) > 0 {
				req.Labels = &in.Labels
			}
			if len(in.Assignees) > 0 {
				req.Assignees = &in.Assignees
			}

			client := p.getClient(ctx)
			issue, _, err := client.Issues.Edit(ctx, owner, repo, in.Number, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to update issue: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"number":     issue.GetNumber(),
				"title":      issue.GetTitle(),
				"state":      issue.GetState(),
				"url":        issue.GetHTMLURL(),
				"updated_at": issue.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Pull Request Tools
// ============================================================================

func (p *githubPack) createPRTool() tool.Tool {
	return tool.NewBuilder("github_create_pr").
		WithDescription("Create a pull request").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo  string `json:"repo"` // owner/repo
				Title string `json:"title"`
				Body  string `json:"body"`
				Head  string `json:"head"` // branch name or user:branch
				Base  string `json:"base"` // target branch
				Draft bool   `json:"draft"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			client := p.getClient(ctx)
			pr, _, err := client.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
				Title: &in.Title,
				Body:  &in.Body,
				Head:  &in.Head,
				Base:  &in.Base,
				Draft: &in.Draft,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create PR: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"number":     pr.GetNumber(),
				"title":      pr.GetTitle(),
				"state":      pr.GetState(),
				"url":        pr.GetHTMLURL(),
				"draft":      pr.GetDraft(),
				"mergeable":  pr.GetMergeable(),
				"created_at": pr.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) getPRTool() tool.Tool {
	return tool.NewBuilder("github_get_pr").
		WithDescription("Get a pull request by number").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo   string `json:"repo"` // owner/repo
				Number int    `json:"number"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			client := p.getClient(ctx)
			pr, _, err := client.PullRequests.Get(ctx, owner, repo, in.Number)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get PR: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"number":        pr.GetNumber(),
				"title":         pr.GetTitle(),
				"body":          pr.GetBody(),
				"state":         pr.GetState(),
				"draft":         pr.GetDraft(),
				"merged":        pr.GetMerged(),
				"mergeable":     pr.GetMergeable(),
				"head":          pr.GetHead().GetRef(),
				"base":          pr.GetBase().GetRef(),
				"user":          pr.GetUser().GetLogin(),
				"url":           pr.GetHTMLURL(),
				"additions":     pr.GetAdditions(),
				"deletions":     pr.GetDeletions(),
				"changed_files": pr.GetChangedFiles(),
				"created_at":    pr.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
				"updated_at":    pr.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) listPRsTool() tool.Tool {
	return tool.NewBuilder("github_list_prs").
		WithDescription("List pull requests in a repository").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo    string `json:"repo"`  // owner/repo
				State   string `json:"state"` // open, closed, all
				PerPage int    `json:"per_page"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			if in.State == "" {
				in.State = "open"
			}
			if in.PerPage == 0 {
				in.PerPage = 30
			}

			client := p.getClient(ctx)
			prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
				State:       in.State,
				ListOptions: github.ListOptions{PerPage: in.PerPage},
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list PRs: %w", err)
			}

			result := make([]map[string]any, len(prs))
			for i, pr := range prs {
				result[i] = map[string]any{
					"number": pr.GetNumber(),
					"title":  pr.GetTitle(),
					"state":  pr.GetState(),
					"draft":  pr.GetDraft(),
					"user":   pr.GetUser().GetLogin(),
					"head":   pr.GetHead().GetRef(),
					"base":   pr.GetBase().GetRef(),
					"url":    pr.GetHTMLURL(),
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count": len(result),
				"prs":   result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) mergePRTool() tool.Tool {
	return tool.NewBuilder("github_merge_pr").
		WithDescription("Merge a pull request").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo          string `json:"repo"` // owner/repo
				Number        int    `json:"number"`
				CommitMessage string `json:"commit_message"`
				MergeMethod   string `json:"merge_method"` // merge, squash, rebase
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			if in.MergeMethod == "" {
				in.MergeMethod = "merge"
			}

			client := p.getClient(ctx)
			result, _, err := client.PullRequests.Merge(ctx, owner, repo, in.Number, in.CommitMessage, &github.PullRequestOptions{
				MergeMethod: in.MergeMethod,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to merge PR: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"merged":  result.GetMerged(),
				"sha":     result.GetSHA(),
				"message": result.GetMessage(),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) createReviewTool() tool.Tool {
	return tool.NewBuilder("github_create_review").
		WithDescription("Create a review on a pull request").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo   string `json:"repo"` // owner/repo
				Number int    `json:"number"`
				Body   string `json:"body"`
				Event  string `json:"event"` // APPROVE, REQUEST_CHANGES, COMMENT
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			if in.Event == "" {
				in.Event = "COMMENT"
			}

			client := p.getClient(ctx)
			review, _, err := client.PullRequests.CreateReview(ctx, owner, repo, in.Number, &github.PullRequestReviewRequest{
				Body:  &in.Body,
				Event: &in.Event,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create review: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":    review.GetID(),
				"state": review.GetState(),
				"body":  review.GetBody(),
				"url":   review.GetHTMLURL(),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Actions/Workflow Tools
// ============================================================================

func (p *githubPack) listWorkflowsTool() tool.Tool {
	return tool.NewBuilder("github_list_workflows").
		WithDescription("List workflows in a repository").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo string `json:"repo"` // owner/repo
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			client := p.getClient(ctx)
			workflows, _, err := client.Actions.ListWorkflows(ctx, owner, repo, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list workflows: %w", err)
			}

			result := make([]map[string]any, len(workflows.Workflows))
			for i, w := range workflows.Workflows {
				result[i] = map[string]any{
					"id":    w.GetID(),
					"name":  w.GetName(),
					"path":  w.GetPath(),
					"state": w.GetState(),
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count":     len(result),
				"workflows": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) triggerWorkflowTool() tool.Tool {
	return tool.NewBuilder("github_trigger_workflow").
		WithDescription("Trigger a workflow dispatch event").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo       string                 `json:"repo"`        // owner/repo
				WorkflowID string                 `json:"workflow_id"` // filename or ID
				Ref        string                 `json:"ref"`         // branch or tag
				Inputs     map[string]interface{} `json:"inputs"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			client := p.getClient(ctx)
			_, err = client.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, repo, in.WorkflowID, github.CreateWorkflowDispatchEventRequest{
				Ref:    in.Ref,
				Inputs: in.Inputs,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to trigger workflow: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"triggered": true,
				"workflow":  in.WorkflowID,
				"ref":       in.Ref,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) getWorkflowRunTool() tool.Tool {
	return tool.NewBuilder("github_get_workflow_run").
		WithDescription("Get a workflow run by ID").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo  string `json:"repo"` // owner/repo
				RunID int64  `json:"run_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			client := p.getClient(ctx)
			run, _, err := client.Actions.GetWorkflowRunByID(ctx, owner, repo, in.RunID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get workflow run: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":          run.GetID(),
				"name":        run.GetName(),
				"status":      run.GetStatus(),
				"conclusion":  run.GetConclusion(),
				"head_branch": run.GetHeadBranch(),
				"head_sha":    run.GetHeadSHA(),
				"url":         run.GetHTMLURL(),
				"created_at":  run.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
				"updated_at":  run.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Search Tools
// ============================================================================

func (p *githubPack) searchCodeTool() tool.Tool {
	return tool.NewBuilder("github_search_code").
		WithDescription("Search for code across GitHub").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query   string `json:"query"`
				PerPage int    `json:"per_page"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.PerPage == 0 {
				in.PerPage = 30
			}

			client := p.getClient(ctx)
			results, _, err := client.Search.Code(ctx, in.Query, &github.SearchOptions{
				ListOptions: github.ListOptions{PerPage: in.PerPage},
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to search code: %w", err)
			}

			items := make([]map[string]any, len(results.CodeResults))
			for i, r := range results.CodeResults {
				items[i] = map[string]any{
					"name":       r.GetName(),
					"path":       r.GetPath(),
					"sha":        r.GetSHA(),
					"url":        r.GetHTMLURL(),
					"repository": r.GetRepository().GetFullName(),
				}
			}

			output, _ := json.Marshal(map[string]any{
				"total_count": results.GetTotal(),
				"items":       items,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) searchIssuesTool() tool.Tool {
	return tool.NewBuilder("github_search_issues").
		WithDescription("Search for issues and pull requests").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query   string `json:"query"`
				PerPage int    `json:"per_page"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.PerPage == 0 {
				in.PerPage = 30
			}

			client := p.getClient(ctx)
			results, _, err := client.Search.Issues(ctx, in.Query, &github.SearchOptions{
				ListOptions: github.ListOptions{PerPage: in.PerPage},
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to search issues: %w", err)
			}

			items := make([]map[string]any, len(results.Issues))
			for i, issue := range results.Issues {
				items[i] = map[string]any{
					"number":     issue.GetNumber(),
					"title":      issue.GetTitle(),
					"state":      issue.GetState(),
					"user":       issue.GetUser().GetLogin(),
					"url":        issue.GetHTMLURL(),
					"repository": issue.GetRepositoryURL(),
					"is_pr":      issue.IsPullRequest(),
				}
			}

			output, _ := json.Marshal(map[string]any{
				"total_count": results.GetTotal(),
				"items":       items,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// File Content Tools
// ============================================================================

func (p *githubPack) getFileTool() tool.Tool {
	return tool.NewBuilder("github_get_file").
		WithDescription("Get file contents from a repository").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo string `json:"repo"` // owner/repo
				Path string `json:"path"`
				Ref  string `json:"ref"` // branch, tag, or commit SHA
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			opts := &github.RepositoryContentGetOptions{}
			if in.Ref != "" {
				opts.Ref = in.Ref
			}

			client := p.getClient(ctx)
			content, _, _, err := client.Repositories.GetContents(ctx, owner, repo, in.Path, opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get file: %w", err)
			}

			if content == nil {
				return tool.Result{}, fmt.Errorf("path is a directory, not a file")
			}

			decoded, err := content.GetContent()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to decode content: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"name":     content.GetName(),
				"path":     content.GetPath(),
				"sha":      content.GetSHA(),
				"size":     content.GetSize(),
				"content":  decoded,
				"encoding": content.GetEncoding(),
				"url":      content.GetHTMLURL(),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) createFileTool() tool.Tool {
	return tool.NewBuilder("github_create_file").
		WithDescription("Create a new file in a repository").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo    string `json:"repo"` // owner/repo
				Path    string `json:"path"`
				Content string `json:"content"`
				Message string `json:"message"`
				Branch  string `json:"branch"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			opts := &github.RepositoryContentFileOptions{
				Message: &in.Message,
				Content: []byte(in.Content),
			}
			if in.Branch != "" {
				opts.Branch = &in.Branch
			}

			client := p.getClient(ctx)
			result, _, err := client.Repositories.CreateFile(ctx, owner, repo, in.Path, opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create file: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"path":   result.Content.GetPath(),
				"sha":    result.Content.GetSHA(),
				"url":    result.Content.GetHTMLURL(),
				"commit": result.Commit.GetSHA(),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *githubPack) updateFileTool() tool.Tool {
	return tool.NewBuilder("github_update_file").
		WithDescription("Update an existing file in a repository").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Repo    string `json:"repo"` // owner/repo
				Path    string `json:"path"`
				Content string `json:"content"`
				Message string `json:"message"`
				SHA     string `json:"sha"` // current file SHA
				Branch  string `json:"branch"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			owner, repo, err := parseRepo(in.Repo)
			if err != nil {
				return tool.Result{}, err
			}

			opts := &github.RepositoryContentFileOptions{
				Message: &in.Message,
				Content: []byte(in.Content),
				SHA:     &in.SHA,
			}
			if in.Branch != "" {
				opts.Branch = &in.Branch
			}

			client := p.getClient(ctx)
			result, _, err := client.Repositories.UpdateFile(ctx, owner, repo, in.Path, opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to update file: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"path":   result.Content.GetPath(),
				"sha":    result.Content.GetSHA(),
				"url":    result.Content.GetHTMLURL(),
				"commit": result.Commit.GetSHA(),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Helpers
// ============================================================================

func getLogins(users []*github.User) []string {
	logins := make([]string, len(users))
	for i, u := range users {
		logins[i] = u.GetLogin()
	}
	return logins
}
