#!/bin/bash
# ==============================================================================
# test-create_trigger.sh — create_trigger tool (both dialects)
#
# Exercises the MCP `create_trigger` tool. Each create_trigger call is a
# SINGLE CREATE TRIGGER statement (the supported path — no multi-statement).
#
# The handler must already exist, so the documented flow is validated on PG:
#   create_function (handler)  →  create_trigger (EXECUTE FUNCTION <handler>)
# MariaDB triggers are inline (SET body), so no separate handler is needed.
#
# Idempotency: any prior trigger/function of the same names is removed first
# via a guarded DB-CLI DROP (mirrors test-seed.sh), so the suite re-runs clean.
# The base table (named $db) the trigger attaches to is created by
# test-alter_table.sh, which the runner always executes before these tests.
#
# Usage: bash test-create_trigger.sh
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

TRG="ctg_probe"
PFN="ctg_probe_fn"   # PostgreSQL trigger handler

cli_pg() {
    local u="postgres" p="test" h="127.0.0.1" port="5432"
    PGPASSWORD="$p" psql -h "$h" -p "$port" -U "$u" -d "$1" -tA "$2" 2>&1
}
cli_md() {
    local u="root" p="test" h="127.0.0.1" port="3306"
    mysql -h "$h" -P "$port" -u "$u" -p"$p" -e "$2" "$1" 2>&1
}

mcp_call_tool() {
    local name="$1" dsn="$2" db="$3" ddl="$4" jddl payload
    jddl=$(printf '%s' "$ddl" | sed 's/\\/\\\\/g; s/"/\\"/g')
    payload=$(printf '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"%s","arguments":{"database":"%s","query":"%s"}},"id":41}' "$name" "$db" "$jddl")
    mcp_call "$dsn" "$payload"
}

# ============================================================
# PostgreSQL
# ============================================================
section "create_trigger [PostgreSQL] → db $PG_DB"
if command -v psql >/dev/null 2>&1; then
    cli_pg "$PG_DB" "DROP TRIGGER IF EXISTS ${TRG} ON ${PG_DB};" >/dev/null 2>&1
    cli_pg "$PG_DB" "DROP FUNCTION IF EXISTS ${PFN};" >/dev/null 2>&1
fi

# 1) handler function via create_function
hfn="CREATE OR REPLACE FUNCTION ${PFN}() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NEW; END';"
resp_h=$(mcp_call_tool create_function "$PG_DSN" "$PG_DB" "$hfn")
if check_envelope "create_function(handler) [PG]" "$resp_h" && echo "$resp_h" | jq -e '.result' >/dev/null 2>&1; then
    pass "create_trigger [PG] → handler ${PFN} created"
else
    fail "create_trigger [PG] → handler creation failed: $(echo "$resp_h" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
fi

# 2) the trigger via create_trigger (single statement)
trg="CREATE TRIGGER ${TRG} BEFORE INSERT ON ${PG_DB} FOR EACH ROW EXECUTE FUNCTION ${PFN}();"
resp_t=$(mcp_call_tool create_trigger "$PG_DSN" "$PG_DB" "$trg")
if check_envelope "create_trigger [PG]" "$resp_t" && echo "$resp_t" | jq -e '.result' >/dev/null 2>&1; then
    pass "create_trigger [PG] → ${TRG} created on ${PG_DB}"
else
    fail "create_trigger [PG] → failed: $(echo "$resp_t" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
fi

# ============================================================
# MariaDB (inline body, no handler)
# ============================================================
section "create_trigger [MariaDB] → db $MARIADB_DB"
if command -v mysql >/dev/null 2>&1; then
    cli_md "$MARIADB_DB" "DROP TRIGGER IF EXISTS ${TRG};" >/dev/null 2>&1
fi
trg_md="CREATE TRIGGER ${TRG} BEFORE INSERT ON ${MARIADB_DB} FOR EACH ROW SET @ctg_probe := 1;"
resp_md=$(mcp_call_tool create_trigger "$MARIADB_DSN" "$MARIADB_DB" "$trg_md")
if check_envelope "create_trigger [MD]" "$resp_md" && echo "$resp_md" | jq -e '.result' >/dev/null 2>&1; then
    pass "create_trigger [MD] → ${TRG} created on ${MARIADB_DB}"
else
    fail "create_trigger [MD] → failed: $(echo "$resp_md" | jq -r '.error.message // .error' 2>/dev/null | head -1)"
fi

# ============================================================
# best-effort cleanup (CLI, guarded)
# ============================================================
if command -v psql >/dev/null 2>&1; then
    cli_pg "$PG_DB" "DROP TRIGGER IF EXISTS ${TRG} ON ${PG_DB};" >/dev/null 2>&1
    cli_pg "$PG_DB" "DROP FUNCTION IF EXISTS ${PFN};" >/dev/null 2>&1
    info "cleanup [PG] → ${TRG}/${PFN} dropped"
fi
if command -v mysql >/dev/null 2>&1; then
    cli_md "$MARIADB_DB" "DROP TRIGGER IF EXISTS ${TRG};" >/dev/null 2>&1
    info "cleanup [MD] → ${TRG} dropped"
fi

summary "CREATE_TRIGGER TEST"
