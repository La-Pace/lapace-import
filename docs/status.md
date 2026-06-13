# Status — `lapace-import`

**Repo status:** Complete (first cut)
**Last updated:** 2026-06-13

## Current state

`lapace-import` exists at `github.com/La-Pace/lapace-import`, builds clean, and
passes its tests. The first cut — LMU offline import into signal-family
SessionStint DBs — is shipped and pushed to `origin/main` (in sync). It is the
Phase 1.5 module in the
[module map](https://github.com/La-Pace/lapace-docs/blob/main/module-map/overview.md).

## What's done

- Repo scaffolded with the ADR-0001 layout, `lapace-core` as the only Go dep,
  and the workspace's standard `AGENTS.md` / `CLAUDE.md`.
- LMU adapter ported from legacy `LaPace/internal/importlmu/` to `internal/lmu/`
  — reader, mapping, converter, group, preview, setup-adapter, writer — plus CLI
  (`cmd/import-lmu/`) and `testdata/`. Import paths retargeted to
  `github.com/La-Pace/lapace-core/contract/...`.
- Shared core extracted to `internal/core/`: generalized `core.Writer`,
  `ReconstructTimestamps`, the `core.Adapter` interface, and the boundary
  isolation test (`import_boundary_test.go`).
- iRacing stub landed: `internal/iracing/adapter.go` implements `core.Adapter`
  (all three methods return "iRacing adapter not yet implemented") and
  `cmd/import-iracing/` prints the message and exits non-zero. Kept — so the
  boundary test enforces adapter sibling-isolation from day one.
- `go build ./...` and `go test ./...` pass (`internal/core`, `internal/lmu`).
- Pushed and in sync: `main` == `origin/main` (0 ahead / 0 behind).

## What's in progress

Nothing active. Remaining work is queued in [`backlog.md`](backlog.md) — P1
(DerivePhase warning, `lapace-control` JSON-output flags), P2 (test-coverage
hardening, including the golden-output test), P3 (docs polish).

## What's blocked

- `lapace-control` subprocess integration depends on the `--json-preview` /
  `--json-result` CLI flags (P1 in `backlog.md`). Both sides of that change are
  separate slices.

## Done-enough criteria

Met — the first-cut criteria from the original plan are satisfied: repo layout,
LMU port + test port, core extraction, iRacing stub, test suite green, docs
accurate.

## Out of scope (still)

- iRacing adapter implementation (stub only).
- Structured-output CLI flags for `lapace-control` integration.
- New signal-family tables/columns.
- Multi-driver files, directory-based recordings, partial session groupings.
