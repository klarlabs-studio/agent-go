// Package pdf provides sandboxed PDF processing tools for agent-go.
//
// MVP tools (registered by Pack):
//   - pdf_extract_text: Extract plain text from a PDF
//   - pdf_metadata: Title, author, page count, and file size when available
//   - pdf_merge: Concatenate multiple PDFs into one
//   - pdf_split: Extract a page range into a new PDF
//
// Not in MVP (not registered): pdf_extract_images, pdf_compress,
// pdf_to_images, pdf_from_html, encryption.
//
// All paths are resolved relative to a configured base directory and
// rejected if they escape that sandbox.
package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pdfread "github.com/ledongthuc/pdf"
	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/sandbox"
)

// Pack returns the PDF processing tools pack.
// The baseDir parameter restricts all file operations to the given directory
// to prevent path traversal attacks. All paths provided by tool inputs are
// resolved relative to baseDir.
func Pack(baseDir string) *pack.Pack {
	return pack.NewBuilder("pdf").
		WithDescription("PDF processing tools: extract text, metadata, merge, and split").
		WithVersion("0.1.0").
		AddTools(
			pdfExtractText(baseDir),
			pdfMetadata(baseDir),
			pdfMerge(baseDir),
			pdfSplit(baseDir),
		).
		AllowInState(agent.StateExplore, "pdf_extract_text", "pdf_metadata").
		AllowInState(agent.StateAct, "pdf_extract_text", "pdf_metadata", "pdf_merge", "pdf_split").
		AllowInState(agent.StateValidate, "pdf_extract_text", "pdf_metadata").
		Build()
}

// --- Path security ---

func safePath(baseDir, userPath string) (string, error) {
	return sandbox.SafePath(baseDir, userPath)
}

// --- Input/Output types ---

type extractTextInput struct {
	Path string `json:"path"`
}

type extractTextOutput struct {
	Text      string `json:"text"`
	PageCount int    `json:"page_count"`
}

type metadataInput struct {
	Path string `json:"path"`
}

type metadataOutput struct {
	Title     string `json:"title,omitempty"`
	Author    string `json:"author,omitempty"`
	Subject   string `json:"subject,omitempty"`
	PageCount int    `json:"page_count"`
	Size      int64  `json:"size"`
}

type mergeInput struct {
	Paths  []string `json:"paths"`
	Output string   `json:"output"`
}

type mergeOutput struct {
	Output    string `json:"output"`
	PageCount int    `json:"page_count"`
}

type splitInput struct {
	Path      string `json:"path"`
	Output    string `json:"output"`
	Pages     string `json:"pages"` // pdfcpu page selection, e.g. "1", "1-3", "1,3"
	StartPage int    `json:"start_page,omitempty"`
	EndPage   int    `json:"end_page,omitempty"`
}

type splitOutput struct {
	Output    string `json:"output"`
	PageCount int    `json:"page_count"`
	Pages     string `json:"pages"`
}

// --- Tool constructors ---

