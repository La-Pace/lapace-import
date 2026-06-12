package main

import (
	"fmt"
	"os"

	"github.com/La-Pace/lapace-import/internal/lmu"
)

func main() {
	adapter := lmu.NewAdapter()
	_ = adapter
	fmt.Fprintln(os.Stderr, "import-lmu: not yet implemented")
	os.Exit(1)
}
