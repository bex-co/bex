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

package bex_websocket_egress

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDownstreamCountsFramesButNotHandshake(t *testing.T) {
	server, client := net.Pipe()
	counter := &atomic.Uint64{}
	wrapped := &downstreamConn{Conn: server, counter: counter}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, client)
		close(done)
	}()
	handshake := []byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\n\r\n")
	frame := []byte{0x81, 0x05, 'h', 'e', 'l', 'l', 'o'}
	if _, err := wrapped.Write(append(handshake, frame...)); err != nil {
		t.Fatal(err)
	}
	if got := counter.Load(); got != uint64(len(frame)) {
		t.Fatalf("counter = %d, want frame bytes %d", got, len(frame))
	}
	_ = wrapped.Close()
	_ = client.Close()
	<-done
}

func TestDownstreamCountsFrameWhenHandshakeAlreadyFlushed(t *testing.T) {
	buffer := &recordingConn{}
	counter := &atomic.Uint64{}
	wrapped := &downstreamConn{Conn: buffer, counter: counter}
	frame := []byte{0x82, 0x03, 1, 2, 3}
	if _, err := wrapped.Write(frame); err != nil {
		t.Fatal(err)
	}
	if counter.Load() != uint64(len(frame)) || !bytes.Equal(buffer.Bytes(), frame) {
		t.Fatal("frame was not forwarded and counted exactly once")
	}
}

func TestDownstreamDoesNotCountClientReads(t *testing.T) {
	server, client := net.Pipe()
	counter := &atomic.Uint64{}
	wrapped := &downstreamConn{Conn: server, counter: counter}
	want := []byte("client-to-server")
	go func() { _, _ = client.Write(want) }()
	got := make([]byte, len(want))
	if _, err := io.ReadFull(wrapped, got); err != nil {
		t.Fatal(err)
	}
	if counter.Load() != 0 || !bytes.Equal(got, want) {
		t.Fatal("client bytes were not forwarded unchanged or were counted")
	}
	_ = wrapped.Close()
	_ = client.Close()
}

func TestDownstreamCountsOnlySuccessfullyWrittenBytes(t *testing.T) {
	wantErr := errors.New("short write")
	conn := &shortWriteConn{limit: 3, err: wantErr}
	counter := &atomic.Uint64{}
	wrapped := &downstreamConn{Conn: conn, counter: counter, decided: true}
	written, err := wrapped.Write([]byte("abcdef"))
	if written != 3 || !errors.Is(err, wantErr) {
		t.Fatalf("Write = (%d, %v), want (3, %v)", written, err, wantErr)
	}
	if counter.Load() != 3 {
		t.Fatalf("counter = %d, want 3", counter.Load())
	}
}

func TestMiddlewareDoesNotCountOrdinaryHTTP(t *testing.T) {
	counter := &atomic.Uint64{}
	handler := &middleware{
		next: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte("ordinary HTTP"))
		}),
		counter: counter,
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if counter.Load() != 0 {
		t.Fatalf("ordinary HTTP counter = %d, want 0", counter.Load())
	}
}

func TestMetricsExposeProcessStartForBoundaryResetDetection(t *testing.T) {
	body := metricsBody()
	if !strings.Contains(body, "bex_websocket_meter_process_start_time_seconds ") {
		t.Fatalf("metrics body missing process start: %s", body)
	}
}

func TestMiddlewareWrapsOnlyWebSocketUpgrades(t *testing.T) {
	wrapped := false
	handler := &middleware{
		next: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, wrapped = writer.(*hijackWriter)
		}),
		counter: &atomic.Uint64{},
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodConnect, "/", nil))
	if wrapped {
		t.Fatal("non-WebSocket hijack-capable request was wrapped")
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Upgrade", "websocket")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if !wrapped {
		t.Fatal("WebSocket upgrade request was not wrapped")
	}
}

func TestFailedHijackDoesNotCount(t *testing.T) {
	counter := &atomic.Uint64{}
	wantErr := errors.New("hijack failed")
	writer := &hijackWriter{
		ResponseWriter: failingHijackWriter{ResponseRecorder: httptest.NewRecorder(), err: wantErr},
		counter:        counter,
	}
	if _, _, err := writer.Hijack(); !errors.Is(err, wantErr) {
		t.Fatalf("Hijack error = %v, want %v", err, wantErr)
	}
	if counter.Load() != 0 {
		t.Fatalf("failed hijack counter = %d, want 0", counter.Load())
	}
}

func TestAbruptCloseLeavesCompleteFrameCountDeterministic(t *testing.T) {
	conn := &recordingConn{}
	counter := &atomic.Uint64{}
	wrapped := &downstreamConn{Conn: conn, counter: counter}
	handshake := []byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\n\r\n")
	frame := []byte{0x82, 0x04, 1, 2, 3, 4}
	if _, err := wrapped.Write(append(handshake, frame...)); err != nil {
		t.Fatal(err)
	}
	if err := wrapped.Close(); err != nil {
		t.Fatal(err)
	}
	if counter.Load() != uint64(len(frame)) {
		t.Fatalf("counter after close = %d, want %d", counter.Load(), len(frame))
	}
}

type recordingConn struct{ bytes.Buffer }

func (c *recordingConn) Close() error                       { return nil }
func (c *recordingConn) LocalAddr() net.Addr                { return nil }
func (c *recordingConn) RemoteAddr() net.Addr               { return nil }
func (c *recordingConn) SetDeadline(_ time.Time) error      { return nil }
func (c *recordingConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *recordingConn) SetWriteDeadline(_ time.Time) error { return nil }

type shortWriteConn struct {
	recordingConn
	limit int
	err   error
}

func (c *shortWriteConn) Write(payload []byte) (int, error) {
	_, _ = c.recordingConn.Write(payload[:c.limit])
	return c.limit, c.err
}

type failingHijackWriter struct {
	*httptest.ResponseRecorder
	err error
}

func (w failingHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, w.err
}
