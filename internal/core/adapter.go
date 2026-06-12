package core

import (
	"context"
	"time"

	"github.com/La-Pace/lapace-core/contract/session"
	"github.com/La-Pace/lapace-import/internal/schema"
)

// Adapter is the contract every sim adapter implements.
type Adapter interface {
	Preview(ctx context.Context, files []string) ([]PreviewEntry, error)
	Convert(ctx context.Context, file string, emit EmitFunc) error
	Group(ctx context.Context, files []string, opts GroupOptions) (GroupResult, error)
}

// EmitFunc receives converted batches of signal-family rows.
// Returning a non-nil error aborts conversion.
// Adapters must emit batches in time order within each table.
type EmitFunc func(schema.SignalFamilyRows) error

type PreviewEntry struct {
	Path             string
	SimSessionType   string
	StartTime        *time.Time
	SessionOffsetSec *float64
	DurationSeconds  *float64
	ChannelCount     int
	NativeSummary    []string
	Issues           []PreviewIssue
}

type PreviewIssue struct {
	Severity IssueSeverity
	Kind     string
	Detail   string
}

type IssueSeverity string

const (
	IssueInfo    IssueSeverity = "info"
	IssueWarning IssueSeverity = "warning"
	IssueError   IssueSeverity = "error"
)

type GroupOptions struct {
	SessionsDir string
}

type GroupResult struct {
	EventID    string
	Stints     []StintSummary
	Duplicates []DuplicateEntry
}

type StintSummary struct {
	StintID string
	Phase   session.Phase
	Tables  int
	Rows    int
	Source  string
	Issues  []PreviewIssue
}

type DuplicateEntry struct {
	Path     string
	Matches  string
	Checksum string
}
