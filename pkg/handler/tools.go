package handler

import (
	"encoding/json"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// buildTools returns the list of MCP tools, honoring read-only mode
func (h *PostgresHandler) buildTools() []protocol.Tool {
	prefix := h.prefix

	t := []protocol.Tool{
		{
			// Tool Definition
			Name:        prefix + "list_tables",
			Description: "List all tables in the public schema (pg_catalog and information_schema are excluded)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					}
				},
				"required": ["database"]
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "desc_table",
			Description: "Describe table structure",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"table": {
						"type": "string",
						"description": "Name of the table"
					}
				},
				"required": ["database", "table"]
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "select_query",
			Description: "Execute a read-only SQL query. Make sure you have knowledge of the table structure before writing WHERE conditions. Call `desc_table` first if necessary",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					},
					"explain": {
						"type": "boolean",
						"description": "Optional (default false). When true, also return the query plan (EXPLAIN ANALYZE) alongside the results"
					}
				},
				"required": ["database", "query"]
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "count_query",
			Description: "Query the number of rows in a certain table.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"table": {
						"type": "string",
						"description": "Name of the table"
					},
					"explain": {
						"type": "boolean",
						"description": "Optional (default false). When true, also return the query plan (EXPLAIN ANALYZE)"
					}
				},
				"required": ["database", "table"]
			}`),
		},
	}

	if h.readOnly {
		return t
	}

	t = append(t,
		protocol.Tool{
			Name:        prefix + "create_table",
			Description: "Create a new table in the POSTGRES server. Make sure you have added proper comments for each column and the table itself",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					}
				},
				"required": ["database", "query"]
			}`),
		},
		protocol.Tool{
			Name:        prefix + "alter_table",
			Description: "Alter an existing table in the POSTGRES server. Make sure you have updated comments for each modified column. DO NOT drop table or existing columns!",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					}
				},
				"required": ["database", "query"]
			}`),
		},
		protocol.Tool{
			Name:        prefix + "insert_query",
			Description: "Execute a write SQL query. Make sure you have knowledge of the table structure before executing the query. Make sure the data types match the columns' definitions",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					},
					"explain": {
						"type": "boolean",
						"description": "Optional (default false). When true, return the EXPLAIN plan INSTEAD of executing the statement (not run, only previewed)"
					}
				},
				"required": ["database", "query"]
			}`),
		},
		protocol.Tool{
			Name:        prefix + "update_query",
			Description: "Execute an update SQL query. Make sure you have knowledge of the table structure before executing the query. Make sure there is always a WHERE condition. Call `desc_table` first if necessary",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					},
					"explain": {
						"type": "boolean",
						"description": "Optional (default false). When true, return the EXPLAIN plan INSTEAD of executing the statement (not run, only previewed)"
					}
				},
				"required": ["database", "query"]
			}`),
		},
		protocol.Tool{
			Name:        prefix + "delete_query",
			Description: "Execute a delete SQL query. Make sure you have knowledge of the table structure before executing the query. Make sure there is always a WHERE condition. Call `desc_table` first if necessary",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					},
					"explain": {
						"type": "boolean",
						"description": "Optional (default false). When true, return the EXPLAIN plan INSTEAD of executing the statement (not run, only previewed)"
					}
				},
				"required": ["database", "query"]
			}`),
		},
	)

	return t
}
