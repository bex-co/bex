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

package registrycreds

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func testMux() (*Service, *http.ServeMux) {
	s, _, _ := newTestService()
	mux := http.NewServeMux()
	s.RegisterREST(mux)
	return s, mux
}

func TestRESTCreateReturnsNoSecretAndReadsBack(t *testing.T) {
	_, mux := testMux()

	body := `{"host":"ghcr.io","username":"alice","authToken":"hunter2","name":"GHCR prod"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/registrycredentials", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "hunter2") {
		t.Fatalf("create response leaked the secret: %s", rec.Body)
	}
	var created credentialWire
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Host != "ghcr.io" || created.Username != "alice" || created.Name != "GHCR prod" || created.Status != "active" {
		t.Errorf("created = %+v", created)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/registrycredentials/"+created.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get => 200, got %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "hunter2") {
		t.Fatalf("get response leaked the secret: %s", rec.Body)
	}
}

func TestRESTCanonicalRenderRouteGetsCredential(t *testing.T) {
	_, mux := testMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/registrycredentials", strings.NewReader(`{"host":"ghcr.io","username":"alice","authToken":"hunter2"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", rec.Code, rec.Body)
	}
	var created credentialWire
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/registrycredentials/"+created.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("canonical get => %d: %s", rec.Code, rec.Body)
	}
	var got credentialWire
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Name != created.Name {
		t.Fatalf("canonical get = %+v, want %+v", got, created)
	}
}

func TestRESTCreateMissingFieldsIsBadRequest(t *testing.T) {
	_, mux := testMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/registrycredentials", strings.NewReader(`{"host":"ghcr.io"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create with no username/secret => 400, got %d: %s", rec.Code, rec.Body)
	}
}

func TestRESTListNeverIncludesSecrets(t *testing.T) {
	_, mux := testMux()
	for _, body := range []string{
		`{"host":"ghcr.io","username":"alice","authToken":"hunter2"}`,
		`{"host":"docker.io","username":"bob","authToken":"hunter3"}`,
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/registrycredentials", strings.NewReader(body)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", rec.Code, rec.Body)
		}
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/registrycredentials", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list => 200, got %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if strings.Contains(body, "hunter2") || strings.Contains(body, "hunter3") {
		t.Fatalf("list response leaked a secret: %s", body)
	}
	var list []credentialWire
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 2 {
		t.Fatalf("list = %+v (err %v)", list, err)
	}
}

func TestRESTPatchUpdatesFieldsAndRotatesSecret(t *testing.T) {
	s, mux := testMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/registrycredentials", strings.NewReader(`{"host":"ghcr.io","username":"alice","authToken":"hunter2"}`)))
	var created credentialWire
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/registrycredentials/"+created.ID, strings.NewReader(`{"username":"alice2","authToken":"hunter3"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch => 200, got %d: %s", rec.Code, rec.Body)
	}
	var updated credentialWire
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Username != "alice2" {
		t.Errorf("username not updated: %+v", updated)
	}
	ctx := context.Background()
	secret, _ := s.Secret.Get(ctx, secretPath(s.WorkspaceOrDefault(ctx), created.ID))
	if secret["password"] != "hunter3" {
		t.Errorf("secret not rotated: %+v", secret)
	}
}

func TestRESTPatchExpiresAtThreeState(t *testing.T) {
	_, mux := testMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/registrycredentials", strings.NewReader(`{"host":"ghcr.io","username":"alice","authToken":"hunter2","expiresAt":"2027-01-01T00:00:00Z"}`)))
	var created credentialWire
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ExpiresAt == "" {
		t.Fatal("create did not set expiresAt")
	}

	// Absent expiresAt in the PATCH body leaves it unchanged.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/registrycredentials/"+created.ID, strings.NewReader(`{"username":"alice2"}`)))
	var untouched credentialWire
	_ = json.Unmarshal(rec.Body.Bytes(), &untouched)
	if untouched.ExpiresAt != created.ExpiresAt {
		t.Errorf("no-op patch changed expiresAt: %+v", untouched)
	}

	// Explicit "" clears it.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/registrycredentials/"+created.ID, strings.NewReader(`{"expiresAt":""}`)))
	var cleared credentialWire
	_ = json.Unmarshal(rec.Body.Bytes(), &cleared)
	if cleared.ExpiresAt != "" || cleared.Status != "active" {
		t.Errorf("explicit empty expiresAt should clear it: %+v", cleared)
	}
}

