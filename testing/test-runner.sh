#!/bin/bash
# Master test runner for all postgres MCP server tools
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "============================================"
echo "  Postgres MCP Server - Test Suite"
echo "============================================"
echo ""

echo "  Requires PG_DSN to be set."
echo "  Example: export PG_DSN=postgresql://user:pass@localhost:5432/mydb"
echo ""

[ -z "$PG_DSN" ] && { echo "⏭️  SKIPPED: PG_DSN not set"; exit 0; }

# Build server once if needed
cd "$SCRIPT_DIR/../"
[ -f "bin/postgres-server" ] || { echo "Building server..."; go build -o bin/postgres-server ./cmd; }
cd "$SCRIPT_DIR"

PASSED=0
FAILED=0
SKIPPED=0
TOTAL=0

for test_script in test-*.sh; do
    if [ "$test_script" = "test-runner.sh" ]; then
        continue
    fi

    TOTAL=$((TOTAL + 1))
    echo "--------------------------------------------"
    echo "Running: $test_script"
    echo "--------------------------------------------"

    if bash "$test_script" 2>&1; then
        PASSED=$((PASSED + 1))
        echo "✅ PASSED: $test_script"
    else
        exit_code=$?
        FAILED=$((FAILED + 1))
        echo "❌ FAILED: $test_script (exit code: $exit_code)"
    fi
    echo ""
done

echo "============================================"
echo "  Test Results Summary"
echo "============================================"
echo "Total tests:  $TOTAL"
echo "Passed:       $PASSED"
echo "Failed:       $FAILED"
echo "============================================"

if [ "$FAILED" -gt 0 ]; then
    echo "❌ Some tests failed!"
    exit 1
else
    echo "✅ All tests passed!"
    exit 0
fi
