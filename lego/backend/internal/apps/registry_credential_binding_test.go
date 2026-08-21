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
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRegistryCredentialCoreCreateSetAndClear(t *testing.T) {
	credentialID := "rgc-primary"
	rc := &fakePullSecrets{name: "web-registry-pull", ok: true}
	svc, cl := rcService(rc)

	created, err := svc.Create(context.Background(), CreateRequest{
		Name: "web", Image: "ghcr.io/acme/private:1", RegistryCredentialID: &credentialID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.RegistryCredentialID == nil || *created.RegistryCredentialID != credentialID || rc.lastID == nil || *rc.lastID != credentialID {
		t.Fatalf("create binding = view %#v resolver %#v", created.RegistryCredentialID, rc.lastID)
	}
	if got := getApp(t, cl, "web"); got.Spec.RegistryCredentialID == nil || *got.Spec.RegistryCredentialID != credentialID || got.Spec.ExternalRegistryPullSecret != "web-registry-pull" {
		t.Fatalf("created spec = %+v", got.Spec)
	}

	second := "rgc-secondary"
	updated, err := svc.SetRegistryCredential(context.Background(), "web", second)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RegistryCredentialID == nil || *updated.RegistryCredentialID != second {
		t.Fatalf("changed binding = %#v", updated.RegistryCredentialID)
	}

	rc.ok = false
	cleared, err := svc.SetRegistryCredential(context.Background(), "web", "")
	if err != nil {
		t.Fatal(err)
	}
	if cleared.RegistryCredentialID == nil || *cleared.RegistryCredentialID != "" {
		t.Fatalf("clear must persist explicit empty binding, got %#v", cleared.RegistryCredentialID)
	}
	got := getApp(t, cl, "web")
	if got.Spec.ExternalRegistryPullSecret != "" {
		t.Fatalf("clear left pull secret reference %q", got.Spec.ExternalRegistryPullSecret)
	}
}

func TestRegistryCredentialCreateRefusesResolutionFailuresBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "unknown", err: core.ErrNotFound},
		{name: "foreign", err: core.ErrForbidden},
		{name: "host-mismatch", err: core.ErrBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := "rgc-" + tc.name
			for _, req := range []CreateRequest{
				{Name: "image", Image: "ghcr.io/acme/private:1", RegistryCredentialID: &id},
				{Name: "docker", Repo: "https://github.com/octo/app", Runtime: "docker", RegistryCredentialID: &id},
			} {
				rc := &fakePullSecrets{err: tc.err}
				svc, cl := rcService(rc)
				_, err := svc.Create(context.Background(), req)
				if !errors.Is(err, tc.err) {
					t.Fatalf("%s create error = %v, want %v", req.Name, err, tc.err)
				}
				var app appv1alpha1.App
				if getErr := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: req.Name}, &app); !apierrors.IsNotFound(getErr) {
					t.Fatalf("failed %s create wrote App: %v", req.Name, getErr)
				}
			}
		})
	}
}

func TestRegistryCredentialDockerBuildRejectsNonDockerRuntime(t *testing.T) {
	id := "rgc-one"
	for _, req := range []CreateRequest{
		{Name: "native", Repo: "https://github.com/octo/app", Runtime: "node", BuildCommand: "npm ci", StartCommand: "npm start", RegistryCredentialID: &id},
		{Name: "buildpack", Repo: "https://github.com/octo/app", Builder: "buildpack", RegistryCredentialID: &id},
	} {
		svc, _ := rcService(&fakePullSecrets{name: "should-not-materialize", ok: true})
		_, err := svc.Create(context.Background(), req)
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "registryCredentialId") {
			t.Fatalf("%s create error = %v, want named bad request", req.Name, err)
		}
	}
}

