#!/bin/bash
# ==============================================================================
# test-delete_query.sh — delete_query tool (WHERE 1=0, safe) (both dialects)
#
# Usage: bash test-delete_query.sh   (uses alter_table's table — run that first)
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

do_test() {
    local label="$1" dsn="$2" db="$3"
    section "delete_query → WHERE 1=0 (safe) [$label]"
    local q payload resp
    q="DELETE FROM ${db} WHERE 1=0"
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"delete_query","arguments":{"database":"%s","query":"%s"}},"id":8}' "$db" "$q")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "delete_query [$label]" "$resp"; then
        return
    fi
    if echo "$resp" | jq -e '.result.content[0].type=="text"' >/dev/null 2>&1; then
        pass "delete_query → write path OK (0 rows deleted)"
    elif echo "$resp" | jq -e '.error' >/dev/null 2>&1; then
        fail "delete_query → MCP error: $(echo "$resp" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
    else
        fail "delete_query → no result and no error"
    fi
}

do_test "PostgreSQL" "$PG_DSN" "$PG_DB"
do_test "MariaDB"    "$MARIADB_DSN" "$MARIADB_DB"

summary "DELETE_QUERY TEST"
