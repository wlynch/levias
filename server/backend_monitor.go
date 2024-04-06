package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/archive"
)

func (b *Backend) ContainerChanges(ctx context.Context, name string) ([]archive.Change, error) {
	return nil, ErrUnimplemented
}
func (b *Backend) ContainerInspect(ctx context.Context, name string, size bool, version string) (interface{}, error) {
	// Return synthetic container for buildkit - we'll spin this up on demand later.
	if strings.HasSuffix(name, ".buildx_buildkit_default") {
		return types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				Name: name,
				State: &types.ContainerState{
					Running: true,
				},
			},
		}, nil
	}
	return nil, errdefs.NotFound(fmt.Errorf("container %s not found", name))
}
func (b *Backend) ContainerLogs(ctx context.Context, name string, config *container.LogsOptions) (msgs <-chan *backend.LogMessage, tty bool, err error) {
	return nil, false, ErrUnimplemented
}
func (b *Backend) ContainerStats(ctx context.Context, name string, config *backend.ContainerStatsConfig) error {
	return ErrUnimplemented
}
func (b *Backend) ContainerTop(name string, psArgs string) (*container.ContainerTopOKBody, error) {
	return nil, ErrUnimplemented
}
func (b *Backend) Containers(ctx context.Context, config *container.ListOptions) ([]*types.Container, error) {
	return nil, ErrUnimplemented
}
