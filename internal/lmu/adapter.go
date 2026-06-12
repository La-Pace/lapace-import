package lmu

import (
	"context"
	"errors"

	"github.com/La-Pace/lapace-import/internal/core"
)

// Adapter implements core.Adapter for LMU official DuckDB exports.
// Methods return "not yet implemented" until the LMU port lands.
type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) Preview(ctx context.Context, files []string) ([]core.PreviewEntry, error) {
	return nil, errors.New("LMU adapter not yet implemented")
}

func (a *Adapter) Convert(ctx context.Context, file string, emit core.EmitFunc) error {
	return errors.New("LMU adapter not yet implemented")
}

func (a *Adapter) Group(ctx context.Context, files []string, opts core.GroupOptions) (core.GroupResult, error) {
	return core.GroupResult{}, errors.New("LMU adapter not yet implemented")
}
