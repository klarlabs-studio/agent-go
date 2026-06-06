package git

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"go.klarlabs.de/agent/domain/tool"
)

// repoDir extracts the repo_dir field from input, defaulting to ".".
func repoDir(input json.RawMessage) string {
	var m struct {
		RepoDir string `json:"repo_dir"`
	}
	if err := json.Unmarshal(input, &m); err == nil && m.RepoDir != "" {
		return m.RepoDir
	}
	return "."
}

// runGit executes a git command in the given directory and returns stdout.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	output := strings.TrimRight(string(out), "\n")
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s failed (exit %d): %s",
				args[0], exitErr.ExitCode(), output)
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return output, nil
}

// jsonResult marshals v and returns a tool.Result.
func jsonResult(v any) (tool.Result, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return tool.Result{}, fmt.Errorf("marshal result: %w", err)
	}
	return tool.NewResult(data), nil
}

// --- git_status ---

type statusEntry struct {
	Index    string `json:"index"`
	WorkTree string `json:"worktree"`
	Path     string `json:"path"`
	OrigPath string `json:"orig_path,omitempty"`
}

type statusOutput struct {
	Clean   bool          `json:"clean"`
	Entries []statusEntry `json:"entries"`
}

func handleGitStatus(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	dir := repoDir(input)
	raw, err := runGit(ctx, dir, "status", "--porcelain")
	if err != nil {
		return tool.Result{}, err
	}

	var entries []statusEntry
	if raw != "" {
		for _, line := range strings.Split(raw, "\n") {
			if len(line) < 3 {
				continue
			}
			entry := statusEntry{
				Index:    string(line[0]),
				WorkTree: string(line[1]),
			}
			rest := line[3:]
			// Handle renames: "R  old -> new"
			if parts := strings.SplitN(rest, " -> ", 2); len(parts) == 2 {
				entry.OrigPath = parts[0]
				entry.Path = parts[1]
			} else {
				entry.Path = rest
			}
			entries = append(entries, entry)
		}
	}

	return jsonResult(statusOutput{
		Clean:   len(entries) == 0,
		Entries: entries,
	})
}

// --- git_log ---

type logInput struct {
	RepoDir string `json:"repo_dir"`
	Limit   int    `json:"limit"`
	Format  string `json:"format"`
	Path    string `json:"path"`
	Author  string `json:"author"`
	Since   string `json:"since"`
	Until   string `json:"until"`
}

type logEntry struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

type logOutput struct {
	Commits []logEntry `json:"commits"`
}

func handleGitLog(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in logInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parse input: %w", err)
	}
	if in.RepoDir == "" {
		in.RepoDir = "."
	}

	if in.Format != "" {
		// Free-form format: return raw output
		args := []string{"log", "--format=" + in.Format}
		if in.Limit > 0 {
			args = append(args, fmt.Sprintf("-n%d", in.Limit))
		}
		if in.Author != "" {
			args = append(args, "--author="+in.Author)
		}
		if in.Since != "" {
			args = append(args, "--since="+in.Since)
		}
		if in.Until != "" {
			args = append(args, "--until="+in.Until)
		}
		if in.Path != "" {
			args = append(args, "--", in.Path)
		}
		raw, err := runGit(ctx, in.RepoDir, args...)
		if err != nil {
			return tool.Result{}, err
		}
		return jsonResult(map[string]string{"raw": raw})
	}

	// Structured output using a delimiter
	const delim = "\x1f"
	format := strings.Join([]string{"%H", "%an", "%aI", "%s"}, delim)
	args := []string{"log", "--format=" + format}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, fmt.Sprintf("-n%d", limit))
	if in.Author != "" {
		args = append(args, "--author="+in.Author)
	}
	if in.Since != "" {
		args = append(args, "--since="+in.Since)
	}
	if in.Until != "" {
		args = append(args, "--until="+in.Until)
	}
	if in.Path != "" {
		args = append(args, "--", in.Path)
	}

	raw, err := runGit(ctx, in.RepoDir, args...)
	if err != nil {
		return tool.Result{}, err
	}

	var commits []logEntry
	if raw != "" {
		for _, line := range strings.Split(raw, "\n") {
			parts := strings.SplitN(line, delim, 4)
			if len(parts) != 4 {
				continue
			}
			commits = append(commits, logEntry{
				Hash:    parts[0],
				Author:  parts[1],
				Date:    parts[2],
				Subject: parts[3],
			})
		}
	}

	return jsonResult(logOutput{Commits: commits})
}

