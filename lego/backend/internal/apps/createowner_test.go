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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// createowner_test.go covers w6/m14's write half: `ownerId` decides which
// workspace a CREATE lands in, on all three surfaces, and a workspace the caller
// does not belong to is refused rather than silently swapped for their own.
// (ownerid_test.go covers the read half — the list filter, w6/m2.)

// twoWorkspaces is the multi-workspace caller this milestone exists for: dana
// belongs to tea-1 (older — her default) and tea-2.
type twoWorkspaces map[string][]string

func (w twoWorkspaces) Tenant(_ context.Context, id core.Identity) (string, bool) {
	ws := w[id.Subject]
	if len(ws) == 0 {
		return "", false
	}
	return ws[0], true
}

func (w twoWorkspaces) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	for _, t := range w[id.Subject] {
		if t == tenantID {
			return true, nil
		}
	}
	return false, nil
}

func danaService(objs ...client.Object) *Service {
	return &Service{Base: &core.Base{
		Client:    fakeClient(objs...),
		Namespace: "default",
		Workspace: twoWorkspaces{"dana": {"tea-1", "tea-2"}},
		Authz:     &fakeChecker{allow: true},
	}}
}

// createdApp reads back the App CR the verb wrote, so the assertions are about
// what actually landed in the cluster, not what the view echoed.
func createdApp(t *testing.T, s *Service, name string) *appv1alpha1.App {
	t.Helper()
	a, err := s.GetApp(core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"}), core.RelCanView, name)
	if err != nil {
		t.Fatalf("reading back %s: %v", name, err)
	}
	return a
}

// createdTenantApp reads back the App CR belonging to exactly tenantID,
// bypassing GetApp's cross-workspace search entirely — needed once two of
// dana's workspaces can share a name (w4/m19): a plain createdApp(t, svc,
// "web") would be ambiguous about which "web" it means. Tries the tenant-
// scoped object name (core.CRName) first, then the bare name IF it belongs to
// tenantID — a fixture seeded with tenantApp (pre-migration bare-named shape)
// is found the second way.
func createdTenantApp(t *testing.T, s *Service, tenantID, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := s.Client.Get(context.Background(), client.ObjectKey{Namespace: s.Namespace, Name: core.CRName(tenantID, name)}, &a); err == nil {
		return &a
	}
	if err := s.Client.Get(context.Background(), client.ObjectKey{Namespace: s.Namespace, Name: name}, &a); err == nil && a.Labels[core.LabelTenant] == tenantID {
		return &a
	}
	t.Fatalf("reading back %s/%s: not found under either object-naming scheme", tenantID, name)
	return nil
}

func TestCreate_OwnerIDLandsInTheNamedWorkspace(t *testing.T) {
	svc := danaService()

	view, err := svc.Create(ctxAs("dana"), CreateRequest{Name: "web", Image: "nginx", OwnerID: "tea-2"})
	if err != nil {
		t.Fatalf("Create(ownerId=tea-2): %v", err)
	}
	if view.OwnerID != "tea-2" {
		t.Errorf("returned ownerId = %q, want tea-2", view.OwnerID)
	}
	if got := createdApp(t, svc, "web").Labels[core.LabelTenant]; got != "tea-2" {
		t.Errorf("App tenant label = %q, want tea-2 — the create must land in the NAMED workspace, not the caller's default (tea-1)", got)
	}
}

// TestCreate_ResponseNameIsThePublicNameNotTheCRName guards w4/m19's
// tenant-prefixed object naming (core.CRName) against leaking through the
// create response: the App CR is object-named "tea-2-web" so two workspaces
// can both own "web", but the AppView the caller sees back — which REST/
// GraphQL both use verbatim as the service "id" (rest.go, graphql.go's "id"
// resolver) — must read "web", the name dana actually typed, never the
// internal composite. A client round-tripping that id into a later GET must
// also resolve, which it does BECAUSE it is the public name: GetApp's first
// candidate is CRName(acting, "web") = the real object name.
func TestCreate_ResponseNameIsThePublicNameNotTheCRName(t *testing.T) {
	svc := danaService()

	view, err := svc.Create(ctxAs("dana"), CreateRequest{Name: "web", Image: "nginx", OwnerID: "tea-2"})
	if err != nil {
		t.Fatalf("Create(ownerId=tea-2): %v", err)
	}
	if view.Name != "web" {
		t.Fatalf("create response name = %q, want the public name %q, not the tenant-prefixed CR name", view.Name, "web")
	}

	got, err := svc.Get(ctxAs("dana"), view.Name)
	if err != nil {
		t.Fatalf("round-tripping the returned id %q: %v", view.Name, err)
	}
	if got.Name != "web" {
		t.Errorf("round-tripped name = %q, want web", got.Name)
	}
}

func TestCreate_NoOwnerIDLandsInTheDefaultWorkspace(t *testing.T) {
	svc := danaService()

	if _, err := svc.Create(ctxAs("dana"), CreateRequest{Name: "web", Image: "nginx"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := createdApp(t, svc, "web").Labels[core.LabelTenant]; got != "tea-1" {
		t.Errorf("App tenant label = %q, want tea-1 — an omitted ownerId means the caller's DEFAULT (oldest) workspace", got)
	}
}

func TestCreate_OwnerIDOfANonMemberWorkspaceIsForbidden(t *testing.T) {
	svc := danaService()

	// Authorization is wide open (fakeChecker{allow:true}) — membership alone
	// must refuse this, so an OpenFGA gap can't turn into a cross-tenant create.
	_, err := svc.Create(ctxAs("dana"), CreateRequest{Name: "web", Image: "nginx", OwnerID: "tea-stranger"})
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Create into a workspace dana doesn't belong to: got %v, want ErrForbidden", err)
	}
	// And nothing was written anywhere — not even into her own workspace.
	var list appv1alpha1.AppList
	if err := svc.Client.List(context.Background(), &list); err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Items) != 0 {
		t.Errorf("a refused create wrote %d App(s): %+v — it must never fall back to the caller's own workspace", len(list.Items), list.Items)
	}
}

// TestGetService_InTheOtherWorkspaceIsNotForbidden is the w6/m11 field bug at
// the feature level: dana creates in tea-2, then reads it back with no ownerId
// (implicit resolution picks tea-1). Before w6/m14 this 403'd her own service.
func TestGetService_InTheOtherWorkspaceIsNotForbidden(t *testing.T) {
	svc := danaService()
	if _, err := svc.Create(ctxAs("dana"), CreateRequest{Name: "web", Image: "nginx", OwnerID: "tea-2"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Get(ctxAs("dana"), "web")
	if err != nil {
		t.Fatalf("Get on her own service in tea-2: %v — an owner must never be 403'd by which of her workspaces the implicit resolution picked", err)
	}
	if got.OwnerID != "tea-2" {
		t.Errorf("ownerId = %q, want tea-2", got.OwnerID)
	}
}

// --- REST (t003) ---

func TestREST_CreateHonorsOwnerIDInTheBody(t *testing.T) {
	svc := danaService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","image":{"imagePath":"nginx"},"ownerId":"tea-2"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/services", strings.NewReader(body))
	mux.ServeHTTP(rec, req.WithContext(ctxAs("dana")))

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/services: %d body=%s", rec.Code, rec.Body.String())
	}
	var out serviceAndDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Service.OwnerID != "tea-2" {
		t.Errorf("response ownerId = %q, want tea-2", out.Service.OwnerID)
	}
	if got := createdApp(t, svc, "web").Labels[core.LabelTenant]; got != "tea-2" {
		t.Errorf("App tenant label = %q, want tea-2", got)
	}
}

func TestREST_CreateAcceptsOfficialCLIImageOwnerID(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		wantStatus int
		wantOwner  string
	}{
		{
			name:       "blank nested owner inherits top-level owner",
			body:       `{"name":"web","ownerId":"tea-2","type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":""}}`,
			wantStatus: http.StatusCreated,
			wantOwner:  "tea-2",
		},
		{
			name:       "nested owner can confirm the default owner",
			body:       `{"name":"web","type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":"tea-1"}}`,
			wantStatus: http.StatusCreated,
			wantOwner:  "tea-1",
		},
		{
			name:       "nested owner cannot select another workspace",
			body:       `{"name":"web","type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":"tea-2"}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "conflicting owner fields are rejected",
			body:       `{"name":"web","ownerId":"tea-1","type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":"tea-2"}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nested non-member owner is forbidden",
			body:       `{"name":"web","ownerId":"tea-stranger","type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":"tea-stranger"}}`,
			wantStatus: http.StatusForbidden,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := danaService()
			mux := http.NewServeMux()
			svc.RegisterREST(mux)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(tc.body))
			mux.ServeHTTP(rec, req.WithContext(ctxAs("dana")))
			if rec.Code != tc.wantStatus {
				t.Fatalf("POST = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			var list appv1alpha1.AppList
			if err := svc.Client.List(context.Background(), &list); err != nil {
				t.Fatal(err)
			}
			if tc.wantStatus != http.StatusCreated {
				if len(list.Items) != 0 {
					t.Fatalf("rejected request wrote %d App(s)", len(list.Items))
				}
				if tc.wantStatus == http.StatusBadRequest && !strings.Contains(rec.Body.String(), "image.ownerId") {
					t.Fatalf("conflict response does not name image.ownerId: %s", rec.Body.String())
				}
				return
			}
			if len(list.Items) != 1 || list.Items[0].Labels[core.LabelTenant] != tc.wantOwner {
				t.Fatalf("created Apps = %#v, want one owned by %s", list.Items, tc.wantOwner)
			}
		})
	}
}

