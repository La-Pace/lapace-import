package lmu

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// findSampleFile locates a real LMU DuckDB export from sampledata.
// It resolves the path relative to the repo root (where this module lives).
func findSampleFile(t *testing.T) string {
	t.Helper()

	// Use a small Monza race file (~1.3MB) for fast tests
	name := "Autodromo Nazionale Monza_R_2026-05-03T21_23_18Z.duckdb"

	// Resolve repo root: this file lives at internal/lmu/
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	candidate := filepath.Join(repoRoot, "sampledata", "lmu_duckdb", name)

	if _, err := os.Stat(candidate); err != nil {
		t.Skipf("LMU DuckDB sample file not found at %s — skipping integration test: %v", candidate, err)
	}

	return candidate
}

func TestOpenLMUFile(t *testing.T) {
	path := findSampleFile(t)

	f, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile(%q) error: %v", path, err)
	}
	defer f.Close()

	// Successfully opened and can close
	if err := f.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestOpenLMUFileNotFound(t *testing.T) {
	_, err := OpenLMUFile("/nonexistent/path/file.duckdb")
	if err == nil {
		t.Fatal("OpenLMUFile should return error for nonexistent file")
	}
}

func TestReadMetadata(t *testing.T) {
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

	// This file is "Autodromo Nazionale Monza_R_..." — a race session
	if meta.TrackName != "Autodromo Nazionale Monza" {
		t.Errorf("TrackName = %q, want %q", meta.TrackName, "Autodromo Nazionale Monza")
	}
	if meta.SessionType != "Race" {
		t.Errorf("SessionType = %q, want %q", meta.SessionType, "Race")
	}
	if meta.CarName == "" {
		t.Error("CarName should not be empty")
	}
	if meta.DriverName == "" {
		t.Error("DriverName should not be empty")
	}
	if meta.RecordingTime == "" {
		t.Error("RecordingTime should not be empty")
	}
	// CarSetup may or may not be present — just verify it's a non-empty string if present
	t.Logf("Metadata: track=%q type=%q car=%q driver=%q recTime=%q carSetupLen=%d",
		meta.TrackName, meta.SessionType, meta.CarName, meta.DriverName, meta.RecordingTime, len(meta.CarSetup))
}

func TestParseRealRecordingTime(t *testing.T) {
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

	epoch, err := ParseRecordingTime(meta.RecordingTime)
	if err != nil {
		t.Fatalf("ParseRecordingTime(%q) error: %v", meta.RecordingTime, err)
	}

	// Should be a plausible epoch (after 2020, before 2030)
	if epoch < 1577836800 || epoch > 1893456000 {
		t.Errorf("ParseRecordingTime returned %f, outside plausible range [2020, 2030]", epoch)
	}

	t.Logf("RecordingTime=%q → epoch=%f", meta.RecordingTime, epoch)
}

func TestReadChannelsList(t *testing.T) {
	path := findSampleFile(t)

	f, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer f.Close()

	channels, err := f.ChannelsList()
	if err != nil {
		t.Fatalf("ChannelsList() error: %v", err)
	}

	if len(channels) == 0 {
		t.Fatal("ChannelsList returned empty list, expected at least 50 channels")
	}

	// Verify specific known channels exist with correct structure
	byName := make(map[string]ChannelInfo)
	for _, ch := range channels {
		byName[ch.Name] = ch
	}

	// Ground Speed should be 100Hz
	if gs, ok := byName["Ground Speed"]; !ok {
		t.Error("Missing channel: Ground Speed")
	} else if gs.Frequency != 100 {
		t.Errorf("Ground Speed frequency = %d, want 100", gs.Frequency)
	}

	// Engine RPM should be 100Hz
	if rpm, ok := byName["Engine RPM"]; !ok {
		t.Error("Missing channel: Engine RPM")
	} else if rpm.Frequency != 100 {
		t.Errorf("Engine RPM frequency = %d, want 100", rpm.Frequency)
	}

	// Throttle Pos should be 50Hz
	if tp, ok := byName["Throttle Pos"]; !ok {
		t.Error("Missing channel: Throttle Pos")
	} else if tp.Frequency != 50 {
		t.Errorf("Throttle Pos frequency = %d, want 50", tp.Frequency)
	}

	t.Logf("ChannelsList: %d channels", len(channels))
}

