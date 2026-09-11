#!/bin/bash
# ==============================================================================
# test-alter_table.sh — alter_table tool: CREATE TABLE IF NOT EXISTS (both dialects)
#
# This is the SETUP test — run it first so the other tests see the table.
# Usage: bash test-alter_table.sh
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

do_test() {
    local label="$1" dsn="$2" db="$3"
    section "alter_table → CREATE TABLE IF NOT EXISTS $db [$label]"
    local q payload resp
    q="CREATE TABLE IF NOT EXISTS ${db}(id INT PRIMARY KEY, name VARCHAR(64) NOT NULL DEFAULT '')"
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"alter_table","arguments":{"database":"%s","query":"%s"}},"id":2}' "$db" "$q")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "alter_table [$label]" "$resp"; then
        return
    fi
    if echo "$resp" | jq -e '.result' >/dev/null 2>&1; then
        pass "alter_table → DDL OK (table $db ready)"
    else
        fail "alter_table → DDL failed: $(echo "$resp" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
    fi
}

do_test "PostgreSQL" "$PG_DSN" "$PG_DB"
do_test "MariaDB"    "$MARIADB_DSN" "$MARIADB_DB"

summary "ALTER_TABLE TEST"
