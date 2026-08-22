package handler

import (
	"fmt"
	"log/slog"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// note: these handlers only take effect when the handler is not in read-only mode.

func (h *PostgresHandler) handleCreateTable(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error("create_table - server is read-only")
		return nil, fmt.Errorf("create_table: server is read-only, cannot execute write operations")
	}

	result, err := h.HandleExec(query, StatementTypeNoExplainCheck)
	if err != nil {
		slog.Error("create_table - failed", "error", err)
		return nil, fmt.Errorf("create_table: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleAlterTable(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error("alter_table - server is read-only")
		return nil, fmt.Errorf("alter_table: server is read-only, cannot execute write operations")
	}

	result, err := h.HandleExec(query, StatementTypeNoExplainCheck)
	if err != nil {
		slog.Error("alter_table - failed", "error", err)
		return nil, fmt.Errorf("alter_table: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleWriteQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error("write_query - server is read-only")
		return nil, fmt.Errorf("write_query: server is read-only, cannot execute write operations")
	}

	result, err := h.HandleExec(query, StatementTypeInsert)
	if err != nil {
		slog.Error("write_query - failed", "error", err)
		return nil, fmt.Errorf("write_query: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleUpdateQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error("update_query - server is read-only")
		return nil, fmt.Errorf("update_query: server is read-only, cannot execute write operations")
	}

	result, err := h.HandleExec(query, StatementTypeUpdate)
	if err != nil {
		slog.Error("update_query - failed", "error", err)
		return nil, fmt.Errorf("update_query: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleDeleteQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error("delete_query - server is read-only")
		return nil, fmt.Errorf("delete_query: server is read-only, cannot execute write operations")
	}

	result, err := h.HandleExec(query, StatementTypeDelete)
	if err != nil {
		slog.Error("delete_query - failed", "error", err)
		return nil, fmt.Errorf("delete_query: %w", err)
	}

	return textResponse(result), nil
}