func pdfExtractText(baseDir string) tool.Tool {
	return tool.NewBuilder("pdf_extract_text").
		WithDescription("Extract plain text content from a PDF document").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path": json.RawMessage(`{"type":"string","description":"Path to the PDF (relative to base directory)"}`),
		}, []string{"path"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in extractTextInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			// #nosec G304 -- path is sanitized via safePath
			f, r, err := pdfread.Open(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open PDF: %w", err)
			}
			defer f.Close()

			plain, err := r.GetPlainText()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to extract text: %w", err)
			}
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(plain); err != nil {
				return tool.Result{}, fmt.Errorf("failed to read text: %w", err)
			}

			out := extractTextOutput{
				Text:      buf.String(),
				PageCount: r.NumPage(),
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func pdfMetadata(baseDir string) tool.Tool {
	return tool.NewBuilder("pdf_metadata").
		WithDescription("Get PDF metadata including title, author, page count, and file size").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path": json.RawMessage(`{"type":"string","description":"Path to the PDF (relative to base directory)"}`),
		}, []string{"path"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in metadataInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			info, err := os.Stat(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to stat PDF: %w", err)
			}

			out := metadataOutput{Size: info.Size()}

			// #nosec G304 -- path is sanitized via safePath
			f, err := os.Open(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open PDF: %w", err)
			}
			defer f.Close()

			pdfInfo, err := pdfcpuapi.PDFInfo(ctx, f, fullPath, nil, false, nil)
			if err != nil {
				// Fall back to page count only when metadata parsing fails.
				n, pcErr := pdfcpuapi.PageCountFile(ctx, fullPath)
				if pcErr != nil {
					return tool.Result{}, fmt.Errorf("failed to read PDF metadata: %w", err)
				}
				out.PageCount = n
			} else {
				out.Title = pdfInfo.Title
				out.Author = pdfInfo.Author
				out.Subject = pdfInfo.Subject
				out.PageCount = pdfInfo.PageCount
			}

			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func pdfMerge(baseDir string) tool.Tool {
	return tool.NewBuilder("pdf_merge").
		WithDescription("Merge multiple PDF documents into one by concatenation").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"paths":  json.RawMessage(`{"type":"array","items":{"type":"string"},"description":"PDF paths to merge in order (relative to base directory)"}`),
			"output": json.RawMessage(`{"type":"string","description":"Output PDF path (relative to base directory)"}`),
		}, []string{"paths", "output"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in mergeInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			if len(in.Paths) == 0 {
				return tool.Result{}, fmt.Errorf("paths must not be empty")
			}

			inFiles := make([]string, 0, len(in.Paths))
			for _, p := range in.Paths {
				full, err := safePath(baseDir, p)
				if err != nil {
					return tool.Result{}, err
				}
				inFiles = append(inFiles, full)
			}
			outPath, err := safePath(baseDir, in.Output)
			if err != nil {
				return tool.Result{}, err
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0750); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create output directory: %w", err)
			}

			if err := pdfcpuapi.MergeCreateFile(ctx, inFiles, outPath, false, nil); err != nil {
				return tool.Result{}, fmt.Errorf("failed to merge PDFs: %w", err)
			}

			n, err := pdfcpuapi.PageCountFile(ctx, outPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("merged but failed to count pages: %w", err)
			}

			out := mergeOutput{Output: in.Output, PageCount: n}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func pdfSplit(baseDir string) tool.Tool {
	return tool.NewBuilder("pdf_split").
		WithDescription("Split a PDF by extracting a page range into a new file").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":       json.RawMessage(`{"type":"string","description":"Source PDF path (relative to base directory)"}`),
			"output":     json.RawMessage(`{"type":"string","description":"Output PDF path (relative to base directory)"}`),
			"pages":      json.RawMessage(`{"type":"string","description":"Page selection (e.g. \"1\", \"1-3\", \"1,3\"). Alternative to start_page/end_page."}`),
			"start_page": json.RawMessage(`{"type":"integer","description":"1-based start page (inclusive), used when pages is empty"}`),
			"end_page":   json.RawMessage(`{"type":"integer","description":"1-based end page (inclusive), used when pages is empty"}`),
		}, []string{"path", "output"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in splitInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			srcPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			outPath, err := safePath(baseDir, in.Output)
			if err != nil {
				return tool.Result{}, err
			}

			pages := strings.TrimSpace(in.Pages)
			if pages == "" {
				if in.StartPage < 1 || in.EndPage < in.StartPage {
					return tool.Result{}, fmt.Errorf("provide pages or a valid start_page/end_page range")
				}
				if in.StartPage == in.EndPage {
					pages = fmt.Sprintf("%d", in.StartPage)
				} else {
					pages = fmt.Sprintf("%d-%d", in.StartPage, in.EndPage)
				}
			}

			if err := os.MkdirAll(filepath.Dir(outPath), 0750); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create output directory: %w", err)
			}

			if err := pdfcpuapi.TrimFile(ctx, srcPath, outPath, []string{pages}, nil); err != nil {
				return tool.Result{}, fmt.Errorf("failed to split PDF: %w", err)
			}

			n, err := pdfcpuapi.PageCountFile(ctx, outPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("split but failed to count pages: %w", err)
			}

			out := splitOutput{Output: in.Output, PageCount: n, Pages: pages}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}
