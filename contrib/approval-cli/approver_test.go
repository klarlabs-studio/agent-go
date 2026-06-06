package approvalcli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.klarlabs.de/agent/domain/policy"
)

func TestNew(t *testing.T) {
	t.Run("defaults to stdin and stdout", func(t *testing.T) {
		approver := New(Config{})
		require.NotNil(t, approver)
		require.NotNil(t, approver.reader)
		require.NotNil(t, approver.writer)
		require.False(t, approver.nonInteractive)
	})

	t.Run("uses custom reader and writer", func(t *testing.T) {
		reader := &bytes.Buffer{}
		writer := &bytes.Buffer{}

		approver := New(Config{
			Reader: reader,
			Writer: writer,
		})

		require.Equal(t, reader, approver.reader)
		require.Equal(t, writer, approver.writer)
	})

	t.Run("non-interactive mode", func(t *testing.T) {
		approver := New(Config{NonInteractive: true})
		require.True(t, approver.nonInteractive)
	})
}

func TestApprove_NonInteractive(t *testing.T) {
	approver := New(Config{NonInteractive: true})

	req := policy.ApprovalRequest{
		RunID:     "run-1",
		ToolName:  "delete_file",
		Reason:    "destructive action",
		RiskLevel: "high",
		Input:     json.RawMessage(`{"path":"/tmp/test"}`),
		Timestamp: time.Now(),
	}

	resp, err := approver.Approve(context.Background(), req)
	require.NoError(t, err)
	require.False(t, resp.Approved)
	require.Equal(t, "cli-auto", resp.Approver)
	require.Equal(t, "auto-denied: non-interactive mode", resp.Reason)
}

func TestApprove_Interactive(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectApproved bool
		expectReason   string
	}{
		{
			name:           "approve with yes",
			input:          "yes\n",
			expectApproved: true,
			expectReason:   "approved by cli user",
		},
		{
			name:           "approve with y",
			input:          "y\n",
			expectApproved: true,
			expectReason:   "approved by cli user",
		},
		{
			name:           "approve with YES (case insensitive)",
			input:          "YES\n",
			expectApproved: true,
			expectReason:   "approved by cli user",
		},
		{
			name:           "deny with no",
			input:          "no\n",
			expectApproved: false,
			expectReason:   "denied by cli user",
		},
		{
			name:           "deny with n",
			input:          "n\n",
			expectApproved: false,
			expectReason:   "denied by cli user",
		},
		{
			name:           "deny with empty input",
			input:          "\n",
			expectApproved: false,
			expectReason:   "denied by cli user",
		},
		{
			name:           "deny with random text",
			input:          "maybe\n",
			expectApproved: false,
			expectReason:   "denied by cli user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &bytes.Buffer{}

			approver := New(Config{
				Reader: reader,
				Writer: writer,
			})

			req := policy.ApprovalRequest{
				RunID:     "run-test",
				ToolName:  "delete_file",
				Reason:    "test action",
				RiskLevel: "high",
				Input:     json.RawMessage(`{"path":"/tmp/test"}`),
				Timestamp: time.Now(),
			}

			resp, err := approver.Approve(context.Background(), req)
			require.NoError(t, err)
			require.Equal(t, tt.expectApproved, resp.Approved)
			require.Equal(t, "cli-user", resp.Approver)
			require.Equal(t, tt.expectReason, resp.Reason)
			require.False(t, resp.Timestamp.IsZero())

			// Verify prompt was printed
			output := writer.String()
			require.Contains(t, output, "Approval Required")
			require.Contains(t, output, "run-test")
			require.Contains(t, output, "delete_file")
			require.Contains(t, output, "high")
			require.Contains(t, output, "test action")
			require.Contains(t, output, "Approve? [yes/no]:")
		})
	}
}

func TestApprove_EOF(t *testing.T) {
	// Empty reader simulates EOF
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}

	approver := New(Config{
		Reader: reader,
		Writer: writer,
	})

	req := policy.ApprovalRequest{
		RunID:    "run-eof",
		ToolName: "delete_file",
	}

	resp, err := approver.Approve(context.Background(), req)
	require.NoError(t, err)
	require.False(t, resp.Approved)
	require.Equal(t, "denied by cli user", resp.Reason)
}

func TestApprove_ContextCancellation(t *testing.T) {
	// Use a pipe reader that blocks forever (never receives data)
	reader, _ := io.Pipe()
	writer := &bytes.Buffer{}

	approver := New(Config{
		Reader: reader,
		Writer: writer,
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	req := policy.ApprovalRequest{
		RunID:    "run-cancel",
		ToolName: "delete_file",
	}

	_, err := approver.Approve(ctx, req)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestIsAffirmative(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"yes", true},
		{"y", true},
		{"YES", true},
		{"Y", true},
		{"Yes", true},
		{"no", false},
		{"n", false},
		{"", false},
		{"maybe", false},
		{"yep", false},
		{"nah", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.expected, isAffirmative(tt.input))
		})
	}
}
