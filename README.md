# Postgres MCP Server

A single-binary MCP server for PostgreSQL, powered by the `gomcpgo/mcp` SDK.
No Node.js, Python, or other runtime required — download, configure the DSN, connect to any MCP client.

## Features

- **9 tools** — full DDL + DML: list tables, describe, query, count, create, alter, insert, update, delete
- **Read-only mode** — hide all write tools with a single flag
- **Optional EXPLAIN** — opt in to the query plan per call (read tools: with results; write tools: preview without executing)
- **CSV output** — all query results formatted as clean CSV
- **stdio transport** — reads JSON-RPC from stdin, writes to stdout (compatible with `mcp-gateway-go`)

## Build

```bash
./run.sh build
# or
go build -o bin/postgres-server ./cmd
```

Requires Go 1.23+.

## Run

```bash
# Using run.sh (requires PG_DSN)
export PG_DSN="postgresql://user:pass@host:5432/mydb?sslmode=disable"
./run.sh run

# Binary directly
./bin/postgres-server \
  --dsn "postgresql://user:pass@host:5432/mydb?sslmode=disable" \
  --read-only \
  --log-level error
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prefix` | *(empty)* | Tool name prefix (e.g. `pg_select_query`) |
| `--dsn` | — | **Required.** Postgres DSN (`postgresql://user:pass@host:port/db`) |
| `--read-only` | `false` | Disable write tools |
| `--log-level` | `error` | `debug \| info \| warn \| error` |
| `--version` | | Print version and exit |

> **Note:** If your Postgres doesn't have SSL enabled, append `?sslmode=disable` or `?sslmode=prefer` to the DSN.

## Tools

### Read-only (always available)

- **`list_tables`** — List all tables in the `public` schema (one name per line)
- **`desc_table`** — Describe table structure as raw `CREATE TABLE` SQL. Param: `table`
- **`select_query`** — Execute a SELECT query. Param: `query`; optional `explain` (bool, default `false`) returns the `EXPLAIN ANALYZE` plan with the results
- **`count_query`** — Get row count for a table. Param: `table`; optional `explain` (bool, default `false`)

### Write (hidden when `--read-only = true`)

- **`create_table`** — Execute DDL to create a table. Param: `query`
- **`alter_table`** — Execute DDL to alter a table. Param: `query`
- **`insert_query`** — Execute an INSERT statement. Param: `query`; optional `explain` (bool, default `false`) returns the `EXPLAIN` plan **instead of executing**
- **`update_query`** — Execute an UPDATE statement (must have WHERE). Param: `query`; optional `explain` (bool, default `false`)
- **`delete_query`** — Execute a DELETE statement (must have WHERE). Param: `query`; optional `explain` (bool, default `false`)

## Usage with `mcp-gateway-go`

```json
{
  "mcpServers": {
    "postgres": {
      "command": "/path/to/bin/postgres-server",
      "args": [
        "--dsn", "postgresql://user:pass@host:5432/mydb?sslmode=disable",
        "--read-only"
      ]
    }
  }
}
```

## Testing

### Unit tests (no DB required)

```bash
go test ./... -v -count=1
```

### Integration tests (requires a running Postgres)

```bash
export PG_DSN="postgresql://user:pass@host:5432/mydb?sslmode=disable"
./testing/test-runner.sh
```

Each integration test spawns the server, sends a JSON-RPC `tools/call`, and validates the MCP response structure with `jq`.

## Project Structure

```
├── cmd/
│   └── main.go          # Entry point: flags, logging, server start
├── pkg/handler/
│   ├── postgres.go      # Handler struct + CallTool router
│   ├── tools.go         # Tool registry (9 tools, read-only filter)
│   ├── params.go        # parseStringParam + isSQLIdentifier helpers
│   ├── db.go            # DB pool, DoQuery, HandleExec, ExplainPlan, MapToCSV
│   ├── query_handlers.go# 4 read handlers
│   ├── write_handlers.go# 5 write handlers
│   ├── helpers.go       # textResponse wrapper
│   └── handler_test.go  # Unit tests (stdlib only, no DB)
├── testing/
│   ├── test-runner.sh   # Master test runner
│   └── test-*.sh        # Per-tool integration tests (require PG_DSN)
├── bin/                 # Built binary (gitignored, .gitkeep tracked)
├── run.sh               # build / run helper
└── go.mod
```

## License

MIT License
