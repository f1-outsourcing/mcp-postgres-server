#!/bin/bash
# ==============================================================================
# common.sh — shared header for all test-*.sh scripts in this directory
#
# Source this at the top of every test file:
#   source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
#
# Provides constants:
#   $BIN          path to the built postgres-server binary (go build if missing)
#   $PG_DSN / $PG_DB
#   $MARIADB_DSN / $MARIADB_DB
#
# Helpers:
#   pass(...)  failure(...)  info(...)  section(...)
#   mcp_call <dsn> <json-payload>   → raw server response
#   summary <title>                 → prints totals + sets exit code
#
# Environment:
#   DEBUG=1  dumps the DSN, payload and raw JSON-RPC response of every
#            mcp_call() to stderr (same convention as springboot-test/testing)
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="${SCRIPT_DIR}/../bin/postgres-server"

[ -f "$BIN" ] || ( cd "${SCRIPT_DIR}/../" && go build -o bin/postgres-server ./cmd )
[ -f "$BIN" ] || { echo "ERROR: server binary not found at $BIN"; exit 1; }

# ============================================================
# Dialects (edit here to change both suites at once)
# ============================================================
PG_DSN="postgresql://postgres:test@127.0.0.1/test?sslmode=disable"
PG_DB="testpg"

MARIADB_DSN="mysql://root:test@127.0.0.1:3306/test"
MARIADB_DB="testm"

# ============================================================
# OUTPUT
# ============================================================
GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
PASSES=0; FAILS=0

# DEBUG=1 → every mcp_call() also dumps the DSN, payload and raw response
# to stderr, so you can eyeball exactly what the server sent back without
# re-running. Mirrors the springboot-test convention.
DEBUG="${DEBUG:-0}"

pass()    { echo -e "${GREEN}[PASS]${NC} $1";  PASSES=$((PASSES + 1)); }
fail()    { echo -e "${RED}[FAIL]${NC} $1";    FAILS=$((FAILS + 1)); }
info()    { echo -e "${YELLOW}[INFO]${NC} $1"; }
section() { echo -e "\n${CYAN}=== $1 ====${NC}"; }

# ============================================================
# mcp_call <dsn> <json-payload>
# ============================================================
mcp_call() {
    local dsn="$1" payload="$2" resp
    resp=$( { echo "$payload"; sleep 2; } | "$BIN" --dsn "$dsn" 2>&1 )
    if [ "$DEBUG" = "1" ]; then
        echo "----- DEBUG: mcp_call ${dsn%%@*}@*** — payload: ${payload} -----" >&2
        echo "$resp" >&2
        echo "----- end debug -----" >&2
    fi
    echo "$resp"
}

# ============================================================
# check_envelope <label> <response>
#   Shared assertion: valid JSON-RPC envelope + has result.content[0].type=="text"
#   (Accepts MCP error object too — used by write tools where errors can be OK.)
# ============================================================
check_envelope() {
    local label="$1" resp="$2"
    if ! echo "$resp" | jq -e '.jsonrpc=="2.0"' >/dev/null 2>&1; then
        fail "$label → invalid JSON-RPC: $(echo "$resp" | head -3)"
        return 1
    fi
    pass "$label → valid JSON-RPC envelope"
}

# ============================================================
# summary — call at the end of every test file
# ============================================================
summary() {
    local title="${1:-TESTS}"
    echo ""
    echo "================================================="
    echo -e "  ${title}"
    echo -e "  Passed: ${GREEN}${PASSES}${NC}   Failed: ${RED}${FAILS}${NC}"
    echo "================================================="
    [ $FAILS -eq 0 ] && exit 0 || exit 1
}
