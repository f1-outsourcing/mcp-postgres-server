package handler

import (
	"fmt"
	"strings"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// This file contains introspection tools for functions/procedures, triggers
// and sequences. They are designed for cross-database comparison, so all
// list output is deterministic (sorted) and plain text (easy to diff).

// parseOptionalStringParam returns the parameter value if present and valid,
// otherwise (zero, false). Unset parameters are not an error.
func parseOptionalStringParam(args map[string]interface{}, key string) (string, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return "", false, nil
	}
	v, err := parseStringParam(args, key)
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// proKindLabel maps pg_proc.prokind (PostgreSQL 11+) to a readable kind.
func proKindLabel(kind string) string {
	switch kind {
	case "f":
		return "function"
	case "p":
		return "procedure"
	case "a":
		return "aggregate"
	case "w":
		return "window"
	default:
		return kind
	}
}

func boolVal(v interface{}) bool {
	b, ok := v.(bool)
	return ok && b
}

// ---------------------------------------------------------------------------
// list_functions
// ---------------------------------------------------------------------------

// handleListFunctions lists functions, procedures, aggregates and window
// functions in a schema (default: public) — equivalent info to `psql \dfn`.
func (h *PostgresHandler) handleListFunctions(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	schema, _, err := parseOptionalStringParam(args, "schema")
	if err != nil {
		return nil, err
	}
	if schema == "" {
		schema = "public"
	}
	if !isSQLIdentifier(schema) {
		return nil, fmt.Errorf("list_functions: invalid schema name: %s", schema)
	}

	sql := `SELECT
    p.proname        AS name,
    n.nspname        AS schema,
    pg_get_function_arguments(p.oid) AS args,
    pg_get_function_result(p.oid)   AS returns,
    p.prokind        AS kind,
    l.lanname        AS language,
    u.usename        AS owner
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
JOIN pg_language l  ON l.oid = p.prolang
JOIN pg_authid u    ON u.oid = p.proowner
WHERE n.nspname = '` + schema + `'
  AND p.prokind IN ('f', 'p', 'a', 'w')
ORDER BY n.nspname, p.proname, pg_get_function_arguments(p.oid);`

	rows, _, err := h.DoQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("list_functions: %w", err)
	}

	if len(rows) == 0 {
		return textResponse("(no functions, procedures or aggregates in schema " + schema + ")"), nil
	}

	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		line := fmt.Sprintf("%s %s.%s(%s) -> %s | language=%s, owner=%s",
			proKindLabel(fmt.Sprintf("%v", r["kind"])),
			fmt.Sprintf("%v", r["schema"]),
			fmt.Sprintf("%v", r["name"]),
			fmt.Sprintf("%v", r["args"]),
			fmt.Sprintf("%v", r["returns"]),
			fmt.Sprintf("%v", r["language"]),
			fmt.Sprintf("%v", r["owner"]),
		)
		lines = append(lines, line)
	}

	return textResponse(strings.Join(lines, "\n")), nil
}

// ---------------------------------------------------------------------------
// desc_function
// ---------------------------------------------------------------------------

