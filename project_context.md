# Project Context: mcp-postgres-server

Generated 2026-09-10. Factual summary for developers and AI agents.

## Purpose

Single-binary **MCP (Model Context Protocol) server for PostgreSQL and MariaDB**. Exposes
database access as 14 standard MCP tools over **stdio JSON-RPC**, so any MCP client
(`mcp-gateway-go`, Continue, VS Code extensions, ...) can query a Postgres or MariaDB database.
The dialect is auto-detected from the DSN URL scheme (`postgresql://` → Postgres, `mysql://` or
`mariadb://` → MariaDB). No Node/Python runtime: one Go binary (`bin/postgres-server`),
module path `github.com/gomcpgo/postgres`, Go 1.23+.

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
pkg/handler/params.go     # parseStringParam(), parseBoolParam(), isSQLIdentifier()
pkg/handler/db.go         # Lazy DB pool, DoQuery/HandleQuery/HandleExec/ExplainPlan/MapToCSV
pkg/handler/query_handlers.go   # list_tables, desc_table, select_query, count_query
pkg/handler/inspect_handlers.go # list_functions, desc_function, list_triggers, desc_trigger, list_sequences
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
  Flags: `--prefix`, `--dsn`, `--read-only`, `--log-level`
  (default `error`), `--version`. DSN falls back to `PG_DSN` env var; missing DSN ⇒ exit 1.
- **`run.sh`** — `./run.sh build` → `go build -o bin/postgres-server ./cmd`;
  `./run.sh run` → `go run` with `PG_DSN` (required).

## Major Components & Responsibilities

| Symbol / File | Responsibility |
|---|---|
| `PostgresHandler` (`postgres.go`) | Holds `prefix`, `dsn`, `readOnly`, `db *sqlx.DB`, `dbPools`. Implements MCP `ListTools`/`CallTool`. `CallTool` strips an optional tool-name prefix (clients may or may not send prefixed names) then `switch`es to per-tool handlers; unknown tool ⇒ error. |
| `buildTools()` (`tools.go`) | Declares all 14 tools with JSON-Schema `InputSchema` (raw JSON strings via `json.RawMessage`). In `--read-only` mode returns only the 9 read/introspection tools. Tool descriptions embed behavioral guidance for LLM callers (e.g. "must have WHERE", "call desc_table first"). |
| `parseStringParam()` (`params.go`) | Extracts a required non-empty string arg; defensively accepts `fmt.Stringer`. |
| `parseBoolParam()` (`params.go`) | Extracts an optional boolean arg (unset/absent ⇒ false); used for the `explain` flag. |
| `isSQLIdentifier()` (`params.go`) | Guard for bare identifier embedding (letters/digits/underscore, no leading digit). Used for `table`/`function`/`trigger`/`schema` params before embedding in generated SQL. |
| `DB()` (`db.go`) | **Lazy** connection: `sqlx.Connect("postgres", dsn)` on first use. `SetDB()` exists for tests. |
| `HandleQuery()`/`DoQuery()` (`db.go`) | Execute, return rows as `[]map[string]interface{}` (with `[]byte`→`string` conversion) + headers; `HandleQuery` formats via `MapToCSV`. |
| `HandleExec()` (`db.go`) | `db.Exec` for writes; returns "N rows affected". |
| `ExplainPlan()` (`db.go`) | Runs `EXPLAIN` (or `EXPLAIN (ANALYZE, BUFFERS)` when `analyze=true`) and returns the plan as text. |
| Handler functions | Thin wrappers: parse param → optional identifier check → `DoQuery`/`HandleExec` → `textResponse()`; optional `explain` flag returns the `EXPLAIN` plan (read tools: alongside results; DML write tools: **instead of executing**). Errors wrapped with tool-name prefix and `slog` logging for write failures. |
| `textResponse()` (`helpers.go`) | Wraps text in `protocol.CallToolResponse{Content: [{Type: "text", ...}]}`. |
| `testing/test-*.sh` | Per-tool smoke tests: pipe one JSON-RPC `tools/call` to the built binary, check response with `jq`; write tests use safe no-ops (`_mcp_smoke_test` table, `WHERE 1=0`). |

