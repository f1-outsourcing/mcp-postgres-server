# Postgres MCP Server

A single-binary MCP server for PostgreSQL, powered by the `gomcpgo/mcp` SDK.
No Node.js, Python, or other runtime required — download, configure the DSN, connect to any MCP client.

## Features

- **10 tools** — full DDL + DML: query, count, describe, create, alter, insert, update, delete
- **Read-only mode** — hide all write tools with a single flag
- **EXPLAIN pre-check** — validate query plan before executing (optional)
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
export PG_DSN="postgresql://user:pass@host:5432/mydb"
./run.sh run

# Binary directly
./bin/postgres-server \
  --dsn "postgresql://user:pass@host:5432/mydb" \
  --read-only \
  --with-explain-check \
  --log-level error
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prefix` | `pg_` | Tool name prefix (e.g. `pg_read_query`) |
| `--dsn` | — | **Required.** Postgres DSN (`postgresql://user:pass@host:port/db`) |
| `--read-only` | `false` | Disable write tools |
| `--with-explain-check` | `false` | Run `EXPLAIN` before executing (validates plan) |
| `--log-level` | `error` | `debug \| info \| warn \| error` |
| `--version` | | Print version and exit |

## Tools

### Read-only (always available)

- **`pg_list_database`** — List all non-template databases
- **`pg_list_table`** — List all tables (schema + name)
- **`pg_desc_table`** — Describe table structure as `CREATE TABLE` SQL. Param: `name`
- **`pg_read_query`** — Execute a SELECT query. Param: `query`
- **`pg_count_query`** — Get row count for a table. Param: `name`

### Write (hidden when `--read-only = true`)

- **`pg_create_table`** — Execute DDL to create a table. Param: `query`
- **`pg_alter_table`** — Execute DDL to alter a table. Param: `query`
- **`pg_write_query`** — Execute an INSERT statement. Param: `query`
- **`pg_update_query`** — Execute an UPDATE statement (must have WHERE). Param: `query`
- **`pg_delete_query`** — Execute a DELETE statement (must have WHERE). Param: `query`

## Usage with `mcp-gateway-go`

```json
{
  "mcpServers": {
    "postgres": {
      "command": "/path/to/bin/postgres-server",
      "args": [
        "--dsn", "postgresql://user:pass@host:5432/mydb",
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
export PG_DSN="postgresql://user:pass@host:5432/mydb"
./testing/test-runner.sh
```

Each integration test spawns the server, sends a JSON-RPC `tools/call`, and validates the MCP response structure with `jq`.

## Project Structure

```
├── cmd/
│   └── main.go          # Entry point: flags, logging, server start
├── pkg/handler/
│   ├── postgres.go      # Handler struct + CallTool router
│   ├── tools.go         # Tool registry (10 tools, read-only filter)
│   ├── params.go        # parseStringParam helper
│   ├── db.go            # DB pool, DoQuery, HandleExec, HandleExplain, MapToCSV
│   ├── query_handlers.go# 5 read handlers
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
