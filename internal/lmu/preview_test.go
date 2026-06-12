package lmu

import (
	"os"
	"path/filepath"
	"testing"

	contract "github.com/La-Pace/lapace-core/contract/session"
)

func TestPreviewFolderWithSessionsDirProposesOutputTreeWithoutWriting(t *testing.T) {
	source := linkSampleDuckDBForPreview(t, "Autodromo Nazionale Monza_R_*.duckdb")
	sessionsDir := t.TempDir()
	meta, err := readMetadata(source)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	preview, err := PreviewFolder(filepath.Dir(source), WithPreviewSessionsDir(sessionsDir))
	if err != nil {
		t.Fatalf("preview folder: %v", err)
	}

	if len(preview.Events) != 1 {
		t.Fatalf("events: got %#v, want one event", preview.Events)
	}
	if len(preview.ValidFiles) != 1 {
		t.Fatalf("valid files: got %#v, want one valid file", preview.ValidFiles)
	}
	validFile := preview.ValidFiles[0]
	if validFile.Path != source {
		t.Fatalf("valid file path: got %q, want %q", validFile.Path, source)
	}
	if validFile.Checksum == "" {
		t.Fatal("valid file checksum should be present")
	}
	if validFile.TrackName != meta.TrackName {
		t.Fatalf("valid file track name: got %q, want %q", validFile.TrackName, meta.TrackName)
	}
	if validFile.SessionType != meta.SessionType {
		t.Fatalf("valid file session type: got %q, want %q", validFile.SessionType, meta.SessionType)
	}
	if validFile.Phase != "race" {
		t.Fatalf("valid file phase: got %q, want race", validFile.Phase)
	}
	event := preview.Events[0]
	eventID := contract.NewEventID(meta.RecordingTime, meta.TrackName).String()
	eventDir := filepath.Join(sessionsDir, eventID)
	if validFile.EventID != eventID {
		t.Fatalf("valid file event id: got %q, want %q", validFile.EventID, eventID)
	}
	if event.EventID != eventID {
		t.Fatalf("event id: got %q, want %q", event.EventID, eventID)
	}
	if event.OutputDir != eventDir {
		t.Fatalf("event output dir: got %q, want %q", event.OutputDir, eventDir)
	}
	if event.EventManifestPath != filepath.Join(eventDir, "event.json") {
		t.Fatalf("event manifest path: got %q", event.EventManifestPath)
	}
	if len(event.Phases) != 1 {
		t.Fatalf("phases: got %#v, want one phase", event.Phases)
	}
	phase := event.Phases[0]
	if phase.Phase != "race" {
		t.Fatalf("phase: got %q, want race", phase.Phase)
	}
	if len(phase.Stints) != 1 {
		t.Fatalf("stints: got %#v, want one stint", phase.Stints)
	}
	stint := phase.Stints[0]
	if stint.StintID != contract.NewSessionStintID(1).String() {
		t.Fatalf("stint id: got %q", stint.StintID)
	}
	if stint.SourceFile != source {
		t.Fatalf("source file: got %q, want %q", stint.SourceFile, source)
	}
	if stint.OutputDBPath != filepath.Join(eventDir, "race", "stint-001.duckdb") {
		t.Fatalf("output db path: got %q", stint.OutputDBPath)
	}
	if stint.Checksum == "" {
		t.Fatal("stint checksum should be present")
	}
	if _, err := os.Stat(eventDir); !os.IsNotExist(err) {
		t.Fatalf("preview should not create event dir; stat err=%v", err)
	}
}

func TestPreviewFolderReportsExistingEventFolderConflict(t *testing.T) {
	source := linkSampleDuckDBForPreview(t, "Autodromo Nazionale Monza_R_*.duckdb")
	sessionsDir := t.TempDir()
	meta, err := readMetadata(source)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	eventID := contract.NewEventID(meta.RecordingTime, meta.TrackName).String()
	eventDir := filepath.Join(sessionsDir, eventID)
	if err := os.MkdirAll(eventDir, 0755); err != nil {
		t.Fatalf("create existing event dir: %v", err)
	}

	preview, err := PreviewFolder(filepath.Dir(source), WithPreviewSessionsDir(sessionsDir))
	if err != nil {
		t.Fatalf("preview folder: %v", err)
	}

	if len(preview.Conflicts) != 1 {
		t.Fatalf("conflicts: got %#v, want one conflict", preview.Conflicts)
	}
	conflict := preview.Conflicts[0]
	if conflict.Type != "existing_event_folder" {
		t.Fatalf("conflict type: got %q", conflict.Type)
	}
	if conflict.Path != eventDir {
		t.Fatalf("conflict path: got %q, want %q", conflict.Path, eventDir)
	}
	if len(preview.Events) != 1 || len(preview.Events[0].Conflicts) != 1 {
		t.Fatalf("event conflicts: got %#v", preview.Events)
	}
}

func linkSampleDuckDBForPreview(t *testing.T, pattern string) string {
	t.Helper()
	files := findSampleFilesByPattern(t, pattern)
	folder := t.TempDir()
	target := filepath.Join(folder, filepath.Base(files[0]))
	if err := os.Symlink(files[0], target); err != nil {
		t.Fatalf("symlink sample file: %v", err)
	}
	return target
}
