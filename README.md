# lapace-import

> **Docs policy**: module-local docs live in this repo's `docs/`. Cross-cutting
> knowledge lives in [`lapace-docs/`](../lapace-docs/) per
> [`DOCS_POLICY.md`](../lapace-docs/DOCS_POLICY.md).

Offline sim-native telemetry import for LaPace. Converts per-sim telemetry files
into LaPace's signal-family SessionStint DuckDB format.

## Direction

- **LMU** — official Le Mans Ultimate DuckDB exports → SessionStint DBs.
- **iRacing** — IBT binary telemetry files → SessionStint DBs (stub).
- Future sims land as new adapter packages under `internal/`.
- Depends only on `lapace-core` for types and DB schema declarations.
- Per-sim CLI binary; `lapace-control` invokes the binaries, never the Go packages.

## Layout

```
internal/
  core/         # shared: writer, math, Adapter interface, supporting types
  lmu/          # LMU adapter (port of legacy internal/importlmu)
  iracing/      # iRacing adapter (initial stub)
cmd/
  import-lmu/         # thin CLI → lmu.Adapter
  import-iracing/     # thin CLI → iracing.Adapter (stub)
docs/
  guides/             # per-sim user guides
  technical/          # adapter-contract.md
  adr/                # module-local ADRs
  status.md           # current state
  backlog.md          # open work
```

## Build & Test

```bash
go build ./...
go test ./...
```

## Quick start (LMU)

```bash
# From the repo root:
go run ./cmd/import-lmu --input=../sessions_data/sampledata/lmu_duckdb/practice_spa.duckdb --sessions-dir=../sessions
```

See [`docs/guides/lmu-import-guide.md`](docs/guides/lmu-import-guide.md) for the full guide.

## References

- [Module Map](https://github.com/La-Pace/lapace-docs/blob/main/module-map/overview.md)
- [Adapter contract](docs/technical/adapter-contract.md) — the boundary this repo enforces
- [ADR-0001: adapter separation](docs/adr/0001-adapter-separation.md)
- [lapace-docs ADR-16](https://github.com/La-Pace/lapace-docs/blob/main/architecture/adr/adr-16-lmu-official-export-import.md) — historical motivation (predates the C2 split)
- Legacy source: `~/Codebase/lapace-workspace/LaPace/internal/importlmu/`, `LaPace/cmd/import-lmu/`