// --- git_diff ---

type diffInput struct {
	RepoDir string `json:"repo_dir"`
	Staged  bool   `json:"staged"`
	Path    string `json:"path"`
	Ref1    string `json:"ref1"`
	Ref2    string `json:"ref2"`
}

type diffOutput struct {
	Diff string `json:"diff"`
}

func handleGitDiff(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in diffInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parse input: %w", err)
	}
	if in.RepoDir == "" {
		in.RepoDir = "."
	}

	args := []string{"diff"}
	if in.Staged {
		args = append(args, "--cached")
	}
	if in.Ref1 != "" {
		args = append(args, in.Ref1)
	}
	if in.Ref2 != "" {
		args = append(args, in.Ref2)
	}
	if in.Path != "" {
		args = append(args, "--", in.Path)
	}

	raw, err := runGit(ctx, in.RepoDir, args...)
	if err != nil {
		return tool.Result{}, err
	}
	return jsonResult(diffOutput{Diff: raw})
}

// --- git_blame ---

type blameInput struct {
	RepoDir string `json:"repo_dir"`
	Path    string `json:"path"`
	Rev     string `json:"rev"`
}

type blameLine struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	LineNo  int    `json:"line_no"`
	Content string `json:"content"`
}

type blameOutput struct {
	Lines []blameLine `json:"lines"`
}

func handleGitBlame(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in blameInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parse input: %w", err)
	}
	if in.RepoDir == "" {
		in.RepoDir = "."
	}
	if in.Path == "" {
		return tool.Result{}, fmt.Errorf("path is required for git blame")
	}

	args := []string{"blame", "--porcelain"}
	if in.Rev != "" {
		args = append(args, in.Rev)
	}
	args = append(args, "--", in.Path)

	raw, err := runGit(ctx, in.RepoDir, args...)
	if err != nil {
		return tool.Result{}, err
	}

	lines := parseBlameOutput(raw)
	return jsonResult(blameOutput{Lines: lines})
}

// parseBlameOutput parses git blame --porcelain output.
func parseBlameOutput(raw string) []blameLine {
	var result []blameLine
	if raw == "" {
		return result
	}

	lines := strings.Split(raw, "\n")
	var current blameLine
	lineNo := 0

	for _, l := range lines {
		switch {
		case len(l) >= 40 && l[0] != '\t' && isHexPrefix(l[:40]):
			// Header line: hash orig-line final-line [num-lines]
			parts := strings.Fields(l)
			current = blameLine{Hash: parts[0]}
			if len(parts) >= 3 {
				_, _ = fmt.Sscanf(parts[2], "%d", &lineNo)
			}
			current.LineNo = lineNo
		case strings.HasPrefix(l, "author "):
			current.Author = strings.TrimPrefix(l, "author ")
		case strings.HasPrefix(l, "author-time "):
			current.Date = strings.TrimPrefix(l, "author-time ")
		case strings.HasPrefix(l, "\t"):
			current.Content = l[1:] // strip leading tab
			result = append(result, current)
		}
	}
	return result
}

func isHexPrefix(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// --- git_branch_list ---

type branchEntry struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
	Remote  bool   `json:"remote"`
}

type branchListOutput struct {
	Branches []branchEntry `json:"branches"`
}

