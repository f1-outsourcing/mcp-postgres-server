#!/bin/bash
# ==============================================================================
# test-list_sequences.sh — list_sequences tool (both dialects)
#
# Requires: test-seed.sh already ran (creates seq_probe).
#
# Usage: bash test-list_sequences.sh
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

do_test() {
    local label="$1" dsn="$2" db="$3"
    section "list_sequences [$label]"
    local payload resp text
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"list_sequences","arguments":{"database":"%s"}},"id":11}' "$db")
    resp=$(mcp_call "$dsn" "$payload")

    if ! check_envelope "list_sequences [$label]" "$resp"; then
        return
    fi
    text=$(echo "$resp" | jq -r '.result.content[0].text // empty' 2>/dev/null)
    if echo "$text" | grep -q 'seq_probe'; then
        pass "list_sequences [$label] → seeded seq_probe detected"
    else
        fail "list_sequences [$label] → seq_probe NOT found (seed missing?)"
        info "RAW JSON-RPC response:"; echo "$resp" | head -c 2000 | sed 's/^/    /'; echo
    fi
    info "$label → $(echo "$text" | grep -c .) sequence line(s) returned"
}

do_test "PostgreSQL" "$PG_DSN" "$PG_DB"
do_test "MariaDB"    "$MARIADB_DSN" "$MARIADB_DB"

summary "LIST_SEQUENCES TEST"
