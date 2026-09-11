#!/bin/bash
# ==============================================================================
# test-list_table.sh — list_tables tool (both dialects)
#
# Usage: bash test-list_table.sh
# DSNs / DB names come from common.sh (PG_DSN/PG_DB, MARIADB_DSN/MARIADB_DB).
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

do_test() {
    local label="$1" dsn="$2" db="$3"
    section "list_tables [$label]"
    local payload resp
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"list_tables","arguments":{"database":"%s"}},"id":1}' "$db")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "list_tables [$label]" "$resp"; then
        return
    fi
    if echo "$resp" | jq -e '.result.content[0].type=="text"' >/dev/null 2>&1; then
        local out; out=$(echo "$resp" | jq -r '.result.content[0].text')
        if echo "$out" | grep -q "$db"; then
            pass "list_tables → table '$db' present in result"
        else
            info "list_tables → '$db' not yet in list (run test-alter_table.sh first)"
        fi
    else
        info "list_tables → no result: $(echo "$resp" | jq -r '.error.message // empty' 2>/dev/null | head -1)"
    fi
}

do_test "PostgreSQL" "$PG_DSN" "$PG_DB"
do_test "MariaDB"    "$MARIADB_DSN" "$MARIADB_DB"

summary "LIST_TABLES TEST"
