#!/bin/bash
# Test script for read_query tool
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$PG_DSN" ]; then
    echo "⏭️  SKIPPED: PG_DSN not set"
    exit 0
fi

cd "$SCRIPT_DIR/../"
[ -f "bin/postgres-server" ] || go build -o bin/postgres-server ./cmd
cd "$SCRIPT_DIR"

echo "=== Testing read_query ==="

RESPONSE=$( ( echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"read_query","arguments":{"query":"SELECT 1 AS one"}},"id":4}'; sleep 2 ) | "$SCRIPT_DIR/../bin/postgres-server" --dsn "$PG_DSN" 2>&1 )

echo "Raw response: $RESPONSE"

echo "$RESPONSE" | jq -e '.jsonrpc == "2.0"' > /dev/null 2>&1 && echo "✓ Valid JSON-RPC" || exit 1
echo "$RESPONSE" | jq -e '.id == 4' > /dev/null 2>&1 && echo "✓ ID preserved" || exit 1

if echo "$RESPONSE" | jq -e '.result.content[0].type == "text"' > /dev/null 2>&1; then
    echo "✓ Result content is text"
    CONTENT=$(echo "$RESPONSE" | jq -r '.result.content[0].text')
    echo "Content preview: ${CONTENT:0:200}"
    if echo "$CONTENT" | grep -q "one"; then
        echo "✓ Query result contains expected column 'one'"
    else
        echo "✗ Expected column 'one' not found in result"
        exit 1
    fi
else
    echo "✗ Missing result content"
    exit 1
fi

echo ""
echo "=== read_query test PASSED ==="
