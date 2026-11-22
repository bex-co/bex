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

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// wrappingWriter is a middleware's writer between FilterErrorCodes and the
// handler, as statusWriter and the gzip writer are.
type wrappingWriter struct{ http.ResponseWriter }

func (w wrappingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func writtenError(t *testing.T, w http.ResponseWriter, rec *httptest.ResponseRecorder, err error) map[string]any {
	t.Helper()
	WriteErr(w, err)
	var body map[string]any
	if jsonErr := json.Unmarshal(rec.Body.Bytes(), &body); jsonErr != nil {
		t.Fatalf("body %s: %v", rec.Body, jsonErr)
	}
	return body
}

// TestFilterErrorCodesDropsARefusedCode pins w8/411's writer: a code keep
// refuses leaves the body, through any writer that unwraps to the filter, and
// the rest of the envelope (status, message, params) is unchanged. A coded
// error without params answers no `params: null`.
func TestFilterErrorCodesDropsARefusedCode(t *testing.T) {
	onlyRenders := func(code string) bool { return code == "invalid_limit" }
	for _, c := range []struct {
		name       string
		err        error
		status     int
		code       string // "" when no code answers
		wantParams bool
	}{
		{"a refused code", ErrNotFound, http.StatusNotFound, "", false},
		{"a refused code keeps its params", NewBadRequestError("WORKSPACE_RESOURCE_LIMIT", "limited", map[string]any{"limit": 25}), http.StatusBadRequest, "", true},
		{"a kept code", NewBadRequestError("invalid_limit", "bad limit", nil), http.StatusBadRequest, "invalid_limit", false},
		{"a timed-out request", fmt.Errorf("read: %w", context.DeadlineExceeded), http.StatusServiceUnavailable, "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			body := writtenError(t, wrappingWriter{FilterErrorCodes(rec, onlyRenders)}, rec, c.err)
			code, hasCode := body["code"]
			if rec.Code != c.status || hasCode != (c.code != "") || (hasCode && code != c.code) || body["message"] == "" {
				t.Fatalf("answer = %d %v, want %d with code %q", rec.Code, body, c.status, c.code)
			}
			if _, hasParams := body["params"]; hasParams != c.wantParams {
				t.Fatalf("answer = %v, want params %v", body, c.wantParams)
			}
		})
	}

	t.Run("an unfiltered writer keeps every code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if body := writtenError(t, rec, rec, ErrNotFound); body["code"] != NotFoundCode {
			t.Fatalf("answer = %v, want code %s", body, NotFoundCode)
		}
	})

	t.Run("a success never asks, and Flush reaches the writer", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := FilterErrorCodes(rec, func(string) bool { t.Fatal("keep ran on a success"); return true })
		WriteJSON(w, http.StatusOK, map[string]string{"id": "srv-a"})
		w.(http.Flusher).Flush()
		if !rec.Flushed {
			t.Fatal("Flush did not reach the underlying writer")
		}
	})
}
