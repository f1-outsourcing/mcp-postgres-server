package handler

import (
	"fmt"
	"log/slog"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

func (h *PostgresHandler) handleListDatabase(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	result, err := h.HandleQuery("SELECT datname FROM pg_database WHERE datistemplate = false;", StatementTypeNoExplainCheck)
	if err != nil {
		return nil, fmt.Errorf("list_database: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleListTable(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	result, err := h.HandleQuery("SELECT table_schema,table_name FROM information_schema.tables ORDER BY table_schema,table_name;", StatementTypeNoExplainCheck)
	if err != nil {
		return nil, fmt.Errorf("list_table: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleDescTable(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	name, err := parseStringParam(args, "name")
	if err != nil {
		return nil, err
	}

	if !isSQLIdentifier(name) {
		slog.Error("desc_table - invalid table name", "name", name)
		return nil, fmt.Errorf("invalid table name: %s", name)
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
    ');' AS create_table_sql
FROM
    information_schema.tables t
JOIN
    information_schema.columns c ON t.table_name = c.table_name
WHERE
    t.table_name = '` + name + `'
GROUP BY
    t.table_name;`

	result, err := h.HandleQuery(descsql, StatementTypeNoExplainCheck)
	if err != nil {
		return nil, fmt.Errorf("desc_table: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleReadQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	result, err := h.HandleQuery(query, StatementTypeSelect)
	if err != nil {
		return nil, fmt.Errorf("read_query: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleCountQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	name, err := parseStringParam(args, "name")
	if err != nil {
		return nil, err
	}

	if !isSQLIdentifier(name) {
		slog.Error("count_query - invalid table name", "name", name)
		return nil, fmt.Errorf("invalid table name: %s", name)
	}

	result, err := h.HandleQuery("SELECT count(1) from "+name+";", StatementTypeNoExplainCheck)
	if err != nil {
		return nil, fmt.Errorf("count_query: %w", err)
	}

	return textResponse(result), nil
}
