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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
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

// cacheToggle names a recorded edge_cache_enabled_to: kept (no toggle), on or off.
func cacheToggle(to *bool) string {
	switch {
	case to == nil:
		return "kept"
	case *to:
		return "on"
	default:
		return "off"
	}
}

// saveCacheProfile saves profile on web and returns the one SetCacheProfile
// row's toggle.
func saveCacheProfile(t *testing.T, svc *Service, sink *captureAuditSink, profile string) string {
	t.Helper()
	sink.events = nil
	if _, err := svc.SetCacheProfile(context.Background(), "web", profile); err != nil {
		t.Fatalf("SetCacheProfile(%s): %v", profile, err)
	}
	if n := sink.countVerb(core.AuditVerbSetCacheProfile); n != 1 {
		t.Fatalf("SetCacheProfile(%s) recorded %d rows, want 1", profile, n)
	}
	return cacheToggle(sink.events[0].EdgeCacheEnabledTo)
}

// w9/m345: each profile save records which way it turned the edge cache, the
// fact Render's edge_cache_enabled / edge_cache_disabled name. A save that keeps
// caching on, or off, records no toggle (Render has no event for it), and a
// purge records its own verb.
func TestCacheProfileSavesRecordWhetherCachingTurnedOnOrOff(t *testing.T) {
	svc, _ := newService(nil, cacheApp("web", "standard", ""))
	sink := &captureAuditSink{}
	svc.Audit = sink
	for _, step := range []struct{ profile, want string }{
		{appv1alpha1.CacheProfileNone, "kept"}, // off -> off
		{appv1alpha1.CacheProfileStatic, "on"}, // off -> on
		{appv1alpha1.CacheProfileAll, "kept"},  // on -> on
		{appv1alpha1.CacheProfileNone, "off"},  // on -> off
		{appv1alpha1.CacheProfileAll, "on"},    // off -> on
	} {
		if got := saveCacheProfile(t, svc, sink, step.profile); got != step.want {
			t.Fatalf("save of %s recorded caching %s, want %s", step.profile, got, step.want)
		}
	}

	sink.events = nil
	if _, err := svc.PurgeCache(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	if n := sink.countVerb(core.AuditVerbPurgeCache); n != 1 {
		t.Fatalf("purge recorded %d rows, want 1", n)
	}
}

// A web service on its image's port gets the operator's edge-cache sidecar
// like a configured-port one (AppSpec.ServesEdgeCache), so a profile save on
// it is Render's edge_cache_enabled / _disabled and its cache can be purged.
func TestImagePortsServiceTogglesAndPurgesItsCache(t *testing.T) {
	for _, mode := range []string{appv1alpha1.PortModeWebImageV1, appv1alpha1.PortModeWebImageConfiguredV1} {
		t.Run(mode, func(t *testing.T) {
			app := cacheApp("web", "standard", appv1alpha1.TypeWebService)
			app.Spec.PortMode = mode
			svc, cl := newService(nil, app)
			sink := &captureAuditSink{}
			svc.Audit = sink
			if got := saveCacheProfile(t, svc, sink, appv1alpha1.CacheProfileStatic); got != "on" {
				t.Fatalf("turning caching on for an image-ports service recorded caching %s, want on", got)
			}

			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			sink.events = nil
			if rec := restDo(t, mux, http.MethodPost, "/v1/services/web/cache/purge", ""); rec.Code != http.StatusAccepted {
				t.Fatalf("purge => %d %s, want Render's 202", rec.Code, rec.Body)
			}
			if getApp(t, cl, "web").Annotations[appv1alpha1.CachePurgedAtAnnotation] == "" {
				t.Fatal("purge must stamp the App so the operator restarts the cache")
			}
			if n := sink.countVerb(core.AuditVerbPurgeCache); n != 1 {
				t.Fatalf("purge recorded %d rows, want 1", n)
			}

			if got := saveCacheProfile(t, svc, sink, appv1alpha1.CacheProfileNone); got != "off" {
				t.Fatalf("turning caching off for an image-ports service recorded caching %s, want off", got)
			}
		})
	}
}

// A refused save records nothing for the feed: the verb authorizes under
// deferred audit, so only a save that landed is an event.
func TestRefusedCacheProfileSaveRecordsNoEvent(t *testing.T) {
	svc, _ := newService(nil, cacheApp("web", "free", ""))
	sink := &captureAuditSink{}
	svc.Audit = sink
	if _, err := svc.SetCacheProfile(context.Background(), "web", appv1alpha1.CacheProfileStatic); err == nil {
		t.Fatal("caching on a free web service must be refused")
	}
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbSetCacheProfile && ev.Outcome == core.AuditAllowed {
			t.Fatalf("a refused save recorded an allowed row: %+v", ev)
		}
	}
}
