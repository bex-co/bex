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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w21/093: Render's add-custom-domain returns the domain and its www or apex
// sibling, so the REST route calls the unexported pair add from its closure
// instead of AddDomain. A refusal recorded "apps.registerDomainRoutes.func14";
// it records the verb AddDomain records on every other surface.
func TestRESTAddCustomDomainRecordsAddDomain(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	svc.Authz = &fakeChecker{allow: false}
	sink := &captureAuditSink{}
	svc.Audit = sink
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest("POST", "/v1/services/web/custom-domains", strings.NewReader(`{"name":"app.example.com"}`)).
		WithContext(ctxAs("user-1"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a refused add = %d %s, want 403", rec.Code, rec.Body)
	}

	if len(sink.events) != 1 || sink.events[0].Verb != core.AuditVerbAddDomain || sink.events[0].Outcome != core.AuditDenied {
		t.Fatalf("recorded %+v, want one denied %s", sink.events, core.AuditVerbAddDomain)
	}
	// AddDomain itself still derives the same name.
	sink.events = nil
	if _, err := svc.AddDomain(ctxAs("user-1"), "web", "app.example.com"); err == nil {
		t.Fatal("a refused AddDomain succeeded")
	}
	if len(sink.events) != 1 || sink.events[0].Verb != core.AuditVerbAddDomain {
		t.Fatalf("AddDomain recorded %+v, want %s", sink.events, core.AuditVerbAddDomain)
	}
}