func TestRESTRegistryCredentialFailureClassificationAndWireValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "unknown", err: core.ErrNotFound, status: http.StatusNotFound},
		{name: "foreign", err: core.ErrForbidden, status: http.StatusForbidden},
		{name: "host-mismatch", err: core.ErrBadRequest, status: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bodies := []string{
				`{"name":"image","type":"web_service","image":{"imagePath":"ghcr.io/acme/private:1","registryCredentialId":"rgc-one"}}`,
				`{"name":"docker","type":"web_service","repo":"https://github.com/octo/app","serviceDetails":{"runtime":"docker","envSpecificDetails":{"registryCredentialId":"rgc-one"}}}`,
			}
			for _, body := range bodies {
				svc, _ := rcService(&fakePullSecrets{err: tc.err})
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
				if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.err.Error()) {
					t.Fatalf("POST %s = %d: %s, want %d with %q", body, rec.Code, rec.Body.String(), tc.status, tc.err)
				}
			}
		})
	}

	for _, body := range []string{
		`{"name":"web","type":"web_service","image":{"imagePath":"ghcr.io/acme/private:1","registryCredentialId":null}}`,
		`{"name":"web","type":"web_service","image":{"imagePath":"ghcr.io/acme/private:1","registryCredentialId":7}}`,
		`{"name":"web","type":"web_service","image":{"imagePath":"ghcr.io/acme/private:1","registryCredentialId":"rgc-one"},"serviceDetails":{"runtime":"docker","envSpecificDetails":{"registryCredentialId":"rgc-two"}}}`,
	} {
		svc, _ := rcService(&fakePullSecrets{name: "web-registry-pull", ok: true})
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("invalid registryCredentialId POST = %d: %s", rec.Code, rec.Body.String())
		}
	}
}

