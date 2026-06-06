// Package pdf provides PDF processing tools for agent-go.
//
// This pack includes tools for PDF operations:
//   - pdf_extract_text: Extract text content from a PDF
//   - pdf_extract_images: Extract images from a PDF
//   - pdf_metadata: Get PDF metadata (title, author, pages)
//   - pdf_merge: Merge multiple PDFs into one
//   - pdf_split: Split a PDF into multiple files
//   - pdf_compress: Compress a PDF to reduce file size
//   - pdf_to_images: Convert PDF pages to images
//   - pdf_from_html: Generate PDF from HTML content
//
// Supports encrypted PDFs with password handling.
package pdf

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the PDF processing tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("pdf").
		WithDescription("PDF processing and manipulation tools").
		WithVersion("0.1.0").
		AddTools(
			pdfExtractText(),
			pdfExtractImages(),
			pdfMetadata(),
			pdfMerge(),
			pdfSplit(),
			pdfCompress(),
			pdfToImages(),
			pdfFromHTML(),
		).
		AllowInState(agent.StateExplore, "pdf_extract_text", "pdf_metadata").
		AllowInState(agent.StateAct, "pdf_extract_text", "pdf_extract_images", "pdf_metadata", "pdf_merge", "pdf_split", "pdf_compress", "pdf_to_images", "pdf_from_html").
		AllowInState(agent.StateValidate, "pdf_metadata").
		Build()
}

func pdfExtractText() tool.Tool {
	return tool.NewBuilder("pdf_extract_text").
		WithDescription("Extract text content from a PDF document").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func pdfExtractImages() tool.Tool {
	return tool.NewBuilder("pdf_extract_images").
		WithDescription("Extract embedded images from a PDF document").
		ReadOnly().
		MustBuild()
}

func pdfMetadata() tool.Tool {
	return tool.NewBuilder("pdf_metadata").
		WithDescription("Get PDF metadata including title, author, page count").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func pdfMerge() tool.Tool {
	return tool.NewBuilder("pdf_merge").
		WithDescription("Merge multiple PDF documents into one").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func pdfSplit() tool.Tool {
	return tool.NewBuilder("pdf_split").
		WithDescription("Split a PDF into multiple documents by page range").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func pdfCompress() tool.Tool {
	return tool.NewBuilder("pdf_compress").
		WithDescription("Compress a PDF to reduce file size").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func pdfToImages() tool.Tool {
	return tool.NewBuilder("pdf_to_images").
		WithDescription("Convert PDF pages to image files").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func pdfFromHTML() tool.Tool {
	return tool.NewBuilder("pdf_from_html").
		WithDescription("Generate a PDF document from HTML content").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}
