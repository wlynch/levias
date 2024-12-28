package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	containerpkg "github.com/docker/docker/container"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

func getPod(ctx context.Context) (string, string, error) {
	podName := GetPod(ctx)
	if podName == "" {
		return "", "", fmt.Errorf("pod name not found")
	}
	ns := GetNamespace(ctx)
	if ns == "" {
		return "", "", fmt.Errorf("namespace not found")
	}
	return ns, podName, nil
}

func (b *Backend) ContainerCreate(ctx context.Context, config backend.ContainerCreateConfig) (container.CreateResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	json.NewEncoder(os.Stderr).Encode(config)

	ns, podName, err := getPod(ctx)
	if err != nil {
		return container.CreateResponse{}, err
	}

	pod, err := b.client.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return container.CreateResponse{}, err
	}

	ec := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:    fmt.Sprintf("levias-%s", rand.String(8)),
			Image:   config.Config.Image,
			Command: config.Config.Entrypoint,
			Args:    config.Config.Cmd,
		},
	}
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ec)

	out, err := b.client.CoreV1().Pods(ns).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		return container.CreateResponse{}, err
	}

	return container.CreateResponse{
		ID: strings.Join([]string{out.Namespace, out.Name, ec.Name}, "."),
	}, nil
}

func (b *Backend) ContainerKill(name string, signal string) error {
	return ErrUnimplemented
}

func (b *Backend) ContainerPause(name string) error {
	return ErrUnimplemented
}

func (b *Backend) ContainerRename(oldName, newName string) error {
	return ErrUnimplemented
}

func (b *Backend) ContainerResize(name string, height, width int) error {
	return ErrUnimplemented
}

func (b *Backend) ContainerRestart(ctx context.Context, name string, options container.StopOptions) error {
	return ErrUnimplemented
}

func (b *Backend) ContainerRm(name string, config *backend.ContainerRmConfig) error {

	return nil
}

func (b *Backend) ContainerStart(ctx context.Context, name string, checkpoint string, checkpointDir string) error {
	return nil
}

func (b *Backend) ContainerStop(ctx context.Context, name string, options container.StopOptions) error {
	return ErrUnimplemented
}

func (b *Backend) ContainerUnpause(name string) error {
	return ErrUnimplemented
}

func (b *Backend) ContainerUpdate(name string, hostConfig *container.HostConfig) (container.ContainerUpdateOKBody, error) {
	return container.ContainerUpdateOKBody{}, ErrUnimplemented
}

func (b *Backend) ContainerWait(ctx context.Context, name string, condition containerpkg.WaitCondition) (<-chan containerpkg.StateStatus, error) {
	s := strings.Split(name, ".")
	if len(s) != 3 {
		return nil, fmt.Errorf("invalid container name %q", name)
	}
	ns, pod, container := s[0], s[1], s[2]

	state := containerpkg.NewState()

	go func() {
		s, err := b.waitForReady(ctx, ns, pod, container)
		if err != nil {
			return
		}
		if s.State.Terminated != nil {
			state.SetStopped(&containerpkg.ExitStatus{
				ExitCode: int(s.State.Terminated.ExitCode),
				ExitedAt: s.State.Terminated.FinishedAt.Time,
			})
		}
		if s.State.Running != nil {
			state.SetRunning(nil, nil, true)
		}
	}()

	return state.Wait(ctx, condition), nil
}