func TestRESTRegistryCredentialCreatePatchAndClear(t *testing.T) {
	rc := &fakePullSecrets{name: "web-registry-pull", ok: true, credentialName: "Private GHCR"}
	svc, cl := rcService(rc)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(`{"name":"web","type":"web_service","image":{"imagePath":"ghcr.io/acme/private:1","registryCredentialId":"rgc-one"}}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
	}
	var created serviceAndDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Service.RegistryCredentialID != "rgc-one" {
		t.Fatalf("POST response = %s, err %v", rec.Body.String(), err)
	}
	if created.Service.RegistryCredential == nil || created.Service.RegistryCredential.ID != "rgc-one" || created.Service.RegistryCredential.Name != "Private GHCR" {
		t.Fatalf("POST registry credential summary = %+v", created.Service.RegistryCredential)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(`{"image":{"imagePath":"ghcr.io/acme/private:2","registryCredentialId":"rgc-two"}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH set = %d: %s", rec.Code, rec.Body.String())
	}
	got := getApp(t, cl, "web")
	if got.Spec.RegistryCredentialID == nil || *got.Spec.RegistryCredentialID != "rgc-two" || got.Spec.Image != "ghcr.io/acme/private:2" {
		t.Fatalf("PATCH set spec = %+v", got.Spec)
	}

	rc.ok = false
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(`{"image":{"imagePath":"ghcr.io/acme/private:2","registryCredentialId":""}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH clear = %d: %s", rec.Code, rec.Body.String())
	}
	got = getApp(t, cl, "web")
	if got.Spec.RegistryCredentialID == nil || *got.Spec.RegistryCredentialID != "" || got.Spec.ExternalRegistryPullSecret != "" {
		t.Fatalf("PATCH clear spec = %+v", got.Spec)
	}
}

func TestRESTDockerBuildRegistryCredentialCreatePatchAndEcho(t *testing.T) {
	rc := &fakePullSecrets{name: "web-registry-pull", ok: true, credentialName: "Private base registry"}
	svc, cl := rcService(rc)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(`{
		"name":"web","type":"web_service","repo":"https://github.com/octo/app",
		"serviceDetails":{"runtime":"docker","envSpecificDetails":{"registryCredentialId":"rgc-one"}}
	}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
	}
	assertDockerBuildCredentialEcho(t, rec.Body.Bytes(), "rgc-one")
	if got := getApp(t, cl, "web"); got.Spec.RegistryCredentialID == nil || *got.Spec.RegistryCredentialID != "rgc-one" || got.Spec.ExternalRegistryPullSecret != "web-registry-pull" {
		t.Fatalf("POST spec = %+v", got.Spec)
	}
	if rc.lastImage != "" {
		t.Fatalf("Docker-build materialization image = %q, want empty", rc.lastImage)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(`{
		"serviceDetails":{"envSpecificDetails":{"registryCredentialId":"rgc-two"}}
	}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body.String())
	}
	assertDockerBuildCredentialEcho(t, rec.Body.Bytes(), "rgc-two")
	if got := getApp(t, cl, "web"); got.Spec.RegistryCredentialID == nil || *got.Spec.RegistryCredentialID != "rgc-two" {
		t.Fatalf("PATCH spec = %+v", got.Spec)
	}
}

func TestRestServicesBatchesCredentialLookups(t *testing.T) {
	// Three apps: two different credential ids, one repeated. Verifies that
	// ResolveCredentialNames is called exactly once (batch), not per-app (N+1).
	idA := "rgc-alpha"
	idB := "rgc-beta"
	rc := &fakePullSecrets{
		credNames: map[string]string{idA: "Alpha Registry", idB: "Beta Registry"},
	}
	svc, _ := rcService(rc)
	ctx := context.Background()

	apps := []AppView{
		{Name: "web-1", RegistryCredentialID: &idA},
		{Name: "web-2", RegistryCredentialID: &idB},
		{Name: "web-3", RegistryCredentialID: &idA}, // duplicate — must not add a second query
	}
	rendered := svc.restServices(ctx, apps)

	if rc.resolveCalls != 1 {
		t.Fatalf("ResolveCredentialNames called %d times, want 1 (batch)", rc.resolveCalls)
	}
	wantNames := []string{"Alpha Registry", "Beta Registry", "Alpha Registry"}
	for i, svc := range rendered {
		if svc.RegistryCredential == nil {
			t.Errorf("rendered[%d].RegistryCredential = nil, want non-nil", i)
			continue
		}
		if svc.RegistryCredential.Name != wantNames[i] {
			t.Errorf("rendered[%d] name = %q, want %q", i, svc.RegistryCredential.Name, wantNames[i])
		}
	}
}

// TestRestServicesScopesCredentialLookupByWorkspace proves the security-audit
// run-1 hardening: registry-credential display-name enrichment is resolved
// against each App's OWN workspace (OwnerID), never a shared unscoped lookup, so
// a credential id can only ever resolve within its owning tenant.
func TestRestServicesScopesCredentialLookupByWorkspace(t *testing.T) {
	idA := "rgc-alpha"
	idB := "rgc-beta"
	rc := &fakePullSecrets{credNames: map[string]string{idA: "Alpha", idB: "Beta"}}
	svc, _ := rcService(rc)
	ctx := context.Background()

	// Same workspace: one batch call, scoped to that workspace.
	svc.restServices(ctx, []AppView{
		{Name: "web-1", OwnerID: "tea-a", RegistryCredentialID: &idA},
		{Name: "web-2", OwnerID: "tea-a", RegistryCredentialID: &idB},
	})
	if rc.resolveCalls != 1 || rc.lastResolveWorkspace != "tea-a" {
		t.Fatalf("same-workspace: calls=%d lastWorkspace=%q, want 1 / tea-a", rc.resolveCalls, rc.lastResolveWorkspace)
	}

	// Different workspaces: one scoped call per workspace — a credential id is
	// never resolved against a foreign tenant's set.
	rc.resolveCalls = 0
	svc.restServices(ctx, []AppView{
		{Name: "web-1", OwnerID: "tea-a", RegistryCredentialID: &idA},
		{Name: "web-2", OwnerID: "tea-b", RegistryCredentialID: &idB},
	})
	if rc.resolveCalls != 2 {
		t.Fatalf("cross-workspace: calls=%d, want 2 (per-workspace scoping)", rc.resolveCalls)
	}
}

func assertDockerBuildCredentialEcho(t *testing.T, raw []byte, want string) {
	t.Helper()
	var response serviceAndDeploy
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	service := response.Service
	if service.ID == "" { // PATCH/GET return the service directly; POST wraps it.
		if err := json.Unmarshal(raw, &service); err != nil {
			t.Fatal(err)
		}
	}
	details, ok := service.ServiceDetails["envSpecificDetails"].(map[string]any)
	if !ok || details["registryCredentialId"] != want {
		t.Fatalf("envSpecificDetails = %#v, want registryCredentialId %q; response=%s", details, want, raw)
	}
	if service.RegistryCredentialID != want || service.RegistryCredential == nil || service.RegistryCredential.ID != want {
		t.Fatalf("credential echo = id %q summary %+v", service.RegistryCredentialID, service.RegistryCredential)
	}
}

func TestRESTDockerBuildRegistryCredentialRejectsNonDockerRuntime(t *testing.T) {
	svc, _ := rcService(&fakePullSecrets{name: "should-not-materialize", ok: true})
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(`{
		"name":"native","type":"web_service","repo":"https://github.com/octo/app",
		"serviceDetails":{"runtime":"node","envSpecificDetails":{"buildCommand":"npm ci","startCommand":"npm start","registryCredentialId":"rgc-one"}}
	}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "registryCredentialId") {
		t.Fatalf("POST = %d: %s, want named 400", rec.Code, rec.Body.String())
	}
}

func TestGraphQLRegistryCredentialCreateAndUpdate(t *testing.T) {
	rc := &fakePullSecrets{name: "web-registry-pull", ok: true}
	svc, cl := rcService(rc)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { createService(name:"web", image:"ghcr.io/acme/private:1", registryCredentialId:"rgc-one") { registryCredentialId } }`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	rc.ok = false
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { setRegistryCredential(id:"web", registryCredentialId:"") { registryCredentialId } }`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	if got := getApp(t, cl, "web"); got.Spec.RegistryCredentialID == nil || *got.Spec.RegistryCredentialID != "" {
		t.Fatalf("GraphQL clear spec = %+v", got.Spec)
	}
}

func TestGraphQLDockerBuildRegistryCredentialCreateAndEcho(t *testing.T) {
	rc := &fakePullSecrets{name: "web-registry-pull", ok: true}
	svc, cl := rcService(rc)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { createService(name:"web", repo:"https://github.com/octo/app", runtime:"docker", registryCredentialId:"rgc-one") { registryCredentialId } }`})
	if len(res.Errors) > 0 || res.Data.(map[string]any)["createService"].(map[string]any)["registryCredentialId"] != "rgc-one" {
		t.Fatalf("GraphQL create = data %#v errors %v", res.Data, res.Errors)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { setRegistryCredential(id:"web", registryCredentialId:"rgc-two") { registryCredentialId } }`})
	if len(res.Errors) > 0 || res.Data.(map[string]any)["setRegistryCredential"].(map[string]any)["registryCredentialId"] != "rgc-two" {
		t.Fatalf("GraphQL update = data %#v errors %v", res.Data, res.Errors)
	}
	if got := getApp(t, cl, "web"); got.Spec.ExternalRegistryPullSecret != "web-registry-pull" {
		t.Fatalf("GraphQL Docker-build spec = %+v", got.Spec)
	}
}

