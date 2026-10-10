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

package apps

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestListNameFilterKeepsEncodedCommas replays the raw queries the pinned CLI's
// generated NewListServicesRequest emits (pinned in
// lego/cli/name_filter_encoding_contract_test.go) against the composed list
// endpoint. The CLI resolves `services update <name>` and `services create
// --from <name>` through this list and keeps only exact-name matches, so a
// comma name that the server split into fragments was "No service named …"
// (w8/m55). Fragment-named decoys prove the split never matches by accident.
func TestListNameFilterKeepsEncodedCommas(t *testing.T) {
	svc, _ := newService(nil,
		renamedApp("comma", appv1alpha1.TypeStaticSite, "qa-literal,雪%+&="),
		renamedApp("frag-a", appv1alpha1.TypeStaticSite, "qa-literal"),
		renamedApp("frag-b", appv1alpha1.TypeStaticSite, "雪%+&="),
		renamedApp("first", appv1alpha1.TypeWebService, "qa-first"),
		renamedApp("second", appv1alpha1.TypeStaticSite, "qa-second"),
		renamedApp("pct", appv1alpha1.TypeStaticSite, "qa-literal%2C"),
	)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, tc := range []struct {
		query string
		want  []string // immutable names, list order
	}{
		{"name=qa-literal%2C%E9%9B%AA%25%2B%26%3D", []string{"comma"}},
		{"name=qa-first,qa-second", []string{"first", "second"}},
		{"name=qa-first,qa-literal%2C%E9%9B%AA%25%2B%26%3D", []string{"comma", "first"}},
		{"name=qa-literal%252C", []string{"pct"}},
		{"name=qa-first&name=qa-second&name=qa-first", []string{"first", "second"}},
		{"name=qa-first,qa-second&type=static_site", []string{"second"}},
		{"name=no-such-name", nil},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services?"+tc.query, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("?%s => %d: %s", tc.query, rec.Code, rec.Body)
		}
		var page []struct {
			Service struct {
				ImmutableName string `json:"immutableName"`
			} `json:"service"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, p := range page {
			got = append(got, p.Service.ImmutableName)
		}
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("?%s matched %q, want %q", tc.query, got, tc.want)
		}
	}
}
