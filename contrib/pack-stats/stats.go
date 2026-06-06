// Package stats provides statistical analysis tools for agents.
// It enables computation of descriptive statistics, distributions,
// correlation analysis, and hypothesis testing.
package stats

import (
	"context"
	"encoding/json"
	"math"
	"sort"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the stats tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("stats").
		WithDescription("Statistical analysis tools").
		AddTools(
			meanTool(),
			medianTool(),
			modeTool(),
			stdDevTool(),
			varianceTool(),
			minTool(),
			maxTool(),
			sumTool(),
			rangeTool(),
			percentileTool(),
			quartilesTool(),
			iqrTool(),
			skewnessTool(),
			kurtosisTool(),
			correlationTool(),
			covarianceTool(),
			zScoreTool(),
			normalizeTool(),
			histogramTool(),
			describeTool(),
			outliersTool(),
			movingAverageTool(),
			exponentialMATool(),
			linearRegressionTool(),
			tTestTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func meanTool() tool.Tool {
	return tool.NewBuilder("stats_mean").
		WithDescription("Calculate arithmetic mean of values").
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
				return tool.Result{Output: json.RawMessage(`{"mean": null, "error": "empty values"}`)}, nil
			}

			sum := 0.0
			for _, v := range params.Values {
				sum += v
			}
			mean := sum / float64(len(params.Values))

			result := map[string]any{"mean": mean, "count": len(params.Values)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func medianTool() tool.Tool {
	return tool.NewBuilder("stats_median").
		WithDescription("Calculate median of values").
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
				return tool.Result{Output: json.RawMessage(`{"median": null}`)}, nil
			}

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

			result := map[string]any{"median": median}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func modeTool() tool.Tool {
	return tool.NewBuilder("stats_mode").
		WithDescription("Calculate mode (most frequent value) of values").
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
				return tool.Result{Output: json.RawMessage(`{"mode": null}`)}, nil
			}

			freq := make(map[float64]int)
			for _, v := range params.Values {
				freq[v]++
			}

			var mode float64
			maxFreq := 0
			for v, f := range freq {
				if f > maxFreq {
					maxFreq = f
					mode = v
				}
			}

			result := map[string]any{"mode": mode, "frequency": maxFreq}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func stdDevTool() tool.Tool {
	return tool.NewBuilder("stats_std_dev").
		WithDescription("Calculate standard deviation of values").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
				Sample bool      `json:"sample,omitempty"` // Use sample std dev (n-1)
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) < 2 {
				return tool.Result{Output: json.RawMessage(`{"std_dev": null}`)}, nil
			}

			mean := calcMean(params.Values)
			variance := calcVariance(params.Values, mean, params.Sample)
			stdDev := math.Sqrt(variance)

			result := map[string]any{
				"std_dev":  stdDev,
				"variance": variance,
				"mean":     mean,
				"sample":   params.Sample,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func varianceTool() tool.Tool {
	return tool.NewBuilder("stats_variance").
		WithDescription("Calculate variance of values").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
				Sample bool      `json:"sample,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) < 2 {
				return tool.Result{Output: json.RawMessage(`{"variance": null}`)}, nil
			}

			mean := calcMean(params.Values)
			variance := calcVariance(params.Values, mean, params.Sample)

			result := map[string]any{"variance": variance, "mean": mean}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func minTool() tool.Tool {
	return tool.NewBuilder("stats_min").
		WithDescription("Find minimum value").
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
				return tool.Result{Output: json.RawMessage(`{"min": null}`)}, nil
			}

			min := params.Values[0]
			minIdx := 0
			for i, v := range params.Values[1:] {
				if v < min {
					min = v
					minIdx = i + 1
				}
			}

			result := map[string]any{"min": min, "index": minIdx}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func maxTool() tool.Tool {
	return tool.NewBuilder("stats_max").
		WithDescription("Find maximum value").
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
				return tool.Result{Output: json.RawMessage(`{"max": null}`)}, nil
			}

			max := params.Values[0]
			maxIdx := 0
			for i, v := range params.Values[1:] {
				if v > max {
					max = v
					maxIdx = i + 1
				}
			}

			result := map[string]any{"max": max, "index": maxIdx}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sumTool() tool.Tool {
	return tool.NewBuilder("stats_sum").
		WithDescription("Calculate sum of values").
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

			sum := 0.0
			for _, v := range params.Values {
				sum += v
			}

			result := map[string]any{"sum": sum, "count": len(params.Values)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func rangeTool() tool.Tool {
	return tool.NewBuilder("stats_range").
		WithDescription("Calculate range (max - min) of values").
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
				return tool.Result{Output: json.RawMessage(`{"range": null}`)}, nil
			}

			min, max := params.Values[0], params.Values[0]
			for _, v := range params.Values[1:] {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}

			result := map[string]any{"range": max - min, "min": min, "max": max}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func percentileTool() tool.Tool {
	return tool.NewBuilder("stats_percentile").
		WithDescription("Calculate percentile of values").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values     []float64 `json:"values"`
				Percentile float64   `json:"percentile"` // 0-100
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) == 0 {
				return tool.Result{Output: json.RawMessage(`{"value": null}`)}, nil
			}

			sorted := make([]float64, len(params.Values))
			copy(sorted, params.Values)
			sort.Float64s(sorted)

			p := params.Percentile / 100.0
			idx := p * float64(len(sorted)-1)
			lower := int(math.Floor(idx))
			upper := int(math.Ceil(idx))

			var value float64
			if lower == upper {
				value = sorted[lower]
			} else {
				value = sorted[lower] + (idx-float64(lower))*(sorted[upper]-sorted[lower])
			}

			result := map[string]any{"value": value, "percentile": params.Percentile}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func quartilesTool() tool.Tool {
	return tool.NewBuilder("stats_quartiles").
		WithDescription("Calculate quartiles (Q1, Q2, Q3) of values").
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
				return tool.Result{Output: json.RawMessage(`{"q1": null, "q2": null, "q3": null}`)}, nil
			}

			sorted := make([]float64, len(params.Values))
			copy(sorted, params.Values)
			sort.Float64s(sorted)

			q1 := calcPercentile(sorted, 25)
			q2 := calcPercentile(sorted, 50)
			q3 := calcPercentile(sorted, 75)

			result := map[string]any{
				"q1":  q1,
				"q2":  q2,
				"q3":  q3,
				"iqr": q3 - q1,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func iqrTool() tool.Tool {
	return tool.NewBuilder("stats_iqr").
		WithDescription("Calculate interquartile range (Q3 - Q1)").
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
				return tool.Result{Output: json.RawMessage(`{"iqr": null}`)}, nil
			}

			sorted := make([]float64, len(params.Values))
			copy(sorted, params.Values)
			sort.Float64s(sorted)

			q1 := calcPercentile(sorted, 25)
			q3 := calcPercentile(sorted, 75)
			iqr := q3 - q1

			result := map[string]any{"iqr": iqr, "q1": q1, "q3": q3}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func skewnessTool() tool.Tool {
	return tool.NewBuilder("stats_skewness").
		WithDescription("Calculate skewness (asymmetry) of distribution").
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

			if len(params.Values) < 3 {
				return tool.Result{Output: json.RawMessage(`{"skewness": null}`)}, nil
			}

			mean := calcMean(params.Values)
			stdDev := math.Sqrt(calcVariance(params.Values, mean, false))

			if stdDev == 0 {
				return tool.Result{Output: json.RawMessage(`{"skewness": 0}`)}, nil
			}

			n := float64(len(params.Values))
			sum := 0.0
			for _, v := range params.Values {
				sum += math.Pow((v-mean)/stdDev, 3)
			}
			skewness := (n / ((n - 1) * (n - 2))) * sum

			result := map[string]any{"skewness": skewness}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func kurtosisTool() tool.Tool {
	return tool.NewBuilder("stats_kurtosis").
		WithDescription("Calculate kurtosis (tailedness) of distribution").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
				Excess bool      `json:"excess,omitempty"` // Return excess kurtosis (subtract 3)
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) < 4 {
				return tool.Result{Output: json.RawMessage(`{"kurtosis": null}`)}, nil
			}

			mean := calcMean(params.Values)
			stdDev := math.Sqrt(calcVariance(params.Values, mean, false))

			if stdDev == 0 {
				return tool.Result{Output: json.RawMessage(`{"kurtosis": null}`)}, nil
			}

			n := float64(len(params.Values))
			sum := 0.0
			for _, v := range params.Values {
				sum += math.Pow((v-mean)/stdDev, 4)
			}
			kurtosis := ((n * (n + 1)) / ((n - 1) * (n - 2) * (n - 3))) * sum

			if params.Excess {
				kurtosis -= (3 * (n - 1) * (n - 1)) / ((n - 2) * (n - 3))
			}

			result := map[string]any{"kurtosis": kurtosis, "excess": params.Excess}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func correlationTool() tool.Tool {
	return tool.NewBuilder("stats_correlation").
		WithDescription("Calculate Pearson correlation coefficient between two variables").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				X []float64 `json:"x"`
				Y []float64 `json:"y"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.X) != len(params.Y) || len(params.X) < 2 {
				return tool.Result{Output: json.RawMessage(`{"correlation": null, "error": "arrays must have same length >= 2"}`)}, nil
			}

			meanX := calcMean(params.X)
			meanY := calcMean(params.Y)

			var sumXY, sumX2, sumY2 float64
			for i := range params.X {
				dx := params.X[i] - meanX
				dy := params.Y[i] - meanY
				sumXY += dx * dy
				sumX2 += dx * dx
				sumY2 += dy * dy
			}

			if sumX2 == 0 || sumY2 == 0 {
				return tool.Result{Output: json.RawMessage(`{"correlation": null}`)}, nil
			}

			correlation := sumXY / math.Sqrt(sumX2*sumY2)

			result := map[string]any{
				"correlation": correlation,
				"r_squared":   correlation * correlation,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func covarianceTool() tool.Tool {
	return tool.NewBuilder("stats_covariance").
		WithDescription("Calculate covariance between two variables").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				X      []float64 `json:"x"`
				Y      []float64 `json:"y"`
				Sample bool      `json:"sample,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.X) != len(params.Y) || len(params.X) < 2 {
				return tool.Result{Output: json.RawMessage(`{"covariance": null}`)}, nil
			}

			meanX := calcMean(params.X)
			meanY := calcMean(params.Y)

			sum := 0.0
			for i := range params.X {
				sum += (params.X[i] - meanX) * (params.Y[i] - meanY)
			}

			n := float64(len(params.X))
			if params.Sample {
				n -= 1
			}
			covariance := sum / n

			result := map[string]any{"covariance": covariance}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func zScoreTool() tool.Tool {
	return tool.NewBuilder("stats_z_score").
		WithDescription("Calculate z-scores (standard scores) for values").
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

			if len(params.Values) < 2 {
				return tool.Result{Output: json.RawMessage(`{"z_scores": null}`)}, nil
			}

			mean := calcMean(params.Values)
			stdDev := math.Sqrt(calcVariance(params.Values, mean, false))

			if stdDev == 0 {
				zScores := make([]float64, len(params.Values))
				result := map[string]any{"z_scores": zScores, "mean": mean, "std_dev": 0}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			zScores := make([]float64, len(params.Values))
			for i, v := range params.Values {
				zScores[i] = (v - mean) / stdDev
			}

			result := map[string]any{"z_scores": zScores, "mean": mean, "std_dev": stdDev}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func normalizeTool() tool.Tool {
	return tool.NewBuilder("stats_normalize").
		WithDescription("Normalize values to 0-1 range (min-max scaling)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
				Min    *float64  `json:"min,omitempty"`
				Max    *float64  `json:"max,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) == 0 {
				return tool.Result{Output: json.RawMessage(`{"normalized": []}`)}, nil
			}

			min, max := params.Values[0], params.Values[0]
			for _, v := range params.Values[1:] {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}

			if params.Min != nil {
				min = *params.Min
			}
			if params.Max != nil {
				max = *params.Max
			}

			rangeVal := max - min
			if rangeVal == 0 {
				normalized := make([]float64, len(params.Values))
				for i := range normalized {
					normalized[i] = 0.5
				}
				result := map[string]any{"normalized": normalized, "min": min, "max": max}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			normalized := make([]float64, len(params.Values))
			for i, v := range params.Values {
				normalized[i] = (v - min) / rangeVal
			}

			result := map[string]any{"normalized": normalized, "min": min, "max": max}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func histogramTool() tool.Tool {
	return tool.NewBuilder("stats_histogram").
		WithDescription("Create histogram bins for values").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
				Bins   int       `json:"bins,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) == 0 {
				return tool.Result{Output: json.RawMessage(`{"bins": [], "counts": []}`)}, nil
			}

			if params.Bins <= 0 {
				params.Bins = int(math.Ceil(math.Sqrt(float64(len(params.Values)))))
				if params.Bins < 5 {
					params.Bins = 5
				}
			}

			min, max := params.Values[0], params.Values[0]
			for _, v := range params.Values[1:] {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}

			binWidth := (max - min) / float64(params.Bins)
			if binWidth == 0 {
				binWidth = 1
			}

			counts := make([]int, params.Bins)
			edges := make([]float64, params.Bins+1)
			for i := 0; i <= params.Bins; i++ {
				edges[i] = min + float64(i)*binWidth
			}

			for _, v := range params.Values {
				bin := int((v - min) / binWidth)
				if bin >= params.Bins {
					bin = params.Bins - 1
				}
				if bin < 0 {
					bin = 0
				}
				counts[bin]++
			}

			result := map[string]any{
				"counts":    counts,
				"edges":     edges,
				"bin_width": binWidth,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func describeTool() tool.Tool {
	return tool.NewBuilder("stats_describe").
		WithDescription("Generate comprehensive descriptive statistics").
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
				return tool.Result{Output: json.RawMessage(`{"error": "empty values"}`)}, nil
			}

			sorted := make([]float64, len(params.Values))
			copy(sorted, params.Values)
			sort.Float64s(sorted)

			mean := calcMean(params.Values)
			variance := calcVariance(params.Values, mean, true)
			stdDev := math.Sqrt(variance)

			result := map[string]any{
				"count":    len(params.Values),
				"mean":     mean,
				"std_dev":  stdDev,
				"variance": variance,
				"min":      sorted[0],
				"max":      sorted[len(sorted)-1],
				"range":    sorted[len(sorted)-1] - sorted[0],
				"q1":       calcPercentile(sorted, 25),
				"median":   calcPercentile(sorted, 50),
				"q3":       calcPercentile(sorted, 75),
				"iqr":      calcPercentile(sorted, 75) - calcPercentile(sorted, 25),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func outliersTool() tool.Tool {
	return tool.NewBuilder("stats_outliers").
		WithDescription("Detect outliers using IQR or z-score method").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values    []float64 `json:"values"`
				Method    string    `json:"method,omitempty"` // iqr, zscore
				Threshold float64   `json:"threshold,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) < 4 {
				return tool.Result{Output: json.RawMessage(`{"outliers": [], "indices": []}`)}, nil
			}

			if params.Method == "" {
				params.Method = "iqr"
			}

			var outliers []float64
			var indices []int

			if params.Method == "zscore" {
				if params.Threshold == 0 {
					params.Threshold = 3.0
				}
				mean := calcMean(params.Values)
				stdDev := math.Sqrt(calcVariance(params.Values, mean, false))

				for i, v := range params.Values {
					z := math.Abs((v - mean) / stdDev)
					if z > params.Threshold {
						outliers = append(outliers, v)
						indices = append(indices, i)
					}
				}
			} else {
				if params.Threshold == 0 {
					params.Threshold = 1.5
				}
				sorted := make([]float64, len(params.Values))
				copy(sorted, params.Values)
				sort.Float64s(sorted)

				q1 := calcPercentile(sorted, 25)
				q3 := calcPercentile(sorted, 75)
				iqr := q3 - q1
				lowerBound := q1 - params.Threshold*iqr
				upperBound := q3 + params.Threshold*iqr

				for i, v := range params.Values {
					if v < lowerBound || v > upperBound {
						outliers = append(outliers, v)
						indices = append(indices, i)
					}
				}
			}

			result := map[string]any{
				"outliers": outliers,
				"indices":  indices,
				"count":    len(outliers),
				"method":   params.Method,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func movingAverageTool() tool.Tool {
	return tool.NewBuilder("stats_moving_average").
		WithDescription("Calculate simple moving average").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
				Window int       `json:"window"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Window <= 0 || params.Window > len(params.Values) {
				return tool.Result{Output: json.RawMessage(`{"error": "invalid window size"}`)}, nil
			}

			ma := make([]float64, len(params.Values)-params.Window+1)
			windowSum := 0.0

			for i := 0; i < params.Window; i++ {
				windowSum += params.Values[i]
			}
			ma[0] = windowSum / float64(params.Window)

			for i := params.Window; i < len(params.Values); i++ {
				windowSum = windowSum - params.Values[i-params.Window] + params.Values[i]
				ma[i-params.Window+1] = windowSum / float64(params.Window)
			}

			result := map[string]any{"moving_average": ma, "window": params.Window}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func exponentialMATool() tool.Tool {
	return tool.NewBuilder("stats_ema").
		WithDescription("Calculate exponential moving average").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values []float64 `json:"values"`
				Alpha  float64   `json:"alpha,omitempty"` // Smoothing factor (0-1)
				Span   int       `json:"span,omitempty"`  // Alternative: span for calculating alpha
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Values) == 0 {
				return tool.Result{Output: json.RawMessage(`{"ema": []}`)}, nil
			}

			alpha := params.Alpha
			if alpha == 0 && params.Span > 0 {
				alpha = 2.0 / (float64(params.Span) + 1)
			}
			if alpha == 0 {
				alpha = 0.3
			}

			ema := make([]float64, len(params.Values))
			ema[0] = params.Values[0]

			for i := 1; i < len(params.Values); i++ {
				ema[i] = alpha*params.Values[i] + (1-alpha)*ema[i-1]
			}

			result := map[string]any{"ema": ema, "alpha": alpha}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func linearRegressionTool() tool.Tool {
	return tool.NewBuilder("stats_linear_regression").
		WithDescription("Perform simple linear regression").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				X []float64 `json:"x"`
				Y []float64 `json:"y"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.X) != len(params.Y) || len(params.X) < 2 {
				return tool.Result{Output: json.RawMessage(`{"error": "arrays must have same length >= 2"}`)}, nil
			}

			n := float64(len(params.X))
			meanX := calcMean(params.X)
			meanY := calcMean(params.Y)

			var sumXY, sumX2 float64
			for i := range params.X {
				dx := params.X[i] - meanX
				sumXY += dx * (params.Y[i] - meanY)
				sumX2 += dx * dx
			}

			if sumX2 == 0 {
				return tool.Result{Output: json.RawMessage(`{"error": "x values have no variance"}`)}, nil
			}

			slope := sumXY / sumX2
			intercept := meanY - slope*meanX

			// Calculate R-squared
			var ssRes, ssTot float64
			for i := range params.X {
				predicted := slope*params.X[i] + intercept
				ssRes += (params.Y[i] - predicted) * (params.Y[i] - predicted)
				ssTot += (params.Y[i] - meanY) * (params.Y[i] - meanY)
			}
			rSquared := 1 - ssRes/ssTot

			// Standard error
			stdErr := math.Sqrt(ssRes / (n - 2))

			result := map[string]any{
				"slope":     slope,
				"intercept": intercept,
				"r_squared": rSquared,
				"std_error": stdErr,
				"equation":  map[string]float64{"m": slope, "b": intercept},
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func tTestTool() tool.Tool {
	return tool.NewBuilder("stats_t_test").
		WithDescription("Perform two-sample t-test").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Sample1 []float64 `json:"sample1"`
				Sample2 []float64 `json:"sample2"`
				Paired  bool      `json:"paired,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Sample1) < 2 || len(params.Sample2) < 2 {
				return tool.Result{Output: json.RawMessage(`{"error": "samples must have at least 2 values"}`)}, nil
			}

			n1 := float64(len(params.Sample1))
			n2 := float64(len(params.Sample2))
			mean1 := calcMean(params.Sample1)
			mean2 := calcMean(params.Sample2)
			var1 := calcVariance(params.Sample1, mean1, true)
			var2 := calcVariance(params.Sample2, mean2, true)

			// Welch's t-test (unequal variances)
			se := math.Sqrt(var1/n1 + var2/n2)
			if se == 0 {
				return tool.Result{Output: json.RawMessage(`{"error": "standard error is zero"}`)}, nil
			}

			tStatistic := (mean1 - mean2) / se

			// Degrees of freedom (Welch-Satterthwaite)
			num := (var1/n1 + var2/n2) * (var1/n1 + var2/n2)
			denom := (var1*var1)/(n1*n1*(n1-1)) + (var2*var2)/(n2*n2*(n2-1))
			df := num / denom

			result := map[string]any{
				"t_statistic":      tStatistic,
				"degrees_freedom":  df,
				"mean_difference":  mean1 - mean2,
				"sample1_mean":     mean1,
				"sample2_mean":     mean2,
				"sample1_variance": var1,
				"sample2_variance": var2,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Helper functions

func calcMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calcVariance(values []float64, mean float64, sample bool) float64 {
	if len(values) < 2 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		diff := v - mean
		sum += diff * diff
	}
	n := float64(len(values))
	if sample {
		n -= 1
	}
	return sum / n
}

func calcPercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p / 100.0) * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	return sorted[lower] + (idx-float64(lower))*(sorted[upper]-sorted[lower])
}
