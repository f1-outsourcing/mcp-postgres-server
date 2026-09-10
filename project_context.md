# Project Context: mcp-postgres-server

Generated 2026-09-10. Factual summary for developers and AI agents.

## Purpose

Single-binary **MCP (Model Context Protocol) server for PostgreSQL**. Exposes Postgres
access as 9 standard MCP tools over **stdio JSON-RPC**, so any MCP client (`mcp-gateway-go`,
Continue, VS Code extensions, ...) can query a Postgres database. No Node/Python runtime:
one Go binary (`bin/postgres-server`), module path `github.com/gomcpgo/postgres`, Go 1.23+.

## High-level Architecture

Layered, single process, no HTTP:

```
MCP client ──stdin/stdout──▶ gomcpgo/mcp server (protocol, JSON-RPC, registry)
                                  │  ListTools / CallTool
                                  ▼
                          PostgresHandler (pkg/handler)
                                  │  sqlx pool (lazy, shared)
                                  ▼
                              PostgreSQL
```

- `cmd/main.go` parses flags, sets up `slog` (stderr), builds `PostgresHandler`,
  registers it with `handler.NewHandlerRegistry()` and starts `server.New(...).Run()`.
- All behavior lives in `pkg/handler` (a single package named `handler` under `pkg`).

## Directory Structure (complete)

```
cmd/main.go               # Entry point: flags, version, logging, server start
pkg/handler/postgres.go   # PostgresHandler struct, ListTools, CallTool router
pkg/handler/tools.go      # buildTools(): tool registry + read-only filter
pkg/handler/params.go     # parseStringParam(), isSQLIdentifier()
pkg/handler/db.go         # Lazy DB pool, DoQuery/HandleQuery/HandleExec/HandleExplain/MapToCSV
pkg/handler/query_handlers.go   # list_tables, desc_table, select_query, count_query
pkg/handler/write_handlers.go   # create_table, alter_table, insert_query, update_query, delete_query
pkg/handler/helpers.go    # textResponse() MCP response wrapper
pkg/handler/handler_test.go     # Unit tests (stdlib-only, no DB)
testing/test-runner.sh    # Master integration runner
testing/test-*.sh         # 8 per-tool smoke tests (spawn binary, pipe JSON-RPC, validate with jq)
run.sh                    # build / run helper script
bin/                      # Build output (gitignored; .gitkeep tracked)
.github/workflows/go.yml          # CI: go build + go test
.github/workflows/docker-image.yml  # Docker push → guoling21cn/go-mcp-postgres (NOTE: no Dockerfile exists in repo)
go.mod / go.sum           # Dependencies
```

## Entry Points

- **`cmd/main.go`** — the only binary entry point.
  Flags: `--prefix`, `--dsn`, `--read-only`, `--with-explain-check`, `--log-level`
  (default `error`), `--version`. DSN falls back to `PG_DSN` env var; missing DSN ⇒ exit 1.
- **`run.sh`** — `./run.sh build` → `go build -o bin/postgres-server ./cmd`;
  `./run.sh run` → `go run` with `PG_DSN` (required).

## Major Components & Responsibilities

| Symbol / File | Responsibility |
|---|---|
| `PostgresHandler` (`postgres.go`) | Holds `prefix`, `dsn`, `readOnly`, `withExplainCheck`, `db *sqlx.DB`. Implements MCP `ListTools`/`CallTool`. `CallTool` strips an optional tool-name prefix (clients may or may not send prefixed names) then `switch`es to per-tool handlers; unknown tool ⇒ error. |
| `buildTools()` (`tools.go`) | Declares all 9 tools with JSON-Schema `InputSchema` (raw JSON strings via `json.RawMessage`). In `--read-only` mode returns only the 4 read tools. Tool descriptions embed behavioral guidance for LLM callers (e.g. "must have WHERE", "call desc_table first"). |
| `parseStringParam()` (`params.go`) | Extracts a required non-empty string arg; defensively accepts `fmt.Stringer`. |
| `isSQLIdentifier()` (`params.go`) | Guard for bare identifier embedding (letters/digits/underscore, no leading digit). Used only for `name` params (`desc_table`, `count_query`). |
| `DB()` (`db.go`) | **Lazy** connection: `sqlx.Connect("postgres", dsn)` on first use. `SetDB()` exists for tests. |
| `HandleQuery()`/`DoQuery()` (`db.go`) | Execute, return rows as `[]map[string]interface{}` (with `[]byte`→`string` conversion) + headers; `HandleQuery` formats via `MapToCSV`. |
| `HandleExec()` (`db.go`) | `db.Exec` for writes; returns "N rows affected" (+ `LastInsertId` for INSERTs). |
| `HandleExplain()` (`db.go`) | Optional `EXPLAIN` pre-check comparing `select_type` against expected statement type (see Gotchas). |
| Handler functions | Thin wrappers: parse param → optional identifier check → `DoQuery`/`HandleExec` with a `StatementType*` expected-type (`db.go` consts) → `textResponse()`; errors wrapped with tool-name prefix and `slog` logging for write failures. |
| `textResponse()` (`helpers.go`) | Wraps text in `protocol.CallToolResponse{Content: [{Type: "text", ...}]}`. |
| `testing/test-*.sh` | Per-tool smoke tests: pipe one JSON-RPC `tools/call` to the built binary, check response with `jq`; write tests use safe no-ops (`_mcp_smoke_test` table, `WHERE 1=0`). |

