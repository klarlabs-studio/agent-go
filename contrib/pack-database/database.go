// Package database provides database query tools for agent-go.
//
// This pack includes tools for database operations:
//   - db_query: Execute a SELECT query and return results
//   - db_execute: Execute an INSERT, UPDATE, or DELETE statement
//   - db_transaction: Execute multiple statements in a transaction
//   - db_schema: Get database schema information
//   - db_tables: List tables in the database
//   - db_describe: Describe a table's columns and types
//
// Uses database/sql from stdlib. The caller must import the appropriate driver
// (e.g., _ "github.com/lib/pq" for PostgreSQL) and pass the *sql.DB.
// Queries are parameterized to prevent SQL injection.
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Config holds the database configuration.
type Config struct {
	// DB is the database connection pool. Required.
	DB *sql.DB

	// Driver is the database driver name (e.g., "postgres", "mysql", "sqlite3").
	// Used for driver-specific schema queries.
	Driver string

	// MaxRows limits the number of rows returned by queries (default: 1000).
	MaxRows int

	// ReadOnly restricts to SELECT queries only when true.
	ReadOnly bool
}

type dbPack struct {
	cfg Config
}

// Pack returns the database tools pack.
func Pack(cfg Config) *pack.Pack {
	if cfg.MaxRows == 0 {
		cfg.MaxRows = 1000
	}

	p := &dbPack{cfg: cfg}

	builder := pack.NewBuilder("database").
		WithDescription("Database query and management tools").
		WithVersion("0.1.0").
		AddTools(
			p.dbQuery(),
			p.dbExecute(),
			p.dbTransaction(),
			p.dbSchema(),
			p.dbTables(),
			p.dbDescribe(),
		).
		AllowInState(agent.StateExplore, "db_query", "db_schema", "db_tables", "db_describe").
		AllowInState(agent.StateAct, "db_query", "db_execute", "db_transaction", "db_schema", "db_tables", "db_describe").
		AllowInState(agent.StateValidate, "db_query", "db_schema", "db_tables", "db_describe")

	return builder.Build()
}

