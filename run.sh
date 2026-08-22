#!/bin/bash

# Show usage if no command provided
function show_usage() {
    echo "Usage: ./run.sh [command]"
    echo "Commands:"
    echo "  build    Build the Postgres MCP server binary"
    echo "  run      Run the Postgres MCP server (requires PG_DSN)"
    exit 1
}

# Handle different commands
case "$1" in
  build)
    echo "Building Postgres MCP server..."
    go build -o bin/postgres-server ./cmd
    ;;
  run)
    echo "Running Postgres MCP server..."
    [ -z "$PG_DSN" ] && { echo "Error: PG_DSN is required"; exit 1; }
    go run ./cmd/main.go --dsn "$PG_DSN"
    ;;
  *)
    show_usage
    ;;
esac
