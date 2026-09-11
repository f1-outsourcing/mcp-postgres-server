#!/bin/bash
# ==============================================================================
# test-teardown.sh — Drop the detection fixtures created by test-seed.sh
#
# Idempotent — safe to run even if seed failed.
# Usage: bash test-teardown.sh
# ==============================================================================
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

pg_user="postgres"; pg_pass="test";  pg_host="127.0.0.1"; pg_port="5432"
md_user="root";     md_pass="test";  md_host="127.0.0.1"; md_port="3306"

# ---- PostgreSQL (into $PG_DB) ----
section "teardown [PostgreSQL] → db $PG_DB"
pg() { PGPASSWORD="$pg_pass" psql -h "$pg_host" -p "$pg_port" -U "$pg_user" -d "$PG_DB" -tA "$@" 2>&1; }
if command -v psql >/dev/null 2>&1; then
    out=$(pg -c "DROP TRIGGER IF EXISTS trg_probe ON $PG_DB;");  [ $? -eq 0 ] && pass "teardown [PG] → trg_probe dropped" || fail "teardown [PG] → trg_probe: $out"
    out=$(pg -c "DROP FUNCTION IF EXISTS trg_probe_fn;");        [ $? -eq 0 ] && pass "teardown [PG] → trg_probe_fn dropped" || fail "teardown [PG] → trg_probe_fn: $out"
    out=$(pg -c "DROP FUNCTION IF EXISTS f_probe;");             [ $? -eq 0 ] && pass "teardown [PG] → f_probe dropped"   || fail "teardown [PG] → f_probe: $out"
    out=$(pg -c "DROP SEQUENCE IF EXISTS seq_probe;");           [ $? -eq 0 ] && pass "teardown [PG] → seq_probe dropped" || fail "teardown [PG] → seq_probe: $out"
else
    info "teardown [PG] → psql not found, skipping"
fi

# ---- MariaDB (into $MARIADB_DB) ----
# NOTE: mysql client puts the database as a positional arg, not -d
section "teardown [MariaDB] → db $MARIADB_DB"
md() { mysql -h "$md_host" -P "$md_port" -u "$md_user" -p"$md_pass" -e "$1" "$MARIADB_DB" 2>&1; }
if command -v mysql >/dev/null 2>&1; then
    out=$(md "DROP TRIGGER IF EXISTS trg_probe;");              [ $? -eq 0 ] && pass "teardown [MD] → trg_probe dropped" || fail "teardown [MD] → trg_probe: $out"
    out=$(md "DROP FUNCTION IF EXISTS f_probe;");               [ $? -eq 0 ] && pass "teardown [MD] → f_probe dropped"   || fail "teardown [MD] → f_probe: $out"
    out=$(md "DROP SEQUENCE IF EXISTS seq_probe;");             [ $? -eq 0 ] && pass "teardown [MD] → seq_probe dropped" || fail "teardown [MD] → seq_probe: $out"
else
    info "teardown [MD] → mysql not found, skipping"
fi

summary "TEARDOWN TEST"