func (p *dbPack) dbQuery() tool.Tool {
	return tool.NewBuilder("db_query").
		WithDescription("Execute a SELECT query and return results as JSON").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query  string `json:"query"`
				Args   []any  `json:"args,omitempty"`
				Limit  int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Query == "" {
				return tool.Result{}, fmt.Errorf("query is required")
			}

			limit := params.Limit
			if limit <= 0 || limit > p.cfg.MaxRows {
				limit = p.cfg.MaxRows
			}

			rows, err := p.cfg.DB.QueryContext(ctx, params.Query, params.Args...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			columns, err := rows.Columns()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get columns: %w", err)
			}

			var results []map[string]any
			count := 0
			for rows.Next() && count < limit {
				values := make([]any, len(columns))
				valuePtrs := make([]any, len(columns))
				for i := range values {
					valuePtrs[i] = &values[i]
				}

				if err := rows.Scan(valuePtrs...); err != nil {
					return tool.Result{}, fmt.Errorf("scan failed: %w", err)
				}

				row := make(map[string]any)
				for i, col := range columns {
					val := values[i]
					if b, ok := val.([]byte); ok {
						row[col] = string(b)
					} else {
						row[col] = val
					}
				}
				results = append(results, row)
				count++
			}

			if err := rows.Err(); err != nil {
				return tool.Result{}, fmt.Errorf("rows iteration error: %w", err)
			}

			result := map[string]any{
				"columns":  columns,
				"rows":     results,
				"count":    len(results),
				"truncated": count >= limit && limit < p.cfg.MaxRows,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *dbPack) dbExecute() tool.Tool {
	return tool.NewBuilder("db_execute").
		WithDescription("Execute an INSERT, UPDATE, or DELETE statement").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if p.cfg.ReadOnly {
				return tool.Result{}, fmt.Errorf("database is configured as read-only")
			}

			var params struct {
				Query string `json:"query"`
				Args  []any  `json:"args,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Query == "" {
				return tool.Result{}, fmt.Errorf("query is required")
			}

			res, err := p.cfg.DB.ExecContext(ctx, params.Query, params.Args...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("execute failed: %w", err)
			}

			rowsAffected, _ := res.RowsAffected()
			lastInsertID, _ := res.LastInsertId()

			result := map[string]any{
				"success":        true,
				"rows_affected":  rowsAffected,
				"last_insert_id": lastInsertID,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *dbPack) dbTransaction() tool.Tool {
	return tool.NewBuilder("db_transaction").
		WithDescription("Execute multiple statements in a transaction").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if p.cfg.ReadOnly {
				return tool.Result{}, fmt.Errorf("database is configured as read-only")
			}

			var params struct {
				Statements []struct {
					Query string `json:"query"`
					Args  []any  `json:"args,omitempty"`
				} `json:"statements"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if len(params.Statements) == 0 {
				return tool.Result{}, fmt.Errorf("at least one statement is required")
			}

			tx, err := p.cfg.DB.BeginTx(ctx, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to begin transaction: %w", err)
			}

			var stmtResults []map[string]any

			for i, stmt := range params.Statements {
				res, err := tx.ExecContext(ctx, stmt.Query, stmt.Args...)
				if err != nil {
					tx.Rollback() //nolint:errcheck // Best effort rollback.
					return tool.Result{}, fmt.Errorf("statement %d failed: %w", i, err)
				}

				rowsAffected, _ := res.RowsAffected()
				stmtResults = append(stmtResults, map[string]any{
					"index":         i,
					"rows_affected": rowsAffected,
				})
			}

			if err := tx.Commit(); err != nil {
				return tool.Result{}, fmt.Errorf("commit failed: %w", err)
			}

			result := map[string]any{
				"success":    true,
				"statements": len(params.Statements),
				"results":    stmtResults,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *dbPack) dbSchema() tool.Tool {
	return tool.NewBuilder("db_schema").
		WithDescription("Get the database schema as JSON").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			tables, err := p.listTables(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list tables: %w", err)
			}

			schema := make([]map[string]any, 0, len(tables))
			for _, table := range tables {
				cols, err := p.describeTable(ctx, table)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to describe %s: %w", table, err)
				}
				schema = append(schema, map[string]any{
					"table":   table,
					"columns": cols,
				})
			}

			result := map[string]any{
				"driver":  p.cfg.Driver,
				"tables":  schema,
				"count":   len(schema),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *dbPack) dbTables() tool.Tool {
	return tool.NewBuilder("db_tables").
		WithDescription("List all tables in the database").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			tables, err := p.listTables(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list tables: %w", err)
			}

			result := map[string]any{
				"tables": tables,
				"count":  len(tables),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *dbPack) dbDescribe() tool.Tool {
	return tool.NewBuilder("db_describe").
		WithDescription("Describe a table's columns, types, and constraints").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Table string `json:"table"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Table == "" {
				return tool.Result{}, fmt.Errorf("table name is required")
			}

			cols, err := p.describeTable(ctx, params.Table)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to describe table: %w", err)
			}

			result := map[string]any{
				"table":   params.Table,
				"columns": cols,
				"count":   len(cols),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// listTables returns all table names from the database.
func (p *dbPack) listTables(ctx context.Context) ([]string, error) {
	var query string
	switch strings.ToLower(p.cfg.Driver) {
	case "postgres", "pgx":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name"
	case "mysql":
		query = "SHOW TABLES"
	case "sqlite3", "sqlite":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	default:
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name"
	}

	rows, err := p.cfg.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// describeTable returns column information for a table.
func (p *dbPack) describeTable(ctx context.Context, table string) ([]map[string]any, error) {
	var query string
	switch strings.ToLower(p.cfg.Driver) {
	case "postgres", "pgx":
		query = `SELECT column_name, data_type, is_nullable, column_default, character_maximum_length
			FROM information_schema.columns WHERE table_name = $1 ORDER BY ordinal_position`
	case "mysql":
		query = `SELECT column_name, data_type, is_nullable, column_default, character_maximum_length
			FROM information_schema.columns WHERE table_name = ? ORDER BY ordinal_position`
	case "sqlite3", "sqlite":
		// SQLite uses PRAGMA; we use a fallback approach.
		return p.describeSQLiteTable(ctx, table)
	default:
		query = `SELECT column_name, data_type, is_nullable, column_default, character_maximum_length
			FROM information_schema.columns WHERE table_name = $1 ORDER BY ordinal_position`
	}

	rows, err := p.cfg.DB.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []map[string]any
	for rows.Next() {
		var name, dataType, nullable string
		var defaultVal, maxLen *string
		if err := rows.Scan(&name, &dataType, &nullable, &defaultVal, &maxLen); err != nil {
			return nil, err
		}

		col := map[string]any{
			"name":     name,
			"type":     dataType,
			"nullable": nullable == "YES",
		}
		if defaultVal != nil {
			col["default"] = *defaultVal
		}
		if maxLen != nil {
			col["max_length"] = *maxLen
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

// describeSQLiteTable uses PRAGMA table_info for SQLite.
func (p *dbPack) describeSQLiteTable(ctx context.Context, table string) ([]map[string]any, error) {
	// table_info is safe here since table names come from sqlite_master.
	rows, err := p.cfg.DB.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info('%s')", strings.ReplaceAll(table, "'", "''")))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []map[string]any
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultVal *string
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}

		col := map[string]any{
			"name":        name,
			"type":        dataType,
			"nullable":    notNull == 0,
			"primary_key": pk > 0,
		}
		if defaultVal != nil {
			col["default"] = *defaultVal
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}
