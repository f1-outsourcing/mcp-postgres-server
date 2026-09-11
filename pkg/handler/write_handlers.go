package handler

import (
	"fmt"
	"log/slog"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// note: these handlers only take effect when the handler is not in read-only mode.

func (h *PostgresHandler) handleCreateTable(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error("create_table - server is read-only")
		return nil, fmt.Errorf("create_table: server is read-only, cannot execute write operations")
	}

	result, err := h.HandleExec(database, query)
	if err != nil {
		slog.Error("create_table - failed", "error", err)
		return nil, fmt.Errorf("create_table: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleAlterTable(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error("alter_table - server is read-only")
		return nil, fmt.Errorf("alter_table: server is read-only, cannot execute write operations")
	}

	result, err := h.HandleExec(database, query)
	if err != nil {
		slog.Error("alter_table - failed", "error", err)
		return nil, fmt.Errorf("alter_table: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleCreateFunction(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error("create_function - server is read-only")
		return nil, fmt.Errorf("create_function: server is read-only, cannot execute write operations")
	}

	result, err := h.HandleExec(database, query)
	if err != nil {
		slog.Error("create_function - failed", "error", err)
		return nil, fmt.Errorf("create_function: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleCreateTrigger(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error("create_trigger - server is read-only")
		return nil, fmt.Errorf("create_trigger: server is read-only, cannot execute write operations")
	}

	result, err := h.HandleExec(database, query)
	if err != nil {
		slog.Error("create_trigger - failed", "error", err)
		return nil, fmt.Errorf("create_trigger: %w", err)
	}

	return textResponse(result), nil
}

func (h *PostgresHandler) handleInsertQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	return h.runDLQuery(args, "insert_query")
}

func (h *PostgresHandler) handleUpdateQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	return h.runDLQuery(args, "update_query")
}

func (h *PostgresHandler) handleDeleteQuery(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	return h.runDLQuery(args, "delete_query")
}

// runDLQuery is shared by the DML write tools (INSERT/UPDATE/DELETE).
//
// When the optional `explain` flag is set (default false) it returns the
// EXPLAIN plan for the statement instead of executing it. It deliberately does
// NOT use ANALYZE, because EXPLAIN (ANALYZE …) would actually execute the DML —
// this is plan preview only.
func (h *PostgresHandler) runDLQuery(args map[string]interface{}, toolName string) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	query, err := parseStringParam(args, "query")
	if err != nil {
		return nil, err
	}

	if h.readOnly {
		slog.Error(toolName+" - server is read-only")
		return nil, fmt.Errorf("%s: server is read-only, cannot execute write operations", toolName)
	}

	if explain, _ := parseBoolParam(args, "explain"); explain {
		plan, err := h.ExplainPlan(database, query, false /* no ANALYZE: would execute the DML */)
		if err != nil {
			return nil, fmt.Errorf("%s (explain): %w", toolName, err)
		}
		return textResponse("--- QUERY PLAN (not executed) ---\n" + plan), nil
	}

	result, err := h.HandleExec(database, query)
	if err != nil {
		slog.Error(toolName+" - failed", "error", err)
		return nil, fmt.Errorf("%s: %w", toolName, err)
	}

	return textResponse(result), nil
}
