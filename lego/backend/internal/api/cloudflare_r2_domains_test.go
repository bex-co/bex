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
	"net/http"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/publichost"
)

func (h *cfHarness) devURL(t *testing.T, method string, body string) *cfReply {
	t.Helper()
	var b []byte
	if body != "" {
		b = []byte(body)
	}
	return h.do(t, method, h.url("/buckets/photos/domains/managed"), b)
}

func TestDevURLEnableDisableAndGet(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	b, err := h.repo.GetObjectBucket(t.Context(), h.ws, "photos")
	if err != nil {
		t.Fatal(err)
	}
	label, _ := publichost.Label(b.ID)
	wantDomain := label + "." + cfTestBaseDomain

	type answer struct {
		BucketID string `json:"bucketId"`
		Domain   string `json:"domain"`
		Enabled  bool   `json:"enabled"`
	}
	check := func(r *cfReply, enabled bool) {
		t.Helper()
		wantOK(t, r)
		got := resultOf[answer](t, r)
		if got != (answer{BucketID: b.ID, Domain: wantDomain, Enabled: enabled}) {
			t.Fatalf("answer = %+v, want bucketId %s, domain %s, enabled %v", got, b.ID, wantDomain, enabled)
		}
	}

	// Private is the default, and the domain is reported either way.
	check(h.devURL(t, http.MethodGet, ""), false)
	check(h.devURL(t, http.MethodPut, `{"enabled":true}`), true)
	check(h.devURL(t, http.MethodGet, ""), true)
	if got, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "photos"); !got.Public {
		t.Fatal("enable did not mark the bucket public")
	}
	// Enabling again is a no-op, as in Cloudflare.
	check(h.devURL(t, http.MethodPut, `{"enabled":true}`), true)
	check(h.devURL(t, http.MethodPut, `{"enabled":false}`), false)
	check(h.devURL(t, http.MethodGet, ""), false)
	if got, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "photos"); got.Public {
		t.Fatal("disable left the bucket public")
	}
}

func TestDevURLRefusesWhatItCannotRead(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	for _, body := range []string{``, `{}`, `{"enabled":"yes"}`, `{"enabled":null}`, `not json`} {
		r := h.devURL(t, http.MethodPut, body)
		wantCF(t, r, http.StatusBadRequest, cfCodeInvalidRequest)
	}
	if got, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "photos"); got.Public {
		t.Fatal("a refused body published the bucket")
	}
	// A missing bucket is 10006, and a method Cloudflare does not have is 7000.
	wantCF(t, h.do(t, http.MethodPut, h.url("/buckets/ghost/domains/managed"), []byte(`{"enabled":true}`)), http.StatusNotFound, cfCodeNoSuchBucket)
	wantCF(t, h.do(t, http.MethodGet, h.url("/buckets/ghost/domains/managed"), nil), http.StatusNotFound, cfCodeNoSuchBucket)
	wantCF(t, h.devURL(t, http.MethodPost, `{"enabled":true}`), http.StatusMethodNotAllowed, cfCodeMethodNotAllow)
	wantCF(t, h.devURL(t, http.MethodDelete, ""), http.StatusMethodNotAllowed, cfCodeMethodNotAllow)
}

// TestDevURLNeedsAHostingDomain: a public bucket's host lives under
// BEX_BASE_DOMAIN, so with none configured the bucket is never marked public.
func TestDevURLNeedsAHostingDomain(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	h.srv.CloudflareR2.baseDomain = ""
	wantCF(t, h.devURL(t, http.MethodPut, `{"enabled":true}`), http.StatusForbidden, cfCodeNotEntitled)
	wantCF(t, h.devURL(t, http.MethodGet, ""), http.StatusForbidden, cfCodeNotEntitled)
	if got, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "photos"); got.Public {
		t.Fatal("a deployment with no hosting domain marked a bucket public")
	}
}

// TestDevURLOnlyAnAdminPublishes: a developer who may write every object cannot
// publish them (ADR112 D6: publishing customer data needs can_manage).
func TestDevURLOnlyAnAdminPublishes(t *testing.T) {
	developer := map[string]bool{core.RelCanView: true, core.RelCanViewSensitive: true, core.RelCanCreate: true}
	c := &relationChecker{held: developer}
	h := newCFHarnessAuthz(t, c)
	c.workspace = h.ws
	h.createBucketDirect("photos")
	wantCF(t, h.devURL(t, http.MethodPut, `{"enabled":true}`), http.StatusForbidden, cfCodeAccessDenied)
	wantCF(t, h.devURL(t, http.MethodPut, `{"enabled":false}`), http.StatusForbidden, cfCodeAccessDenied)
	if got, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "photos"); got.Public {
		t.Fatal("a developer published a bucket")
	}
	wantOK(t, h.devURL(t, http.MethodGet, ""))

	c.held = map[string]bool{core.RelCanView: true, core.RelCanManage: true}
	wantOK(t, h.devURL(t, http.MethodPut, `{"enabled":true}`))
}

// TestDevURLOnASystemBucket: render-<region> can be made public by the same call
// (ADR112 D4); the Render CLI has no way to know.
func TestDevURLOnASystemBucket(t *testing.T) {
	h := newCFHarness(t)
	r := h.do(t, http.MethodPut, h.url("/buckets/render-oregon/domains/managed"), []byte(`{"enabled":true}`))
	wantOK(t, r)
	b, err := h.repo.GetObjectBucket(t.Context(), h.ws, objectstorage.SystemBucketName(objectstorage.Oregon))
	if err != nil || !b.Public {
		t.Fatalf("system bucket = %+v, %v; want public", b, err)
	}
	if got := resultOf[map[string]any](t, r); !strings.HasPrefix(got["domain"].(string), "pub-") {
		t.Fatalf("domain = %v", got["domain"])
	}
}

// TestTheGatewayVerbsAreNotOnThePublicMux: the object gateway's lookup and
// egress calls live on bex-api's cluster-internal listener only, behind their own
// bearer. The public mux, which api.bex.co routes straight to, must not serve
// them, whatever bearer a caller sends.
func TestTheGatewayVerbsAreNotOnThePublicMux(t *testing.T) {
	hydra := newClassHydraScoped(t, "identity-1", "dcr-client", "openid "+core.ScopeSensitive, []string{bexResource}, map[string]bool{"dcr-client": false})
	srv := NewServer(&core.Base{}, Deps{ObjectStorage: newCFHarness(t).svc})
	srv.HydraAdminURL = hydra.url
	srv.OAuthResource = bexResource
	srv.OAuthRequireAudience = true
	srv.OAuthPlatformClients = hydra.platformClientIDs
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/internal/object-gateway/public-bucket?host=pub-x.onbex.test", ""},
		{http.MethodPost, "/internal/object-gateway/egress", `{"bytes":{}}`},
	} {
		for _, token := range []string{"", testToken, "anything"} {
			w := do(t, h, c.method, c.path, token, c.body)
			if w.Code == http.StatusOK || strings.Contains(w.Body.String(), "bucketId") || strings.Contains(w.Body.String(), "accepted") {
				t.Errorf("%s %s on the public mux = %d %s: an internal verb is reachable from the internet", c.method, c.path, w.Code, w.Body)
			}
		}
	}
}
