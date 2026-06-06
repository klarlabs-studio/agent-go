// Package ast provides Go AST parsing and analysis tools for agents.
package ast

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the Go AST tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("ast").
		WithDescription("Go AST parsing and analysis tools").
		AddTools(
			parseTool(),
			extractFunctionsTool(),
			extractTypesTool(),
			extractInterfacesTool(),
			extractStructsTool(),
			extractImportsTool(),
			extractConstsTool(),
			extractVarsTool(),
			extractCommentsTool(),
			findSymbolTool(),
			getCallGraphTool(),
			formatTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("ast_parse").
		WithDescription("Parse Go source code into AST").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"package":     f.Name.Name,
				"num_decls":   len(f.Decls),
				"num_imports": len(f.Imports),
				"has_doc":     f.Doc != nil,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractFunctionsTool() tool.Tool {
	return tool.NewBuilder("ast_extract_functions").
		WithDescription("Extract function declarations from Go source").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			var functions []map[string]any
			ast.Inspect(f, func(n ast.Node) bool {
				if fn, ok := n.(*ast.FuncDecl); ok {
					fnInfo := map[string]any{
						"name":       fn.Name.Name,
						"exported":   ast.IsExported(fn.Name.Name),
						"has_doc":    fn.Doc != nil,
						"start_line": fset.Position(fn.Pos()).Line,
						"end_line":   fset.Position(fn.End()).Line,
					}
					if fn.Recv != nil && len(fn.Recv.List) > 0 {
						fnInfo["receiver"] = true
					}
					if fn.Type.Params != nil {
						fnInfo["num_params"] = len(fn.Type.Params.List)
					}
					if fn.Type.Results != nil {
						fnInfo["num_results"] = len(fn.Type.Results.List)
					}
					functions = append(functions, fnInfo)
				}
				return true
			})

			result := map[string]any{
				"functions": functions,
				"count":     len(functions),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractTypesTool() tool.Tool {
	return tool.NewBuilder("ast_extract_types").
		WithDescription("Extract type declarations from Go source").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			var types []map[string]any
			ast.Inspect(f, func(n ast.Node) bool {
				if ts, ok := n.(*ast.TypeSpec); ok {
					typeInfo := map[string]any{
						"name":     ts.Name.Name,
						"exported": ast.IsExported(ts.Name.Name),
					}
					switch ts.Type.(type) {
					case *ast.StructType:
						typeInfo["kind"] = "struct"
					case *ast.InterfaceType:
						typeInfo["kind"] = "interface"
					case *ast.ArrayType:
						typeInfo["kind"] = "array"
					case *ast.MapType:
						typeInfo["kind"] = "map"
					case *ast.ChanType:
						typeInfo["kind"] = "chan"
					case *ast.FuncType:
						typeInfo["kind"] = "func"
					default:
						typeInfo["kind"] = "alias"
					}
					types = append(types, typeInfo)
				}
				return true
			})

			result := map[string]any{
				"types": types,
				"count": len(types),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractInterfacesTool() tool.Tool {
	return tool.NewBuilder("ast_extract_interfaces").
		WithDescription("Extract interface declarations with methods").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			var interfaces []map[string]any
			ast.Inspect(f, func(n ast.Node) bool {
				if ts, ok := n.(*ast.TypeSpec); ok {
					if it, ok := ts.Type.(*ast.InterfaceType); ok {
						var methods []string
						if it.Methods != nil {
							for _, m := range it.Methods.List {
								if len(m.Names) > 0 {
									methods = append(methods, m.Names[0].Name)
								}
							}
						}
						interfaces = append(interfaces, map[string]any{
							"name":        ts.Name.Name,
							"exported":    ast.IsExported(ts.Name.Name),
							"methods":     methods,
							"num_methods": len(methods),
						})
					}
				}
				return true
			})

			result := map[string]any{
				"interfaces": interfaces,
				"count":      len(interfaces),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractStructsTool() tool.Tool {
	return tool.NewBuilder("ast_extract_structs").
		WithDescription("Extract struct declarations with fields").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			var structs []map[string]any
			ast.Inspect(f, func(n ast.Node) bool {
				if ts, ok := n.(*ast.TypeSpec); ok {
					if st, ok := ts.Type.(*ast.StructType); ok {
						var fields []map[string]any
						if st.Fields != nil {
							for _, field := range st.Fields.List {
								for _, name := range field.Names {
									fields = append(fields, map[string]any{
										"name":     name.Name,
										"exported": ast.IsExported(name.Name),
									})
								}
							}
						}
						structs = append(structs, map[string]any{
							"name":       ts.Name.Name,
							"exported":   ast.IsExported(ts.Name.Name),
							"fields":     fields,
							"num_fields": len(fields),
						})
					}
				}
				return true
			})

			result := map[string]any{
				"structs": structs,
				"count":   len(structs),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractImportsTool() tool.Tool {
	return tool.NewBuilder("ast_extract_imports").
		WithDescription("Extract import declarations").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ImportsOnly)
			if err != nil {
				return tool.Result{}, err
			}

			var imports []map[string]any
			for _, imp := range f.Imports {
				impInfo := map[string]any{
					"path": strings.Trim(imp.Path.Value, `"`),
				}
				if imp.Name != nil {
					impInfo["alias"] = imp.Name.Name
				}
				imports = append(imports, impInfo)
			}

			result := map[string]any{
				"imports": imports,
				"count":   len(imports),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractConstsTool() tool.Tool {
	return tool.NewBuilder("ast_extract_consts").
		WithDescription("Extract constant declarations").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			var consts []map[string]any
			ast.Inspect(f, func(n ast.Node) bool {
				if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.CONST {
					for _, spec := range gd.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range vs.Names {
								consts = append(consts, map[string]any{
									"name":     name.Name,
									"exported": ast.IsExported(name.Name),
								})
							}
						}
					}
				}
				return true
			})

			result := map[string]any{
				"constants": consts,
				"count":     len(consts),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractVarsTool() tool.Tool {
	return tool.NewBuilder("ast_extract_vars").
		WithDescription("Extract variable declarations").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			var vars []map[string]any
			ast.Inspect(f, func(n ast.Node) bool {
				if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.VAR {
					for _, spec := range gd.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range vs.Names {
								vars = append(vars, map[string]any{
									"name":     name.Name,
									"exported": ast.IsExported(name.Name),
								})
							}
						}
					}
				}
				return true
			})

			result := map[string]any{
				"variables": vars,
				"count":     len(vars),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractCommentsTool() tool.Tool {
	return tool.NewBuilder("ast_extract_comments").
		WithDescription("Extract comments from Go source").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			var comments []map[string]any
			for _, cg := range f.Comments {
				for _, c := range cg.List {
					comments = append(comments, map[string]any{
						"text": c.Text,
						"line": fset.Position(c.Pos()).Line,
					})
				}
			}

			result := map[string]any{
				"comments": comments,
				"count":    len(comments),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func findSymbolTool() tool.Tool {
	return tool.NewBuilder("ast_find_symbol").
		WithDescription("Find a symbol by name in Go source").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
				Name   string `json:"name"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			var found []map[string]any
			ast.Inspect(f, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					if node.Name.Name == params.Name {
						found = append(found, map[string]any{
							"kind": "function",
							"name": params.Name,
							"line": fset.Position(node.Pos()).Line,
						})
					}
				case *ast.TypeSpec:
					if node.Name.Name == params.Name {
						found = append(found, map[string]any{
							"kind": "type",
							"name": params.Name,
							"line": fset.Position(node.Pos()).Line,
						})
					}
				case *ast.ValueSpec:
					for _, name := range node.Names {
						if name.Name == params.Name {
							found = append(found, map[string]any{
								"kind": "value",
								"name": params.Name,
								"line": fset.Position(name.Pos()).Line,
							})
						}
					}
				}
				return true
			})

			result := map[string]any{
				"found": found,
				"count": len(found),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getCallGraphTool() tool.Tool {
	return tool.NewBuilder("ast_call_graph").
		WithDescription("Extract function call graph from Go source").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			calls := make(map[string][]string)
			var currentFunc string

			ast.Inspect(f, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					currentFunc = node.Name.Name
					if calls[currentFunc] == nil {
						calls[currentFunc] = []string{}
					}
				case *ast.CallExpr:
					if currentFunc != "" {
						if ident, ok := node.Fun.(*ast.Ident); ok {
							calls[currentFunc] = append(calls[currentFunc], ident.Name)
						} else if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
							calls[currentFunc] = append(calls[currentFunc], sel.Sel.Name)
						}
					}
				}
				return true
			})

			result := map[string]any{
				"calls": calls,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatTool() tool.Tool {
	return tool.NewBuilder("ast_format").
		WithDescription("Format Go source code").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "source.go", params.Source, parser.ParseComments)
			if err != nil {
				return tool.Result{}, err
			}

			var buf strings.Builder
			if err := printer.Fprint(&buf, fset, f); err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"formatted": buf.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
