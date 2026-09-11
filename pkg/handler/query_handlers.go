package handler

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// handleListTable lists public-schema tables one per line, no CSV header.
func (h *PostgresHandler) handleListTable(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	var sql string
	switch h.dialect {
	case DialectMariadb:
		// MariaDB has no "public" schema (schema == database); list base
		// tables of the current database, excluding system schemas.
		sql = `SELECT table_name FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
  AND table_type = 'BASE TABLE'
ORDER BY table_name;`
	default:
		sql = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name;"
	}

	rows, _, err := h.DoQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("list_tables: %w", err)
	}

	names := make([]string, 0, len(rows))
	for _, r := range rows {
		if v, ok := r["table_name"]; ok {
			names = append(names, fmt.Sprintf("%v", v))
		}
	}

	return textResponse(strings.Join(names, "\n")), nil
}

func (h *PostgresHandler) handleDescTable(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	table, err := parseStringParam(args, "table")
	if err != nil {
		return nil, err
	}

	if !isSQLIdentifier(table) {
		slog.Error("desc_table - invalid table name", "table", table)
		return nil, fmt.Errorf("invalid table name: %s", table)
	}

	var (
		descsql   string
		resultKey string
	)
	switch h.dialect {
	case DialectMariadb:
		// MariaDB: GROUP_CONCAT instead of string_agg; PK detected via
		// constraint_name = 'PRIMARY'; computed column addressed by its alias.
		// Use alias 'ddl' (not 'desc' — reserved in MariaDB).
		// IFNULL guards against SQL NULL leaking through CONCAT (any NULL arg → NULL result).
		descsql =
			`SELECT IFNULL(
    CONCAT('CREATE TABLE ', t.table_name, ' (',
        IFNULL(GROUP_CONCAT(
            CONCAT(c.column_name, ' ', UPPER(c.data_type),
                IFNULL(CONCAT('(', c.character_maximum_length, ')'), ''),
                IF(c.is_nullable = 'NO', ' NOT NULL', '')
            ) SEPARATOR ', '
        ), ''),
        IFNULL(CONCAT(', PRIMARY KEY (',
            (SELECT GROUP_CONCAT(kcu2.column_name ORDER BY kcu2.ordinal_position SEPARATOR ', ')
             FROM information_schema.key_column_usage kcu2
             WHERE kcu2.table_schema = t.table_schema
               AND kcu2.table_name = t.table_name
               AND kcu2.constraint_name = 'PRIMARY'), ')'), ''),
        ');'), 'UNKNOWN') AS ddl
FROM
    information_schema.tables t
JOIN
    information_schema.columns c ON t.table_name = c.table_name AND t.table_schema = c.table_schema
WHERE
    t.table_schema = DATABASE()
  AND t.table_name = '` + table + `'
GROUP BY
    t.table_name;`
		resultKey = "ddl"
	default:
		descsql =
			`SELECT
    'CREATE TABLE ' || t.table_name || ' (' ||
    string_agg(
        c.column_name || ' ' || c.data_type ||
        CASE
            WHEN c.character_maximum_length IS NOT NULL THEN '(' || c.character_maximum_length || ')'
            ELSE ''
        END ||
        CASE
            WHEN c.is_nullable = 'NO' THEN ' NOT NULL'
            ELSE ''
        END, ', '
    ) ||
    ', PRIMARY KEY (' || (
        SELECT string_agg(kcu.column_name, ', ')
        FROM information_schema.key_column_usage kcu
        WHERE kcu.table_name = t.table_name AND kcu.constraint_name LIKE '%_pkey'
    ) || ')' ||
    ');'
FROM
    information_schema.tables t
JOIN
    information_schema.columns c ON t.table_name = c.table_name
WHERE
    t.table_name = '` + table + `'
GROUP BY
    t.table_name;`
		// lib/pq returns unnamed computed columns under the key "?column?".
		resultKey = "?column?"
	}

	rows, _, err := h.DoQuery(database, descsql)
	if err != nil {
		return nil, fmt.Errorf("desc_table: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("desc_table: table %s not found", table)
	}

	return textResponse(fmt.Sprintf("%v", rows[0][resultKey])), nil
}

func (h *PostgresHandler) handleSelectQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	explain, _ := parseBoolParam(args, "explain")

	result, err := h.HandleQuery(database, query)
	if err != nil {
		return nil, fmt.Errorf("select_query: %w", err)
	}

	if !explain {
		return textResponse(result), nil
	}

	plan, err := h.ExplainPlan(database, query, true /* analyze: safe for SELECT */)
	if err != nil {
		return nil, fmt.Errorf("select_query (explain): %w", err)
	}

	return textResponse("--- QUERY PLAN ---\n" + plan + "\n\n--- RESULTS ---\n" + result), nil
}

func (h *PostgresHandler) handleCountQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	table, err := parseStringParam(args, "table")
	if err != nil {
		return nil, err
	}

	if !isSQLIdentifier(table) {
		slog.Error("count_query - invalid table name", "table", table)
		return nil, fmt.Errorf("invalid table name: %s", table)
	}

	sql := "SELECT count(1) from " + table + ";"

	result, err := h.HandleQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("count_query: %w", err)
	}

	explain, _ := parseBoolParam(args, "explain")
	if !explain {
		return textResponse(result), nil
	}

	plan, err := h.ExplainPlan(database, sql, true /* analyze: safe for SELECT */)
	if err != nil {
		return nil, fmt.Errorf("count_query (explain): %w", err)
	}

	return textResponse("--- QUERY PLAN ---\n" + plan + "\n\n--- RESULTS ---\n" + result), nil
}
