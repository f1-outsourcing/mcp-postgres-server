#!/bin/bash
# ==============================================================================
# test-desc_table.sh — desc_table tool (both dialects)
# Run AFTER test-alter_table.sh so the table exists.
#
# Usage: bash test-desc_table.sh
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

do_test() {
    local label="$1" dsn="$2" db="$3"
    section "desc_table on $db [$label]"
    local payload resp
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"desc_table","arguments":{"database":"%s","table":"%s"}},"id":3}' "$db" "$db")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "desc_table [$label]" "$resp"; then
        return
    fi
    if echo "$resp" | jq -e '.result.content[0].type=="text"' >/dev/null 2>&1; then
        local out; out=$(echo "$resp" | jq -r '.result.content[0].text')
        if echo "$out" | grep -qi 'id\|name\|column\|type'; then
            pass "desc_table → schema columns visible"
        else
            fail "desc_table → unexpected output: $(echo "$out" | head -2)"
        fi
    else
        fail "desc_table → no result (run test-alter_table.sh first): $(echo "$resp" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
    fi
}

do_test "PostgreSQL" "$PG_DSN" "$PG_DB"
do_test "MariaDB"    "$MARIADB_DSN" "$MARIADB_DB"

summary "DESC_TABLE TEST"
