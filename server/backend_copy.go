package main

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
)

func (b *Backend) ContainerArchivePath(name string, path string) (content io.ReadCloser, stat *types.ContainerPathStat, err error) {
	return nil, nil, ErrUnimplemented
}

func (b *Backend) ContainerExport(ctx context.Context, name string, out io.Writer) error {
	return ErrUnimplemented
}

func (b *Backend) ContainerExtractToDir(name, path string, copyUIDGID, noOverwriteDirNonDir bool, content io.Reader) error {
	return ErrUnimplemented
}

func (b *Backend) ContainerStatPath(name string, path string) (stat *types.ContainerPathStat, err error) {
	return nil, ErrUnimplemented
}
