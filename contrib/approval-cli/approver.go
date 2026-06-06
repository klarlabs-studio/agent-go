// Package approvalcli provides interactive CLI-based approval for agent-go.
//
// This package implements the policy.Approver interface to enable human approval
// of agent actions via the command line. When an agent requests approval for a
// destructive or high-risk action, the user is prompted to approve or deny.
//
// # Usage
//
//	approver := approvalcli.New(approvalcli.Config{})
//
//	engine, err := api.New(
//		api.WithApprover(approver),
//	)
//
// # Non-Interactive Mode
//
// When running in environments without a terminal (CI/CD, background processes),
// set NonInteractive to true. All requests will be automatically denied with a
// reason indicating non-interactive mode.
//
//	approver := approvalcli.New(approvalcli.Config{
//		NonInteractive: true,
//	})
package approvalcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/policy"
)

// Config configures the CLI approver.
type Config struct {
	// Reader is the input source for reading user responses.
	// Defaults to os.Stdin.
	Reader io.Reader

	// Writer is the output destination for printing prompts.
	// Defaults to os.Stdout.
	Writer io.Writer

	// NonInteractive when true, automatically denies all requests
	// with a reason indicating non-interactive mode.
	NonInteractive bool
}

// Approver implements policy.Approver via interactive CLI prompts.
type Approver struct {
	reader         io.Reader
	writer         io.Writer
	nonInteractive bool
}

// New creates a new CLI approver.
func New(cfg Config) *Approver {
	reader := cfg.Reader
	if reader == nil {
		reader = os.Stdin
	}

	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}

	return &Approver{
		reader:         reader,
		writer:         writer,
		nonInteractive: cfg.NonInteractive,
	}
}

// Approve prompts the user for approval via the CLI.
// This implements the policy.Approver interface.
func (a *Approver) Approve(ctx context.Context, req policy.ApprovalRequest) (policy.ApprovalResponse, error) {
	// Non-interactive mode: auto-deny
	if a.nonInteractive {
		return policy.ApprovalResponse{
			Approved:  false,
			Approver:  "cli-auto",
			Reason:    "auto-denied: non-interactive mode",
			Timestamp: time.Now(),
		}, nil
	}

	// Print the approval prompt
	_, _ = fmt.Fprintf(a.writer, "\n--- Approval Required ---\n")
	_, _ = fmt.Fprintf(a.writer, "Run ID:     %s\n", req.RunID)
	_, _ = fmt.Fprintf(a.writer, "Tool:       %s\n", req.ToolName)
	_, _ = fmt.Fprintf(a.writer, "Risk Level: %s\n", req.RiskLevel)
	_, _ = fmt.Fprintf(a.writer, "Reason:     %s\n", req.Reason)
	if len(req.Input) > 0 {
		_, _ = fmt.Fprintf(a.writer, "Input:      %s\n", string(req.Input))
	}
	_, _ = fmt.Fprintf(a.writer, "Approve? [yes/no]: ")

	// Read user input with context cancellation support
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(a.reader)
		if scanner.Scan() {
			resultCh <- strings.TrimSpace(scanner.Text())
		} else {
			if err := scanner.Err(); err != nil {
				errCh <- err
			} else {
				// EOF - treat as denial
				resultCh <- ""
			}
		}
	}()

	select {
	case <-ctx.Done():
		return policy.ApprovalResponse{}, ctx.Err()
	case err := <-errCh:
		return policy.ApprovalResponse{}, fmt.Errorf("reading input: %w", err)
	case input := <-resultCh:
		approved := isAffirmative(input)
		reason := "approved by cli user"
		if !approved {
			reason = "denied by cli user"
		}

		return policy.ApprovalResponse{
			Approved:  approved,
			Approver:  "cli-user",
			Reason:    reason,
			Timestamp: time.Now(),
		}, nil
	}
}

// isAffirmative checks if the input string indicates approval.
func isAffirmative(input string) bool {
	lower := strings.ToLower(input)
	switch lower {
	case "yes", "y":
		return true
	default:
		return false
	}
}

// Ensure Approver implements policy.Approver.
var _ policy.Approver = (*Approver)(nil)
