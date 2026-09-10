package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gomcpgo/mcp/pkg/protocol"
	"github.com/jmoiron/sqlx"
)

// PostgresHandler implements the MCP handler interfaces for postgres operations
type PostgresHandler struct {
	prefix           string
	dsn              string
	readOnly         bool
	withExplainCheck bool
	db               *sqlx.DB
	dbPools          map[string]*sqlx.DB
}

// NewPostgresHandler creates a new postgres handler with default prefix
func NewPostgresHandler() *PostgresHandler {
	return &PostgresHandler{}
}

// NewPostgresHandlerWithPrefix creates a new postgres handler with custom prefix
func NewPostgresHandlerWithPrefix(prefix string) *PostgresHandler {
	return &PostgresHandler{prefix: prefix}
}

// SetDSN stores the Postgres DSN. The physical connection is opened lazily on
// the first request that needs it (see DB()). No live connect is attempted here
// so the server can start reading stdin immediately — matching the filesystem
// server's non-blocking startup.
func (h *PostgresHandler) SetDSN(dsn string) {
	h.dsn = dsn
}

// SetReadOnly disables write tools (create/alter/write/update/delete) when true
func (h *PostgresHandler) SetReadOnly(readOnly bool) {
	h.readOnly = readOnly
}

// SetWithExplainCheck enables the EXPLAIN query-plan pre-check when true
func (h *PostgresHandler) SetWithExplainCheck(explainCheck bool) {
	h.withExplainCheck = explainCheck
}

// ListTools provides a list of all available tools in the postgres handler
func (h *PostgresHandler) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	return &protocol.ListToolsResponse{Tools: h.buildTools()}, nil
}

// CallTool handles execution of postgres tools
func (h *PostgresHandler) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	slog.Debug("PostgresHandler: Executing tool", "tool", req.Name)

	// this keeps the server respond to the original "select_query" as well.
	// I have the impression vscode/continuedev is buggy with the auto prefix adding.
	actualToolName := req.Name
	if strings.HasPrefix(req.Name, h.prefix) {
		actualToolName = strings.TrimPrefix(req.Name, h.prefix)
	}

	switch actualToolName {
	case "list_tables":
		return h.handleListTable(req.Arguments)
	case "create_table":
		return h.handleCreateTable(req.Arguments)
	case "alter_table":
		return h.handleAlterTable(req.Arguments)
	case "desc_table":
		return h.handleDescTable(req.Arguments)
	case "select_query":
		return h.handleReadQuery(req.Arguments)
	case "count_query":
		return h.handleCountQuery(req.Arguments)
	case "insert_query":
		return h.handleInsertQuery(req.Arguments)
	case "update_query":
		return h.handleUpdateQuery(req.Arguments)
	case "delete_query":
		return h.handleDeleteQuery(req.Arguments)
	default:
		return nil, fmt.Errorf("unknown tool: %s", req.Name)
	}
}
