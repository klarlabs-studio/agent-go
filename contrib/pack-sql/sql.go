// Package sql provides SQL database tools for agent-go.
//
// Supports PostgreSQL, MySQL, and SQLite with parameterized queries,
// schema inspection, and transaction support.
package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

// sqlIdentifierRegex validates SQL identifiers (table/column names).
// Only allows alphanumeric characters and underscores, must start with letter or underscore.
var sqlIdentifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateIdentifier checks if a string is a valid SQL identifier to prevent SQL injection.
func validateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	if len(name) > 128 {
		return fmt.Errorf("identifier too long (max 128 characters)")
	}
	if !sqlIdentifierRegex.MatchString(name) {
		return fmt.Errorf("invalid identifier %q: must contain only alphanumeric characters and underscores, starting with a letter or underscore", name)
	}
	return nil
}

// quoteIdentifier wraps an identifier in quotes appropriate for the database driver.
func (p *sqlPack) quoteIdentifier(name string) string {
	switch p.cfg.Driver {
	case "postgres", "pgx":
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	case "mysql":
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	default:
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
}

// Config holds database connection settings.
type Config struct {
	Driver string // postgres, mysql, sqlite3
	DSN    string // Data Source Name
}

// Pack returns the SQL tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &sqlPack{cfg: cfg}

	return pack.NewBuilder("sql").
		WithDescription("SQL database tools for queries, schema inspection, and data management").
		WithVersion("1.0.0").
		AddTools(
			p.queryTool(),
			p.executeTool(),
			p.getTablesTool(),
			p.describeTableTool(),
			p.getSchemasTool(),
			p.getIndexesTool(),
			p.getConstraintsTool(),
			p.explainQueryTool(),
			p.insertTool(),
			p.updateTool(),
			p.deleteTool(),
			p.countTool(),
			p.transactionTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type sqlPack struct {
	cfg Config
	db  *sql.DB
}

func (p *sqlPack) getDB() (*sql.DB, error) {
	if p.db != nil {
		return p.db, nil
	}

	db, err := sql.Open(p.cfg.Driver, p.cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	p.db = db
	return p.db, nil
}

// ============================================================================
// Query Tools
// ============================================================================

func (p *sqlPack) queryTool() tool.Tool {
	return tool.NewBuilder("sql_query").
		WithDescription("Execute a SELECT query and return results").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query  string        `json:"query"`
				Params []interface{} `json:"params,omitempty"`
				Limit  int           `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Safety check - only allow SELECT
			if !isSelectQuery(in.Query) {
				return tool.Result{}, fmt.Errorf("query tool only supports SELECT statements")
			}

			if in.Limit == 0 {
				in.Limit = 100
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			// Add LIMIT if not present
			query := in.Query
			if !strings.Contains(strings.ToUpper(query), "LIMIT") {
				query = fmt.Sprintf("%s LIMIT %d", query, in.Limit)
			}

			rows, err := db.QueryContext(ctx, query, in.Params...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			results, err := scanRows(rows)
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(map[string]any{
				"count": len(results),
				"rows":  results,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) executeTool() tool.Tool {
	return tool.NewBuilder("sql_execute").
		WithDescription("Execute a raw SQL statement (INSERT, UPDATE, DELETE, DDL)").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query  string        `json:"query"`
				Params []interface{} `json:"params,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			result, err := db.ExecContext(ctx, in.Query, in.Params...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("execute failed: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()
			lastInsertID, _ := result.LastInsertId()

			output, _ := json.Marshal(map[string]any{
				"rows_affected":  rowsAffected,
				"last_insert_id": lastInsertID,
				"success":        true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Schema Inspection Tools
// ============================================================================

func (p *sqlPack) getTablesTool() tool.Tool {
	return tool.NewBuilder("sql_get_tables").
		WithDescription("List all tables in the database").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Schema string `json:"schema,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			var query string
			var args []interface{}

			switch p.cfg.Driver {
			case "postgres", "pgx":
				schema := in.Schema
				if schema == "" {
					schema = "public"
				}
				query = `SELECT table_name FROM information_schema.tables
				         WHERE table_schema = $1 AND table_type = 'BASE TABLE'
				         ORDER BY table_name`
				args = []interface{}{schema}
			case "mysql":
				query = `SELECT table_name FROM information_schema.tables
				         WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
				         ORDER BY table_name`
			case "sqlite3":
				query = `SELECT name FROM sqlite_master
				         WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
				         ORDER BY name`
			default:
				return tool.Result{}, fmt.Errorf("unsupported driver: %s", p.cfg.Driver)
			}

			rows, err := db.QueryContext(ctx, query, args...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get tables: %w", err)
			}
			defer rows.Close()

			var tables []string
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err != nil {
					return tool.Result{}, err
				}
				tables = append(tables, name)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(tables),
				"tables": tables,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) describeTableTool() tool.Tool {
	return tool.NewBuilder("sql_describe_table").
		WithDescription("Get column information for a table").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Table  string `json:"table"`
				Schema string `json:"schema,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			var query string
			var args []interface{}

			switch p.cfg.Driver {
			case "postgres", "pgx":
				schema := in.Schema
				if schema == "" {
					schema = "public"
				}
				query = `SELECT column_name, data_type, is_nullable, column_default
				         FROM information_schema.columns
				         WHERE table_schema = $1 AND table_name = $2
				         ORDER BY ordinal_position`
				args = []interface{}{schema, in.Table}
			case "mysql":
				query = `SELECT column_name, data_type, is_nullable, column_default
				         FROM information_schema.columns
				         WHERE table_schema = DATABASE() AND table_name = ?
				         ORDER BY ordinal_position`
				args = []interface{}{in.Table}
			case "sqlite3":
				query = fmt.Sprintf("PRAGMA table_info(%s)", in.Table)
			default:
				return tool.Result{}, fmt.Errorf("unsupported driver: %s", p.cfg.Driver)
			}

			rows, err := db.QueryContext(ctx, query, args...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to describe table: %w", err)
			}
			defer rows.Close()

			var columns []map[string]any

			if p.cfg.Driver == "sqlite3" {
				for rows.Next() {
					var cid int
					var name, dtype string
					var notnull, pk int
					var dflt sql.NullString
					if err := rows.Scan(&cid, &name, &dtype, &notnull, &dflt, &pk); err != nil {
						return tool.Result{}, err
					}
					columns = append(columns, map[string]any{
						"name":        name,
						"type":        dtype,
						"nullable":    notnull == 0,
						"default":     dflt.String,
						"primary_key": pk == 1,
					})
				}
			} else {
				for rows.Next() {
					var name, dtype, nullable string
					var dflt sql.NullString
					if err := rows.Scan(&name, &dtype, &nullable, &dflt); err != nil {
						return tool.Result{}, err
					}
					columns = append(columns, map[string]any{
						"name":     name,
						"type":     dtype,
						"nullable": nullable == "YES",
						"default":  dflt.String,
					})
				}
			}

			output, _ := json.Marshal(map[string]any{
				"table":   in.Table,
				"count":   len(columns),
				"columns": columns,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) getSchemasTool() tool.Tool {
	return tool.NewBuilder("sql_get_schemas").
		WithDescription("List all schemas in the database").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			var query string

			switch p.cfg.Driver {
			case "postgres", "pgx":
				query = `SELECT schema_name FROM information_schema.schemata
				         WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
				         ORDER BY schema_name`
			case "mysql":
				query = `SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`
			case "sqlite3":
				// SQLite doesn't have schemas
				output, _ := json.Marshal(map[string]any{
					"count":   1,
					"schemas": []string{"main"},
				})
				return tool.Result{Output: output}, nil
			default:
				return tool.Result{}, fmt.Errorf("unsupported driver: %s", p.cfg.Driver)
			}

			rows, err := db.QueryContext(ctx, query)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get schemas: %w", err)
			}
			defer rows.Close()

			var schemas []string
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err != nil {
					return tool.Result{}, err
				}
				schemas = append(schemas, name)
			}

			output, _ := json.Marshal(map[string]any{
				"count":   len(schemas),
				"schemas": schemas,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) getIndexesTool() tool.Tool {
	return tool.NewBuilder("sql_get_indexes").
		WithDescription("List indexes for a table").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Table  string `json:"table"`
				Schema string `json:"schema,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			var query string
			var args []interface{}

			switch p.cfg.Driver {
			case "postgres", "pgx":
				schema := in.Schema
				if schema == "" {
					schema = "public"
				}
				query = `SELECT indexname, indexdef FROM pg_indexes
				         WHERE schemaname = $1 AND tablename = $2`
				args = []interface{}{schema, in.Table}
			case "mysql":
				query = `SELECT index_name, column_name, non_unique
				         FROM information_schema.statistics
				         WHERE table_schema = DATABASE() AND table_name = ?`
				args = []interface{}{in.Table}
			case "sqlite3":
				query = fmt.Sprintf("PRAGMA index_list(%s)", in.Table)
			default:
				return tool.Result{}, fmt.Errorf("unsupported driver: %s", p.cfg.Driver)
			}

			rows, err := db.QueryContext(ctx, query, args...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get indexes: %w", err)
			}
			defer rows.Close()

			results, err := scanRows(rows)
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(map[string]any{
				"table":   in.Table,
				"count":   len(results),
				"indexes": results,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) getConstraintsTool() tool.Tool {
	return tool.NewBuilder("sql_get_constraints").
		WithDescription("List constraints for a table").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Table  string `json:"table"`
				Schema string `json:"schema,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			var query string
			var args []interface{}

			switch p.cfg.Driver {
			case "postgres", "pgx":
				schema := in.Schema
				if schema == "" {
					schema = "public"
				}
				query = `SELECT constraint_name, constraint_type
				         FROM information_schema.table_constraints
				         WHERE table_schema = $1 AND table_name = $2`
				args = []interface{}{schema, in.Table}
			case "mysql":
				query = `SELECT constraint_name, constraint_type
				         FROM information_schema.table_constraints
				         WHERE table_schema = DATABASE() AND table_name = ?`
				args = []interface{}{in.Table}
			case "sqlite3":
				// SQLite has limited constraint info via PRAGMA
				query = fmt.Sprintf("PRAGMA foreign_key_list(%s)", in.Table)
			default:
				return tool.Result{}, fmt.Errorf("unsupported driver: %s", p.cfg.Driver)
			}

			rows, err := db.QueryContext(ctx, query, args...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get constraints: %w", err)
			}
			defer rows.Close()

			results, err := scanRows(rows)
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(map[string]any{
				"table":       in.Table,
				"count":       len(results),
				"constraints": results,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) explainQueryTool() tool.Tool {
	return tool.NewBuilder("sql_explain_query").
		WithDescription("Get query execution plan").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query  string        `json:"query"`
				Params []interface{} `json:"params,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			var explainQuery string
			switch p.cfg.Driver {
			case "postgres", "pgx":
				explainQuery = "EXPLAIN ANALYZE " + in.Query
			case "mysql":
				explainQuery = "EXPLAIN " + in.Query
			case "sqlite3":
				explainQuery = "EXPLAIN QUERY PLAN " + in.Query
			default:
				return tool.Result{}, fmt.Errorf("unsupported driver: %s", p.cfg.Driver)
			}

			rows, err := db.QueryContext(ctx, explainQuery, in.Params...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("explain failed: %w", err)
			}
			defer rows.Close()

			results, err := scanRows(rows)
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(map[string]any{
				"query": in.Query,
				"plan":  results,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Data Modification Tools
// ============================================================================

func (p *sqlPack) insertTool() tool.Tool {
	return tool.NewBuilder("sql_insert").
		WithDescription("Insert a row into a table").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Table  string                 `json:"table"`
				Values map[string]interface{} `json:"values"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Validate table name to prevent SQL injection
			if err := validateIdentifier(in.Table); err != nil {
				return tool.Result{}, fmt.Errorf("invalid table name: %w", err)
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			// Build INSERT statement
			columns := make([]string, 0, len(in.Values))
			placeholders := make([]string, 0, len(in.Values))
			values := make([]interface{}, 0, len(in.Values))

			i := 1
			for col, val := range in.Values {
				// Validate column name to prevent SQL injection
				if err := validateIdentifier(col); err != nil {
					return tool.Result{}, fmt.Errorf("invalid column name: %w", err)
				}
				columns = append(columns, p.quoteIdentifier(col))
				values = append(values, val)
				switch p.cfg.Driver {
				case "postgres", "pgx":
					placeholders = append(placeholders, fmt.Sprintf("$%d", i))
				default:
					placeholders = append(placeholders, "?")
				}
				i++
			}

			// Table and column names are validated above; placeholders use parameterized values
			query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", // #nosec G201 -- identifiers validated
				p.quoteIdentifier(in.Table),
				strings.Join(columns, ", "),
				strings.Join(placeholders, ", "))

			result, err := db.ExecContext(ctx, query, values...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("insert failed: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()
			lastInsertID, _ := result.LastInsertId()

			output, _ := json.Marshal(map[string]any{
				"rows_affected":  rowsAffected,
				"last_insert_id": lastInsertID,
				"success":        true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) updateTool() tool.Tool {
	return tool.NewBuilder("sql_update").
		WithDescription("Update rows in a table").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Table  string                 `json:"table"`
				Set    map[string]interface{} `json:"set"`
				Where  string                 `json:"where"`
				Params []interface{}          `json:"params,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Validate table name to prevent SQL injection
			if err := validateIdentifier(in.Table); err != nil {
				return tool.Result{}, fmt.Errorf("invalid table name: %w", err)
			}

			if in.Where == "" {
				return tool.Result{}, fmt.Errorf("WHERE clause is required for UPDATE")
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			// Build UPDATE statement
			setClauses := make([]string, 0, len(in.Set))
			values := make([]interface{}, 0, len(in.Set)+len(in.Params))

			i := 1
			for col, val := range in.Set {
				// Validate column name to prevent SQL injection
				if err := validateIdentifier(col); err != nil {
					return tool.Result{}, fmt.Errorf("invalid column name: %w", err)
				}
				quotedCol := p.quoteIdentifier(col)
				switch p.cfg.Driver {
				case "postgres", "pgx":
					setClauses = append(setClauses, fmt.Sprintf("%s = $%d", quotedCol, i))
				default:
					setClauses = append(setClauses, fmt.Sprintf("%s = ?", quotedCol))
				}
				values = append(values, val)
				i++
			}
			values = append(values, in.Params...)

			// Table and column names validated; WHERE uses parameterized values via in.Params
			query := fmt.Sprintf("UPDATE %s SET %s WHERE %s", // #nosec G201 -- identifiers validated, WHERE uses params
				p.quoteIdentifier(in.Table),
				strings.Join(setClauses, ", "),
				in.Where)

			result, err := db.ExecContext(ctx, query, values...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("update failed: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()

			output, _ := json.Marshal(map[string]any{
				"rows_affected": rowsAffected,
				"success":       true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) deleteTool() tool.Tool {
	return tool.NewBuilder("sql_delete").
		WithDescription("Delete rows from a table").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Table  string        `json:"table"`
				Where  string        `json:"where"`
				Params []interface{} `json:"params,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Validate table name to prevent SQL injection
			if err := validateIdentifier(in.Table); err != nil {
				return tool.Result{}, fmt.Errorf("invalid table name: %w", err)
			}

			if in.Where == "" {
				return tool.Result{}, fmt.Errorf("WHERE clause is required for DELETE")
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			// Table name validated; WHERE uses parameterized values via in.Params
			query := fmt.Sprintf("DELETE FROM %s WHERE %s", p.quoteIdentifier(in.Table), in.Where) // #nosec G201 -- table validated, WHERE uses params

			result, err := db.ExecContext(ctx, query, in.Params...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("delete failed: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()

			output, _ := json.Marshal(map[string]any{
				"rows_affected": rowsAffected,
				"success":       true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) countTool() tool.Tool {
	return tool.NewBuilder("sql_count").
		WithDescription("Count rows in a table").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Table  string        `json:"table"`
				Where  string        `json:"where,omitempty"`
				Params []interface{} `json:"params,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Validate table name to prevent SQL injection
			if err := validateIdentifier(in.Table); err != nil {
				return tool.Result{}, fmt.Errorf("invalid table name: %w", err)
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			// Table name validated; WHERE uses parameterized values via in.Params
			query := fmt.Sprintf("SELECT COUNT(*) FROM %s", p.quoteIdentifier(in.Table)) // #nosec G201 -- table validated
			if in.Where != "" {
				query += " WHERE " + in.Where
			}

			var count int64
			err = db.QueryRowContext(ctx, query, in.Params...).Scan(&count)
			if err != nil {
				return tool.Result{}, fmt.Errorf("count failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"table": in.Table,
				"count": count,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *sqlPack) transactionTool() tool.Tool {
	return tool.NewBuilder("sql_transaction").
		WithDescription("Execute multiple statements in a transaction").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Statements []struct {
					Query  string        `json:"query"`
					Params []interface{} `json:"params,omitempty"`
				} `json:"statements"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			db, err := p.getDB()
			if err != nil {
				return tool.Result{}, err
			}

			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to begin transaction: %w", err)
			}

			results := make([]map[string]any, len(in.Statements))

			for i, stmt := range in.Statements {
				result, err := tx.ExecContext(ctx, stmt.Query, stmt.Params...)
				if err != nil {
					_ = tx.Rollback() // #nosec G104 -- best-effort rollback, original error takes precedence
					return tool.Result{}, fmt.Errorf("statement %d failed: %w", i, err)
				}

				rowsAffected, _ := result.RowsAffected()
				results[i] = map[string]any{
					"statement":     i,
					"rows_affected": rowsAffected,
				}
			}

			if err := tx.Commit(); err != nil {
				return tool.Result{}, fmt.Errorf("commit failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
				"results": results,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Helpers
// ============================================================================

func isSelectQuery(query string) bool {
	q := strings.TrimSpace(strings.ToUpper(query))
	return strings.HasPrefix(q, "SELECT") || strings.HasPrefix(q, "WITH")
}

func scanRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for readability
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	return results, rows.Err()
}
