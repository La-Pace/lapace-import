# ADR-0001: Adapter Separation in `lapace-import`

**Status:** ACCEPTED
**Date:** 2026-06-11
**Related:** [lapace-docs ADR-16 (LMU official export import)](../../lapace-docs/architecture/adr/adr-16-lmu-official-export-import.md), [lapace-docs ADR-20 (mixed-rate telemetry)](../../lapace-docs/architecture/adr/adr-20-mixed-rate-telemetry.md), [lapace-docs ADR-23 (modular repo split)](../../lapace-docs/architecture/adr-23-modular-repo-split.md)

## Context

The legacy monorepo contained a single-purpose `internal/importlmu` package and a `cmd/import-lmu/main.go` CLI. ADR-16 mandated a standalone CLI for converting LMU's official DuckDB telemetry exports into LaPace's signal-family SessionStint storage. ADR-23 then froze the legacy monorepo and required every module to live in its own repo, with `lapace-core` as the only shared dependency.

When the workspace committed to a multi-sim roadmap (iRacing is the next sim, with more to follow), two facts emerged that the original ADR-16 layout could not accommodate:

1. **iRacing's IBT format is structurally unrelated to LMU's per-channel DuckDB export.** A reader that works for one will not work for the other. The two formats share only the *output* contract (LaPace's signal-family SessionStint DBs).
2. **ADR-23 already established that adapters are not shared Go packages** — every module owns its own reader, writer, and supporting code. This principle applies across sim importers as well.

Without a layout decision, the iRacing work would either (a) duplicate ~700 LOC of LMU pipeline code in a separate repo, (b) wedge iRacing's reader into the LMU package under a runtime check, or (c) collapse live and offline LMU concerns into `lapace-capture`, violating ADR-11's process-topology separation. All three outcomes are worse than a deliberate split.

## Decision

**The `lapace-import` repo is a unified importer. It contains one shared core package and one adapter package per sim. Each sim has its own thin CLI binary.**

### Layout

```
lapace-import/
├── internal/
│   ├── core/         # shared: writer, math, Adapter interface, supporting types
│   ├── lmu/          # LMU adapter (port of legacy internal/importlmu)
│   └── iracing/      # iRacing adapter (initial stub)
├── cmd/
│   ├── import-lmu/main.go      # thin CLI → lmu.Adapter
│   └── import-iracing/main.go  # thin CLI → iracing.Adapter
├── docs/
│   ├── guides/       # per-sim user guides
│   ├── technical/    # adapter-contract.md (the boundary)
│   └── adr/          # module-local ADRs
└── README.md
```

### Boundary

The shared core owns exactly three things:

1. The DuckDB writer and SessionStint write path.
2. Math that has no sim-specific input and identical semantics across sims (currently `ReconstructTimestamps`).
3. The `core.Adapter` interface and the supporting types adapters return (`PreviewEntry`, `GroupResult`, etc.).

Adapters own everything sim-specific: reader, channel mapping, phase derivation (including the default policy), sim-specific start-time parsing, stint grouping, unit normalization, sample-validity / missing-data policy, rate alignment, identity and provenance, and setup parsing.

The full contract — including what adapters must NOT do and the four-layer test strategy — is in `docs/technical/adapter-contract.md`.

### Why one binary per sim (not `--sim lmu` vs `--sim iracing`)

ADR-16's isolation argument applies here. Two binaries keep adapter code physically separate at the linker level, which is the strongest enforcement. A `--sim` flag would couple adapters at the binary level for negligible UX gain (the CLI surface is identical). `lapace-control`'s tool-registry model (`ToolImportLMU`, `ToolImportIRacing`, etc.) absorbs N binaries cleanly.

### Why a unified repo (not one repo per sim)

A unified repo shares the writer, the math, and the boundary tests. iRacing will reuse ~500 LOC of generic pipeline code from the LMU package. Per-sim repos would duplicate that, then duplicate it again for sim #4, and would require fixing any schema change in N places.

The shared layer is intentionally thin so it does not become a leaky abstraction. If a future sim genuinely needs shared logic that does not fit the three core categories, that is a new ADR.

## Consequences

### Positive

- iRacing work is an additive PR (`internal/iracing/` + `cmd/import-iracing/main.go`), not a fresh repo.
- The SessionStint write path is the only place canonical DDL lives; adapter code cannot accidentally couple to the live capture writer.
- Each adapter's `SessionType` vocabulary and default policy is local — iRacing's "Warmup → testing" doesn't pollute LMU's policy and vice versa.
- Adapter contract is enforceable: boundary import test, schema validation test, golden output tests, and per-adapter coverage tests.
- The `lapace-control` integration (currently direct Go import in legacy) becomes a clean CLI subprocess invocation per ADR-19's "structured preview" hint. The `lapace-control` consumer no longer reaches into the importer's Go types.

### Negative

- One more repo to bootstrap (`lapace-import` is the 10th module repo).
- The shared core is a place to overreach. Discipline is required to keep it at three responsibilities; the open-questions list in `adapter-contract.md` is the soft guardrail.
- iRacing's adapter ships as a stub initially; the first sim author who picks it up inherits a clean but empty package and the responsibility to add golden test coverage.
- The CLI integration with `lapace-control` requires structured output (likely JSON) on the importer side. This is a follow-up CLI change.

### Neutral

- The legacy `internal/importlmu` and `cmd/import-lmu` paths are superseded by `internal/lmu` and `cmd/import-lmu` (renamed package, same binary name). ADR-16 in `lapace-docs` is not rewritten; a forward pointer is added at its foot.
- `lapace-control`'s import-LMU endpoint currently calls `importlmu.GroupAndImport` etc. directly. The migration to a CLI subprocess invocation is a separate, scoped change in `lapace-control`.

## Alternatives considered

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| **Unified repo with adapter packages (chosen)** | iRacing is additive; shared math is shared; boundary is enforceable | One more repo; discipline required to keep core thin | ✅ Selected |
| One repo per sim (`lapace-import-lmu`, `lapace-import-iracing`) | Maximum isolation; per-sim versioning | Doubled migration tax; schema fixes in N places; no shared test scaffolding | ❌ Operational cost too high |
| Fold importer into `lapace-capture` | One repo for "all LMU data paths" | Couples live and offline; breaks ADR-11's process-topology separation; iRacing is not a "capture" concern | ❌ Live/offline boundary violation |
| Fold importer into `lapace-dev-tools` | Already a "tooling" bucket; no new repo | Dev-tools writes test fixtures, not production SessionStint DBs; iRacing would compound the misfit | ❌ Wrong responsibility bucket |
| Single binary with `--sim` flag | One binary to ship; smaller CI | Couples adapters at the linker level; defeats ADR-16's isolation; bloats the binary for negligible UX win | ❌ Rejected on principle |
| Keep the legacy flat package (`internal/importlmu`) and add `internal/importiracing` | No new ADR needed | Sibling packages with no shared code; re-implements the duplication problem at the package level | ❌ Worst of both worlds |

## See also

- `docs/technical/adapter-contract.md` — the full contract this ADR establishes
- `docs/guides/lmu-import-guide.md` — a complete user-facing example of an adapter
- `lapace-docs/architecture/adr/adr-16-lmu-official-export-import.md` — historical motivation (predates this ADR)
- `lapace-docs/architecture/adr/adr-20-mixed-rate-telemetry.md` — the signal-family schema the writer emits
- `lapace-docs/architecture/adr-23-modular-repo-split.md` — the modular repo split this ADR fits inside
