// Package code provides code analysis and transformation tools for agent-go.
//
// The pack uses an interface-based approach, allowing any language parser or
// analysis engine to be plugged in. All tools delegate heavy operations to
// the CodeAnalyzer interface.
package code

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// CodeAnalyzer provides code analysis capabilities.
type CodeAnalyzer interface {
	// ParseAST parses source code and returns an AST representation.
	ParseAST(ctx context.Context, source string, language string) (*ASTNode, error)

	// FindReferences finds all references to a symbol in the given source files.
	FindReferences(ctx context.Context, symbol string, files []SourceFile) ([]Reference, error)

	// RenameSymbol renames a symbol across all provided source files.
	RenameSymbol(ctx context.Context, oldName, newName string, files []SourceFile) ([]FileEdit, error)

	// ExtractFunction extracts a code range into a new function.
	ExtractFunction(ctx context.Context, file SourceFile, startLine, endLine int, funcName string) (*FileEdit, error)

	// Lint runs lint checks on source code.
	Lint(ctx context.Context, source string, language string, rules []string) ([]Diagnostic, error)

	// Format formats source code according to language conventions.
	Format(ctx context.Context, source string, language string) (string, error)

	// Refactor applies a named refactoring pattern.
	Refactor(ctx context.Context, source string, language string, pattern string, opts map[string]any) (string, error)
}

// ASTNode represents a node in an abstract syntax tree.
type ASTNode struct {
	Type     string     `json:"type"`
	Name     string     `json:"name,omitempty"`
	Start    Position   `json:"start"`
	End      Position   `json:"end"`
	Children []*ASTNode `json:"children,omitempty"`
	Value    string     `json:"value,omitempty"`
}

// Position represents a location in source code.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
	Offset int `json:"offset,omitempty"`
}

// SourceFile represents a source file with its content.
type SourceFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language,omitempty"`
}

// Reference represents a reference to a symbol in source code.
type Reference struct {
	File     string   `json:"file"`
	Position Position `json:"position"`
	Kind     string   `json:"kind"` // "definition", "usage", "import"
	Context  string   `json:"context,omitempty"`
}

// FileEdit represents a set of edits to a file.
type FileEdit struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Edits   []Edit `json:"edits,omitempty"`
}

// Edit represents a single text edit.
type Edit struct {
	Start       Position `json:"start"`
	End         Position `json:"end"`
	Replacement string   `json:"replacement"`
}

// Diagnostic represents a lint or analysis finding.
type Diagnostic struct {
	File     string   `json:"file,omitempty"`
	Position Position `json:"position"`
	Severity string   `json:"severity"` // "error", "warning", "info", "hint"
	Message  string   `json:"message"`
	Rule     string   `json:"rule,omitempty"`
	Fix      *Edit    `json:"fix,omitempty"`
}

// Config holds code pack configuration.
type Config struct {
	// Analyzer provides code analysis capabilities (required).
	Analyzer CodeAnalyzer

	// DefaultLanguage is the default language for analysis.
	DefaultLanguage string
}

