# Roadmap

waddler is a single-binary DuckDB ETL tool. The Quack hub turns it into a
"many pipelines → one shared DuckDB" system. The next milestone generalizes the
hub from *one shared database* into *many isolated, per-tenant databases* — a
small control plane in the spirit of MotherDuck's hypertenancy ("a flock of
ducklings").

## Now

- Pipelines: sources → SQL transform → validation → output.
- `quack` output/source + `waddler hub` (native Quack): push/read against one
  shared DuckDB over an authenticated protocol.

## Next — a mini multi-tenant control plane

Evolve the hub from one shared engine into a control plane in front of many:

1. **Routing / lifecycle.** Accept `{query, tenant token}`, route to that
   tenant's engine, stream results back. (The hub already does the single-engine
   version of this.)
2. **Per-tenant engines.** Spin up a DuckDB instance per tenant on first use;
   track state; scale to zero after an idle timeout.
3. **Isolation.** Process-per-tenant with per-engine memory/thread caps and a
   concurrency queue so one heavy query can't starve others. Benchmark the
   noisy-neighbor behavior before/after.
4. **Caching.** A result/parquet cache in object storage; measure hit rate and
   p50/p99.
5. **Metadata + auth + file visibility.** A tenant catalog, token auth, and
   per-tenant control over which tables/files are visible.
6. **Observability.** Structured logs and a `/metrics` endpoint exposing cold
   start time, idle scaledown, latency, and cache hit rate.

Each item is one milestone and one build-in-public writeup. Quack can stay the
transport between the control plane and its per-tenant engines, so the protocol
work here carries forward rather than being thrown away.

## Stretch — write a little C++

Ship a small DuckDB extension (a scalar function or query-timing
instrumentation) or land one modest fix in DuckDB OSS, to demonstrate working in
the engine's actual codebase, not just around it.
