package main

import (
	"context"
	"errors"
	"io"

	"github.com/distribution/reference"
	imagerouter "github.com/docker/docker/api/server/router/image"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	dockerimage "github.com/docker/docker/image"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	_ imagerouter.Backend = &Backend{}
)

func (b *Backend) ExportImage(ctx context.Context, names []string, outStream io.Writer) error {
	return nil
}
func (b *Backend) GetImage(ctx context.Context, refOrID string, options backend.GetImageOpts) (*dockerimage.Image, error) {
	return nil, errdefs.NotFound(errors.New("image not found"))
}
func (b *Backend) ImageDelete(ctx context.Context, imageRef string, force bool, prune bool) ([]image.DeleteResponse, error) {
	return nil, nil
}
func (b *Backend) ImageHistory(ctx context.Context, imageName string) ([]*image.HistoryResponseItem, error) {
	return nil, nil
}
func (b *Backend) Images(ctx context.Context, opts image.ListOptions) ([]*image.Summary, error) {
	return nil, nil
}
func (b *Backend) ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*types.ImagesPruneReport, error) {
	return nil, nil
}
func (b *Backend) ImportImage(ctx context.Context, ref reference.Named, platform *v1.Platform, msg string, layerReader io.Reader, changes []string) (dockerimage.ID, error) {
	return "", nil
}
func (b *Backend) LoadImage(ctx context.Context, inTar io.ReadCloser, outStream io.Writer, quiet bool) error {
	return nil
}

func (b *Backend) PullImage(ctx context.Context, ref reference.Named, platform *v1.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error {
	return nil
}
func (b *Backend) PushImage(ctx context.Context, ref reference.Named, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error {
	return nil
}

func (b *Backend) TagImage(ctx context.Context, id dockerimage.ID, newRef reference.Named) error {
	return nil
}
