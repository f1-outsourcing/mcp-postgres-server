package handler

import (
	"encoding/csv"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	"github.com/jmoiron/sqlx"
)

const (
	StatementTypeNoExplainCheck = ""
	StatementTypeSelect         = "SELECT"
	StatementTypeInsert         = "INSERT"
	StatementTypeUpdate         = "UPDATE"
	StatementTypeDelete         = "DELETE"
)

// ExplainResult represents one row of an EXPLAIN result
type ExplainResult struct {
	Id           *string `db:"id"`
	SelectType   *string `db:"select_type"`
	Table        *string `db:"table"`
	Partitions   *string `db:"partitions"`
	Type         *string `db:"type"`
	PossibleKeys *string `db:"possible_keys"`
	Key          *string `db:"key"`
	KeyLen       *string `db:"key_len"`
	Ref          *string `db:"ref"`
	Rows         *string `db:"rows"`
	Filtered     *string `db:"filtered"`
	Extra        *string `db:"Extra"`
}

// SetDB overrides the connection pool (used by tests)
func (h *PostgresHandler) SetDB(db *sqlx.DB) {
	h.db = db
}

// DB returns the shared connection pool, establishing it lazily from the configured DSN
func (h *PostgresHandler) DB() (*sqlx.DB, error) {
	if h.db != nil {
		return h.db, nil
	}

	db, err := sqlx.Connect("postgres", h.dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to establish database connection: %v", err)
	}

	h.db = db

	return db, nil
}

// HandleQuery runs a query and returns the result as CSV
func (h *PostgresHandler) HandleQuery(query, expect string) (string, error) {
	result, headers, err := h.DoQuery(query, expect)
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
func (h *PostgresHandler) DoQuery(query, expect string) ([]map[string]interface{}, []string, error) {
	db, err := h.DB()
	if err != nil {
		return nil, nil, err
	}

	if len(expect) > 0 {
		if err := h.HandleExplain(query, expect); err != nil {
			return nil, nil, err
		}
	}

	rows, err := db.Queryx(query)
	if err != nil {
		return nil, nil, err
	}

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
func (h *PostgresHandler) HandleExec(query, expect string) (string, error) {
	db, err := h.DB()
	if err != nil {
		return "", err
	}

	if len(expect) > 0 {
		if err := h.HandleExplain(query, expect); err != nil {
			return "", err
		}
	}

	result, err := db.Exec(query)
	if err != nil {
		return "", err
	}

	ra, err := result.RowsAffected()
	if err != nil {
		return "", err
	}

	switch expect {
	case StatementTypeInsert:
		li, err := result.LastInsertId()
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("%d rows affected, last insert id: %d", ra, li), nil
	default:
		return fmt.Sprintf("%d rows affected", ra), nil
	}
}

// HandleExplain checks the query plan of the given query matches the expected statement type
func (h *PostgresHandler) HandleExplain(query, expect string) error {
	if !h.withExplainCheck {
		return nil
	}

	db, err := h.DB()
	if err != nil {
		return err
	}

	rows, err := db.Queryx(fmt.Sprintf("EXPLAIN %s", query))
	if err != nil {
		return err
	}

	result := []ExplainResult{}
	for rows.Next() {
		var row ExplainResult
		if err := rows.StructScan(&row); err != nil {
			return err
		}
		result = append(result, row)
	}

	if len(result) != 1 {
		return fmt.Errorf("unable to check query plan, denied")
	}

	match := false
	switch expect {
	case StatementTypeInsert:
		fallthrough
	case StatementTypeUpdate:
		fallthrough
	case StatementTypeDelete:
		if *result[0].SelectType == expect {
			match = true
		}
	default:
		// for SELECT type query, the select_type will be multiple values
		// here we check if it's not INSERT, UPDATE or DELETE
		match = true
		for _, typ := range []string{StatementTypeInsert, StatementTypeUpdate, StatementTypeDelete} {
			if *result[0].SelectType == typ {
				match = false
				break
			}
		}
	}

	if !match {
		return fmt.Errorf("query plan does not match expected pattern, denied")
	}

	return nil
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
