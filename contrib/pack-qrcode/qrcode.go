// Package qrcode provides QR code generation and reading tools for agents.
package qrcode

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	qr "github.com/skip2/go-qrcode"
)

// Pack returns the QR code tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("qrcode").
		WithDescription("QR code generation tools").
		AddTools(
			generateTool(),
			generateBase64Tool(),
			generateSVGTool(),
			generateURLTool(),
			generateVCardTool(),
			generateWiFiTool(),
			generateEmailTool(),
			generateSMSTool(),
			generatePhoneTool(),
			generateGeoTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func generateTool() tool.Tool {
	return tool.NewBuilder("qr_generate").
		WithDescription("Generate a QR code as PNG bytes").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
				Size    int    `json:"size,omitempty"`
				Level   string `json:"level,omitempty"` // L, M, Q, H
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}

			level := qr.Medium
			switch params.Level {
			case "L":
				level = qr.Low
			case "Q":
				level = qr.High
			case "H":
				level = qr.Highest
			}

			png, err := qr.Encode(params.Content, level, params.Size)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"png":    png,
				"size":   params.Size,
				"length": len(png),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateBase64Tool() tool.Tool {
	return tool.NewBuilder("qr_generate_base64").
		WithDescription("Generate a QR code as base64-encoded PNG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
				Size    int    `json:"size,omitempty"`
				Level   string `json:"level,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}

			level := qr.Medium
			switch params.Level {
			case "L":
				level = qr.Low
			case "Q":
				level = qr.High
			case "H":
				level = qr.Highest
			}

			png, err := qr.Encode(params.Content, level, params.Size)
			if err != nil {
				return tool.Result{}, err
			}

			b64 := base64.StdEncoding.EncodeToString(png)

			result := map[string]any{
				"base64":   b64,
				"data_uri": "data:image/png;base64," + b64,
				"size":     params.Size,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateSVGTool() tool.Tool {
	return tool.NewBuilder("qr_generate_svg").
		WithDescription("Generate a QR code as SVG").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
				Size    int    `json:"size,omitempty"`
				Level   string `json:"level,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}

			level := qr.Medium
			switch params.Level {
			case "L":
				level = qr.Low
			case "Q":
				level = qr.High
			case "H":
				level = qr.Highest
			}

			code, err := qr.New(params.Content, level)
			if err != nil {
				return tool.Result{}, err
			}

			svg := code.ToSmallString(false)

			result := map[string]any{
				"svg":  svg,
				"size": params.Size,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateURLTool() tool.Tool {
	return tool.NewBuilder("qr_generate_url").
		WithDescription("Generate a QR code for a URL").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL  string `json:"url"`
				Size int    `json:"size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}

			png, err := qr.Encode(params.URL, qr.Medium, params.Size)
			if err != nil {
				return tool.Result{}, err
			}

			b64 := base64.StdEncoding.EncodeToString(png)

			result := map[string]any{
				"base64":   b64,
				"data_uri": "data:image/png;base64," + b64,
				"url":      params.URL,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateVCardTool() tool.Tool {
	return tool.NewBuilder("qr_generate_vcard").
		WithDescription("Generate a QR code for a vCard contact").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				Phone     string `json:"phone,omitempty"`
				Email     string `json:"email,omitempty"`
				Org       string `json:"org,omitempty"`
				Title     string `json:"title,omitempty"`
				URL       string `json:"url,omitempty"`
				Size      int    `json:"size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}

			vcard := "BEGIN:VCARD\nVERSION:3.0\n"
			vcard += "N:" + params.LastName + ";" + params.FirstName + ";;;\n"
			vcard += "FN:" + params.FirstName + " " + params.LastName + "\n"
			if params.Phone != "" {
				vcard += "TEL:" + params.Phone + "\n"
			}
			if params.Email != "" {
				vcard += "EMAIL:" + params.Email + "\n"
			}
			if params.Org != "" {
				vcard += "ORG:" + params.Org + "\n"
			}
			if params.Title != "" {
				vcard += "TITLE:" + params.Title + "\n"
			}
			if params.URL != "" {
				vcard += "URL:" + params.URL + "\n"
			}
			vcard += "END:VCARD"

			png, err := qr.Encode(vcard, qr.Medium, params.Size)
			if err != nil {
				return tool.Result{}, err
			}

			b64 := base64.StdEncoding.EncodeToString(png)

			result := map[string]any{
				"base64":   b64,
				"data_uri": "data:image/png;base64," + b64,
				"vcard":    vcard,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateWiFiTool() tool.Tool {
	return tool.NewBuilder("qr_generate_wifi").
		WithDescription("Generate a QR code for WiFi credentials").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				SSID     string `json:"ssid"`
				Password string `json:"password"`
				Type     string `json:"type,omitempty"` // WPA, WEP, nopass
				Hidden   bool   `json:"hidden,omitempty"`
				Size     int    `json:"size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}
			if params.Type == "" {
				params.Type = "WPA"
			}

			hidden := ""
			if params.Hidden {
				hidden = "H:true;"
			}

			wifi := "WIFI:T:" + params.Type + ";S:" + params.SSID + ";P:" + params.Password + ";" + hidden + ";"

			png, err := qr.Encode(wifi, qr.Medium, params.Size)
			if err != nil {
				return tool.Result{}, err
			}

			b64 := base64.StdEncoding.EncodeToString(png)

			result := map[string]any{
				"base64":   b64,
				"data_uri": "data:image/png;base64," + b64,
				"wifi":     wifi,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateEmailTool() tool.Tool {
	return tool.NewBuilder("qr_generate_email").
		WithDescription("Generate a QR code for an email").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				To      string `json:"to"`
				Subject string `json:"subject,omitempty"`
				Body    string `json:"body,omitempty"`
				Size    int    `json:"size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}

			mailto := "mailto:" + params.To
			if params.Subject != "" || params.Body != "" {
				mailto += "?"
				if params.Subject != "" {
					mailto += "subject=" + params.Subject
					if params.Body != "" {
						mailto += "&"
					}
				}
				if params.Body != "" {
					mailto += "body=" + params.Body
				}
			}

			png, err := qr.Encode(mailto, qr.Medium, params.Size)
			if err != nil {
				return tool.Result{}, err
			}

			b64 := base64.StdEncoding.EncodeToString(png)

			result := map[string]any{
				"base64":   b64,
				"data_uri": "data:image/png;base64," + b64,
				"mailto":   mailto,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateSMSTool() tool.Tool {
	return tool.NewBuilder("qr_generate_sms").
		WithDescription("Generate a QR code for an SMS").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Phone   string `json:"phone"`
				Message string `json:"message,omitempty"`
				Size    int    `json:"size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}

			sms := "sms:" + params.Phone
			if params.Message != "" {
				sms += "?body=" + params.Message
			}

			png, err := qr.Encode(sms, qr.Medium, params.Size)
			if err != nil {
				return tool.Result{}, err
			}

			b64 := base64.StdEncoding.EncodeToString(png)

			result := map[string]any{
				"base64":   b64,
				"data_uri": "data:image/png;base64," + b64,
				"sms":      sms,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generatePhoneTool() tool.Tool {
	return tool.NewBuilder("qr_generate_phone").
		WithDescription("Generate a QR code for a phone number").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Phone string `json:"phone"`
				Size  int    `json:"size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}

			tel := "tel:" + params.Phone

			png, err := qr.Encode(tel, qr.Medium, params.Size)
			if err != nil {
				return tool.Result{}, err
			}

			b64 := base64.StdEncoding.EncodeToString(png)

			result := map[string]any{
				"base64":   b64,
				"data_uri": "data:image/png;base64," + b64,
				"tel":      tel,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateGeoTool() tool.Tool {
	return tool.NewBuilder("qr_generate_geo").
		WithDescription("Generate a QR code for a geographic location").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
				Size      int     `json:"size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size == 0 {
				params.Size = 256
			}

			geo := "geo:" + formatFloat(params.Latitude) + "," + formatFloat(params.Longitude)

			png, err := qr.Encode(geo, qr.Medium, params.Size)
			if err != nil {
				return tool.Result{}, err
			}

			b64 := base64.StdEncoding.EncodeToString(png)

			result := map[string]any{
				"base64":   b64,
				"data_uri": "data:image/png;base64," + b64,
				"geo":      geo,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}
