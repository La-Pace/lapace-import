package lmu

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	contract "github.com/La-Pace/lapace-core/contract/session"
)

type PreviewResult struct {
	SelectedFolder string            `json:"selectedFolder"`
	Files          []string          `json:"files"`
	ValidFiles     []ValidFile       `json:"validFiles,omitempty"`
	Duplicates     []DuplicateFile   `json:"duplicates,omitempty"`
	InvalidFiles   []InvalidFile     `json:"invalidFiles,omitempty"`
	Events         []PreviewEvent    `json:"events,omitempty"`
	Conflicts      []PreviewConflict `json:"conflicts,omitempty"`
}

type ValidFile struct {
	Path          string `json:"path"`
	Checksum      string `json:"checksum"`
	EventID       string `json:"eventID"`
	Phase         string `json:"phase"`
	TrackName     string `json:"trackName"`
	TrackLayout   string `json:"trackLayout,omitempty"`
	SessionType   string `json:"sessionType"`
	RecordingTime string `json:"recordingTime"`
}

type InvalidFile struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type PreviewEvent struct {
	EventID           string            `json:"eventID"`
	OutputDir         string            `json:"outputDir"`
	EventManifestPath string            `json:"eventManifestPath"`
	Phases            []PreviewPhase    `json:"phases"`
	Conflicts         []PreviewConflict `json:"conflicts,omitempty"`
}

type PreviewPhase struct {
	Phase     string         `json:"phase"`
	OutputDir string         `json:"outputDir"`
	Stints    []PreviewStint `json:"stints"`
}

type PreviewStint struct {
	StintID      string `json:"stintID"`
	SourceFile   string `json:"sourceFile"`
	Checksum     string `json:"checksum,omitempty"`
	OutputDBPath string `json:"outputDbPath"`
}

