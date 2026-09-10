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

	rows, _, err := h.DoQuery(database, "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name;", StatementTypeNoExplainCheck)
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

	descsql :=
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

	rows, _, err := h.DoQuery(database, descsql, StatementTypeNoExplainCheck)
	if err != nil {
		return nil, fmt.Errorf("desc_table: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("desc_table: table %s not found", table)
	}

	return textResponse(fmt.Sprintf("%v", rows[0]["?column?"])), nil
}

func (h *PostgresHandler) handleReadQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	result, err := h.HandleQuery(database, query, StatementTypeSelect)
	if err != nil {
		return nil, fmt.Errorf("select_query: %w", err)
	}

	return textResponse(result), nil
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

	result, err := h.HandleQuery(database, "SELECT count(1) from "+table+";", StatementTypeNoExplainCheck)
	if err != nil {
		return nil, fmt.Errorf("count_query: %w", err)
	}

	return textResponse(result), nil
}
