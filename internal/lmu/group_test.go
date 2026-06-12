package lmu

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/La-Pace/lapace-core/contract/session"
)

// findSampleDir returns the directory containing LMU DuckDB sample files.
// It searches relative to the test file, then falls back to the main checkout
// (for worktrees that may not have large binary sample data).
func findSampleDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	dir := filepath.Join(repoRoot, "sampledata", "lmu_duckdb")
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	// Fall back to main checkout (worktrees may not have untracked DuckDB files)
	fallback := filepath.Join("/Users/yuhangzhan/Codebase/lapace-workspace/Lapace", "sampledata", "lmu_duckdb")
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	t.Skipf("LMU sample data dir not found at %s or %s", dir, fallback)
	return ""
}

// findSampleFilesByPattern finds sample files matching a glob pattern.
func findSampleFilesByPattern(t *testing.T, pattern string) []string {
	t.Helper()
	dir := findSampleDir(t)
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatalf("glob pattern %q: %v", pattern, err)
	}
	if len(matches) == 0 {
		t.Skipf("no sample files matching %q in %s", pattern, dir)
	}
	return matches
}

// TDD Slice #1: Single file -> v2 layout
// Import one LMU file, verify it creates stint-001.duckdb + event.json
// in correct directory structure (sessions/<eventID>/<phase>/stint-001.duckdb)
func TestGroupAndImport_SingleFile_CreatesV2Layout(t *testing.T) {
	// Pick a small sample file
	files := findSampleFilesByPattern(t, "Autodromo Nazionale Monza_R_*.duckdb")
	if len(files) < 1 {
		t.Skip("need at least 1 Monza race sample file")
	}

	sessionsDir := t.TempDir()

	result, err := GroupAndImport(context.Background(), files[:1], sessionsDir)
	if err != nil {
		t.Fatalf("GroupAndImport error: %v", err)
	}

	// Verify result has one stint
	if len(result.Stints) != 1 {
		t.Fatalf("expected 1 stint, got %d", len(result.Stints))
	}
	stint := result.Stints[0]

	// Verify stint ID
	if stint.StintID != session.NewSessionStintID(1) {
		t.Errorf("StintID = %q, want %q", stint.StintID, session.NewSessionStintID(1))
	}

	// Verify phase is Race (Monza R file)
	if stint.Phase != session.PhaseRace {
		t.Errorf("Phase = %v, want PhaseRace", stint.Phase)
	}

	// Verify event ID is non-empty
	if result.EventID == "" {
		t.Error("EventID should not be empty")
	}

	// Verify DBPath follows v2 layout: sessions/<eventID>/<phase>/stint-001.duckdb
	expectedRelPath := result.EventID.String() + "/" + stint.Phase.Slug() + "/" + stint.StintID.String() + ".duckdb"
	expectedDBPath := filepath.Join(sessionsDir, expectedRelPath)
	if stint.DBPath != expectedDBPath {
		t.Errorf("DBPath = %q, want %q", stint.DBPath, expectedDBPath)
	}

	// Verify the DuckDB file actually exists
	if _, err := os.Stat(stint.DBPath); err != nil {
		t.Errorf("stint DuckDB file does not exist at %q: %v", stint.DBPath, err)
	}

	// Verify event.json exists in the event directory
	eventJSONPath := filepath.Join(sessionsDir, result.EventID.String(), "event.json")
	if _, err := os.Stat(eventJSONPath); err != nil {
		t.Fatalf("event.json does not exist at %q: %v", eventJSONPath, err)
	}

	// Verify event.json is valid JSON with correct structure
	data, err := os.ReadFile(eventJSONPath)
	if err != nil {
		t.Fatalf("read event.json: %v", err)
	}

	var manifest session.EventManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse event.json: %v", err)
	}

	if manifest.ID != result.EventID {
		t.Errorf("manifest.ID = %q, want %q", manifest.ID, result.EventID)
	}

	// Should have exactly one phase
	if len(manifest.Phases) != 1 {
		t.Fatalf("expected 1 phase in manifest, got %d", len(manifest.Phases))
	}

	phaseManifest := manifest.Phases[0]
	if phaseManifest.Phase != session.PhaseRace {
		t.Errorf("phase = %v, want PhaseRace", phaseManifest.Phase)
	}

	// Phase should have exactly one stint
	if len(phaseManifest.Stints) != 1 {
		t.Fatalf("expected 1 stint in phase, got %d", len(phaseManifest.Stints))
	}

	stintManifest := phaseManifest.Stints[0]
	if stintManifest.ID != session.NewSessionStintID(1) {
		t.Errorf("stint ID = %q, want %q", stintManifest.ID, session.NewSessionStintID(1))
	}

	// Verify StintPath returns the correct relative path
	expectedStintPath := result.EventID.String() + "/" + stint.Phase.Slug() + "/" + stint.StintID.String() + ".duckdb"
	actualStintPath := manifest.StintPath(stint.Phase.Slug(), stint.StintID)
	if actualStintPath != expectedStintPath {
		t.Errorf("StintPath = %q, want %q", actualStintPath, expectedStintPath)
	}

	t.Logf("EventID: %s", result.EventID)
	t.Logf("Stint:  ID=%s Phase=%s DBPath=%s", stint.StintID, stint.Phase.Slug(), stint.DBPath)
}

