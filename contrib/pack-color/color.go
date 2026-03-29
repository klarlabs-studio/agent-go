// Package color provides color conversion and manipulation tools for agents.
package color

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the color tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("color").
		WithDescription("Color conversion and manipulation tools").
		AddTools(
			parseHexTool(),
			toHexTool(),
			rgbToHslTool(),
			hslToRgbTool(),
			complementaryTool(),
			lightenTool(),
			darkenTool(),
			mixTool(),
			contrastRatioTool(),
			generatePaletteTool(),
			nameToHexTool(),
			validateTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func parseHexTool() tool.Tool {
	return tool.NewBuilder("color_parse_hex").
		WithDescription("Parse a hex color to RGB").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Hex string `json:"hex"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			r, g, b, err := hexToRGB(params.Hex)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"valid": true,
				"hex":   normalizeHex(params.Hex),
				"r":     r,
				"g":     g,
				"b":     b,
				"rgb":   fmt.Sprintf("rgb(%d, %d, %d)", r, g, b),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func toHexTool() tool.Tool {
	return tool.NewBuilder("color_to_hex").
		WithDescription("Convert RGB to hex").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				R int `json:"r"`
				G int `json:"g"`
				B int `json:"b"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			r := clamp(params.R, 0, 255)
			g := clamp(params.G, 0, 255)
			b := clamp(params.B, 0, 255)

			hex := fmt.Sprintf("#%02x%02x%02x", r, g, b)

			result := map[string]any{
				"hex":      hex,
				"hex_long": strings.ToUpper(hex),
				"r":        r,
				"g":        g,
				"b":        b,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func rgbToHslTool() tool.Tool {
	return tool.NewBuilder("color_rgb_to_hsl").
		WithDescription("Convert RGB to HSL").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				R int `json:"r"`
				G int `json:"g"`
				B int `json:"b"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			h, s, l := rgbToHSL(params.R, params.G, params.B)

			result := map[string]any{
				"h":   math.Round(h*10) / 10,
				"s":   math.Round(s*1000) / 10,
				"l":   math.Round(l*1000) / 10,
				"hsl": fmt.Sprintf("hsl(%.0f, %.0f%%, %.0f%%)", h, s*100, l*100),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hslToRgbTool() tool.Tool {
	return tool.NewBuilder("color_hsl_to_rgb").
		WithDescription("Convert HSL to RGB").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				H float64 `json:"h"` // 0-360
				S float64 `json:"s"` // 0-100 or 0-1
				L float64 `json:"l"` // 0-100 or 0-1
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Normalize s and l to 0-1 if they're in 0-100 range
			s := params.S
			l := params.L
			if s > 1 {
				s /= 100
			}
			if l > 1 {
				l /= 100
			}

			r, g, b := hslToRGB(params.H, s, l)

			result := map[string]any{
				"r":   r,
				"g":   g,
				"b":   b,
				"hex": fmt.Sprintf("#%02x%02x%02x", r, g, b),
				"rgb": fmt.Sprintf("rgb(%d, %d, %d)", r, g, b),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func complementaryTool() tool.Tool {
	return tool.NewBuilder("color_complementary").
		WithDescription("Get complementary color").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Hex string `json:"hex"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			r, g, b, err := hexToRGB(params.Hex)
			if err != nil {
				return tool.Result{}, err
			}

			h, s, l := rgbToHSL(r, g, b)
			h = math.Mod(h+180, 360)
			cr, cg, cb := hslToRGB(h, s, l)

			result := map[string]any{
				"original":      normalizeHex(params.Hex),
				"complementary": fmt.Sprintf("#%02x%02x%02x", cr, cg, cb),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lightenTool() tool.Tool {
	return tool.NewBuilder("color_lighten").
		WithDescription("Lighten a color").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Hex    string  `json:"hex"`
				Amount float64 `json:"amount"` // 0-100
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			r, g, b, err := hexToRGB(params.Hex)
			if err != nil {
				return tool.Result{}, err
			}

			h, s, l := rgbToHSL(r, g, b)
			l = math.Min(1, l+params.Amount/100)
			nr, ng, nb := hslToRGB(h, s, l)

			result := map[string]any{
				"original":  normalizeHex(params.Hex),
				"lightened": fmt.Sprintf("#%02x%02x%02x", nr, ng, nb),
				"amount":    params.Amount,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func darkenTool() tool.Tool {
	return tool.NewBuilder("color_darken").
		WithDescription("Darken a color").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Hex    string  `json:"hex"`
				Amount float64 `json:"amount"` // 0-100
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			r, g, b, err := hexToRGB(params.Hex)
			if err != nil {
				return tool.Result{}, err
			}

			h, s, l := rgbToHSL(r, g, b)
			l = math.Max(0, l-params.Amount/100)
			nr, ng, nb := hslToRGB(h, s, l)

			result := map[string]any{
				"original": normalizeHex(params.Hex),
				"darkened": fmt.Sprintf("#%02x%02x%02x", nr, ng, nb),
				"amount":   params.Amount,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func mixTool() tool.Tool {
	return tool.NewBuilder("color_mix").
		WithDescription("Mix two colors").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Hex1   string  `json:"hex1"`
				Hex2   string  `json:"hex2"`
				Weight float64 `json:"weight,omitempty"` // 0-1, default 0.5
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			r1, g1, b1, err := hexToRGB(params.Hex1)
			if err != nil {
				return tool.Result{}, err
			}

			r2, g2, b2, err := hexToRGB(params.Hex2)
			if err != nil {
				return tool.Result{}, err
			}

			weight := params.Weight
			if weight == 0 {
				weight = 0.5
			}

			r := int(float64(r1)*weight + float64(r2)*(1-weight))
			g := int(float64(g1)*weight + float64(g2)*(1-weight))
			b := int(float64(b1)*weight + float64(b2)*(1-weight))

			result := map[string]any{
				"color1": normalizeHex(params.Hex1),
				"color2": normalizeHex(params.Hex2),
				"mixed":  fmt.Sprintf("#%02x%02x%02x", r, g, b),
				"weight": weight,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func contrastRatioTool() tool.Tool {
	return tool.NewBuilder("color_contrast_ratio").
		WithDescription("Calculate WCAG contrast ratio").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Hex1 string `json:"hex1"`
				Hex2 string `json:"hex2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			r1, g1, b1, err := hexToRGB(params.Hex1)
			if err != nil {
				return tool.Result{}, err
			}

			r2, g2, b2, err := hexToRGB(params.Hex2)
			if err != nil {
				return tool.Result{}, err
			}

			l1 := relativeLuminance(r1, g1, b1)
			l2 := relativeLuminance(r2, g2, b2)

			ratio := (math.Max(l1, l2) + 0.05) / (math.Min(l1, l2) + 0.05)

			result := map[string]any{
				"color1":     normalizeHex(params.Hex1),
				"color2":     normalizeHex(params.Hex2),
				"ratio":      math.Round(ratio*100) / 100,
				"aa_normal":  ratio >= 4.5,
				"aa_large":   ratio >= 3,
				"aaa_normal": ratio >= 7,
				"aaa_large":  ratio >= 4.5,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generatePaletteTool() tool.Tool {
	return tool.NewBuilder("color_generate_palette").
		WithDescription("Generate a color palette").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Hex   string `json:"hex"`
				Type  string `json:"type,omitempty"` // complementary, triadic, tetradic, analogous, monochromatic
				Count int    `json:"count,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			r, g, b, err := hexToRGB(params.Hex)
			if err != nil {
				return tool.Result{}, err
			}

			h, s, l := rgbToHSL(r, g, b)

			var colors []string
			colors = append(colors, normalizeHex(params.Hex))

			switch strings.ToLower(params.Type) {
			case "complementary":
				cr, cg, cb := hslToRGB(math.Mod(h+180, 360), s, l)
				colors = append(colors, fmt.Sprintf("#%02x%02x%02x", cr, cg, cb))

			case "triadic":
				for _, offset := range []float64{120, 240} {
					cr, cg, cb := hslToRGB(math.Mod(h+offset, 360), s, l)
					colors = append(colors, fmt.Sprintf("#%02x%02x%02x", cr, cg, cb))
				}

			case "tetradic":
				for _, offset := range []float64{90, 180, 270} {
					cr, cg, cb := hslToRGB(math.Mod(h+offset, 360), s, l)
					colors = append(colors, fmt.Sprintf("#%02x%02x%02x", cr, cg, cb))
				}

			case "analogous":
				for _, offset := range []float64{-30, 30} {
					cr, cg, cb := hslToRGB(math.Mod(h+offset+360, 360), s, l)
					colors = append(colors, fmt.Sprintf("#%02x%02x%02x", cr, cg, cb))
				}

			case "monochromatic":
				count := params.Count
				if count <= 0 {
					count = 5
				}
				step := 1.0 / float64(count+1)
				for i := 1; i <= count; i++ {
					newL := step * float64(i)
					cr, cg, cb := hslToRGB(h, s, newL)
					colors = append(colors, fmt.Sprintf("#%02x%02x%02x", cr, cg, cb))
				}

			default: // shades
				for _, lMod := range []float64{0.2, 0.4, 0.6, 0.8} {
					cr, cg, cb := hslToRGB(h, s, l*lMod)
					colors = append(colors, fmt.Sprintf("#%02x%02x%02x", cr, cg, cb))
				}
			}

			result := map[string]any{
				"base":    normalizeHex(params.Hex),
				"palette": colors,
				"type":    params.Type,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func nameToHexTool() tool.Tool {
	return tool.NewBuilder("color_name_to_hex").
		WithDescription("Convert color name to hex").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			colorMap := map[string]string{
				"black":   "#000000",
				"white":   "#ffffff",
				"red":     "#ff0000",
				"green":   "#008000",
				"blue":    "#0000ff",
				"yellow":  "#ffff00",
				"cyan":    "#00ffff",
				"magenta": "#ff00ff",
				"orange":  "#ffa500",
				"purple":  "#800080",
				"pink":    "#ffc0cb",
				"brown":   "#a52a2a",
				"gray":    "#808080",
				"grey":    "#808080",
			}

			hex, found := colorMap[strings.ToLower(params.Name)]
			if !found {
				result := map[string]any{
					"found": false,
					"name":  params.Name,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"found": true,
				"name":  params.Name,
				"hex":   hex,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("color_validate").
		WithDescription("Validate a color value").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Color string `json:"color"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			color := strings.TrimSpace(params.Color)
			valid := false
			format := ""

			// Check hex
			hexRegex := regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
			if hexRegex.MatchString(color) {
				valid = true
				format = "hex"
			}

			// Check rgb()
			rgbRegex := regexp.MustCompile(`^rgb\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*\)$`)
			if rgbRegex.MatchString(color) {
				valid = true
				format = "rgb"
			}

			// Check hsl()
			hslRegex := regexp.MustCompile(`^hsl\(\s*(\d{1,3})\s*,\s*(\d{1,3})%\s*,\s*(\d{1,3})%\s*\)$`)
			if hslRegex.MatchString(color) {
				valid = true
				format = "hsl"
			}

			result := map[string]any{
				"color":  params.Color,
				"valid":  valid,
				"format": format,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Helper functions

func hexToRGB(hex string) (int, int, int, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string(hex[0]) + string(hex[0]) + string(hex[1]) + string(hex[1]) + string(hex[2]) + string(hex[2])
	}
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color")
	}

	// Parse hex color components (each is 0-255)
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)

	return int(r), int(g), int(b), nil // #nosec G115 -- values are hex color components (0-255)
}

func normalizeHex(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string(hex[0]) + string(hex[0]) + string(hex[1]) + string(hex[1]) + string(hex[2]) + string(hex[2])
	}
	return "#" + strings.ToLower(hex)
}

func rgbToHSL(r, g, b int) (float64, float64, float64) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255

	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	l := (max + min) / 2

	if max == min {
		return 0, 0, l
	}

	d := max - min
	s := d / (1 - math.Abs(2*l-1))

	var h float64
	switch max {
	case rf:
		h = math.Mod((gf-bf)/d, 6)
	case gf:
		h = (bf-rf)/d + 2
	case bf:
		h = (rf-gf)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}

	return h, s, l
}

func hslToRGB(h, s, l float64) (int, int, int) {
	if s == 0 {
		v := int(l * 255)
		return v, v, v
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	hueToRGB := func(p, q, t float64) float64 {
		if t < 0 {
			t += 1
		}
		if t > 1 {
			t -= 1
		}
		if t < 1.0/6 {
			return p + (q-p)*6*t
		}
		if t < 1.0/2 {
			return q
		}
		if t < 2.0/3 {
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}

	r := hueToRGB(p, q, h/360+1.0/3)
	g := hueToRGB(p, q, h/360)
	b := hueToRGB(p, q, h/360-1.0/3)

	return int(r * 255), int(g * 255), int(b * 255)
}

func relativeLuminance(r, g, b int) float64 {
	sRGBtoLinear := func(v int) float64 {
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}

	return 0.2126*sRGBtoLinear(r) + 0.7152*sRGBtoLinear(g) + 0.0722*sRGBtoLinear(b)
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
