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
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestOriginServerOnTheDeployedTraefik is w8/321's actual-proxy gate. The edge
// plugin runs interpreted (Yaegi) inside the deployed Traefik release, with
// the sanitizer on the entrypoint and a web service's chain on the router, as
// in production: the app's Server header moves to X-Render-Origin-Server, an
// app without one reads "Render", and streams, gzip and WebSockets still pass
// through unbuffered. Opt-in: needs Docker.
func TestOriginServerOnTheDeployedTraefik(t *testing.T) {
	if os.Getenv("BEX_TRAEFIK_INTEGRATION") == "" {
		t.Skip("set BEX_TRAEFIK_INTEGRATION=1 to run the plugin on the deployed Traefik release")
	}
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	base := startOriginServerTraefik(t, startEdgeOrigin(t, originServerProbe(hold)))

	for _, tc := range []struct{ path, want string }{{"/nginx", "nginx/1.27.5"}, {"/none", "Render"}} {
		resp, err := http.Get(base + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.Header.Get("X-Render-Origin-Server") != tc.want || resp.Header.Get("Server") != "" ||
			resp.Header.Get("X-Powered-By") != "app" || resp.Header.Get("Rndr-Id") == "" {
			t.Errorf("%s headers %v, want X-Render-Origin-Server %q, no Server, the app's and the edge's own headers kept", tc.path, resp.Header, tc.want)
		}
	}
	t.Run("an event stream is not buffered", func(t *testing.T) { assertEventStreamUnbuffered(t, base) })
	t.Run("a gzipped stream arrives event by event", func(t *testing.T) { assertEdgeStreamUnbuffered(t, base) })
	t.Run("a slow body of known length is not held", func(t *testing.T) { assertKnownLengthUnbuffered(t, base) })
	t.Run("a WebSocket upgrade echoes", func(t *testing.T) { assertWebSocketEcho(t, strings.TrimPrefix(base, "http://")) })
}

// originServerProbe is the gate's app: Server headers of each kind, an event
// stream whose second event waits for the test (so the first reaches the
// client only through a flush), a gzipped ndjson stream and a WebSocket echo.
func originServerProbe(hold chan struct{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		h := rw.Header()
		switch req.URL.Path {
		case "/nginx":
			h.Set("Server", "nginx/1.27.5")
			h.Set("X-Powered-By", "app")
			_, _ = io.WriteString(rw, "hi")
		case "/none":
			h.Set("X-Powered-By", "app")
			_, _ = io.WriteString(rw, "hi")
		case "/events":
			h.Set("Server", "uvicorn")
			h.Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(rw, "data: one\n\n")
			rw.(http.Flusher).Flush()
			select {
			case <-hold:
			case <-req.Context().Done():
			case <-time.After(20 * time.Second):
			}
			_, _ = io.WriteString(rw, "data: two\n\n")
		case "/stream": // as the compression gate's, for assertEdgeStreamUnbuffered
			h.Set("Content-Type", "application/x-ndjson")
			for i := range 3 {
				_, _ = fmt.Fprintf(rw, "{\"event\":%d,\"pad\":%q}\n", i, strings.Repeat("x", 2048))
				rw.(http.Flusher).Flush()
				time.Sleep(time.Second)
			}
		case "/slow":
			// A body of known length whose second half waits for the test.
			h.Set("Server", "slow-app")
			h.Set("Content-Type", "text/plain")
			h.Set("Content-Length", "10")
			_, _ = io.WriteString(rw, "hello")
			rw.(http.Flusher).Flush()
			select {
			case <-hold:
			case <-req.Context().Done():
			case <-time.After(20 * time.Second):
			}
			_, _ = io.WriteString(rw, "world")
		case "/ws":
			echoWebSocket(rw)
		default:
			_, _ = io.WriteString(rw, "ok")
		}
	}
}

func echoWebSocket(rw http.ResponseWriter) {
	conn, brw, err := http.NewResponseController(rw).Hijack()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	_, _ = brw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	_ = brw.Flush()
	buf := make([]byte, 4)
	if _, err := io.ReadFull(brw, buf); err == nil {
		_, _ = conn.Write(buf)
	}
}

// startOriginServerTraefik runs the deployed Traefik with both edge plugins
// loaded as production loads them: the client-identity sanitizer on the
// entrypoint, and a web service's router chain from the operator's own specs
// in its order (edge compression, the WebSocket meter, the origin-server
// instance last). It returns the edge's base URL.
func startOriginServerTraefik(t *testing.T, originPort int) string {
	t.Helper()
	app := &appv1alpha1.App{}
	app.Name = "probe"
	names, specs := edgeCompressionMiddlewares(app)
	middlewares := map[string]any{"client-ip": map[string]any{traefikPluginKey: map[string]any{"bexClientIPHeaders": map[string]any{}}}}
	for i, name := range names {
		middlewares[name] = specs[i]
	}
	middlewares["probe-ws-egress"] = websocketMeterSpec(app)
	middlewares["probe-origin-server"] = originServerSpec
	plugins := []struct{ name, dir, module string }{
		{"bexClientIPHeaders", "clientipheaders", "github.com/bex-co/bex-client-ip-headers"},
		{"bexWebsocketEgress", "websocketegress", "github.com/bex-co/bex-websocket-egress"},
	}
	dockerArgs := make([]string, 0, 2*len(plugins))
	traefikArgs := make([]string, 0, len(plugins)+1)
	for _, p := range plugins {
		src, err := filepath.Abs("../../../../deploy/gitops/charts/traefik-plugins/" + p.dir)
		if err != nil {
			t.Fatal(err)
		}
		dockerArgs = append(dockerArgs, "-v", src+":/plugins-local/src/"+p.module+":ro")
		traefikArgs = append(traefikArgs, "--experimental.localPlugins."+p.name+".moduleName="+p.module)
	}
	return runEdgeTraefik(t, originPort, edgeTraefikConfig{
		label: "origin-server", chain: append(names, "probe-ws-egress", "probe-origin-server"), middlewares: middlewares, readyPath: "/",
		dockerArgs:  dockerArgs,
		traefikArgs: append(traefikArgs, "--entrypoints.web.http.middlewares=client-ip@file"),
	})
}

func assertEventStreamUnbuffered(t *testing.T, base string) {
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(base + "/events")
	if err != nil {
		t.Fatalf("no head while the origin holds the stream: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || line != "data: one\n" || resp.Header.Get("X-Render-Origin-Server") != "uvicorn" {
		t.Fatalf("first event %q err %v headers %v, want it while the origin still holds the stream", line, err, resp.Header)
	}
}

func assertKnownLengthUnbuffered(t *testing.T, base string) {
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(base + "/slow")
	if err != nil {
		t.Fatalf("no head while the origin holds the body: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	first := make([]byte, 5)
	if _, err := io.ReadFull(resp.Body, first); err != nil || string(first) != "hello" || resp.Header.Get("X-Render-Origin-Server") != "slow-app" {
		t.Fatalf("first bytes %q err %v headers %v, want them while the origin still holds the body", first, err, resp.Header)
	}
}

func assertWebSocketEcho(t *testing.T, addr string) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	_, _ = io.WriteString(conn, "GET /ws HTTP/1.1\r\nHost: app\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade %v err %v, want 101", resp, err)
	}
	_, _ = io.WriteString(conn, "ping")
	echo := make([]byte, 4)
	if _, err := io.ReadFull(br, echo); err != nil || string(echo) != "ping" {
		t.Fatalf("echo %q err %v, want ping", echo, err)
	}
}
