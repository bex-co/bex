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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w10/m377: an unpaged routes or headers list is Render's first page of 20; an
// explicit limit still reaches 100.
func TestRESTUnpagedStaticRulesAreRendersFirstPage(t *testing.T) {
	site := sampleStaticApp("site")
	for i := range 25 {
		site.Spec.Routes = append(site.Spec.Routes, appv1alpha1.StaticRoute{Type: "redirect", Source: fmt.Sprintf("/old-%02d", i), Destination: "/new"})
		site.Spec.Headers = append(site.Spec.Headers, appv1alpha1.StaticHeader{Path: fmt.Sprintf("/p-%02d", i), Name: "X-Test", Value: "v"})
	}
	svc, _ := newService(nil, site)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	count := func(path string) int {
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		var items []json.RawMessage
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &items) != nil {
			t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body)
		}
		return len(items)
	}
	for list, total := range map[string]int{"routes": len(site.Spec.Routes), "headers": len(site.Spec.Headers)} {
		path := "/v1/services/site/" + list
		if got := count(path); got != core.DefaultPageLimit {
			t.Errorf("unpaged %s = %d, want %d", list, got, core.DefaultPageLimit)
		}
		if got := count(path + "?limit=100"); got != total {
			t.Errorf("%s?limit=100 = %d, want all %d", list, got, total)
		}
	}
}
