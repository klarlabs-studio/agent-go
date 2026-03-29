package agent

import (
	"encoding/json"
	"time"
)

// EvidenceType classifies the source of evidence.
type EvidenceType string

const (
	EvidenceToolResult EvidenceType = "tool_result" // Result from tool execution
	EvidenceHumanInput EvidenceType = "human_input" // Input from human
	EvidenceSystemNote EvidenceType = "system_note" // System-generated observation
)

// Evidence represents an observation or result accumulated during a run.
type Evidence struct {
	Type      EvidenceType    `json:"type"`
	Source    string          `json:"source"` // Tool name or "system"
	Content   json.RawMessage `json:"content"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewToolEvidence creates evidence from a tool result.
func NewToolEvidence(toolName string, content json.RawMessage) Evidence {
	return Evidence{
		Type:      EvidenceToolResult,
		Source:    toolName,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewHumanEvidence creates evidence from human input.
func NewHumanEvidence(content json.RawMessage) Evidence {
	return Evidence{
		Type:      EvidenceHumanInput,
		Source:    "human",
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewSystemEvidence creates system-generated evidence.
func NewSystemEvidence(note string) Evidence {
	content, _ := json.Marshal(map[string]string{"note": note})
	return Evidence{
		Type:      EvidenceSystemNote,
		Source:    "system",
		Content:   content,
		Timestamp: time.Now(),
	}
}
