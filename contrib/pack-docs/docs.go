// Package docs provides documentation generation and management tools for agent-go.
//
// The pack uses an interface-based approach, allowing any documentation engine
// or static site generator to be plugged in.
package docs

import (
	"context"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// DocEngine provides documentation generation capabilities.
type DocEngine interface {
	// GenerateDocs generates documentation from source code.
	GenerateDocs(ctx context.Context, opts GenerateOptions) (*DocOutput, error)

	// CheckLinks verifies all links in documentation files.
	CheckLinks(ctx context.Context, paths []string) (*LinkReport, error)

	// SpellCheck checks documentation for spelling errors.
	SpellCheck(ctx context.Context, text string, language string) ([]SpellIssue, error)

	// ExtractAPIDocs extracts API documentation from source code annotations.
	ExtractAPIDocs(ctx context.Context, opts ExtractOptions) (*APIDoc, error)
}

// Translator translates documentation text.
type Translator interface {
	// Translate translates text to the target language.
	Translate(ctx context.Context, text, sourceLang, targetLang string) (string, error)
}

// GenerateOptions configures documentation generation.
type GenerateOptions struct {
	SourcePaths []string `json:"source_paths"`
	OutputDir   string   `json:"output_dir,omitempty"`
	Format      string   `json:"format,omitempty"` // "markdown", "html", "rst"
	Language    string   `json:"language,omitempty"`
	Template    string   `json:"template,omitempty"`
}

// DocOutput contains generated documentation.
type DocOutput struct {
	Files     []DocFile `json:"files"`
	Summary   string    `json:"summary,omitempty"`
	Generated int       `json:"generated"`
}

// DocFile represents a generated documentation file.
type DocFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Title   string `json:"title,omitempty"`
}

// LinkReport contains link check results.
type LinkReport struct {
	Total   int         `json:"total"`
	Valid   int         `json:"valid"`
	Broken  int         `json:"broken"`
	Skipped int         `json:"skipped"`
	Links   []LinkCheck `json:"links,omitempty"`
}

// LinkCheck represents a single link check result.
type LinkCheck struct {
	URL    string `json:"url"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Status string `json:"status"` // "valid", "broken", "timeout", "skipped"
	Code   int    `json:"code,omitempty"`
	Error  string `json:"error,omitempty"`
}

// SpellIssue represents a spelling issue.
type SpellIssue struct {
	Word        string   `json:"word"`
	Line        int      `json:"line,omitempty"`
	Column      int      `json:"column,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// ExtractOptions configures API documentation extraction.
type ExtractOptions struct {
	SourcePaths []string `json:"source_paths"`
	Language    string   `json:"language,omitempty"`
	Format      string   `json:"format,omitempty"` // "openapi", "markdown", "html"
}

// APIDoc contains extracted API documentation.
type APIDoc struct {
	Title     string     `json:"title"`
	Version   string     `json:"version,omitempty"`
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	Types     []TypeDoc  `json:"types,omitempty"`
	Content   string     `json:"content,omitempty"`
}

// Endpoint represents a documented API endpoint.
type Endpoint struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
	Request     string `json:"request,omitempty"`
	Response    string `json:"response,omitempty"`
}

// TypeDoc represents a documented type.
type TypeDoc struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Fields      string `json:"fields,omitempty"`
}

// Config holds docs pack configuration.
type Config struct {
	// Engine is the documentation engine (required).
	Engine DocEngine

	// Translator is an optional text translator.
	Translator Translator

	// DefaultLanguage is the default documentation language.
	DefaultLanguage string
}

// Pack returns the documentation tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &docsPack{cfg: cfg}
	if p.cfg.DefaultLanguage == "" {
		p.cfg.DefaultLanguage = "en"
	}

	tools := []tool.Tool{
		p.generateDocsTool(),
		p.checkLinksTool(),
		p.spellCheckTool(),
		p.extractAPIDocsTool(),
	}

	if cfg.Translator != nil {
		tools = append(tools, p.translateTool())
	}

	return pack.NewBuilder("docs").
		WithDescription("Documentation tools: generate docs, check links, spell check, extract API docs, translate").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type docsPack struct {
	cfg Config
}

func (p *docsPack) generateDocsTool() tool.Tool {
	return tool.NewBuilder("docs_generate").
		WithDescription("Generate documentation from source code").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				SourcePaths []string `json:"source_paths"`
				OutputDir   string   `json:"output_dir,omitempty"`
				Format      string   `json:"format,omitempty"`
				Template    string   `json:"template,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if len(in.SourcePaths) == 0 {
				return tool.Result{}, fmt.Errorf("source_paths is required")
			}

			result, err := p.cfg.Engine.GenerateDocs(ctx, GenerateOptions{
				SourcePaths: in.SourcePaths,
				OutputDir:   in.OutputDir,
				Format:      in.Format,
				Language:    p.cfg.DefaultLanguage,
				Template:    in.Template,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("generate docs failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *docsPack) checkLinksTool() tool.Tool {
	return tool.NewBuilder("docs_check_links").
		WithDescription("Check for broken links in documentation files").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Paths []string `json:"paths"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if len(in.Paths) == 0 {
				return tool.Result{}, fmt.Errorf("paths is required")
			}

			report, err := p.cfg.Engine.CheckLinks(ctx, in.Paths)
			if err != nil {
				return tool.Result{}, fmt.Errorf("check links failed: %w", err)
			}

			output, _ := json.Marshal(report)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *docsPack) spellCheckTool() tool.Tool {
	return tool.NewBuilder("docs_spell_check").
		WithDescription("Check documentation text for spelling errors").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Text     string `json:"text"`
				Language string `json:"language,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Text == "" {
				return tool.Result{}, fmt.Errorf("text is required")
			}

			lang := in.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			issues, err := p.cfg.Engine.SpellCheck(ctx, in.Text, lang)
			if err != nil {
				return tool.Result{}, fmt.Errorf("spell check failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(issues),
				"issues": issues,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *docsPack) extractAPIDocsTool() tool.Tool {
	return tool.NewBuilder("docs_extract_api").
		WithDescription("Extract API documentation from source code annotations").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				SourcePaths []string `json:"source_paths"`
				Language    string   `json:"language,omitempty"`
				Format      string   `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if len(in.SourcePaths) == 0 {
				return tool.Result{}, fmt.Errorf("source_paths is required")
			}

			apiDoc, err := p.cfg.Engine.ExtractAPIDocs(ctx, ExtractOptions{
				SourcePaths: in.SourcePaths,
				Language:    in.Language,
				Format:      in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("extract api docs failed: %w", err)
			}

			output, _ := json.Marshal(apiDoc)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *docsPack) translateTool() tool.Tool {
	return tool.NewBuilder("docs_translate").
		WithDescription("Translate documentation text to another language").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Text       string `json:"text"`
				SourceLang string `json:"source_lang,omitempty"`
				TargetLang string `json:"target_lang"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Text == "" {
				return tool.Result{}, fmt.Errorf("text is required")
			}
			if in.TargetLang == "" {
				return tool.Result{}, fmt.Errorf("target_lang is required")
			}
			if in.SourceLang == "" {
				in.SourceLang = p.cfg.DefaultLanguage
			}

			translated, err := p.cfg.Translator.Translate(ctx, in.Text, in.SourceLang, in.TargetLang)
			if err != nil {
				return tool.Result{}, fmt.Errorf("translation failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"original":    in.Text,
				"translated":  translated,
				"source_lang": in.SourceLang,
				"target_lang": in.TargetLang,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
