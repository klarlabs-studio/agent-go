// Package git provides Git operation tools for agent-go.
//
// This pack includes tools for Git version control:
//   - git_status: Get repository status
//   - git_log: View commit history
//   - git_diff: Show changes between commits or working tree
//   - git_blame: Annotate file lines with commit info
//   - git_branch_list: List branches
//   - git_commit: Create a new commit (destructive)
//   - git_add: Stage files for commit
//   - git_checkout: Switch branches or restore files (destructive)
//   - git_show: Show commit details
//   - git_stash: Stash operations
//   - git_tag: Tag operations
//
// All tools shell out to the git CLI and support a repo_dir parameter
// to set the working directory for the command.
package git

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the Git tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("git").
		WithDescription("Git version control tools").
		WithVersion("0.1.0").
		AddTools(
			gitStatus(),
			gitLog(),
			gitDiff(),
			gitBlame(),
			gitBranchList(),
			gitCommit(),
			gitAdd(),
			gitCheckout(),
			gitShow(),
			gitStash(),
			gitTag(),
		).
		AllowInState(agent.StateExplore,
			"git_status", "git_log", "git_diff", "git_blame",
			"git_branch_list", "git_show",
		).
		AllowInState(agent.StateAct,
			"git_status", "git_log", "git_diff", "git_blame",
			"git_branch_list", "git_commit", "git_add",
			"git_checkout", "git_show", "git_stash", "git_tag",
		).
		AllowInState(agent.StateValidate,
			"git_status", "git_log", "git_diff", "git_show",
		).
		Build()
}

func gitStatus() tool.Tool {
	return tool.NewBuilder("git_status").
		WithDescription("Get the status of the working tree").
		ReadOnly().
		WithHandler(handleGitStatus).
		MustBuild()
}

func gitLog() tool.Tool {
	return tool.NewBuilder("git_log").
		WithDescription("View commit history").
		ReadOnly().
		Cacheable().
		WithHandler(handleGitLog).
		MustBuild()
}

func gitDiff() tool.Tool {
	return tool.NewBuilder("git_diff").
		WithDescription("Show changes between commits, branches, or working tree").
		ReadOnly().
		WithHandler(handleGitDiff).
		MustBuild()
}

func gitBlame() tool.Tool {
	return tool.NewBuilder("git_blame").
		WithDescription("Show what revision and author last modified each line of a file").
		ReadOnly().
		Cacheable().
		WithHandler(handleGitBlame).
		MustBuild()
}

func gitBranchList() tool.Tool {
	return tool.NewBuilder("git_branch_list").
		WithDescription("List local and remote branches").
		ReadOnly().
		WithHandler(handleGitBranchList).
		MustBuild()
}

func gitCommit() tool.Tool {
	return tool.NewBuilder("git_commit").
		WithDescription("Create a new commit with staged changes").
		Destructive().
		WithHandler(handleGitCommit).
		MustBuild()
}

func gitAdd() tool.Tool {
	return tool.NewBuilder("git_add").
		WithDescription("Stage files for the next commit").
		WithRiskLevel(tool.RiskLow).
		WithHandler(handleGitAdd).
		MustBuild()
}

func gitCheckout() tool.Tool {
	return tool.NewBuilder("git_checkout").
		WithDescription("Switch branches or restore working tree files").
		Destructive().
		WithHandler(handleGitCheckout).
		MustBuild()
}

func gitShow() tool.Tool {
	return tool.NewBuilder("git_show").
		WithDescription("Show details of a commit").
		ReadOnly().
		Cacheable().
		WithHandler(handleGitShow).
		MustBuild()
}

func gitStash() tool.Tool {
	return tool.NewBuilder("git_stash").
		WithDescription("Stash working directory changes").
		WithRiskLevel(tool.RiskMedium).
		WithHandler(handleGitStash).
		MustBuild()
}

func gitTag() tool.Tool {
	return tool.NewBuilder("git_tag").
		WithDescription("Create, list, or delete tags").
		WithRiskLevel(tool.RiskMedium).
		WithHandler(handleGitTag).
		MustBuild()
}
