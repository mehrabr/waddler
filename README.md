# waddler

> **No-code ETL for the rest of us.**  
> One YAML file. One command. DuckDB does the rest.

Waddler is a single Go binary that reads a YAML pipeline file and runs a complete ETL pipeline using DuckDB as the query engine. No Python environment, no Spark cluster, no Airflow. Drop CSVs, JSONs, or Parquet files in, write your SQL, get clean output — on a schedule if you want.

---

## Quick start

```bash
CGO_ENABLED=1 go install github.com/mehrabr/waddler/cmd/waddler@latest

waddler run pipeline.yml
waddler validate pipeline.yml
waddler serve pipeline.yml   # blocks; Ctrl+C to stop
waddler sources
```

---

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

---

## Source types

| Type | Required fields | Notes |
|------|----------------|-------|
| `csv` | `path` | Auto-detects schema |
| `json` | `path` | Auto-detects schema |
| `parquet` | `path` | — |
| `postgres` | `dsn`, `table` | Uses DuckDB's postgres extension |
| `motherduck` | `table`, `options.database` | Set `MOTHERDUCK_TOKEN` env var |

## Output types

| Type | Required fields | Notes |
|------|----------------|-------|
| `parquet` | `path` | Default compression: snappy |
| `csv` | `path` | UTF-8, header row included |
| `motherduck` | `database`, `table` | Set `MOTHERDUCK_TOKEN`; supports `mode: append` |

---

## Validation rules

```yaml
validate:
  - name: no negative prices
    sql: SELECT COUNT(*) FROM ({transform}) WHERE price < 0
    expect: 0         # exact match

  - name: at least 100 rows
    sql: SELECT COUNT(*) FROM ({transform})
    expect_min: 100   # lower bound

  - name: reasonable size
    sql: SELECT COUNT(*) FROM ({transform})
    expect_max: 1000000
```

`{transform}` is substituted with the pipeline's SQL at runtime.

---

## Scheduling

```yaml
schedule: "0 6 * * *"   # daily at 6am
```

```bash
waddler serve pipeline.yml
```

A systemd service or Docker container on a $5/month VPS is enough for most small orgs.

---

## Building from source

```bash
git clone https://github.com/mehrabr/waddler
cd waddler
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go build -o waddler ./cmd/waddler
```

`go-duckdb` embeds DuckDB's C++ library and requires a C compiler.  
macOS: `xcode-select --install` · Linux: `apt install build-essential`

---

## Examples

- [`examples/donor_report.yml`](examples/donor_report.yml)
- [`examples/scheduled_sync.yml`](examples/scheduled_sync.yml)
- [`examples/postgres_to_parquet.yml`](examples/postgres_to_parquet.yml)

---

MIT License
