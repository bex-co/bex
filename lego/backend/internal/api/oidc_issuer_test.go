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
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestOIDCIssuerOwnsItsHost drives the composed handler, every wrapper
// included: on the issuer host only the issuer answers (so /healthz is its
// 404, and no API route is reachable there), and the issuer's paths answer
// nothing on the API host.
func TestOIDCIssuerOwnsItsHost(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{OIDCIssuer: censusOIDCIssuer(t)})
	srv.HydraAdminURL = fakeHydraURL(t)
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	const discovery = "/tea-c0000000000000000000/.well-known/openid-configuration"
	serve := func(host, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}

	// The port is not part of the routing key, as on a dev-N issuer.
	for _, host := range []string{oidcIssuerHost, oidcIssuerHost + ":54020"} {
		w := serve(host, discovery)
		if w.Code != http.StatusOK || !strings.HasPrefix(w.Body.String(), `{"issuer":"https://`+oidcIssuerHost+`/tea-c0000000000000000000",`) {
			t.Errorf("discovery on %s = %d %q", host, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/healthz", "/pricing", "/v1/services", "/graphql"} {
		if w := serve(oidcIssuerHost, path); w.Code != http.StatusNotFound || w.Body.String() != "404 page not found\n" {
			t.Errorf("%s on the issuer host = %d %q, want the issuer's 404", path, w.Code, w.Body.String())
		}
	}
	if w := serve("api.example", discovery); w.Code == http.StatusOK {
		t.Errorf("the issuer answered on the API host: %d %q", w.Code, w.Body.String())
	}
	if w := serve("api.example", "/healthz"); w.Code != http.StatusOK {
		t.Errorf("/healthz on the API host = %d", w.Code)
	}
}
