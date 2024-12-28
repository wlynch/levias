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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	b.mu.Lock()
	defer b.mu.Unlock()

	ns, pod, err := getPod(ctx)
	if err != nil {
		return nil, err
	}
	pods, err := b.client.CoreV1().Pods(ns).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]*types.Container, 0, len(pods.Spec.EphemeralContainers))
	for i, ec := range pods.Spec.EphemeralContainers {
		status := pods.Status.EphemeralContainerStatuses[i]
		out = append(out, &types.Container{
			ID:      strings.Join([]string{ns, pod, ec.Name}, "."),
			Names:   []string{ec.Name},
			Image:   ec.Image,
			State:   status.State.String(),
			ImageID: status.ImageID,
			Command: strings.Join(ec.Command, " "),
		})
	}
	return out, nil
}
