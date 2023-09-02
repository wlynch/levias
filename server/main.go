package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type CreateRequest struct {
	*container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/_ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL.Path)
	})
	r.HandleFunc("/v1.24/events", func(w http.ResponseWriter, r *http.Request) {
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

	})
	r.HandleFunc("/v1.24/containers/{id}/json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL.Path)
	})
	r.HandleFunc("/v1.24/containers/{id}/start", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL.Path)
		w.WriteHeader(304)
	})
	r.Handle("/v1.24/containers/create", http.HandlerFunc(containerCreate))
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := httputil.DumpRequest(r, true)
		if err != nil {
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
		fmt.Println(string(raw))

		w.WriteHeader(http.StatusNotFound)
	})
	if err := http.ListenAndServe(":8080", r); err != nil {
		panic(err)
	}
}

func containerCreate(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	raw, err := httputil.DumpRequest(req, true)
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
	}
	fmt.Println(string(raw))

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

	// use the current context in kubeconfig
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), nil).ClientConfig()
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	pod, err := clientset.CoreV1().Pods("default").Get(ctx, "forever", metav1.GetOptions{})
	if err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		return
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

	_, err = clientset.CoreV1().Pods("default").UpdateEphemeralContainers(ctx, "forever", pod, metav1.UpdateOptions{})
	if err != nil {
		fmt.Println(err)

		switch {
		case errors.IsAlreadyExists(err):
			http.Error(w, fmt.Sprint(err), http.StatusConflict)
		default:
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
	}

	resp := container.CreateResponse{
		ID: ec.Name,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	json.NewEncoder(w).Encode(resp)
}

var (
	regex = regexp.MustCompile(`[^a-zA-Z0-9]`)
)

func k8sName(name string) string {
	s := strings.ToLower(name)
	return regex.ReplaceAllString(s, "-")
}