func handleGitBranchList(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	dir := repoDir(input)

	raw, err := runGit(ctx, dir, "branch", "-a", "--no-color")
	if err != nil {
		return tool.Result{}, err
	}

	var branches []branchEntry
	if raw != "" {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			current := false
			if strings.HasPrefix(line, "* ") {
				current = true
				line = strings.TrimPrefix(line, "* ")
			}
			// Skip HEAD pointers like "remotes/origin/HEAD -> origin/main"
			if strings.Contains(line, " -> ") {
				continue
			}
			remote := strings.HasPrefix(line, "remotes/")
			branches = append(branches, branchEntry{
				Name:    line,
				Current: current,
				Remote:  remote,
			})
		}
	}

	return jsonResult(branchListOutput{Branches: branches})
}

// --- git_commit ---

type commitInput struct {
	RepoDir    string `json:"repo_dir"`
	Message    string `json:"message"`
	AllowEmpty bool   `json:"allow_empty"`
}

type commitOutput struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Summary string `json:"summary"`
}

func handleGitCommit(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in commitInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parse input: %w", err)
	}
	if in.RepoDir == "" {
		in.RepoDir = "."
	}
	if in.Message == "" {
		return tool.Result{}, fmt.Errorf("message is required for git commit")
	}

	args := []string{"commit", "-m", in.Message}
	if in.AllowEmpty {
		args = append(args, "--allow-empty")
	}

	summary, err := runGit(ctx, in.RepoDir, args...)
	if err != nil {
		return tool.Result{}, err
	}

	// Get the hash of the new commit
	hash, hashErr := runGit(ctx, in.RepoDir, "rev-parse", "HEAD")
	if hashErr != nil {
		hash = ""
	}

	return jsonResult(commitOutput{
		Hash:    strings.TrimSpace(hash),
		Message: in.Message,
		Summary: summary,
	})
}

// --- git_add ---

type addInput struct {
	RepoDir string   `json:"repo_dir"`
	Paths   []string `json:"paths"`
	All     bool     `json:"all"`
}

type addOutput struct {
	Added   []string `json:"added"`
	Summary string   `json:"summary"`
}

func handleGitAdd(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in addInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parse input: %w", err)
	}
	if in.RepoDir == "" {
		in.RepoDir = "."
	}

	args := []string{"add"}
	if in.All {
		args = append(args, "-A")
	} else if len(in.Paths) > 0 {
		args = append(args, "--")
		args = append(args, in.Paths...)
	} else {
		return tool.Result{}, fmt.Errorf("either paths or all is required for git add")
	}

	summary, err := runGit(ctx, in.RepoDir, args...)
	if err != nil {
		return tool.Result{}, err
	}

	added := in.Paths
	if in.All {
		added = []string{"--all"}
	}

	return jsonResult(addOutput{
		Added:   added,
		Summary: summary,
	})
}

// --- git_checkout ---

type checkoutInput struct {
	RepoDir string   `json:"repo_dir"`
	Ref     string   `json:"ref"`
	Paths   []string `json:"paths"`
	Create  bool     `json:"create"`
}

type checkoutOutput struct {
	Summary string `json:"summary"`
}

func handleGitCheckout(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in checkoutInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parse input: %w", err)
	}
	if in.RepoDir == "" {
		in.RepoDir = "."
	}
	if in.Ref == "" && len(in.Paths) == 0 {
		return tool.Result{}, fmt.Errorf("ref or paths is required for git checkout")
	}

	args := []string{"checkout"}
	if in.Create {
		args = append(args, "-b")
	}
	if in.Ref != "" {
		args = append(args, in.Ref)
	}
	if len(in.Paths) > 0 {
		args = append(args, "--")
		args = append(args, in.Paths...)
	}

	summary, err := runGit(ctx, in.RepoDir, args...)
	if err != nil {
		return tool.Result{}, err
	}

	return jsonResult(checkoutOutput{Summary: summary})
}

// --- git_show ---

type showInput struct {
	RepoDir string `json:"repo_dir"`
	Rev     string `json:"rev"`
	Format  string `json:"format"`
}

