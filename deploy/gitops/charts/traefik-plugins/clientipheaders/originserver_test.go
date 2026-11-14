// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bex_client_ip_headers

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func originServerEdge(t *testing.T, app http.Handler) *httptest.Server {
	t.Helper()
	backend := httptest.NewServer(app)
	t.Cleanup(backend.Close)
	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	return originServerEdgeTo(t, target)
}

// originServerEdgeTo puts the origin-server instance in front of a reverse
// proxy to target, as Traefik's sits in front of an app.
func originServerEdgeTo(t *testing.T, target *url.URL) *httptest.Server {
	t.Helper()
	h, err := New(context.Background(), httputil.NewSingleHostReverseProxy(target), &Config{OriginServer: true}, "origin-server")
	if err != nil {
		t.Fatal(err)
	}
	edge := httptest.NewServer(h)
	t.Cleanup(edge.Close)
	return edge
}

// w8/321: Render moves a web service's Server header to X-Render-Origin-Server
// and names "Render" there when the app sent none; every other app header
// passes through.
func TestOriginServerMovesTheAppsServerHeader(t *testing.T) {
	for _, tc := range []struct {
		name   string
		app    http.HandlerFunc
		status int
		want   string
	}{
		{"nginx", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Server", "nginx/1.27.5")
			w.Header().Set("X-Powered-By", "app")
			_, _ = io.WriteString(w, "hi")
		}, http.StatusOK, "nginx/1.27.5"},
		{"no server", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Powered-By", "app")
			w.WriteHeader(http.StatusNotFound)
		}, http.StatusNotFound, "Render"},
		{"its own origin header is overwritten", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Server", "gunicorn")
			w.Header().Set(originServerHeader, "spoofed")
			w.Header().Set("X-Powered-By", "app")
		}, http.StatusOK, "gunicorn"},
		{"an empty server reads as none", func(w http.ResponseWriter, _ *http.Request) {
			w.Header()["Server"] = []string{""}
			w.Header().Set("X-Powered-By", "app")
		}, http.StatusOK, "Render"},
		{"early hints first", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Link", "</app.css>; rel=preload")
			w.WriteHeader(http.StatusEarlyHints)
			w.Header().Set("Server", "uvicorn")
			w.Header().Set("X-Powered-By", "app")
			_, _ = io.WriteString(w, "hi")
		}, http.StatusOK, "uvicorn"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edge := originServerEdge(t, tc.app)
			resp, err := http.Get(edge.URL)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != tc.status || resp.Header.Get(originServerHeader) != tc.want ||
				resp.Header.Get("Server") != "" || resp.Header.Get("X-Powered-By") != "app" {
				t.Fatalf("status %d headers %v, want %d with %s %q, no Server, the app's own headers kept",
					resp.StatusCode, resp.Header, tc.status, originServerHeader, tc.want)
			}
		})
	}
}

// An informational answer goes out with the header map as it stands, so the
// app's Server moves on it too.
func TestOriginServerMovesServerOnEarlyHints(t *testing.T) {
	h, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "uvicorn")
		w.WriteHeader(http.StatusEarlyHints)
		_, _ = io.WriteString(w, "hi")
	}), &Config{OriginServer: true}, "origin-server")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Result().Header; got.Get("Server") != "" || got.Get(originServerHeader) != "uvicorn" {
		t.Fatalf("headers %v, want the app's Server moved", got)
	}
}

