// Package spreadsheet provides tools for reading and writing CSV and Excel files.
package spreadsheet

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Config holds spreadsheet pack configuration.
type Config struct {
	// MaxRows limits the number of rows to read (0 = unlimited)
	MaxRows int
	// MaxCols limits the number of columns to read (0 = unlimited)
	MaxCols int
	// DefaultSheet is the default sheet name for Excel files
	DefaultSheet string
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		MaxRows:      100000,
		MaxCols:      1000,
		DefaultSheet: "Sheet1",
	}
}

type spreadsheetPack struct {
	cfg Config
}

// Pack creates a new spreadsheet tools pack.
func Pack(cfg Config) *pack.Pack {
	p := &spreadsheetPack{cfg: cfg}

	return pack.NewBuilder("spreadsheet").
		WithDescription("Tools for reading and writing CSV and Excel spreadsheet files").
		WithVersion("1.0.0").
		AddTools(
			// CSV tools
			p.readCSVTool(),
			p.writeCSVTool(),
			p.appendCSVTool(),
			p.parseCSVTool(),
			p.generateCSVTool(),
			// Excel tools
			p.readExcelTool(),
			p.writeExcelTool(),
			p.getSheetsTool(),
			p.createSheetTool(),
			p.deleteSheetTool(),
			p.getCellTool(),
			p.setCellTool(),
			p.getRangeTool(),
			p.setRangeTool(),
			p.insertRowTool(),
			p.insertColTool(),
			p.deleteRowTool(),
			p.deleteColTool(),
			p.mergeCellsTool(),
			p.setCellStyleTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// readCSVTool reads a CSV file.
func (p *spreadsheetPack) readCSVTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_read_csv").
		WithDescription("Read a CSV file and return its contents as JSON").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path      string `json:"path"`
				Delimiter string `json:"delimiter,omitempty"`
				HasHeader bool   `json:"has_header,omitempty"`
				Limit     int    `json:"limit,omitempty"`
				Offset    int    `json:"offset,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			file, err := os.Open(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			reader := csv.NewReader(file)
			if params.Delimiter != "" && len(params.Delimiter) > 0 {
				reader.Comma = rune(params.Delimiter[0])
			}

			records, err := reader.ReadAll()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read CSV: %w", err)
			}

			var headers []string
			startRow := 0
			if params.HasHeader && len(records) > 0 {
				headers = records[0]
				startRow = 1
			}

			// Apply offset and limit
			endRow := len(records)
			if params.Offset > 0 {
				startRow += params.Offset
			}
			if params.Limit > 0 && startRow+params.Limit < endRow {
				endRow = startRow + params.Limit
			}
			if startRow >= len(records) {
				startRow = len(records)
			}
			if endRow > len(records) {
				endRow = len(records)
			}

			var rows []interface{}
			for i := startRow; i < endRow; i++ {
				if params.HasHeader && len(headers) > 0 {
					row := make(map[string]string)
					for j, val := range records[i] {
						if j < len(headers) {
							row[headers[j]] = val
						} else {
							row[fmt.Sprintf("col_%d", j)] = val
						}
					}
					rows = append(rows, row)
				} else {
					rows = append(rows, records[i])
				}
			}

			result := map[string]interface{}{
				"rows":       rows,
				"row_count":  len(rows),
				"col_count":  len(headers),
				"has_header": params.HasHeader,
			}
			if params.HasHeader {
				result["headers"] = headers
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// writeCSVTool writes data to a CSV file.
func (p *spreadsheetPack) writeCSVTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_write_csv").
		WithDescription("Write data to a CSV file").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path      string          `json:"path"`
				Data      json.RawMessage `json:"data"`
				Headers   []string        `json:"headers,omitempty"`
				Delimiter string          `json:"delimiter,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			// Parse data - can be array of arrays or array of objects
			var rows [][]string
			var dataArray []interface{}
			if err := json.Unmarshal(params.Data, &dataArray); err != nil {
				return tool.Result{}, fmt.Errorf("data must be an array: %w", err)
			}

			for _, item := range dataArray {
				switch v := item.(type) {
				case []interface{}:
					row := make([]string, len(v))
					for i, cell := range v {
						row[i] = fmt.Sprintf("%v", cell)
					}
					rows = append(rows, row)
				case map[string]interface{}:
					if len(params.Headers) == 0 {
						// Extract headers from first object
						for k := range v {
							params.Headers = append(params.Headers, k)
						}
					}
					row := make([]string, len(params.Headers))
					for i, h := range params.Headers {
						if val, ok := v[h]; ok {
							row[i] = fmt.Sprintf("%v", val)
						}
					}
					rows = append(rows, row)
				}
			}

			file, err := os.Create(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create file: %w", err)
			}
			defer file.Close()

			writer := csv.NewWriter(file)
			if params.Delimiter != "" && len(params.Delimiter) > 0 {
				writer.Comma = rune(params.Delimiter[0])
			}

			// Write headers if present
			if len(params.Headers) > 0 {
				if err := writer.Write(params.Headers); err != nil {
					return tool.Result{}, fmt.Errorf("failed to write headers: %w", err)
				}
			}

			// Write data rows
			for _, row := range rows {
				if err := writer.Write(row); err != nil {
					return tool.Result{}, fmt.Errorf("failed to write row: %w", err)
				}
			}
			writer.Flush()

			if err := writer.Error(); err != nil {
				return tool.Result{}, fmt.Errorf("CSV write error: %w", err)
			}

			result := map[string]interface{}{
				"path":      params.Path,
				"row_count": len(rows),
				"success":   true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// appendCSVTool appends rows to a CSV file.
func (p *spreadsheetPack) appendCSVTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_append_csv").
		WithDescription("Append rows to an existing CSV file").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path      string          `json:"path"`
				Data      json.RawMessage `json:"data"`
				Delimiter string          `json:"delimiter,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			var rows [][]string
			var dataArray []interface{}
			if err := json.Unmarshal(params.Data, &dataArray); err != nil {
				return tool.Result{}, fmt.Errorf("data must be an array: %w", err)
			}

			for _, item := range dataArray {
				if v, ok := item.([]interface{}); ok {
					row := make([]string, len(v))
					for i, cell := range v {
						row[i] = fmt.Sprintf("%v", cell)
					}
					rows = append(rows, row)
				}
			}

			file, err := os.OpenFile(params.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}

			writeErr := func() error {
				writer := csv.NewWriter(file)
				if params.Delimiter != "" && len(params.Delimiter) > 0 {
					writer.Comma = rune(params.Delimiter[0])
				}

				for _, row := range rows {
					if err := writer.Write(row); err != nil {
						return fmt.Errorf("failed to write row: %w", err)
					}
				}
				writer.Flush()
				return writer.Error()
			}()

			if closeErr := file.Close(); writeErr != nil {
				return tool.Result{}, writeErr
			} else if closeErr != nil {
				return tool.Result{}, fmt.Errorf("failed to close file: %w", closeErr)
			}

			result := map[string]interface{}{
				"path":       params.Path,
				"rows_added": len(rows),
				"success":    true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// parseCSVTool parses CSV content from a string.
func (p *spreadsheetPack) parseCSVTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_parse_csv").
		WithDescription("Parse CSV content from a string").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content   string `json:"content"`
				Delimiter string `json:"delimiter,omitempty"`
				HasHeader bool   `json:"has_header,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Content == "" {
				return tool.Result{}, fmt.Errorf("content is required")
			}

			reader := csv.NewReader(strings.NewReader(params.Content))
			if params.Delimiter != "" && len(params.Delimiter) > 0 {
				reader.Comma = rune(params.Delimiter[0])
			}

			records, err := reader.ReadAll()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse CSV: %w", err)
			}

			var headers []string
			startRow := 0
			if params.HasHeader && len(records) > 0 {
				headers = records[0]
				startRow = 1
			}

			var rows []interface{}
			for i := startRow; i < len(records); i++ {
				if params.HasHeader && len(headers) > 0 {
					row := make(map[string]string)
					for j, val := range records[i] {
						if j < len(headers) {
							row[headers[j]] = val
						}
					}
					rows = append(rows, row)
				} else {
					rows = append(rows, records[i])
				}
			}

			result := map[string]interface{}{
				"rows":      rows,
				"row_count": len(rows),
			}
			if params.HasHeader {
				result["headers"] = headers
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// generateCSVTool generates CSV content as a string.
func (p *spreadsheetPack) generateCSVTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_generate_csv").
		WithDescription("Generate CSV content as a string from data").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data      json.RawMessage `json:"data"`
				Headers   []string        `json:"headers,omitempty"`
				Delimiter string          `json:"delimiter,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var rows [][]string
			var dataArray []interface{}
			if err := json.Unmarshal(params.Data, &dataArray); err != nil {
				return tool.Result{}, fmt.Errorf("data must be an array: %w", err)
			}

			for _, item := range dataArray {
				switch v := item.(type) {
				case []interface{}:
					row := make([]string, len(v))
					for i, cell := range v {
						row[i] = fmt.Sprintf("%v", cell)
					}
					rows = append(rows, row)
				case map[string]interface{}:
					if len(params.Headers) == 0 {
						for k := range v {
							params.Headers = append(params.Headers, k)
						}
					}
					row := make([]string, len(params.Headers))
					for i, h := range params.Headers {
						if val, ok := v[h]; ok {
							row[i] = fmt.Sprintf("%v", val)
						}
					}
					rows = append(rows, row)
				}
			}

			var buf bytes.Buffer
			writer := csv.NewWriter(&buf)
			if params.Delimiter != "" && len(params.Delimiter) > 0 {
				writer.Comma = rune(params.Delimiter[0])
			}

			if len(params.Headers) > 0 {
				if err := writer.Write(params.Headers); err != nil {
					return tool.Result{}, fmt.Errorf("failed to write headers: %w", err)
				}
			}

			for _, row := range rows {
				if err := writer.Write(row); err != nil {
					return tool.Result{}, fmt.Errorf("failed to write row: %w", err)
				}
			}
			writer.Flush()

			result := map[string]interface{}{
				"content":   buf.String(),
				"row_count": len(rows),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// readExcelTool reads an Excel file.
func (p *spreadsheetPack) readExcelTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_read_excel").
		WithDescription("Read an Excel file and return sheet contents").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path      string `json:"path"`
				Sheet     string `json:"sheet,omitempty"`
				HasHeader bool   `json:"has_header,omitempty"`
				Limit     int    `json:"limit,omitempty"`
				Offset    int    `json:"offset,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open Excel file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			rows, err := f.GetRows(sheet)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get rows: %w", err)
			}

			var headers []string
			startRow := 0
			if params.HasHeader && len(rows) > 0 {
				headers = rows[0]
				startRow = 1
			}

			endRow := len(rows)
			if params.Offset > 0 {
				startRow += params.Offset
			}
			if params.Limit > 0 && startRow+params.Limit < endRow {
				endRow = startRow + params.Limit
			}
			if startRow >= len(rows) {
				startRow = len(rows)
			}

			var resultRows []interface{}
			for i := startRow; i < endRow; i++ {
				if params.HasHeader && len(headers) > 0 {
					row := make(map[string]string)
					for j, val := range rows[i] {
						if j < len(headers) {
							row[headers[j]] = val
						} else {
							row[fmt.Sprintf("col_%d", j)] = val
						}
					}
					resultRows = append(resultRows, row)
				} else {
					resultRows = append(resultRows, rows[i])
				}
			}

			result := map[string]interface{}{
				"rows":       resultRows,
				"row_count":  len(resultRows),
				"sheet":      sheet,
				"has_header": params.HasHeader,
			}
			if params.HasHeader {
				result["headers"] = headers
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// writeExcelTool writes data to an Excel file.
func (p *spreadsheetPack) writeExcelTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_write_excel").
		WithDescription("Write data to an Excel file").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path    string          `json:"path"`
				Sheet   string          `json:"sheet,omitempty"`
				Data    json.RawMessage `json:"data"`
				Headers []string        `json:"headers,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			sheet := params.Sheet
			if sheet == "" {
				sheet = p.cfg.DefaultSheet
			}

			var f *excelize.File
			if _, err := os.Stat(params.Path); err == nil {
				f, err = excelize.OpenFile(params.Path)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
				}
			} else {
				f = excelize.NewFile()
			}
			defer f.Close()

			// Ensure sheet exists
			idx, err := f.GetSheetIndex(sheet)
			if err != nil || idx < 0 {
				if _, err := f.NewSheet(sheet); err != nil {
					return tool.Result{}, fmt.Errorf("failed to create sheet: %w", err)
				}
			}

			var dataArray []interface{}
			if err := json.Unmarshal(params.Data, &dataArray); err != nil {
				return tool.Result{}, fmt.Errorf("data must be an array: %w", err)
			}

			rowNum := 1

			// Write headers if present
			if len(params.Headers) > 0 {
				for col, header := range params.Headers {
					cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
					_ = f.SetCellValue(sheet, cell, header) // #nosec G104 -- best-effort write, errors handled at save
				}
				rowNum++
			}

			// Write data
			for _, item := range dataArray {
				switch v := item.(type) {
				case []interface{}:
					for col, val := range v {
						cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
						_ = f.SetCellValue(sheet, cell, val) // #nosec G104 -- best-effort write, errors handled at save
					}
				case map[string]interface{}:
					if len(params.Headers) == 0 {
						for k := range v {
							params.Headers = append(params.Headers, k)
						}
						for col, header := range params.Headers {
							cell, _ := excelize.CoordinatesToCellName(col+1, 1)
							_ = f.SetCellValue(sheet, cell, header) // #nosec G104 -- best-effort write, errors handled at save
						}
						rowNum = 2
					}
					for col, h := range params.Headers {
						if val, ok := v[h]; ok {
							cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
							_ = f.SetCellValue(sheet, cell, val) // #nosec G104 -- best-effort write, errors handled at save
						}
					}
				}
				rowNum++
			}

			if err := f.SaveAs(params.Path); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"path":      params.Path,
				"sheet":     sheet,
				"row_count": rowNum - 1,
				"success":   true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// getSheetsTool gets list of sheets in an Excel file.
func (p *spreadsheetPack) getSheetsTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_get_sheets").
		WithDescription("Get list of sheets in an Excel file").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheets := f.GetSheetList()

			result := map[string]interface{}{
				"sheets": sheets,
				"count":  len(sheets),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// createSheetTool creates a new sheet.
func (p *spreadsheetPack) createSheetTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_create_sheet").
		WithDescription("Create a new sheet in an Excel file").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path  string `json:"path"`
				Sheet string `json:"sheet"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Sheet == "" {
				return tool.Result{}, fmt.Errorf("path and sheet are required")
			}

			var f *excelize.File
			var err error

			if _, statErr := os.Stat(params.Path); statErr == nil {
				f, err = excelize.OpenFile(params.Path)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
				}
			} else {
				f = excelize.NewFile()
			}
			defer f.Close()

			idx, err := f.NewSheet(params.Sheet)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create sheet: %w", err)
			}

			if err := f.SaveAs(params.Path); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"path":    params.Path,
				"sheet":   params.Sheet,
				"index":   idx,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// deleteSheetTool deletes a sheet.
