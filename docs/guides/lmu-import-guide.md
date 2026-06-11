# LMU Import Guide

Import official LMU DuckDB telemetry exports into LaPace's signal-family SessionStint storage.

The `lapace-import-lmu` tool converts LMU's per-channel DuckDB exports into LaPace's `source_samples` + signal-family format, groups files by event and phase, and writes stint databases following the v2 storage model (ADR-17, ADR-20).

For end-to-end workflow testing (folder picker, structured preview, conflict reporting, job history, job logs), drive LMU Import through `lapace-control` so the full pipeline is validated together. Use the CLI path below when isolating importer behavior or scripting a batch conversion.

---

## When to use this

- You have official LMU telemetry exports (`.duckdb` files created by LMU's built-in exporter) and want to analyze them with LaPace's coaching pipeline.
- You want to validate telemetry offline without running LMU or the live capture stack.
- You are migrating from LMU's native format into LaPace's session-oriented layout for post-session coaching.

This tool is an offline, one-time conversion. It does not integrate with the running receiver or coach daemon.

---

## CLI reference

```bash
go run ./cmd/import-lmu/main.go [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--input` | Yes | — | Path to an official LMU DuckDB export file. Repeat for multi-file import. Supports glob patterns. |
| `--sessions-dir` | No | `./sessions` | Output directory for v2 session layout. |
| `--dry-run` | No | `false` | Read and convert all inputs but skip writing output databases. Use to preview what would be imported. |
| `--verbose` | No | `false` | Print per-channel import details (table names, row counts, categories). |

### Examples

**Single file import:**

```bash
go run ./cmd/import-lmu/main.go \
  --input=~/Downloads/lmu_export_2026-05-31.duckdb \
  --sessions-dir=./sessions
```

**Multi-file import (same event, multiple phases):**

```bash
go run ./cmd/import-lmu/main.go \
  --input=~/Downloads/practice_2026-05-31.duckdb \
  --input=~/Downloads/qualifying_2026-05-31.duckdb \
  --input=~/Downloads/race_2026-05-31.duckdb \
  --sessions-dir=./sessions \
  --verbose
```

**Glob pattern (all exports in a directory):**

```bash
go run ./cmd/import-lmu/main.go \
  --input="./exports/*.duckdb" \
  --sessions-dir=./sessions
```

**Dry-run (preview without writing):**

```bash
go run ./cmd/import-lmu/main.go \
  --input=~/Downloads/lmu_export_2026-05-31.duckdb \
  --dry-run \
  --verbose
```

---

## How it works

### Step 1: Read and classify

For each input file, the importer:

1. Opens the LMU DuckDB in read-only mode.
2. Reads the `metadata` KV table for driver, track, vehicle, and session-type information.
3. Reads `channelsList` and `eventsList` for frequency and unit metadata.
4. Classifies each table into one of four categories based on column structure:

| Category | LMU columns | LaPace columns | Timestamp source |
|-----------|------------|----------------|-----------------|
| Scalar | `value` | `ts, value` | Reconstructed from frequency |
| Event | `ts, value` | `ts, value` | Preserved from source |
| Wheel | `value1–4` | `ts, value1–4` | Reconstructed from frequency |
| EventWheel | `ts, value1–4` | `ts, value1–4` | Preserved from source |

### Step 2: Timestamp reconstruction

LMU exports scalar and wheel data without timestamps. The importer reconstructs timestamps using:

```
ts = row_index / frequency + session_start_offset
```

Where:
- `frequency` comes from `channelsList` (events default to frequency 1)
- `session_start_offset` is derived from `RecordingTime` in metadata (LMU uses an underscore-quirky ISO 8601 string that the adapter normalizes first)

For the underlying math and the boundary with the shared core, see `docs/technical/adapter-contract.md`.

### Step 3: Group and deduplicate

When importing multiple files:

1. **Checksum detection** — each file's SHA-256 is computed. Files matching a previously seen checksum are recorded as duplicates and skipped.
2. **Phase grouping** — files are grouped by `SessionType` → LaPace phase:
   - `Practice` → `practice`
   - `Qualify` → `qualifying`
   - `Race` → `race`
   - (anything else) → `testing`
3. **Stint assignment** — within each phase, files are assigned sequential stint IDs (`stint-001`, `stint-002`, …) in recording-time order.

Unknown `SessionType` values are mapped to `testing` *and* recorded in the run summary as a warning — silent fallback is not the policy. See `docs/technical/adapter-contract.md` for the unknown-phase rule.

### Step 4: Write v2 layout

Each source file becomes exactly one stint database. The output follows the v2 path convention (ADR-17):

```text
sessions/
  <eventID>/
    event.json                          ← event manifest (provenance + lifecycle)
    practice/
      stint-001.duckdb                  ← one LMU export file = one stint DB
    qualifying/
      stint-001.duckdb
    race/
      stint-001.duckdb
```

The event manifest (`event.json`) records:
- Event identity (driver, track, vehicle, layout)
- Phase manifests with `status: closed` and `closeReason: import_complete`
- Stint manifests with `source: lmu_import`, `startReason: official_file_start`, `endReason: official_file_boundary`, plus source file checksum for provenance

### Step 5: Name aliasing

Approximately 30 LMU table names are mapped to LaPace-equivalent names (e.g., `TyresRimTemp` → `Rim Temp`, `Brakes Temp` → `Brake Temp`). Unmapped tables pass through to a longtail table (`channel_longtail_scalar`, `channel_longtail_wheel4`, or `channel_longtail_int`) with a reconstructed timestamp column. No data is dropped.

### Step 6: CarSetup preservation

The `CarSetup` metadata JSON (if present) is preserved as a raw VARCHAR in `session_metadata.car_setup`. The importer does not parse or normalize the JSON — that is deferred to future coaching features that consume it.

---

## Output format

Each stint database contains:

| Table | Description |
|-------|-------------|
| `lapace_version` | Version marker (`1.0.0-import-lmu`), `schema_type = "signal-family"` |
| `session_metadata` | Driver, track, vehicle, phase, recording time, frame count, duration |
| `source_samples` | Dense `sample_seq` spine with reconstructed clocks |
| `<signal-family>` tables | Per-domain rows (`driver_controls`, `vehicle_dynamics`, `tyre_state`, etc.) |
| `channel_profiles` | Per-channel cadence, storage family/column, mode, sample counts, source, quality |
| `channel_longtail_*` | Unmapped native channels preserved for future use |

The `session_id` in `session_metadata` uses the format `import-<RecordingTime>`.

---

## Duplicate detection

When multiple input files resolve to the same checksum (e.g., you accidentally pass the same file twice, or two files are byte-identical), the importer:

1. Skips the duplicate file — it is **not** written to the output.
2. Reports the duplicate in the summary output with the matching file's path.

```text
Import complete: Event 2026-05-31T142300Z_Spa
  Stints:     2
    stint-001  phase=practice    tables=42  rows=845230  file=practice_spa.duckdb
    stint-002  phase=qualifying  tables=42  rows=210450  file=qualifying_spa.duckdb
  Duplicates: 1 (skipped)
    practice_spa_copy.duckdb  (matches practice_spa.duckdb)
```

---

## Using imported sessions

After import, the resulting session directory is compatible with LaPace's coaching pipeline:

```bash
# Dry-run coaching analysis (no LLM call)
go run github.com/La-Pace/lapace-coaching/cmd/coach sessions/<eventID>/practice/stint-001.duckdb --dry-run

# Full coaching with LLM debrief
go run github.com/La-Pace/lapace-coaching/cmd/coach sessions/<eventID>/practice/stint-001.duckdb
```

Imported SessionStint DBs are indistinguishable from SHM-captured ones to the loader — same signal-family schema, same `session_metadata` shape, same channel profile layout.

---

## Troubleshooting

### "no such table: channelsList"

The input file is not a valid LMU DuckDB export. The importer expects LMU's per-channel format with `metadata`, `channelsList`, and `eventsList` tables. Verify the file was created by LMU's built-in exporter, not by a third-party tool.

### "error: --input is required"

You must specify at least one `--input` flag. Use `--input=path1 --input=path2` for multiple files.

### Glob patterns resolve to zero files

If using a glob pattern like `--input="./exports/*.duckdb"` and no files match, the literal pattern string is used as the path (which will then fail to open). Verify the glob resolves correctly by listing the directory first:

```bash
ls ./exports/*.duckdb
```

### Timestamps look wrong (year 1970 or far-future)

The importer derives session start offset from `RecordingTime` in LMU's metadata. If `RecordingTime` is missing or in an unexpected format, timestamps will be incorrect. Run with `--verbose` to see per-channel import details, and check the `session_metadata` table in the output.

### "Duplicate detected — skipped"

This is informational, not an error. The file you passed is byte-identical to a previously imported file. If this is unexpected, verify the file paths and check for accidental copies.

### Unknown session type warning

If a `SessionType` is not in the recognized set (`Practice`, `Qualify`, `Race`), the importer maps it to `testing` and surfaces a warning in the run summary. This is the documented policy — the LMU adapter does not silently invent new phase mappings. If you see this warning repeatedly, the LMU file may have a session type the adapter doesn't recognize, in which case the LMU adapter package needs a code update.

---

## See also

- `docs/technical/adapter-contract.md` — the contract this adapter implements
- `docs/adr/0001-adapter-separation.md` — why the importer is split this way
- `lapace-docs/architecture/adr/adr-16-lmu-official-export-import.md` — historical motivation (predates the C2 split; treat as background, not the current package-boundary authority)
- `lapace-docs/architecture/adr/adr-20-mixed-rate-telemetry.md` — the signal-family schema
- `sampledata/lmu_duckdb/` — official LMU export files for testing
