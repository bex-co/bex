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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// ownerid_test.go covers w6/m2/t004: the explicit ownerId override on top of
// w1/m9's automatic caller-tenant scoping — reuses apps_test.go's tenantApp
// (core.LabelTenant) and fakeWorkspace helpers rather than reinventing them.

// fakeChecker allows every object except one explicitly denied — lets a test
// pass the coarse Authorize(RelCanView) gate (workspace:default) while still
// exercising a specific ownerId's AuthorizeOn as forbidden.
type fakeChecker struct {
	allow bool   // when true and deny=="", every object is allowed
	deny  string // if set, exactly this object is denied; everything else allowed
}

func (c *fakeChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	if c.deny != "" {
		return object != c.deny, nil
	}
	return c.allow, nil
}

func ctxAs(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func TestList_OwnerIDFieldFromLabel(t *testing.T) {
	cl := fakeClient(tenantApp("web", "tea-1"), sampleApp("hand-applied"))
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	list, err := svc.List(ctxAs("user-a"), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byName := map[string]AppView{}
	for _, a := range list {
		byName[a.Name] = a
	}
	if byName["web"].OwnerID != "tea-1" {
		t.Errorf("labeled app: OwnerID = %q, want tea-1", byName["web"].OwnerID)
	}
	if byName["hand-applied"].OwnerID != "" {
		t.Errorf("unlabeled (hand-applied) app: OwnerID = %q, want empty", byName["hand-applied"].OwnerID)
	}
}

func TestList_OwnerIDFilterScopesResults(t *testing.T) {
	cl := fakeClient(tenantApp("web", "tea-1"), tenantApp("api", "tea-2"))
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Authz: &fakeChecker{allow: true}}}

	list, err := svc.List(ctxAs("user-a"), "tea-1")
	if err != nil {
		t.Fatalf("List(tea-1): %v", err)
	}
	if len(list) != 1 || list[0].Name != "web" {
		t.Fatalf("scoped list = %+v, want only web", list)
	}
}

func TestList_OwnerIDFilterForbiddenWhenCallerCantAccess(t *testing.T) {
	cl := fakeClient(tenantApp("web", "tea-1"))
	// The coarse gate (workspace:default) passes; only workspace:tea-2 — the
	// requested scope — is denied. A request scoped to tea-2 must be refused
	// before any App is read, not silently emptied.
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default",
		Authz: &fakeChecker{deny: core.WorkspaceObject("tea-2")}}}

	if _, err := svc.List(ctxAs("user-a"), "tea-2"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("want ErrForbidden for an inaccessible ownerId, got %v", err)
	}
}

// TestREST_ServicesOwnerIDQueryParam wires the ?ownerId= filter all the way
// through the REST route (not just the Service verb) — GET
// /v1/services?ownerId=tea-1 must return only that workspace's services and
// carry ownerId in the response shape.
func TestREST_ServicesOwnerIDQueryParam(t *testing.T) {
	cl := fakeClient(tenantApp("web", "tea-1"), tenantApp("api", "tea-2"))
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Authz: &fakeChecker{allow: true}}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/services?ownerId=tea-1", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/services?ownerId=tea-1: %d body=%s", rec.Code, rec.Body.String())
	}
	var list []serviceWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 || list[0].Service.Name != "web" || list[0].Service.OwnerID != "tea-1" {
		t.Fatalf("scoped REST list = %+v, want only web/tea-1", list)
	}
}
