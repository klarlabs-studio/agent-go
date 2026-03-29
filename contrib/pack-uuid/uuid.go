// Package uuid provides UUID generation and parsing tools for agents.
package uuid

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/google/uuid"
)

// Pack returns the UUID tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("uuid").
		WithDescription("UUID generation and parsing tools").
		AddTools(
			generateV4Tool(),
			generateV7Tool(),
			parseTool(),
			validateTool(),
			versionTool(),
			formatTool(),
			compareTool(),
			batchGenerateTool(),
			fromStringTool(),
			nilUUIDTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func generateV4Tool() tool.Tool {
	return tool.NewBuilder("uuid_generate_v4").
		WithDescription("Generate a random UUID v4").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			id := uuid.New()

			result := map[string]any{
				"uuid":    id.String(),
				"version": 4,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateV7Tool() tool.Tool {
	return tool.NewBuilder("uuid_generate_v7").
		WithDescription("Generate a time-ordered UUID v7").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			id, err := uuid.NewV7()
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"uuid":    id.String(),
				"version": 7,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("uuid_parse").
		WithDescription("Parse a UUID string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UUID string `json:"uuid"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			id, err := uuid.Parse(params.UUID)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"valid":   true,
				"uuid":    id.String(),
				"version": id.Version(),
				"variant": id.Variant().String(),
				"urn":     id.URN(),
				"bytes":   id[:],
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("uuid_validate").
		WithDescription("Validate a UUID string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UUID    string `json:"uuid"`
				Version int    `json:"version,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			id, err := uuid.Parse(params.UUID)
			valid := err == nil
			errorMsg := ""

			if err != nil {
				errorMsg = err.Error()
			} else if params.Version != 0 && int(id.Version()) != params.Version {
				valid = false
				errorMsg = "version mismatch"
			}

			result := map[string]any{
				"uuid":  params.UUID,
				"valid": valid,
				"error": errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func versionTool() tool.Tool {
	return tool.NewBuilder("uuid_version").
		WithDescription("Get the version of a UUID").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UUID string `json:"uuid"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			id, err := uuid.Parse(params.UUID)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"uuid":    id.String(),
				"version": int(id.Version()),
				"variant": id.Variant().String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatTool() tool.Tool {
	return tool.NewBuilder("uuid_format").
		WithDescription("Format a UUID in different styles").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UUID   string `json:"uuid"`
				Format string `json:"format,omitempty"` // standard, upper, no-dashes, urn
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			id, err := uuid.Parse(params.UUID)
			if err != nil {
				return tool.Result{}, err
			}

			var formatted string
			switch strings.ToLower(params.Format) {
			case "upper":
				formatted = strings.ToUpper(id.String())
			case "no-dashes", "nodashes":
				formatted = strings.ReplaceAll(id.String(), "-", "")
			case "urn":
				formatted = id.URN()
			default:
				formatted = id.String()
			}

			result := map[string]any{
				"uuid":      id.String(),
				"formatted": formatted,
				"format":    params.Format,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compareTool() tool.Tool {
	return tool.NewBuilder("uuid_compare").
		WithDescription("Compare two UUIDs").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UUID1 string `json:"uuid1"`
				UUID2 string `json:"uuid2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			id1, err := uuid.Parse(params.UUID1)
			if err != nil {
				return tool.Result{}, err
			}

			id2, err := uuid.Parse(params.UUID2)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"uuid1": id1.String(),
				"uuid2": id2.String(),
				"equal": id1 == id2,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func batchGenerateTool() tool.Tool {
	return tool.NewBuilder("uuid_batch_generate").
		WithDescription("Generate multiple UUIDs").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Count   int `json:"count"`
				Version int `json:"version,omitempty"` // 4 or 7, defaults to 4
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			count := params.Count
			if count <= 0 {
				count = 1
			}
			if count > 1000 {
				count = 1000
			}

			version := params.Version
			if version == 0 {
				version = 4
			}

			uuids := make([]string, count)
			for i := 0; i < count; i++ {
				var id uuid.UUID
				var err error
				if version == 7 {
					id, err = uuid.NewV7()
					if err != nil {
						return tool.Result{}, err
					}
				} else {
					id = uuid.New()
				}
				uuids[i] = id.String()
			}

			result := map[string]any{
				"uuids":   uuids,
				"count":   count,
				"version": version,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fromStringTool() tool.Tool {
	return tool.NewBuilder("uuid_from_string").
		WithDescription("Create a deterministic UUID from a string (namespace UUID)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Namespace string `json:"namespace"` // dns, url, oid, x500, or a UUID
				Name      string `json:"name"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var namespace uuid.UUID
			switch strings.ToLower(params.Namespace) {
			case "dns":
				namespace = uuid.NameSpaceDNS
			case "url":
				namespace = uuid.NameSpaceURL
			case "oid":
				namespace = uuid.NameSpaceOID
			case "x500":
				namespace = uuid.NameSpaceX500
			default:
				var err error
				namespace, err = uuid.Parse(params.Namespace)
				if err != nil {
					return tool.Result{}, err
				}
			}

			id := uuid.NewSHA1(namespace, []byte(params.Name))

			result := map[string]any{
				"uuid":      id.String(),
				"namespace": namespace.String(),
				"name":      params.Name,
				"version":   5,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func nilUUIDTool() tool.Tool {
	return tool.NewBuilder("uuid_nil").
		WithDescription("Get the nil UUID or check if a UUID is nil").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UUID string `json:"uuid,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.UUID == "" {
				result := map[string]any{
					"nil_uuid": uuid.Nil.String(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			id, err := uuid.Parse(params.UUID)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"uuid":   id.String(),
				"is_nil": id == uuid.Nil,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
