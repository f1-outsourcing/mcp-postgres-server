#!/bin/bash

# Show usage if no command provided
function show_usage() {
    echo "Usage: ./run.sh [command]"
    echo "Commands:"
    echo "  build    Build the MCP server binary (Postgres + MariaDB)"
    echo "  run      Run the MCP server (requires PG_DSN or MARIADB_DSN)"
    exit 1
}

# Handle different commands
case "$1" in
  build)
    echo "Building MCP server..."
    go build -o bin/postgres-server ./cmd
    ;;
  run)
    DSN="${PG_DSN:-${MARIADB_DSN:-}}"
    [ -z "$DSN" ] && { echo "Error: PG_DSN or MARIADB_DSN is required"; exit 1; }
    echo "Running MCP server (DSN: ${DSN%%@*}@...)"
    go run ./cmd --dsn "$DSN"
    ;;
  *)
    show_usage
    ;;
esac
