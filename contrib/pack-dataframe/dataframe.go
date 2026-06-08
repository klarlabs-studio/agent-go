// Package dataframe provides tabular data manipulation tools for agents.
// It enables DataFrame-like operations on structured data including filtering,
// grouping, aggregation, joining, and transformations.
package dataframe

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// DataFrame represents a tabular data structure with named columns.
type DataFrame struct {
	Columns []string          `json:"columns"`
	Rows    []map[string]any  `json:"rows"`
	Types   map[string]string `json:"types,omitempty"`
}

// Pack returns the dataframe tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("dataframe").
		WithDescription("Tabular data manipulation tools").
		AddTools(
			createTool(),
			fromJSONTool(),
			fromCSVTool(),
			toJSONTool(),
			toCSVTool(),
			selectColumnsTool(),
			dropColumnsTool(),
			renameColumnsTool(),
			filterTool(),
			sortByTool(),
			limitTool(),
			offsetTool(),
			groupByTool(),
			aggregateTool(),
			joinTool(),
			unionTool(),
			distinctTool(),
			addColumnTool(),
			mapColumnTool(),
			fillNATool(),
			dropNATool(),
			pivotTool(),
			meltTool(),
			describeTool(),
			headTool(),
			tailTool(),
			sampleTool(),
			shapeTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func createTool() tool.Tool {
	return tool.NewBuilder("df_create").
		WithDescription("Create a new DataFrame from columns and rows").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Columns []string `json:"columns"`
				Rows    [][]any  `json:"rows"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			df := DataFrame{
				Columns: params.Columns,
				Rows:    make([]map[string]any, len(params.Rows)),
			}

			for i, row := range params.Rows {
				df.Rows[i] = make(map[string]any)
				for j, col := range params.Columns {
					if j < len(row) {
						df.Rows[i][col] = row[j]
					}
				}
			}

			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fromJSONTool() tool.Tool {
	return tool.NewBuilder("df_from_json").
		WithDescription("Create a DataFrame from JSON array of objects").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data []map[string]any `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Extract columns from first row
			var columns []string
			colSet := make(map[string]bool)
			for _, row := range params.Data {
				for col := range row {
					if !colSet[col] {
						columns = append(columns, col)
						colSet[col] = true
					}
				}
			}
			sort.Strings(columns)

			df := DataFrame{
				Columns: columns,
				Rows:    params.Data,
			}

			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fromCSVTool() tool.Tool {
	return tool.NewBuilder("df_from_csv").
		WithDescription("Create a DataFrame from CSV string").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				CSV       string `json:"csv"`
				Delimiter string `json:"delimiter,omitempty"`
				HasHeader bool   `json:"has_header,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Delimiter == "" {
				params.Delimiter = ","
			}

			lines := strings.Split(strings.TrimSpace(params.CSV), "\n")
			if len(lines) == 0 {
				return tool.Result{Output: json.RawMessage(`{"columns":[],"rows":[]}`)}, nil
			}

			// Parse header
			var columns []string
			startRow := 0
			if params.HasHeader || true { // Default to having header
				columns = parseCSVLine(lines[0], params.Delimiter)
				startRow = 1
			} else {
				// Generate column names
				firstRow := parseCSVLine(lines[0], params.Delimiter)
				for i := range firstRow {
					columns = append(columns, fmt.Sprintf("col%d", i))
				}
			}

			// Parse rows
			rows := make([]map[string]any, 0, len(lines)-startRow)
			for i := startRow; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == "" {
					continue
				}
				values := parseCSVLine(lines[i], params.Delimiter)
				row := make(map[string]any)
				for j, col := range columns {
					if j < len(values) {
						row[col] = inferType(values[j])
					}
				}
				rows = append(rows, row)
			}

			df := DataFrame{Columns: columns, Rows: rows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseCSVLine(line, delimiter string) []string {
	// Simple CSV parsing (doesn't handle quoted fields with delimiters)
	return strings.Split(line, delimiter)
}

func inferType(s string) any {
	s = strings.TrimSpace(s)
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	return s
}

func toJSONTool() tool.Tool {
	return tool.NewBuilder("df_to_json").
		WithDescription("Convert DataFrame to JSON array").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var df DataFrame
			if err := json.Unmarshal(input, &df); err != nil {
				return tool.Result{}, err
			}
			output, _ := json.Marshal(df.Rows)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func toCSVTool() tool.Tool {
	return tool.NewBuilder("df_to_csv").
		WithDescription("Convert DataFrame to CSV string").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Delimiter string `json:"delimiter,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Delimiter == "" {
				params.Delimiter = ","
			}

			var sb strings.Builder
			sb.WriteString(strings.Join(params.Columns, params.Delimiter))
			sb.WriteString("\n")

			for _, row := range params.Rows {
				var values []string
				for _, col := range params.Columns {
					values = append(values, fmt.Sprintf("%v", row[col]))
				}
				sb.WriteString(strings.Join(values, params.Delimiter))
				sb.WriteString("\n")
			}

			result := map[string]string{"csv": sb.String()}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func selectColumnsTool() tool.Tool {
	return tool.NewBuilder("df_select").
		WithDescription("Select specific columns from DataFrame").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Columns []string `json:"select_columns"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			newRows := make([]map[string]any, len(params.Rows))
			for i, row := range params.Rows {
				newRows[i] = make(map[string]any)
				for _, col := range params.Columns {
					if v, ok := row[col]; ok {
						newRows[i][col] = v
					}
				}
			}

			df := DataFrame{Columns: params.Columns, Rows: newRows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func dropColumnsTool() tool.Tool {
	return tool.NewBuilder("df_drop").
		WithDescription("Drop specified columns from DataFrame").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				DropColumns []string `json:"drop_columns"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			dropSet := make(map[string]bool)
			for _, col := range params.DropColumns {
				dropSet[col] = true
			}

			var newColumns []string
			for _, col := range params.Columns {
				if !dropSet[col] {
					newColumns = append(newColumns, col)
				}
			}

			newRows := make([]map[string]any, len(params.Rows))
			for i, row := range params.Rows {
				newRows[i] = make(map[string]any)
				for _, col := range newColumns {
					if v, ok := row[col]; ok {
						newRows[i][col] = v
					}
				}
			}

			df := DataFrame{Columns: newColumns, Rows: newRows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func renameColumnsTool() tool.Tool {
	return tool.NewBuilder("df_rename").
		WithDescription("Rename columns in DataFrame").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Mapping map[string]string `json:"mapping"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			newColumns := make([]string, len(params.Columns))
			for i, col := range params.Columns {
				if newName, ok := params.Mapping[col]; ok {
					newColumns[i] = newName
				} else {
					newColumns[i] = col
				}
			}

			newRows := make([]map[string]any, len(params.Rows))
			for i, row := range params.Rows {
				newRows[i] = make(map[string]any)
				for _, col := range params.Columns {
					newCol := col
					if renamed, ok := params.Mapping[col]; ok {
						newCol = renamed
					}
					if v, ok := row[col]; ok {
						newRows[i][newCol] = v
					}
				}
			}

			df := DataFrame{Columns: newColumns, Rows: newRows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func filterTool() tool.Tool {
	return tool.NewBuilder("df_filter").
		WithDescription("Filter DataFrame rows based on conditions").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Column   string `json:"column"`
				Operator string `json:"operator"` // eq, ne, gt, gte, lt, lte, contains, startswith, endswith
				Value    any    `json:"value"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var filtered []map[string]any
			for _, row := range params.Rows {
				val := row[params.Column]
				if matchCondition(val, params.Operator, params.Value) {
					filtered = append(filtered, row)
				}
			}

			df := DataFrame{Columns: params.Columns, Rows: filtered}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func matchCondition(val any, operator string, target any) bool {
	valStr := fmt.Sprintf("%v", val)
	targetStr := fmt.Sprintf("%v", target)

	switch operator {
	case "eq", "==", "=":
		return valStr == targetStr
	case "ne", "!=":
		return valStr != targetStr
	case "contains":
		return strings.Contains(valStr, targetStr)
	case "startswith":
		return strings.HasPrefix(valStr, targetStr)
	case "endswith":
		return strings.HasSuffix(valStr, targetStr)
	case "gt", ">":
		return toFloat(val) > toFloat(target)
	case "gte", ">=":
		return toFloat(val) >= toFloat(target)
	case "lt", "<":
		return toFloat(val) < toFloat(target)
	case "lte", "<=":
		return toFloat(val) <= toFloat(target)
	default:
		return false
	}
}

func toFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

func sortByTool() tool.Tool {
	return tool.NewBuilder("df_sort").
		WithDescription("Sort DataFrame by one or more columns").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				By        []string `json:"by"`
				Ascending []bool   `json:"ascending,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Default all to ascending
			if len(params.Ascending) == 0 {
				params.Ascending = make([]bool, len(params.By))
				for i := range params.Ascending {
					params.Ascending[i] = true
				}
			}

			rows := make([]map[string]any, len(params.Rows))
			copy(rows, params.Rows)

			sort.SliceStable(rows, func(i, j int) bool {
				for k, col := range params.By {
					asc := true
					if k < len(params.Ascending) {
						asc = params.Ascending[k]
					}

					vi := fmt.Sprintf("%v", rows[i][col])
					vj := fmt.Sprintf("%v", rows[j][col])

					if vi != vj {
						if asc {
							return vi < vj
						}
						return vi > vj
					}
				}
				return false
			})

			df := DataFrame{Columns: params.Columns, Rows: rows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func limitTool() tool.Tool {
	return tool.NewBuilder("df_limit").
		WithDescription("Limit DataFrame to first N rows").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				N int `json:"n"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n > len(params.Rows) {
				n = len(params.Rows)
			}

			df := DataFrame{Columns: params.Columns, Rows: params.Rows[:n]}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func offsetTool() tool.Tool {
	return tool.NewBuilder("df_offset").
		WithDescription("Skip first N rows of DataFrame").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				N int `json:"n"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n > len(params.Rows) {
				n = len(params.Rows)
			}

			df := DataFrame{Columns: params.Columns, Rows: params.Rows[n:]}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func groupByTool() tool.Tool {
	return tool.NewBuilder("df_group_by").
		WithDescription("Group DataFrame rows by columns").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				By []string `json:"by"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			groups := make(map[string][]map[string]any)
			for _, row := range params.Rows {
				var keyParts []string
				for _, col := range params.By {
					keyParts = append(keyParts, fmt.Sprintf("%v", row[col]))
				}
				key := strings.Join(keyParts, "|")
				groups[key] = append(groups[key], row)
			}

			result := make(map[string]any)
			result["groups"] = groups
			result["columns"] = params.Columns
			result["group_by"] = params.By

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func aggregateTool() tool.Tool {
	return tool.NewBuilder("df_aggregate").
		WithDescription("Aggregate DataFrame with group by and aggregation functions").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				GroupBy      []string          `json:"group_by"`
				Aggregations map[string]string `json:"aggregations"` // column -> function (sum, avg, min, max, count)
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Group rows
			groups := make(map[string][]map[string]any)
			groupKeys := make(map[string]map[string]any)
			for _, row := range params.Rows {
				var keyParts []string
				keyMap := make(map[string]any)
				for _, col := range params.GroupBy {
					keyParts = append(keyParts, fmt.Sprintf("%v", row[col]))
					keyMap[col] = row[col]
				}
				key := strings.Join(keyParts, "|")
				groups[key] = append(groups[key], row)
				groupKeys[key] = keyMap
			}

			// Aggregate
			var resultRows []map[string]any
			var columns []string
			columns = append(columns, params.GroupBy...)
			for col := range params.Aggregations {
				columns = append(columns, col)
			}

			for key, rows := range groups {
				resultRow := make(map[string]any)
				for k, v := range groupKeys[key] {
					resultRow[k] = v
				}

				for col, fn := range params.Aggregations {
					var values []float64
					for _, row := range rows {
						values = append(values, toFloat(row[col]))
					}
					resultRow[col] = aggregate(values, fn)
				}
				resultRows = append(resultRows, resultRow)
			}

			df := DataFrame{Columns: columns, Rows: resultRows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func aggregate(values []float64, fn string) float64 {
	if len(values) == 0 {
		return 0
	}

	switch fn {
	case "sum":
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum
	case "avg", "mean":
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))
	case "min":
		min := values[0]
		for _, v := range values[1:] {
			if v < min {
				min = v
			}
		}
		return min
	case "max":
		max := values[0]
		for _, v := range values[1:] {
			if v > max {
				max = v
			}
		}
		return max
	case "count":
		return float64(len(values))
	default:
		return 0
	}
}

func joinTool() tool.Tool {
	return tool.NewBuilder("df_join").
		WithDescription("Join two DataFrames on specified columns").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Left  DataFrame `json:"left"`
				Right DataFrame `json:"right"`
				On    []string  `json:"on"`
				How   string    `json:"how"` // inner, left, right, outer
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.How == "" {
				params.How = "inner"
			}

			// Build index for right table
			rightIndex := make(map[string][]map[string]any)
			for _, row := range params.Right.Rows {
				key := buildJoinKey(row, params.On)
				rightIndex[key] = append(rightIndex[key], row)
			}

			// Merge columns
			colSet := make(map[string]bool)
			var columns []string
			for _, col := range params.Left.Columns {
				columns = append(columns, col)
				colSet[col] = true
			}
			for _, col := range params.Right.Columns {
				if !colSet[col] {
					columns = append(columns, col)
				}
			}

			var resultRows []map[string]any
			usedRight := make(map[string]bool)

			for _, leftRow := range params.Left.Rows {
				key := buildJoinKey(leftRow, params.On)
				rightRows := rightIndex[key]

				if len(rightRows) > 0 {
					usedRight[key] = true
					for _, rightRow := range rightRows {
						merged := make(map[string]any)
						for k, v := range leftRow {
							merged[k] = v
						}
						for k, v := range rightRow {
							if _, exists := merged[k]; !exists {
								merged[k] = v
							}
						}
						resultRows = append(resultRows, merged)
					}
				} else if params.How == "left" || params.How == "outer" {
					resultRows = append(resultRows, leftRow)
				}
			}

			if params.How == "right" || params.How == "outer" {
				for _, rightRow := range params.Right.Rows {
					key := buildJoinKey(rightRow, params.On)
					if !usedRight[key] {
						resultRows = append(resultRows, rightRow)
					}
				}
			}

			df := DataFrame{Columns: columns, Rows: resultRows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func buildJoinKey(row map[string]any, on []string) string {
	var parts []string
	for _, col := range on {
		parts = append(parts, fmt.Sprintf("%v", row[col]))
	}
	return strings.Join(parts, "|")
}

func unionTool() tool.Tool {
	return tool.NewBuilder("df_union").
		WithDescription("Union two DataFrames (concatenate rows)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				First  DataFrame `json:"first"`
				Second DataFrame `json:"second"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			rows := make([]map[string]any, 0, len(params.First.Rows)+len(params.Second.Rows))
			rows = append(rows, params.First.Rows...)
			rows = append(rows, params.Second.Rows...)
			df := DataFrame{Columns: params.First.Columns, Rows: rows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func distinctTool() tool.Tool {
	return tool.NewBuilder("df_distinct").
		WithDescription("Remove duplicate rows from DataFrame").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Columns []string `json:"on_columns,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			cols := params.Columns
			if len(cols) == 0 {
				cols = params.DataFrame.Columns
			}

			seen := make(map[string]bool)
			var unique []map[string]any

			for _, row := range params.Rows {
				key := buildJoinKey(row, cols)
				if !seen[key] {
					seen[key] = true
					unique = append(unique, row)
				}
			}

			df := DataFrame{Columns: params.DataFrame.Columns, Rows: unique}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func addColumnTool() tool.Tool {
	return tool.NewBuilder("df_add_column").
		WithDescription("Add a new column with a constant value or computed from expression").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Name  string `json:"name"`
				Value any    `json:"value,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			columns := append(append([]string{}, params.Columns...), params.Name)
			rows := make([]map[string]any, len(params.Rows))
			for i, row := range params.Rows {
				rows[i] = make(map[string]any)
				for k, v := range row {
					rows[i][k] = v
				}
				rows[i][params.Name] = params.Value
			}

			df := DataFrame{Columns: columns, Rows: rows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func mapColumnTool() tool.Tool {
	return tool.NewBuilder("df_map_column").
		WithDescription("Transform column values using a mapping").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Column  string         `json:"column"`
				Mapping map[string]any `json:"mapping"`
				Default any            `json:"default,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			rows := make([]map[string]any, len(params.Rows))
			for i, row := range params.Rows {
				rows[i] = make(map[string]any)
				for k, v := range row {
					rows[i][k] = v
				}
				key := fmt.Sprintf("%v", row[params.Column])
				if mapped, ok := params.Mapping[key]; ok {
					rows[i][params.Column] = mapped
				} else if params.Default != nil {
					rows[i][params.Column] = params.Default
				}
			}

			df := DataFrame{Columns: params.Columns, Rows: rows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fillNATool() tool.Tool {
	return tool.NewBuilder("df_fill_na").
		WithDescription("Fill null/empty values with specified value").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Value     any            `json:"value,omitempty"`
				PerColumn map[string]any `json:"per_column,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			rows := make([]map[string]any, len(params.Rows))
			for i, row := range params.Rows {
				rows[i] = make(map[string]any)
				for k, v := range row {
					rows[i][k] = v
					if isNA(v) {
						if colVal, ok := params.PerColumn[k]; ok {
							rows[i][k] = colVal
						} else if params.Value != nil {
							rows[i][k] = params.Value
						}
					}
				}
			}

			df := DataFrame{Columns: params.DataFrame.Columns, Rows: rows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isNA(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok && (s == "" || s == "null" || s == "NA" || s == "NaN") {
		return true
	}
	return false
}

func dropNATool() tool.Tool {
	return tool.NewBuilder("df_drop_na").
		WithDescription("Drop rows with null/empty values").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Columns []string `json:"columns,omitempty"`
				How     string   `json:"how,omitempty"` // any, all
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.How == "" {
				params.How = "any"
			}
			cols := params.Columns
			if len(cols) == 0 {
				cols = params.DataFrame.Columns
			}

			var filtered []map[string]any
			for _, row := range params.Rows {
				naCount := 0
				for _, col := range cols {
					if isNA(row[col]) {
						naCount++
					}
				}

				keep := true
				if params.How == "any" && naCount > 0 {
					keep = false
				} else if params.How == "all" && naCount == len(cols) {
					keep = false
				}

				if keep {
					filtered = append(filtered, row)
				}
			}

			df := DataFrame{Columns: params.DataFrame.Columns, Rows: filtered}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pivotTool() tool.Tool {
	return tool.NewBuilder("df_pivot").
		WithDescription("Pivot DataFrame from long to wide format").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				Index   string `json:"index"`
				Columns string `json:"columns"`
				Values  string `json:"values"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Find unique column values
			colVals := make(map[string]bool)
			for _, row := range params.Rows {
				colVals[fmt.Sprintf("%v", row[params.Columns])] = true
			}

			// Build new columns
			var newCols []string
			newCols = append(newCols, params.Index)
			for col := range colVals {
				newCols = append(newCols, col)
			}
			sort.Strings(newCols[1:])

			// Pivot data
			pivoted := make(map[string]map[string]any)
			for _, row := range params.Rows {
				idx := fmt.Sprintf("%v", row[params.Index])
				col := fmt.Sprintf("%v", row[params.Columns])
				val := row[params.Values]

				if _, ok := pivoted[idx]; !ok {
					pivoted[idx] = make(map[string]any)
					pivoted[idx][params.Index] = row[params.Index]
				}
				pivoted[idx][col] = val
			}

			var resultRows []map[string]any
			for _, row := range pivoted {
				resultRows = append(resultRows, row)
			}

			df := DataFrame{Columns: newCols, Rows: resultRows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func meltTool() tool.Tool {
	return tool.NewBuilder("df_melt").
		WithDescription("Melt DataFrame from wide to long format").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				IDVars    []string `json:"id_vars"`
				ValueVars []string `json:"value_vars"`
				VarName   string   `json:"var_name,omitempty"`
				ValueName string   `json:"value_name,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.VarName == "" {
				params.VarName = "variable"
			}
			if params.ValueName == "" {
				params.ValueName = "value"
			}

			var columns []string
			columns = append(columns, params.IDVars...)
			columns = append(columns, params.VarName, params.ValueName)

			var resultRows []map[string]any
			for _, row := range params.Rows {
				for _, valVar := range params.ValueVars {
					newRow := make(map[string]any)
					for _, idVar := range params.IDVars {
						newRow[idVar] = row[idVar]
					}
					newRow[params.VarName] = valVar
					newRow[params.ValueName] = row[valVar]
					resultRows = append(resultRows, newRow)
				}
			}

			df := DataFrame{Columns: columns, Rows: resultRows}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func describeTool() tool.Tool {
	return tool.NewBuilder("df_describe").
		WithDescription("Generate descriptive statistics for DataFrame columns").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var df DataFrame
			if err := json.Unmarshal(input, &df); err != nil {
				return tool.Result{}, err
			}

			stats := make(map[string]map[string]any)
			for _, col := range df.Columns {
				var values []float64
				var strCount int
				for _, row := range df.Rows {
					if v, ok := row[col]; ok {
						if f := toFloat(v); f != 0 || fmt.Sprintf("%v", v) == "0" {
							values = append(values, f)
						} else {
							strCount++
						}
					}
				}

				colStats := make(map[string]any)
				colStats["count"] = len(df.Rows)
				colStats["non_null"] = len(values) + strCount

				if len(values) > 0 {
					sort.Float64s(values)
					colStats["min"] = values[0]
					colStats["max"] = values[len(values)-1]
					colStats["mean"] = aggregate(values, "avg")
					colStats["sum"] = aggregate(values, "sum")

					// Median
					mid := len(values) / 2
					if len(values)%2 == 0 {
						colStats["median"] = (values[mid-1] + values[mid]) / 2
					} else {
						colStats["median"] = values[mid]
					}
				}

				stats[col] = colStats
			}

			output, _ := json.Marshal(stats)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func headTool() tool.Tool {
	return tool.NewBuilder("df_head").
		WithDescription("Get first N rows of DataFrame (default 5)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				N int `json:"n,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n == 0 {
				n = 5
			}
			if n > len(params.Rows) {
				n = len(params.Rows)
			}

			df := DataFrame{Columns: params.Columns, Rows: params.Rows[:n]}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func tailTool() tool.Tool {
	return tool.NewBuilder("df_tail").
		WithDescription("Get last N rows of DataFrame (default 5)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				N int `json:"n,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n == 0 {
				n = 5
			}
			if n > len(params.Rows) {
				n = len(params.Rows)
			}

			start := len(params.Rows) - n
			df := DataFrame{Columns: params.Columns, Rows: params.Rows[start:]}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sampleTool() tool.Tool {
	return tool.NewBuilder("df_sample").
		WithDescription("Get random sample of N rows from DataFrame").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DataFrame
				N    int   `json:"n,omitempty"`
				Seed int64 `json:"seed,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n == 0 {
				n = 5
			}
			if n > len(params.Rows) {
				n = len(params.Rows)
			}

			// Simple sampling - take every kth element
			step := len(params.Rows) / n
			if step == 0 {
				step = 1
			}

			var sampled []map[string]any
			for i := 0; i < len(params.Rows) && len(sampled) < n; i += step {
				sampled = append(sampled, params.Rows[i])
			}

			df := DataFrame{Columns: params.Columns, Rows: sampled}
			output, _ := json.Marshal(df)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func shapeTool() tool.Tool {
	return tool.NewBuilder("df_shape").
		WithDescription("Get the shape (rows, columns) of DataFrame").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var df DataFrame
			if err := json.Unmarshal(input, &df); err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"rows":         len(df.Rows),
				"columns":      len(df.Columns),
				"column_names": df.Columns,
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
