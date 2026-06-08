// Package mathutil provides math operation tools for agents.
package mathutil

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"sort"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the math utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("math").
		WithDescription("Math operation utilities").
		AddTools(
			basicTool(),
			roundTool(),
			absTool(),
			powerTool(),
			sqrtTool(),
			trigTool(),
			logTool(),
			minMaxTool(),
			clampTool(),
			randomTool(),
			statsTool(),
			percentTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func basicTool() tool.Tool {
	return tool.NewBuilder("math_basic").
		WithDescription("Basic arithmetic operations").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A  float64 `json:"a"`
				B  float64 `json:"b"`
				Op string  `json:"op"` // add, sub, mul, div, mod
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var value float64
			var err string

			switch params.Op {
			case "add", "+":
				value = params.A + params.B
			case "sub", "-":
				value = params.A - params.B
			case "mul", "*":
				value = params.A * params.B
			case "div", "/":
				if params.B == 0 {
					err = "division by zero"
				} else {
					value = params.A / params.B
				}
			case "mod", "%":
				if params.B == 0 {
					err = "modulo by zero"
				} else {
					value = math.Mod(params.A, params.B)
				}
			default:
				err = "unknown operation: " + params.Op
			}

			result := map[string]any{
				"result": value,
				"a":      params.A,
				"b":      params.B,
				"op":     params.Op,
			}
			if err != "" {
				result["error"] = err
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func roundTool() tool.Tool {
	return tool.NewBuilder("math_round").
		WithDescription("Round number to specified precision").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value     float64 `json:"value"`
				Precision int     `json:"precision,omitempty"`
				Mode      string  `json:"mode,omitempty"` // round, floor, ceil, trunc
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			multiplier := math.Pow(10, float64(params.Precision))
			value := params.Value * multiplier

			var rounded float64
			switch params.Mode {
			case "floor":
				rounded = math.Floor(value) / multiplier
			case "ceil":
				rounded = math.Ceil(value) / multiplier
			case "trunc":
				rounded = math.Trunc(value) / multiplier
			default: // round
				rounded = math.Round(value) / multiplier
			}

			result := map[string]any{
				"rounded":   rounded,
				"original":  params.Value,
				"precision": params.Precision,
				"mode":      params.Mode,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func absTool() tool.Tool {
	return tool.NewBuilder("math_abs").
		WithDescription("Absolute value").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			abs := math.Abs(params.Value)

			result := map[string]any{
				"absolute":     abs,
				"original":     params.Value,
				"was_negative": params.Value < 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func powerTool() tool.Tool {
	return tool.NewBuilder("math_power").
		WithDescription("Power and exponentiation").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Base     float64 `json:"base"`
				Exponent float64 `json:"exponent"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			power := math.Pow(params.Base, params.Exponent)

			result := map[string]any{
				"result":   power,
				"base":     params.Base,
				"exponent": params.Exponent,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sqrtTool() tool.Tool {
	return tool.NewBuilder("math_sqrt").
		WithDescription("Square root and nth root").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				N     float64 `json:"n,omitempty"` // nth root, default 2 (square root)
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n == 0 {
				n = 2
			}

			var root float64
			var err string

			switch {
			case params.Value < 0 && int(n)%2 == 0:
				err = "cannot compute even root of negative number"
			case params.Value < 0:
				// Odd root of negative number
				root = -math.Pow(-params.Value, 1/n)
			default:
				root = math.Pow(params.Value, 1/n)
			}

			result := map[string]any{
				"root":  root,
				"value": params.Value,
				"n":     n,
			}
			if err != "" {
				result["error"] = err
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func trigTool() tool.Tool {
	return tool.NewBuilder("math_trig").
		WithDescription("Trigonometric functions").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value   float64 `json:"value"`
				Func    string  `json:"func"` // sin, cos, tan, asin, acos, atan
				Degrees bool    `json:"degrees,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			value := params.Value
			if params.Degrees {
				value = value * math.Pi / 180
			}

			var computed float64
			var err string

			switch params.Func {
			case "sin":
				computed = math.Sin(value)
			case "cos":
				computed = math.Cos(value)
			case "tan":
				computed = math.Tan(value)
			case "asin":
				if params.Value < -1 || params.Value > 1 {
					err = "asin input must be between -1 and 1"
				} else {
					computed = math.Asin(params.Value)
					if params.Degrees {
						computed = computed * 180 / math.Pi
					}
				}
			case "acos":
				if params.Value < -1 || params.Value > 1 {
					err = "acos input must be between -1 and 1"
				} else {
					computed = math.Acos(params.Value)
					if params.Degrees {
						computed = computed * 180 / math.Pi
					}
				}
			case "atan":
				computed = math.Atan(params.Value)
				if params.Degrees {
					computed = computed * 180 / math.Pi
				}
			default:
				err = "unknown function: " + params.Func
			}

			result := map[string]any{
				"result":  computed,
				"func":    params.Func,
				"input":   params.Value,
				"degrees": params.Degrees,
			}
			if err != "" {
				result["error"] = err
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func logTool() tool.Tool {
	return tool.NewBuilder("math_log").
		WithDescription("Logarithmic functions").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				Base  float64 `json:"base,omitempty"` // default e (natural log)
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var logValue float64
			var err string
			base := params.Base

			switch {
			case params.Value <= 0:
				err = "logarithm of non-positive number is undefined"
			case base == 0 || base == math.E:
				logValue = math.Log(params.Value)
				base = math.E
			case base == 10:
				logValue = math.Log10(params.Value)
			case base == 2:
				logValue = math.Log2(params.Value)
			case base <= 0 || base == 1:
				err = "logarithm base must be positive and not equal to 1"
			default:
				logValue = math.Log(params.Value) / math.Log(base)
			}

			result := map[string]any{
				"result": logValue,
				"value":  params.Value,
				"base":   base,
			}
			if err != "" {
				result["error"] = err
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func minMaxTool() tool.Tool {
	return tool.NewBuilder("math_minmax").
		WithDescription("Find minimum or maximum of values").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) == 0 {
				result := map[string]any{
					"error": "empty values array",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			minVal := params.Values[0]
			maxVal := params.Values[0]
			minIdx := 0
			maxIdx := 0

			for i, v := range params.Values {
				if v < minVal {
					minVal = v
					minIdx = i
				}
				if v > maxVal {
					maxVal = v
					maxIdx = i
				}
			}

			result := map[string]any{
				"min":       minVal,
				"max":       maxVal,
				"min_index": minIdx,
				"max_index": maxIdx,
				"range":     maxVal - minVal,
				"count":     len(params.Values),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func clampTool() tool.Tool {
	return tool.NewBuilder("math_clamp").
		WithDescription("Clamp value between min and max").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				Min   float64 `json:"min"`
				Max   float64 `json:"max"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			clamped := params.Value
			wasClamped := false

			if clamped < params.Min {
				clamped = params.Min
				wasClamped = true
			} else if clamped > params.Max {
				clamped = params.Max
				wasClamped = true
			}

			result := map[string]any{
				"clamped":     clamped,
				"original":    params.Value,
				"min":         params.Min,
				"max":         params.Max,
				"was_clamped": wasClamped,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func randomTool() tool.Tool {
	return tool.NewBuilder("math_random").
		WithDescription("Generate random numbers").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Min     float64 `json:"min,omitempty"`
				Max     float64 `json:"max,omitempty"`
				Count   int     `json:"count,omitempty"`
				Integer bool    `json:"integer,omitempty"`
				Seed    int64   `json:"seed,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Default range 0-1
			min := params.Min
			max := params.Max
			if min == 0 && max == 0 {
				max = 1
			}

			count := params.Count
			if count <= 0 {
				count = 1
			}
			if count > 1000 {
				count = 1000
			}

			// Create random source
			var rng *rand.Rand
			if params.Seed != 0 {
				rng = rand.New(rand.NewSource(params.Seed))
			} else {
				rng = rand.New(rand.NewSource(time.Now().UnixNano()))
			}

			values := make([]float64, count)
			for i := range values {
				r := rng.Float64()
				value := min + r*(max-min)
				if params.Integer {
					value = math.Floor(value)
				}
				values[i] = value
			}

			var resultValue any
			if count == 1 {
				resultValue = values[0]
			} else {
				resultValue = values
			}

			result := map[string]any{
				"value": resultValue,
				"min":   min,
				"max":   max,
				"count": count,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func statsTool() tool.Tool {
	return tool.NewBuilder("math_stats").
		WithDescription("Statistical calculations").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) == 0 {
				result := map[string]any{
					"error": "empty values array",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			n := float64(len(params.Values))

			// Sum and mean
			sum := 0.0
			for _, v := range params.Values {
				sum += v
			}
			mean := sum / n

			// Variance and std dev
			variance := 0.0
			for _, v := range params.Values {
				diff := v - mean
				variance += diff * diff
			}
			variance /= n
			stdDev := math.Sqrt(variance)

			// Median
			sorted := make([]float64, len(params.Values))
			copy(sorted, params.Values)
			sort.Float64s(sorted)

			var median float64
			mid := len(sorted) / 2
			if len(sorted)%2 == 0 {
				median = (sorted[mid-1] + sorted[mid]) / 2
			} else {
				median = sorted[mid]
			}

			// Mode (most frequent)
			counts := make(map[float64]int)
			for _, v := range params.Values {
				counts[v]++
			}
			var mode float64
			maxCount := 0
			for v, c := range counts {
				if c > maxCount {
					maxCount = c
					mode = v
				}
			}

			result := map[string]any{
				"count":    len(params.Values),
				"sum":      sum,
				"mean":     mean,
				"median":   median,
				"mode":     mode,
				"variance": variance,
				"std_dev":  stdDev,
				"min":      sorted[0],
				"max":      sorted[len(sorted)-1],
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func percentTool() tool.Tool {
	return tool.NewBuilder("math_percent").
		WithDescription("Percentage calculations").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value   float64 `json:"value"`
				Total   float64 `json:"total,omitempty"`
				Percent float64 `json:"percent,omitempty"`
				Op      string  `json:"op,omitempty"` // of, change, from
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			result := make(map[string]any)

			switch params.Op {
			case "of":
				// What is X% of Y?
				result["result"] = params.Percent / 100 * params.Total
				result["description"] = "percent of total"
			case "change":
				// Percentage change from value to total
				if params.Value == 0 {
					result["error"] = "cannot calculate change from zero"
				} else {
					change := ((params.Total - params.Value) / params.Value) * 100
					result["result"] = change
					result["description"] = "percentage change"
				}
			case "from":
				// What percent is value of total?
				if params.Total == 0 {
					result["error"] = "cannot calculate percentage of zero"
				} else {
					pct := (params.Value / params.Total) * 100
					result["result"] = pct
					result["description"] = "value as percentage of total"
				}
			default:
				// Default: what percent is value of total
				if params.Total == 0 {
					result["error"] = "total cannot be zero"
				} else {
					pct := (params.Value / params.Total) * 100
					result["result"] = pct
					result["description"] = "value as percentage of total"
				}
			}

			result["value"] = params.Value
			result["total"] = params.Total
			result["percent"] = params.Percent

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
