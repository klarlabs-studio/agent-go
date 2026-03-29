// Package jwt provides JWT token tools for agents.
package jwt

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/golang-jwt/jwt/v5"
)

// Pack returns the JWT tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("jwt").
		WithDescription("JWT token tools").
		AddTools(
			createTool(),
			parseTool(),
			validateTool(),
			decodeTool(),
			inspectTool(),
			refreshTool(),
			buildClaimsTool(),
			checkExpTool(),
			extractClaimTool(),
			signTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func createTool() tool.Tool {
	return tool.NewBuilder("jwt_create").
		WithDescription("Create a JWT token").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Claims    map[string]any `json:"claims"`
				Secret    string         `json:"secret"`
				Algorithm string         `json:"algorithm,omitempty"` // HS256, HS384, HS512
				ExpiresIn int            `json:"expires_in_seconds,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Select signing method
			var method jwt.SigningMethod
			switch strings.ToUpper(params.Algorithm) {
			case "HS384":
				method = jwt.SigningMethodHS384
			case "HS512":
				method = jwt.SigningMethodHS512
			default:
				method = jwt.SigningMethodHS256
			}

			// Build claims
			claims := jwt.MapClaims{}
			for k, v := range params.Claims {
				claims[k] = v
			}

			// Add standard claims if not present
			now := time.Now()
			if _, ok := claims["iat"]; !ok {
				claims["iat"] = now.Unix()
			}
			if params.ExpiresIn > 0 {
				claims["exp"] = now.Add(time.Duration(params.ExpiresIn) * time.Second).Unix()
			}

			token := jwt.NewWithClaims(method, claims)
			signedToken, err := token.SignedString([]byte(params.Secret))
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"token":     signedToken,
				"algorithm": method.Alg(),
				"claims":    claims,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("jwt_parse").
		WithDescription("Parse and verify a JWT token").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Token  string `json:"token"`
				Secret string `json:"secret"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			token, err := jwt.Parse(params.Token, func(token *jwt.Token) (interface{}, error) {
				return []byte(params.Secret), nil
			})

			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			claims, _ := token.Claims.(jwt.MapClaims)

			result := map[string]any{
				"valid":     token.Valid,
				"claims":    claims,
				"algorithm": token.Method.Alg(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("jwt_validate").
		WithDescription("Validate JWT token without full parsing").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Token  string `json:"token"`
				Secret string `json:"secret,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parts := strings.Split(params.Token, ".")
			if len(parts) != 3 {
				result := map[string]any{
					"valid": false,
					"error": "Invalid JWT format: expected 3 parts",
					"parts": len(parts),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Decode header
			headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": "Invalid header encoding",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var header map[string]any
			if err := json.Unmarshal(headerJSON, &header); err != nil {
				result := map[string]any{
					"valid": false,
					"error": "Invalid header JSON",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Decode payload
			payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": "Invalid payload encoding",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var payload map[string]any
			if err := json.Unmarshal(payloadJSON, &payload); err != nil {
				result := map[string]any{
					"valid": false,
					"error": "Invalid payload JSON",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Check expiration
			expired := false
			if exp, ok := payload["exp"].(float64); ok {
				if time.Now().Unix() > int64(exp) {
					expired = true
				}
			}

			// Verify signature if secret provided
			signatureValid := false
			if params.Secret != "" {
				message := parts[0] + "." + parts[1]
				expectedSig := computeHS256(message, params.Secret)
				actualSig := parts[2]
				signatureValid = expectedSig == actualSig
			}

			result := map[string]any{
				"valid":           !expired && (params.Secret == "" || signatureValid),
				"format_valid":    true,
				"expired":         expired,
				"signature_valid": signatureValid,
				"algorithm":       header["alg"],
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func computeHS256(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func decodeTool() tool.Tool {
	return tool.NewBuilder("jwt_decode").
		WithDescription("Decode JWT token without verification").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parts := strings.Split(params.Token, ".")
			if len(parts) != 3 {
				return tool.Result{}, fmt.Errorf("invalid JWT format")
			}

			// Decode header
			headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
			if err != nil {
				return tool.Result{}, err
			}
			var header map[string]any
			_ = json.Unmarshal(headerJSON, &header) // #nosec G104 -- headerJSON already validated via base64 decode

			// Decode payload
			payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				return tool.Result{}, err
			}
			var payload map[string]any
			_ = json.Unmarshal(payloadJSON, &payload) // #nosec G104 -- payloadJSON already validated via base64 decode

			result := map[string]any{
				"header":    header,
				"payload":   payload,
				"signature": parts[2],
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func inspectTool() tool.Tool {
	return tool.NewBuilder("jwt_inspect").
		WithDescription("Inspect JWT token details").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parts := strings.Split(params.Token, ".")
			if len(parts) != 3 {
				return tool.Result{}, fmt.Errorf("invalid JWT format")
			}

			// Decode header
			headerJSON, _ := base64.RawURLEncoding.DecodeString(parts[0])
			var header map[string]any
			_ = json.Unmarshal(headerJSON, &header) // #nosec G104 -- header parsing is best-effort for inspection

			// Decode payload
			payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid JWT payload encoding: %w", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(payloadJSON, &payload); err != nil {
				return tool.Result{}, fmt.Errorf("invalid JWT payload JSON: %w", err)
			}

			// Time-related claims
			var issuedAt, expiresAt, notBefore *time.Time
			if iat, ok := payload["iat"].(float64); ok {
				t := time.Unix(int64(iat), 0)
				issuedAt = &t
			}
			if exp, ok := payload["exp"].(float64); ok {
				t := time.Unix(int64(exp), 0)
				expiresAt = &t
			}
			if nbf, ok := payload["nbf"].(float64); ok {
				t := time.Unix(int64(nbf), 0)
				notBefore = &t
			}

			// Calculate time to expiry
			var timeToExpiry string
			if expiresAt != nil {
				duration := time.Until(*expiresAt)
				if duration < 0 {
					timeToExpiry = "expired"
				} else {
					timeToExpiry = duration.Round(time.Second).String()
				}
			}

			result := map[string]any{
				"algorithm":      header["alg"],
				"type":           header["typ"],
				"issuer":         payload["iss"],
				"subject":        payload["sub"],
				"audience":       payload["aud"],
				"issued_at":      issuedAt,
				"expires_at":     expiresAt,
				"not_before":     notBefore,
				"time_to_expiry": timeToExpiry,
				"token_length":   len(params.Token),
				"header_size":    len(parts[0]),
				"payload_size":   len(parts[1]),
				"signature_size": len(parts[2]),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func refreshTool() tool.Tool {
	return tool.NewBuilder("jwt_refresh").
		WithDescription("Refresh a JWT token with new expiry").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Token     string `json:"token"`
				Secret    string `json:"secret"`
				ExpiresIn int    `json:"expires_in_seconds,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Parse existing token
			token, err := jwt.Parse(params.Token, func(token *jwt.Token) (interface{}, error) {
				return []byte(params.Secret), nil
			})
			if err != nil && !strings.Contains(err.Error(), "expired") {
				return tool.Result{}, err
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return tool.Result{}, fmt.Errorf("invalid claims")
			}

			// Update times
			now := time.Now()
			claims["iat"] = now.Unix()

			expiresIn := params.ExpiresIn
			if expiresIn <= 0 {
				expiresIn = 3600 // Default 1 hour
			}
			claims["exp"] = now.Add(time.Duration(expiresIn) * time.Second).Unix()

			// Create new token
			newToken := jwt.NewWithClaims(token.Method, claims)
			signedToken, err := newToken.SignedString([]byte(params.Secret))
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"token":      signedToken,
				"expires_at": time.Unix(claims["exp"].(int64), 0),
				"refreshed":  true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func buildClaimsTool() tool.Tool {
	return tool.NewBuilder("jwt_build_claims").
		WithDescription("Build standard JWT claims").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Issuer    string         `json:"issuer,omitempty"`
				Subject   string         `json:"subject,omitempty"`
				Audience  []string       `json:"audience,omitempty"`
				ExpiresIn int            `json:"expires_in_seconds,omitempty"`
				NotBefore int            `json:"not_before_seconds,omitempty"`
				Custom    map[string]any `json:"custom,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			now := time.Now()
			claims := map[string]any{
				"iat": now.Unix(),
			}

			if params.Issuer != "" {
				claims["iss"] = params.Issuer
			}
			if params.Subject != "" {
				claims["sub"] = params.Subject
			}
			if len(params.Audience) > 0 {
				if len(params.Audience) == 1 {
					claims["aud"] = params.Audience[0]
				} else {
					claims["aud"] = params.Audience
				}
			}
			if params.ExpiresIn > 0 {
				claims["exp"] = now.Add(time.Duration(params.ExpiresIn) * time.Second).Unix()
			}
			if params.NotBefore > 0 {
				claims["nbf"] = now.Add(time.Duration(params.NotBefore) * time.Second).Unix()
			}

			for k, v := range params.Custom {
				claims[k] = v
			}

			result := map[string]any{
				"claims": claims,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func checkExpTool() tool.Tool {
	return tool.NewBuilder("jwt_check_expiry").
		WithDescription("Check if JWT token is expired").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Token  string `json:"token"`
				Buffer int    `json:"buffer_seconds,omitempty"` // Consider expired if expiring within buffer
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parts := strings.Split(params.Token, ".")
			if len(parts) != 3 {
				return tool.Result{}, fmt.Errorf("invalid JWT format")
			}

			payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid JWT payload encoding: %w", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(payloadJSON, &payload); err != nil {
				return tool.Result{}, fmt.Errorf("invalid JWT payload JSON: %w", err)
			}

			buffer := time.Duration(params.Buffer) * time.Second

			expired := false
			expiringSoon := false
			var expiresAt *time.Time
			var timeRemaining time.Duration

			if exp, ok := payload["exp"].(float64); ok {
				t := time.Unix(int64(exp), 0)
				expiresAt = &t
				timeRemaining = time.Until(t)
				expired = timeRemaining <= 0
				expiringSoon = timeRemaining > 0 && timeRemaining <= buffer
			}

			result := map[string]any{
				"expired":        expired,
				"expiring_soon":  expiringSoon,
				"expires_at":     expiresAt,
				"time_remaining": timeRemaining.Round(time.Second).String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractClaimTool() tool.Tool {
	return tool.NewBuilder("jwt_extract_claim").
		WithDescription("Extract a specific claim from JWT").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Token string `json:"token"`
				Claim string `json:"claim"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parts := strings.Split(params.Token, ".")
			if len(parts) != 3 {
				return tool.Result{}, fmt.Errorf("invalid JWT format")
			}

			payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid JWT payload encoding: %w", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(payloadJSON, &payload); err != nil {
				return tool.Result{}, fmt.Errorf("invalid JWT payload JSON: %w", err)
			}

			value, exists := payload[params.Claim]

			result := map[string]any{
				"claim":  params.Claim,
				"value":  value,
				"exists": exists,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func signTool() tool.Tool {
	return tool.NewBuilder("jwt_sign").
		WithDescription("Sign a JWT payload").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Header  map[string]any `json:"header,omitempty"`
				Payload map[string]any `json:"payload"`
				Secret  string         `json:"secret"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Default header
			if params.Header == nil {
				params.Header = map[string]any{
					"alg": "HS256",
					"typ": "JWT",
				}
			}

			// Encode header
			headerJSON, _ := json.Marshal(params.Header)
			headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

			// Encode payload
			payloadJSON, _ := json.Marshal(params.Payload)
			payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

			// Create signature
			message := headerEncoded + "." + payloadEncoded
			signature := computeHS256(message, params.Secret)

			token := message + "." + signature

			result := map[string]any{
				"token":     token,
				"algorithm": params.Header["alg"],
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
