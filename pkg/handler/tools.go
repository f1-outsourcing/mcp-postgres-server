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
			Description: "List all tables in the database (system schemas are excluded)",
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
			Description: "Describe table structure including column names, types, nullability, defaults, and primary key info",
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
		{
			// Tool Definition
			Name:        prefix + "list_functions",
			Description: "List functions and procedures in the current database. One line per: kind, name, argument signature, return type, language, owner. On PostgreSQL, also includes aggregates and window functions. Sorted for easy cross-database diffing",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to inspect"
					},
					"schema": {
						"type": "string",
						"description": "Optional schema name (default: public)"
					}
				},
				"required": ["database"]
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "desc_function",
			Description: "Return the full definition (source code) of a single function or procedure. If the name is ambiguous, the list of available overloads is returned — re-call with `args` to pick one",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to inspect"
					},
					"function": {
						"type": "string",
						"description": "Name of the function or procedure (unqualified)"
					},
					"schema": {
						"type": "string",
						"description": "Optional schema name (default: public)"
					},
					"args": {
						"type": "string",
						"description": "Optional argument signature to disambiguate overloads, e.g. 'id integer'. Omit to list overloads instead"
					}
				},
				"required": ["database", "function"]
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "list_triggers",
			Description: "List all table triggers (timing, scope, events, target function) on tables in the current database. Restrict with `table` if needed. On PostgreSQL, event triggers are also included. Sorted for easy cross-database diffing",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to inspect"
					},
					"table": {
						"type": "string",
						"description": "Optional table name; when set, only triggers on this table are listed"
					}
				},
				"required": ["database"]
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "desc_trigger",
			Description: "Return the full DDL (CREATE TRIGGER statement) of a single trigger on a table",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to inspect"
					},
					"table": {
						"type": "string",
						"description": "Name of the table the trigger is defined on"
					},
					"trigger": {
						"type": "string",
						"description": "Name of the trigger"
					}
				},
				"required": ["database", "table", "trigger"]
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "list_sequences",
			Description: "List sequences in the current database with their parameters (start, increment, min, max, cycle). On PostgreSQL, also includes the cache value. Sorted for easy cross-database diffing",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to inspect"
					},
					"schema": {
						"type": "string",
						"description": "Optional schema name (default: public)"
					}
				},
				"required": ["database"]
			}`),
		},
	}

	if h.readOnly {
		return t
	}

	t = append(t,
		protocol.Tool{
			Name:        prefix + "create_table",
			Description: "Create a new table in the database server. Make sure you have added proper comments for each column and the table itself",
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
			Description: "Alter an existing table in the database server. Make sure you have updated comments for each modified column. DO NOT drop table or existing columns!",
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
		protocol.Tool{
			Name:        prefix + "create_function",
			Description: "Create a stored function or procedure (CREATE [OR REPLACE] FUNCTION / PROCEDURE). Provide the full DDL. For triggers, call this tool FIRST to create the handler function, then call `create_trigger`",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"query": {
						"type": "string",
						"description": "The full CREATE [OR REPLACE] FUNCTION or CREATE [OR REPLACE] PROCEDURE statement"
					}
				},
				"required": ["database", "query"]
			}`),
		},
		protocol.Tool{
			Name:        prefix + "create_trigger",
			Description: "Create a trigger on a table (CREATE TRIGGER). The handler function MUST already exist — call `create_function` first if it does not. Provide the full CREATE TRIGGER statement",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database": {
						"type": "string",
						"description": "Name of the database to run the query against"
					},
					"query": {
						"type": "string",
						"description": "The full CREATE TRIGGER statement"
					}
				},
				"required": ["database", "query"]
			}`),
		},
	)

	return t
}
