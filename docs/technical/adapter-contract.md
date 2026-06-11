# Adapter Contract

This document defines the boundary between `lapace-import/internal/core` and the per-sim adapter packages (`internal/lmu`, `internal/iracing`, future `internal/<sim>`). It is the contract future sim authors implement against.

If you are adding a new sim adapter, read this first. The matching `0001-adapter-separation.md` ADR records *why* the boundary is shaped this way; this doc records *what* the boundary is.

## Principle

**Shared core owns LaPace's vocabulary. Adapters own the translation from sim vocabulary to LaPace vocabulary. The boundary is *vocabulary*, not file format.**

A sim adapter may parse any native format it needs, but the only thing it exposes across the boundary is LaPace-shaped data and adapter-neutral metadata.

**"LaPace vocabulary"** means: canonical session and stint identifiers, session phase values, signal-family table and column names, setup sheet shape, schema version metadata, and contract-defined row types. It is owned by `lapace-core` and re-exported through `internal/core` only where the contract naturally lives there.

## What lives in `internal/core/`

The shared core has three responsibilities and no more:

1. **The DuckDB writer and SessionStint write path.** Core owns the canonical write into a SessionStint DuckDB file: opens the file, runs `schema.CreateSignalFamilyTablesSQL()`, sets `lapace_version.schema_type = "signal-family"`, appends rows, validates output schema before commit. Adapters call into the writer; they do not open DuckDB files themselves. This is the isolation boundary ADR-16 and ADR-20 require: adapter code cannot accidentally couple to the live capture pipeline's writer.

2. **Math that has no sim-specific input and identical semantics across sims.** Currently this is `ReconstructTimestamps(rowCount, frequencyHz, startEpochSeconds) []float64`. The formula `ts[i] = i / frequencyHz + startEpochSeconds` is the same math for LMU and iRacing. Anything that takes a sim-native type as input does not belong here.

3. **The `core.Adapter` interface and the supporting types adapters return** (`PreviewEntry`, `GroupResult`, etc., defined below). These are part of the contract, not part of the implementation.

That is the complete list. If you find yourself wanting to add a fourth category to core, stop and write an ADR — the boundary is intentional and growing it is the wrong default. Duplication between adapters is preferable to contaminating core.

`lapace-core` (the workspace-wide contract package) is the vocabulary owner. Adapters import `lapace-core` directly for type definitions. `internal/core` does not re-export vocabulary types; it provides the *write* and *math* shared by all adapters.

## What lives in an adapter package

Each adapter (`internal/lmu`, `internal/iracing`, …) is a self-contained translator. It owns:

- **The reader.** Parsing the sim's native file format (LMU's per-channel DuckDB tables, iRacing's IBT binary header + variable records).
- **The channel alias map.** The sim's native channel names mapped to LaPace's signal-family column names or to longtail categories.
- **The session-type → phase mapping, including the default policy.** Each sim's `SessionType` enum differs; the default fallback (what to do with unrecognized types) is a per-adapter policy decision. Unknown types must emit a warning in the run summary — silent fallback is not allowed.
- **The sim-specific start-time parser.** LMU's `RecordingTime` is an ISO 8601 string with underscore quirks that must be normalized before parsing. iRacing's `sessionStartDate` is a native `time_t`. These are different problems; the adapters own them.
- **The stint grouping logic.** How multi-file imports get divided into stints is sim-specific (LMU's grouping uses metadata + file ordering; iRacing's may use session-id + recording boundaries).
- **Unit normalization.** Converting from sim-native units (m/s vs km/h, °C vs °F, kPa vs bar, 0–1 vs 0–100 throttle) to LaPace's canonical units. The contract requires this; silent unit mismatches corrupt downstream coaching.
- **Sample validity / missing data policy.** How to handle absent channels, partial-file channels, sentinel values, NaN/Inf, paused sim samples, replay samples, garage/pit-menu samples, disconnected controls, corrupt rows. The adapter owns interpretation; core can validate shape.
- **Rate alignment / mixed-rate handling.** Mapping native channel sampling rates to canonical family rows. The adapter must not be forced to pretend all data shares one clock.
- **Identity and provenance.** Source file checksums, native driver/car/track/session identifiers, and any native session type that does not map cleanly to a `session.Phase` is preserved in provenance so a future consumer can recover the raw value.
- **The setup parser, if any.** LMU has a `CarSetup` JSON blob in metadata; iRacing has its own setup format. Adapters own these.
- **The package's `Adapter` implementation.** Concretely:

  ```go
  package lmu  // or iracing, etc.

  // Adapter implements core.Adapter for a sim.
  type Adapter struct {
      // sim-specific config
  }

  func (a *Adapter) Preview(ctx context.Context, files []string) ([]core.PreviewEntry, error)
  func (a *Adapter) Convert(ctx context.Context, file string, emit core.EmitFunc) error
  func (a *Adapter) Group(ctx context.Context, files []string, opts core.GroupOptions) (core.GroupResult, error)
  ```