## Important Dependencies & Integrations

Runtime (`go.mod`):
- `github.com/gomcpgo/mcp v1.0.1` — MCP protocol, `handler.HandlerRegistry`, `server`, `protocol` types. The server lifecycle, JSON-RPC framing, stdio transport are all provided by this SDK — **not by this repo**.
- `github.com/jmoiron/sqlx v1.4.0` — DB pool/queries.
- `github.com/lib/pq v1.10.9` — Postgres driver (blank import in `db.go`).
- `github.com/go-sql-driver/mysql v1.8.1` — MySQL/MariaDB driver (blank import in `db.go`).

External:
- A running PostgreSQL (DSN required; append `?sslmode=disable`/`prefer` if no TLS).
- MCP client (e.g. `mcp-gateway-go`) configured with the binary path + flags (see README JSON snippet).
- `jq` for the shell integration tests.
- GitHub Actions: Go CI build/test; Docker image push to DockerHub `guoling21cn/go-mcp-postgres` (tagged with `github.RUN_ID`; needs `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` secrets).

## Data Flow (typical tool call)

1. Client writes JSON-RPC `tools/call` to **stdin** (stdio transport from the mcp SDK).
2. SDK routes to `PostgresHandler.CallTool` (`postgres.go`); prefix trimmed if present.
3. Param parsed via `parseStringParam`; identifier-checked where a `table` name is embedded in SQL.
4. `handleX()` calls `DoQuery` (reads) or `HandleExec` (writes) from `db.go`; if the optional `explain` flag is set it also calls `ExplainPlan` (read tools: alongside results; DML writes: instead of executing).
5. `DB()` opens the `sqlx` pool lazily from `dsn` on first request.
7. Result → `MapToCSV` (reads) or rows-affected string (writes) → `textResponse()` → response JSON on **stdout**.
8. All logging goes to **stderr** (`slog` text handler) — stdout is reserved for JSON-RPC.

## Key Domain Concepts

- **MCP tool**: name, description, JSON-Schema input; invoked via `tools/call`.
- **Read tools (always available)**: `list_tables`, `desc_table`, `select_query`, `count_query`.
- **Introspection tools (always available, for cross-DB diffing)**: `list_functions`, `desc_function`, `list_triggers`, `desc_trigger`, `list_sequences`. Use Postgres-native catalog functions (`pg_get_functiondef`, `pg_get_function_arguments`, `pg_get_triggerdef`, `pg_sequence`, `pg_trigger` bitflags). All list outputs are deterministically sorted so the agent can `diff` two databases reliably.
- **Write tools (hidden by `--read-only`)**: `create_table`, `alter_table`, `insert_query` (INSERT), `update_query` (UPDATE), `delete_query` (DELETE).
- **Read-only mode**: filtered in `buildTools()` *and* re-checked inside every write handler (double gate).
- **Prefix**: optional tool-name prefix (`--prefix`); `CallTool` accepts both prefixed and bare names.
- **Optional EXPLAIN** (per-call `explain` flag, default `false`): `select_query`/`count_query` return the `EXPLAIN (ANALYZE, BUFFERS)` plan **with** results; `insert_query`/`update_query`/`delete_query` return the plan **instead of executing** (plain `EXPLAIN`, no `ANALYZE`, so the DML is not run). Implemented by `ExplainPlan()` in `db.go`.
- **CSV output**: all SELECT results returned as CSV text.

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

- `PG_DSN` — Postgres DSN env var; used when `--dsn` flag is empty.
- `MARIADB_DSN` — MariaDB DSN env var; used when both `--dsn` and `PG_DSN` are empty.
- Behavior flags: `--prefix`, `--read-only`, `--log-level`
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

