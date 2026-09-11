#!/bin/bash
# ==============================================================================
# run-all-tests.sh — Run all test-*.sh scripts in this directory
#
# Usage:
#   bash run-all-tests.sh
#
# Order matters: test-alter_table runs first so the table $DB exists for
# the other scripts. Each script's output is streamed to the terminal;
# the summary table greps [PASS]/[FAIL] from that output.
#
# DSNs live in common.sh. Edit that file to point at a different server.
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'

# Ordered list (alphabetical would put alter_table before all others, but
# pinning the order here makes the intent explicit).
TESTS=(
    test-list_table.sh
    test-alter_table.sh      # create the base table — must run before tests that reference it
    test-seed.sh             # create f_probe / trg_probe / seq_probe (via psql / mysql)
    test-desc_table.sh
    test-select_query.sh
    test-count_query.sh
    test-write_query.sh
    test-update_query.sh
    test-delete_query.sh
    test-list_functions.sh   # asserts f_probe is detected
    test-list_triggers.sh    # asserts trg_probe is detected
    test-list_sequences.sh   # asserts seq_probe is detected
    test-create_function.sh  # exercises create_function (own cf_probe fixture)
    test-create_trigger.sh   # exercises create_trigger (own ctg_probe fixture)
    test-teardown.sh         # drop the seed fixtures (safe to run even if seed failed)
)

# Drop any that are missing on disk
FILTER=()
for t in "${TESTS[@]}"; do
    [ -f "$SCRIPT_DIR/$t" ] && FILTER+=("$t")
done

if [ ${#FILTER[@]} -eq 0 ]; then
    echo -e "${RED}ERROR: no test scripts found in ${SCRIPT_DIR}${NC}"; exit 1
fi

echo "================================================="
echo "  MCP Server Test Suite"
echo "================================================="
echo -e "  Scripts: ${CYAN}${#FILTER[@]}${NC}"
echo -e "  Dialects: PostgreSQL + MariaDB (see common.sh)"
echo "================================================="
echo ""

RESULTS=()
TOTAL_PASS=0
TOTAL_FAIL=0

echo -e "${GREEN}==> Starting at $(date +%H:%M:%S)${NC}"
echo ""

for name in "${FILTER[@]}"; do
    echo -e "${CYAN}---------------------------------------------------${NC}"
    echo -e "${CYAN}>>> ${name}${NC}"
    echo -e "${CYAN}---------------------------------------------------${NC}"

    START=$(date +%s)
    OUTPUT=$(bash "$SCRIPT_DIR/$name" 2>&1)
    EXIT_CODE=$?
    END=$(date +%s)
    DUR=$((END - START))
    echo "$OUTPUT"

    P=$(echo "$OUTPUT" | grep -c '\[PASS\]' 2>/dev/null); P=${P:-0}
    F=$(echo "$OUTPUT" | grep -c '\[FAIL\]' 2>/dev/null); F=${F:-0}

    TOTAL_PASS=$((TOTAL_PASS + P))
    TOTAL_FAIL=$((TOTAL_FAIL + F))

    if [ $F -gt 0 ] || [ $EXIT_CODE -ne 0 ]; then
        RESULTS+=("$name|FAIL|${DUR}s|$P|$F")
    else
        RESULTS+=("$name|PASS|${DUR}s|$P|$F")
    fi
    echo ""
done

# ============================================================
# SUMMARY
# ============================================================
echo ""
echo "================================================="
echo "  Test Suite Summary"
echo "================================================="
printf "  %-40s %-8s %-8s %-7s %-8s\n" "Script" "Status" "Time" "Pass" "Fail"
printf "  %-40s %-8s %-8s %-7s %-8s\n" "----------------------------------------" "--------" "--------" "-------" "--------"

ALL_PASS=1
for line in "${RESULTS[@]}"; do
    IFS='|' read -r n s d p f <<< "$line"
    if [ "$s" = "PASS" ]; then
        printf "  %-40s ${GREEN}%-8s${NC} %-8s %-7s %-8s\n" "$n" "PASS" "$d" "$p" "$f"
    else
        printf "  %-40s ${RED}%-8s${NC} %-8s %-7s %-8s\n" "$n" "FAIL" "$d" "$p" "$f"
        ALL_PASS=0
    fi
done

echo ""
echo -e "  Total:  ${GREEN}${TOTAL_PASS} passed${NC}, ${RED}${TOTAL_FAIL} failed${NC}"
echo "================================================="

[ $ALL_PASS -eq 1 ] && exit 0 || exit 1
