// Package ascii provides ASCII art and text tools for agents.
package ascii

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the ASCII tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("ascii").
		WithDescription("ASCII art and text tools").
		AddTools(
			toCodeTool(),
			fromCodeTool(),
			isASCIITool(),
			stripNonASCIITool(),
			boxTool(),
			bannerTool(),
			tableTool(),
			progressBarTool(),
			convertTool(),
			chartTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func toCodeTool() tool.Tool {
	return tool.NewBuilder("ascii_to_code").
		WithDescription("Convert character to ASCII code").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Char string `json:"char"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Char) == 0 {
				result := map[string]any{
					"error": "empty character",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			r := []rune(params.Char)[0]

			result := map[string]any{
				"char":    string(r),
				"decimal": int(r),
				"hex":     strings.ToUpper(string("0123456789abcdef"[int(r)/16]) + string("0123456789abcdef"[int(r)%16])),
				"binary":  intToBinary(int(r)),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fromCodeTool() tool.Tool {
	return tool.NewBuilder("ascii_from_code").
		WithDescription("Convert ASCII code to character").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Code < 0 || params.Code > 127 {
				result := map[string]any{
					"error": "code must be between 0 and 127",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			char := string(rune(params.Code))
			printable := params.Code >= 32 && params.Code < 127

			result := map[string]any{
				"code":      params.Code,
				"char":      char,
				"printable": printable,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isASCIITool() tool.Tool {
	return tool.NewBuilder("ascii_is_ascii").
		WithDescription("Check if text is ASCII").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			isASCII := true
			var nonASCII []string
			for _, r := range params.Text {
				if r > 127 {
					isASCII = false
					nonASCII = append(nonASCII, string(r))
				}
			}

			result := map[string]any{
				"text":      params.Text,
				"is_ascii":  isASCII,
				"non_ascii": nonASCII,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func stripNonASCIITool() tool.Tool {
	return tool.NewBuilder("ascii_strip_non_ascii").
		WithDescription("Remove non-ASCII characters").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text        string `json:"text"`
				Replacement string `json:"replacement,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var result strings.Builder
			stripped := 0
			for _, r := range params.Text {
				if r <= 127 {
					result.WriteRune(r)
				} else {
					stripped++
					if params.Replacement != "" {
						result.WriteString(params.Replacement)
					}
				}
			}

			resultMap := map[string]any{
				"result":   result.String(),
				"stripped": stripped,
			}
			output, _ := json.Marshal(resultMap)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func boxTool() tool.Tool {
	return tool.NewBuilder("ascii_box").
		WithDescription("Draw ASCII box around text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text    string `json:"text"`
				Style   string `json:"style,omitempty"`
				Padding int    `json:"padding,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			padding := params.Padding
			if padding < 0 {
				padding = 0
			}

			lines := strings.Split(params.Text, "\n")
			maxLen := 0
			for _, line := range lines {
				if len(line) > maxLen {
					maxLen = len(line)
				}
			}

			width := maxLen + 2 + (padding * 2)

			var top, bottom, side, corner string
			switch params.Style {
			case "double":
				corner = "+"
				top = "="
				side = "|"
				bottom = "="
			case "rounded":
				corner = "."
				top = "-"
				side = "|"
				bottom = "-"
			default:
				corner = "+"
				top = "-"
				side = "|"
				bottom = "-"
			}

			var result strings.Builder

			// Top border
			result.WriteString(corner)
			result.WriteString(strings.Repeat(top, width-2))
			result.WriteString(corner)
			result.WriteString("\n")

			// Padding lines
			paddingLine := side + strings.Repeat(" ", width-2) + side + "\n"
			for i := 0; i < padding; i++ {
				result.WriteString(paddingLine)
			}

			// Content lines
			for _, line := range lines {
				result.WriteString(side)
				result.WriteString(strings.Repeat(" ", padding+1))
				result.WriteString(line)
				result.WriteString(strings.Repeat(" ", maxLen-len(line)+padding+1))
				result.WriteString(side)
				result.WriteString("\n")
			}

			// Padding lines
			for i := 0; i < padding; i++ {
				result.WriteString(paddingLine)
			}

			// Bottom border
			result.WriteString(corner)
			result.WriteString(strings.Repeat(bottom, width-2))
			result.WriteString(corner)

			resultMap := map[string]any{
				"box":    result.String(),
				"width":  width,
				"height": len(lines) + 2 + (padding * 2),
			}
			output, _ := json.Marshal(resultMap)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func bannerTool() tool.Tool {
	return tool.NewBuilder("ascii_banner").
		WithDescription("Create ASCII banner text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text   string `json:"text"`
				Char   string `json:"char,omitempty"`
				Height int    `json:"height,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			char := params.Char
			if char == "" {
				char = "#"
			}

			height := params.Height
			if height < 3 {
				height = 5
			}

			// Simple block letter implementation
			text := strings.ToUpper(params.Text)
			var lines []string
			for i := 0; i < height; i++ {
				lines = append(lines, "")
			}

			for _, r := range text {
				letter := getBlockLetter(r, char, height)
				for i, line := range letter {
					lines[i] += line + " "
				}
			}

			banner := strings.Join(lines, "\n")

			resultMap := map[string]any{
				"banner": banner,
				"height": height,
			}
			output, _ := json.Marshal(resultMap)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func tableTool() tool.Tool {
	return tool.NewBuilder("ascii_table").
		WithDescription("Create ASCII table").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Headers []string   `json:"headers"`
				Rows    [][]string `json:"rows"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Calculate column widths
			colWidths := make([]int, len(params.Headers))
			for i, h := range params.Headers {
				colWidths[i] = len(h)
			}
			for _, row := range params.Rows {
				for i, cell := range row {
					if i < len(colWidths) && len(cell) > colWidths[i] {
						colWidths[i] = len(cell)
					}
				}
			}

			// Build table
			var result strings.Builder

			// Separator line
			separator := "+"
			for _, w := range colWidths {
				separator += strings.Repeat("-", w+2) + "+"
			}

			// Header
			result.WriteString(separator + "\n")
			result.WriteString("|")
			for i, h := range params.Headers {
				result.WriteString(" " + h + strings.Repeat(" ", colWidths[i]-len(h)+1) + "|")
			}
			result.WriteString("\n")
			result.WriteString(separator + "\n")

			// Rows
			for _, row := range params.Rows {
				result.WriteString("|")
				for i := 0; i < len(colWidths); i++ {
					cell := ""
					if i < len(row) {
						cell = row[i]
					}
					result.WriteString(" " + cell + strings.Repeat(" ", colWidths[i]-len(cell)+1) + "|")
				}
				result.WriteString("\n")
			}
			result.WriteString(separator)

			resultMap := map[string]any{
				"table":   result.String(),
				"columns": len(params.Headers),
				"rows":    len(params.Rows),
			}
			output, _ := json.Marshal(resultMap)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func progressBarTool() tool.Tool {
	return tool.NewBuilder("ascii_progress").
		WithDescription("Create ASCII progress bar").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Percent   int    `json:"percent"`
				Width     int    `json:"width,omitempty"`
				FillChar  string `json:"fill_char,omitempty"`
				EmptyChar string `json:"empty_char,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			width := params.Width
			if width <= 0 {
				width = 50
			}

			fillChar := params.FillChar
			if fillChar == "" {
				fillChar = "#"
			}

			emptyChar := params.EmptyChar
			if emptyChar == "" {
				emptyChar = "-"
			}

			percent := params.Percent
			if percent < 0 {
				percent = 0
			}
			if percent > 100 {
				percent = 100
			}

			filled := width * percent / 100
			empty := width - filled

			bar := "[" + strings.Repeat(fillChar, filled) + strings.Repeat(emptyChar, empty) + "]"

			resultMap := map[string]any{
				"bar":     bar,
				"percent": percent,
				"width":   width,
			}
			output, _ := json.Marshal(resultMap)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func convertTool() tool.Tool {
	return tool.NewBuilder("ascii_convert").
		WithDescription("Convert text to ASCII art style").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text"`
				Style string `json:"style,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var result string
			switch params.Style {
			case "fullwidth":
				result = toFullwidth(params.Text)
			case "small_caps":
				result = toSmallCaps(params.Text)
			case "inverted":
				result = invertedText(params.Text)
			case "strikethrough":
				result = strikethroughText(params.Text)
			default:
				result = params.Text
			}

			resultMap := map[string]any{
				"result": result,
				"style":  params.Style,
			}
			output, _ := json.Marshal(resultMap)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func chartTool() tool.Tool {
	return tool.NewBuilder("ascii_chart").
		WithDescription("Create ASCII bar chart").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data     map[string]int `json:"data"`
				MaxWidth int            `json:"max_width,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			maxWidth := params.MaxWidth
			if maxWidth <= 0 {
				maxWidth = 40
			}

			// Find max value and max label length
			maxValue := 0
			maxLabelLen := 0
			for label, value := range params.Data {
				if value > maxValue {
					maxValue = value
				}
				if len(label) > maxLabelLen {
					maxLabelLen = len(label)
				}
			}

			var result strings.Builder
			for label, value := range params.Data {
				barLen := 0
				if maxValue > 0 {
					barLen = maxWidth * value / maxValue
				}
				result.WriteString(label)
				result.WriteString(strings.Repeat(" ", maxLabelLen-len(label)))
				result.WriteString(" |")
				result.WriteString(strings.Repeat("█", barLen))
				result.WriteString(" ")
				result.WriteString(intToString(value))
				result.WriteString("\n")
			}

			resultMap := map[string]any{
				"chart": strings.TrimSuffix(result.String(), "\n"),
			}
			output, _ := json.Marshal(resultMap)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func intToBinary(n int) string {
	if n == 0 {
		return "0"
	}
	var result strings.Builder
	for n > 0 {
		if n%2 == 0 {
			result.WriteString("0")
		} else {
			result.WriteString("1")
		}
		n /= 2
	}
	// Reverse
	s := result.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

func getBlockLetter(r rune, char string, height int) []string {
	lines := make([]string, height)
	width := height - 1

	if !unicode.IsLetter(r) && r != ' ' {
		for i := range lines {
			lines[i] = strings.Repeat(" ", width)
		}
		return lines
	}

	if r == ' ' {
		for i := range lines {
			lines[i] = strings.Repeat(" ", width/2)
		}
		return lines
	}

	// Simple block letter
	for i := range lines {
		lines[i] = strings.Repeat(char, width)
	}
	return lines
}

func toFullwidth(text string) string {
	var result strings.Builder
	for _, r := range text {
		if r >= '!' && r <= '~' {
			result.WriteRune(r + 0xFF00 - 0x20)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func toSmallCaps(text string) string {
	smallCaps := map[rune]rune{
		'a': 'ᴀ', 'b': 'ʙ', 'c': 'ᴄ', 'd': 'ᴅ', 'e': 'ᴇ',
		'f': 'ꜰ', 'g': 'ɢ', 'h': 'ʜ', 'i': 'ɪ', 'j': 'ᴊ',
		'k': 'ᴋ', 'l': 'ʟ', 'm': 'ᴍ', 'n': 'ɴ', 'o': 'ᴏ',
		'p': 'ᴘ', 'q': 'ǫ', 'r': 'ʀ', 's': 's', 't': 'ᴛ',
		'u': 'ᴜ', 'v': 'ᴠ', 'w': 'ᴡ', 'x': 'x', 'y': 'ʏ',
		'z': 'ᴢ',
	}

	var result strings.Builder
	for _, r := range text {
		if sc, ok := smallCaps[unicode.ToLower(r)]; ok {
			result.WriteRune(sc)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func invertedText(text string) string {
	var result strings.Builder
	for _, r := range text {
		result.WriteRune(r)
		result.WriteRune(0x0332) // Combining low line
	}
	return result.String()
}

func strikethroughText(text string) string {
	var result strings.Builder
	for _, r := range text {
		result.WriteRune(r)
		result.WriteRune(0x0336) // Combining long stroke overlay
	}
	return result.String()
}