// Pack returns the code analysis tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &codePack{cfg: cfg}

	return pack.NewBuilder("code").
		WithDescription("Code analysis and transformation tools: AST parsing, references, refactoring, linting, formatting").
		WithVersion("1.0.0").
		AddTools(
			p.parseASTTool(),
			p.findReferencesTool(),
			p.renameSymbolTool(),
			p.extractFunctionTool(),
			p.lintTool(),
			p.formatTool(),
			p.refactorTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type codePack struct {
	cfg Config
}

func (p *codePack) parseASTTool() tool.Tool {
	return tool.NewBuilder("code_parse_ast").
		WithDescription("Parse source code into an abstract syntax tree").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Source   string `json:"source"`
				Language string `json:"language,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Source == "" {
				return tool.Result{}, fmt.Errorf("source is required")
			}

			lang := in.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			ast, err := p.cfg.Analyzer.ParseAST(ctx, in.Source, lang)
			if err != nil {
				return tool.Result{}, fmt.Errorf("parse failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"language": lang,
				"ast":      ast,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *codePack) findReferencesTool() tool.Tool {
	return tool.NewBuilder("code_find_references").
		WithDescription("Find all references to a symbol across source files").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Symbol string       `json:"symbol"`
				Files  []SourceFile `json:"files"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Symbol == "" {
				return tool.Result{}, fmt.Errorf("symbol is required")
			}
			if len(in.Files) == 0 {
				return tool.Result{}, fmt.Errorf("files is required")
			}

			refs, err := p.cfg.Analyzer.FindReferences(ctx, in.Symbol, in.Files)
			if err != nil {
				return tool.Result{}, fmt.Errorf("find references failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"symbol":     in.Symbol,
				"count":      len(refs),
				"references": refs,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *codePack) renameSymbolTool() tool.Tool {
	return tool.NewBuilder("code_rename_symbol").
		WithDescription("Rename a symbol across all source files").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				OldName string       `json:"old_name"`
				NewName string       `json:"new_name"`
				Files   []SourceFile `json:"files"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.OldName == "" || in.NewName == "" {
				return tool.Result{}, fmt.Errorf("old_name and new_name are required")
			}
			if len(in.Files) == 0 {
				return tool.Result{}, fmt.Errorf("files is required")
			}

			edits, err := p.cfg.Analyzer.RenameSymbol(ctx, in.OldName, in.NewName, in.Files)
			if err != nil {
				return tool.Result{}, fmt.Errorf("rename failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"old_name":      in.OldName,
				"new_name":      in.NewName,
				"files_changed": len(edits),
				"edits":         edits,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *codePack) extractFunctionTool() tool.Tool {
	return tool.NewBuilder("code_extract_function").
		WithDescription("Extract a code range into a new function").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				File      SourceFile `json:"file"`
				StartLine int        `json:"start_line"`
				EndLine   int        `json:"end_line"`
				FuncName  string     `json:"function_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.File.Content == "" {
				return tool.Result{}, fmt.Errorf("file content is required")
			}
			if in.FuncName == "" {
				return tool.Result{}, fmt.Errorf("function_name is required")
			}
			if in.StartLine <= 0 || in.EndLine <= 0 || in.EndLine < in.StartLine {
				return tool.Result{}, fmt.Errorf("valid start_line and end_line are required")
			}

			edit, err := p.cfg.Analyzer.ExtractFunction(ctx, in.File, in.StartLine, in.EndLine, in.FuncName)
			if err != nil {
				return tool.Result{}, fmt.Errorf("extract function failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"function_name": in.FuncName,
				"result":        edit,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *codePack) lintTool() tool.Tool {
	return tool.NewBuilder("code_lint").
		WithDescription("Run lint checks on source code").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Source   string   `json:"source"`
				Language string   `json:"language,omitempty"`
				Rules    []string `json:"rules,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Source == "" {
				return tool.Result{}, fmt.Errorf("source is required")
			}

			lang := in.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			diagnostics, err := p.cfg.Analyzer.Lint(ctx, in.Source, lang, in.Rules)
			if err != nil {
				return tool.Result{}, fmt.Errorf("lint failed: %w", err)
			}

			errors := 0
			warnings := 0
			for _, d := range diagnostics {
				switch d.Severity {
				case "error":
					errors++
				case "warning":
					warnings++
				}
			}

			output, _ := json.Marshal(map[string]any{
				"language":    lang,
				"count":       len(diagnostics),
				"errors":      errors,
				"warnings":    warnings,
				"diagnostics": diagnostics,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *codePack) formatTool() tool.Tool {
	return tool.NewBuilder("code_format").
		WithDescription("Format source code according to language conventions").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Source   string `json:"source"`
				Language string `json:"language,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Source == "" {
				return tool.Result{}, fmt.Errorf("source is required")
			}

			lang := in.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			formatted, err := p.cfg.Analyzer.Format(ctx, in.Source, lang)
			if err != nil {
				return tool.Result{}, fmt.Errorf("format failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"language":  lang,
				"formatted": formatted,
				"changed":   formatted != in.Source,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *codePack) refactorTool() tool.Tool {
	return tool.NewBuilder("code_refactor").
		WithDescription("Apply a named refactoring pattern to source code").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Source   string         `json:"source"`
				Language string         `json:"language,omitempty"`
				Pattern  string         `json:"pattern"`
				Options  map[string]any `json:"options,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Source == "" {
				return tool.Result{}, fmt.Errorf("source is required")
			}
			if in.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			lang := in.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			result, err := p.cfg.Analyzer.Refactor(ctx, in.Source, lang, in.Pattern, in.Options)
			if err != nil {
				return tool.Result{}, fmt.Errorf("refactor failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"language":   lang,
				"pattern":    in.Pattern,
				"refactored": result,
				"changed":    result != in.Source,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
