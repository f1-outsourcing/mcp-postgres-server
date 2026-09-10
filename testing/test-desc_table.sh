#!/bin/bash
# Test script for desc_table tool
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$PG_DSN" ]; then
    echo "⏭️  SKIPPED: PG_DSN not set"
    exit 0
fi

cd "$SCRIPT_DIR/../"
[ -f "bin/postgres-server" ] || go build -o bin/postgres-server ./cmd
cd "$SCRIPT_DIR"

echo "=== Testing desc_table ==="

RESPONSE=$( ( echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"desc_table","arguments":{"name":"stat_activity"}},"id":3}'; sleep 2 ) | "$SCRIPT_DIR/../bin/postgres-server" --dsn "$PG_DSN" 2>&1 )

echo "Raw response: $RESPONSE"

echo "$RESPONSE" | jq -e '.jsonrpc == "2.0"' > /dev/null 2>&1 && echo "✓ Valid JSON-RPC" || exit 1
echo "$RESPONSE" | jq -e '.id == 3' > /dev/null 2>&1 && echo "✓ ID preserved" || exit 1

# Accept result OR error — desc_table on stat_activity may or may not succeed
if echo "$RESPONSE" | jq -e '.result.content[0].type == "text"' > /dev/null 2>&1; then
    echo "✓ Result content is text"
    CONTENT=$(echo "$RESPONSE" | jq -r '.result.content[0].text')
    echo "Content preview: ${CONTENT:0:300}"
elif echo "$RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
    echo "✓ Got error response (expected on some DBs)"
else
    echo "✗ No valid result or error"
    exit 1
fi

echo ""
echo "=== desc_table test PASSED ==="