func TestRESTDeleteRemovesCredential(t *testing.T) {
	_, mux := testMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/registrycredentials", strings.NewReader(`{"host":"ghcr.io","username":"alice","authToken":"hunter2"}`)))
	var created credentialWire
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/v1/registrycredentials/"+created.ID, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete => 204, got %d: %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/registrycredentials/"+created.ID, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete => 404, got %d: %s", rec.Code, rec.Body)
	}
}

// TestRESTReportsExpiredStatus pins w2/m14/t007's acceptance bar directly at
// the REST surface: a credential with a past expiresAt reports "expired" —
// not silently hidden — on both create and a subsequent GET. GraphQL/MCP
// share the identical CredentialView.Status computation (rest.go/graphql.go/
// mcp.go all just pass it through via toWire, no adapter-specific logic), so
// this one surface's coverage extends structurally to the other two.
func TestRESTReportsExpiredStatus(t *testing.T) {
	_, mux := testMux()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/registrycredentials",
		strings.NewReader(`{"host":"registry.gitlab.com","username":"bob","authToken":"s3cr3t","expiresAt":"2020-01-01T00:00:00Z"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	var created credentialWire
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Status != "expired" {
		t.Fatalf("create with a past expiresAt = %+v, want status=expired", created)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/registrycredentials/"+created.ID, nil))
	var got credentialWire
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Status != "expired" {
		t.Fatalf("get = %+v, want status=expired — a stale credential must never be silently hidden", got)
	}
}

// TestRESTNonDefaultWorkspaceCredentialIsFullyReachable is the REST half of
// w4/128: POST and the list route bound `?ownerId`, the three by-id routes did
// not. The note left REST unchecked and GraphQL verified; the asymmetry was the
// same on both surfaces, so both bind it now.
func TestRESTNonDefaultWorkspaceCredentialIsFullyReachable(t *testing.T) {
	s, _, _ := newTestService()
	s.Workspace = fakeWorkspaceResolver{"tea-default"}
	mux := http.NewServeMux()
	s.RegisterREST(mux)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "u1", Method: "session"})

	req := httptest.NewRequest("POST", "/v1/registrycredentials",
		strings.NewReader(`{"ownerId":"tea-other","host":"ghcr.io","username":"alice","authToken":"hunter2"}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", rec.Code, rec.Body)
	}
	var created credentialWire
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.OwnerID != "tea-other" {
		t.Fatalf("created ownerId = %q, want tea-other", created.OwnerID)
	}

	// Control: no ?ownerId still means the caller's default workspace.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/registrycredentials/"+created.ID, nil).WithContext(ctx))
	if rec.Code == http.StatusOK {
		t.Errorf("get without ownerId => 200, want a miss in the default workspace: %s", rec.Body)
	}

	for _, tc := range []struct {
		name, method, body string
		want               int
	}{
		{"get", "GET", "", http.StatusOK},
		{"patch", "PATCH", `{"username":"alice2"}`, http.StatusOK},
		{"delete", "DELETE", "", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			r := httptest.NewRequest(tc.method, "/v1/registrycredentials/"+created.ID+"?ownerId=tea-other", body).WithContext(ctx)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, r)
			if rec.Code != tc.want {
				t.Fatalf("%s with ?ownerId => %d, want %d: %s", tc.method, rec.Code, tc.want, rec.Body)
			}
		})
	}
}
