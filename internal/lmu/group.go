package lmu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	contract "github.com/La-Pace/lapace-core/contract/session"
	"github.com/La-Pace/lapace-import/internal/core"
)

// GroupResult holds the result of a GroupAndImport call.
type GroupResult struct {
	EventID    contract.EventID
	Stints     []StintResult
	Duplicates []DuplicateFile
}

// StintResult holds the result of importing one stint.
type StintResult struct {
	StintID    contract.SessionStintID
	Phase      contract.Phase
	DBPath     string
	SourceFile string
	Checksum   string
	Stats      ImportStats
}

// DuplicateFile records a file skipped because its checksum matches another file.
type DuplicateFile struct {
	Path     string
	Checksum string
	Matches  string // path of the file it duplicates
}

// GroupAndImport imports one or more LMU files into v2 session layout.
// It groups files by Event+Phase, creating separate SessionStints for each file.
// Output follows: sessions/<eventID>/<phase>/stint-###.duckdb
func GroupAndImport(ctx context.Context, files []string, sessionsDir string, opts ...ImportOption) (*GroupResult, error) {
	cfg := defaultImportConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files to import")
	}

	// Read metadata from first file to determine EventID
	meta, err := readMetadata(files[0])
	if err != nil {
		return nil, err
	}

	eventID := contract.NewEventID(meta.RecordingTime, meta.TrackName)
	driverID := contract.NewDriverID(meta.DriverName)
	startedAt, err := ParseRecordingTimeAsTime(meta.RecordingTime)
	if err != nil {
		return nil, fmt.Errorf("parse RecordingTime %q: %w", meta.RecordingTime, err)
	}

	result := &GroupResult{
		EventID: eventID,
	}

	// Phase 1: Compute checksums, detect duplicates, classify by phase
	type fileEntry struct {
		path     string
		checksum string
		phase    contract.Phase
	}

	checksums := make(map[string]string) // checksum -> first file path with that checksum
	var entries []fileEntry

	// Track phase order for deterministic output
	phaseOrder := make(map[string]int)
	phaseCount := 0

	for _, filePath := range files {
		checksum, err := fileChecksum(filePath)
		if err != nil {
			return nil, fmt.Errorf("checksum %q: %w", filePath, err)
		}

		// Check for duplicates
		if existingPath, dup := checksums[checksum]; dup {
			result.Duplicates = append(result.Duplicates, DuplicateFile{
				Path:     filePath,
				Checksum: checksum,
				Matches:  existingPath,
			})
			continue
		}
		checksums[checksum] = filePath

		fileMeta, err := readMetadata(filePath)
		if err != nil {
			return nil, err
		}

		phase, _ := DerivePhase(fileMeta.SessionType)
		phaseSlug := phase.Slug()

		if _, seen := phaseOrder[phaseSlug]; !seen {
			phaseCount++
			phaseOrder[phaseSlug] = phaseCount
		}

		entries = append(entries, fileEntry{
			path:     filePath,
			checksum: checksum,
			phase:    phase,
		})
	}

	// Group entries by phase (preserving first-seen order)
	type phaseGroup struct {
		phase   contract.Phase
		entries []fileEntry
		seq     int
	}
	groups := make(map[string]*phaseGroup)
	for _, e := range entries {
		slug := e.phase.Slug()
		g, exists := groups[slug]
		if !exists {
			g = &phaseGroup{
				phase: e.phase,
				seq:   phaseOrder[slug],
			}
			groups[slug] = g
		}
		g.entries = append(g.entries, e)
	}

	// Sort phase slugs by their first-seen order for deterministic output
	sortedSlugs := make([]string, 0, len(groups))
	for slug := range groups {
		sortedSlugs = append(sortedSlugs, slug)
	}
	sort.Slice(sortedSlugs, func(i, j int) bool {
		return groups[sortedSlugs[i]].seq < groups[sortedSlugs[j]].seq
	})

	// Phase 2: Import each phase group
	var eventPhases []contract.PhaseManifest

	for _, slug := range sortedSlugs {
		group := groups[slug]
		var stintManifests []contract.SessionStintManifest

		for i, entry := range group.entries {
			stintSeq := i + 1
			stintID := contract.NewSessionStintID(stintSeq)

			// Create output directory
			phaseDir := filepath.Join(sessionsDir, eventID.String(), slug)
			if err := os.MkdirAll(phaseDir, 0755); err != nil {
				return nil, fmt.Errorf("create phase dir %q: %w", phaseDir, err)
			}

			dbPath := filepath.Join(phaseDir, stintID.String()+".duckdb")

			// Open source and write
			lmu, err := OpenLMUFile(entry.path)
			if err != nil {
				return nil, fmt.Errorf("open LMU file %q: %w", entry.path, err)
			}

			writer, err := core.NewWriter(dbPath)
			if err != nil {
				lmu.Close()
				return nil, fmt.Errorf("create output DB %q: %w", dbPath, err)
			}

			stats, err := ImportAll(ctx, lmu, writer)
			lmu.Close()
			if err != nil {
				writer.Close()
				return nil, fmt.Errorf("import %q: %w", entry.path, err)
			}
			writer.Close()

			result.Stints = append(result.Stints, StintResult{
				StintID:    stintID,
				Phase:      group.phase,
				DBPath:     dbPath,
				SourceFile: entry.path,
				Checksum:   entry.checksum,
				Stats:      *stats,
			})

			// Build stint manifest
			stintManifest := contract.SessionStintManifest{
				ID:             stintID,
				Sequence:       stintSeq,
				Status:         contract.StatusClosed,
				Source:         contract.SourceLMUImport,
				SourceFile:     entry.path,
				SourceChecksum: entry.checksum,
				StartReason:    contract.StintStartReasonOfficialFileStart,
				EndReason:      contract.StintEndReasonOfficialFileBoundary,
				DriverID:       driverID,
				DriverName:     meta.DriverName,
				StartedAt:      startedAt,
			}
			stintManifests = append(stintManifests, stintManifest)
		}

		// Build phase manifest
		phaseManifest := contract.PhaseManifest{
			Phase:     group.phase,
			Sequence:  group.seq,
			Status:    contract.StatusClosed,
			StartedAt: startedAt,
			EndReason: contract.PhaseEndReasonImportComplete,
			Stints:    stintManifests,
		}
		eventPhases = append(eventPhases, phaseManifest)
	}

	// Build and write event manifest
	eventDir := filepath.Join(sessionsDir, eventID.String())
	if err := os.MkdirAll(eventDir, 0755); err != nil {
		return nil, fmt.Errorf("create event dir %q: %w", eventDir, err)
	}

	trackID := contract.SlugNormalize(meta.TrackName)
	layoutID := contract.SlugNormalize(meta.TrackLayout)
	if meta.TrackLayout == "" {
		layoutID = ""
	}

	manifest := &contract.EventManifest{
		ID:           eventID,
		Status:       contract.StatusClosed,
		CloseReason:  contract.EventCloseReasonImportComplete,
		DriverID:     driverID,
		DriverName:   meta.DriverName,
		TrackID:      trackID,
		TrackName:    meta.TrackName,
		LayoutID:     layoutID,
		LayoutName:   meta.TrackLayout,
		VehicleName:  meta.CarName,
		VehicleClass: meta.CarClass,
		StartedAt:    startedAt,
		Phases:       eventPhases,
	}

	eventJSONPath := filepath.Join(eventDir, "event.json")
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal event.json: %w", err)
	}
	if err := os.WriteFile(eventJSONPath, append(manifestJSON, '\n'), 0644); err != nil {
		return nil, fmt.Errorf("write event.json: %w", err)
	}

	return result, nil
}

// readMetadata opens an LMU file, reads its metadata, and closes it.
func readMetadata(path string) (*LMUMetadata, error) {
	lmu, err := OpenLMUFile(path)
	if err != nil {
		return nil, fmt.Errorf("open LMU file %q: %w", path, err)
	}
	meta, err := lmu.Metadata()
	lmu.Close()
	if err != nil {
		return nil, fmt.Errorf("read metadata from %q: %w", path, err)
	}
	return meta, nil
}

// fileChecksum computes SHA-256 of a file.
func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %q: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
