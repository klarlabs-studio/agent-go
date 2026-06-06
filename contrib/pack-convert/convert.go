// Package convert provides unit and format conversion tools for agents.
package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the conversion tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("convert").
		WithDescription("Unit and format conversion utilities").
		AddTools(
			temperatureTool(),
			lengthTool(),
			weightTool(),
			volumeTool(),
			areaTool(),
			speedTool(),
			dataSizeTool(),
			timeDurationTool(),
			angleTool(),
			pressureTool(),
			energyTool(),
			numberBaseTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func temperatureTool() tool.Tool {
	return tool.NewBuilder("convert_temperature").
		WithDescription("Convert between temperature units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"` // celsius, fahrenheit, kelvin
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Convert to Celsius first
			var celsius float64
			switch strings.ToLower(params.From) {
			case "celsius", "c":
				celsius = params.Value
			case "fahrenheit", "f":
				celsius = (params.Value - 32) * 5 / 9
			case "kelvin", "k":
				celsius = params.Value - 273.15
			default:
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Convert from Celsius to target
			var converted float64
			switch strings.ToLower(params.To) {
			case "celsius", "c":
				converted = celsius
			case "fahrenheit", "f":
				converted = celsius*9/5 + 32
			case "kelvin", "k":
				converted = celsius + 273.15
			default:
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lengthTool() tool.Tool {
	// Base unit: meters
	units := map[string]float64{
		"mm":         0.001,
		"millimeter": 0.001,
		"cm":         0.01,
		"centimeter": 0.01,
		"m":          1,
		"meter":      1,
		"km":         1000,
		"kilometer":  1000,
		"in":         0.0254,
		"inch":       0.0254,
		"ft":         0.3048,
		"foot":       0.3048,
		"feet":       0.3048,
		"yd":         0.9144,
		"yard":       0.9144,
		"mi":         1609.344,
		"mile":       1609.344,
		"nm":         1852,
		"nautical":   1852,
	}

	return tool.NewBuilder("convert_length").
		WithDescription("Convert between length units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			meters := params.Value * fromFactor
			converted := meters / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func weightTool() tool.Tool {
	// Base unit: kilograms
	units := map[string]float64{
		"mg":        0.000001,
		"milligram": 0.000001,
		"g":         0.001,
		"gram":      0.001,
		"kg":        1,
		"kilogram":  1,
		"t":         1000,
		"tonne":     1000,
		"oz":        0.0283495,
		"ounce":     0.0283495,
		"lb":        0.453592,
		"pound":     0.453592,
		"st":        6.35029,
		"stone":     6.35029,
		"ton":       907.185, // US short ton
	}

	return tool.NewBuilder("convert_weight").
		WithDescription("Convert between weight/mass units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			kg := params.Value * fromFactor
			converted := kg / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func volumeTool() tool.Tool {
	// Base unit: liters
	units := map[string]float64{
		"ml":         0.001,
		"milliliter": 0.001,
		"l":          1,
		"liter":      1,
		"litre":      1,
		"m3":         1000,
		"gal":        3.78541, // US gallon
		"gallon":     3.78541,
		"qt":         0.946353,
		"quart":      0.946353,
		"pt":         0.473176,
		"pint":       0.473176,
		"cup":        0.236588,
		"floz":       0.0295735, // US fluid ounce
		"tbsp":       0.0147868,
		"tsp":        0.00492892,
	}

	return tool.NewBuilder("convert_volume").
		WithDescription("Convert between volume units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			liters := params.Value * fromFactor
			converted := liters / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func areaTool() tool.Tool {
	// Base unit: square meters
	units := map[string]float64{
		"mm2":     0.000001,
		"cm2":     0.0001,
		"m2":      1,
		"km2":     1000000,
		"ha":      10000,
		"hectare": 10000,
		"in2":     0.00064516,
		"ft2":     0.092903,
		"yd2":     0.836127,
		"ac":      4046.86,
		"acre":    4046.86,
		"mi2":     2589988.11,
	}

	return tool.NewBuilder("convert_area").
		WithDescription("Convert between area units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			sqm := params.Value * fromFactor
			converted := sqm / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func speedTool() tool.Tool {
	// Base unit: meters per second
	units := map[string]float64{
		"mps":   1,
		"m/s":   1,
		"kmh":   0.277778,
		"km/h":  0.277778,
		"kph":   0.277778,
		"mph":   0.44704,
		"mi/h":  0.44704,
		"fps":   0.3048,
		"ft/s":  0.3048,
		"knot":  0.514444,
		"knots": 0.514444,
		"mach":  343, // At sea level, 20°C
	}

	return tool.NewBuilder("convert_speed").
		WithDescription("Convert between speed units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			mps := params.Value * fromFactor
			converted := mps / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func dataSizeTool() tool.Tool {
	// Base unit: bytes
	units := map[string]float64{
		"b":    1,
		"byte": 1,
		"kb":   1024,
		"kib":  1024,
		"mb":   1024 * 1024,
		"mib":  1024 * 1024,
		"gb":   1024 * 1024 * 1024,
		"gib":  1024 * 1024 * 1024,
		"tb":   1024 * 1024 * 1024 * 1024,
		"tib":  1024 * 1024 * 1024 * 1024,
		"pb":   1024 * 1024 * 1024 * 1024 * 1024,
		"pib":  1024 * 1024 * 1024 * 1024 * 1024,
		// Decimal units
		"kbyte": 1000,
		"mbyte": 1000 * 1000,
		"gbyte": 1000 * 1000 * 1000,
		"tbyte": 1000 * 1000 * 1000 * 1000,
	}

	return tool.NewBuilder("convert_data_size").
		WithDescription("Convert between data size units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			bytes := params.Value * fromFactor
			converted := bytes / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
				"bytes":     bytes,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func timeDurationTool() tool.Tool {
	// Base unit: seconds
	units := map[string]float64{
		"ns":          0.000000001,
		"nanosecond":  0.000000001,
		"us":          0.000001,
		"microsecond": 0.000001,
		"ms":          0.001,
		"millisecond": 0.001,
		"s":           1,
		"second":      1,
		"min":         60,
		"minute":      60,
		"h":           3600,
		"hour":        3600,
		"d":           86400,
		"day":         86400,
		"w":           604800,
		"week":        604800,
		"mo":          2592000, // 30 days
		"month":       2592000,
		"y":           31536000, // 365 days
		"year":        31536000,
	}

	return tool.NewBuilder("convert_time_duration").
		WithDescription("Convert between time duration units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			seconds := params.Value * fromFactor
			converted := seconds / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
				"seconds":   seconds,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func angleTool() tool.Tool {
	// Base unit: radians
	units := map[string]float64{
		"rad":     1,
		"radian":  1,
		"deg":     math.Pi / 180,
		"degree":  math.Pi / 180,
		"grad":    math.Pi / 200,
		"gradian": math.Pi / 200,
		"turn":    2 * math.Pi,
		"rev":     2 * math.Pi,
	}

	return tool.NewBuilder("convert_angle").
		WithDescription("Convert between angle units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			radians := params.Value * fromFactor
			converted := radians / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pressureTool() tool.Tool {
	// Base unit: pascals
	units := map[string]float64{
		"pa":     1,
		"pascal": 1,
		"kpa":    1000,
		"mpa":    1000000,
		"bar":    100000,
		"mbar":   100,
		"atm":    101325,
		"psi":    6894.76,
		"mmhg":   133.322,
		"torr":   133.322,
		"inhg":   3386.39,
	}

	return tool.NewBuilder("convert_pressure").
		WithDescription("Convert between pressure units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			pascals := params.Value * fromFactor
			converted := pascals / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func energyTool() tool.Tool {
	// Base unit: joules
	units := map[string]float64{
		"j":       1,
		"joule":   1,
		"kj":      1000,
		"mj":      1000000,
		"cal":     4.184,
		"calorie": 4.184,
		"kcal":    4184,
		"wh":      3600,
		"kwh":     3600000,
		"ev":      1.60218e-19,
		"btu":     1055.06,
		"ft-lb":   1.35582,
	}

	return tool.NewBuilder("convert_energy").
		WithDescription("Convert between energy units").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromFactor, ok := units[strings.ToLower(params.From)]
			if !ok {
				result := map[string]any{"error": "unknown source unit: " + params.From}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			toFactor, ok := units[strings.ToLower(params.To)]
			if !ok {
				result := map[string]any{"error": "unknown target unit: " + params.To}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			joules := params.Value * fromFactor
			converted := joules / toFactor

			result := map[string]any{
				"value":     params.Value,
				"from":      params.From,
				"to":        params.To,
				"converted": converted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func numberBaseTool() tool.Tool {
	return tool.NewBuilder("convert_number_base").
		WithDescription("Convert between number bases (binary, octal, decimal, hex)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Value    string `json:"value"`
				FromBase int    `json:"from_base,omitempty"` // default 10
				ToBase   int    `json:"to_base,omitempty"`   // default 10
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fromBase := params.FromBase
			if fromBase == 0 {
				fromBase = 10
			}
			toBase := params.ToBase
			if toBase == 0 {
				toBase = 10
			}

			if fromBase < 2 || fromBase > 36 || toBase < 2 || toBase > 36 {
				result := map[string]any{"error": "base must be between 2 and 36"}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Parse the input
			value := strings.TrimSpace(params.Value)
			value = strings.TrimPrefix(value, "0x")
			value = strings.TrimPrefix(value, "0X")
			value = strings.TrimPrefix(value, "0b")
			value = strings.TrimPrefix(value, "0B")
			value = strings.TrimPrefix(value, "0o")
			value = strings.TrimPrefix(value, "0O")

			num, err := strconv.ParseInt(value, fromBase, 64)
			if err != nil {
				result := map[string]any{"error": fmt.Sprintf("invalid number for base %d: %s", fromBase, params.Value)}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			converted := strconv.FormatInt(num, toBase)
			if toBase == 16 {
				converted = strings.ToUpper(converted)
			}

			result := map[string]any{
				"value":     params.Value,
				"from_base": fromBase,
				"to_base":   toBase,
				"converted": converted,
				"decimal":   num,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
