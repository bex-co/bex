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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Until a Wasabi credential is installed (ADR112 D9), Render's early-access
// object storage answers as it did when it was not offered (ADR018, w6/097):
// each of the four operations says so instead of the router's bare 404, so the
// code can ship before any bucket exists. The region is not looked at either:
// there is nothing to serve it from.
func TestObjectStorageAnswersNotOfferedWithoutACredential(t *testing.T) {
	mux := (&Server{RenderObjects: &renderObjects{}}).restHandler()
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/objects/tea-a/oregon"},
		{http.MethodPut, "/v1/objects/tea-a/oregon/dumps/prices.native"},
		{http.MethodGet, "/v1/objects/tea-a/frankfurt/dumps/prices.native"},
		{http.MethodDelete, "/v1/objects/tea-a/fsn1/a"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"sizeBytes":1}`)))
		var body struct{ Message string }
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != http.StatusNotFound || !strings.Contains(body.Message, objectStorageNotOffered) {
			t.Errorf("%s %s = %d %s, want 404 naming that object storage is not offered", tc.method, tc.path, rec.Code, rec.Body)
		}
	}
}
