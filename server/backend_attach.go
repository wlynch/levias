package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/backend"
	"github.com/moby/moby/pkg/stdcopy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (b *Backend) ContainerAttach(name string, c *backend.ContainerAttachConfig) error {
	// y u no pass in context docker?
	ctx := context.TODO()

	json.NewEncoder(os.Stdout).Encode(c)

	s := strings.Split(name, ".")
	if len(s) != 3 {
		return fmt.Errorf("invalid container name %q", name)
	}
	ns, pod, container := s[0], s[1], s[2]

	if _, err := b.waitForReady(ctx, ns, pod, container); err != nil {
		return err
	}

	req := b.client.CoreV1().Pods(ns).GetLogs(pod, &corev1.PodLogOptions{
		Container: container,
	})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer podLogs.Close()

	// k8s doesn't support separate stdout/stderr :(
	stdin, stdout, stderr, err := c.GetStreams(c.MuxStreams)
	if err != nil {
		return err
	}
	defer stdin.Close()
	if c.MuxStreams {
		stderr = stdcopy.NewStdWriter(stderr, stdcopy.Stderr)
		stdout = stdcopy.NewStdWriter(stdout, stdcopy.Stdout)
	}

	/*
		if c.UseStdin {
			go func() {
				reader := bufio.NewReader(stdin)
				for {
					line, err := reader.ReadBytes('\n')
					if err != nil {
						return
					}
					fmt.Println(string(line))
				}
			}()
		}
	*/

	_, err = io.Copy(stdout, podLogs)
	fmt.Println("done reading logs!")
	return err
}

func (b *Backend) waitForReady(ctx context.Context, namespace, pod, container string) (*corev1.ContainerStatus, error) {
	if namespace == "" {
		namespace = "default"
	}
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		pod, err := b.client.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		for _, c := range pod.Status.EphemeralContainerStatuses {
			if c.Name == container {
				if c.State.Running != nil || c.State.Terminated != nil {
					return &c, nil
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}
