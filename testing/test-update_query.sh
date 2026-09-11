#!/bin/bash
# ==============================================================================
# test-update_query.sh — update_query tool (no-op WHERE 1=0) (both dialects)
#
# Usage: bash test-update_query.sh   (uses alter_table's table — run that first)
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

do_test() {
    local label="$1" dsn="$2" db="$3"
    section "update_query → no-op WHERE 1=0 [$label]"
    local q payload resp
    q="UPDATE ${db} SET id=id WHERE 1=0"
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"update_query","arguments":{"database":"%s","query":"%s"}},"id":7}' "$db" "$q")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "update_query [$label]" "$resp"; then
        return
    fi
    if echo "$resp" | jq -e '.result.content[0].type=="text"' >/dev/null 2>&1; then
        pass "update_query → write path OK (0 rows updated)"
    elif echo "$resp" | jq -e '.error' >/dev/null 2>&1; then
        fail "update_query → MCP error: $(echo "$resp" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
    else
        fail "update_query → no result and no error"
    fi
}

do_test "PostgreSQL" "$PG_DSN" "$PG_DB"
do_test "MariaDB"    "$MARIADB_DSN" "$MARIADB_DB"

summary "UPDATE_QUERY TEST"