1. **`create_table`/`alter_table` are DDL and cannot be `EXPLAIN`ed** — the `explain` flag is intentionally only exposed on the read tools and the DML write tools (INSERT/UPDATE/DELETE), not on DDL.
2. **`desc_table` output is a synthetic pseudo-`CREATE TABLE` string** built by one big `information_schema` SQL (`query_handlers.go`), not real DDL: primary keys only (guessed via `constraint_name LIKE '%_pkey'`), no indexes/FKs/defaults; result read via the `?column?` key (Postgres name for the unnamed computed column).
3. **`select_query`/`insert_query` execute arbitrary SQL as given** — only the tool description constrains the LLM; the only enforcement is read-only mode for the write *tools*, not the statement type. "UPDATE must have WHERE" is description-only.
4. **Read-only ≠ schema-only**: `select_query` has no SELECT-only restriction (it runs whatever SQL the caller passes).
5. **`.github/workflows/docker-image.yml` builds a `Dockerfile` that doesn't exist** in the current tree — that workflow is currently broken.
6. **`.gitignore` also ignores `.github/`**, `bin/`, `*.swp`, and generic leftovers (Composer, WordPress, `target/`) — the ignore file is boilerplate, not project-specific; `.github`/`bin/.gitkeep` are still tracked despite this.
7. **Stray files**: `.README.md.swp` vim swap (gitignored), `.continue` (19-byte IDE artifact).
8. **`count_query` CSV**: expects a header row from `MapToCSV`; `list_tables` returns plain newline-separated names (no header) — inconsistent format between read tools.
9. **`parseStringParam`'s `fmt.Stringer` branch** is defensive (args arrive as native strings after JSON decode) — don't rely on it for non-string JSON values (`42` ⇒ error).
10. **`version` is hardcoded** (`"1.0.0"`) in `cmd/main.go`; there is also a TODO about a hardcoded prefix related to `mcp-gateway-go` not sending args.
11. **Git history is mixed**: older commits are an upstream MySQL/Docker variant (`guoling21cn`); recent commits ("first working version on stdio", "removed list databases", "application alignment") pivoted it to a Postgres stdio server. Some legacy assumptions may linger.

## Notes for AI Coding Agents

- **Keep stdout protocol-clean**: anything you add must log to stderr via `slog`.
- When **adding a tool**, touch all of: `buildTools()` in `tools.go`, the `switch` in `CallTool` (`postgres.go`), a new `handleX` in `query_handlers.go`/`write_handlers.go` (add read-only guard to write tools), the tool table in `README.md`, and an entry in `handler_test.go` if it affects tool counts (the RO test hardcodes 14 RO / 9 RO+introspection).
- **`go test ./...` must stay DB-free**; use `SetDB` only if you inject fakes.
- Do **not** change `textResponse` shape or CSV format without checking client-side consumers.
- `run.sh` and all `testing/test-*.sh` scripts assume binary at `bin/postgres-server` and repo root one level up from `testing/`.
- There is **no linter, no import ordering tool, no Makefile** — follow existing style (tabs, Go standard formatting).
- Dependencies should stay minimal (4 runtime deps: mcp SDK, sqlx, lib/pq, go-sql-driver/mysql); the value proposition is "single binary, no other runtime".
- **`explain` flag semantics**: read tools use `EXPLAIN (ANALYZE, BUFFERS)` (safe — SELECT-only side effects) and append the plan *after* the results; DML write tools use plain `EXPLAIN` (no `ANALYZE`) and return *only* the plan — this is deliberate, `EXPLAIN (ANALYZE …)` would execute the INSERT/UPDATE/DELETE.
- DDL tools (`create_table`/`alter_table`) have **no** `explain` flag — `EXPLAIN` cannot be applied to DDL.


## Update log

### 2026-09-11 — MariaDB dual-dialect support (Option A)

All 14 MCP tools now work against **PostgreSQL and MariaDB** in a single binary.
The dialect is **auto-detected from the DSN URL scheme** (`postgresql://` → Postgres,
`mysql://`/`mariadb://` → MariaDB) — no new CLI flag.

**New files:**
- `pkg/handler/dialect.go` — `Dialect` type (`DialectPostgres`, `DialectMariadb`), `DialectFromDSN()` (URL scheme detection), `String()`.

