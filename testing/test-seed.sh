#!/bin/bash
# ==============================================================================
# test-seed.sh — Create detection fixtures (function, trigger, sequence)
#                via DB CLI (psql / mysql), NOT via the MCP server.
#
# Must run AFTER test-alter_table.sh (which creates the base table).
# The three list_* tests then assert these fixture names appear in the result.
#
# Fixtures (per dialect):
#   f_probe   — function
#   trg_probe — trigger on the base table
#   seq_probe — sequence
#
# Databases targeted: $PG_DB (PostgreSQL) and $MARIADB_DB (MariaDB)
#   (i.e. testpg / testm — same as the MCP database parameter)
#
# Usage: bash test-seed.sh
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

# ------------------------------------------------------------
# Parse credentials/host/port from the DSNs in common.sh
# ------------------------------------------------------------
pg_dsn()  { echo "$PG_DSN"; }
md_dsn()  { echo "$MARIADB_DSN"; }

pg_user="postgres"; pg_pass="test";  pg_host="127.0.0.1"; pg_port="5432"
md_user="root";     md_pass="test";  md_host="127.0.0.1"; md_port="3306"

# ============================================================
# PostgreSQL seed (via psql, into $PG_DB)
# ============================================================
section "seed [PostgreSQL] → db $PG_DB"
pg() { PGPASSWORD="$pg_pass" psql -h "$pg_host" -p "$pg_port" -U "$pg_user" -d "$PG_DB" -tA -v ON_ERROR_STOP=1 "$@" 2>&1; }

if command -v psql >/dev/null 2>&1; then
    # sanity: can we reach the DB?
    if ! pg -c "SELECT 1" >/dev/null 2>&1; then
        fail "seed [PG] → cannot connect to db $PG_DB on $pg_host:$pg_port"
    else
        # f_probe — function (idempotent: OR REPLACE)
        out=$(pg -c "CREATE OR REPLACE FUNCTION f_probe() RETURNS integer LANGUAGE sql AS 'SELECT 42';")
        [ $? -eq 0 ] && pass "seed [PG] → f_probe created" || fail "seed [PG] → f_probe: $out"

        # Trigger handler (must be RETURNS trigger, separate from f_probe)
        out=$(pg -c "CREATE OR REPLACE FUNCTION trg_probe_fn() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NEW; END';")
        [ $? -eq 0 ] || fail "seed [PG] → trg_probe_fn: $out"

        # trg_probe — trigger on base table (idempotent: DROP first)
        pg -c "DROP TRIGGER IF EXISTS trg_probe ON $PG_DB;" >/dev/null 2>&1
        out=$(pg -c "CREATE TRIGGER trg_probe BEFORE INSERT ON $PG_DB FOR EACH ROW EXECUTE FUNCTION trg_probe_fn();")
        [ $? -eq 0 ] && pass "seed [PG] → trg_probe created" || fail "seed [PG] → trg_probe: $out"

        # seq_probe — sequence (idempotent: IF NOT EXISTS)
        out=$(pg -c "CREATE SEQUENCE IF NOT EXISTS seq_probe;")
        [ $? -eq 0 ] && pass "seed [PG] → seq_probe created" || fail "seed [PG] → seq_probe: $out"
    fi
else
    fail "seed [PG] → psql not found in PATH"
fi

# ============================================================
# MariaDB seed (via mysql, into $MARIADB_DB)
# NOTE: mysql client puts the database as a positional arg, not -d
# ============================================================
section "seed [MariaDB] → db $MARIADB_DB"
md() { mysql -h "$md_host" -P "$md_port" -u "$md_user" -p"$md_pass" -e "$1" "$MARIADB_DB" 2>&1; }

if command -v mysql >/dev/null 2>&1; then
    # sanity: can we reach the DB?
    if ! md "SELECT 1" >/dev/null 2>&1; then
        fail "seed [MariaDB] → cannot connect to db $MARIADB_DB on $md_host:$md_port"
    else
        # f_probe — function (single-statement body, no BEGIN…END needed)
        out=$(md "DROP FUNCTION IF EXISTS f_probe;
                  CREATE FUNCTION f_probe() RETURNS INT DETERMINISTIC RETURN 42;")
        [ $? -eq 0 ] && pass "seed [MariaDB] → f_probe created" || fail "seed [MariaDB] → f_probe: $out"

        # trg_probe — trigger on base table (idempotent: DROP first)
        out=$(md "DROP TRIGGER IF EXISTS trg_probe;
                  CREATE TRIGGER trg_probe BEFORE INSERT ON $MARIADB_DB
                       FOR EACH ROW SET @seed_probe := 0;")
        [ $? -eq 0 ] && pass "seed [MariaDB] → trg_probe created" || fail "seed [MariaDB] → trg_probe: $out"

        # seq_probe — sequence (MariaDB 10.3+)
        out=$(md "CREATE SEQUENCE IF NOT EXISTS seq_probe;")
        [ $? -eq 0 ] && pass "seed [MariaDB] → seq_probe created" || fail "seed [MariaDB] → seq_probe: $out"
    fi
else
    fail "seed [MariaDB] → mysql client not found in PATH"
fi

summary "SEED TEST"
