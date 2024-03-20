package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/gorilla/mux"
	"github.com/wlynch/levias/pkg/token"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	ctx := context.Background()

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

	r := mux.NewRouter()
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := httputil.DumpRequest(r, true)
		if err != nil {
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
		fmt.Println(string(raw))

		w.WriteHeader(http.StatusNotFound)
	})

	ts := token.NewFileTokenSource("/var/run/secrets/kubernetes.io/serviceaccount/token")
	// Load cluster authenticated client
	client := internalClient(ts)
	verifier, err := NewVerifierFromToken(ctx, client, ts)
	if err != nil {
		log.Fatal(err)
	}
	srv := &Server{
		client:   clientset,
		verifier: verifier,
	}
	addRoutes(r, srv)

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
