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
	"testing"
)

// w2/m251: Render Build Sources entered the pin before bex serves them
// (w2/m254). Every operation names that instead of answering a bare 404 — also
// through the strict Render gate, even with a body the gate would refuse.
func TestBuildSourcesAnswerNotOffered(t *testing.T) {
	gated, err := newRenderRequestValidator((&Server{}).restHandler())
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range buildSourceRoutes {
		method, path, _ := splitMethodPath(route)
		rec := requestOpenAPITest(t, gated, method, fillWildcards(path, "x"), "application/json", `{"unknown":true}`)
		var body struct{ Message string }
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != http.StatusNotFound || body.Message != buildSourcesNotOffered {
			t.Errorf("%s = %d %s, want 404 %q", route, rec.Code, rec.Body, buildSourcesNotOffered)
		}
	}
}
