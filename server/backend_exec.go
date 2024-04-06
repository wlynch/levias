package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

func (b *Backend) ContainerExecCreate(name string, config *types.ExecConfig) (string, error) {
	ctx := context.TODO()

	fmt.Println(name)
	json.NewEncoder(os.Stdout).Encode(config)

	s := strings.Split(name, ".")
	if len(s) != 3 {
		return "", fmt.Errorf("invalid container name %q", name)
	}
	ns, podName, image := s[0], s[1], s[2]
	if image == "buildx_buildkit_default" {
		image = "moby/buildkit"
	}

	pod, err := b.client.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	ec := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:  fmt.Sprintf("levias-%s", rand.String(8)),
			Image: image,
			Args:  config.Cmd,
		},
	}
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ec)

	out, err := b.client.CoreV1().Pods(ns).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		return "", err
	}

	return strings.Join([]string{out.Namespace, out.Name, ec.Name}, "."), nil

}

func (b *Backend) ContainerExecInspect(id string) (*backend.ExecInspect, error) {
	return nil, ErrUnimplemented
}

func (b *Backend) ContainerExecResize(name string, height, width int) error { return ErrUnimplemented }

func (b *Backend) ContainerExecStart(ctx context.Context, name string, options container.ExecStartOptions) error {
	return nil
}

func (b *Backend) ExecExists(name string) (bool, error) { return false, ErrUnimplemented }
