package main

import (
	"fmt"
	"os"

	"github.com/La-Pace/lapace-import/internal/iracing"
)

func main() {
	adapter := iracing.NewAdapter()
	_ = adapter
	fmt.Fprintln(os.Stderr, "import-iracing: not yet implemented")
	os.Exit(1)
}
