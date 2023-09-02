package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"

	"golang.org/x/oauth2"
)

const (
	defaultURL       = "http://levias.tekton-pipelines.svc.cluster.local"
	defaultTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

func main() {
	flag.Parse()

	url, err := url.Parse(getenv("LEVIAS_URL", defaultURL))
	if err != nil {
		panic(err)
	}
	if url.Scheme == "" {
		url.Scheme = "http"
	}
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "starting levias proxy on", l.Addr())

	transport := http.DefaultTransport
	ts := &fileTokenSource{path: getenv("LEVIAS_TOKEN_PATH", defaultTokenPath)}
	if token, err := ts.Token(); err != nil {
		fmt.Fprintln(os.Stderr, "unable to read token, falling back to no credentials:", err)
	} else {
		transport = &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
		}
	}
	srv := &http.Server{
		Handler: &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetURL(url)
			},
			Transport: transport,
		},
	}
	go func() {
		if err := srv.Serve(l); err != nil {
			panic(err)
		}
	}()

	cmd := exec.Command("docker", os.Args[1:]...)
	cmd.Env = []string{fmt.Sprintf("DOCKER_HOST=tcp://%s", l.Addr())}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	os.Exit(cmd.ProcessState.ExitCode())
}

func getenv(name, defaultVal string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defaultVal
}

type fileTokenSource struct {
	path string
}

func (f *fileTokenSource) Token() (*oauth2.Token, error) {
	b, err := os.ReadFile(f.path)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: string(b),
	}, nil
}
