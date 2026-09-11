#!/bin/bash
# ==============================================================================
# test-select_query.sh — select_query tool (SELECT 1 AS one) (both dialects)
#
# Usage: bash test-select_query.sh
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

do_test() {
    local label="$1" dsn="$2" db="$3"
    section "select_query → SELECT 1 AS one [$label]"
    local payload resp
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"select_query","arguments":{"database":"%s","query":"SELECT 1 AS one"}},"id":4}' "$db")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "select_query [$label]" "$resp"; then
        return
    fi
    if echo "$resp" | jq -e '.result.content[0].type=="text"' >/dev/null 2>&1; then
        local out; out=$(echo "$resp" | jq -r '.result.content[0].text')
        if echo "$out" | grep -q 'one'; then
            pass "select_query → column 'one' present"
        else
            fail "select_query → column 'one' missing: $(echo "$out" | head -2)"
        fi
    else
        fail "select_query → no result: $(echo "$resp" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
    fi
}

do_test "PostgreSQL" "$PG_DSN" "$PG_DB"
do_test "MariaDB"    "$MARIADB_DSN" "$MARIADB_DB"

summary "SELECT_QUERY TEST"