// handleDescFunction returns the full definition (source) of a single
// function/procedure — equivalent info to `psql \df+`. Overloads are resolved
// via the optional `args` parameter.
func (h *PostgresHandler) handleDescFunction(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	name, err := parseStringParam(args, "function")
	if err != nil {
		return nil, err
	}
	if !isSQLIdentifier(name) {
		return nil, fmt.Errorf("desc_function: invalid function name: %s", name)
	}

	schema, _, err := parseOptionalStringParam(args, "schema")
	if err != nil {
		return nil, err
	}
	if schema == "" {
		schema = "public"
	}
	if !isSQLIdentifier(schema) {
		return nil, fmt.Errorf("desc_function: invalid schema name: %s", schema)
	}

	wantedArgs, hasArgs, err := parseOptionalStringParam(args, "args")
	if err != nil {
		return nil, err
	}

	sql := `SELECT
    n.nspname AS schema,
    p.proname AS name,
    pg_get_function_arguments(p.oid) AS args,
    pg_get_functiondef(p.oid)        AS definition
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE p.proname = '` + name + `'
  AND n.nspname = '` + schema + `'
ORDER BY p.oid;`

	rows, _, err := h.DoQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("desc_function: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("desc_function: function %s.%s not found", schema, name)
	}

	// Resolve overloads.
	matched := rows
	if hasArgs {
		matched = nil
		for _, r := range rows {
			if fmt.Sprintf("%v", r["args"]) == wantedArgs {
				matched = append(matched, r)
			}
		}
		if len(matched) == 0 {
			avail := make([]string, 0, len(rows))
			for _, r := range rows {
				avail = append(avail, fmt.Sprintf("  %s(%s)", name, r["args"]))
			}
			return nil, fmt.Errorf("desc_function: no overload of %s with args (%q). Available overloads (pass `args` to disambiguate):\n%s",
				name, wantedArgs, strings.Join(avail, "\n"))
		}
	}

	if len(matched) > 1 && !hasArgs {
		avail := make([]string, 0, len(matched))
		for _, r := range matched {
			avail = append(avail, fmt.Sprintf("  %s(%s)", name, r["args"]))
		}
		return textResponse(fmt.Sprintf("Multiple overloads of %s.%s found — pass `args` to pick one:\n%s",
			schema, name, strings.Join(avail, "\n"))), nil
	}

	return textResponse(fmt.Sprintf("%v", matched[0]["definition"])), nil
}

// ---------------------------------------------------------------------------
// list_triggers
// ---------------------------------------------------------------------------

// handleListTriggers lists table triggers (plus database-level event
// triggers) — equivalent info to the trigger section of `psql \d <table>`.
// The optional `table` parameter restricts the listing to one table.
func (h *PostgresHandler) handleListTriggers(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	table, hasTable, err := parseOptionalStringParam(args, "table")
	if err != nil {
		return nil, err
	}
	if hasTable && !isSQLIdentifier(table) {
		return nil, fmt.Errorf("list_triggers: invalid table name: %s", table)
	}

	tableFilter := ""
	if hasTable {
		tableFilter = fmt.Sprintf("\n  AND c.relname = '%s'", table)
	}

	sql := `SELECT
    c.relname  AS table_name,
    t.tgname   AS trigger_name,
    (t.tgtype & 2048) > 0 AS timing_before,
    (t.tgtype & 4096) > 0 AS timing_after,
    (t.tgtype & 8192) > 0 AS timing_insteadof,
    (t.tgtype & 16)  > 0 AS on_row,
    (t.tgtype & 32)  > 0 AS on_insert,
    (t.tgtype & 64)  > 0 AS on_delete,
    (t.tgtype & 128) > 0 AS on_update,
    (t.tgtype & 256) > 0 AS on_truncate,
    t.tgenabled  AS enabled,
    fn.nspname || '.' || fp.proname AS function_name
FROM pg_trigger t
JOIN pg_class c ON c.oid = t.tgrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_proc fp ON fp.oid = t.tgfoid
JOIN pg_namespace fn ON fn.oid = fp.pronamespace
WHERE NOT t.tgisinternal
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')` + tableFilter + `
ORDER BY c.relname, t.tgname;`

	rows, _, err := h.DoQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("list_triggers: %w", err)
	}

	var b strings.Builder

	if len(rows) == 0 {
		b.WriteString("(no table triggers)\n")
	} else {
		for _, r := range rows {
			b.WriteString(fmt.Sprintf("%s / %s — ", r["table_name"], r["trigger_name"]))

			timing := "INSTEAD OF"
			if boolVal(r["timing_before"]) {
				timing = "BEFORE"
			} else if boolVal(r["timing_after"]) {
				timing = "AFTER"
			}
			b.WriteString(timing + " ")

			scope := "STATEMENT"
			if boolVal(r["on_row"]) {
				scope = "ROW"
			}
			b.WriteString(scope + " ")

			var events []string
			if boolVal(r["on_insert"]) {
				events = append(events, "INSERT")
			}
			if boolVal(r["on_update"]) {
				events = append(events, "UPDATE")
			}
			if boolVal(r["on_delete"]) {
				events = append(events, "DELETE")
			}
			if boolVal(r["on_truncate"]) {
				events = append(events, "TRUNCATE")
			}
			b.WriteString("ON " + strings.Join(events, ", ") + " ")

			b.WriteString(fmt.Sprintf("-> %s", r["function_name"]))

			enabled := fmt.Sprintf("%v", r["enabled"])
			b.WriteString("  [enabled: " + enabled + "]")
			b.WriteString("\n")
		}
	}

	// Database-level event triggers (psql has no direct equivalent listing).
	evRows, _, err := h.DoQuery(database, `SELECT evtname, evtevent, evttags, evtenabled FROM pg_event_trigger ORDER BY evtname;`)
	if err != nil {
		return nil, fmt.Errorf("list_triggers (event triggers): %w", err)
	}

	b.WriteString("\n--- EVENT TRIGGERS (database-level) ---\n")
	if len(evRows) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, r := range evRows {
			b.WriteString(fmt.Sprintf("%s — ON %s (tags: %s) [enabled: %s]\n",
				r["evtname"], r["evtevent"], r["evttags"], r["evtenabled"]))
		}
	}

	return textResponse(strings.TrimRight(b.String(), "\n")), nil
}

