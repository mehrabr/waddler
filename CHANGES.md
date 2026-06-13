# What changed in this fix

The goal was a tool that **runs** and does its core job well: a friendly DuckDB
wrapper that takes a YAML pipeline (sources → SQL transform → checks → output)
and executes it. To get there, two things that could never work were removed,
and the SQL the tool generates was made correct and safe.

## Correctness fixes (these are why it now runs)

1. **`append` to MotherDuck no longer produces a syntax error.**
   The old code generated `CREATE TABLE IF NOT EXISTS t AS (SELECT ...) WHERE false`,
   which DuckDB rejects (`CREATE TABLE ... AS SELECT` takes no trailing `WHERE`).
   It now wraps the result in a subquery:
   `CREATE TABLE IF NOT EXISTS t AS SELECT * FROM (<result>) AS _src WHERE 1=0`,
   then `INSERT INTO t BY NAME SELECT * FROM (<result>) AS _src`. This is also
   robust to transforms that end in `ORDER BY` / `GROUP BY`.

2. **File paths with quotes or spaces no longer break.**
   `read_csv_auto('/data/o'brien.csv')` was a parser error. Paths (and DSNs, and
   connection strings) are now quoted as SQL literals with embedded quotes
   doubled (`internal/sqlutil`).

3. **The transform runs once, not many times.**
   It used to run for the row count, again for every validation rule, and again
   for the output. It is now materialized into a temp table once; the count,
   the checks, and the output all read from that, so results are consistent and
   large transforms aren't re-executed.

## Security fixes

4. **No more SQL injection through identifiers.**
   View names, table names, and ATTACH aliases were interpolated raw, so a
   source named `x"; DROP ...` executed arbitrary SQL. Identifiers are now
   quoted, and source names are validated as plain identifiers up front.

5. **Secrets no longer leak into errors or logs.**
   The engine used to attach the full SQL — including `motherduck_token=...` —
   to every error. It no longer includes SQL text in errors. A new `secret`
   package resolves `${VAR}` references, and literal tokens in YAML are rejected
   by `validate`.

6. **`parquet` compression is validated** against an allowlist, closing another
   string-interpolation hole and giving a clear error for typos.

## Removed (could not work; out of scope for the core tool)

7. **The `quack` source/output type** was removed. It used
   `ATTACH ... (TYPE quack)`, a DuckDB extension that the bundled engine cannot
   load, and its connection string mixed incompatible syntaxes. It never ran.

8. **The `relay` subcommand and package** were removed. It was the server half
   of the `quack` feature; the two halves spoke different protocols and never
   connected, and it isn't part of the core ETL premise. (It can be reintroduced
   later on top of DuckDB's real client/server support.)

## Cleanups

9. Two copy-pasted `expandToken` helpers collapsed into one `secret.Expand`.
10. MotherDuck configuration unified on top-level `database` / `table` / `token`
    fields (previously split inconsistently between top-level and `options`).
11. `postgres` DSNs may now use `${VAR}`, so no DSN with a password needs to live
    in a file.
12. Examples now point at bundled sample data and run as-is; the example with a
    hard-coded Postgres password was changed to use `${CRM_DSN}`.

## New layout

```
internal/
  config/    pipeline parsing + validation
  engine/    the DuckDB wrapper (safe Exec, quoting, materialize/count, exports)
  source/    csv/json/parquet/postgres/motherduck → views
  loader/    parquet/csv/motherduck outputs
  runner/    orchestrates: register → materialize → validate → write
  secret/    ${VAR} resolution (new)
  sqlutil/   identifier/literal quoting (new)
cmd/waddler/ CLI: run, validate, serve, sources
examples/    runnable example + sample data
```

## Verifying it builds and runs

The DuckDB-backed packages need cgo and network access to fetch dependencies, so
build them in your own environment:

```bash
CGO_ENABLED=1 go build -o waddler ./cmd/waddler
CGO_ENABLED=1 go test ./...
cd examples && ../waddler run donor_report.yml
```

The pure-logic packages (`sqlutil`, `secret`, `config`) have unit tests covering
the quoting rules, the injection regression case, and the validation rules.

---

# Phase 2 — native Quack hub

Adds a working "push results to a shared hub" path, built on DuckDB's native
Quack client/server protocol (DuckDB ≥ 1.5.3) instead of the hand-rolled HTTP
relay that was removed during the cleanup.

- **`build(deps)`** — bumped go-duckdb to the v2 line that embeds DuckDB ≥ 1.5.3,
  and added an engine version guard (`RequireDuckDB`) so a too-old engine fails
  with a clear message instead of an obscure Quack error.
- **`quack` output** — pushes the materialized result to a hub with a real
  `ATTACH 'quack:host:port' (TYPE quack, TOKEN …)` + `CREATE OR REPLACE TABLE` /
  `INSERT … BY NAME`. Supports `replace` (default) and `append`.
- **`quack` source** — reads a table from a hub over the same protocol.
- **`waddler hub`** — serves a persistent DuckDB file via `quack_serve`,
  generating and persisting an auth token. Replaces the old `relay` command.
- Tokens use `${VAR}` / `WADDLER_QUACK_TOKEN` and are never written to YAML or
  logs; URLs and identifiers go through the same quoting helpers as the rest of
  the tool.

This is the foundation for the next milestone — see `docs/ROADMAP.md`.
