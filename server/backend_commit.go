package main

import (
	"context"

	"github.com/docker/docker/api/types/backend"
)

func (b *Backend) CreateImageFromContainer(ctx context.Context, name string, config *backend.CreateImageConfig) (imageID string, err error) {
	return "", ErrUnimplemented
}
