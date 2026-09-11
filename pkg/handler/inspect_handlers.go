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

// routineTypeLabel maps information_schema.routines.ROUTINE_TYPE (MariaDB) to
// a readable kind.
func routineTypeLabel(kind string) string {
	switch kind {
	case "FUNCTION":
		return "function"
	case "PROCEDURE":
		return "procedure"
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
		if h.dialect == DialectMariadb {
			schema = database
		} else {
			schema = "public"
		}
	}
	if !isSQLIdentifier(schema) {
		return nil, fmt.Errorf("list_functions: invalid schema name: %s", schema)
	}

	var kindLabel func(v interface{}) string
	var emptyMsg string
	switch h.dialect {
	case DialectMariadb:
		// MariaDB: routines (functions + procedures) live in
		// information_schema.routines; per-parameter details come from
		// information_schema.parameters. MariaDB has no aggregate/window
		// function objects, so only these two kinds appear.
		kindLabel = func(v interface{}) string { return routineTypeLabel(fmt.Sprintf("%v", v)) }
		emptyMsg = "(no functions or procedures in schema " + schema + ")"
	default:
		kindLabel = func(v interface{}) string { return proKindLabel(fmt.Sprintf("%v", v)) }
		emptyMsg = "(no functions, procedures or aggregates in schema " + schema + ")"
	}

	var sql string
	if h.dialect == DialectMariadb {
		sql = `SELECT
    r.ROUTINE_SCHEMA AS schema,
    r.ROUTINE_NAME   AS name,
    r.ROUTINE_TYPE   AS kind,
    COALESCE(GROUP_CONCAT(p.PARAMETER_NAME ORDER BY p.ORDINAL_POSITION SEPARATOR ', '), '') AS args,
    MAX(IF(p.PARAMETER_MODE = 'RETURN', CONCAT(p.DATA_TYPE, IFNULL(CONCAT('(', p.CHARACTER_MAXIMUM_LENGTH, ')'), '')), NULL)) AS returns,
    'SQL'            AS language,
    r.DEFINER        AS owner
FROM information_schema.routines r
LEFT JOIN information_schema.parameters p
       ON p.SPECIFIC_SCHEMA = r.ROUTINE_SCHEMA
      AND p.SPECIFIC_NAME   = r.SPECIFIC_NAME
WHERE r.ROUTINE_SCHEMA = '` + schema + `'
GROUP BY r.ROUTINE_SCHEMA, r.ROUTINE_NAME, r.ROUTINE_TYPE, r.DEFINER
ORDER BY r.ROUTINE_SCHEMA, r.ROUTINE_NAME;`
	} else {
		sql = `SELECT
    p.proname        AS name,
    n.nspname        AS schema,
    pg_get_function_arguments(p.oid) AS args,
    pg_get_function_result(p.oid)   AS returns,
    p.prokind        AS kind,
    l.lanname        AS language,
    pg_get_userbyid(p.proowner) AS owner
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
JOIN pg_language l  ON l.oid = p.prolang
WHERE n.nspname = '` + schema + `'
  AND p.prokind IN ('f', 'p', 'a', 'w')
ORDER BY n.nspname, p.proname, pg_get_function_arguments(p.oid);`
	}

	rows, _, err := h.DoQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("list_functions: %w", err)
	}

	if len(rows) == 0 {
		return textResponse(emptyMsg), nil
	}

	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		line := fmt.Sprintf("%s %s.%s(%s) -> %s | language=%s, owner=%s",
			kindLabel(r["kind"]),
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
		if h.dialect == DialectMariadb {
			schema = database
		} else {
			schema = "public"
		}
	}
	if !isSQLIdentifier(schema) {
		return nil, fmt.Errorf("desc_function: invalid schema name: %s", schema)
	}

	wantedArgs, hasArgs, err := parseOptionalStringParam(args, "args")
	if err != nil {
		return nil, err
	}

	// MariaDB: information_schema.routines.ROUTINE_DEFINITION carries the
	// body only; SHOW CREATE FUNCTION/PROCEDURE gives full DDL including
	// DETERMINISTIC / SQL SECURITY clauses. We emulate that via a small
	// dynamic query: first check which kind of routine this is, then emit
	// the appropriate SHOW CREATE.
	if h.dialect == DialectMariadb {
		// Use information_schema.routines for the body. For functions
		// ROUTINE_DEFINITION holds the full SQL; for procedures it may be
		// NULL (multi-statement body) — in that case fall back to
		// SHOW CREATE PROCEDURE and grab the Body column.
		rows, _, err := h.DoQuery(database,
			"SELECT ROUTINE_TYPE, DATA_TYPE, IS_DETERMINISTIC, SECURITY_TYPE, ROUTINE_DEFINITION "+
				"FROM information_schema.routines "+
				"WHERE ROUTINE_SCHEMA = DATABASE() AND ROUTINE_NAME = `"+name+"` "+
				"LIMIT 1")
		if err != nil {
			return nil, fmt.Errorf("desc_function: %w", err)
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("desc_function: routine %s.%s not found in database %s", schema, name, database)
		}
		rtype := fmt.Sprintf("%v", rows[0]["ROUTINE_TYPE"])
		def := rows[0]["ROUTINE_DEFINITION"]

		if def == nil || fmt.Sprintf("%v", def) == "<nil>" || fmt.Sprintf("%v", def) == "NULL" {
			// Procedure body — ROUTINE_DEFINITION is NULL; use SHOW CREATE PROCEDURE.
			srows, kheaders, err := h.DoQuery(database, "SHOW CREATE PROCEDURE `"+name+"`")
			if err != nil {
				return nil, fmt.Errorf("desc_function: %w", err)
			}
			if len(srows) == 0 {
				return nil, fmt.Errorf("desc_function: procedure %s.%s not found", schema, name)
			}
			last := kheaders[len(kheaders)-1]
			def = srows[0][last]
		}

		return textResponse(fmt.Sprintf(
			"-- %s %s()\n-- returns: %v | deterministic: %v | security: %v\n%v",
			rtype, name, rows[0]["DATA_TYPE"], rows[0]["IS_DETERMINISTIC"], rows[0]["SECURITY_TYPE"], def,
		)), nil
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

	var sql string
	switch h.dialect {
	case DialectMariadb:
		// MariaDB: information_schema.triggers already exposes timing /
		// event as plain columns (no bitflag decoding needed). Trigger
		// "enabled" state is not tracked — the column is omitted, and the
		// row rendering below adapts via presence checks.
		tableFilter := ""
		if hasTable {
			tableFilter = "\n  AND tr.EVENT_OBJECT_TABLE = '" + table + "'"
		}
		sql = `SELECT
    tr.EVENT_OBJECT_SCHEMA AS schema_name,
    tr.EVENT_OBJECT_TABLE AS table_name,
    tr.TRIGGER_NAME       AS trigger_name,
    tr.ACTION_TIMING      AS timing,
    tr.EVENT_MANIPULATION AS event,
    tr.ACTION_STATEMENT   AS function_name,
    1                     AS on_row
FROM information_schema.triggers tr
WHERE tr.EVENT_OBJECT_SCHEMA NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')` + tableFilter + `
ORDER BY tr.EVENT_OBJECT_TABLE, tr.TRIGGER_NAME;`
	default:
		tableFilter := ""
		if hasTable {
			tableFilter = fmt.Sprintf("\n  AND c.relname = '%s'", table)
		}
		sql = `SELECT
    c.relname  AS table_name,
    t.tgname   AS trigger_name,
    (t.tgtype & 2) = 0 AS timing_before,
    (t.tgtype & 2)  > 0 AS timing_after,
    (t.tgtype & 4)  > 0 AS timing_insteadof,
    (t.tgtype & 1)  > 0 AS on_row,
    (t.tgtype & 16) > 0 AS on_insert,
    (t.tgtype & 32) > 0 AS on_delete,
    (t.tgtype & 64) > 0 AS on_update,
    (t.tgtype & 128) > 0 AS on_truncate,
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
	}

	rows, _, err := h.DoQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("list_triggers: %w", err)
	}

	var b strings.Builder

	if len(rows) == 0 {
		b.WriteString("(no table triggers)\n")
	} else if h.dialect == DialectMariadb {
		for _, r := range rows {
			b.WriteString(fmt.Sprintf("%s / %s — %s %s — ON %s -> %s\n",
				r["table_name"], r["trigger_name"],
				r["timing"], // BEFORE | AFTER
				"ROW",
				r["event"],  // INSERT | UPDATE | DELETE
				r["function_name"],
			))
		}
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

	if h.dialect == DialectPostgres {
		// Database-level event triggers are Postgres-only; MariaDB has no
		// counterpart, so the section is skipped for that dialect.
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

	// MariaDB: SHOW CREATE TRIGGER returns the full DDL directly; the
	// first column of the result is the trigger name and the last is the
	// SQL. We take the last column.
	if h.dialect == DialectMariadb {
		showStmt := "SHOW CREATE TRIGGER `" + trigger + "`"
		rows, _, err := h.DoQuery(database, showStmt)
		if err != nil {
			return nil, fmt.Errorf("desc_trigger: %w", err)
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("desc_trigger: trigger %s not found on table %s", trigger, table)
		}
		// MariaDB SHOW CREATE TRIGGER columns: Trigger | SQL (the DDL is
		// the second column, named "SQL" in MariaDB and "Body"/"Statement"
		// in older MySQL). Try known names, then fall back to value scan.
		r0 := rows[0]
		for _, key := range []string{"SQL", "Body", "Statement"} {
			if v, ok := r0[key]; ok && v != nil {
				return textResponse(fmt.Sprintf("%v", v)), nil
			}
		}
		// Fallback: pick the longest string value (the DDL is the largest field).
		best := ""
		for _, v := range r0 {
			if s, ok := v.(string); ok && len(s) > len(best) {
				best = s
			}
		}
		return textResponse(best), nil
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
		if h.dialect == DialectMariadb {
			schema = database
		} else {
			schema = "public"
		}
	}
	if !isSQLIdentifier(schema) {
		return nil, fmt.Errorf("list_sequences: invalid schema name: %s", schema)
	}

	var emptyMsg string
	if h.dialect == DialectMariadb {
		// MariaDB 10.3+ has CREATE SEQUENCE objects, surfaced through a
		// custom `SEQUENCES` table in information_schema. The exact column
		// set varies slightly between MariaDB versions (10.3 vs 10.5+);
		// SELECT * then picks up whichever columns are present.
		emptyMsg = "(no sequences in schema " + schema + " — requires MariaDB 10.3 or newer)"
	} else {
		emptyMsg = "(no sequences in schema " + schema + ")"
	}

	var sql string
	switch h.dialect {
	case DialectMariadb:
		// Stable, version-agnostic discovery: both MariaDB >= 10.3 and
		// MySQL >= 8.4 expose sequence objects in information_schema.TABLES
		// with TABLE_TYPE = 'SEQUENCE'. The physical catalog tables
		// (MariaDB SEQ_10_x_x_SEQUENCES, MySQL information_schema.SEQUENCES)
		// differ per flavor/version — never reference them directly.
		nrows, _, nerr := h.DoQuery(database,
			"SELECT TABLE_NAME FROM information_schema.TABLES "+
				"WHERE TABLE_SCHEMA = '"+database+"' AND TABLE_TYPE = 'SEQUENCE' "+
				"ORDER BY TABLE_NAME")
		if nerr != nil {
			return nil, fmt.Errorf("list_sequences: %w", nerr)
		}
		if len(nrows) == 0 {
			return textResponse(emptyMsg), nil
		}

		// Per-sequence full definition: SHOW CREATE SEQUENCE is stable on
		// MariaDB 10.3+ and MySQL 8.4+. Read generically via headers so
		// neither engine's exact column set is assumed. If it is unusable
		// on a build, fall back to the bare name.
		lines := make([]string, 0, len(nrows))
		for _, r := range nrows {
			name := fmt.Sprintf("%v", r["TABLE_NAME"])
			defs, hdefs, derr := h.DoQuery(database, "SHOW CREATE SEQUENCE `"+name+"`")
			if derr != nil || len(defs) == 0 || len(hdefs) == 0 {
				lines = append(lines, database+"."+name)
				continue
			}
			// Both engines put the full CREATE statement in the last column.
			lines = append(lines, database+"."+name+": "+fmt.Sprintf("%v", defs[0][hdefs[len(hdefs)-1]]))
		}
		return textResponse(strings.Join(lines, "\n")), nil
	default:
		sql = `SELECT
    s.relname AS name,
    n.nspname AS schema,
    seq.seqstart     AS start_value,
    seq.seqmin       AS minimum_value,
    seq.seqmax       AS maximum_value,
    seq.seqincrement AS increment_by,
    seq.seqcycle     AS cycle,
    seq.seqcache     AS cache_size
FROM pg_class s
JOIN pg_namespace n ON n.oid = s.relnamespace
JOIN pg_sequence seq ON seq.seqrelid = s.oid
WHERE n.nspname = '` + schema + `'
ORDER BY n.nspname, s.relname;`
	}

	// PostgreSQL-only: the MariaDB branch returns earlier in the switch
	// (via SHOW CREATE SEQUENCE), so `sql` is only non-empty for Postgres here.
	rows, _, err := h.DoQuery(database, sql)
	if err != nil {
		return nil, fmt.Errorf("list_sequences: %w", err)
	}

	if len(rows) == 0 {
		return textResponse(emptyMsg), nil
	}

	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%s.%s — START %v, INCREMENT %v, MIN %v, MAX %v, CACHE %v, CYCLE %v",
			r["schema"], r["name"], r["start_value"], r["increment_by"],
			r["minimum_value"], r["maximum_value"], r["cache_size"], r["cycle"]))
	}

	return textResponse(strings.Join(lines, "\n")), nil
}