**Modified files:**
- `pkg/handler/postgres.go` — added `dialect Dialect` field, `DBType()` method; `SetDSN` now calls `DialectFromDSN`.
- `pkg/handler/db.go` — blank import `go-sql-driver/mysql`; `driverName(d)` returns `"mysql"` or `"postgres"`; `replaceDatabaseName(d, dsn, dbName)` validates scheme per dialect; `DB()` uses dialect-aware driver + DSN; `ExplainPlan()` uses `EXPLAIN ANALYZE` (MariaDB) / `EXPLAIN (ANALYZE, BUFFERS)` (Postgres) for reads, plain `EXPLAIN` for writes; `MapToCSV` now handles `nil` values (renders `NULL` instead of `<nil>`).
- `pkg/handler/query_handlers.go` — `handleListTable` and `handleDescTable` use `information_schema` with `GROUP_CONCAT`/`IFNULL` for MariaDB; Postgres keeps `string_agg`/`?column?`.
- `pkg/handler/inspect_handlers.go` — all 5 introspection tools have dialect branches: MariaDB uses `information_schema.routines`/`parameters`/`triggers`/`SEQ_10_5_6_SEQUENCES`, `SHOW CREATE FUNCTION`/`PROCEDURE`/`TRIGGER`; Postgres keeps `pg_proc`/`pg_trigger`/`pg_sequence`/`pg_get_*`. Event triggers section gated behind `dialect == DialectPostgres`.
- `pkg/handler/tools.go` — tool descriptions made dialect-neutral (removed "POSTGRES server", "public schema", "pg_catalog" references).
- `cmd/main.go` — flag help updated; `MARIADB_DSN` env var fallback; server name becomes `mcp-mariadb-server` when dialect is MariaDB; log message includes `db_type`.
- `run.sh` — accepts `PG_DSN` or `MARIADB_DSN`.
- `go.mod` — added `github.com/go-sql-driver/mysql v1.8.1`.

**MariaDB dialect notes:**
- No database-level event triggers (Postgres-only `pg_event_trigger`) — section omitted.
- No aggregate/window functions (MariaDB has only FUNCTION and PROCEDURE, not separate object types like Postgres).
- `EXPLAIN ANALYZE` in MariaDB **executes** the statement (fine for SELECT; DML write tools never call it with `analyze=true`).
- Sequences via `information_schema.SEQ_10_5_6_SEQUENCES` (MariaDB 10.3+); no `CACHE` column exposed.
- `desc_table` result key is `"desc"` (alias) in MariaDB vs `"?column?"` (unnamed computed column) in Postgres — both handled correctly.
- NULL values from MariaDB DECIMAL columns arrive as `[]byte`; MapToCSV now renders `NULL` for `nil` (was a latent bug for Postgres too).

**Verification required (run locally):**
```bash
cd /path/to/mcp-postgres-server
go mod tidy
go build ./...
go test ./... -count=1
```

### 2026-09-10 — added introspection tools (functions, triggers, sequences)

Five new **always-available** read-only tools for cross-database comparison
(mimicking the psql `\dfn`/`\ds`/`\d` output):

| Tool | Equivalent of | Notes |
|---|---|---|
| `list_functions` | `\dfn` | Functions, procedures **and** aggregates/window fns (`pg_proc.prokind IN 'f','p','a','w'`), one line per overload, sorted |
| `desc_function` | `\df+` | Full definition via `pg_get_functiondef()`; optional `args` resolves overloads |
| `list_triggers` | trigger section of `\d` | Table triggers (via `pg_trigger` bitflags) **plus** database-level `pg_event_trigger` section |
| `desc_trigger` | (none) | Full `CREATE TRIGGER` DDL via `pg_get_triggerdef()`, including the `WHEN` clause |
| `list_sequences` | `\ds` | `pg_sequence` joined to `pg_class` |

