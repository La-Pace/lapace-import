package lmu

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/La-Pace/lapace-import/internal/core"
	schema "github.com/La-Pace/lapace-import/internal/schema"
	_ "github.com/duckdb/duckdb-go/v2"
)

func TestWriteLMUToLapaceDB(t *testing.T) {
	// Critical end-to-end test: open a real LMU DuckDB, convert it,
	// write to Lapace format, then verify the output is readable.
	path := findSampleFile(t)

	f, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer f.Close()

	meta, err := f.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error: %v", err)
	}

	phase, _ := DerivePhase(meta.SessionType)
	phaseSlug := phase.Slug()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, phaseSlug+".duckdb")

	writer, err := core.NewWriter(outputPath)
	if err != nil {
		t.Fatalf("core.NewWriter error: %v", err)
	}

	if _, err := ImportAll(context.Background(), f, writer); err != nil {
		t.Fatalf("ImportAll error: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Writer Close error: %v", err)
	}

	verifyOutputDB(t, outputPath, meta, phaseSlug)
}

func verifyOutputDB(t *testing.T, dbPath string, meta *LMUMetadata, phase string) {
	t.Helper()

	dsn := dbPath + "?access_mode=READ_ONLY"
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		t.Fatalf("open output DB: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Verify session_metadata
	var trackName, sessionType string
	err = db.QueryRowContext(ctx,
		`SELECT track_name, session_type FROM session_metadata LIMIT 1`,
	).Scan(&trackName, &sessionType)
	if err != nil {
		t.Fatalf("read session_metadata: %v", err)
	}
	if trackName != meta.TrackName {
		t.Errorf("session_metadata.track_name = %q, want %q", trackName, meta.TrackName)
	}
	if sessionType != phase {
		t.Errorf("session_metadata.session_type = %q, want %q", sessionType, phase)
	}

	for _, table := range []string{"source_samples", "driver_controls", "vehicle_dynamics", "powertrain"} {
		var count int
		if err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, table)).Scan(&count); err != nil {
			t.Fatalf("read %s: %v", table, err)
		}
		if count == 0 {
			t.Errorf("%s has 0 rows", table)
		}
	}

	var firstSessionTS float64
	err = db.QueryRowContext(ctx, `SELECT session_ts_raw FROM source_samples ORDER BY sample_seq LIMIT 1`).Scan(&firstSessionTS)
	if err != nil {
		t.Fatalf("read first source sample timestamp: %v", err)
	}
	if firstSessionTS < 0.0 || firstSessionTS > 1.0 {
		t.Errorf("first session_ts_raw = %f, expected session-relative (near 0.0)", firstSessionTS)
	}

	// Verify lapace_version table has canonical schema
	var schemaVersion string
	var schemaType string
	var dataVersion int
	err = db.QueryRowContext(ctx, `SELECT schema_version, data_version, schema_type FROM lapace_version LIMIT 1`).Scan(&schemaVersion, &dataVersion, &schemaType)
	if err != nil {
		t.Fatalf("read lapace_version: %v", err)
	}
	if schemaVersion == "" {
		t.Error("lapace_version.schema_version should not be empty")
	}
	if dataVersion <= 0 {
		t.Errorf("lapace_version.data_version = %d, want positive version", dataVersion)
	}
	if schemaType != schema.SignalFamilySchemaType {
		t.Errorf("lapace_version.schema_type = %q, want %q", schemaType, schema.SignalFamilySchemaType)
	}

	t.Logf("Output DB verified: track=%q type=%q firstTS=%f ver=%q",
		trackName, sessionType, firstSessionTS, schemaVersion)
}

