package lmu

import (
	"context"
	"fmt"

	contract "github.com/La-Pace/lapace-core/contract/session"
	"github.com/La-Pace/lapace-import/internal/core"
)

// Adapter implements core.Adapter for LMU official DuckDB exports.
type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) Preview(ctx context.Context, files []string) ([]core.PreviewEntry, error) {
	entries := make([]core.PreviewEntry, 0, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		entry := core.PreviewEntry{Path: file}
		lmuFile, err := OpenLMUFile(file)
		if err != nil {
			entry.Issues = append(entry.Issues, core.PreviewIssue{Severity: core.IssueError, Kind: "open_failed", Detail: err.Error()})
			entries = append(entries, entry)
			continue
		}

		meta, err := lmuFile.Metadata()
		if err != nil {
			_ = lmuFile.Close()
			entry.Issues = append(entry.Issues, core.PreviewIssue{Severity: core.IssueError, Kind: "metadata_failed", Detail: err.Error()})
			entries = append(entries, entry)
			continue
		}
		entry.SimSessionType = meta.SessionType
		entry.NativeSummary = []string{
			fmt.Sprintf("track=%s", meta.TrackName),
			fmt.Sprintf("layout=%s", meta.TrackLayout),
			fmt.Sprintf("car=%s", meta.CarName),
			fmt.Sprintf("driver=%s", meta.DriverName),
		}

		if startedAt, err := ParseRecordingTimeAsTime(meta.RecordingTime); err == nil {
			entry.StartTime = &startedAt
		} else {
			entry.Issues = append(entry.Issues, core.PreviewIssue{Severity: core.IssueWarning, Kind: "missing_recording_time", Detail: err.Error()})
		}
		if _, exact := DerivePhase(meta.SessionType); !exact {
			entry.Issues = append(entry.Issues, core.PreviewIssue{Severity: core.IssueWarning, Kind: "unknown_session_type", Detail: meta.SessionType})
		}

		channels, channelErr := lmuFile.ChannelsList()
		events, eventErr := lmuFile.EventsList()
		if channelErr != nil {
			entry.Issues = append(entry.Issues, core.PreviewIssue{Severity: core.IssueWarning, Kind: "channels_failed", Detail: channelErr.Error()})
		}
		if eventErr != nil {
			entry.Issues = append(entry.Issues, core.PreviewIssue{Severity: core.IssueWarning, Kind: "events_failed", Detail: eventErr.Error()})
		}
		entry.ChannelCount = len(channels) + len(events)

		if err := lmuFile.Close(); err != nil {
			entry.Issues = append(entry.Issues, core.PreviewIssue{Severity: core.IssueWarning, Kind: "close_failed", Detail: err.Error()})
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (a *Adapter) Convert(ctx context.Context, file string, emit core.EmitFunc) error {
	if emit == nil {
		return fmt.Errorf("emit function is required")
	}
	lmuFile, err := OpenLMUFile(file)
	if err != nil {
		return err
	}
	defer lmuFile.Close()

	cfg := defaultImportConfig()
	meta, data, _, err := readImportedTelemetry(ctx, lmuFile, cfg)
	if err != nil {
		return err
	}
	rows, err := buildSignalFamilyRows(meta, data)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := emit(row); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) Group(ctx context.Context, files []string, opts core.GroupOptions) (core.GroupResult, error) {
	if len(files) == 0 {
		return core.GroupResult{}, fmt.Errorf("no files to group")
	}

	result := core.GroupResult{}
	checksums := make(map[string]string)
	phaseCounts := make(map[string]int)

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return core.GroupResult{}, err
		}

		checksum, err := fileChecksum(file)
		if err != nil {
			return core.GroupResult{}, fmt.Errorf("checksum %q: %w", file, err)
		}
		if existing, ok := checksums[checksum]; ok {
			result.Duplicates = append(result.Duplicates, core.DuplicateEntry{Path: file, Matches: existing, Checksum: checksum})
			continue
		}
		checksums[checksum] = file

		meta, err := readMetadata(file)
		if err != nil {
			return core.GroupResult{}, err
		}
		if result.EventID == "" {
			result.EventID = contract.NewEventID(meta.RecordingTime, meta.TrackName).String()
		}

		phase, exact := DerivePhase(meta.SessionType)
		phaseCounts[phase.Slug()]++
		stintID := contract.NewSessionStintID(phaseCounts[phase.Slug()]).String()
		summary := core.StintSummary{
			StintID: stintID,
			Phase:   phase,
			Source:  file,
			Issues:  nil,
		}
		if !exact {
			summary.Issues = append(summary.Issues, core.PreviewIssue{Severity: core.IssueWarning, Kind: "unknown_session_type", Detail: meta.SessionType})
		}

		lmuFile, err := OpenLMUFile(file)
		if err != nil {
			return core.GroupResult{}, err
		}
		stats, importErr := ImportAll(ctx, lmuFile, nil, WithDryRun(true))
		closeErr := lmuFile.Close()
		if importErr != nil {
			return core.GroupResult{}, importErr
		}
		if closeErr != nil {
			return core.GroupResult{}, closeErr
		}
		summary.Tables = stats.TablesProcessed
		summary.Rows = stats.ScalarRows + stats.EventRows + stats.WheelRows + stats.EventWheelRows
		result.Stints = append(result.Stints, summary)
	}
	return result, nil
}