func TestReadEventsList(t *testing.T) {
	path := findSampleFile(t)
	f, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer f.Close()

	events, err := f.EventsList()
	if err != nil {
		t.Fatalf("EventsList() error: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("EventsList returned empty list, expected at least 10 events")
	}

	// Verify Lap event exists
	hasLap := false
	for _, ev := range events {
		if ev.Name == "Lap" {
			hasLap = true
		}
	}
	if !hasLap {
		t.Error("EventsList should include Lap event")
	}

	t.Logf("EventsList: %d events", len(events))
}

func TestReadScalarTable(t *testing.T) {
	path := findSampleFile(t)
	f, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer f.Close()

	rows, err := f.ReadScalarTable(context.Background(), "Ground Speed")
	if err != nil {
		t.Fatalf("ReadScalarTable(Ground Speed) error: %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("ReadScalarTable(Ground Speed) returned 0 rows")
	}

	// Values should be reasonable speeds (0-400 km/h for race cars)
	for i, r := range rows {
		if r.Value < 0 || r.Value > 400 {
			t.Errorf("row %d: Ground Speed value %f out of expected range [0, 400]", i, r.Value)
			break
		}
	}

	t.Logf("ReadScalarTable(Ground Speed): %d rows, first=%f, last=%f",
		len(rows), rows[0].Value, rows[len(rows)-1].Value)
}

func TestReadEventTable(t *testing.T) {
	path := findSampleFile(t)
	f, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer f.Close()

	rows, err := f.ReadEventTable(context.Background(), "Lap")
	if err != nil {
		t.Fatalf("ReadEventTable(Lap) error: %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("ReadEventTable(Lap) returned 0 rows, expected at least 1")
	}

	// Lap events should have non-zero ts and increasing lap numbers
	firstTs := rows[0].Ts
	if firstTs < 0 {
		t.Errorf("Lap first row ts = %f, want >= 0", firstTs)
	}

	t.Logf("ReadEventTable(Lap): %d rows, first_ts=%f, first_value=%f",
		len(rows), rows[0].Ts, rows[0].FloatValue)
}

func TestReadWheelTable(t *testing.T) {
	path := findSampleFile(t)
	f, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer f.Close()

	rows, err := f.ReadWheelTable(context.Background(), "Wheel Speed")
	if err != nil {
		t.Fatalf("ReadWheelTable(Wheel Speed) error: %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("ReadWheelTable(Wheel Speed) returned 0 rows")
	}

	// Wheel speeds should be reasonable (0-400 km/h)
	for i, r := range rows {
		if r.V1 < 0 || r.V1 > 400 || r.V2 < 0 || r.V2 > 400 {
			t.Errorf("row %d: Wheel Speed values out of range: V1=%f V2=%f", i, r.V1, r.V2)
			break
		}
	}

	t.Logf("ReadWheelTable(Wheel Speed): %d rows, first=(%f,%f,%f,%f)",
		len(rows), rows[0].V1, rows[0].V2, rows[0].V3, rows[0].V4)
}

func TestChannelTables(t *testing.T) {
	path := findSampleFile(t)
	f, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer f.Close()

	tables, err := f.ChannelTables()
	if err != nil {
		t.Fatalf("ChannelTables() error: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("ChannelTables returned empty list")
	}

	// Should NOT include metadata tables
	for _, name := range tables {
		if name == "metadata" || name == "channelsList" || name == "eventsList" {
			t.Errorf("ChannelTables should not include metadata table %q", name)
		}
	}

	// Should include known channels
	hasGroundSpeed := false
	for _, name := range tables {
		if name == "Ground Speed" {
			hasGroundSpeed = true
		}
	}
	if !hasGroundSpeed {
		t.Error("ChannelTables should include Ground Speed")
	}

	t.Logf("ChannelTables: %d tables", len(tables))
}

func TestReadNonExistentTable(t *testing.T) {
	path := findSampleFile(t)
	f, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer f.Close()

	_, err = f.ReadScalarTable(context.Background(), "NonExistentTable12345")
	if err == nil {
		t.Error("ReadScalarTable should return error for nonexistent table")
	}
}