## The `core.Adapter` interface

```go
package core

import (
    "context"
    "time"

    "github.com/La-Pace/lapace-core/contract/db/schema"
    "github.com/La-Pace/lapace-core/contract/session"
)

// PreviewEntry is the result of a dry-run preview.
// Preview may expose native metadata for human inspection only —
// preview output must not be consumed as canonical import data.
type PreviewEntry struct {
    Path              string             // source file path
    SimSessionType    string             // raw sim-native value, preserved for provenance
    StartTime         *time.Time         // wall-clock session start, when known
    SessionOffsetSec  *float64           // non-zero for partial / mid-session recordings
    DurationSeconds   *float64           // nil if not derivable without a full scan
    ChannelCount      int                // adapter-defined count
    NativeSummary     []string           // human-readable, not contract logic
    Issues            []PreviewIssue     // warnings, not silent fallbacks
}

type PreviewIssue struct {
    Severity IssueSeverity            // info | warning | error
    Kind     string                   // e.g. "unknown_session_type", "duplicate_checksum", "missing_recording_time"
    Detail   string
}

type IssueSeverity string

const (
    IssueInfo    IssueSeverity = "info"
    IssueWarning IssueSeverity = "warning"
    IssueError   IssueSeverity = "error"
)

// GroupOptions carries orchestration inputs.
// Dry-run and verbose flags live in the CLI, not here.
type GroupOptions struct {
    SessionsDir string
}

// GroupResult describes the proposed stint layout.
// Adapters return this without writing any output.
type GroupResult struct {
    EventID    string
    Stints     []StintSummary
    Duplicates []DuplicateEntry
}

type StintSummary struct {
    StintID string             // e.g. "stint-001"
    Phase   session.Phase      // already mapped to LaPace vocabulary
    Tables  int
    Rows    int
    Source  string             // source file path
    Issues  []PreviewIssue
}

type DuplicateEntry struct {
    Path     string
    Matches  string             // path of the file it duplicates
    Checksum string
}

// EmitFunc receives converted batches of signal-family rows.
// Returning a non-nil error aborts conversion.
// Adapters must emit batches in time order within each table.
type EmitFunc func(schema.SignalFamilyRows) error

// Adapter is the contract every sim adapter implements.
type Adapter interface {
    Preview(ctx context.Context, files []string) ([]PreviewEntry, error)
    Convert(ctx context.Context, file string, emit EmitFunc) error
    Group(ctx context.Context, files []string, opts GroupOptions) (GroupResult, error)
}
```

The interface is deliberately small. Anything that would force a sim-specific concept into the signature (e.g., a "channel" type that varies by sim) does not belong here — adapters return LaPace-shaped data, not sim-shaped data.

**Preview is the only place native metadata may surface**, and it is explicitly for human inspection. CLIs and `lapace-control` may display it; no downstream code may branch on it. If you find yourself wanting to drive workflow from `PreviewEntry.SimSessionType`, that's a signal the contract is wrong — promote the field to a `session.Phase` and route it through `Group()`.

### Future extension: optional `Grouper`

If a future sim genuinely needs to ship conversion without grouping (e.g., a sim whose files are always single-stint and grouping is trivially derived), the adapter can implement only `Preview` and `Convert`. CLIs that require grouping do a type assertion against an optional `Grouper` interface (TBD when first needed). Today, LMU and iRacing implement all three.

## Worked example: timestamp handling

LMU and iRacing both use a `row / frequency + start` timestamp model, but the *inputs* to that formula differ. The math is shared; the parsing is per-adapter.

**LMU adapter** (`internal/lmu/timestamps.go`):