// ---------------------------------------------------------------------------
// desc_trigger
// ---------------------------------------------------------------------------

// handleDescTrigger returns the full DDL of a single trigger on a table —
// the complete `CREATE TRIGGER ...` statement including the WHEN clause.
func (h *PostgresHandler) handleDescTrigger(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	table, err := parseStringParam(args, "table")
	if err != nil {
		return nil, err
	}
	if !isSQLIdentifier(table) {
		return nil, fmt.Errorf("desc_trigger: invalid table name: %s", table)
	}

	trigger, err := parseStringParam(args, "trigger")
	if err != nil {
		return nil, err
	}
	if !isSQLIdentifier(trigger) {
		return nil, fmt.Errorf("desc_trigger: invalid trigger name: %s", trigger)
	}

	sql := `SELECT pg_get_triggerdef(oid) AS definition
FROM pg_trigger
WHERE tgname = '` + trigger + `'
  AND tgrelid = to_regclass('` + table + `');`

	rows, _, err := h.DoQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("desc_trigger: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("desc_trigger: trigger %s not found on table %s", trigger, table)
	}

	return textResponse(fmt.Sprintf("%v", rows[0]["definition"])), nil
}

// ---------------------------------------------------------------------------
// list_sequences
// ---------------------------------------------------------------------------

// handleListSequences lists sequences in a schema (default: public) with
// their parameters — equivalent info to `psql \ds`.
func (h *PostgresHandler) handleListSequences(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	database, err := parseStringParam(args, "database")
	if err != nil {
		return nil, err
	}

	schema, _, err := parseOptionalStringParam(args, "schema")
	if err != nil {
		return nil, err
	}
	if schema == "" {
		schema = "public"
	}
	if !isSQLIdentifier(schema) {
		return nil, fmt.Errorf("list_sequences: invalid schema name: %s", schema)
	}

	sql := `SELECT
    s.relname AS name,
    n.nspname AS schema,
    seq.start_value,
    seq.minimum_value,
    seq.maximum_value,
    seq.increment_by,
    seq.cycle,
    seq.cache_size
FROM pg_class s
JOIN pg_namespace n ON n.oid = s.relnamespace
JOIN pg_sequence seq ON seq.seqrelid = s.oid
WHERE n.nspname = '` + schema + `'
ORDER BY n.nspname, s.relname;`

	rows, _, err := h.DoQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("list_sequences: %w", err)
	}

	if len(rows) == 0 {
		return textResponse("(no sequences in schema " + schema + ")"), nil
	}

	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%s.%s — START %v, INCREMENT %v, MIN %v, MAX %v, CACHE %v, CYCLE %v",
			r["schema"], r["name"], r["start_value"], r["increment_by"],
			r["minimum_value"], r["maximum_value"], r["cache_size"], r["cycle"]))
	}

	return textResponse(strings.Join(lines, "\n")), nil
}
