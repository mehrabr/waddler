# waddler

ETL pipelines from a single YAML file, powered by DuckDB.

The motivation was simple: small orgs with a volunteer data manager or a one-person ops team shouldn't need to wrangle Python environments or pay for Fivetran to join two CSVs and write a Parquet file. Describe your sources, write your SQL, declare your output. Done.

It's a Go binary with no runtime dependencies. Hand someone the executable and they can run it without touching pip or conda or any of that.

## Quick start

```bash
CGO_ENABLED=1 go install github.com/mehrabr/waddler/cmd/waddler@latest

waddler run pipeline.yml
waddler validate pipeline.yml    # check config without running
waddler serve pipeline.yml       # run on a schedule, blocks until Ctrl+C
waddler sources                  # list supported source/output types
```

`go-duckdb` embeds DuckDB's C++ library, so you need a C compiler: `xcode-select --install` on macOS or `apt install build-essential` on Linux.

## Pipeline format

Each pipeline is one YAML file. Sources become named views in DuckDB, your SQL runs against them, and the result goes to the output.

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
    dn.email,
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

`csv`, `json`, and `parquet` take a `path`. DuckDB infers the schema automatically.

`postgres` takes a `dsn` and `table`. Uses DuckDB's postgres extension, which installs itself on first use.

`motherduck` takes a `table` plus `MOTHERDUCK_TOKEN` in your environment (or `options.token` in the YAML). Set `options.database` if the target isn't `my_db`.

## Outputs

`parquet` and `csv` take a `path`. Parquet defaults to snappy; set `compression: zstd` for smaller files.

`motherduck` takes `database` and `table`. Replaces the table by default. Set `mode: append` to insert rows instead.

`quack` takes a `url` and `table`. The token must reference an environment variable (`token: ${RELAY_TOKEN}`) — the validator rejects bare literals. Non-local URLs log a warning reminding you to put nginx in front.

## Validation

The `validate` block runs SQL assertions against your transform result before anything gets written. All failures are collected and reported together, so you see everything at once rather than fixing one thing at a time.

```yaml
validate:
  - name: no negative prices
    sql: SELECT COUNT(*) FROM ({transform}) WHERE price < 0
    expect: 0

  - name: at least 100 rows
    sql: SELECT COUNT(*) FROM ({transform})
    expect_min: 100

  - name: not exploding in size
    sql: SELECT COUNT(*) FROM ({transform})
    expect_max: 1000000
```

`{transform}` is substituted with your pipeline SQL at runtime.

## Scheduling

Add a `schedule` field and use `waddler serve` instead of `waddler run`. It blocks and reruns on that schedule. A systemd unit or a small Docker container on a cheap VPS is all you need.

```yaml
schedule: "0 6 * * *"   # 6am daily
```

## Relay

The relay lets multiple waddler instances push pipeline results into a shared DuckDB file on a central server. The main use case is a regional org where each branch office runs waddler locally and pushes weekly data to a hub that a coordinator can query in one place.

Start the hub on your central server:

```bash
waddler relay \
  --db ./hub.duckdb \
  --port 9494 \
  --token-file ./relay.token \
  --allowed-pipelines donor_report,weekly_sync
```

On first start it generates a random token and writes it to `--token-file`. Hand that token to each branch office as `RELAY_TOKEN`. Each branch pipeline then uses a quack output pointing at the hub:

```yaml
output:
  type: quack
  url: quack:hub.example.com:9494
  token: ${RELAY_TOKEN}
  table: donor_report
  mode: replace
```

The relay enforces the allowlist and a row limit (default 10M, set with `--max-rows`). Rejected runs are logged server-side with a reason. Accepted runs are logged with pipeline name, row count, and duration.

**SSL:** put nginx in front for anything beyond localhost. Here's a minimal config:

```nginx
server {
    listen 443 ssl;
    server_name hub.example.com;

    ssl_certificate     /etc/letsencrypt/live/hub.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/hub.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:9494;
        proxy_set_header Host $host;
    }
}
```

The relay is the self-hosted option. If you want managed storage, replication, or access controls beyond a token, MotherDuck is the right call instead.

See [`examples/relay_hub.yml`](examples/relay_hub.yml) and [`examples/relay_client.yml`](examples/relay_client.yml) for the full setup.

## Building from source

```bash
git clone https://github.com/mehrabr/waddler
cd waddler
go mod tidy
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go build -o waddler ./cmd/waddler
```

## Examples

- [`examples/donor_report.yml`](examples/donor_report.yml) - two CSVs joined and aggregated to Parquet
- [`examples/scheduled_sync.yml`](examples/scheduled_sync.yml) - nightly cron sync to MotherDuck with validation
- [`examples/postgres_to_parquet.yml`](examples/postgres_to_parquet.yml) - Postgres table exported to zstd Parquet
- [`examples/relay_hub.yml`](examples/relay_hub.yml) - hub server setup notes
- [`examples/relay_client.yml`](examples/relay_client.yml) - branch office client pushing to the hub

---

MIT
