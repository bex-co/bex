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

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
)

// The attachment upload streams past the buffered JSON body limit (ADR095 D2):
// the handler bounds its own stream, and buffering a 25 MiB body per request
// is exactly what the exemption avoids. Every other POST keeps the limit.
func TestAttachmentUploadBypassesTheBufferedBodyLimit(t *testing.T) {
	const global = 16
	var seen int
	handler := restBodyLimit(withBodyLimit(global))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = len(b)
		w.WriteHeader(http.StatusCreated)
	}))
	body := strings.Repeat("x", global*4)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, agentsessions.AttachmentUploadPath, strings.NewReader(body)))
	if rec.Code != http.StatusCreated || seen != len(body) {
		t.Fatalf("upload: status %d, handler saw %d bytes", rec.Code, seen)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/agent-sessions", strings.NewReader(body)))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("ordinary POST: status %d, want 413", rec.Code)
	}
}
