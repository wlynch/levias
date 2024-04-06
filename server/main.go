package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/docker/docker/api/server"
	"github.com/docker/docker/api/server/middleware"
	"github.com/docker/docker/api/server/router/container"
	"github.com/docker/docker/api/server/router/system"
	"github.com/docker/docker/runconfig"
	"github.com/sirupsen/logrus"
	"github.com/wlynch/levias/pkg/token"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	ctx := context.Background()

	logrus.SetLevel(logrus.DebugLevel)

	// use the current context in kubeconfig
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), nil).ClientConfig()
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	clientset.RESTClient()

	ts := token.NewFileTokenSource("/var/run/secrets/kubernetes.io/serviceaccount/token")
	// Load cluster authenticated client
	client := internalClient(ts)
	verifier, err := NewVerifierFromToken(ctx, client, ts)
	if err != nil {
		log.Fatal(err)
	}

	/*
		srv := &Server{
			client:   clientset,
			verifier: verifier,
		}
		addRoutes(r, srv)
	*/

	b := &Backend{
		config:   config,
		client:   clientset,
		verifier: verifier,
	}
	s := &server.Server{}
	vm, err := middleware.NewVersionMiddleware("1.45", "1.45", "1.45")
	if err != nil {
		log.Fatalf("failed to create version middleware: %v", err)
	}
	s.UseMiddleware(&logmiddleware{})
	s.UseMiddleware(&nameTransform{})
	s.UseMiddleware(vm)
	s.UseMiddleware(&AuthMiddleware{verifier: verifier})

	r := s.CreateMux(
		system.NewRouter(b, b, nil, func() map[string]bool { return map[string]bool{} }),
		container.NewRouter(b, runconfig.ContainerDecoder{}, false /* cgroup2 */),
		//grpc.NewRouter(b),
	)
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

func internalClient(ts oauth2.TokenSource) *http.Client {
	// Add the Kubernetes cluster's CA to the system CA pool, and to
	// the default transport.
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	certs, err := os.ReadFile("/var/run/root-ca/ca.crt")
	if err != nil {
		log.Fatalf("Failed to read RootCAs: %v", err)
	}
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		log.Println("No certs appended, using system certs only")
	}

	base := http.DefaultTransport.(*http.Transport).Clone()
	base.TLSClientConfig.RootCAs = rootCAs

	return &http.Client{
		Transport: &oauth2.Transport{
			Source: ts,
			Base:   base,
		},
	}
}

type logmiddleware struct{}

func (l *logmiddleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		fmt.Println(r.Method, r.URL)
		err := handler(ctx, w, r, vars)
		fmt.Println(r.URL, err)
		return err
	}
}

// This is a big hack because moby backend API isn't consistent about plumbing through contexts.
// This ensures that names always have the same scheme when needed.
type nameTransform struct{}

func (l *nameTransform) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		name, ok := vars["name"]
		if ok {
			if len(strings.Split(name, ".")) < 3 {
				ns, pod, err := getPod(ctx)
				if err != nil {
					return err
				}
				vars["name"] = strings.Join([]string{ns, pod, name}, ".")
			}
		}
		return handler(ctx, w, r, vars)
	}
}
