/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package healthprobe

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func serve(t *testing.T, h http.Handler) (addr string, port int) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	host, p, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	port, _ = strconv.Atoi(p)
	return host, port
}

// A status is healthy exactly when it is 2xx or 3xx, as on Render.
func TestHealthy(t *testing.T) {
	for status, want := range map[int]bool{
		100: false, 199: false, 200: true, 204: true, 299: true, 300: true, 301: true, 302: true,
		304: true, 399: true, 400: false, 404: false, 429: false, 500: false, 503: false,
	} {
		if got := Healthy(status); got != want {
			t.Errorf("Healthy(%d) = %v, want %v", status, got, want)
		}
	}
}

// A redirect answers the check and is never followed: the kubelet's HTTP
// prober followed a relative Location and scored the 500 behind it.
func TestCheckNeverFollowsARedirect(t *testing.T) {
	var elsewhere atomic.Int32
	addr, port := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/elsewhere":
			elsewhere.Add(1)
			http.Error(w, "boom", http.StatusInternalServerError)
		case "/rel":
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		case "/abs":
			http.Redirect(w, r, "https://example.invalid/", http.StatusMovedPermanently)
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/missing":
			http.NotFound(w, r)
		default:
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	}))
	for path, wantErr := range map[string]bool{"/ok": false, "/rel": false, "/abs": false, "/missing": true, "/boom": true} {
		err := Check(context.Background(), addr, port, path, "", 2*time.Second)
		if (err != nil) != wantErr {
			t.Errorf("Check(%s) = %v, want error %v", path, err, wantErr)
		}
	}
	if n := elsewhere.Load(); n != 0 {
		t.Fatalf("the redirect target was requested %d times, want 0", n)
	}
}

// The query string reaches the app as authored, and the Host header is the one
// asked for, not the pod address.
func TestCheckSendsPathQueryAndHost(t *testing.T) {
	var gotURI, gotHost string
	addr, port := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI, gotHost = r.RequestURI, r.Host
	}))
	if err := Check(context.Background(), addr, port, "/hc?echo_code=302&echo_header=Location:/x", "app.onbex.co", 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if gotURI != "/hc?echo_code=302&echo_header=Location:/x" || gotHost != "app.onbex.co" {
		t.Fatalf("request = %q Host %q", gotURI, gotHost)
	}
	if err := Check(context.Background(), addr, port, "/hc", "", 2*time.Second); err != nil || gotHost != net.JoinHostPort(addr, strconv.Itoa(port)) {
		t.Fatalf("no Host asked for: %v, Host %q", err, gotHost)
	}
}

// A response slower than the timeout fails the check, and a closed port too.
func TestCheckTimeoutAndRefused(t *testing.T) {
	addr, port := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	start := time.Now()
	if err := Check(context.Background(), addr, port, "/", "", 200*time.Millisecond); err == nil {
		t.Fatal("a response slower than the timeout passed")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("timeout not enforced: %v", time.Since(start))
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Check(context.Background(), "127.0.0.1", closedPort, "/", "", time.Second); err == nil {
		t.Fatal("a refused connection passed")
	}
}

// Install copies the executable read-and-execute only, and replaces an existing
// file atomically.
func TestInstall(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "health-probe")
	if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Install(dest); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o555 || info.Size() < 1024 {
		t.Fatalf("installed %v size %d", info.Mode(), info.Size())
	}
	entries, _ := os.ReadDir(filepath.Dir(dest))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}