// TDD Slice #2: Manifest source provenance
// Verify event.json stint has source: "lmu_import", sourceFile, sourceChecksum,
// startReason: "official_file_start", endReason: "official_file_boundary"
func TestGroupAndImport_ManifestSourceProvenance(t *testing.T) {
	files := findSampleFilesByPattern(t, "Autodromo Nazionale Monza_R_*.duckdb")
	if len(files) < 1 {
		t.Skip("need at least 1 Monza race sample file")
	}

	sessionsDir := t.TempDir()

	result, err := GroupAndImport(context.Background(), files[:1], sessionsDir)
	if err != nil {
		t.Fatalf("GroupAndImport error: %v", err)
	}

	// Read and parse event.json
	eventJSONPath := filepath.Join(sessionsDir, result.EventID.String(), "event.json")
	data, err := os.ReadFile(eventJSONPath)
	if err != nil {
		t.Fatalf("read event.json: %v", err)
	}

	var manifest session.EventManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse event.json: %v", err)
	}

	if len(manifest.Phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(manifest.Phases))
	}
	if len(manifest.Phases[0].Stints) != 1 {
		t.Fatalf("expected 1 stint, got %d", len(manifest.Phases[0].Stints))
	}

	stintManifest := manifest.Phases[0].Stints[0]

	// Verify source is "lmu_import"
	if stintManifest.Source != session.SourceLMUImport {
		t.Errorf("Source = %q, want %q", stintManifest.Source, session.SourceLMUImport)
	}

	// Verify sourceFile is the original file path
	if stintManifest.SourceFile != files[0] {
		t.Errorf("SourceFile = %q, want %q", stintManifest.SourceFile, files[0])
	}

	// Verify sourceChecksum is a non-empty SHA-256 hex string (64 chars)
	if len(stintManifest.SourceChecksum) != 64 {
		t.Errorf("SourceChecksum length = %d, want 64 (SHA-256 hex)", len(stintManifest.SourceChecksum))
	}

	// Verify startReason is "official_file_start"
	if stintManifest.StartReason != session.StintStartReasonOfficialFileStart {
		t.Errorf("StartReason = %q, want %q", stintManifest.StartReason, session.StintStartReasonOfficialFileStart)
	}

	// Verify endReason is "official_file_boundary"
	if stintManifest.EndReason != session.StintEndReasonOfficialFileBoundary {
		t.Errorf("EndReason = %q, want %q", stintManifest.EndReason, session.StintEndReasonOfficialFileBoundary)
	}

	// Verify the StintResult also has correct checksum
	stint := result.Stints[0]
	if stint.Checksum != stintManifest.SourceChecksum {
		t.Errorf("StintResult.Checksum = %q, manifest.SourceChecksum = %q, should match",
			stint.Checksum, stintManifest.SourceChecksum)
	}

	t.Logf("Source: %s", stintManifest.Source)
	t.Logf("SourceFile: %s", stintManifest.SourceFile)
	t.Logf("SourceChecksum: %s...", stintManifest.SourceChecksum[:16])
	t.Logf("StartReason: %s", stintManifest.StartReason)
	t.Logf("EndReason: %s", stintManifest.EndReason)
}