```go
func (a *Adapter) sessionStartEpoch(recordingTime string) (float64, error) {
    // LMU quirk: "2026-05-03T21_23_18Z" uses underscores in the time portion.
    normalized := strings.ReplaceAll(recordingTime, "_", ":")
    normalized = strings.Replace(normalized, "T:", "T", 1)
    t, err := time.Parse(time.RFC3339, normalized)
    if err != nil {
        return 0, fmt.Errorf("parse RecordingTime %q: %w", recordingTime, err)
    }
    return float64(t.Unix()), nil
}

func (a *Adapter) reconstructTimestamps(rowCount, frequency int, recordingTime string) ([]float64, error) {
    start, err := a.sessionStartEpoch(recordingTime)
    if err != nil {
        return nil, err
    }
    return core.ReconstructTimestamps(rowCount, frequency, start), nil
}
```

**iRacing adapter** (`internal/iracing/timestamps.go`):

```go
func (a *Adapter) reconstructTimestamps(rowCount int, header ibt.Header) []float64 {
    // iRacing's sessionStartDate is already a native time_t. No string parsing.
    // sessionStartTime is non-zero for mid-session recordings; add it.
    return core.ReconstructTimestamps(
        rowCount,
        header.TickRate,
        float64(header.DiskSubHeader.SessionStartDate) + header.DiskSubHeader.SessionStartTime,
    )
}
```

Both adapters call the same `core.ReconstructTimestamps`. The sim-specific work — parsing the start time, adding `sessionStartTime` for partial recordings — stays in the adapter. The math stays in core.

## Worked example: phase derivation

LMU's `SessionType` vocabulary and iRacing's `SessionType` vocabulary are not 1:1. Each adapter owns its own mapping, its own default policy, *and* the obligation to surface a warning when the fallback is used.

```go
// internal/lmu/phase.go
func DerivePhase(sessionType string) (session.Phase, bool) {
    switch sessionType {
    case "Practice": return session.PhasePractice, true
    case "Qualify":  return session.PhaseQualifying, true
    case "Race":     return session.PhaseRace, true
    default:         return session.PhaseTesting, false  // false = "this was a fallback, surface a warning"
    }
}

// internal/iracing/phase.go
func DerivePhase(sessionType string) (session.Phase, bool) {
    switch sessionType {
    case "Practice":   return session.PhasePractice, true
    case "Qualify":    return session.PhaseQualifying, true
    case "Race":       return session.PhaseRace, true
    case "Time Trial": return session.PhaseTesting, false
    case "Warmup":     return session.PhaseTesting, false
    default:           return session.PhaseTesting, false
    }
}
```

The `bool` return is a *contract obligation*: the adapter must emit a `PreviewIssue{IssueWarning, "unknown_session_type", …}` whenever the fallback is used. Silently sending `Race 2` → testing is unacceptable for a coaching product.

If a future sim has a native session concept that cannot be faithfully represented by an existing `session.Phase` (e.g., "Sprint", "Heat", "Multi-class split"), the right move is to propose a new phase to `lapace-core` via a workspace ADR — not to wedge it into `testing`.

## What an adapter must NOT do

- **Open a DuckDB file directly.** Use the core writer. The writer is the only place the canonical DDL lives, and it validates output schema before commit.
- **Hand-build canonical DDL or run schema migrations from adapter code.** If a new table is needed, it is a `lapace-core` change.
- **Invent canonical column names.** Adapters map native aliases to existing canonical signal-family names. If a signal does not exist, it goes to the approved longtail tables (`channel_longtail_*`) or requires a contract change.
- **Silently drop recognized native channels.** Dropping is allowed only when explicit, tested, and documented in the adapter's user guide. New sim authors will otherwise ignore hard channels.
- **Resample or smooth data.** Import preserves source data. Derived transformations belong in `lapace-coaching`. The only exception is the rate alignment required by the signal-family schema (LOCF to the dense `sample_seq` spine) — that is the schema's job, not the adapter's.
- **Import another adapter.** `internal/lmu` and `internal/iracing` are siblings. If you find yourself wanting to share code, first ask whether it is truly sim-neutral canonical logic. If not, duplicate it or create an adapter-private helper package. Promotion to core requires the three-category rule or an ADR.
- **Mutate `lapace-core` casually.** Adding vocabulary to `lapace-core` requires a workspace-level ADR or explicit contract review, because it affects all modules.
- **Encode CLI behavior as correctness.** Adapter behavior must be testable as library code. CLI wiring should not contain mapping, grouping, or conversion logic.
- **Silently use a fallback policy.** Unknown `SessionType`, unrecognized channel, missing required metadata — all must surface as `PreviewIssue` entries, not silent defaults.

## Testing the boundary

The contract is enforceable, not just aspirational. The test strategy has four layers:

1. **Boundary import test** in `internal/core/import_boundary_test.go`: greps the `internal/<sim>` packages for forbidden imports (each other, `lapace-capture`, `lapace-receiver`, `lapace-coaching`, `lapace-dashboard`, `lapace-control`, and the DuckDB Go driver). Fails CI if violated. Mirrors the legacy monorepo's `internal/contract/session/boundary_test.go` pattern.

2. **Schema validation test** in `internal/core/writer_validation_test.go`: the core writer validates the output DuckDB before commit — expected tables exist, no extra canonical tables, `lapace_version.schema_type = "signal-family"` is set, schema version is compatible.

3. **Golden output tests** in each adapter's `*_test.go`: fixed native fixture input → expected canonical DuckDB output. Verify schema, metadata, row counts, phase, timestamps, units, and selected signal values. This is the most important enforcement — the contract is only as good as what the golden outputs actually check.

4. **Per-adapter coverage tests:**
   - **Alias mapping coverage**: every documented native channel maps to a canonical name; unknown native channels go to longtail explicitly; no canonical name typos; no duplicate ambiguous mappings unless intentionally supported.
   - **Unit normalization**: representative conversions for each unit class.
   - **Timestamp reconstruction**: zero/invalid frequency, non-zero recording offset, partial recording, row count, monotonicity, precision tolerance.
   - **Unknown phase policy**: known native values map to the correct phase *and* unknown values produce a `PreviewIssue{IssueWarning, "unknown_session_type", …}`.

If you add a new adapter, update the boundary test to whitelist the new adapter package (it should not appear as a forbidden import for itself, but its siblings should be forbidden).

## Adding a new sim

The process for adding a third sim:

1. Create `internal/<sim>/` with the adapter package, implementing `core.Adapter`.
2. Add the sim's reader, channel mapping, phase derivation, group logic, unit normalization, and rate alignment.
3. Add a CLI in `cmd/import-<sim>/main.go` that wires the adapter to flags. Model it on `cmd/import-lmu/main.go`.
4. Add the new binary to `lapace-control`'s tool registry as a supervised offline job.
5. Add native fixture data, ideally small enough to commit.
6. Add a golden output test against a known input.
7. Add the four coverage tests: alias mapping, unit normalization, timestamp behavior, phase mapping (including unknown values).
8. Update `docs/guides/<sim>-import-guide.md` (copy `lmu-import-guide.md` as a starting point, replace the CLI flags and the sim-specific quirks).
9. Update the boundary test in `internal/core/import_boundary_test.go` to whitelist the new adapter package.
10. Add a `STATUS.md` row for the new adapter and a `BACKLOG.md` entry for any deferred work.

If you find yourself needing a new canonical signal-family table or column, a new session phase, or any change to `lapace-core` — stop, that's a workspace ADR in `lapace-docs/architecture/adr/`, not a module-local decision. The checklist above is for "new adapter" only.

## Open questions (deferred)

These are intentionally not yet decided; document them when a future sim forces the issue:

- **Partial session groupings.** If a future sim has files that span multiple "sessions" in a single container, the `Adapter` interface may need a `DiscoverSessions(files) []SessionRef` method. Today LMU and iRacing are file-per-session.
- **Multi-driver files.** If a future sim records multiple drivers in one file (e.g., replay bundles), the `Convert` signature may need to emit per-driver `SignalFamilyRows`. Today both LMU and iRacing are single-driver per file.
- **Directory-based recordings.** If a future sim's "file" is actually a directory of partial files, the `files []string` parameter shape may need to change. Today both are file-based.
- **Setup sheet as a first-class output.** LMU's `CarSetup` is currently preserved as raw JSON in `session_metadata`. A future coaching feature may want a structured `SetupSheet` row. That is a `lapace-core` change and an adapter signature change.
- **Optional `Grouper` interface.** If a future sim needs to ship conversion without grouping, add `Grouper` as an optional interface and have CLIs type-assert.

## See also

- `0001-adapter-separation.md` — the ADR that established this boundary
- `docs/guides/lmu-import-guide.md` — a complete user-facing example of an adapter
- `lapace-docs/architecture/adr/adr-16-lmu-official-export-import.md` — historical motivation for the importer's existence. **Predates the modular `lapace-import` layout; treat as background, not the current package-boundary authority.**
- `lapace-docs/architecture/adr/adr-20-mixed-rate-telemetry.md` — the signal-family schema the writer emits