## Important Dependencies & Integrations

Runtime (`go.mod`):
- `github.com/gomcpgo/mcp v1.0.1` — MCP protocol, `handler.HandlerRegistry`, `server`, `protocol` types. The server lifecycle, JSON-RPC framing, stdio transport are all provided by this SDK — **not by this repo**.
- `github.com/jmoiron/sqlx v1.4.0` — DB pool/queries.
- `github.com/lib/pq v1.10.9` — Postgres driver (blank import in `db.go`).

External:
- A running PostgreSQL (DSN required; append `?sslmode=disable`/`prefer` if no TLS).
- MCP client (e.g. `mcp-gateway-go`) configured with the binary path + flags (see README JSON snippet).
- `jq` for the shell integration tests.
- GitHub Actions: Go CI build/test; Docker image push to DockerHub `guoling21cn/go-mcp-postgres` (tagged with `github.RUN_ID`; needs `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` secrets).

## Data Flow (typical tool call)

1. Client writes JSON-RPC `tools/call` to **stdin** (stdio transport from the mcp SDK).
2. SDK routes to `PostgresHandler.CallTool` (`postgres.go`); prefix trimmed if present.
3. Param parsed via `parseStringParam`; identifier-checked where a `table` name is embedded in SQL.
4. `handleX()` calls `DoQuery` (reads) or `HandleExec` (writes) from `db.go`, passing the expected statement type.
5. If `withExplainCheck` is on: `HandleExplain` runs `EXPLAIN <query>` first (currently MySQL-shaped — see Gotchas).
6. `DB()` opens the `sqlx` pool lazily from `dsn` on first request.
7. Result → `MapToCSV` (reads) or rows-affected string (writes) → `textResponse()` → response JSON on **stdout**.
8. All logging goes to **stderr** (`slog` text handler) — stdout is reserved for JSON-RPC.

## Key Domain Concepts

- **MCP tool**: name, description, JSON-Schema input; invoked via `tools/call`.
- **Read tools (always available)**: `list_tables`, `desc_table`, `select_query`, `count_query`.
- **Write tools (hidden by `--read-only`)**: `create_table`, `alter_table`, `insert_query` (INSERT), `update_query` (UPDATE), `delete_query` (DELETE).
- **Read-only mode**: filtered in `buildTools()` *and* re-checked inside every write handler (double gate).
- **Prefix**: optional tool-name prefix (`--prefix`); `CallTool` accepts both prefixed and bare names.
- **EXPLAIN pre-check** (`--with-explain-check`): validate query plan matches expected statement type before executing.
- **CSV output**: all SELECT results returned as CSV text.
- **Statement-type consts** (`db.go`): `StatementTypeNoExplainCheck`/`Select`/`Insert`/`Update`/`Delete`.

## Build / Test / Lint / Deploy Commands

```bash
# Build
./run.sh build                      # → bin/postgres-server
go build -o bin/postgres-server ./cmd

# Run (stdio server; requires a DSN)
export PG_DSN="postgresql://user:pass@host:5432/mydb?sslmode=disable"
./run.sh run
./bin/postgres-server --dsn "$PG_DSN" --read-only --log-level info

# Unit tests (no DB needed)
go test ./... -v -count=1

# Integration tests (need PG_DSN + a live Postgres + jq)
./testing/test-runner.sh            # skips (exit 0) if PG_DSN unset

# No explicit lint/format config in repo; CI runs `go mod tidy && go build ./... && go test ./...` (.github/workflows/go.yml)

# Docker (workflow only)
docker build . --file Dockerfile    # ⚠️ Dockerfile is NOT present in the current tree
```

## Configuration / Environment Variables