// TDD Slice #3: Two same-phase files -> two SessionStints
// Import two LMU files with same track+driver+phase, verify two stints
// in same Event+Phase with correct stint-001 and stint-002 IDs.
func TestGroupAndImport_TwoSamePhaseFiles_TwoStints(t *testing.T) {
	dir := findSampleDir(t)
	files := []string{
		filepath.Join(dir, "Bahrain International Circuit_R_2026-05-27T00_42_42Z.duckdb"),
		filepath.Join(dir, "Bahrain International Circuit_R_2026-05-27T00_42_58Z.duckdb"),
	}
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			t.Skipf("Bahrain race sample file unavailable: %s", file)
		}
	}

	sessionsDir := t.TempDir()

	result, err := GroupAndImport(context.Background(), files[:2], sessionsDir)
	if err != nil {
		t.Fatalf("GroupAndImport error: %v", err)
	}

	// Should have 2 stints
	if len(result.Stints) != 2 {
		t.Fatalf("expected 2 stints, got %d", len(result.Stints))
	}

	// Both stints should be in the same event
	if result.EventID == "" {
		t.Fatal("EventID should not be empty")
	}

	// Verify stint IDs are sequential
	stint1 := result.Stints[0]
	stint2 := result.Stints[1]
	if stint1.StintID != session.NewSessionStintID(1) {
		t.Errorf("first stint ID = %q, want %q", stint1.StintID, session.NewSessionStintID(1))
	}
	if stint2.StintID != session.NewSessionStintID(2) {
		t.Errorf("second stint ID = %q, want %q", stint2.StintID, session.NewSessionStintID(2))
	}

	// Both should be race phase
	if stint1.Phase != session.PhaseRace {
		t.Errorf("first stint phase = %v, want PhaseRace", stint1.Phase)
	}
	if stint2.Phase != session.PhaseRace {
		t.Errorf("second stint phase = %v, want PhaseRace", stint2.Phase)
	}

	// Both DuckDB files should exist
	if _, err := os.Stat(stint1.DBPath); err != nil {
		t.Errorf("stint1 DuckDB does not exist at %q: %v", stint1.DBPath, err)
	}
	if _, err := os.Stat(stint2.DBPath); err != nil {
		t.Errorf("stint2 DuckDB does not exist at %q: %v", stint2.DBPath, err)
	}

	// Both should have different source files
	if stint1.SourceFile == stint2.SourceFile {
		t.Error("stints should have different source files")
	}

	// Verify event.json has one phase with two stints
	eventJSONPath := filepath.Join(sessionsDir, result.EventID.String(), "event.json")
	data, err := os.ReadFile(eventJSONPath)
	if err != nil {
		t.Fatalf("read event.json: %v", err)
	}

	var manifest session.EventManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse event.json: %v", err)
	}

	// One phase
	if len(manifest.Phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(manifest.Phases))
	}

	// Phase should have two stints
	if len(manifest.Phases[0].Stints) != 2 {
		t.Fatalf("expected 2 stints in phase, got %d", len(manifest.Phases[0].Stints))
	}

	sm1 := manifest.Phases[0].Stints[0]
	sm2 := manifest.Phases[0].Stints[1]
	if sm1.ID != session.NewSessionStintID(1) {
		t.Errorf("manifest stint1 ID = %q, want %q", sm1.ID, session.NewSessionStintID(1))
	}
	if sm2.ID != session.NewSessionStintID(2) {
		t.Errorf("manifest stint2 ID = %q, want %q", sm2.ID, session.NewSessionStintID(2))
	}

	// Each stint should have proper source provenance
	for i, sm := range manifest.Phases[0].Stints {
		if sm.Source != session.SourceLMUImport {
			t.Errorf("stint %d Source = %q, want lmu_import", i+1, sm.Source)
		}
		if sm.SourceFile == "" {
			t.Errorf("stint %d SourceFile should not be empty", i+1)
		}
		if sm.SourceChecksum == "" {
			t.Errorf("stint %d SourceChecksum should not be empty", i+1)
		}
		if sm.StartReason != session.StintStartReasonOfficialFileStart {
			t.Errorf("stint %d StartReason = %q, want official_file_start", i+1, sm.StartReason)
		}
		if sm.EndReason != session.StintEndReasonOfficialFileBoundary {
			t.Errorf("stint %d EndReason = %q, want official_file_boundary", i+1, sm.EndReason)
		}
	}

	t.Logf("EventID: %s", result.EventID)
	t.Logf("Stint1: ID=%s Phase=%s File=%s", stint1.StintID, stint1.Phase.Slug(), filepath.Base(stint1.SourceFile))
	t.Logf("Stint2: ID=%s Phase=%s File=%s", stint2.StintID, stint2.Phase.Slug(), filepath.Base(stint2.SourceFile))
}

