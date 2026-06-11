# Backlog — `lapace-import`

**Last updated:** 2026-06-11

## P0 — bootstrap the repo

- [ ] Create `github.com/La-Pace/lapace-import` repo with the layout from ADR-0001.
- [ ] Set up `go.mod` with `lapace-core` as the only workspace dep.
- [ ] Copy the workspace's standard `AGENTS.md`, `CLAUDE.md`, CI config from an existing module repo (recommend `lapace-dev-tools` as a template — closest in shape).

## P0 — port the LMU adapter

- [ ] Port `LaPace/internal/importlmu/*.go` → `lapace-import/internal/lmu/`. File-by-file mapping:
  - `reader.go` → `internal/lmu/reader.go`
  - `mapping.go` → `internal/lmu/mapping.go`
  - `setup_adapter.go` → `internal/lmu/setup_adapter.go`
  - `group.go` → `internal/lmu/group.go`
  - `preview.go` → `internal/lmu/preview.go`
  - `import.go` → `internal/lmu/adapter.go` (top-level orchestration; satisfies `core.Adapter`)
  - `writer.go` → split: writer shell stays in `internal/core/writer.go`; sim-specific row construction moves to `internal/lmu/rows.go`
  - `converter.go` → split: `ReconstructTimestamps` to `internal/core/timestamps.go`; `DerivePhase` to `internal/lmu/phase.go`; `ParseRecordingTime` stays in `internal/lmu/timestamps.go`
- [ ] Update import paths from `github.com/user/lapace/internal/...` to `github.com/La-Pace/lapace-core/contract/...`.
- [ ] Port all `*_test.go` files and `integration_test.go` alongside the source files.
- [ ] Port `testdata/` and verify integration tests still pass against `sampledata/lmu_duckdb/`.

## P0 — extract shared core

- [ ] `internal/core/writer.go` — owns `LapaceWriter`, calls `schema.CreateSignalFamilyTablesSQL()`, sets `lapace_version.schema_type = "signal-family"`, validates output schema before commit.
- [ ] `internal/core/timestamps.go` — `ReconstructTimestamps(rowCount, frequencyHz, startEpochSeconds) []float64`. LMU and iRacing both call this.
- [ ] `internal/core/adapter.go` — the `Adapter` interface, `PreviewEntry`, `GroupResult`, `EmitFunc`, supporting types from `docs/technical/adapter-contract.md`.
- [ ] `internal/core/import_boundary_test.go` — greps adapter packages for forbidden imports (each other, `lapace-capture`, `lapace-receiver`, `lapace-coaching`, `lapace-dashboard`, `lapace-control`, DuckDB Go driver).
- [ ] `internal/core/writer_validation_test.go` — writer validates output DuckDB before commit.

## P1 — iRacing stub

- [ ] `internal/iracing/adapter.go` — stub implementing `core.Adapter`. `Preview` returns a hardcoded "not yet implemented" `PreviewIssue`. `Convert` and `Group` return `errors.New("not yet implemented")`.
- [ ] `cmd/import-iracing/main.go` — thin CLI matching `cmd/import-lmu/main.go`'s flag shape. Prints the "not yet implemented" message and exits non-zero.
- [ ] Decide: should the iRacing stub be removed from the first cut and added only when iRacing work starts? Argument for keeping it: makes the boundary test enforce "every adapter is sibling-isolated" from day one. Argument for removing: a stub that does nothing is a maintenance tax. **Decision needed from maintainer.**

## P1 — lapace-control integration (separate slice, but tracked here for context)

- [ ] Add `--json-preview` flag to `cmd/import-lmu/main.go` that emits `[]PreviewEntry` as JSON instead of the human-readable summary.
- [ ] Add `--json-result` flag that emits `GroupResult` as JSON after writing.
- [ ] Add a JSON streaming mode for `Convert` (one `SignalFamilyRows` per line, or a documented batch envelope).
- [ ] `lapace-control` consumer changes are tracked in `lapace-control`'s own backlog, not here.

## P2 — test coverage hardening

- [ ] Golden output test: pick a small `sampledata/lmu_duckdb/` fixture, capture the full output DuckDB bytes (or a canonicalized subset of tables), commit as `testdata/golden_lmu.duckdb`. Test reads input, runs adapter, compares output.
- [ ] Alias mapping coverage test: enumerate every entry in `internal/lmu/mapping.go`, assert each maps to either a canonical signal-family column or a longtail table. Same for iRacing when its mapping lands.
- [ ] Unit normalization tests: representative conversions for each unit class LMU uses. Same for iRacing when it lands.
- [ ] Timestamp reconstruction tests: zero frequency, negative frequency, non-zero recording offset, partial recording, monotonicity check, precision tolerance.
- [ ] Unknown phase policy test: assert that an unknown LMU `SessionType` produces a `PreviewEntry.Issues` entry with `Severity == IssueWarning` and `Kind == "unknown_session_type"`.
- [ ] Schema validation test: writer rejects output that is missing `lapace_version.schema_type = "signal-family"`, has extra canonical tables, or has wrong column types.

## P3 — documentation polish

- [ ] Cross-link `docs/guides/lmu-import-guide.md` and `docs/technical/adapter-contract.md` once both are stable. Currently the lmu guide references the contract doc; the contract doc references the lmu guide. Both directions are wired but a real `README.md` at the repo root with a docs index would be nicer.
- [ ] Add a `docs/guides/iracing-import-guide.md` skeleton (header + "not yet implemented" body) once the iRacing stub lands. This keeps the docs tree symmetrical and makes the "what's coming" obvious to a maintainer browsing the repo.
- [ ] Once the first sim author ports iRacing, update `adapter-contract.md`'s "Open questions" section to mark items as either "resolved" or "still open."

## Cleanup (not in this repo)

- [ ] Prune the stale worktree at `/Users/yuhangzhan/Codebase/lapace-workspace/LaPace/.claude/worktrees/lmu-import/` once the port is done. Branch `fix/lmu-import-coaching-compat` is already merged into main; the worktree is just noise. **Requires careful-mode confirmation before `git worktree remove`.**
- [ ] Archive `LaPace/docs/active/lmu-import-guide.md` after the port lands (it has been superseded by `lapace-import/docs/guides/lmu-import-guide.md`).
- [ ] Update `LaPace/cmd/control/main.go` and `LaPace/internal/control/state.go` to remove the direct `importlmu` Go imports once `lapace-control` switches to CLI subprocess invocation. (Tracked in `lapace-control`'s backlog, not here.)
