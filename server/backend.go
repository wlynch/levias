package main

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/docker/docker/api/server/router/system"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/swarm"
	systypes "github.com/docker/docker/api/types/system"
	"github.com/docker/docker/errdefs"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrUnimplemented = errdefs.NotImplemented(errors.New("not implemented"))
)

type Backend struct {
	system.Backend
	system.ClusterBackend

	client   *kubernetes.Clientset
	verifier *Verifier
}

func (b *Backend) SystemInfo(context.Context) (*systypes.Info, error) {
	return &systypes.Info{}, nil
}

func (b *Backend) SystemVersion(context.Context) (types.Version, error) {
	return types.Version{
		Platform:     struct{ Name string }{Name: "levias"},
		APIVersion:   "1.45",
		Arch:         runtime.GOARCH,
		Os:           runtime.GOOS,
		Experimental: true,
		GoVersion:    runtime.Version(),
	}, nil
}

func (b *Backend) SystemDiskUsage(ctx context.Context, opts system.DiskUsageOptions) (*types.DiskUsage, error) {
	return nil, ErrUnimplemented
}

func (b *Backend) SubscribeToEvents(since, until time.Time, ef filters.Args) ([]events.Message, chan interface{}) {
	return nil, nil
}

func (b *Backend) UnsubscribeFromEvents(chan interface{}) {

}

func (b *Backend) AuthenticateToRegistry(ctx context.Context, authConfig *registry.AuthConfig) (string, string, error) {
	return "", "", ErrUnimplemented
}

func (b *Backend) Info(context.Context) swarm.Info {
	return swarm.Info{}
}
