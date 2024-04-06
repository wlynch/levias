package main

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

func (b *Backend) ContainersPrune(ctx context.Context, pruneFilters filters.Args) (*types.ContainersPruneReport, error) {
	return nil, ErrUnimplemented
}
