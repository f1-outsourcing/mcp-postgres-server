#!/bin/bash
# ==============================================================================
# test-write_query.sh — insert_query tool (safe 0-row insert) (both dialects)
#
# Usage: bash test-write_query.sh   (uses alter_table's table — run that first)
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

do_test() {
    local label="$1" dsn="$2" db="$3"
    section "insert_query → 0-row safe insert [$label]"
    local q payload resp
    q="INSERT INTO ${db}(id) SELECT id FROM ${db} WHERE 1=0"
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"insert_query","arguments":{"database":"%s","query":"%s"}},"id":6}' "$db" "$q")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "insert_query [$label]" "$resp"; then
        return
    fi
    if echo "$resp" | jq -e '.result.content[0].type=="text"' >/dev/null 2>&1; then
        pass "insert_query → write path OK (0 rows affected)"
    elif echo "$resp" | jq -e '.error' >/dev/null 2>&1; then
        fail "insert_query → MCP error: $(echo "$resp" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
    else
        fail "insert_query → no result and no error"
    fi
}

do_test "PostgreSQL" "$PG_DSN" "$PG_DB"
do_test "MariaDB"    "$MARIADB_DSN" "$MARIADB_DB"

summary "INSERT_QUERY TEST"
