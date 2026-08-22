package handler

import (
	"context"
	"testing"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// These tests cover the handler API surface without a real database:
// tool filtering (read-only mode), unknown tool dispatch and parameter
// parsing. DB-backed behavior is covered by the shell-based integration
// tests in testing/ (requires PG_DSN).

func TestReadOnlyToolFiltering(t *testing.T) {
	h := NewPostgresHandlerWithPrefix("pg_")

	h.SetReadOnly(false)
	resp, err := h.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if got := len(resp.Tools); got != 10 {
		t.Errorf("rw mode: expected 10 tools, got %d", got)
	}

	h.SetReadOnly(true)
	resp, err = h.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	tools := resp.Tools
	if len(tools) != 5 {
		t.Fatalf("ro mode: expected 5 tools, got %d", len(tools))
	}

	names := map[string]bool{}
	for _, tt := range tools {
		names[tt.Name] = true
	}

	for _, name := range []string{
		"pg_list_database", "pg_list_table", "pg_desc_table",
		"pg_read_query", "pg_count_query",
	} {
		if !names[name] {
			t.Errorf("ro mode: missing read-only tool: %s", name)
		}
	}

	for _, name := range []string{
		"pg_create_table", "pg_alter_table", "pg_write_query",
		"pg_update_query", "pg_delete_query",
	} {
		if names[name] {
			t.Errorf("ro mode: unexpected write tool: %s", name)
		}
	}
}

func TestCallToolUnknownTool(t *testing.T) {
	h := NewPostgresHandlerWithPrefix("pg_")
	req := &protocol.CallToolRequest{Name: "pg_bogus_tool"}
	resp, err := h.CallTool(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error for unknown tool, got response %v", resp)
	}
}

func TestParseStringParam(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		v, err := parseStringParam(map[string]interface{}{"x": "hello"}, "x")
		if err != nil || v != "hello" {
			t.Errorf("got %q, %v; want %q, nil", v, err, "hello")
		}
	})
	t.Run("missing_key", func(t *testing.T) {
		if _, err := parseStringParam(map[string]interface{}{}, "x"); err == nil {
			t.Error("expected error for missing key")
		}
	})
	t.Run("empty_string", func(t *testing.T) {
		if _, err := parseStringParam(map[string]interface{}{"x": ""}, "x"); err == nil {
			t.Error("expected error for empty string")
		}
	})
	t.Run("non_string_value", func(t *testing.T) {
		if _, err := parseStringParam(map[string]interface{}{"x": 42}, "x"); err == nil {
			t.Error("expected error for non-string value")
		}
	})
}
