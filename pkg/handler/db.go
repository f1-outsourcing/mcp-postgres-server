package handler

import (
	"encoding/csv"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/lib/pq"
	"github.com/jmoiron/sqlx"
)

// SetDB overrides the connection pool (used by tests)
func (h *PostgresHandler) SetDB(db *sqlx.DB) {
	h.db = db
}

// replaceDatabaseName returns the dsn with its database (url path) segment
// replaced by dbName, preserving any other url components (query params, etc.).
// DSNs without a url path are returned unchanged.
func replaceDatabaseName(dsn, dbName string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	if !(u.Scheme == "postgres" || u.Scheme == "postgresql") {
		return dsn
	}
	u.Path = "/" + dbName
	return u.String()
}

// DB returns a shared connection pool for the given database, establishing it
// lazily by swapping the database name out of the configured DSN. Pools are
// keyed by database name so multiple databases can coexist within the process.
func (h *PostgresHandler) DB(dbName string) (*sqlx.DB, error) {
	// A single injected pool (see SetDB) short-circuits per-db pooling — tests only.
	if h.db != nil {
		return h.db, nil
	}

	if h.dbPools == nil {
		h.dbPools = map[string]*sqlx.DB{}
	}
	if pool, ok := h.dbPools[dbName]; ok {
		return pool, nil
	}

	poolDsn := replaceDatabaseName(h.dsn, dbName)
	db, err := sqlx.Connect("postgres", poolDsn)
	if err != nil {
		return nil, fmt.Errorf("failed to establish database connection: %v", err)
	}

	h.dbPools[dbName] = db

	return db, nil
}

// HandleQuery runs a query and returns the result as CSV
func (h *PostgresHandler) HandleQuery(dbName, query string) (string, error) {
	result, headers, err := h.DoQuery(dbName, query)
	if err != nil {
		return "", err
	}

	s, err := MapToCSV(result, headers)
	if err != nil {
		return "", err
	}

	return s, nil
}

// DoQuery runs a query and returns the raw result rows with their column headers
func (h *PostgresHandler) DoQuery(dbName, query string) ([]map[string]interface{}, []string, error) {
	db, err := h.DB(dbName)
	if err != nil {
		return nil, nil, err
	}

	rows, err := db.Queryx(query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	result := []map[string]interface{}{}
	for rows.Next() {
		row, err := rows.SliceScan()
		if err != nil {
			return nil, nil, err
		}

		resultRow := map[string]interface{}{}
		for i, col := range cols {
			switch v := row[i].(type) {
			case []byte:
				resultRow[col] = string(v)
			default:
				resultRow[col] = v
			}
		}
		result = append(result, resultRow)
	}

	return result, cols, nil
}

// HandleExec runs a write statement and reports how many rows were affected
func (h *PostgresHandler) HandleExec(dbName, query string) (string, error) {
	db, err := h.DB(dbName)
	if err != nil {
		return "", err
	}

	result, err := db.Exec(query)
	if err != nil {
		return "", err
	}

	ra, err := result.RowsAffected()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d rows affected", ra), nil
}

// ExplainPlan returns a human-readable query plan for the given statement.
//
// When analyze is true it uses EXPLAIN (ANALYZE, BUFFERS), which EXECUTES the
// statement (appropriate for SELECT); for writes callers must pass
// analyze=false so the statement is only planned, not actually run.
func (h *PostgresHandler) ExplainPlan(dbName, query string, analyze bool) (string, error) {
	db, err := h.DB(dbName)
	if err != nil {
		return "", err
	}

	verb := "EXPLAIN "
	if analyze {
		verb = "EXPLAIN (ANALYZE, BUFFERS) "
	}
	stmt := verb + query

	rows, err := db.Queryx(stmt)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var out strings.Builder
	out.WriteString(strings.Join(cols, " | "))
	out.WriteByte('\n')

	for rows.Next() {
		vals := make([]interface{}, len(cols))
		refs := make([]interface{}, len(cols))
		for i := range cols {
			refs[i] = &vals[i]
		}
		if err := rows.Scan(refs...); err != nil {
			return "", err
		}
		parts := make([]string, len(cols))
		for i, v := range vals {
			switch t := v.(type) {
			case []byte:
				parts[i] = string(t)
			default:
				parts[i] = fmt.Sprintf("%v", t)
			}
		}
		out.WriteString(strings.Join(parts, " | "))
		out.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

// MapToCSV formats query results as CSV
func MapToCSV(m []map[string]interface{}, headers []string) (string, error) {
	var csvBuf strings.Builder
	writer := csv.NewWriter(&csvBuf)

	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("failed to write headers: %v", err)
	}

	for _, item := range m {
		row := make([]string, len(headers))
		for i, header := range headers {
			value, exists := item[header]
			if !exists {
				return "", fmt.Errorf("key '%s' not found in map", header)
			}
			row[i] = fmt.Sprintf("%v", value)
		}
		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("failed to write row: %v", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("error flushing CSV writer: %v", err)
	}

	return csvBuf.String(), nil
}
