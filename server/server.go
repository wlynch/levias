package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"time"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
)

type Server struct {
	client   *kubernetes.Clientset
	verifier *Verifier
}

func addRoutes(mux *mux.Router, s *Server) {
	mux.HandleFunc("/_ping", handlePing)
	mux.HandleFunc("/v1.24/events", handleEvents)
	mux.HandleFunc("/v1.24/images/{owner}/{image}/json", handleImagesJSON)
	mux.HandleFunc("/v1.24/containers/{container}/json", handleContainersJSON)
	mux.HandleFunc("/v1.24/containers/{container}/exec", s.handleContainersExec)
	mux.HandleFunc("/v1.24/containers/{id}/start", handleContainersStart)
	mux.HandleFunc("/v1.24/containers/create", s.handleContainersCreate)
	mux.HandleFunc("/v1.24/exec/{container}/start", s.handleExecContainersStart)
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	raw, err := httputil.DumpRequest(r, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println(string(raw))
	fmt.Println(r.URL.Path, r.URL.Query())

	rawFilters := r.URL.Query().Get("filters")
	filter, err := filters.FromJSON(rawFilters)
	if err != nil {
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println(filter)

	for _, out := range []events.Message{
		// Docker CLI looks for destroy event to close the stream.
		{
			Type:   "container",
			Action: "destroy",
			Status: "destroy",
			Actor: events.Actor{
				Attributes: map[string]string{
					"exitCode": "0",
				},
			},
			Time: time.Now().Unix(),
		},
	} {
		json.NewEncoder(os.Stdout).Encode(out)
		json.NewEncoder(w).Encode(out)
	}
}

func handleImagesJSON(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
}

func handleContainersJSON(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	json.NewEncoder(w).Encode(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: true,
			},
		},
	})
}

type CreateRequest struct {
	*container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
}

func (s *Server) handleContainersExec(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	raw, err := httputil.DumpRequest(r, true)
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	fmt.Println(string(raw))

	claims, err := s.verifier.GetClaims(r.Context(), r)
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusUnauthorized)
		return
	}
	ns := claims.Kubernetes.Namespace
	pod := claims.Kubernetes.Pod.Name

	data := new(CreateRequest)
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	data.Image = "moby/buildkit"

	ctx := r.Context()
	resp, err := s.createContainer(ctx, ns, pod, data)
	if err != nil {
		switch {
		case errors.IsAlreadyExists(err):
			http.Error(w, fmt.Sprint(err), http.StatusConflict)
		default:
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleExecContainersStart(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)

	raw, err := httputil.DumpRequest(r, true)
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	fmt.Println(string(raw))
	//io.ReadAll(r.Body)

	claims, err := s.verifier.GetClaims(r.Context(), r)
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusUnauthorized)
		return
	}
	ns := claims.Kubernetes.Namespace
	pod := claims.Kubernetes.Pod.Name

	container := mux.Vars(r)["container"]

	if err := s.waitForReady(r.Context(), ns, pod, container); err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}

	req := s.client.CoreV1().Pods(ns).GetLogs(pod, &corev1.PodLogOptions{
		Container: container,
	})
	podLogs, err := req.Stream(r.Context())
	if err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	defer podLogs.Close()

	inStream, outStream, err := httputils.HijackConnection(w)
	if err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	defer httputils.CloseStreams(inStream, outStream)
	if _, ok := r.Header["Upgrade"]; ok {
		contentType := types.MediaTypeRawStream
		fmt.Fprint(outStream, "HTTP/1.1 101 UPGRADED\r\nContent-Type: "+contentType+"\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n")
	} else {
		fmt.Fprint(outStream, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n")
	}
	// copy headers that were removed as part of hijack
	if err := w.Header().WriteSubset(outStream, nil); err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(outStream, "\r\n")

	stdin := inStream
	stdout := outStream

	//stderr := stdcopy.NewStdWriter(outStream, stdcopy.Stderr)
	stdout = stdcopy.NewStdWriter(outStream, stdcopy.Stderr)

	/*
		w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
		w.WriteHeader(200)
	*/

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

	//sw := stdcopy.NewStdWriter(stdout, stdcopy.Stderr)

	//sw2 := stdcopy.NewStdWriter(w, stdcopy.Stdout)

	io.Copy(stdout, io.TeeReader(podLogs, os.Stdout))

	//<-r.Cancel
}

func handleContainersStart(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	w.WriteHeader(304)
}

func (s *Server) handleContainersCreate(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	raw, err := httputil.DumpRequest(req, true)
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	fmt.Println(string(raw))

	claims, err := s.verifier.GetClaims(ctx, req)
	if err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprint(err), http.StatusUnauthorized)
		return
	}
	ns := claims.Kubernetes.Namespace
	pod := claims.Kubernetes.Pod.Name
	fmt.Println("$$$", ns, pod)

	data := new(CreateRequest)
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	defer req.Body.Close()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}

	resp, err := s.createContainer(ctx, ns, pod, data)
	if err != nil {
		switch {
		case errors.IsAlreadyExists(err):
			http.Error(w, fmt.Sprint(err), http.StatusConflict)
		default:
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) createContainer(ctx context.Context, ns string, podName string, data *CreateRequest) (*container.CreateResponse, error) {
	pod, err := s.client.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ec := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:    fmt.Sprintf("levias-%s", rand.String(8)),
			Image:   data.Image,
			Command: data.Entrypoint,
			Args:    data.Cmd,
		},
	}
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ec)

	_, err = s.client.CoreV1().Pods(ns).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return &container.CreateResponse{
		ID: ec.Name,
	}, nil
}

func (s *Server) waitForReady(ctx context.Context, namespace, pod, container string) error {
	if namespace == "" {
		namespace = "default"
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		pod, err := s.client.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for _, c := range pod.Status.EphemeralContainerStatuses {
			if c.Name == container {
				if c.State.Running != nil {
					return nil
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}
