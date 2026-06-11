# lapace-import

> **Docs policy**: module-local docs go in this repo's `docs/`. Cross-cutting
> knowledge goes in [`lapace-docs/`](../lapace-docs/) per
> [`DOCS_POLICY.md`](../lapace-docs/DOCS_POLICY.md).

Offline sim-native telemetry import for LaPace — LMU DuckDB exports and iRacing IBT files into signal-family SessionStint DBs.

## Direction

- **LMU** — official Le Mans Ultimate DuckDB exports → SessionStint DBs.
- **iRacing** — IBT binary telemetry → SessionStint DBs (stub).
- Per-sim CLI; `lapace-control` invokes the binaries, not the Go packages.
- Adapter packages own sim-specific policy (vocabulary, units, phase mapping). `internal/core` owns the writer, the math, and the `core.Adapter` interface. The boundary is *vocabulary*, not file format.

## Build & Test

```bash
go build ./...
go test ./...
```

## References

- [Module Map](https://github.com/La-Pace/lapace-docs/blob/main/module-map/overview.md)
- [Adapter contract](docs/technical/adapter-contract.md)
- [ADR-0001: adapter separation](docs/adr/0001-adapter-separation.md)
- [lapace-docs ADR-16](https://github.com/La-Pace/lapace-docs/blob/main/architecture/adr/adr-16-lmu-official-export-import.md) — historical motivation
- Legacy source: `~/Codebase/lapace-workspace/LaPace/internal/importlmu/`, `LaPace/cmd/import-lmu/`
