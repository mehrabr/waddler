# waddler

I got tired of setting up Python environments every time someone needed a CSV cleaned and joined against a Postgres table. So I built this. It's a Go binary that takes a YAML file describing your data sources and a SQL transform, runs it through DuckDB, and writes the result wherever you want. That's pretty much it.

The reason it's Go and not Python is that a Go binary has no runtime dependencies. You hand someone an executable, they run it. No pip, no virtualenv, no "which python3 is this using."

## Quick start

```bash
CGO_ENABLED=1 go install github.com/mehrabr/waddler/cmd/waddler@latest

waddler run pipeline.yml
waddler validate pipeline.yml
waddler serve pipeline.yml   # blocks on a cron schedule, Ctrl+C to stop
waddler sources
```

Note: `go-duckdb` bundles DuckDB's C++ library so CGO is required. You need a C compiler: `xcode-select --install` on macOS, `apt install build-essential` on Linux.

## Pipeline format

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
  SELECT
    d.donor_id,
    dn.name,
    ROUND(SUM(d.amount), 2) AS total_donated,
    CASE
      WHEN SUM(d.amount) >= 1000 THEN 'major'
      WHEN SUM(d.amount) >= 100  THEN 'regular'
      ELSE 'small'
    END AS donor_tier
  FROM donations d
  JOIN donors dn USING (donor_id)
  WHERE d.amount > 0
  GROUP BY ALL
  ORDER BY total_donated DESC

validate:
  - name: no negative totals
    sql: SELECT COUNT(*) FROM ({transform}) WHERE total_donated < 0
    expect: 0

output:
  type: parquet
  path: output/donor_report.parquet

schedule: "0 6 * * *"
```

## Sources

`csv`, `json`, and `parquet` just need a `path`. DuckDB handles schema detection automatically.

`postgres` needs a `dsn` and `table`. It uses DuckDB's postgres extension which installs itself on first use, so nothing extra to set up.

`motherduck` needs a `table` and `MOTHERDUCK_TOKEN` in your environment (or `options.token` in the yaml if you prefer). Add `options.database` if your target isn't `my_db`.

## Outputs

`parquet` and `csv` need a `path`. Parquet defaults to snappy compression, pass `compression: zstd` if you want smaller files.

`motherduck` needs `database` and `table`. Defaults to replacing the table each run, set `mode: append` to insert instead.

## Validation

You can add a `validate` block that runs before output is written. If any check fails the pipeline stops and tells you what went wrong, nothing gets written.

```yaml
validate:
  - name: no negative prices
    sql: SELECT COUNT(*) FROM ({transform}) WHERE price < 0
    expect: 0

  - name: at least 100 rows
    sql: SELECT COUNT(*) FROM ({transform})
    expect_min: 100
```

`{transform}` gets substituted with your pipeline SQL at runtime, so you're asserting against the actual result set.

## Scheduling

Add a `schedule` field with a cron expression and use `waddler serve` instead of `waddler run`. It blocks and reruns the pipeline on that schedule. A systemd unit or a docker container on a cheap VPS works fine for this.

```yaml
schedule: "0 6 * * *"   # 6am daily
```

## Building from source

```bash
git clone https://github.com/mehrabr/waddler
cd waddler
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go build -o waddler ./cmd/waddler
```

## Examples

- [`examples/donor_report.yml`](examples/donor_report.yml)
- [`examples/scheduled_sync.yml`](examples/scheduled_sync.yml)
- [`examples/postgres_to_parquet.yml`](examples/postgres_to_parquet.yml)

MIT
