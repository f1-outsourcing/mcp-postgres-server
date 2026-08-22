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
			Name:        prefix + "list_database",
			Description: "List all databases in the POSTGRES server",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "list_table",
			Description: "List all tables in the POSTGRES server",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "desc_table",
			Description: "Describe table structure",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Name of the table"
					}
				},
				"required": ["name"]
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "read_query",
			Description: "Execute a read-only SQL query. Make sure you have knowledge of the table structure before writing WHERE conditions. Call `desc_table` first if necessary",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					}
				},
				"required": ["query"]
			}`),
		},
		{
			// Tool Definition
			Name:        prefix + "count_query",
			Description: "Query the number of rows in a certain table.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Name of the table"
					}
				},
				"required": ["name"]
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
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					}
				},
				"required": ["query"]
			}`),
		},
		protocol.Tool{
			Name:        prefix + "alter_table",
			Description: "Alter an existing table in the POSTGRES server. Make sure you have updated comments for each modified column. DO NOT drop table or existing columns!",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					}
				},
				"required": ["query"]
			}`),
		},
		protocol.Tool{
			Name:        prefix + "write_query",
			Description: "Execute a write SQL query. Make sure you have knowledge of the table structure before executing the query. Make sure the data types match the columns' definitions",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					}
				},
				"required": ["query"]
			}`),
		},
		protocol.Tool{
			Name:        prefix + "update_query",
			Description: "Execute an update SQL query. Make sure you have knowledge of the table structure before executing the query. Make sure there is always a WHERE condition. Call `desc_table` first if necessary",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					}
				},
				"required": ["query"]
			}`),
		},
		protocol.Tool{
			Name:        prefix + "delete_query",
			Description: "Execute a delete SQL query. Make sure you have knowledge of the table structure before executing the query. Make sure there is always a WHERE condition. Call `desc_table` first if necessary",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "The SQL query to execute"
					}
				},
				"required": ["query"]
			}`),
		},
	)

	return t
}
