#!/bin/bash
# ==============================================================================
# test-count_query.sh — count_query tool on the test table (both dialects)
# Run AFTER test-alter_table.sh so the table exists.
#
# Usage: bash test-count_query.sh
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

do_test() {
    local label="$1" dsn="$2" db="$3"
    section "count_query on $db [$label]"
    local payload resp
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"count_query","arguments":{"database":"%s","table":"%s"}},"id":5}' "$db" "$db")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "count_query [$label]" "$resp"; then
        return
    fi
    if echo "$resp" | jq -e '.result.content[0].type=="text"' >/dev/null 2>&1; then
        local out; out=$(echo "$resp" | jq -r '.result.content[0].text')
        if echo "$out" | grep -qE '[0-9]'; then
            pass "count_query → returned a count: $(echo "$out" | head -1)"
        else
            fail "count_query → no number in result: $(echo "$out" | head -2)"
        fi
    else
        fail "count_query → no result (run test-alter_table.sh first): $(echo "$resp" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
    fi
}

do_test "PostgreSQL" "$PG_DSN" "$PG_DB"
do_test "MariaDB"    "$MARIADB_DSN" "$MARIADB_DB"

summary "COUNT_QUERY TEST"