func (p *spreadsheetPack) deleteSheetTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_delete_sheet").
		WithDescription("Delete a sheet from an Excel file").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path  string `json:"path"`
				Sheet string `json:"sheet"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Sheet == "" {
				return tool.Result{}, fmt.Errorf("path and sheet are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			if err := f.DeleteSheet(params.Sheet); err != nil {
				return tool.Result{}, fmt.Errorf("failed to delete sheet: %w", err)
			}

			if err := f.Save(); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"path":    params.Path,
				"sheet":   params.Sheet,
				"deleted": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// getCellTool gets a cell value.
func (p *spreadsheetPack) getCellTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_get_cell").
		WithDescription("Get the value of a specific cell").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path  string `json:"path"`
				Sheet string `json:"sheet,omitempty"`
				Cell  string `json:"cell"` // e.g., "A1"
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Cell == "" {
				return tool.Result{}, fmt.Errorf("path and cell are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			value, err := f.GetCellValue(sheet, params.Cell)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get cell: %w", err)
			}

			formula, _ := f.GetCellFormula(sheet, params.Cell)

			result := map[string]interface{}{
				"cell":    params.Cell,
				"value":   value,
				"sheet":   sheet,
				"formula": formula,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// setCellTool sets a cell value.
func (p *spreadsheetPack) setCellTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_set_cell").
		WithDescription("Set the value of a specific cell").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path    string      `json:"path"`
				Sheet   string      `json:"sheet,omitempty"`
				Cell    string      `json:"cell"`
				Value   interface{} `json:"value"`
				Formula string      `json:"formula,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Cell == "" {
				return tool.Result{}, fmt.Errorf("path and cell are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			if params.Formula != "" {
				if err := f.SetCellFormula(sheet, params.Cell, params.Formula); err != nil {
					return tool.Result{}, fmt.Errorf("failed to set formula: %w", err)
				}
			} else {
				if err := f.SetCellValue(sheet, params.Cell, params.Value); err != nil {
					return tool.Result{}, fmt.Errorf("failed to set cell: %w", err)
				}
			}

			if err := f.Save(); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"cell":    params.Cell,
				"sheet":   sheet,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// getRangeTool gets values from a range.
