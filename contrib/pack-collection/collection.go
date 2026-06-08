// Package collection provides collection/array utilities for agents.
package collection

import (
	"context"
	"encoding/json"
	"math/rand"
	"sort"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the collection utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("collection").
		WithDescription("Collection and array utilities").
		AddTools(
			uniqueTool(),
			flattenTool(),
			chunkTool(),
			shuffleTool(),
			sampleTool(),
			reverseTool(),
			sortTool(),
			groupByTool(),
			partitionTool(),
			intersectTool(),
			unionTool(),
			differenceTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func uniqueTool() tool.Tool {
	return tool.NewBuilder("collection_unique").
		WithDescription("Remove duplicate values from array").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []any `json:"items"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			seen := make(map[string]bool)
			unique := make([]any, 0)

			for _, item := range params.Items {
				// Convert to JSON for comparison
				key, _ := json.Marshal(item)
				keyStr := string(key)
				if !seen[keyStr] {
					seen[keyStr] = true
					unique = append(unique, item)
				}
			}

			result := map[string]any{
				"unique":             unique,
				"original_count":     len(params.Items),
				"unique_count":       len(unique),
				"duplicates_removed": len(params.Items) - len(unique),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func flattenTool() tool.Tool {
	return tool.NewBuilder("collection_flatten").
		WithDescription("Flatten nested arrays").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []any `json:"items"`
				Depth int   `json:"depth,omitempty"` // -1 for infinite
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			depth := params.Depth
			if depth == 0 {
				depth = 1
			}

			flattened := flattenArray(params.Items, depth)

			result := map[string]any{
				"flattened": flattened,
				"count":     len(flattened),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func flattenArray(items []any, depth int) []any {
	if depth == 0 {
		return items
	}

	var flattened []any
	for _, item := range items {
		if arr, ok := item.([]any); ok {
			if depth < 0 {
				flattened = append(flattened, flattenArray(arr, -1)...)
			} else {
				flattened = append(flattened, flattenArray(arr, depth-1)...)
			}
		} else {
			flattened = append(flattened, item)
		}
	}
	return flattened
}

func chunkTool() tool.Tool {
	return tool.NewBuilder("collection_chunk").
		WithDescription("Split array into chunks of specified size").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []any `json:"items"`
				Size  int   `json:"size"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Size <= 0 {
				result := map[string]any{"error": "chunk size must be positive"}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var chunks [][]any
			for i := 0; i < len(params.Items); i += params.Size {
				end := i + params.Size
				if end > len(params.Items) {
					end = len(params.Items)
				}
				chunks = append(chunks, params.Items[i:end])
			}

			result := map[string]any{
				"chunks":      chunks,
				"chunk_count": len(chunks),
				"chunk_size":  params.Size,
				"total_items": len(params.Items),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func shuffleTool() tool.Tool {
	return tool.NewBuilder("collection_shuffle").
		WithDescription("Randomly shuffle array").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []any `json:"items"`
				Seed  int64 `json:"seed,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Copy items to avoid modifying original
			shuffled := make([]any, len(params.Items))
			copy(shuffled, params.Items)

			var rng *rand.Rand
			if params.Seed != 0 {
				rng = rand.New(rand.NewSource(params.Seed))
			} else {
				rng = rand.New(rand.NewSource(time.Now().UnixNano()))
			}

			rng.Shuffle(len(shuffled), func(i, j int) {
				shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
			})

			result := map[string]any{
				"shuffled": shuffled,
				"count":    len(shuffled),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sampleTool() tool.Tool {
	return tool.NewBuilder("collection_sample").
		WithDescription("Get random sample from array").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []any `json:"items"`
				Count int   `json:"count,omitempty"`
				Seed  int64 `json:"seed,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			count := params.Count
			if count <= 0 {
				count = 1
			}
			if count > len(params.Items) {
				count = len(params.Items)
			}

			var rng *rand.Rand
			if params.Seed != 0 {
				rng = rand.New(rand.NewSource(params.Seed))
			} else {
				rng = rand.New(rand.NewSource(time.Now().UnixNano()))
			}

			// Fisher-Yates partial shuffle
			items := make([]any, len(params.Items))
			copy(items, params.Items)

			for i := 0; i < count; i++ {
				j := i + rng.Intn(len(items)-i)
				items[i], items[j] = items[j], items[i]
			}

			sample := items[:count]

			result := map[string]any{
				"sample":      sample,
				"sample_size": len(sample),
				"total_items": len(params.Items),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func reverseTool() tool.Tool {
	return tool.NewBuilder("collection_reverse").
		WithDescription("Reverse array order").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []any `json:"items"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			reversed := make([]any, len(params.Items))
			for i, item := range params.Items {
				reversed[len(params.Items)-1-i] = item
			}

			result := map[string]any{
				"reversed": reversed,
				"count":    len(reversed),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sortTool() tool.Tool {
	return tool.NewBuilder("collection_sort").
		WithDescription("Sort array").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items   []any  `json:"items"`
				Key     string `json:"key,omitempty"`     // For objects
				Reverse bool   `json:"reverse,omitempty"` // Descending
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sorted := make([]any, len(params.Items))
			copy(sorted, params.Items)

			sort.Slice(sorted, func(i, j int) bool {
				var a, b any
				if params.Key != "" {
					if m, ok := sorted[i].(map[string]any); ok {
						a = m[params.Key]
					}
					if m, ok := sorted[j].(map[string]any); ok {
						b = m[params.Key]
					}
				} else {
					a = sorted[i]
					b = sorted[j]
				}

				result := compareValues(a, b)
				if params.Reverse {
					return result > 0
				}
				return result < 0
			})

			result := map[string]any{
				"sorted": sorted,
				"count":  len(sorted),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compareValues(a, b any) int {
	// Try numeric comparison
	aFloat, aOk := toFloat(a)
	bFloat, bOk := toFloat(b)
	if aOk && bOk {
		if aFloat < bFloat {
			return -1
		} else if aFloat > bFloat {
			return 1
		}
		return 0
	}

	// String comparison
	aStr, _ := a.(string)
	bStr, _ := b.(string)
	if aStr < bStr {
		return -1
	} else if aStr > bStr {
		return 1
	}
	return 0
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func groupByTool() tool.Tool {
	return tool.NewBuilder("collection_group_by").
		WithDescription("Group array items by key").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items []map[string]any `json:"items"`
				Key   string           `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			groups := make(map[string][]map[string]any)

			for _, item := range params.Items {
				keyVal := ""
				if v, ok := item[params.Key]; ok {
					keyBytes, _ := json.Marshal(v)
					keyVal = string(keyBytes)
				}
				groups[keyVal] = append(groups[keyVal], item)
			}

			result := map[string]any{
				"groups":      groups,
				"group_count": len(groups),
				"total_items": len(params.Items),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func partitionTool() tool.Tool {
	return tool.NewBuilder("collection_partition").
		WithDescription("Partition array into two groups based on predicate").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items    []map[string]any `json:"items"`
				Key      string           `json:"key"`
				Value    any              `json:"value"`
				Operator string           `json:"operator,omitempty"` // eq, neq, gt, lt, gte, lte
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			op := params.Operator
			if op == "" {
				op = "eq"
			}

			var truthy, falsy []map[string]any

			for _, item := range params.Items {
				itemVal := item[params.Key]
				matches := false

				switch op {
				case "eq", "==":
					matches = compareAny(itemVal, params.Value) == 0
				case "neq", "!=":
					matches = compareAny(itemVal, params.Value) != 0
				case "gt", ">":
					matches = compareAny(itemVal, params.Value) > 0
				case "lt", "<":
					matches = compareAny(itemVal, params.Value) < 0
				case "gte", ">=":
					matches = compareAny(itemVal, params.Value) >= 0
				case "lte", "<=":
					matches = compareAny(itemVal, params.Value) <= 0
				}

				if matches {
					truthy = append(truthy, item)
				} else {
					falsy = append(falsy, item)
				}
			}

			result := map[string]any{
				"matching":           truthy,
				"not_matching":       falsy,
				"matching_count":     len(truthy),
				"not_matching_count": len(falsy),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compareAny(a, b any) int {
	// Try numeric
	aF, aOk := toFloat(a)
	bF, bOk := toFloat(b)
	if aOk && bOk {
		if aF < bF {
			return -1
		} else if aF > bF {
			return 1
		}
		return 0
	}

	// JSON comparison
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	aStr := string(aJSON)
	bStr := string(bJSON)

	if aStr < bStr {
		return -1
	} else if aStr > bStr {
		return 1
	}
	return 0
}

func intersectTool() tool.Tool {
	return tool.NewBuilder("collection_intersect").
		WithDescription("Find common items between arrays").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A []any `json:"a"`
				B []any `json:"b"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Build set from B
			bSet := make(map[string]bool)
			for _, item := range params.B {
				key, _ := json.Marshal(item)
				bSet[string(key)] = true
			}

			// Find intersection
			seen := make(map[string]bool)
			var intersection []any

			for _, item := range params.A {
				key, _ := json.Marshal(item)
				keyStr := string(key)
				if bSet[keyStr] && !seen[keyStr] {
					seen[keyStr] = true
					intersection = append(intersection, item)
				}
			}

			result := map[string]any{
				"intersection": intersection,
				"count":        len(intersection),
				"a_count":      len(params.A),
				"b_count":      len(params.B),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func unionTool() tool.Tool {
	return tool.NewBuilder("collection_union").
		WithDescription("Combine arrays removing duplicates").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A []any `json:"a"`
				B []any `json:"b"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			seen := make(map[string]bool)
			var union []any

			for _, item := range params.A {
				key, _ := json.Marshal(item)
				keyStr := string(key)
				if !seen[keyStr] {
					seen[keyStr] = true
					union = append(union, item)
				}
			}

			for _, item := range params.B {
				key, _ := json.Marshal(item)
				keyStr := string(key)
				if !seen[keyStr] {
					seen[keyStr] = true
					union = append(union, item)
				}
			}

			result := map[string]any{
				"union":   union,
				"count":   len(union),
				"a_count": len(params.A),
				"b_count": len(params.B),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func differenceTool() tool.Tool {
	return tool.NewBuilder("collection_difference").
		WithDescription("Find items in A that are not in B").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A []any `json:"a"`
				B []any `json:"b"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Build set from B
			bSet := make(map[string]bool)
			for _, item := range params.B {
				key, _ := json.Marshal(item)
				bSet[string(key)] = true
			}

			// Find difference
			seen := make(map[string]bool)
			var difference []any

			for _, item := range params.A {
				key, _ := json.Marshal(item)
				keyStr := string(key)
				if !bSet[keyStr] && !seen[keyStr] {
					seen[keyStr] = true
					difference = append(difference, item)
				}
			}

			result := map[string]any{
				"difference": difference,
				"count":      len(difference),
				"a_count":    len(params.A),
				"b_count":    len(params.B),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