func TestWriteSignalFamilyRowsWritesDent8LongtailAndTelemetryGaps(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "signal-family.duckdb")

	writer, err := core.NewWriter(outputPath)
	if err != nil {
		t.Fatalf("core.NewWriter error: %v", err)
	}

	rows := []schema.SignalFamilyRows{
		{
			SourceSamples: schema.SourceSamplesRow{
				SampleSeq:  0,
				CapturedAt: time.Unix(0, 0).UTC(),
				Source:     "test",
			},
			ChannelLongtailDent8: []schema.ChannelLongtailDent8Row{
				{
					ChannelName: "DentSeverity",
					SampleSeq:   0,
					Value:       [8]float64{0, 1, 2, 3, 4, 5, 6, 7},
				},
			},
			TelemetryGaps: []schema.TelemetryGapsRow{
				{
					GapID:          "gap-1",
					Source:         "test",
					MissingFromSeq: 10,
					MissingToSeq:   12,
					Reason:         "fixture",
					CreatedAt:      time.Unix(123, 0).UTC(),
				},
			},
		},
	}
	if err := writer.WriteSignalFamilyRows(context.Background(), rows); err != nil {
		t.Fatalf("WriteSignalFamilyRows error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Writer Close error: %v", err)
	}

	db, err := sql.Open("duckdb", outputPath+"?access_mode=READ_ONLY")
	if err != nil {
		t.Fatalf("open output DB: %v", err)
	}
	defer db.Close()

	var value8 float64
	if err := db.QueryRow(`SELECT value8 FROM channel_longtail_dent8 WHERE channel_name = 'DentSeverity' AND sample_seq = 0`).Scan(&value8); err != nil {
		t.Fatalf("read channel_longtail_dent8: %v", err)
	}
	if value8 != 7 {
		t.Fatalf("channel_longtail_dent8.value8 = %v, want 7", value8)
	}

	var missingFrom, missingTo int64
	var reason string
	if err := db.QueryRow(`SELECT missing_from_seq, missing_to_seq, reason FROM telemetry_gaps WHERE gap_id = 'gap-1'`).Scan(&missingFrom, &missingTo, &reason); err != nil {
		t.Fatalf("read telemetry_gaps: %v", err)
	}
	if missingFrom != 10 || missingTo != 12 || reason != "fixture" {
		t.Fatalf("telemetry_gaps = from %d to %d reason %q, want 10 to 12 fixture", missingFrom, missingTo, reason)
	}
}

// TDD Slice #46: Tyre Wear conversion — LMU exports 0-100 "remaining life %",
// contract expects 0.0-1.0 "wear fraction". The importer must convert before writing.
func TestConvertWearFraction(t *testing.T) {
	tests := []struct {
		name   string
		raw    WheelRow
		wantV1 float64
		wantV2 float64
		wantV3 float64
		wantV4 float64
	}{
		{"nearly new (98.8% remaining)", WheelRow{98.8, 98.8, 98.8, 98.8}, 0.012, 0.012, 0.012, 0.012},
		{"dead tyre (0% remaining)", WheelRow{0.0, 0.0, 0.0, 0.0}, 1.0, 1.0, 1.0, 1.0},
		{"brand new (100% remaining)", WheelRow{100.0, 100.0, 100.0, 100.0}, 0.0, 0.0, 0.0, 0.0},
		{"half worn", WheelRow{50.0, 50.0, 50.0, 50.0}, 0.5, 0.5, 0.5, 0.5},
		{"mixed per-wheel", WheelRow{100.0, 75.0, 25.0, 10.0}, 0.0, 0.25, 0.75, 0.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertWearFraction(tt.raw)
			if math.Abs(got.V1-tt.wantV1) > 0.001 {
				t.Errorf("V1 = %v, want %v", got.V1, tt.wantV1)
			}
			if math.Abs(got.V2-tt.wantV2) > 0.001 {
				t.Errorf("V2 = %v, want %v", got.V2, tt.wantV2)
			}
			if math.Abs(got.V3-tt.wantV3) > 0.001 {
				t.Errorf("V3 = %v, want %v", got.V3, tt.wantV3)
			}
			if math.Abs(got.V4-tt.wantV4) > 0.001 {
				t.Errorf("V4 = %v, want %v", got.V4, tt.wantV4)
			}
		})
	}
}