type showOutput struct {
	Output string `json:"output"`
}

func handleGitShow(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in showInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parse input: %w", err)
	}
	if in.RepoDir == "" {
		in.RepoDir = "."
	}

	rev := in.Rev
	if rev == "" {
		rev = "HEAD"
	}

	args := []string{"show"}
	if in.Format != "" {
		args = append(args, "--format="+in.Format)
	}
	args = append(args, rev)

	raw, err := runGit(ctx, in.RepoDir, args...)
	if err != nil {
		return tool.Result{}, err
	}

	return jsonResult(showOutput{Output: raw})
}

// --- git_stash ---

type stashInput struct {
	RepoDir string `json:"repo_dir"`
	Action  string `json:"action"` // push, pop, list, apply, drop
	Message string `json:"message"`
	Index   int    `json:"index"`
}

type stashOutput struct {
	Summary string `json:"summary"`
}

func handleGitStash(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in stashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parse input: %w", err)
	}
	if in.RepoDir == "" {
		in.RepoDir = "."
	}

	action := in.Action
	if action == "" {
		action = "push"
	}

	args := []string{"stash"}
	switch action {
	case "push":
		args = append(args, "push")
		if in.Message != "" {
			args = append(args, "-m", in.Message)
		}
	case "pop":
		args = append(args, "pop")
		if in.Index > 0 {
			args = append(args, fmt.Sprintf("stash@{%d}", in.Index))
		}
	case "apply":
		args = append(args, "apply")
		if in.Index > 0 {
			args = append(args, fmt.Sprintf("stash@{%d}", in.Index))
		}
	case "drop":
		args = append(args, "drop")
		if in.Index > 0 {
			args = append(args, fmt.Sprintf("stash@{%d}", in.Index))
		}
	case "list":
		args = append(args, "list")
	default:
		return tool.Result{}, fmt.Errorf("unknown stash action: %s (use push, pop, apply, drop, or list)", action)
	}

	raw, err := runGit(ctx, in.RepoDir, args...)
	if err != nil {
		return tool.Result{}, err
	}

	return jsonResult(stashOutput{Summary: raw})
}

// --- git_tag ---

type tagInput struct {
	RepoDir string `json:"repo_dir"`
	Action  string `json:"action"` // list, create, delete
	Name    string `json:"name"`
	Message string `json:"message"`
	Rev     string `json:"rev"`
}

type tagOutput struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags,omitempty"`
}

func handleGitTag(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in tagInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parse input: %w", err)
	}
	if in.RepoDir == "" {
		in.RepoDir = "."
	}

	action := in.Action
	if action == "" {
		action = "list"
	}

	switch action {
	case "list":
		raw, err := runGit(ctx, in.RepoDir, "tag", "--list")
		if err != nil {
			return tool.Result{}, err
		}
		var tags []string
		if raw != "" {
			tags = strings.Split(raw, "\n")
		}
		return jsonResult(tagOutput{Tags: tags})

	case "create":
		if in.Name == "" {
			return tool.Result{}, fmt.Errorf("name is required for tag create")
		}
		args := []string{"tag"}
		if in.Message != "" {
			args = append(args, "-a", in.Name, "-m", in.Message)
		} else {
			args = append(args, in.Name)
		}
		if in.Rev != "" {
			args = append(args, in.Rev)
		}
		summary, err := runGit(ctx, in.RepoDir, args...)
		if err != nil {
			return tool.Result{}, err
		}
		return jsonResult(tagOutput{Summary: summary})

	case "delete":
		if in.Name == "" {
			return tool.Result{}, fmt.Errorf("name is required for tag delete")
		}
		summary, err := runGit(ctx, in.RepoDir, "tag", "-d", in.Name)
		if err != nil {
			return tool.Result{}, err
		}
		return jsonResult(tagOutput{Summary: summary})

	default:
		return tool.Result{}, fmt.Errorf("unknown tag action: %s (use list, create, or delete)", action)
	}
}
