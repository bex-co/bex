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

package controller

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// edgeTraefikImage is the Traefik the deployed chart runs: the `image.tag` pin
// in deploy/gitops/base/values/traefik.values.yaml (w8/m145).
const edgeTraefikImage = "traefik:v3.7.17@sha256:30ff62491aacdb862ceb8c9edcf2627da638d7deafdd4b6dab32fe34f8aa3e90"

// TestEdgeCompressionOnTheDeployedTraefik is w8/m83's actual-proxy gate: the
// operator's own middleware chain, loaded into the deployed Traefik release,
// gzips eligible responses that decode byte for byte, leaves partial, encoded,
// excluded and refused responses as the origin wrote them, and does not
// buffer a stream. The no-transform and strong-ETag rows pin the two
// differences from Render signed off in ADR018. Opt-in: needs Docker.
func TestEdgeCompressionOnTheDeployedTraefik(t *testing.T) {
	if os.Getenv("BEX_TRAEFIK_INTEGRATION") == "" {
		t.Skip("set BEX_TRAEFIK_INTEGRATION=1 to run the chain on the deployed Traefik release")
	}
	text := bytes.Repeat([]byte("bex compression probe line\n"), 12000)
	authored := gzipBytes(t, text)
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		h := rw.Header()
		switch req.URL.Path {
		case "/text":
			h.Set("Content-Type", "text/plain; charset=utf-8")
		case "/etag":
			h.Set("Content-Type", "text/plain")
			h.Set("ETag", `"v1"`)
		case "/no-transform":
			h.Set("Content-Type", "text/plain")
			h.Set("Cache-Control", "no-cache, no-transform")
			h.Set("ETag", `"v1"`)
		case "/authored":
			h.Set("Content-Type", "text/plain")
			h.Set("Content-Encoding", "gzip")
			_, _ = rw.Write(authored)
			return
		case "/stream":
			h.Set("Content-Type", "application/x-ndjson")
			for i := range 3 {
				_, _ = fmt.Fprintf(rw, "{\"event\":%d,\"pad\":%q}\n", i, strings.Repeat("x", 2048))
				rw.(http.Flusher).Flush()
				time.Sleep(time.Second)
			}
			return
		case "/events":
			h.Set("Content-Type", "text/event-stream")
		case "/png":
			h.Set("Content-Type", "image/png")
		case "/range":
			h.Set("Content-Type", "text/plain")
			http.ServeContent(rw, req, "probe.txt", time.Time{}, bytes.NewReader(text))
			return
		}
		_, _ = rw.Write(text)
	}))
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	origin.Listener = listener
	origin.Start()
	t.Cleanup(origin.Close)

	app := &appv1alpha1.App{}
	app.Name = "probe"
	names, specs := edgeCompressionMiddlewares(app)
	middlewares := map[string]any{}
	for i, name := range names {
		middlewares[name] = specs[i]
	}
	routing := map[string]any{
		"routers": map[string]any{"web": map[string]any{"rule": "PathPrefix(`/`)", "service": "origin", "middlewares": names}},
		"services": map[string]any{"origin": map[string]any{"loadBalancer": map[string]any{"servers": []any{
			map[string]any{"url": fmt.Sprintf("http://host.docker.internal:%d", listener.Addr().(*net.TCPAddr).Port)}}}}},
	}
	if len(middlewares) > 0 {
		routing["middlewares"] = middlewares
	}
	dynamic, _ := json.Marshal(map[string]any{"http": routing})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dynamic.yml"), dynamic, 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	name := fmt.Sprintf("bex-edge-compress-it-%d", os.Getpid())
	run := exec.Command("docker", "run", "-d", "--rm", "--name", name, "--add-host", "host.docker.internal:host-gateway",
		"-p", fmt.Sprintf("%d:8000", port), "-v", dir+":/dynamic:ro",
		edgeTraefikImage, "--entrypoints.web.address=:8000", "--providers.file.filename=/dynamic/dynamic.yml")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("docker run: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForRoute(t, name, base+"/text")

	for _, tc := range []struct {
		name, path, acceptEncoding, rangeHeader string
		wantStatus                              int
		wantEncoding, wantETag                  string
		want                                    []byte
	}{
		{name: "gzip decodes exactly", path: "/text", acceptEncoding: "gzip", wantStatus: 200, wantEncoding: "gzip", want: text},
		{name: "a Brotli offer still gets gzip", path: "/text", acceptEncoding: "br, gzip", wantStatus: 200, wantEncoding: "gzip", want: text},
		{name: "Brotli only is identity", path: "/text", acceptEncoding: "br", wantStatus: 200, want: text},
		{name: "a browser offer gets gzip", path: "/text", acceptEncoding: "gzip, deflate, br, zstd", wantStatus: 200, wantEncoding: "gzip", want: text},
		{name: "zstd only is identity", path: "/text", acceptEncoding: "zstd", wantStatus: 200, want: text},
		{name: "no offer is identity", path: "/text", wantStatus: 200, want: text},
		{name: "refused coding is identity", path: "/text", acceptEncoding: "gzip;q=0, br;q=0", wantStatus: 200, want: text},
		{name: "strong ETag kept (ADR018 divergence)", path: "/etag", acceptEncoding: "gzip", wantStatus: 200, wantEncoding: "gzip", wantETag: `"v1"`, want: text},
		{name: "no-transform compressed (ADR018 divergence)", path: "/no-transform", acceptEncoding: "gzip", wantStatus: 200, wantEncoding: "gzip", wantETag: `"v1"`, want: text},
		{name: "authored encoding passes through", path: "/authored", acceptEncoding: "gzip, br", wantStatus: 200, wantEncoding: "gzip", want: text},
		{name: "event stream untouched", path: "/events", acceptEncoding: "gzip, br", wantStatus: 200, want: text},
		{name: "image untouched", path: "/png", acceptEncoding: "gzip, br", wantStatus: 200, want: text},
		{name: "small range untouched", path: "/range", acceptEncoding: "gzip, br", rangeHeader: "bytes=0-99", wantStatus: 206, want: text[:100]},
		{name: "large range untouched", path: "/range", acceptEncoding: "gzip, br, zstd", rangeHeader: "bytes=100-200099", wantStatus: 206, want: text[100:200100]},
		{name: "streamed ndjson decodes exactly", path: "/stream", acceptEncoding: "gzip", wantStatus: 200, wantEncoding: "gzip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, base+tc.path, nil)
			req.Header.Set("Accept-Encoding", tc.acceptEncoding)
			if tc.acceptEncoding == "" {
				req.Header.Del("Accept-Encoding")
			}
			if tc.rangeHeader != "" {
				req.Header.Set("Range", tc.rangeHeader)
			}
			resp, err := (&http.Transport{DisableCompression: true}).RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != tc.wantStatus || resp.Header.Get("Content-Encoding") != tc.wantEncoding || resp.Header.Get("ETag") != tc.wantETag {
				t.Fatalf("got %d encoding %q etag %q, want %d %q %q", resp.StatusCode, resp.Header.Get("Content-Encoding"), resp.Header.Get("ETag"),
					tc.wantStatus, tc.wantEncoding, tc.wantETag)
			}
			if tc.wantEncoding != "" && tc.want != nil && len(body) >= len(text)/10 {
				t.Errorf("encoded body is %d bytes for a %d-byte response", len(body), len(text))
			}
			if tc.wantEncoding == "gzip" {
				body = gunzipBytes(t, body)
			}
			if tc.want != nil && !bytes.Equal(body, tc.want) {
				t.Errorf("decoded body differs from what the origin wrote (%d vs %d bytes)", len(body), len(tc.want))
			}
		})
	}

	assertEdgeStreamUnbuffered(t, base)
}

// assertEdgeStreamUnbuffered checks that the first gzip-encoded event of
// /stream decodes while the origin is still sleeping before the second.
func assertEdgeStreamUnbuffered(t *testing.T, base string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+"/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	start := time.Now()
	resp, err := (&http.Transport{DisableCompression: true}).RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("stream encoding = %q, want gzip", resp.Header.Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	first, err := bufio.NewReader(zr).ReadString('\n')
	if err != nil || !strings.HasPrefix(first, `{"event":0`) {
		t.Fatalf("first event = %q, %v", first, err)
	}
	if elapsed := time.Since(start); elapsed > 900*time.Millisecond {
		t.Fatalf("first event arrived after %s: the stream was buffered", elapsed)
	}

}

func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	_, _ = w.Write(b)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func gunzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// waitForRoute waits until Traefik serves the router, failing with its log.
func waitForRoute(t *testing.T, container, url string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(url); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	logs, _ := exec.Command("docker", "logs", container).CombinedOutput()
	t.Fatalf("Traefik never routed %s:\n%s", url, logs)
}