type PreviewConflict struct {
	Type    string `json:"type"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type previewConfig struct {
	sessionsDir string
}

type PreviewOption func(*previewConfig)

func WithPreviewSessionsDir(sessionsDir string) PreviewOption {
	return func(cfg *previewConfig) {
		cfg.sessionsDir = sessionsDir
	}
}

func PreviewFolder(folder string, opts ...PreviewOption) (*PreviewResult, error) {
	cfg := previewConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	files, err := scanTopLevelDuckDBFiles(folder)
	if err != nil {
		return nil, err
	}
	preview, err := PreviewFiles(files, opts...)
	if err != nil {
		return nil, err
	}
	preview.SelectedFolder = folder
	return preview, nil
}

func PreviewFiles(files []string, opts ...PreviewOption) (*PreviewResult, error) {
	cfg := previewConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	selectedFiles := append([]string(nil), files...)
	duplicates, err := duplicateDuckDBFiles(selectedFiles)
	if err != nil {
		return nil, err
	}
	invalidFiles := invalidDuckDBFiles(selectedFiles)
	validFiles, err := validDuckDBFiles(selectedFiles, invalidFiles, duplicates)
	if err != nil {
		return nil, err
	}
	preview := &PreviewResult{
		Files:        selectedFiles,
		ValidFiles:   validFiles,
		Duplicates:   duplicates,
		InvalidFiles: invalidFiles,
	}
	if cfg.sessionsDir != "" {
		events, conflicts, err := previewOutputTree(selectedFiles, invalidFiles, duplicates, cfg.sessionsDir)
		if err != nil {
			return nil, err
		}
		preview.Events = events
		preview.Conflicts = conflicts
	}
	return preview, nil
}

func validDuckDBFiles(files []string, invalidFiles []InvalidFile, duplicates []DuplicateFile) ([]ValidFile, error) {
	invalid := make(map[string]bool)
	for _, file := range invalidFiles {
		invalid[file.Path] = true
	}
	duplicate := make(map[string]bool)
	for _, file := range duplicates {
		duplicate[file.Path] = true
	}

	var valid []ValidFile
	for _, file := range files {
		if invalid[file] || duplicate[file] {
			continue
		}
		meta, err := readMetadata(file)
		if err != nil {
			return nil, err
		}
		checksum, err := fileChecksum(file)
		if err != nil {
			return nil, err
		}
		phase, _ := DerivePhase(meta.SessionType)
		valid = append(valid, ValidFile{
			Path:          file,
			Checksum:      checksum,
			EventID:       contract.NewEventID(meta.RecordingTime, meta.TrackName).String(),
			Phase:         phase.Slug(),
			TrackName:     meta.TrackName,
			TrackLayout:   meta.TrackLayout,
			SessionType:   meta.SessionType,
			RecordingTime: meta.RecordingTime,
		})
	}
	return valid, nil
}

func scanTopLevelDuckDBFiles(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(filepath.Ext(name), ".duckdb") {
			files = append(files, filepath.Join(folder, name))
		}
	}
	sort.Strings(files)
	return files, nil
}

func duplicateDuckDBFiles(files []string) ([]DuplicateFile, error) {
	seen := make(map[string]string)
	var duplicates []DuplicateFile
	for _, file := range files {
		checksum, err := fileChecksum(file)
		if err != nil {
			return nil, err
		}
		if firstPath, ok := seen[checksum]; ok {
			duplicates = append(duplicates, DuplicateFile{
				Path:     file,
				Checksum: checksum,
				Matches:  firstPath,
			})
			continue
		}
		seen[checksum] = file
	}
	return duplicates, nil
}

func invalidDuckDBFiles(files []string) []InvalidFile {
	var invalid []InvalidFile
	for _, file := range files {
		lmu, err := OpenLMUFile(file)
		if err != nil {
			invalid = append(invalid, InvalidFile{Path: file, Error: err.Error()})
			continue
		}
		if _, err := lmu.Metadata(); err != nil {
			invalid = append(invalid, InvalidFile{Path: file, Error: err.Error()})
		}
		lmu.Close()
	}
	return invalid
}

func previewOutputTree(files []string, invalidFiles []InvalidFile, duplicates []DuplicateFile, sessionsDir string) ([]PreviewEvent, []PreviewConflict, error) {
	invalid := make(map[string]bool)
	for _, file := range invalidFiles {
		invalid[file.Path] = true
	}
	duplicate := make(map[string]bool)
	for _, file := range duplicates {
		duplicate[file.Path] = true
	}

	type previewEventBuilder struct {
		event      PreviewEvent
		phaseOrder []string
		phases     map[string]*PreviewPhase
	}

	var eventOrder []string
	events := make(map[string]*previewEventBuilder)
	for _, file := range files {
		if invalid[file] || duplicate[file] {
			continue
		}

		meta, err := readMetadata(file)
		if err != nil {
			continue
		}
		checksum, err := fileChecksum(file)
		if err != nil {
			return nil, nil, err
		}

		eventID := contract.NewEventID(meta.RecordingTime, meta.TrackName).String()
		eventDir := filepath.Join(sessionsDir, eventID)
		builder, ok := events[eventID]
		if !ok {
			builder = &previewEventBuilder{
				event: PreviewEvent{
					EventID:           eventID,
					OutputDir:         eventDir,
					EventManifestPath: filepath.Join(eventDir, "event.json"),
				},
				phases: make(map[string]*PreviewPhase),
			}
			if _, err := os.Stat(eventDir); err == nil {
				builder.event.Conflicts = append(builder.event.Conflicts, PreviewConflict{
					Type:    "existing_event_folder",
					Path:    eventDir,
					Message: "event folder already exists; importing into an existing event is blocked for MVP",
				})
			} else if err != nil && !os.IsNotExist(err) {
				return nil, nil, err
			}
			events[eventID] = builder
			eventOrder = append(eventOrder, eventID)
		}

		sessionPhase, _ := DerivePhase(meta.SessionType)
		phaseSlug := sessionPhase.Slug()
		previewPhase, ok := builder.phases[phaseSlug]
		if !ok {
			previewPhase = &PreviewPhase{
				Phase:     phaseSlug,
				OutputDir: filepath.Join(eventDir, phaseSlug),
			}
			builder.phases[phaseSlug] = previewPhase
			builder.phaseOrder = append(builder.phaseOrder, phaseSlug)
		}
		stintID := contract.NewSessionStintID(len(previewPhase.Stints) + 1).String()
		previewPhase.Stints = append(previewPhase.Stints, PreviewStint{
			StintID:      stintID,
			SourceFile:   file,
			Checksum:     checksum,
			OutputDBPath: filepath.Join(previewPhase.OutputDir, stintID+".duckdb"),
		})
	}

	var result []PreviewEvent
	var conflicts []PreviewConflict
	for _, eventID := range eventOrder {
		builder := events[eventID]
		for _, phaseSlug := range builder.phaseOrder {
			builder.event.Phases = append(builder.event.Phases, *builder.phases[phaseSlug])
		}
		result = append(result, builder.event)
		conflicts = append(conflicts, builder.event.Conflicts...)
	}
	return result, conflicts, nil
}
