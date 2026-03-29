// Package random provides random generation tools for agents.
package random

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the random generation tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("random").
		WithDescription("Random generation tools").
		AddTools(
			intTool(),
			floatTool(),
			stringTool(),
			bytesTool(),
			boolTool(),
			choiceTool(),
			shuffleTool(),
			sampleTool(),
			passwordTool(),
			tokenTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func intTool() tool.Tool {
	return tool.NewBuilder("random_int").
		WithDescription("Generate a random integer").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Min int64 `json:"min,omitempty"`
				Max int64 `json:"max,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Max == 0 && params.Min == 0 {
				params.Max = 100
			}

			if params.Max <= params.Min {
				params.Max = params.Min + 100
			}

			rangeSize := big.NewInt(params.Max - params.Min + 1)
			n, err := rand.Int(rand.Reader, rangeSize)
			if err != nil {
				return tool.Result{}, err
			}

			value := n.Int64() + params.Min

			result := map[string]any{
				"value": value,
				"min":   params.Min,
				"max":   params.Max,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func floatTool() tool.Tool {
	return tool.NewBuilder("random_float").
		WithDescription("Generate a random float").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Min       float64 `json:"min,omitempty"`
				Max       float64 `json:"max,omitempty"`
				Precision int     `json:"precision,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Max == 0 && params.Min == 0 {
				params.Max = 1.0
			}

			if params.Max <= params.Min {
				params.Max = params.Min + 1.0
			}

			// Generate random bytes and convert to float
			precision := int64(1000000) // 6 decimal places
			rangeSize := big.NewInt(precision)
			n, err := rand.Int(rand.Reader, rangeSize)
			if err != nil {
				return tool.Result{}, err
			}

			ratio := float64(n.Int64()) / float64(precision)
			value := params.Min + ratio*(params.Max-params.Min)

			result := map[string]any{
				"value": value,
				"min":   params.Min,
				"max":   params.Max,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func stringTool() tool.Tool {
	return tool.NewBuilder("random_string").
		WithDescription("Generate a random string").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Length  int    `json:"length,omitempty"`
				Charset string `json:"charset,omitempty"` // alphanumeric, alpha, numeric, hex, custom
				Custom  string `json:"custom,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			length := params.Length
			if length <= 0 {
				length = 16
			}
			if length > 1000 {
				length = 1000
			}

			var charset string
			switch strings.ToLower(params.Charset) {
			case "alpha":
				charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
			case "numeric":
				charset = "0123456789"
			case "hex":
				charset = "0123456789abcdef"
			case "custom":
				charset = params.Custom
			default:
				charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			}

			if charset == "" {
				charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			}

			result := make([]byte, length)
			charsetLen := big.NewInt(int64(len(charset)))
			for i := 0; i < length; i++ {
				n, err := rand.Int(rand.Reader, charsetLen)
				if err != nil {
					return tool.Result{}, err
				}
				result[i] = charset[n.Int64()]
			}

			output, _ := json.Marshal(map[string]any{
				"value":  string(result),
				"length": length,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func bytesTool() tool.Tool {
	return tool.NewBuilder("random_bytes").
		WithDescription("Generate random bytes").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Length   int    `json:"length,omitempty"`
				Encoding string `json:"encoding,omitempty"` // hex, base64
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			length := params.Length
			if length <= 0 {
				length = 32
			}
			if length > 1024 {
				length = 1024
			}

			bytes := make([]byte, length)
			_, err := rand.Read(bytes)
			if err != nil {
				return tool.Result{}, err
			}

			var encoded string
			switch strings.ToLower(params.Encoding) {
			case "base64":
				encoded = base64.StdEncoding.EncodeToString(bytes)
			default:
				encoded = hex.EncodeToString(bytes)
			}

			result := map[string]any{
				"value":    encoded,
				"length":   length,
				"encoding": params.Encoding,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func boolTool() tool.Tool {
	return tool.NewBuilder("random_bool").
		WithDescription("Generate a random boolean").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Probability float64 `json:"probability,omitempty"` // 0-1, probability of true
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			prob := params.Probability
			if prob == 0 {
				prob = 0.5
			}

			n, err := rand.Int(rand.Reader, big.NewInt(100))
			if err != nil {
				return tool.Result{}, err
			}

			value := float64(n.Int64())/100 < prob

			result := map[string]any{
				"value":       value,
				"probability": prob,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func choiceTool() tool.Tool {
	return tool.NewBuilder("random_choice").
		WithDescription("Pick a random item from a list").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []any `json:"items"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Items) == 0 {
				result := map[string]any{
					"value": nil,
					"index": -1,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(params.Items))))
			if err != nil {
				return tool.Result{}, err
			}

			index := n.Int64()

			result := map[string]any{
				"value": params.Items[index],
				"index": index,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func shuffleTool() tool.Tool {
	return tool.NewBuilder("random_shuffle").
		WithDescription("Shuffle a list randomly").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []any `json:"items"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Fisher-Yates shuffle
			shuffled := make([]any, len(params.Items))
			copy(shuffled, params.Items)

			for i := len(shuffled) - 1; i > 0; i-- {
				n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
				if err != nil {
					return tool.Result{}, err
				}
				j := n.Int64()
				shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
			}

			result := map[string]any{
				"shuffled": shuffled,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sampleTool() tool.Tool {
	return tool.NewBuilder("random_sample").
		WithDescription("Sample random items from a list without replacement").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []any `json:"items"`
				Count int   `json:"count"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			count := params.Count
			if count > len(params.Items) {
				count = len(params.Items)
			}

			// Shuffle and take first count items
			shuffled := make([]any, len(params.Items))
			copy(shuffled, params.Items)

			for i := len(shuffled) - 1; i > 0; i-- {
				n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
				if err != nil {
					return tool.Result{}, err
				}
				j := n.Int64()
				shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
			}

			result := map[string]any{
				"sample": shuffled[:count],
				"count":  count,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func passwordTool() tool.Tool {
	return tool.NewBuilder("random_password").
		WithDescription("Generate a secure random password").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Length     int  `json:"length,omitempty"`
				Uppercase  bool `json:"uppercase,omitempty"`
				Lowercase  bool `json:"lowercase,omitempty"`
				Digits     bool `json:"digits,omitempty"`
				Symbols    bool `json:"symbols,omitempty"`
				ExcludeAmb bool `json:"exclude_ambiguous,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			length := params.Length
			if length <= 0 {
				length = 16
			}
			if length > 128 {
				length = 128
			}

			// Default to all character types
			if !params.Uppercase && !params.Lowercase && !params.Digits && !params.Symbols {
				params.Uppercase = true
				params.Lowercase = true
				params.Digits = true
				params.Symbols = true
			}

			var charset string
			if params.Lowercase {
				if params.ExcludeAmb {
					charset += "abcdefghjkmnpqrstuvwxyz"
				} else {
					charset += "abcdefghijklmnopqrstuvwxyz"
				}
			}
			if params.Uppercase {
				if params.ExcludeAmb {
					charset += "ABCDEFGHJKMNPQRSTUVWXYZ"
				} else {
					charset += "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
				}
			}
			if params.Digits {
				if params.ExcludeAmb {
					charset += "23456789"
				} else {
					charset += "0123456789"
				}
			}
			if params.Symbols {
				charset += "!@#$%^&*()-_=+[]{}|;:,.<>?"
			}

			password := make([]byte, length)
			charsetLen := big.NewInt(int64(len(charset)))
			for i := 0; i < length; i++ {
				n, err := rand.Int(rand.Reader, charsetLen)
				if err != nil {
					return tool.Result{}, err
				}
				password[i] = charset[n.Int64()]
			}

			result := map[string]any{
				"password": string(password),
				"length":   length,
				"entropy":  float64(length) * 6.5, // approximate bits of entropy
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func tokenTool() tool.Tool {
	return tool.NewBuilder("random_token").
		WithDescription("Generate a secure random token").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Length   int    `json:"length,omitempty"`
				Encoding string `json:"encoding,omitempty"` // hex, base64, base64url
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			length := params.Length
			if length <= 0 {
				length = 32
			}
			if length > 256 {
				length = 256
			}

			bytes := make([]byte, length)
			_, err := rand.Read(bytes)
			if err != nil {
				return tool.Result{}, err
			}

			var token string
			switch strings.ToLower(params.Encoding) {
			case "base64":
				token = base64.StdEncoding.EncodeToString(bytes)
			case "base64url":
				token = base64.URLEncoding.EncodeToString(bytes)
			default:
				token = hex.EncodeToString(bytes)
			}

			result := map[string]any{
				"token":    token,
				"bytes":    length,
				"encoding": params.Encoding,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
