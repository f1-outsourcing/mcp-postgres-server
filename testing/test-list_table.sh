#!/bin/bash
# Test script for list_table tool
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$PG_DSN" ]; then
    echo "⏭️  SKIPPED: PG_DSN not set"
    exit 0
fi

cd "$SCRIPT_DIR/../"
[ -f "bin/postgres-server" ] || go build -o bin/postgres-server ./cmd
cd "$SCRIPT_DIR"

echo "=== Testing list_table ==="

RESPONSE=$( ( echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"list_table","arguments":{}},"id":2}'; sleep 2 ) | "$SCRIPT_DIR/../bin/postgres-server" --dsn "$PG_DSN" 2>&1 )

echo "Raw response: $RESPONSE"

echo "$RESPONSE" | jq -e '.jsonrpc == "2.0"' > /dev/null 2>&1 && echo "✓ Valid JSON-RPC" || exit 1
echo "$RESPONSE" | jq -e '.id == 2' > /dev/null 2>&1 && echo "✓ ID preserved" || exit 1

if echo "$RESPONSE" | jq -e '.result.content[0].type == "text"' > /dev/null 2>&1; then
    echo "✓ Result content is text"
    CONTENT=$(echo "$RESPONSE" | jq -r '.result.content[0].text')
    echo "Content preview: ${CONTENT:0:200}"
else
    echo "✗ Missing result content"
    exit 1
fi

echo ""
echo "=== list_table test PASSED ==="
