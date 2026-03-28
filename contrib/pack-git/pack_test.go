package git_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	git "github.com/felixgeelhaar/agent-go/contrib/pack-git"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestRegister(t *testing.T) {
	p := git.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "git" {
		t.Errorf("expected pack name %q, got %q", "git", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := git.Pack()
	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
		if tt.Description() == "" {
			t.Errorf("tool %q has empty description", tt.Name())
		}
	}
}

func TestExpectedToolNames(t *testing.T) {
	p := git.Pack()
	expected := map[string]bool{
		"git_status":      true,
		"git_log":         true,
		"git_diff":        true,
		"git_blame":       true,
		"git_branch_list": true,
		"git_commit":      true,
		"git_add":         true,
		"git_checkout":    true,
		"git_show":        true,
		"git_stash":       true,
		"git_tag":         true,
	}
	for _, tt := range p.Tools {
		if !expected[tt.Name()] {
			t.Errorf("unexpected tool: %s", tt.Name())
		}
		delete(expected, tt.Name())
	}
	for name := range expected {
		t.Errorf("missing tool: %s", name)
	}
}

func TestDestructiveAnnotations(t *testing.T) {
	p := git.Pack()
	destructive := map[string]bool{
		"git_commit":   true,
		"git_checkout": true,
	}
	for _, tt := range p.Tools {
		ann := tt.Annotations()
		if destructive[tt.Name()] {
			if !ann.Destructive {
				t.Errorf("tool %q should be destructive", tt.Name())
			}
			if !ann.RequiresApproval {
				t.Errorf("tool %q should require approval", tt.Name())
			}
		}
	}
}

func TestReadOnlyAnnotations(t *testing.T) {
	p := git.Pack()
	readOnly := map[string]bool{
		"git_status":      true,
		"git_log":         true,
		"git_diff":        true,
		"git_blame":       true,
		"git_branch_list": true,
		"git_show":        true,
	}
	for _, tt := range p.Tools {
		ann := tt.Annotations()
		if readOnly[tt.Name()] && !ann.ReadOnly {
			t.Errorf("tool %q should be read-only", tt.Name())
		}
	}
}

// gitAvailable returns true if git CLI is installed.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// initTestRepo creates a temporary git repository with one commit and returns its path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.local"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial commit"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v failed: %v\n%s", c, err, out)
		}
	}
	return dir
}

func inputJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func findTool(t *testing.T, name string) tool.Tool {
	t.Helper()
	p := git.Pack()
	tt, ok := p.GetTool(name)
	if !ok {
		t.Fatalf("tool %q not found in pack", name)
	}
	return tt
}

func TestHandlerGitStatus(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)
	tt := findTool(t, "git_status")

	// Clean repo
	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]string{"repo_dir": dir}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out struct {
		Clean   bool `json:"clean"`
		Entries []struct {
			Path string `json:"path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Clean {
		t.Error("expected clean repo")
	}

	// Create untracked file
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err = tt.Execute(context.Background(), inputJSON(t, map[string]string{"repo_dir": dir}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Clean {
		t.Error("expected dirty repo")
	}
	if len(out.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out.Entries))
	}
	if out.Entries[0].Path != "new.txt" {
		t.Errorf("expected path new.txt, got %q", out.Entries[0].Path)
	}
}

func TestHandlerGitLog(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)
	tt := findTool(t, "git_log")

	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"limit":    5,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out struct {
		Commits []struct {
			Hash    string `json:"hash"`
			Subject string `json:"subject"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(out.Commits))
	}
	if out.Commits[0].Subject != "initial commit" {
		t.Errorf("expected subject %q, got %q", "initial commit", out.Commits[0].Subject)
	}
}

func TestHandlerGitLogCustomFormat(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)
	tt := findTool(t, "git_log")

	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"format":   "%H",
		"limit":    1,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out struct {
		Raw string `json:"raw"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Raw) != 40 {
		t.Errorf("expected 40-char hash, got %q", out.Raw)
	}
}

func TestHandlerGitDiff(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	// Create and stage a file
	fpath := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(fpath, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "a.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	tt := findTool(t, "git_diff")

	// Staged diff
	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"staged":   true,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out struct {
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Diff == "" {
		t.Error("expected non-empty staged diff")
	}
}

func TestHandlerGitAdd(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	tt := findTool(t, "git_add")
	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"paths":    []string{"b.txt"},
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out struct {
		Added []string `json:"added"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Added) != 1 || out.Added[0] != "b.txt" {
		t.Errorf("expected [b.txt], got %v", out.Added)
	}

	// Verify file is staged
	raw, err2 := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err2 != nil {
		t.Fatal(err2)
	}
	if string(raw[0]) != "A" {
		t.Errorf("expected file to be staged, got status: %q", string(raw))
	}
}

func TestHandlerGitAddAll(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "d.txt"), []byte("d"), 0644); err != nil {
		t.Fatal(err)
	}

	tt := findTool(t, "git_add")
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"all":      true,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestHandlerGitAddNoPathsOrAll(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_add")
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
	}))
	if err == nil {
		t.Error("expected error when neither paths nor all is set")
	}
}

