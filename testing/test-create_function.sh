#!/bin/bash
# ==============================================================================
# test-create_function.sh — create_function tool (both dialects)
#
# Exercises the MCP `create_function` tool with a CREATE [OR REPLACE] FUNCTION
# statement (idempotent, so safe to re-run). Dialect-specific DDL is used.
# Cleanup is best-effort via the DB CLI (guarded), mirroring seed/teardown.
#
# Usage: bash test-create_function.sh
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

FN="cf_probe"

do_test() {
    local label="$1" dsn="$2" db="$3" ddl="$4"
    section "create_function [$label] → db $db"
    local jddl payload resp
    jddl=$(printf '%s' "$ddl" | sed 's/\\/\\\\/g; s/"/\\"/g')   # JSON-escape the DDL
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"create_function","arguments":{"database":"%s","query":"%s"}},"id":40}' "$db" "$jddl")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "create_function [$label]" "$resp"; then
        return
    fi
    if echo "$resp" | jq -e '.result' >/dev/null 2>&1; then
        pass "create_function [$label] → $FN created (OR REPLACE)"
    else
        fail "create_function [$label] → failed: $(echo "$resp" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
    fi
}

# ---- PostgreSQL ----
do_test "PostgreSQL" "$PG_DSN" "$PG_DB" \
  "CREATE OR REPLACE FUNCTION ${FN}() RETURNS integer LANGUAGE sql AS 'SELECT 42';"

# ---- MariaDB ----
do_test "MariaDB" "$MARIADB_DSN" "$MARIADB_DB" \
  "CREATE OR REPLACE FUNCTION ${FN}() RETURNS INT DETERMINISTIC RETURN 42;"

# ---- best-effort cleanup (CLI, guarded) ----
pg_user="postgres"; pg_pass="test";  pg_host="127.0.0.1"; pg_port="5432"
md_user="root";     md_pass="test";  md_host="127.0.0.1"; md_port="3306"
if command -v psql  >/dev/null 2>&1; then
    PGPASSWORD="$pg_pass" psql -h "$pg_host" -p "$pg_port" -U "$pg_user" -d "$PG_DB" -c "DROP FUNCTION IF EXISTS ${FN};" >/dev/null 2>&1 \
        && info "cleanup [PG] → ${FN} dropped" || info "cleanup [PG] → ${FN} drop skipped"
fi
if command -v mysql >/dev/null 2>&1; then
    mysql -h "$md_host" -P "$md_port" -u "$md_user" -p"$md_pass" -e "DROP FUNCTION IF EXISTS ${FN};" "$MARIADB_DB" >/dev/null 2>&1 \
        && info "cleanup [MD] → ${FN} dropped" || info "cleanup [MD] → ${FN} drop skipped"
fi

summary "CREATE_FUNCTION TEST"
