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
	"time"

	"github.com/wlynch/levias/pkg/token"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/oauth2"
)

const (
	defaultURL       = "http://levias.default.svc.cluster.local"
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
	fmt.Fprintln(os.Stderr, "starting levias proxy on", l.Addr(), "->", url.String())

	transport := http.DefaultTransport
	ts := token.NewFileTokenSource(getenv("LEVIAS_TOKEN_PATH", defaultTokenPath))
	if token, err := ts.Token(); err != nil {
		fmt.Fprintln(os.Stderr, "unable to read token, falling back to no credentials:", err)
	} else {
		transport = &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base:   transport,
		}
	}
	transport = &logtransport{base: transport}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			fmt.Fprintln(os.Stderr, r.In.Method, r.In.URL)
			r.SetURL(url)
		},
		Transport: transport,
	}
	h2s := &http2.Server{}
	srv := &http.Server{
		Handler: h2c.NewHandler(proxy, h2s),
	}

	if os.Args[1] == "levias-daemon" {
		fmt.Fprintf(os.Stdout, "DOCKER_HOST=tcp://%s", l.Addr())
		if err := srv.Serve(l); err != nil {
			panic(err)
		}
	} else {
		// Start up the server in the background and run command
		go func() {
			if err := srv.Serve(l); err != nil {
				panic(err)
			}
		}()
		time.Sleep(1 * time.Second)
	}

	cmd := exec.Command("docker", os.Args[1:]...)
	cmd.Env = []string{fmt.Sprintf("DOCKER_HOST=tcp://%s", l.Addr())}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println("Running...", cmd.String())
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running docker:", err)
	}

	// Flush stdout/stderr to make sure everything is printed before we exit.
	os.Stdout.Sync()
	os.Stderr.Sync()

	os.Exit(cmd.ProcessState.ExitCode())
}

func getenv(name, defaultVal string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defaultVal
}

type logtransport struct {
	base http.RoundTripper
}

func (t *logtransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if rawreq, err := httputil.DumpRequest(req, false); err == nil {
		fmt.Println(">>>", string(rawreq))
	}

	resp, err := t.base.RoundTrip(req)
	if err == nil {
		// This might cause issues on attachments since the reverse proxy doesn't like the
		// HTTP upgrade. It's not really an error though.
		/*
			if rawresp, err := httputil.DumpResponse(resp, true); err == nil {
				fmt.Println(string(rawresp))
			}
		*/
		fmt.Println("<<<", resp.Request.URL, resp.Status)
	}

	return resp, err
}
