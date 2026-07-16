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
			rc := &fakePullSecrets{err: tc.err}
			svc, cl := rcService(rc)
			_, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "ghcr.io/acme/private:1", RegistryCredentialID: &id})
			if !errors.Is(err, tc.err) {
				t.Fatalf("create error = %v, want %v", err, tc.err)
			}
			var app appv1alpha1.App
			if getErr := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &app); !apierrors.IsNotFound(getErr) {
				t.Fatalf("failed create wrote App: %v", getErr)
			}
		})
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
			svc, _ := rcService(&fakePullSecrets{err: tc.err})
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(`{"name":"web","type":"web_service","image":{"imagePath":"ghcr.io/acme/private:1","registryCredentialId":"rgc-one"}}`)))
			if rec.Code != tc.status {
				t.Fatalf("POST = %d: %s, want %d", rec.Code, rec.Body.String(), tc.status)
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

func TestMCPRegistryCredentialArgsReachCoreRequest(t *testing.T) {
	id := "rgc-one"
	req := (createWebServiceArgs{Name: "web", Image: "ghcr.io/acme/private:1", RegistryCredentialID: &id}).toCreateRequest()
	if req.RegistryCredentialID == nil || *req.RegistryCredentialID != id {
		t.Fatalf("MCP create request = %+v", req)
	}
}
