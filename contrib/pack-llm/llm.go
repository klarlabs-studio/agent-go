// Package llm provides LLM completion tools for agent-go.
//
// This pack includes tools for LLM operations:
//   - llm_complete: Generate text completion
//   - llm_chat: Have a multi-turn conversation
//   - llm_embed: Generate embeddings for text
//   - llm_summarize: Summarize text content
//   - llm_extract: Extract structured data from text
//   - llm_classify: Classify text into categories
//   - llm_translate: Translate text between languages
//
// Supports OpenAI, Anthropic, Google, Azure OpenAI, and local models via Ollama.
package llm

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the LLM tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("llm").
		WithDescription("LLM completion and text processing tools").
		WithVersion("0.1.0").
		AddTools(
			llmComplete(),
			llmChat(),
			llmEmbed(),
			llmSummarize(),
			llmExtract(),
			llmClassify(),
			llmTranslate(),
		).
		AllowInState(agent.StateExplore, "llm_complete", "llm_chat", "llm_embed", "llm_summarize", "llm_extract", "llm_classify", "llm_translate").
		AllowInState(agent.StateAct, "llm_complete", "llm_chat", "llm_embed", "llm_summarize", "llm_extract", "llm_classify", "llm_translate").
		AllowInState(agent.StateDecide, "llm_complete", "llm_chat").
		Build()
}

func llmComplete() tool.Tool {
	return tool.NewBuilder("llm_complete").
		WithDescription("Generate text completion from a prompt").
		ReadOnly().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func llmChat() tool.Tool {
	return tool.NewBuilder("llm_chat").
		WithDescription("Have a multi-turn conversation with an LLM").
		ReadOnly().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func llmEmbed() tool.Tool {
	return tool.NewBuilder("llm_embed").
		WithDescription("Generate vector embeddings for text").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func llmSummarize() tool.Tool {
	return tool.NewBuilder("llm_summarize").
		WithDescription("Summarize text content").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func llmExtract() tool.Tool {
	return tool.NewBuilder("llm_extract").
		WithDescription("Extract structured data from unstructured text").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func llmClassify() tool.Tool {
	return tool.NewBuilder("llm_classify").
		WithDescription("Classify text into predefined categories").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func llmTranslate() tool.Tool {
	return tool.NewBuilder("llm_translate").
		WithDescription("Translate text between languages").
		ReadOnly().
		Cacheable().
		MustBuild()
}
