#!/bin/bash
# Test script for update_query tool (no-op smoke test)
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$PG_DSN" ]; then
    echo "⏭️  SKIPPED: PG_DSN not set"
    exit 0
fi

cd "$SCRIPT_DIR/../"
[ -f "bin/postgres-server" ] || go build -o bin/postgres-server ./cmd
cd "$SCRIPT_DIR"

echo "=== Testing update_query ==="

# Use a harmless UPDATE with 0-row WHERE clause
RESPONSE=$( ( echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"update_query","arguments":{"query":"UPDATE _mcp_smoke_test SET id = 1 WHERE 1=0"}},"id":8}'; sleep 2 ) | "$SCRIPT_DIR/../bin/postgres-server" --dsn "$PG_DSN" 2>&1 )

echo "Raw response: $RESPONSE"

echo "$RESPONSE" | jq -e '.jsonrpc == "2.0"' > /dev/null 2>&1 && echo "✓ Valid JSON-RPC" || exit 1
echo "$RESPONSE" | jq -e '.id == 8' > /dev/null 2>&1 && echo "✓ ID preserved" || exit 1

# Accept result OR error
if echo "$RESPONSE" | jq -e '.result' > /dev/null 2>&1; then
    echo "✓ Got MCP result (update executed)"
elif echo "$RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
    echo "✓ Got MCP error (table not yet created — expected smoke test)"
else
    echo "✗ No valid result or error"
    exit 1
fi

echo ""
echo "=== update_query test PASSED ==="
