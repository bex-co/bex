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

package egressquery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// codex round-8 #10: the egress instant query — which the usage rollup calls
// per spec — rejects an over-budget body instead of decoding it.
func TestInstantRejectsOversizedResponse(t *testing.T) {
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = 'a'
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		const total = core.MaxUpstreamResponseBytes + 2
		for written := int64(0); written < total; {
			n := int64(len(chunk))
			if remaining := total - written; remaining < n {
				n = remaining
			}
			if _, err := w.Write(chunk[:n]); err != nil {
				return
			}
			written += n
		}
	}))
	defer srv.Close()

	_, err := Instant(context.Background(), srv.Client(), srv.URL, "up", time.Now())
	if !errors.Is(err, core.ErrUpstreamResponseTooLarge) {
		t.Fatalf("oversized instant-query response => %v, want ErrUpstreamResponseTooLarge", err)
	}
}