func TestHandlerGitCommit(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	// Create and stage a file
	if err := os.WriteFile(filepath.Join(dir, "e.txt"), []byte("e"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "e.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	tt := findTool(t, "git_commit")
	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"message":  "add e.txt",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out struct {
		Hash    string `json:"hash"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if out.Message != "add e.txt" {
		t.Errorf("expected message %q, got %q", "add e.txt", out.Message)
	}
}

func TestHandlerGitCommitNoMessage(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_commit")
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
	}))
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestHandlerGitBlame(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	// Create a file with content and commit it
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("line one\nline two\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{
		{"git", "add", "f.txt"},
		{"git", "commit", "-m", "add f.txt"},
	} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", c, err, out)
		}
	}

	tt := findTool(t, "git_blame")
	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"path":     "f.txt",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out struct {
		Lines []struct {
			Hash    string `json:"hash"`
			Author  string `json:"author"`
			Content string `json:"content"`
			LineNo  int    `json:"line_no"`
		} `json:"lines"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(out.Lines))
	}
	if out.Lines[0].Content != "line one" {
		t.Errorf("expected first line content %q, got %q", "line one", out.Lines[0].Content)
	}
	if out.Lines[0].Author != "Test" {
		t.Errorf("expected author %q, got %q", "Test", out.Lines[0].Author)
	}
}

func TestHandlerGitBlameNoPath(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_blame")
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
	}))
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestHandlerGitBranchList(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_branch_list")
	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]string{"repo_dir": dir}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out struct {
		Branches []struct {
			Name    string `json:"name"`
			Current bool   `json:"current"`
		} `json:"branches"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Branches) == 0 {
		t.Fatal("expected at least one branch")
	}
	found := false
	for _, b := range out.Branches {
		if b.Current {
			found = true
		}
	}
	if !found {
		t.Error("expected one branch to be current")
	}
}

func TestHandlerGitCheckout(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_checkout")
	// Create a new branch
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"ref":      "test-branch",
		"create":   true,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Verify we are on the new branch
	out, _ := exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if branch := string(out[:len(out)-1]); branch != "test-branch" {
		t.Errorf("expected branch test-branch, got %q", branch)
	}
}

func TestHandlerGitCheckoutNoRefOrPaths(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_checkout")
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
	}))
	if err == nil {
		t.Error("expected error for missing ref and paths")
	}
}

func TestHandlerGitShow(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_show")
	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"rev":      "HEAD",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Output == "" {
		t.Error("expected non-empty show output")
	}
}

func TestHandlerGitStash(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	// Create a tracked file, commit, then modify
	fpath := filepath.Join(dir, "stash.txt")
	if err := os.WriteFile(fpath, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{
		{"git", "add", "stash.txt"},
		{"git", "commit", "-m", "add stash.txt"},
	} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", c, err, out)
		}
	}
	if err := os.WriteFile(fpath, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	tt := findTool(t, "git_stash")

	// Stash push
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "push",
		"message":  "test stash",
	}))
	if err != nil {
		t.Fatalf("stash push: %v", err)
	}

	// Verify file is back to original
	content, _ := os.ReadFile(fpath)
	if string(content) != "original" {
		t.Errorf("expected original content after stash, got %q", string(content))
	}

	// Stash list
	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "list",
	}))
	if err != nil {
		t.Fatalf("stash list: %v", err)
	}
	var out struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Summary == "" {
		t.Error("expected non-empty stash list")
	}

	// Stash pop
	_, err = tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "pop",
	}))
	if err != nil {
		t.Fatalf("stash pop: %v", err)
	}

	content, _ = os.ReadFile(fpath)
	if string(content) != "modified" {
		t.Errorf("expected modified content after pop, got %q", string(content))
	}
}

func TestHandlerGitStashInvalidAction(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_stash")
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "invalid",
	}))
	if err == nil {
		t.Error("expected error for invalid stash action")
	}
}

func TestHandlerGitTag(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_tag")

	// Create tag
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "create",
		"name":     "v1.0.0",
		"message":  "release 1.0.0",
	}))
	if err != nil {
		t.Fatalf("tag create: %v", err)
	}

	// List tags
	result, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "list",
	}))
	if err != nil {
		t.Fatalf("tag list: %v", err)
	}
	var out struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Tags) != 1 || out.Tags[0] != "v1.0.0" {
		t.Errorf("expected [v1.0.0], got %v", out.Tags)
	}

	// Delete tag
	_, err = tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "delete",
		"name":     "v1.0.0",
	}))
	if err != nil {
		t.Fatalf("tag delete: %v", err)
	}

	// List again - should be empty (use fresh variable to avoid stale data)
	result, err = tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "list",
	}))
	if err != nil {
		t.Fatalf("tag list after delete: %v", err)
	}
	var outAfterDelete struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(result.Output, &outAfterDelete); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(outAfterDelete.Tags) != 0 {
		t.Errorf("expected no tags, got %v", outAfterDelete.Tags)
	}
}

func TestHandlerGitTagInvalidAction(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_tag")
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "invalid",
	}))
	if err == nil {
		t.Error("expected error for invalid tag action")
	}
}

func TestHandlerGitTagCreateNoName(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	tt := findTool(t, "git_tag")
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]any{
		"repo_dir": dir,
		"action":   "create",
	}))
	if err == nil {
		t.Error("expected error for missing tag name")
	}
}

func TestHandlerNotARepo(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir() // Not a git repo

	tt := findTool(t, "git_status")
	_, err := tt.Execute(context.Background(), inputJSON(t, map[string]string{"repo_dir": dir}))
	if err == nil {
		t.Error("expected error for non-repo directory")
	}
}

func TestDefaultRepoDir(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	// This test just verifies that empty input defaults to "." and does not panic.
	// It may fail if the current directory is not a git repo, which is fine.
	tt := findTool(t, "git_status")
	_, _ = tt.Execute(context.Background(), inputJSON(t, map[string]string{}))
}