// TDD Slice #4: Duplicate checksum detection
// Import two files where one is a duplicate (same checksum), verify it's
// reported in Duplicates and not imported.
func TestGroupAndImport_DuplicateChecksumDetection(t *testing.T) {
	files := findSampleFilesByPattern(t, "Autodromo Nazionale Monza_R_*.duckdb")
	if len(files) < 1 {
		t.Skip("need at least 1 Monza race sample file")
	}

	// Create a duplicate copy of the sample file
	tmpDir := t.TempDir()
	dupPath := filepath.Join(tmpDir, "duplicate.duckdb")
	srcData, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read source file: %v", err)
	}
	if err := os.WriteFile(dupPath, srcData, 0644); err != nil {
		t.Fatalf("write duplicate file: %v", err)
	}

	sessionsDir := t.TempDir()

	// Import both the original and the duplicate
	result, err := GroupAndImport(context.Background(), []string{files[0], dupPath}, sessionsDir)
	if err != nil {
		t.Fatalf("GroupAndImport error: %v", err)
	}

	// Should have only 1 stint (duplicate not imported)
	if len(result.Stints) != 1 {
		t.Fatalf("expected 1 stint (duplicate skipped), got %d", len(result.Stints))
	}

	// Should report 1 duplicate
	if len(result.Duplicates) != 1 {
		t.Fatalf("expected 1 duplicate, got %d", len(result.Duplicates))
	}

	dup := result.Duplicates[0]

	// Duplicate should reference the duplicate file path
	if dup.Path != dupPath {
		t.Errorf("duplicate Path = %q, want %q", dup.Path, dupPath)
	}

	// Duplicate should match the original file
	if dup.Matches != files[0] {
		t.Errorf("duplicate Matches = %q, want %q", dup.Matches, files[0])
	}

	// Checksum should be non-empty and match between original and duplicate
	if dup.Checksum == "" {
		t.Error("duplicate Checksum should not be empty")
	}
	if result.Stints[0].Checksum != dup.Checksum {
		t.Errorf("stint Checksum = %q, duplicate Checksum = %q, should match",
			result.Stints[0].Checksum, dup.Checksum)
	}

	t.Logf("Stints: %d (duplicate skipped)", len(result.Stints))
	t.Logf("Duplicates: %d", len(result.Duplicates))
	t.Logf("Duplicate: path=%q matches=%q", filepath.Base(dup.Path), filepath.Base(dup.Matches))
}

