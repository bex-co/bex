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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// flushErrorWriter flushes only through FlushError, as Traefik's own
// brotli/zstd compression writer does. Compiled-only: under Yaegi an
// interpreted fake hides FlushError from compiled code, while the writer
// Traefik hands the plugin is compiled, so CI's Yaegi run leaves this file
// out.
type flushErrorWriter struct {
	http.ResponseWriter
	flushed int
}

func (w *flushErrorWriter) FlushError() error {
	w.flushed++
	return nil
}

// A stream flushes through any writer net/http can flush, not only an
// http.Flusher.
func TestOriginServerFlushesAFlushErrorWriter(t *testing.T) {
	h, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: one\n\n")
	}), &Config{OriginServer: true}, "origin-server")
	if err != nil {
		t.Fatal(err)
	}
	w := &flushErrorWriter{ResponseWriter: httptest.NewRecorder()}
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.flushed == 0 {
		t.Fatal("the event stream never flushed through FlushError")
	}
}
