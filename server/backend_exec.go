package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var (
	// TODO: make this a TTL cache
	state = map[string]*ExecState{}
)

type ExecState struct {
	cfg *types.ExecConfig

	running  bool
	exitCode *int
}

func (b *Backend) ContainerExecCreate(name string, config *types.ExecConfig) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	//ctx := context.TODO()

	fmt.Println(name)
	json.NewEncoder(os.Stdout).Encode(config)

	/*
		s := strings.Split(name, ".")
		if len(s) != 3 {
			return "", fmt.Errorf("invalid container name %q", name)
		}

			_, _, image := s[0], s[1], s[2]
			if image == "buildx_buildkit_default" {
				image = "moby/buildkit"
			}
	*/

	/*
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
	*/

	id := fmt.Sprintf("%s.%s", name, rand.String(8))
	state[id] = &ExecState{
		cfg: config,
	}

	return id, nil
}

func (b *Backend) ContainerExecInspect(id string) (*backend.ExecInspect, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	//ctx := context.TODO()

	s, ok := state[id]
	if !ok {
		return nil, fmt.Errorf("exec config not found for %q", id)
	}
	return &backend.ExecInspect{
		ID:       id,
		ExitCode: s.exitCode,
		Running:  false,
	}, nil

	/*
		s := strings.Split(id, ".")
		if len(s) != 4 {
			return nil, fmt.Errorf("invalid container name %q", id)
		}
		ns, podName, image, _ := s[0], s[1], s[2], s[3]

		pod, err := b.client.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		for _, ec := range pod.Status.EphemeralContainerStatuses {
			fmt.Println("@@@", ec.Name, image)
			if ec.Name == image {
				var exitCode *int
				if ec.State.Terminated != nil {
					i := int(ec.State.Terminated.ExitCode)
					exitCode = &i
				}
				return &backend.ExecInspect{
					Running:     *ec.Started,
					ID:          id,
					ExitCode:    exitCode,
					ContainerID: ec.ContainerID,
				}, nil
			}
		}
		return nil, fmt.Errorf("ephemeral container %q not found", image)
	*/
}

func (b *Backend) ContainerExecResize(name string, height, width int) error { return ErrUnimplemented }

func (b *Backend) ContainerExecStart(ctx context.Context, name string, options container.ExecStartOptions) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	log.Println("ContainerExecStart", name)
	s := strings.Split(name, ".")
	if len(s) != 4 {
		return fmt.Errorf("invalid container name %q", name)
	}
	ns, pod, container, execID := s[0], s[1], s[2], s[3]

	fmt.Println("$", ns, pod, container, execID)

	state, ok := state[name]
	if !ok {
		return fmt.Errorf("exec config not found for %q", name)
	}

	req := b.client.CoreV1().RESTClient().Post().Resource("pods").Name(pod).Namespace(ns).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: container,
		Stdin:     options.Stdin != nil,
		Stdout:    options.Stdout != nil,
		Stderr:    options.Stderr != nil,
		Command:   state.cfg.Cmd,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(b.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("NewSPDYExecutor: %w", err)
	}

	timeout := int64(60)
	watcher, err := b.client.CoreV1().Pods(ns).Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: &timeout,
		FieldSelector:  fmt.Sprintf("metadata.name=%s", pod),
	})
	defer watcher.Stop()
	if ok, err := waitForEphemeralContainer(watcher, container); err != nil {
		return fmt.Errorf("waitForEphemeralContainer: %w", err)
	} else if !ok {
		return fmt.Errorf("ephemeral container %q not running", container)
	}

	if err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  options.Stdin,
		Stdout: options.Stdout,
		Stderr: options.Stderr,
	}); err != nil {
		// TODO: can we grab the exit code from the exec status?
		i := 1
		state.exitCode = &i
		return fmt.Errorf("StreamWithContext: %w", err)
	}

	return nil
}

func waitForEphemeralContainer(watcher watch.Interface, container string) (bool, error) {
	running := false
	for event := range watcher.ResultChan() {
		log.Println(event)
		switch event.Type {
		case watch.Added, watch.Modified:
			pod := event.Object.(*corev1.Pod)
			for _, ec := range pod.Status.EphemeralContainerStatuses {
				if ec.Name == container {
					log.Printf("Ephemeral container %q status: %+v\n", container, ec.State)
					running = ec.State.Running != nil
				}
			}
		case watch.Deleted:
			return false, nil
		case watch.Error:
			fmt.Errorf("Unexpected error watching pod: %w", event)
		}

		if running {
			return true, nil
		}
	}
	return running, nil
}

func (b *Backend) ExecExists(name string) (bool, error) { return true, nil }
