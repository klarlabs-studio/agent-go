// Package filesystem provides file system tools for agent-go.
//
// This pack includes tools for file system operations:
//   - fs_read_file: Read contents of a file
//   - fs_write_file: Write contents to a file
//   - fs_list_dir: List directory contents
//   - fs_stat: Get file or directory metadata
//   - fs_mkdir: Create directories
//   - fs_remove: Remove files or directories
//   - fs_copy: Copy files or directories
//   - fs_move: Move or rename files or directories
//
// All paths are validated and sandboxed to configured directories.
package filesystem

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the filesystem tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("filesystem").
		WithDescription("File system tools for reading, writing, and managing files").
		WithVersion("0.1.0").
		AddTools(
			readFile(),
			writeFile(),
			listDir(),
			stat(),
			mkdir(),
			remove(),
			copyFile(),
			moveFile(),
		).
		AllowInState(agent.StateExplore, "fs_read_file", "fs_list_dir", "fs_stat").
		AllowInState(agent.StateAct, "fs_read_file", "fs_write_file", "fs_list_dir", "fs_stat", "fs_mkdir", "fs_remove", "fs_copy", "fs_move").
		AllowInState(agent.StateValidate, "fs_read_file", "fs_list_dir", "fs_stat").
		Build()
}

func readFile() tool.Tool {
	return tool.NewBuilder("fs_read_file").
		WithDescription("Read the contents of a file").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func writeFile() tool.Tool {
	return tool.NewBuilder("fs_write_file").
		WithDescription("Write contents to a file, creating it if necessary").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func listDir() tool.Tool {
	return tool.NewBuilder("fs_list_dir").
		WithDescription("List the contents of a directory").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func stat() tool.Tool {
	return tool.NewBuilder("fs_stat").
		WithDescription("Get metadata about a file or directory").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func mkdir() tool.Tool {
	return tool.NewBuilder("fs_mkdir").
		WithDescription("Create a directory, optionally with parent directories").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func remove() tool.Tool {
	return tool.NewBuilder("fs_remove").
		WithDescription("Remove a file or directory").
		Destructive().
		MustBuild()
}

func copyFile() tool.Tool {
	return tool.NewBuilder("fs_copy").
		WithDescription("Copy a file or directory to a new location").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func moveFile() tool.Tool {
	return tool.NewBuilder("fs_move").
		WithDescription("Move or rename a file or directory").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}