All new code lives in **`pkg/handler/inspect_handlers.go`** (new file, one flat
package as everywhere else). Handlers follow the existing parse→query→text pattern
and use `isSQLIdentifier` for any name/scheme/table/trigger/schema they embed in
generated SQL. Tool count is now **14 total / 9 read-only** (was 9 / 4); the
`TestReadOnlyToolFiltering` unit test was updated accordingly. `desc_function`
intentionally rejects overloads without an explicit `args` and lists the available
candidates so the caller can retry disambiguated.

### 2026-09-10 — fixed `MapToCSV` build error

`pkg/handler/db.go:204` had `for _, item := m` (missing `range`) — a
syntax error introduced while rewriting this file. Correct line is
`for _, item := range m`. The earlier suspicion in the notes that the
loop was valid Go was wrong.

### 2026-09-10 — removed hidden EXPLAIN gate; added per-call `explain` flag

The `--with-explain-check` flag and its `HandleExplain`/`ExplainResult`/`StatementType*`
mechanism were **removed**. The old pre-check was MySQL-shaped (`ExplainResult` scanned
`select_type`/`key_len`/`ref`/`filtered` columns) and did not work against real Postgres,
whose `EXPLAIN` emits a `QUERY PLAN` text column. The statement-type consts
(`StatementTypeNoExplainCheck`/`Select`/`Insert`/`Update`/`Delete`) are gone; `DoQuery`/
`HandleQuery`/`HandleExec` no longer take an `expect` parameter.

In place of it, a new **optional boolean `explain` flag (default `false`)** is accepted
per tool call:
- `select_query` / `count_query`: when `true`, `ExplainPlan()` runs `EXPLAIN (ANALYZE,
  BUFFERS)` and the plan is returned **alongside** the CSV results.
- `insert_query` / `update_query` / `delete_query`: when `true`, `ExplainPlan()` runs plain
  `EXPLAIN` (no `ANALYZE`, so the DML is **not executed**) and **only** the plan is returned.
- `create_table` / `alter_table`: DDL cannot be `EXPLAIN`ed — no `explain` flag.

New: `ExplainPlan()` and `parseBoolParam()` (`db.go`/`params.go`); removed the handler
field `withExplainCheck`, `SetWithExplainCheck`, and the `--with-explain-check` CLI flag
(`cmd/main.go`). `handleReadQuery` renamed to `handleSelectQuery`. All handler functions
now pass only `(database, query)` to `DoQuery`/`HandleExec`.

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


## 2026-09-11 — Investigation: can current tools create triggers / stored procedures?

Question: are the existing tools sufficient to CREATE triggers and stored procedures?
**Verdict: No — not as intended.** Inspection is fully supported; creation is not.

**Why (write path):** the only write tools are `create_table`, `alter_table`,
`insert_query`, `update_query`, `delete_query` (see `buildTools()` in `tools.go`,
routed in `CallTool` in `postgres.go`, executed via `handleCreateTable`/
`handleAlterTable`/`runDLQuery` in `write_handlers.go`). There is **no dedicated
tool** for DDL of functions/procedures/triggers — nothing named `create_trigger`,
`create_procedure`, or a generic `exec_ddl`. So an LLM has no tool that semantically
matches "create a trigger/procedure".

- All write handlers funnel into **`HandleExec()` → `db.Exec(query)`** (single
  pool, single `Exec` call — `db.go`). No multi-call / transaction / multi-statement
  helper exists.
- `insert/update/delete` go through `runDLQuery`; `explain` flag does nothing
  useful here (DDL can't be EXPLAINed, and these DML tools aren't the DDL case).

**Why (statement-count requirement — the real blocker):**
- **Stored procedure** = ONE statement (`CREATE [OR REPLACE] PROCEDURE ... END;`).
  Works with a single `db.Exec` on **both** dialects (the `;` inside `BEGIN..END`
  are part of the statement, not separators).
- **Stored function (expression/PL body)** = ONE statement on both dialects → also
  fine with a single `Exec`.
