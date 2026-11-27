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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/registryread"
	"github.com/bex-co/bex/lego/backend/internal/registryread/registrytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestIsDeployableImageE2E is w1/m673 t003's proof against REAL Postgres and
// REAL OpenFGA: Render's isDeployableImage answers a member and an API key
// holding the registry-credential read, refuses a key without it, and refuses
// a credential of a workspace the caller does not belong to exactly as
// registryCredential(id) does.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestIsDeployableImageE2E -v
func TestIsDeployableImageE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()
	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	run := uniqueRunSuffix()
	adminA, dev, adminB := "admin-a-"+run, "dev-"+run, "admin-b-"+run
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	workspace := func(name, admin string) store.Tenant {
		t.Helper()
		ws, err := st.CreateWorkspace(ctx, name+"-"+run, store.PlanPro, admin)
		if err != nil {
			t.Fatal(err)
		}
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+admin, "admin"); err != nil {
			t.Fatal(err)
		}
		return ws
	}
	wsA, wsB := workspace("image-check-a", adminA), workspace("image-check-b", adminB)
	if err := st.AddMember(ctx, dev, wsA.ID, "developer"); err != nil {
		t.Fatal(err)
	}
	if err := roles.GrantWorkspaceRole(ctx, wsA.ID, "user:"+dev, "developer"); err != nil {
		t.Fatal(err)
	}

	keys := &creatorKeys{run: run, keys: map[string]apikeys.APIKey{}}
	resolver := tenancy.NewTenantService(st, roles)
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz: checker, Workspace: resolver, KeyCreators: keys, KeyGrants: resolver,
	}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: roles,
		APIKeys: keys, KeyBinder: resolver,
		RegistryCredsStore: st, Secrets: newMemSecretStore(),
	})
	reg := registrytest.New(t)
	reg.Push("acme/app:1", registryread.DefaultPlatform)
	reg.Push("acme/arm:1", v1.Platform{OS: "linux", Architecture: "arm64"})
	reg.Push("private/app:1", registryread.DefaultPlatform)
	srv.RegistryCreds.ImageCheck = &registryread.Checker{Transport: reg.Transport()}
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema

	session := func(subject string) core.Identity { return core.Identity{Subject: subject, Method: "session"} }
	credential := func(admin string, ws store.Tenant) string {
		t.Helper()
		out := graphqlDo(t, srv, session(admin), fmt.Sprintf(
			`mutation { createRegistryCredential(ownerId: %q, host: %q, username: %q, authToken: %q) { id } }`,
			ws.ID, registrytest.Host, registrytest.Username, registrytest.Password))
		created, _ := out["data"].(map[string]any)["createRegistryCredential"].(map[string]any)
		id, _ := created["id"].(string)
		if id == "" {
			t.Fatalf("create a credential in %s: %v", ws.ID, out)
		}
		return id
	}
	credA, credB := credential(adminA, wsA), credential(adminB, wsB)

	check := func(id core.Identity, image, credentialID string) map[string]any {
		t.Helper()
		input := fmt.Sprintf(`{imageUrl: %q}`, image)
		if credentialID != "" {
			input = fmt.Sprintf(`{imageUrl: %q, registryCredentialId: %q}`, image, credentialID)
		}
		return graphqlDo(t, srv, id, `{ isDeployableImage(externalImageReference: `+input+`) { deployable failureCode failureReason } }`)
	}
	answer := func(out map[string]any) (bool, any) {
		t.Helper()
		data, _ := out["data"].(map[string]any)
		got, ok := data["isDeployableImage"].(map[string]any)
		if !ok || out["errors"] != nil {
			t.Fatalf("isDeployableImage = %v", out)
		}
		return got["deployable"] == true, got["failureCode"]
	}

	// A member of A gets every answer, with and without A's credential.
	for _, c := range []struct {
		image, credential string
		deployable        bool
		code              any
	}{
		{registrytest.Ref("acme/app:1"), "", true, nil},
		{registrytest.Ref("acme/app:2"), "", false, registryread.FailureDoesNotExist},
		{registrytest.Ref("acme/arm:1"), "", false, registryread.FailureUnsupportedPlatform},
		{"127.0.0.1:5000/acme/app:1", "", false, registryread.FailureInvalidInputURL},
		{registrytest.Ref("private/app:1"), "", false, registryread.FailureDoesNotExist},
		{registrytest.Ref("private/app:1"), credA, true, nil},
	} {
		deployable, code := answer(check(session(dev), c.image, c.credential))
		if deployable != c.deployable || code != c.code {
			t.Errorf("%s with credential %q = %v %v, want %v %v", c.image, c.credential, deployable, code, c.deployable, c.code)
		}
	}

	// B's credential is not A's member's to use: the same refusal
	// registryCredential(id) gives, and no verdict.
	want := graphqlDo(t, srv, session(dev), fmt.Sprintf(`{ registryCredential(id: %q) { id } }`, credB))
	got := check(session(dev), registrytest.Ref("private/app:1"), credB)
	if want["errors"] == nil || got["errors"] == nil || graphqlCode(got) != graphqlCode(want) ||
		errorMessage(got) != errorMessage(want) {
		t.Errorf("another workspace's credential = %v; want registryCredential's refusal %v", got, want)
	}

	// A key needs the registry-credential read (Workspaces Read), as
	// registryCredentials does.
	mint := func(permissions string) apikeys.APIKey {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/api-keys",
			strings.NewReader(`{"name":"image-check","ownerId":"`+wsA.ID+`","permissions":`+permissions+`}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux := srv.restHandler()
		srv.withScopeClassREST(mux, mux).ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, session(dev))))
		var key apikeys.APIKey
		if err := json.Unmarshal(rec.Body.Bytes(), &key); err != nil || rec.Code != http.StatusCreated {
			t.Fatalf("mint %s: %d %s", permissions, rec.Code, rec.Body)
		}
		return key
	}
	keyOf := func(k apikeys.APIKey) core.Identity {
		return core.Identity{Subject: k.ID, ClientID: k.ID, Method: core.MethodOAuth2}
	}
	registryKey := mint(`{"grants":[{"permission":"workspaces","level":"read"}]}`)
	servicesKey := mint(`{"grants":[{"permission":"services","level":"read_write"}]}`)
	if deployable, code := answer(check(keyOf(registryKey), registrytest.Ref("private/app:1"), credA)); !deployable || code != nil {
		t.Errorf("a Workspaces Read key = %v %v, want deployable", deployable, code)
	}
	if out := check(keyOf(servicesKey), registrytest.Ref("acme/app:1"), ""); graphqlCode(out) != core.KeyPermissionDeniedCode {
		t.Errorf("a key without the registry read = %v, want %s", out, core.KeyPermissionDeniedCode)
	}
}

func errorMessage(out map[string]any) string {
	errs, _ := out["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	msg, _ := errs[0].(map[string]any)["message"].(string)
	return msg
}
