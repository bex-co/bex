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
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/apps"
)

// w8/026 (A): the global 2 MiB body cap refused every Blueprint between 2 and
// 10 MiB with 413 before the validate handler — and its own 10 MiB cap —
// could run. Only that route is widened; every other route keeps the global
// cap.
func TestRESTBodyLimitWidensOnlyBlueprintValidation(t *testing.T) {
	const global = 2 << 20
	s := &Server{MaxBodyBytes: global}
	var reached int
	handler := s.restBodyLimit(withBodyLimit(global))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reached = len(body)
		w.WriteHeader(http.StatusOK)
	}))
	for _, tc := range []struct {
		path string
		size int
		want int
	}{
		{apps.BlueprintValidationPath, 3 << 20, http.StatusOK},
		{apps.BlueprintValidationPath, (99 << 20) / 10, http.StatusOK},
		{apps.BlueprintValidationPath, apps.MaxBlueprintValidationBodyBytes + 1, http.StatusRequestEntityTooLarge},
		{"/v1/blueprints/deploy", 3 << 20, http.StatusRequestEntityTooLarge},
		{"/v1/services", global + 1, http.StatusRequestEntityTooLarge},
	} {
		reached = 0
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(make([]byte, tc.size))))
		if rec.Code != tc.want || (tc.want == http.StatusOK && reached != tc.size) {
			t.Errorf("POST %s with %d bytes = %d (handler read %d), want %d", tc.path, tc.size, rec.Code, reached, tc.want)
		}
	}
}
