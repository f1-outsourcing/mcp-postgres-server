#!/bin/bash
# Test script for insert_query tool (DDL smoke test)
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$PG_DSN" ]; then
    echo "⏭️  SKIPPED: PG_DSN not set"
    exit 0
fi

cd "$SCRIPT_DIR/../"
[ -f "bin/postgres-server" ] || go build -o bin/postgres-server ./cmd
cd "$SCRIPT_DIR"

echo "=== Testing insert_query ==="

# Use a safe DDL statement that won't fail on most databases
RESPONSE=$( ( echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"insert_query","arguments":{"query":"CREATE TABLE IF NOT EXISTS _mcp_smoke_test(id INT)"}},"id":6}'; sleep 2 ) | "$SCRIPT_DIR/../bin/postgres-server" --dsn "$PG_DSN" 2>&1 )

echo "Raw response: $RESPONSE"

echo "$RESPONSE" | jq -e '.jsonrpc == "2.0"' > /dev/null 2>&1 && echo "✓ Valid JSON-RPC" || exit 1
echo "$RESPONSE" | jq -e '.id == 6' > /dev/null 2>&1 && echo "✓ ID preserved" || exit 1

# Accept result OR error — MCP plumbing test
if echo "$RESPONSE" | jq -e '.result' > /dev/null 2>&1; then
    echo "✓ Got MCP result (write succeeded)"
elif echo "$RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
    echo "✓ Got MCP error (write failed, e.g. permission — expected smoke test)"
else
    echo "✗ No valid result or error"
    exit 1
fi

echo ""
echo "=== insert_query test PASSED ==="