- **Trigger** = ALWAYS **TWO** statements: `CREATE [OR REPLACE] FUNCTION ... $$...$$`
  *then* `CREATE TRIGGER ... EXECUTE FUNCTION ...`. A single `Exec` only reliably
  runs one statement unless the driver permits multi-statement. `toMariaDBDSN()`
  (`db.go`) does **not** set MariaDB `?multiStatements=true`, so on MariaDB a
  function+trigger pair as one string **will not** both execute. On Postgres
  `lib/pq`'s *Exec* *may* run multiple `;`-separated statements (simple query
  protocol), but that is driver-specific and unverified here — treat as a risk,
  not a guarantee.

**Read path (fine):** `desc_function` returns full DDL incl. bodies
(`pg_get_functiondef` / `SHOW CREATE PROCEDuRE|FUNCTION`), and `desc_trigger`
returns full `CREATE TRIGGER` DDL (both in `inspect_handlers.go`). `list_functions`
/ `list_triggers` also cover both. So *inspecting* existing objects is complete.

**Conclusion:** creation of triggers/procedures is NOT sufficiently supported.
Gaps to close (for a future change, not now):
1. Add a DDL-write tool (e.g. `create_object` / generic `exec_ddl`, or rename
   `create_table`→object-level) so an LLM has a clear target for
   functions/procedures/triggers.
2. Execute trigger creation as **two** sequential `Exec` calls (function, then
   trigger) rather than one multi-statement string — or, if keeping one string,
   set MariaDB DSN to `?multiStatements=true` in `toMariaDBDSN()` and accept it
   only returns the last statement's result.
3. Keep `explain` out of the DDL path (DDL can't be EXPLAINed; consistent with
   `create_table`/`alter_table`).
4. Verify live: single-statement `CREATE PROCEDURE` (both dialects) and
   two-statement trigger function+trigger on Postgres/MariaDB before committing
   to any approach.
### 2026-09-11 — added `create_function` and `create_trigger` tools

Two new **write tools** (hidden in `--read-only` mode), added alongside the
existing `create_table`/`alter_table` DDL tools:

| Tool | Purpose |
|---|---|
| `create_function` | `CREATE [OR REPLACE] FUNCTION` **or** `CREATE [OR REPLACE] PROCEDURE` (single statement, works on both Postgres and MariaDB) |
| `create_trigger`  | `CREATE TRIGGER` (single statement; handler function **must** already exist — call `create_function` first) |

**Files changed:**
- `tools.go` — tool definitions appended after `delete_query` in the write-tools block; descriptions include the "call `create_function` first" cross-reference
- `write_handlers.go` — `handleCreateFunction` and `handleCreateTrigger` (~50 lines, identical structure to `handleCreateTable`/`handleAlterTable`)
- `postgres.go` — `CallTool` switch cases for both tools
- `handler_test.go` — expected RW count updated 14 → 16; both names added to the RO exclusion list

**Design notes:**
- No `explain` flag (DDL can't be `EXPLAIN`ed, consistent with `create_table`/`alter_table`)
- Each tool runs exactly **one** `db.Exec` call — no multi-statement issue
- `create_function` handles both `FUNCTION` and `PROCEDURE` (one tool, no split — avoids adding tool noise for a small use-case distinction)
- `create_trigger` expects the handler function to already exist; the description tells the LLM to call `create_function` first

**Tool count:** 16 total (was 14) / 9 read-only (unchanged).

**New integration tests** (`testing/`, added to `run-all-tests.sh` before teardown):
- `test-create_function.sh` — calls `create_function` on both dialects with `CREATE OR REPLACE FUNCTION cf_probe` (idempotent), best-effort CLI cleanup.
- `test-create_trigger.sh` — validates the documented flow: PG does `create_function` (handler `ctg_probe_fn`) → `create_trigger` (single `CREATE TRIGGER … EXECUTE FUNCTION`); MariaDB does a single inline-body `CREATE TRIGGER`. Each `create_trigger` call stays a **single** statement (no multi-statement); idempotency via guarded DB-CLI `DROP` first (mirrors `test-seed.sh`). Self-cleanup at the end.

Note: these require a live Postgres + MariaDB (via `common.sh` DSNs), like the other integration tests. Run `bash testing/test-create_function.sh` / `bash testing/test-create_trigger.sh` directly, or via `run-all-tests.sh`.
