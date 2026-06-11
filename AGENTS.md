# lapace-import — Agent Rules

## Mission

Offline sim-native telemetry import for LaPace. Convert per-sim telemetry files (LMU DuckDB exports, iRacing IBT, future sims) into LaPace's signal-family SessionStint DBs.

## Rules

- Depends only on `github.com/La-Pace/lapace-core` for contract types and DB schema.
- No imports from coaching, receiver, dashboard, capture, or control module internals. (Adapter packages must not import each other either.)
- The DuckDB Go driver is forbidden inside adapter packages — only `internal/core` may import it.
- `internal/core` owns exactly three things: the writer, the math, and the `core.Adapter` interface. Anything else is a new ADR.
- Each sim has its own thin CLI binary in `cmd/import-<sim>/`. No `--sim` flag.
- Adapters own their `SessionType` → phase mapping, including the default policy. Unknown session types must emit a `PreviewIssue{IssueWarning, "unknown_session_type", …}` — silent fallback is not allowed.
- `lapace-control` invokes the binaries as subprocesses (ADR-19), not the Go packages.

## Build & Test

```bash
go build ./...
go test ./...
```

The boundary import test (`internal/core/import_boundary_test.go`) enforces the no-cross-adapter-imports rule. Add a new adapter to the whitelist when adding a new sim.

## Reference

- `docs/technical/adapter-contract.md` — the boundary this repo enforces
- `docs/adr/0001-adapter-separation.md` — why the layout is shaped this way
- Legacy source: `~/Codebase/lapace-workspace/LaPace/internal/importlmu/`, `LaPace/cmd/import-lmu/`
