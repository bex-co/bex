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

// Package websocketegress is a Traefik local HTTP middleware. It wraps only
// hijacked downstream connections, so ordinary HTTP response bytes remain in
// Traefik's router counter while WebSocket server→client frames get their own
// exact-App counter.
package bex_websocket_egress

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultMetricsAddr = ":9101"
	maxHandshakeBytes  = 64 * 1024
	maxAppCounters     = 16 * 1024
)

type Config struct {
	AppID       string `json:"appId,omitempty"`
	MetricsAddr string `json:"metricsAddr,omitempty"`
}

func CreateConfig() *Config { return &Config{MetricsAddr: defaultMetricsAddr} }

var processState = struct {
	once     sync.Once
	ready    atomic.Bool
	count    atomic.Int64
	counters sync.Map // string app id -> *atomic.Uint64
	started  int64
}{started: time.Now().Unix()}

type middleware struct {
	next    http.Handler
	appID   string
	counter *atomic.Uint64
}

func New(_ context.Context, next http.Handler, config *Config, _ string) (http.Handler, error) {
	if config == nil || config.AppID == "" {
		return nil, fmt.Errorf("websocket egress appId is required")
	}
	if strings.ContainsAny(config.AppID, "\n\r\"") {
		return nil, fmt.Errorf("websocket egress appId contains an unsafe metric-label character")
	}
	addr := config.MetricsAddr
	if addr == "" {
		addr = defaultMetricsAddr
	}
	processState.once.Do(func() {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		processState.ready.Store(true)
		go serveMetrics(listener)
	})
	value, loaded := processState.counters.LoadOrStore(config.AppID, &atomic.Uint64{})
	if !loaded && processState.count.Add(1) > maxAppCounters {
		processState.counters.CompareAndDelete(config.AppID, value)
		processState.count.Add(-1)
		return nil, fmt.Errorf("websocket egress App counter limit reached")
	}
	return &middleware{next: next, appID: config.AppID, counter: value.(*atomic.Uint64)}, nil
}

func (m *middleware) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !strings.EqualFold(strings.TrimSpace(request.Header.Get("Upgrade")), "websocket") {
		m.next.ServeHTTP(writer, request)
		return
	}
	m.next.ServeHTTP(&hijackWriter{ResponseWriter: writer, counter: m.counter}, request)
}

type hijackWriter struct {
	http.ResponseWriter
	counter *atomic.Uint64
}

func (w *hijackWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *hijackWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying response writer does not support hijacking")
	}
	conn, readWriter, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}
	return &downstreamConn{Conn: conn, counter: w.counter}, readWriter, nil
}

type downstreamConn struct {
	net.Conn
	counter  *atomic.Uint64
	mu       sync.Mutex
	decided  bool
	httpHead bool
	pending  []byte
}

// CloseRead and CloseWrite preserve half-close support exposed by TCP
// connections through the wrapper. The fallback matches net.Conn's only
// available close operation for transports without half-close support.
func (c *downstreamConn) CloseRead() error {
	if closer, ok := c.Conn.(interface{ CloseRead() error }); ok {
		return closer.CloseRead()
	}
	return c.Conn.Close()
}

func (c *downstreamConn) CloseWrite() error {
	if closer, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return closer.CloseWrite()
	}
	return c.Conn.Close()
}

func (c *downstreamConn) Write(payload []byte) (int, error) {
	written, err := c.Conn.Write(payload)
	if written > 0 {
		c.account(payload[:written])
	}
	return written, err
}

// account excludes a 101 response header if the reverse proxy emits it on the
// hijacked connection. If net/http flushed the header before Hijack, the first
// bytes are a WebSocket frame and the entire write is counted.
func (c *downstreamConn) account(payload []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.decided && !c.httpHead {
		c.counter.Add(uint64(len(payload)))
		return
	}
	c.pending = append(c.pending, payload...)
	if !c.decided {
		if len(c.pending) < len("HTTP/") {
			return
		}
		c.httpHead = bytes.HasPrefix(c.pending, []byte("HTTP/"))
		c.decided = true
		if !c.httpHead {
			c.counter.Add(uint64(len(c.pending)))
			c.pending = nil
			return
		}
	}
	if len(c.pending) > maxHandshakeBytes {
		// An unbounded/malformed response is not silently treated as frame bytes.
		processState.ready.Store(false)
		c.pending = nil
		return
	}
	if end := bytes.Index(c.pending, []byte("\r\n\r\n")); end >= 0 {
		frameStart := end + 4
		c.counter.Add(uint64(len(c.pending) - frameStart))
		c.pending = nil
		c.httpHead = false
	}
}

// serveMetrics uses a tiny bounded HTTP/1 listener instead of net/http.Server.
// Traefik executes local plugins through Yaegi, whose reflection bridge can
// silently drop writes made by a plugin callback passed to net/http.
func serveMetrics(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			processState.ready.Store(false)
			return
		}
		go serveMetricsConnection(conn)
	}
}

func serveMetricsConnection(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(io.LimitReader(conn, 8*1024))
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return
		}
		if line == "\r\n" {
			break
		}
	}
	if !strings.HasPrefix(requestLine, "GET /metrics ") {
		body := "not found\n"
		_, _ = conn.Write([]byte("HTTP/1.1 404 Not Found\r\nContent-Type: text/plain\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\nConnection: close\r\n\r\n" + body))
		return
	}
	body := metricsBody()
	_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain; version=0.0.4\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\nConnection: close\r\n\r\n" + body))
}

func metricsBody() string {
	var body strings.Builder
	body.WriteString("# HELP bex_websocket_egress_bytes_total WebSocket frame bytes written from an App router to public clients after upgrade.\n")
	body.WriteString("# TYPE bex_websocket_egress_bytes_total counter\n")
	var appIDs []string
	processState.counters.Range(func(key, _ any) bool {
		appIDs = append(appIDs, key.(string))
		return true
	})
	sort.Strings(appIDs)
	for _, appID := range appIDs {
		value, _ := processState.counters.Load(appID)
		body.WriteString("bex_websocket_egress_bytes_total{app_id=" + strconv.Quote(appID) + "} " + strconv.FormatUint(value.(*atomic.Uint64).Load(), 10) + "\n")
	}
	body.WriteString("# HELP bex_websocket_meter_healthy 1 when the per-router hijack wrapper and metrics listener are healthy.\n")
	body.WriteString("# TYPE bex_websocket_meter_healthy gauge\n")
	healthy := 0
	if processState.ready.Load() {
		healthy = 1
	}
	body.WriteString("bex_websocket_meter_healthy " + strconv.Itoa(healthy) + "\n")
	body.WriteString("# HELP bex_websocket_meter_process_start_time_seconds Unix time when this process-local WebSocket counter started.\n")
	body.WriteString("# TYPE bex_websocket_meter_process_start_time_seconds gauge\n")
	body.WriteString("bex_websocket_meter_process_start_time_seconds " + strconv.FormatInt(processState.started, 10) + "\n")
	return body.String()
}
