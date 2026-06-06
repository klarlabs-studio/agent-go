// Package shell provides shell command execution tools for agent-go.
//
// This pack includes tools for executing shell commands:
//   - shell_exec: Execute a shell command and return output
//   - shell_exec_background: Execute a command in the background
//   - shell_script: Execute a shell script
//
// Commands can be allowlisted/denylisted for security.
// Supports timeout configuration and working directory specification.
package shell

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the shell tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("shell").
		WithDescription("Shell command execution tools").
		WithVersion("0.1.0").
		AddTools(
			shellExec(),
			shellExecBackground(),
			shellScript(),
		).
		AllowInState(agent.StateAct, "shell_exec", "shell_exec_background", "shell_script").
		Build()
}

func shellExec() tool.Tool {
	return tool.NewBuilder("shell_exec").
		WithDescription("Execute a shell command and return stdout/stderr").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func shellExecBackground() tool.Tool {
	return tool.NewBuilder("shell_exec_background").
		WithDescription("Execute a shell command in the background").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func shellScript() tool.Tool {
	return tool.NewBuilder("shell_script").
		WithDescription("Execute a shell script from a string").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}