- `PG_DSN` — only env var; required (`--dsn` flag also works and wins if set).
- Behavior flags: `--prefix`, `--read-only`, `--with-explain-check`, `--log-level`
  (`debug|info|warn|error`, default `error`), `--version`.
- No config files, no env-based feature toggles beyond the above. CI secrets: `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`.

## Coding Conventions & Patterns

- Standard Go module layout: `cmd/` for the binary, `pkg/` for reusable code.
- Flat package `handler` (no sub-packages); files split by responsibility, all one package.
- `PostgresHandler` methods, exported entry API (`ListTools`/`CallTool`/`Set*`), unexported `handleX` per tool.
- Errors: wrapped with `%w`, prefixed with tool name (`"select_query: %w"`); handler returns `(nil, err)` on failure.
- Logging: `log/slog`, text handler on **stderr**, `slog.Error` for expected guard rejections.
- Raw JSON for tool input schemas (`json.RawMessage`), not Go structs.
- Tests: stdlib `testing` only, no DB (DB behavior covered by shell integration tests); small table/subtests (`TestParseStringParam`).
- Shell tests: `set -e`, JSON-RPC via stdin pipe + `sleep 2`, `jq -e` assertions; write tests written as safe no-ops.

## Inferred Architectural Decisions

- **Lazy DB connection** (`DB()` in `db.go`): the server must start and answer `ListTools` even before/without a reachable Postgres. `SetDSN` deliberately does not connect (documented in `postgres.go`).
- **Read-only enforcement is double-gated** (registry filter + per-handler checks) so a bypassed registry can't execute writes.
- **Prefix dual-acceptance** in `CallTool` (comment in `postgres.go`: some clients auto-add the prefix, some don't).
- **Identifier safety only for `table` param**; full-SQL `query` params are passed through verbatim by design — the LLM writes the SQL, so there is no query-level sanitization.
- **CSV as the canonical result format** for LLM-friendly, unambiguous parsing.
- **No transaction usage**, single shared `sqlx.DB` pool for the process lifetime.
- **stdio-only transport**; keep stdout clean of all non-protocol output (hence stderr logging).

## Easy-to-Misunderstand Areas / Gotchas

1. **`HandleExplain` is MySQL-shaped, not Postgres-shaped** (`db.go`): `ExplainResult` scans columns like `select_type`, `partitions`, `key_len` (MySQL `EXPLAIN` schema). Postgres `EXPLAIN` emits a different column set, and DDL statements can't be `EXPLAIN`ed at all. `--with-explain-check` is likely broken/erroring on real Postgres — do not extend it without fixing first.
2. **`desc_table` output is a synthetic pseudo-`CREATE TABLE` string** built by one big `information_schema` SQL (`query_handlers.go`), not real DDL: primary keys only (guessed via `constraint_name LIKE '%_pkey'`), no indexes/FKs/defaults; result read via the `?column?` key (Postgres name for the unnamed computed column).
3. **`select_query`/`insert_query` execute arbitrary SQL as given** — only the tool description constrains the LLM; the only enforcement is read-only mode for the write *tools*, not the statement type. "UPDATE must have WHERE" is description-only.
4. **Read-only ≠ schema-only**: `select_query` has no SELECT-only restriction (unless the broken EXPLAIN check is enabled).
5. **`.github/workflows/docker-image.yml` builds a `Dockerfile` that doesn't exist** in the current tree — that workflow is currently broken.
6. **`.gitignore` also ignores `.github/`**, `bin/`, `*.swp`, and generic leftovers (Composer, WordPress, `target/`) — the ignore file is boilerplate, not project-specific; `.github`/`bin/.gitkeep` are still tracked despite this.
7. **Stray files**: `.README.md.swp` vim swap (gitignored), `.continue` (19-byte IDE artifact).
8. **`count_query` CSV**: expects a header row from `MapToCSV`; `list_tables` returns plain newline-separated names (no header) — inconsistent format between read tools.
9. **`parseStringParam`'s `fmt.Stringer` branch** is defensive (args arrive as native strings after JSON decode) — don't rely on it for non-string JSON values (`42` ⇒ error).
10. **`version` is hardcoded** (`"1.0.0"`) in `cmd/main.go`; there is also a TODO about a hardcoded prefix related to `mcp-gateway-go` not sending args.
11. **Git history is mixed**: older commits are an upstream MySQL/Docker variant (`guoling21cn`); recent commits ("first working version on stdio", "removed list databases", "application alignment") pivoted it to a Postgres stdio server. Some legacy assumptions may linger.

## Notes for AI Coding Agents

- **Keep stdout protocol-clean**: anything you add must log to stderr via `slog`.
- When **adding a tool**, touch all of: `buildTools()` in `tools.go`, the `switch` in `CallTool` (`postgres.go`), a new `handleX` in `query_handlers.go`/`write_handlers.go` (add read-only guard to write tools), the tool table in `README.md`, and an entry in `handler_test.go` if it affects tool counts (the RO test hardcodes 4/9).
- **`go test ./...` must stay DB-free**; use `SetDB` only if you inject fakes.
- Do **not** change `textResponse` shape or CSV format without checking client-side consumers.
- `run.sh` and all `testing/test-*.sh` scripts assume binary at `bin/postgres-server` and repo root one level up from `testing/`.
- There is **no linter, no import ordering tool, no Makefile** — follow existing style (tabs, Go standard formatting).
- Dependencies should stay minimal (3 runtime deps); the value proposition is "single binary, no other runtime".
- If fixing `--with-explain-check`: real Postgres `EXPLAIN` uses a single `QUERY PLAN` text column (or `FORMAT JSON/YAML`), and cannot be applied to DDL — `select_query`/`count_query` pass statement types that the current code path can't meaningfully verify.


## Update log

### 2026-09-10 — added required `database` parameter per tool call

Every tool (`list_tables`, `desc_table`, `count_query`, `select_query`, `create_table`,
`alter_table`, `insert_query`, `update_query`, `delete_query`) now requires a
`database` string argument. `buildTools()` (`tools.go`) declares it in each
tool's JSON-Schema; every `handleX` in `query_handlers.go`/`write_handlers.go`
parses it via `parseStringParam(args, "database")` and threads it through
`HandleQuery`/`DoQuery`/`HandleExec`/`HandleExplain`/`DB` in `db.go`.

`replaceDatabaseName(dsn, dbName)` in `db.go` rewrites only the URL path
segment of the configured DSN, preserving query params like `sslmode`.
Connection pools are now cached per database in `PostgresHandler.dbPools`
(new field, `postgres.go`) so multiple databases can be used within a
single server session; `SetDB` still short-circuits per-db pooling for tests.

**Behavioral contract change for MCP clients**: every `tools/call` request
must include `"database": "<dbname>"` in `params.arguments`. Calls without
it now fail with `missing required parameter: database`.
The shell smoke tests in `testing/test-*.sh` have not been updated yet to
send the new arg — update them before running the integration suite.
`project_context.md` previously stated tools had no per-call database arg;
that guidance is superseded by this entry.


## 2026-09-10 — Robust stdio error handling

### What changed
**New file:** `cmd/stdiotransport.go` (line-based stdio transport, ~200 lines)
**Modified:** `cmd/main.go` (inject the new transport into `server.Options{Transport: ...}`)

### Why
The library's default `StdioTransport` wraps stdin in a **single shared** `json.Decoder` over a `bufio.Reader`. One malformed line (e.g. missing comma between JSON object fields) left the decoder in an error state, so all subsequent reads failed — the server silently stopped responding to any further valid requests.

### What the new transport does
- Reads one line at a time (`bufio.Reader.ReadString('\n')`)
- Each line is independently parsed with `json.Unmarshal` (no shared decode state)
- On parse failure: sends a proper JSON-RPC error response with code `-32700 Parse error` to stdout
- On invalid request shape: sends `-32600 Invalid Request` to stdout
- Empty lines are skipped
- Implements the same `transport.Transport` interface the library's `server.Transport` field expects
- `Stop()` is idempotent (uses `sync.Once`)

### Impact
- `--dsn` / `--read-only` / `--with-explain-check` / `--log-level` flags: **unchanged**
- All handler code: **unchanged**
- `tools/list`, `tools/call` behavior on valid input: **identical**
- Only difference: **bad JSON now produces an explicit JSON-RPC error on stdout**, and the server keeps running for the next line

### How to verify
```bash
# 1. Bad line — previously: silence; now: JSON-RPC error
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
  | bin/postgres-server --dsn 'postgresql://postgres:test@127.0.0.1/test?sslmode=disable'
# (works as before)

# 2. Multi-line session with bad line first
printf '%s\n%s\n' \
  '{"bad json","missing":' \
  '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
  | bin/postgres-server --dsn 'postgresql://postgres:test@127.0.0.1/test?sslmode=disable'
# Should now print:
#   {"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error: ..."}}
#   {"jsonrpc":"2.0","id":1,"result":{"tools":[...]}
```

### Build
```bash
go build -o bin/postgres-server ./cmd
```
