# waddler

**Zero-code ETL pipelines powered by DuckDB.** Describe a pipeline in one YAML
file — sources, a SQL transform, optional data-quality checks, and an output —
and `waddler` runs it through an embedded DuckDB. One static binary, no server,
no orchestration.

```yaml
name: monthly-donor-report
sources:
  - name: donations
    type: csv
    path: data/donations_2024.csv
  - name: donors
    type: csv
    path: data/donors.csv
transform: |
  SELECT d.donor_id, dn.name, ROUND(SUM(d.amount), 2) AS total_donated,
         COUNT(*) AS donation_count,
         CASE WHEN SUM(d.amount) >= 1000 THEN 'major'
              WHEN SUM(d.amount) >= 100  THEN 'regular'
              ELSE 'small' END AS donor_tier
  FROM donations d JOIN donors dn USING (donor_id)
  WHERE d.amount > 0
  GROUP BY ALL ORDER BY total_donated DESC
output:
  type: parquet
  path: output/donor_report.parquet
```

```
waddler run donor_report.yml
✅ monthly-donor-report: 2 rows → output/donor_report.parquet in 22ms
```

## Install

Requires Go 1.22+ and a C compiler (DuckDB is embedded via cgo).

```bash
git clone https://github.com/mehrabr/waddler.git
cd waddler
CGO_ENABLED=1 go build -o waddler ./cmd/waddler
# optional: move it onto your PATH
sudo mv waddler /usr/local/bin/
```

## Quick start

The repo ships a runnable example and sample data:

```bash
cd examples
waddler run donor_report.yml
# writes output/donor_report.parquet
```

Inspect the result with the DuckDB CLI:

```bash
duckdb -c "SELECT * FROM 'examples/output/donor_report.parquet'"
```

## Commands

```
waddler run <pipeline.yml>        Execute a pipeline once
waddler validate <pipeline.yml>   Check a pipeline file without running it
waddler serve <pipeline.yml>      Run on the pipeline's cron schedule (Ctrl+C to stop)
waddler sources                   List supported source and output types
```

## Pipeline reference

A pipeline file has up to six top-level keys:

- **`name`** (required) — pipeline name, shown in logs and output.
- **`sources`** (required) — one or more inputs. Each becomes a view named after
  its `name`, which your transform queries directly.
- **`transform`** (required) — a single SQL `SELECT` over the source views.
- **`validate`** (optional) — data-quality rules run before the output is
  written. If any fail, nothing is written.
- **`output`** (required) — where the result goes.
- **`schedule`** (optional) — a cron expression for `waddler serve`.

### Sources

| type | required fields | notes |
|------|-----------------|-------|
| `csv` | `path` | local CSV |
| `json` | `path` | local JSON |
| `parquet` | `path` | local Parquet |
| `postgres` | `dsn`, `table` | `dsn` may use `${VAR}`; optional `options.schema` (default `public`) |
| `motherduck` | `table` | `MOTHERDUCK_TOKEN` env or `token: ${VAR}`; optional `database` (default `my_db`) |
| `quack` | `url`, `table` | reads a table from a `waddler hub`; `WADDLER_QUACK_TOKEN` env or `token: ${VAR}` |

### Outputs

| type | required fields | notes |
|------|-----------------|-------|
| `parquet` | `path` | optional `compression`: snappy (default), zstd, gzip, lz4, uncompressed |
| `csv` | `path` | header row included |
| `motherduck` | `table` | `mode`: `replace` (default) or `append`; optional `database` |
| `quack` | `url`, `table` | pushes to a `waddler hub`; `mode`: `replace` (default) or `append`; `WADDLER_QUACK_TOKEN` env or `token: ${VAR}` |

### Validation

Each rule runs a SQL query that returns a single number and compares it to an
expectation. Use the `{transform}` placeholder to refer to the transform's
result:

```yaml
validate:
  - name: no negative totals
    sql: SELECT COUNT(*) FROM ({transform}) WHERE total_donated < 0
    expect: 0
  - name: at least one row
    sql: SELECT COUNT(*) FROM ({transform})
    expect_min: 1
```

Supported checks: `expect` (exact), `expect_min`, `expect_max`. All failing
rules are reported together.

### Scheduling

```yaml
schedule: "0 6 * * *"   # daily at 6am
```

```bash
waddler serve pipeline.yml
```

## Secrets

Never put credentials in a pipeline file. Reference an environment variable:

```yaml
sources:
  - name: customers
    type: postgres
    dsn: ${CRM_DSN}
    table: customers
```

```bash
export CRM_DSN="host=localhost dbname=crm user=readonly password=..."
export MOTHERDUCK_TOKEN="..."
waddler run pipeline.yml
```

`waddler` refuses literal tokens in YAML and never prints connection strings or
tokens in its error output.

## Development

```bash
CGO_ENABLED=1 go test ./...     # run the test suite
go vet ./...
gofmt -l .                      # should print nothing
```

## License

MIT — see [LICENSE](LICENSE).

## Quack hub: push results to a shared DuckDB

A `waddler hub` serves a persistent DuckDB file over DuckDB's native **Quack**
protocol. Branch pipelines push their results to it with a `quack` output —
useful for "many small pipelines → one shared warehouse" without standing up a
database server. (Quack needs DuckDB ≥ 1.5.3, which the bundled engine provides.)

Start a hub:

```bash
waddler hub --listen 0.0.0.0:9494 --token-file hub.token
# prints the listen URI; writes an auth token to hub.token on first start
```

Point a pipeline at it:

```yaml
output:
  type: quack
  url: quack:hub.example.com:9494
  table: donor_report
  mode: replace            # or append
  token: ${WADDLER_QUACK_TOKEN}
```

```bash
export WADDLER_QUACK_TOKEN="$(cat hub.token)"
waddler run examples/quack_client.yml
```

You can also read from a hub by using `type: quack` as a source (same `url` /
`table` / token). See `examples/quack_client.yml` and `examples/quack_hub.md`.
The transport, authentication, and wire protocol are all DuckDB's Quack — there
is no bespoke relay server to run or secure.
