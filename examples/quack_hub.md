# Running a waddler hub

A hub is a persistent DuckDB file served over DuckDB's native Quack protocol.
Branch pipelines push results to it with a `quack` output.

## Start the hub

```bash
waddler hub \
  --db ./hub.duckdb \
  --listen 0.0.0.0:9494 \
  --token-file ./hub.token
```

On first start it generates an auth token and writes it to `hub.token`
(`0600`). The hub database persists across restarts; the token is reused.

## Point pipelines at it

Give each branch the token as an environment variable and use a `quack` output:

```bash
export WADDLER_QUACK_TOKEN="$(cat hub.token)"
waddler run quack_client.yml
```

```yaml
output:
  type: quack
  url: quack:hub.example.com:9494
  table: donor_report
  mode: append           # accumulate across branches
  token: ${WADDLER_QUACK_TOKEN}
```

## Inspect the hub

The hub file is a normal DuckDB database:

```bash
duckdb hub.duckdb "SELECT * FROM main.donor_report ORDER BY total_donated DESC"
```

## Notes

- Quack needs DuckDB ≥ 1.5.3 (the bundled engine provides it).
- Transport, auth, and the wire protocol are all DuckDB's Quack — there is no
  bespoke relay server to run or secure.
- Put the hub behind TLS / a private network for production; treat the token
  like a password.
