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
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func cacheApp(name, tier, typ string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Tier, a.Spec.Type = tier, typ
	return a
}

func restDo(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

// Render's serviceDetails.cache.profile reads back on a web service and
// PATCHes as Render's API does (w5/m151).
func TestRESTCacheProfileRoundTripsOnAPaidWebService(t *testing.T) {
	svc, cl := newService(nil, cacheApp("web", "standard", ""))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := restDo(t, mux, http.MethodGet, "/v1/services/web", "")
	var read struct {
		ServiceDetails struct {
			Cache struct{ Profile string } `json:"cache"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &read); err != nil || read.ServiceDetails.Cache.Profile != "no-cache" {
		t.Fatalf("GET cache = %+v (%v), want Render's default no-cache", read.ServiceDetails.Cache, err)
	}
	rec = restDo(t, mux, http.MethodPatch, "/v1/services/web", `{"serviceDetails":{"cache":{"profile":"origin-controlled-all"}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH cache => %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.CacheProfile; got != appv1alpha1.CacheProfileAll {
		t.Fatalf("spec.cacheProfile = %q, want origin-controlled-all", got)
	}
}

func TestRESTCacheProfileRefusesWhatRenderRefuses(t *testing.T) {
	for _, tc := range []struct {
		name, tier, typ, profile, want string
	}{
		{"a free web service", "free", "", "origin-controlled", "not available for free web services"},
		{"a background worker", "standard", appv1alpha1.TypeBackgroundWorker, "origin-controlled", "only for web services"},
		{"an unknown profile", "standard", "", "aggressive", "cache.profile must be one of"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil, cacheApp("web", tc.tier, tc.typ))
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := restDo(t, mux, http.MethodPatch, "/v1/services/web", `{"serviceDetails":{"cache":{"profile":"`+tc.profile+`"}}}`)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("PATCH => %d %s, want a 400 saying %q", rec.Code, rec.Body, tc.want)
			}
			if getApp(t, cl, "web").Spec.CacheProfile != "" {
				t.Fatal("a refused PATCH must not change the profile")
			}
		})
	}
}

func TestRESTPurgeCacheStampsACachingServiceAndRefusesOneThatDoesNotCache(t *testing.T) {
	caching := cacheApp("web", "standard", "")
	caching.Spec.CacheProfile = appv1alpha1.CacheProfileStatic
	svc, cl := newService(nil, caching, cacheApp("plain", "standard", ""))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := restDo(t, mux, http.MethodPost, "/v1/services/web/cache/purge", "")
	if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
		t.Fatalf("purge => %d %q, want Render's 202 with no body", rec.Code, rec.Body)
	}
	if getApp(t, cl, "web").Annotations[appv1alpha1.CachePurgedAtAnnotation] == "" {
		t.Fatal("purge must stamp the App so the operator restarts the cache")
	}
	if rec := restDo(t, mux, http.MethodPost, "/v1/services/plain/cache/purge", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("purge of a service that does not cache => %d, want 400", rec.Code)
	}
}