func TestREST_CreateWithANonMemberOwnerIDIs403(t *testing.T) {
	svc := danaService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","image":{"imagePath":"nginx"},"ownerId":"tea-stranger"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/services", strings.NewReader(body))
	mux.ServeHTTP(rec, req.WithContext(ctxAs("dana")))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /v1/services with a non-member ownerId: %d body=%s, want 403", rec.Code, rec.Body.String())
	}
}

func TestREST_CreateResponseContainsDeployIDWhenStoreActive(t *testing.T) {
	// Render's POST /v1/services returns {service, deployId} (serviceAndDeploy).
	// When the control-plane store is active it mints a deploy row and the
	// response should carry its id; without a store the field is omitted.
	rec := &recordingStore{}
	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","image":{"imagePath":"nginx"}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/services", strings.NewReader(body))
	mux.ServeHTTP(rr, req.WithContext(core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})))

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /v1/services: %d body=%s", rr.Code, rr.Body.String())
	}
	var out serviceAndDeploy
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.DeployID != "dep-test" {
		t.Errorf("deployId = %q, want dep-test", out.DeployID)
	}
	if out.Service.ID == "" {
		t.Errorf("service.id missing")
	}
}

// --- GraphQL (t004) ---

// gqlSchema builds the one-feature schema the mutation tests execute against —
// the same fragment the composition root merges into the real schema.
func gqlSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return schema
}

