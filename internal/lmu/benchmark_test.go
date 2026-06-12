package lmu

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/La-Pace/lapace-import/internal/core"
)

func BenchmarkImportAll(b *testing.B) {
	path := findSampleFileB(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		lmu, err := OpenLMUFile(path)
		if err != nil {
			b.Fatalf("OpenLMUFile: %v", err)
		}

		tmpDir := b.TempDir()
		outputPath := filepath.Join(tmpDir, "bench.duckdb")
		writer, err := core.NewWriter(outputPath)
		if err != nil {
			lmu.Close()
			b.Fatalf("core.NewWriter: %v", err)
		}
		b.StartTimer()

		stats, err := ImportAll(context.Background(), lmu, writer)

		b.StopTimer()
		lmu.Close()
		writer.Close()
		if err != nil {
			b.Fatalf("ImportAll: %v", err)
		}

		totalRows := stats.ScalarRows + stats.EventRows + stats.WheelRows + stats.EventWheelRows
		b.ReportMetric(float64(totalRows)/b.Elapsed().Seconds(), "rows/sec")
		b.StartTimer()
	}
}

// findSampleFileB locates a real LMU DuckDB export from sampledata.
func findSampleFileB(b *testing.B) string {
	b.Helper()
	name := "Autodromo Nazionale Monza_R_2026-05-03T21_23_18Z.duckdb"
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	candidate := filepath.Join(repoRoot, "sampledata", "lmu_duckdb", name)
	if _, err := os.Stat(candidate); err != nil {
		b.Skipf("LMU DuckDB sample file not found at %s — skipping benchmark: %v", candidate, err)
	}
	return candidate
}
