# Status — `lapace-import`

**Repo status:** Planned
**Last updated:** 2026-06-11

## Current state

The `lapace-import` repo does not yet exist as a directory in the workspace. The directory tree at `lapace-import/` is staging for the new repo. Code will be bootstrapped from the legacy `LaPace/internal/importlmu/` and `LaPace/cmd/import-lmu/` (which is frozen per ADR-23).

## What's done

- Architecture decided: unified importer repo with per-sim adapter packages (ADR-0001).
- Boundary contract drafted: `docs/technical/adapter-contract.md` codifies what lives in `internal/core` vs each adapter, the `core.Adapter` interface, and the test strategy.
- User guide migrated: `docs/guides/lmu-import-guide.md` is the port of the legacy `lmu-import-guide.md`, updated for the new repo path and the C2 split.
- Workspace-level pointers staged: `lapace-docs/CONTEXT.md` pipeline diagram will include the offline import path; `lapace-docs/module-map/overview.md` will list `lapace-import` as a phase-1.5 module.

## What's in progress

Nothing yet — this is the planning slice. No code has been ported.

## What's blocked

- **Code bootstrap depends on `lapace-core` being consumable from a fresh repo.** It is, per the existing module map.
- **`lapace-control` integration depends on a structured-output CLI flag** (likely `--json-preview` and a JSON emit mode for `Group` / `Convert`). The importer side is a follow-up CLI change in this repo; the consumer side is a follow-up change in `lapace-control`.

## Done-enough criteria

For the first cut of `lapace-import`, "done enough" means:

1. Repo exists at `github.com/La-Pace/lapace-import` with the layout in ADR-0001.
2. `internal/lmu/` is a port of legacy `internal/importlmu/`, with `ReconstructTimestamps` extracted to `internal/core/`.
3. `internal/core/` contains the writer, the math, and the `core.Adapter` interface.
4. `cmd/import-lmu/main.go` is a thin CLI that wires `lmu.Adapter` to the legacy flags.
5. All existing LMU tests pass (`reader_test.go`, `mapping_test.go`, `converter_test.go`, `group_test.go`, `writer_test.go`, `preview_test.go`, `setup_adapter_test.go`, `import_test.go`, `integration_test.go`, `benchmark_test.go`).
6. The four-layer test strategy is in place: boundary import test, schema validation test, golden output test, per-adapter coverage tests.
7. `internal/iracing/` exists as a stub package that implements `core.Adapter` (possibly with `Preview` only; `Group` and `Convert` can be `panic("not yet implemented")` for the stub).
8. `cmd/import-iracing/main.go` exists and prints a clear "not yet implemented" message.
9. `docs/guides/lmu-import-guide.md` and `docs/technical/adapter-contract.md` are accurate against the ported code.

## Out of scope (for this slice)

- iRacing adapter implementation. Stub only.
- Structured-output CLI flags (JSON preview, JSON emit). Required for `lapace-control` integration but a separate slice.
- New signal-family tables or columns. Adapter contract is closed at the current schema.
- Multi-driver files, directory-based recordings, partial session groupings. Listed as open questions in `adapter-contract.md`; deferred until a sim forces them.