func TestMCPRegistryCredentialArgsReachCoreRequest(t *testing.T) {
	id := "rgc-one"
	for _, req := range []CreateRequest{
		(createWebServiceArgs{Name: "image", Image: "ghcr.io/acme/private:1", RegistryCredentialID: &id}).toCreateRequest(),
		(createWebServiceArgs{Name: "docker", Repo: "https://github.com/octo/app", Runtime: "docker", RegistryCredentialID: &id}).toCreateRequest(),
	} {
		if req.RegistryCredentialID == nil || *req.RegistryCredentialID != id {
			t.Fatalf("MCP create request = %+v", req)
		}
	}
}

func TestMCPDockerBuildRegistryCredentialCreateUpdateAndEcho(t *testing.T) {
	rc := &fakePullSecrets{name: "web-registry-pull", ok: true}
	svc, cl := rcService(rc)
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "bex", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	call := func(name string, arguments map[string]any) renderService {
		t.Helper()
		res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v result=%+v", name, err, res)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var got renderService
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	created := call("create_web_service", map[string]any{
		"name": "web", "repo": "https://github.com/octo/app", "runtime": "docker",
		"buildCommand": "", "startCommand": "", "registryCredentialId": "rgc-one",
	})
	if created.RegistryCredentialID != "rgc-one" {
		t.Fatalf("MCP create echo = %+v", created)
	}
	updated := call("update_service", map[string]any{"serviceId": "web", "registryCredentialId": "rgc-two"})
	if updated.RegistryCredentialID != "rgc-two" {
		t.Fatalf("MCP update echo = %+v", updated)
	}
	if got := getApp(t, cl, "web"); got.Spec.RegistryCredentialID == nil || *got.Spec.RegistryCredentialID != "rgc-two" || got.Spec.ExternalRegistryPullSecret != "web-registry-pull" {
		t.Fatalf("MCP Docker-build spec = %+v", got.Spec)
	}
}
