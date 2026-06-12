package core

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCoreImportsOnlyAllowedPackages(t *testing.T) {
	allowed := []string{
		"github.com/La-Pace/lapace-core/contract/",
		"github.com/La-Pace/lapace-import/internal/schema",
		"github.com/duckdb/duckdb-go/v2",
	}
	banned := []string{
		"/internal/lmu",
		"/internal/iracing",
		"lapace-receiver",
		"lapace-coaching",
		"lapace-dashboard",
		"lapace-capture",
		"lapace-control",
		"lapace-dev-tools",
		"lapace-voice",
		"github.com/user/lapace",
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", name, err)
			}

			if !strings.Contains(path, ".") {
				continue
			}

			allowedOk := false
			for _, allowedPath := range allowed {
				if strings.HasPrefix(path, allowedPath) {
					allowedOk = true
					break
				}
			}
			if !allowedOk {
				t.Fatalf("%s imports non-allowed package %q", name, path)
			}

			for _, bannedPath := range banned {
				if strings.Contains(path, bannedPath) {
					t.Fatalf("%s imports banned package %q", name, path)
				}
			}
		}
	}
}