// A body of known length reaches the client within the proxy's flush
// interval, as Traefik's proxy flushes it, not only once net/http's buffer
// fills or the app finishes.
func TestOriginServerFlushesAKnownLengthBodyInTime(t *testing.T) {
	next := make(chan struct{})
	defer close(next)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		br := bufio.NewReader(conn)
		for {
			line, err := br.ReadString('\n')
			if err != nil || line == "\r\n" {
				break
			}
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nServer: slow-app\r\nContent-Type: text/plain\r\nContent-Length: 10\r\n\r\nhello")
		if _, ok := <-next; ok {
			_, _ = io.WriteString(conn, "world")
		}
		<-next
	}()
	edge := originServerEdgeTo(t, &url.URL{Scheme: "http", Host: ln.Addr().String()})
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(edge.URL)
	if err != nil {
		t.Fatalf("no head while the app holds the body: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	first := make([]byte, 5)
	if _, err := io.ReadFull(resp.Body, first); err != nil || string(first) != "hello" || resp.Header.Get(originServerHeader) != "slow-app" {
		t.Fatalf("first bytes %q err %v headers %v, want them before the app finishes", first, err, resp.Header)
	}
	next <- struct{}{}
	rest, err := io.ReadAll(resp.Body)
	if err != nil || string(rest) != "world" {
		t.Fatalf("rest %q err %v, want world", rest, err)
	}
}

// A handler that never writes still answers through net/http after it returns.
func TestOriginServerNamesRenderOnAnEmptyAnswer(t *testing.T) {
	h, err := New(context.Background(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), &Config{OriginServer: true}, "origin-server")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Result().Header.Get(originServerHeader); got != "Render" {
		t.Fatalf("%s = %q, want Render", originServerHeader, got)
	}
}

// The writer flushes as the proxy would: a body of unknown length or an
// event stream at once, a known length, a 204 and a 304 not.
func TestOriginServerFlushesOnlyStreams(t *testing.T) {
	for _, tc := range []struct {
		name          string
		code          int
		contentLength string
		contentType   string
		want          bool
	}{
		{"unknown length", http.StatusOK, "", "application/json", true},
		{"event stream of known length", http.StatusOK, "10", "text/event-stream", true},
		{"known length", http.StatusOK, "2", "text/plain", false},
		{"no content", http.StatusNoContent, "", "", false},
		{"not modified", http.StatusNotModified, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.contentLength != "" {
					w.Header().Set("Content-Length", tc.contentLength)
				}
				if tc.contentType != "" {
					w.Header().Set("Content-Type", tc.contentType)
				}
				w.WriteHeader(tc.code)
			}), &Config{OriginServer: true}, "origin-server")
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			if rec.Flushed != tc.want {
				t.Fatalf("flushed = %v, want %v", rec.Flushed, tc.want)
			}
		})
	}
}

// rawStreamBackend answers one request by hand, as a streaming app does: the
// head, then each chunk only once the test releases it. A raw connection, not
// an http.Handler, so the test also runs under Yaegi (yaegi test), where the
// plugin really runs and an interpreted handler cannot flush.
func rawStreamBackend(t *testing.T, contentType string, chunks []string, next chan struct{}) *url.URL {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		br := bufio.NewReader(conn)
		for {
			line, err := br.ReadString('\n')
			if err != nil || line == "\r\n" {
				break
			}
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nServer: stream-app\r\nContent-Type: "+contentType+"\r\nTransfer-Encoding: chunked\r\n\r\n")
		for _, chunk := range chunks {
			if _, ok := <-next; !ok {
				return
			}
			_, _ = io.WriteString(conn, strconv.FormatInt(int64(len(chunk)), 16)+"\r\n"+chunk+"\r\n")
		}
		<-next
	}()
	return &url.URL{Scheme: "http", Host: ln.Addr().String()}
}

// A streamed answer reaches the client as the app sends it: the head before
// any event, then each chunk before the stream ends, for an event stream and
// for any body of unknown length.
func TestOriginServerKeepsStreaming(t *testing.T) {
	for _, contentType := range []string{"text/event-stream", "application/x-ndjson"} {
		t.Run(contentType, func(t *testing.T) {
			next := make(chan struct{})
			defer close(next)
			edge := originServerEdgeTo(t, rawStreamBackend(t, contentType, []string{"one\n", "two\n"}, next))
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(edge.URL)
			if err != nil {
				t.Fatalf("no head before the first chunk: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.Header.Get(originServerHeader) != "stream-app" || resp.Header.Get("Server") != "" {
				t.Fatalf("headers %v, want the moved Server", resp.Header)
			}
			body := bufio.NewReader(resp.Body)
			for _, want := range []string{"one\n", "two\n"} {
				next <- struct{}{}
				line, err := body.ReadString('\n')
				if err != nil || line != want {
					t.Fatalf("chunk %q err %v, want %q before the stream ends", line, err, want)
				}
			}
		})
	}
}

// A WebSocket upgrade still hijacks through the wrapper and echoes.
func TestOriginServerKeepsWebSocketUpgrades(t *testing.T) {
	edge := originServerEdge(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, rw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Errorf("backend hijack: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = rw.Flush()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(rw, buf); err == nil {
			_, _ = conn.Write(buf)
		}
	}))
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(edge.URL, "http://"), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: app\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade response %v err %v, want 101", resp, err)
	}
	_, _ = io.WriteString(conn, "ping")
	echo := make([]byte, 4)
	if _, err := io.ReadFull(br, echo); err != nil || string(echo) != "ping" {
		t.Fatalf("echo %q err %v, want ping over the upgraded connection", echo, err)
	}
}

// The entrypoint instance is the request sanitizer alone: it never touches a
// response's Server header, so static sites and platform hosts keep theirs.
func TestTheEntrypointInstanceLeavesServerAlone(t *testing.T) {
	h, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "bex-static")
		w.WriteHeader(http.StatusOK)
	}), CreateConfig(), "client-ip-headers")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("Server") != "bex-static" || rec.Header().Get(originServerHeader) != "" {
		t.Fatalf("headers %v, want Server kept and no %s", rec.Header(), originServerHeader)
	}
}
