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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// apikey_member_verbs_e2e_test.go is w9/116's proof against REAL Postgres and
// REAL OpenFGA, through the composed REST (Render's /v1/owners/{ownerId}/members
// and bex's /v1/workspaces/{id}/members), GraphQL and MCP surfaces: an Admin's
// API key changes a member's role and removes a member, as Render's key with
// "Update member roles and remove members" does. A Developer's key is refused;
// a key never acts on its creator or on a key's binding; a key whose grant
// lacks the permission is refused under an Admin creator; and once the Admin
// is demoted their key is refused on its next request.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestAPIKeyManagesMembersAsItsCreatorE2E -v
func TestAPIKeyManagesMembersAsItsCreatorE2E(t *testing.T) {
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
	admin, admin2, dev, target := "admin-"+run, "admin2-"+run, "dev-"+run, "target-"+run
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	// Scale offers all five roles.
	ws, err := st.CreateWorkspace(ctx, "keymembers-"+run, store.PlanScale, admin)
	if err != nil {
		t.Fatal(err)
	}
	seat := func(subject, role string) {
		t.Helper()
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+subject, role); err != nil {
			t.Fatalf("grant %s %s: %v", subject, role, err)
		}
	}
	add := func(subject, role string) {
		t.Helper()
		if err := st.AddMember(ctx, subject, ws.ID, role); err != nil {
			t.Fatal(err)
		}
		seat(subject, role)
	}
	seat(admin, "admin")
	add(admin2, "admin")
	add(dev, "developer")
	add(target, "viewer")
	// One member per surface for the removals.
	removable := map[string]string{}
	for _, s := range []string{"render", "rest", "graphql", "mcp"} {
		removable[s] = "rm-" + s + "-" + run
		add(removable[s], "viewer")
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
	})
	mux := srv.restHandler()
	gated := srv.withScopeClassREST(mux, mux)
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema

	session := func(subject string) core.Identity { return core.Identity{Subject: subject, Method: "session"} }
	keyOf := func(id string) core.Identity {
		return core.Identity{Subject: id, ClientID: id, Method: core.MethodOAuth2}
	}
	rest := func(id core.Identity, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, id)))
		return rec
	}
	mint := func(creator, permissions string) string {
		t.Helper()
		body := `{"name":"members","ownerId":"` + ws.ID + `"`
		if permissions != "" {
			body += `,"permissions":` + permissions
		}
		rec := rest(session(creator), http.MethodPost, "/v1/api-keys", body+"}")
		var key apikeys.APIKey
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &key) != nil {
			t.Fatalf("%s mints a key: %d %s", creator, rec.Code, rec.Body)
		}
		return key.ID
	}
	userID := func(subject string) string {
		t.Helper()
		owned, err := st.OwnerIDsForSubjects(ctx, []string{subject})
		if err != nil || owned[subject] == "" {
			t.Fatalf("own- id of %s: %v", subject, err)
		}
		return owned[subject]
	}
	roleOf := func(subject string) string {
		t.Helper()
		m, err := st.GetTenantMember(ctx, ws.ID, subject)
		if errors.Is(err, store.ErrNotFound) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return m.Role
	}
	heldRoles := func(subject string) []string {
		t.Helper()
		var held []string
		for _, role := range members.Roles {
			ok, err := checker.(core.FreshChecker).CheckFresh(ctx, "user:"+subject, role, core.WorkspaceObject(ws.ID))
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				held = append(held, role)
			}
		}
		return held
	}

	// The three surfaces, each answering "" for an allowed change, else the
	// refusal's code ("FORBIDDEN" for a bare 403).
	type surface struct {
		name   string
		change func(id core.Identity, subject, role string) string
		remove func(id core.Identity, subject string) string
	}
	restCode := func(rec *httptest.ResponseRecorder) string {
		switch {
		case rec.Code < 300:
			return ""
		case rec.Code == http.StatusForbidden || rec.Code == http.StatusConflict:
			var body struct {
				Code string `json:"code"`
			}
			if json.Unmarshal(rec.Body.Bytes(), &body) == nil && body.Code != "" {
				return body.Code
			}
			return "FORBIDDEN"
		}
		t.Fatalf("REST answered %d %s", rec.Code, rec.Body)
		return ""
	}
	gqlCode := func(out map[string]any) string {
		code := graphqlCode(out)
		if code == "" && out["errors"] != nil {
			t.Fatalf("GraphQL answered %v", out)
		}
		return code
	}
	mcpCode := func(text string, isError bool) string {
		if !isError {
			return ""
		}
		for _, code := range []string{members.ErrorCannotChangeOwnRole, members.ErrorCannotRemoveSelf, members.ErrorMemberIsMachine, core.KeyPermissionDeniedCode} {
			if strings.Contains(text, code) {
				return code
			}
		}
		if strings.Contains(text, "not granted") {
			return core.KeyPermissionDeniedCode
		}
		if strings.Contains(strings.ToLower(text), "forbidden") {
			return "FORBIDDEN"
		}
		t.Fatalf("MCP answered %s", text)
		return ""
	}
	surfaces := []surface{
		{"REST /v1/owners", func(id core.Identity, subject, role string) string {
			if role != "ADMIN" && role != "DEVELOPER" {
				role = "WORKSPACE_" + role // Render's enum
			}
			return restCode(rest(id, http.MethodPatch, "/v1/owners/"+ws.ID+"/members/"+userID(subject), `{"role":"`+role+`"}`))
		}, func(id core.Identity, subject string) string {
			return restCode(rest(id, http.MethodDelete, "/v1/owners/"+ws.ID+"/members/"+userID(subject), ""))
		}},
		{"REST /v1/workspaces", func(id core.Identity, subject, role string) string {
			return restCode(rest(id, http.MethodPatch, "/v1/workspaces/"+ws.ID+"/members/"+subject, `{"role":"`+role+`"}`))
		}, func(id core.Identity, subject string) string {
			return restCode(rest(id, http.MethodDelete, "/v1/workspaces/"+ws.ID+"/members/"+subject, ""))
		}},
		{"GraphQL", func(id core.Identity, subject, role string) string {
			return gqlCode(graphqlDo(t, srv, id, fmt.Sprintf(`mutation { changeWorkspaceMemberRole(workspaceId: %q, subject: %q, role: %q) { subject role } }`, ws.ID, subject, role)))
		}, func(id core.Identity, subject string) string {
			return gqlCode(graphqlDo(t, srv, id, fmt.Sprintf(`mutation { removeWorkspaceMember(workspaceId: %q, subject: %q) }`, ws.ID, subject)))
		}},
		{"MCP", func(id core.Identity, subject, role string) string {
			res := mcpCall(t, srv, id, "change_workspace_member_role", map[string]any{"subject": subject, "role": role})
			return mcpCode(mcpText(res), res.IsError)
		}, func(id core.Identity, subject string) string {
			res := mcpCall(t, srv, id, "remove_workspace_member", map[string]any{"subject": subject})
			return mcpCode(mcpText(res), res.IsError)
		}},
	}
	removedOn := map[string]string{
		"REST /v1/owners": removable["render"], "REST /v1/workspaces": removable["rest"],
		"GraphQL": removable["graphql"], "MCP": removable["mcp"],
	}

	keyA := mint(admin, "")
	keyA2 := mint(admin2, "")
	keyD := mint(dev, "")
	keyRead := mint(admin, `{"grants":[{"permission":"workspaces","level":"read"}]}`)

	// An Admin's key changes a member's role and removes a member on every
	// surface; the row and the tuple both follow.
	for i, s := range surfaces {
		to := []string{"CONTRIBUTOR", "DEVELOPER", "BILLING", "VIEWER"}[i]
		if code := s.change(keyOf(keyA), target, to); code != "" {
			t.Fatalf("%s: an Admin's key sets %s = %s", s.name, to, code)
		}
		want := strings.ToLower(to)
		if got := roleOf(target); got != want {
			t.Errorf("%s: row = %q, want %q", s.name, got, want)
		}
		if held := heldRoles(target); len(held) != 1 || held[0] != want {
			t.Errorf("%s: tuples = %v, want exactly %s", s.name, held, want)
		}
		gone := removedOn[s.name]
		if code := s.remove(keyOf(keyA), gone); code != "" {
			t.Fatalf("%s: an Admin's key removes a member = %s", s.name, code)
		}
		if roleOf(gone) != "" || len(heldRoles(gone)) != 0 {
			t.Errorf("%s: the removed member keeps row %q, tuples %v", s.name, roleOf(gone), heldRoles(gone))
		}
	}

	for _, s := range surfaces {
		// A Developer's key is refused both verbs.
		if code := s.change(keyOf(keyD), target, "ADMIN"); code != "FORBIDDEN" {
			t.Errorf("%s: a Developer's key promotes = %q, want FORBIDDEN", s.name, code)
		}
		if code := s.remove(keyOf(keyD), target); code != "FORBIDDEN" {
			t.Errorf("%s: a Developer's key removes = %q, want FORBIDDEN", s.name, code)
		}
		// A key never acts on its creator, in either direction, or on a key's
		// binding (its own or another's).
		for _, to := range []string{"DEVELOPER", "ADMIN"} {
			if code := s.change(keyOf(keyA), admin, to); code != members.ErrorCannotChangeOwnRole {
				t.Errorf("%s: a key sets its creator to %s = %q", s.name, to, code)
			}
		}
		if code := s.remove(keyOf(keyA), admin); code != members.ErrorCannotRemoveSelf {
			t.Errorf("%s: a key removes its creator = %q", s.name, code)
		}
		if s.name != "REST /v1/owners" { // a key has no own- user id to name
			if code := s.change(keyOf(keyA), keyA, "VIEWER"); code != members.ErrorCannotChangeOwnRole {
				t.Errorf("%s: a key re-roles its own binding = %q", s.name, code)
			}
			if code := s.change(keyOf(keyA), keyA2, "VIEWER"); code != members.ErrorMemberIsMachine {
				t.Errorf("%s: a key re-roles another key's binding = %q", s.name, code)
			}
		}
		// A key whose grant lacks "Update member roles and remove members" is
		// refused under an Admin creator.
		if code := s.change(keyOf(keyRead), target, "DEVELOPER"); code != core.KeyPermissionDeniedCode {
			t.Errorf("%s: a Workspaces Read key re-roles = %q", s.name, code)
		}
		if code := s.remove(keyOf(keyRead), target); code != core.KeyPermissionDeniedCode {
			t.Errorf("%s: a Workspaces Read key removes = %q", s.name, code)
		}
	}
	if roleOf(admin) != "admin" || roleOf(target) != "viewer" || roleOf(keyA) != "admin" || roleOf(keyA2) != "admin" {
		t.Fatalf("a refused change moved a row: admin=%s target=%s keyA=%s keyA2=%s",
			roleOf(admin), roleOf(target), roleOf(keyA), roleOf(keyA2))
	}
	// The other Admin's key may change admin's role: the guard is on each
	// key's own creator.
	if code := surfaces[1].change(keyOf(keyA2), admin, "DEVELOPER"); code != "" {
		t.Fatalf("another Admin's key demotes admin = %s", code)
	}

	// admin is now a Developer: their key is refused on its very next request,
	// on every surface, and follows them back up.
	for _, s := range surfaces {
		if code := s.change(keyOf(keyA), target, "CONTRIBUTOR"); code != "FORBIDDEN" {
			t.Errorf("%s: the demoted Admin's key re-roles = %q", s.name, code)
		}
		if code := s.remove(keyOf(keyA), target); code != "FORBIDDEN" {
			t.Errorf("%s: the demoted Admin's key removes = %q", s.name, code)
		}
	}
	if code := surfaces[1].change(keyOf(keyA2), admin, "ADMIN"); code != "" {
		t.Fatalf("re-promote admin = %s", code)
	}
	if code := surfaces[2].change(keyOf(keyA), target, "CONTRIBUTOR"); code != "" {
		t.Errorf("the re-promoted Admin's key re-roles = %s", code)
	}
	if roleOf(target) != "contributor" {
		t.Errorf("target = %q, want contributor", roleOf(target))
	}
}