func (p *spreadsheetPack) getRangeTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_get_range").
		WithDescription("Get values from a cell range").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path  string `json:"path"`
				Sheet string `json:"sheet,omitempty"`
				Range string `json:"range"` // e.g., "A1:C10"
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Range == "" {
				return tool.Result{}, fmt.Errorf("path and range are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			// Parse range
			parts := strings.Split(params.Range, ":")
			if len(parts) != 2 {
				return tool.Result{}, fmt.Errorf("invalid range format, use A1:B2")
			}

			startCol, startRow, err := excelize.CellNameToCoordinates(parts[0])
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid start cell: %w", err)
			}
			endCol, endRow, err := excelize.CellNameToCoordinates(parts[1])
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid end cell: %w", err)
			}

			var values [][]interface{}
			for row := startRow; row <= endRow; row++ {
				var rowValues []interface{}
				for col := startCol; col <= endCol; col++ {
					cell, _ := excelize.CoordinatesToCellName(col, row)
					value, _ := f.GetCellValue(sheet, cell)
					rowValues = append(rowValues, value)
				}
				values = append(values, rowValues)
			}

			result := map[string]interface{}{
				"range":     params.Range,
				"values":    values,
				"row_count": len(values),
				"sheet":     sheet,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// setRangeTool sets values in a range.
func (p *spreadsheetPack) setRangeTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_set_range").
		WithDescription("Set values in a cell range").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path   string          `json:"path"`
				Sheet  string          `json:"sheet,omitempty"`
				Start  string          `json:"start"` // e.g., "A1"
				Values json.RawMessage `json:"values"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Start == "" {
				return tool.Result{}, fmt.Errorf("path and start are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			startCol, startRow, err := excelize.CellNameToCoordinates(params.Start)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid start cell: %w", err)
			}

			var values [][]interface{}
			if err := json.Unmarshal(params.Values, &values); err != nil {
				return tool.Result{}, fmt.Errorf("values must be 2D array: %w", err)
			}

			cellCount := 0
			for rowIdx, row := range values {
				for colIdx, value := range row {
					cell, _ := excelize.CoordinatesToCellName(startCol+colIdx, startRow+rowIdx)
					if err := f.SetCellValue(sheet, cell, value); err != nil {
						return tool.Result{}, fmt.Errorf("failed to set cell %s: %w", cell, err)
					}
					cellCount++
				}
			}

			if err := f.Save(); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"start":      params.Start,
				"cell_count": cellCount,
				"sheet":      sheet,
				"success":    true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// insertRowTool inserts a row.
func (p *spreadsheetPack) insertRowTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_insert_row").
		WithDescription("Insert a new row at a specific position").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path  string `json:"path"`
				Sheet string `json:"sheet,omitempty"`
				Row   int    `json:"row"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Row < 1 {
				return tool.Result{}, fmt.Errorf("path and valid row number are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			if err := f.InsertRows(sheet, params.Row, 1); err != nil {
				return tool.Result{}, fmt.Errorf("failed to insert row: %w", err)
			}

			if err := f.Save(); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"row":     params.Row,
				"sheet":   sheet,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// insertColTool inserts a column.
func (p *spreadsheetPack) insertColTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_insert_col").
		WithDescription("Insert a new column at a specific position").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path  string `json:"path"`
				Sheet string `json:"sheet,omitempty"`
				Col   string `json:"col"` // e.g., "A"
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Col == "" {
				return tool.Result{}, fmt.Errorf("path and col are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			if err := f.InsertCols(sheet, params.Col, 1); err != nil {
				return tool.Result{}, fmt.Errorf("failed to insert column: %w", err)
			}

			if err := f.Save(); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"col":     params.Col,
				"sheet":   sheet,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// deleteRowTool deletes a row.
func (p *spreadsheetPack) deleteRowTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_delete_row").
		WithDescription("Delete a row at a specific position").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path  string `json:"path"`
				Sheet string `json:"sheet,omitempty"`
				Row   int    `json:"row"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Row < 1 {
				return tool.Result{}, fmt.Errorf("path and valid row number are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			if err := f.RemoveRow(sheet, params.Row); err != nil {
				return tool.Result{}, fmt.Errorf("failed to delete row: %w", err)
			}

			if err := f.Save(); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"row":     params.Row,
				"sheet":   sheet,
				"deleted": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// deleteColTool deletes a column.
func (p *spreadsheetPack) deleteColTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_delete_col").
		WithDescription("Delete a column at a specific position").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path  string `json:"path"`
				Sheet string `json:"sheet,omitempty"`
				Col   string `json:"col"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Col == "" {
				return tool.Result{}, fmt.Errorf("path and col are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			if err := f.RemoveCol(sheet, params.Col); err != nil {
				return tool.Result{}, fmt.Errorf("failed to delete column: %w", err)
			}

			if err := f.Save(); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"col":     params.Col,
				"sheet":   sheet,
				"deleted": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// mergeCellsTool merges cells.
func (p *spreadsheetPack) mergeCellsTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_merge_cells").
		WithDescription("Merge a range of cells").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path        string `json:"path"`
				Sheet       string `json:"sheet,omitempty"`
				TopLeft     string `json:"top_left"`
				BottomRight string `json:"bottom_right"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.TopLeft == "" || params.BottomRight == "" {
				return tool.Result{}, fmt.Errorf("path, top_left, and bottom_right are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			if err := f.MergeCell(sheet, params.TopLeft, params.BottomRight); err != nil {
				return tool.Result{}, fmt.Errorf("failed to merge cells: %w", err)
			}

			if err := f.Save(); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"range":   params.TopLeft + ":" + params.BottomRight,
				"sheet":   sheet,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// setCellStyleTool sets cell style.
func (p *spreadsheetPack) setCellStyleTool() tool.Tool {
	return tool.NewBuilder("spreadsheet_set_cell_style").
		WithDescription("Set style for a cell or range").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path      string  `json:"path"`
				Sheet     string  `json:"sheet,omitempty"`
				Cell      string  `json:"cell,omitempty"`
				Range     string  `json:"range,omitempty"`
				Bold      bool    `json:"bold,omitempty"`
				Italic    bool    `json:"italic,omitempty"`
				FontSize  float64 `json:"font_size,omitempty"`
				FontColor string  `json:"font_color,omitempty"`
				FillColor string  `json:"fill_color,omitempty"`
				Alignment string  `json:"alignment,omitempty"` // left, center, right
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || (params.Cell == "" && params.Range == "") {
				return tool.Result{}, fmt.Errorf("path and cell or range are required")
			}

			f, err := excelize.OpenFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			sheet := params.Sheet
			if sheet == "" {
				sheet = f.GetSheetName(0)
			}

			// Build style
			style := &excelize.Style{}
			if params.Bold || params.Italic || params.FontSize > 0 || params.FontColor != "" {
				style.Font = &excelize.Font{
					Bold:   params.Bold,
					Italic: params.Italic,
					Size:   params.FontSize,
					Color:  params.FontColor,
				}
			}
			if params.FillColor != "" {
				style.Fill = excelize.Fill{
					Type:    "pattern",
					Pattern: 1,
					Color:   []string{params.FillColor},
				}
			}
			if params.Alignment != "" {
				style.Alignment = &excelize.Alignment{
					Horizontal: params.Alignment,
				}
			}

			styleID, err := f.NewStyle(style)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create style: %w", err)
			}

			if params.Range != "" {
				parts := strings.Split(params.Range, ":")
				if len(parts) == 2 {
					if err := f.SetCellStyle(sheet, parts[0], parts[1], styleID); err != nil {
						return tool.Result{}, fmt.Errorf("failed to set style: %w", err)
					}
				}
			} else {
				if err := f.SetCellStyle(sheet, params.Cell, params.Cell, styleID); err != nil {
					return tool.Result{}, fmt.Errorf("failed to set style: %w", err)
				}
			}

			if err := f.Save(); err != nil {
				return tool.Result{}, fmt.Errorf("failed to save file: %w", err)
			}

			result := map[string]interface{}{
				"sheet":   sheet,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Helper function to ensure imports are used
var (
	_ = io.EOF
	_ = filepath.Base
	_ = strconv.Itoa
)
