// Package ocr provides document OCR and text extraction tools for agent-go.
//
// The pack uses an interface-based approach, allowing any OCR engine
// (Tesseract, Google Vision, AWS Textract, etc.) to be plugged in.
package ocr

import (
	"context"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// OCREngine provides text extraction capabilities from images and documents.
type OCREngine interface {
	// ExtractText extracts all text from an image or document.
	ExtractText(ctx context.Context, input DocumentInput, opts ExtractOptions) (*ExtractResult, error)

	// DetectTables detects and extracts tables from a document.
	DetectTables(ctx context.Context, input DocumentInput) ([]Table, error)

	// DetectLayout analyzes the document layout and structure.
	DetectLayout(ctx context.Context, input DocumentInput) (*Layout, error)
}

// DocumentInput represents an image or document for OCR processing.
type DocumentInput struct {
	// Data is base64-encoded image/document data.
	Data string `json:"data,omitempty"`

	// URL is a URL to the image/document.
	URL string `json:"url,omitempty"`

	// Format is the document format (e.g., "png", "jpeg", "pdf", "tiff").
	Format string `json:"format,omitempty"`

	// Pages specifies which pages to process for multi-page documents.
	Pages []int `json:"pages,omitempty"`
}

// ExtractOptions configures text extraction behavior.
type ExtractOptions struct {
	// Languages specifies expected languages (ISO 639-1 codes).
	Languages []string `json:"languages,omitempty"`

	// Mode specifies extraction mode: "text", "handwriting", "mixed".
	Mode string `json:"mode,omitempty"`

	// IncludeBoundingBoxes includes word/line bounding boxes in output.
	IncludeBoundingBoxes bool `json:"include_bounding_boxes,omitempty"`

	// IncludeConfidence includes confidence scores in output.
	IncludeConfidence bool `json:"include_confidence,omitempty"`
}

// ExtractResult contains the extracted text and metadata.
type ExtractResult struct {
	Text       string  `json:"text"`
	Language   string  `json:"language,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Pages      []Page  `json:"pages,omitempty"`
}

// Page represents a single page of extracted text.
type Page struct {
	Number int     `json:"number"`
	Text   string  `json:"text"`
	Lines  []Line  `json:"lines,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

// Line represents a line of text with optional bounding box.
type Line struct {
	Text        string       `json:"text"`
	Confidence  float64      `json:"confidence,omitempty"`
	BoundingBox *BoundingBox `json:"bounding_box,omitempty"`
	Words       []Word       `json:"words,omitempty"`
}

// Word represents a single word with optional bounding box.
type Word struct {
	Text        string       `json:"text"`
	Confidence  float64      `json:"confidence,omitempty"`
	BoundingBox *BoundingBox `json:"bounding_box,omitempty"`
}

// BoundingBox represents a rectangular region.
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Table represents an extracted table.
type Table struct {
	Rows        int          `json:"rows"`
	Columns     int          `json:"columns"`
	Cells       [][]Cell     `json:"cells"`
	Confidence  float64      `json:"confidence,omitempty"`
	BoundingBox *BoundingBox `json:"bounding_box,omitempty"`
}

// Cell represents a table cell.
type Cell struct {
	Text       string  `json:"text"`
	Row        int     `json:"row"`
	Column     int     `json:"column"`
	RowSpan    int     `json:"row_span,omitempty"`
	ColSpan    int     `json:"col_span,omitempty"`
	IsHeader   bool    `json:"is_header,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// Layout represents the detected document layout.
type Layout struct {
	Blocks []Block `json:"blocks"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

// Block represents a layout block in the document.
type Block struct {
	Type        string       `json:"type"` // "text", "table", "figure", "header", "footer", "list"
	Text        string       `json:"text,omitempty"`
	Confidence  float64      `json:"confidence,omitempty"`
	BoundingBox *BoundingBox `json:"bounding_box,omitempty"`
	Order       int          `json:"order"`
}

// InvoiceParser parses structured data from invoice documents.
type InvoiceParser interface {
	// ParseInvoice extracts structured invoice data.
	ParseInvoice(ctx context.Context, input DocumentInput) (*Invoice, error)
}

// ReceiptParser parses structured data from receipt documents.
type ReceiptParser interface {
	// ParseReceipt extracts structured receipt data.
	ParseReceipt(ctx context.Context, input DocumentInput) (*Receipt, error)
}

// Invoice represents parsed invoice data.
type Invoice struct {
	InvoiceNumber string     `json:"invoice_number,omitempty"`
	Date          string     `json:"date,omitempty"`
	DueDate       string     `json:"due_date,omitempty"`
	Vendor        Entity     `json:"vendor,omitempty"`
	Customer      Entity     `json:"customer,omitempty"`
	LineItems     []LineItem `json:"line_items,omitempty"`
	Subtotal      float64    `json:"subtotal,omitempty"`
	Tax           float64    `json:"tax,omitempty"`
	Total         float64    `json:"total"`
	Currency      string     `json:"currency,omitempty"`
	Confidence    float64    `json:"confidence,omitempty"`
}

// Receipt represents parsed receipt data.
type Receipt struct {
	Merchant  string     `json:"merchant,omitempty"`
	Date      string     `json:"date,omitempty"`
	Items     []LineItem `json:"items,omitempty"`
	Subtotal  float64    `json:"subtotal,omitempty"`
	Tax       float64    `json:"tax,omitempty"`
	Total     float64    `json:"total"`
	Currency  string     `json:"currency,omitempty"`
	PayMethod string     `json:"payment_method,omitempty"`
}

// Entity represents a business entity.
type Entity struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
	Phone   string `json:"phone,omitempty"`
	Email   string `json:"email,omitempty"`
	TaxID   string `json:"tax_id,omitempty"`
}

// LineItem represents an item on an invoice or receipt.
type LineItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity,omitempty"`
	UnitPrice   float64 `json:"unit_price,omitempty"`
	Amount      float64 `json:"amount"`
}

// Config holds OCR pack configuration.
type Config struct {
	// Engine is the OCR engine (required).
	Engine OCREngine

	// InvoiceParser is an optional invoice parser.
	InvoiceParser InvoiceParser

	// ReceiptParser is an optional receipt parser.
	ReceiptParser ReceiptParser

	// DefaultLanguages are the default languages for OCR.
	DefaultLanguages []string
}

// Pack returns the OCR tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &ocrPack{cfg: cfg}
	if len(p.cfg.DefaultLanguages) == 0 {
		p.cfg.DefaultLanguages = []string{"en"}
	}

	tools := []tool.Tool{
		p.extractTextTool(),
		p.detectTablesTool(),
		p.detectLayoutTool(),
		p.handwritingOCRTool(),
	}

	if cfg.InvoiceParser != nil {
		tools = append(tools, p.parseInvoiceTool())
	}
	if cfg.ReceiptParser != nil {
		tools = append(tools, p.parseReceiptTool())
	}

	return pack.NewBuilder("ocr").
		WithDescription("Document OCR tools: text extraction, table detection, invoice/receipt parsing, layout analysis").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type ocrPack struct {
	cfg Config
}

func (p *ocrPack) extractTextTool() tool.Tool {
	return tool.NewBuilder("ocr_extract_text").
		WithDescription("Extract text from an image or document using OCR").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data      string   `json:"data,omitempty"`
				URL       string   `json:"url,omitempty"`
				Format    string   `json:"format,omitempty"`
				Languages []string `json:"languages,omitempty"`
				Pages     []int    `json:"pages,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" {
				return tool.Result{}, fmt.Errorf("data or url is required")
			}

			langs := in.Languages
			if len(langs) == 0 {
				langs = p.cfg.DefaultLanguages
			}

			result, err := p.cfg.Engine.ExtractText(ctx, DocumentInput{
				Data:   in.Data,
				URL:    in.URL,
				Format: in.Format,
				Pages:  in.Pages,
			}, ExtractOptions{
				Languages:         langs,
				Mode:              "text",
				IncludeConfidence: true,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("text extraction failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"text":       result.Text,
				"language":   result.Language,
				"confidence": result.Confidence,
				"pages":      len(result.Pages),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ocrPack) detectTablesTool() tool.Tool {
	return tool.NewBuilder("ocr_detect_tables").
		WithDescription("Detect and extract tables from a document").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data   string `json:"data,omitempty"`
				URL    string `json:"url,omitempty"`
				Format string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" {
				return tool.Result{}, fmt.Errorf("data or url is required")
			}

			tables, err := p.cfg.Engine.DetectTables(ctx, DocumentInput{
				Data:   in.Data,
				URL:    in.URL,
				Format: in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("table detection failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(tables),
				"tables": tables,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ocrPack) detectLayoutTool() tool.Tool {
	return tool.NewBuilder("ocr_detect_layout").
		WithDescription("Analyze document layout and structure").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data   string `json:"data,omitempty"`
				URL    string `json:"url,omitempty"`
				Format string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" {
				return tool.Result{}, fmt.Errorf("data or url is required")
			}

			layout, err := p.cfg.Engine.DetectLayout(ctx, DocumentInput{
				Data:   in.Data,
				URL:    in.URL,
				Format: in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("layout detection failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"blocks": layout.Blocks,
				"width":  layout.Width,
				"height": layout.Height,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ocrPack) handwritingOCRTool() tool.Tool {
	return tool.NewBuilder("ocr_handwriting").
		WithDescription("Extract handwritten text from an image").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data      string   `json:"data,omitempty"`
				URL       string   `json:"url,omitempty"`
				Format    string   `json:"format,omitempty"`
				Languages []string `json:"languages,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" {
				return tool.Result{}, fmt.Errorf("data or url is required")
			}

			langs := in.Languages
			if len(langs) == 0 {
				langs = p.cfg.DefaultLanguages
			}

			result, err := p.cfg.Engine.ExtractText(ctx, DocumentInput{
				Data:   in.Data,
				URL:    in.URL,
				Format: in.Format,
			}, ExtractOptions{
				Languages:         langs,
				Mode:              "handwriting",
				IncludeConfidence: true,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("handwriting OCR failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"text":       result.Text,
				"confidence": result.Confidence,
				"language":   result.Language,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ocrPack) parseInvoiceTool() tool.Tool {
	return tool.NewBuilder("ocr_parse_invoice").
		WithDescription("Parse structured data from an invoice document").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data   string `json:"data,omitempty"`
				URL    string `json:"url,omitempty"`
				Format string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" {
				return tool.Result{}, fmt.Errorf("data or url is required")
			}

			invoice, err := p.cfg.InvoiceParser.ParseInvoice(ctx, DocumentInput{
				Data:   in.Data,
				URL:    in.URL,
				Format: in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("invoice parsing failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"invoice": invoice,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ocrPack) parseReceiptTool() tool.Tool {
	return tool.NewBuilder("ocr_parse_receipt").
		WithDescription("Parse structured data from a receipt").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data   string `json:"data,omitempty"`
				URL    string `json:"url,omitempty"`
				Format string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" {
				return tool.Result{}, fmt.Errorf("data or url is required")
			}

			receipt, err := p.cfg.ReceiptParser.ParseReceipt(ctx, DocumentInput{
				Data:   in.Data,
				URL:    in.URL,
				Format: in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("receipt parsing failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"receipt": receipt,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
