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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestGraphQLBatchExecutesArrayBody is w4/m100 t002: Apollo BatchHttpLink sends
// a JSON array of operations in one POST so the Metrics page's fan-out shares
// one auth-admission slot instead of N concurrent whoamis.
func TestGraphQLBatchExecutesArrayBody(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{})
	body := `[{"query":"{ services { id } }"},{"query":"{ services { name } }"}]`
	w := do(t, h, http.MethodPost, "/graphql", testToken, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode batch: %v (body %s)", err, w.Body.String())
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	for i, r := range results {
		if errs, ok := r["errors"]; ok && errs != nil {
			t.Errorf("result[%d] errors: %v", i, errs)
		}
		if r["data"] == nil {
			t.Errorf("result[%d] missing data: %v", i, r)
		}
	}
}