// TDD Slice #5: Two different-phase files
// Import two files with different phases (e.g. practice + race),
// verify two PhaseManifests in EventManifest.
func TestGroupAndImport_TwoDifferentPhaseFiles_TwoPhaseManifests(t *testing.T) {
	// Use Barcelona P (practice) + Bahrain R (race) — small files for fast test.
	// They have different tracks but the caller groups them into one event.
	practiceFiles := findSampleFilesByPattern(t, "Circuit de Barcelona_P_2026-04-07T01_41_46Z.duckdb")
	raceFiles := findSampleFilesByPattern(t, "Bahrain International Circuit_R_2026-05-27T00_42_42Z.duckdb")
	if len(practiceFiles) < 1 || len(raceFiles) < 1 {
		t.Skip("need Barcelona practice and Bahrain race sample files")
	}

	files := []string{practiceFiles[0], raceFiles[0]}
	sessionsDir := t.TempDir()

	result, err := GroupAndImport(context.Background(), files, sessionsDir)
	if err != nil {
		t.Fatalf("GroupAndImport error: %v", err)
	}

	// Should have 2 stints (one per phase)
	if len(result.Stints) != 2 {
		t.Fatalf("expected 2 stints, got %d", len(result.Stints))
	}

	// Stints should have different phases
	stint1 := result.Stints[0]
	stint2 := result.Stints[1]
	if stint1.Phase == stint2.Phase {
		t.Errorf("stints should have different phases, both are %v", stint1.Phase)
	}

	// Verify one is practice, one is race
	phases := map[session.Phase]bool{stint1.Phase: true, stint2.Phase: true}
	if !phases[session.PhasePractice] {
		t.Errorf("expected PhasePractice in results, got %v and %v", stint1.Phase, stint2.Phase)
	}
	if !phases[session.PhaseRace] {
		t.Errorf("expected PhaseRace in results, got %v and %v", stint1.Phase, stint2.Phase)
	}

	// Verify event.json has two phases
	eventJSONPath := filepath.Join(sessionsDir, result.EventID.String(), "event.json")
	data, err := os.ReadFile(eventJSONPath)
	if err != nil {
		t.Fatalf("read event.json: %v", err)
	}

	var manifest session.EventManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse event.json: %v", err)
	}

	if len(manifest.Phases) != 2 {
		t.Fatalf("expected 2 phases in manifest, got %d", len(manifest.Phases))
	}

	// Each phase should have exactly one stint
	for i, pm := range manifest.Phases {
		if len(pm.Stints) != 1 {
			t.Errorf("phase %d (%s): expected 1 stint, got %d", i+1, pm.Phase.Slug(), len(pm.Stints))
		}
	}

	// Verify phase slugs
	manifestPhases := map[session.Phase]bool{}
	for _, pm := range manifest.Phases {
		manifestPhases[pm.Phase] = true
	}
	if !manifestPhases[session.PhasePractice] {
		t.Error("manifest should include PhasePractice")
	}
	if !manifestPhases[session.PhaseRace] {
		t.Error("manifest should include PhaseRace")
	}

	// Verify both DuckDB files exist in separate phase directories
	for _, stint := range result.Stints {
		if _, err := os.Stat(stint.DBPath); err != nil {
			t.Errorf("stint DuckDB does not exist at %q: %v", stint.DBPath, err)
		}
	}

	t.Logf("EventID: %s", result.EventID)
	for _, stint := range result.Stints {
		t.Logf("Stint: ID=%s Phase=%s File=%s", stint.StintID, stint.Phase.Slug(), filepath.Base(stint.SourceFile))
	}
}