func TestGraphQL_CreateServiceHonorsOwnerID(t *testing.T) {
	svc := danaService()
	res := graphql.Do(graphql.Params{
		Schema:        gqlSchema(t, svc),
		Context:       ctxAs("dana"),
		RequestString: `mutation { createService(name: "web", image: "nginx", ownerId: "tea-2") { id ownerId } }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	if got := createdApp(t, svc, "web").Labels[core.LabelTenant]; got != "tea-2" {
		t.Errorf("App tenant label = %q, want tea-2", got)
	}
}

func TestGraphQL_CreateServiceWithANonMemberOwnerIDErrors(t *testing.T) {
	svc := danaService()
	res := graphql.Do(graphql.Params{
		Schema:        gqlSchema(t, svc),
		Context:       ctxAs("dana"),
		RequestString: `mutation { createService(name: "web", image: "nginx", ownerId: "tea-stranger") { id } }`,
	})
	if len(res.Errors) == 0 {
		t.Fatal("createService into a non-member workspace: no error, want forbidden")
	}
	if !strings.Contains(strings.ToLower(res.Errors[0].Message), "forbidden") {
		t.Errorf("error = %q, want a forbidden error", res.Errors[0].Message)
	}
}

// TestCreate_NameTakenByAnotherWorkspaceStillSucceeds is w4/m19's whole point:
// a name is claimed per workspace, not platform-wide, so `ownerId: tea-2` on a
// name already used in tea-1 must create tea-2's OWN "web" — leaving tea-1's
// untouched — never silently redeploy tea-1's service (the bug w6/m14 nearly
// shipped, back when GetApp's cross-workspace serving doubled as the create
// path's existence probe) and never refuse outright either (the milestone this
// test now guards: two workspaces coexisting under the same name).
func TestCreate_NameTakenByAnotherWorkspaceStillSucceeds(t *testing.T) {
	svc := danaService(tenantApp("web", "tea-1")) // "web" already exists in tea-1
	before := createdTenantApp(t, svc, "tea-1", "web").Spec.Image

	view, err := svc.Create(ctxAs("dana"), CreateRequest{Name: "web", Image: "nginx:2", OwnerID: "tea-2"})
	if err != nil {
		t.Fatalf("Create(name=web, ownerId=tea-2) with web already in tea-1: %v, want success", err)
	}
	if view.OwnerID != "tea-2" {
		t.Errorf("returned ownerId = %q, want tea-2", view.OwnerID)
	}
	tea1 := createdTenantApp(t, svc, "tea-1", "web")
	if tea1.Labels[core.LabelTenant] != "tea-1" || tea1.Spec.Image != before {
		t.Errorf("tea-1's App was mutated by a create aimed at tea-2: tenant=%q image=%q (was %q)",
			tea1.Labels[core.LabelTenant], tea1.Spec.Image, before)
	}
	tea2 := createdTenantApp(t, svc, "tea-2", "web")
	if tea2.Labels[core.LabelTenant] != "tea-2" || tea2.Spec.Image != "nginx:2" {
		t.Errorf("tea-2's App = %+v, want tenant=tea-2 image=nginx:2", tea2)
	}
}

// The same name in the SAME workspace is a conflict, not an update-in-place —
// Create never redeploys (w4/m19); Deploy and push-to-deploy own that.
func TestCreate_SameNameSameWorkspaceIsAConflict(t *testing.T) {
	svc := danaService(tenantApp("web", "tea-1"))

	_, err := svc.Create(ctxAs("dana"), CreateRequest{Name: "web", Image: "nginx:2", OwnerID: "tea-1"})
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("re-create in the SAME workspace: got %v, want ErrConflict", err)
	}
	if got := createdTenantApp(t, svc, "tea-1", "web").Spec.Image; got == "nginx:2" {
		t.Error("a rejected create must not have applied its image to the existing App")
	}
}
