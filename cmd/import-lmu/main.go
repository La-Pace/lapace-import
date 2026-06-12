// Command import-lmu converts official LMU DuckDB telemetry exports into
// Lapace-format v2 session files.
//
// LMU exports store telemetry in per-channel tables without timestamps on scalar
// data. This tool reconstructs timestamps from frequency metadata and writes
// each channel in Lapace's (ts, value) / (ts, value1-4) format.
//
// Supports grouping multiple LMU files for the same event into separate
// SessionStints with full provenance tracking.
//
// For the full import workflow, CLI reference, and troubleshooting, see
// docs/active/lmu-import-guide.md.
//
// Usage:
//
//	go run cmd/import-lmu/main.go --input=sample.duckdb --sessions-dir=./sessions
//	go run cmd/import-lmu/main.go --input=a.duckdb --input=b.duckdb --sessions-dir=./sessions --verbose
//	go run cmd/import-lmu/main.go --input=sample.duckdb --sessions-dir=./sessions --dry-run
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/La-Pace/lapace-import/internal/lmu"
)

func main() {
	// Accept multiple --input flags
	var inputFiles []string
	flag.Func("input", "Path to official LMU DuckDB export file (required, can be specified multiple times)", func(s string) error {
		inputFiles = append(inputFiles, s)
		return nil
	})
	sessionsDir := flag.String("sessions-dir", "", "Output sessions directory for v2 layout (default: ./sessions)")
	dryRun := flag.Bool("dry-run", false, "Read and convert but skip writing to output database")
	verbose := flag.Bool("verbose", false, "Print per-channel import details")
	flag.Parse()

	if len(inputFiles) == 0 {
		fmt.Fprintln(os.Stderr, "error: --input is required (specify one or more LMU DuckDB files)")
		os.Exit(1)
	}

	// Determine sessions directory
	outDir := *sessionsDir
	if outDir == "" {
		outDir = filepath.Join(".", "sessions")
	}

	// Expand glob patterns in input files
	var expandedFiles []string
	for _, pattern := range inputFiles {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Fatalf("Error expanding glob pattern %q: %v", pattern, err)
		}
		if len(matches) == 0 {
			// Not a glob, use as-is
			expandedFiles = append(expandedFiles, pattern)
		} else {
			expandedFiles = append(expandedFiles, matches...)
		}
	}

	if *verbose {
		fmt.Printf("Input files: %d\n", len(expandedFiles))
		for _, f := range expandedFiles {
			fmt.Printf("  %s\n", f)
		}
		fmt.Printf("Output: %s\n", outDir)
	}

	opts := []lmu.ImportOption{
		lmu.WithVerbose(*verbose),
	}

	if *dryRun {
		opts = append(opts, lmu.WithDryRun(true))
		fmt.Println("\n[dry-run] Would import to:", outDir)
	}

	result, err := lmu.GroupAndImport(context.Background(), expandedFiles, outDir, opts...)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Print results
	fmt.Printf("\nImport complete: Event %s\n", result.EventID)
	fmt.Printf("  Stints:     %d\n", len(result.Stints))
	for _, stint := range result.Stints {
		fmt.Printf("    %s  phase=%-12s  tables=%d  rows=%d  file=%s\n",
			stint.StintID, stint.Phase.Slug(),
			stint.Stats.TablesProcessed, stint.Stats.ScalarRows+stint.Stats.EventRows+stint.Stats.WheelRows+stint.Stats.EventWheelRows,
			filepath.Base(stint.SourceFile))
	}
	if len(result.Duplicates) > 0 {
		fmt.Printf("  Duplicates: %d (skipped)\n", len(result.Duplicates))
		for _, dup := range result.Duplicates {
			fmt.Printf("    %s  (matches %s)\n", filepath.Base(dup.Path), filepath.Base(dup.Matches))
		}
	}
}
