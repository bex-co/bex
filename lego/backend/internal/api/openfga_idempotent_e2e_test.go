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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// openfga_idempotent_e2e_test.go pins w21/099 against a REAL OpenFGA: its
// /write answers 400 `write_failed_due_to_invalid_input` for a delete of a
// tuple that does not exist and for a write of one that already does, unless
// the request says `on_missing` / `on_duplicate: "ignore"`. Every revoke
// (workspace delete, member removal, leaving, account teardown, API-key
// revoke) and every grant retry leans on authz being idempotent, and a fake
// OpenFGA cannot say whether the real one is — which is how DELETE
// /v1/api-keys/{id} answered 500 for a key whose developer tuple was already
// gone, forever (36 logged answers in three weeks).
//
// Same harness and skip condition as the other e2e files:
//
//	BEX_TEST_DB_URI=postgres://postgres:pw@localhost:5433/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api/ -run 'E2E' -v

// fgaRawWrite posts a /write body straight to OpenFGA, so a test can put the
// store in a state the checker's own verbs would never produce (a tuple deleted
// behind bex's back). It returns the status and body.
func fgaRawWrite(t *testing.T, fgaURL, token, body string) (int, string) {
	t.Helper()
	do := func(method, path string, payload []byte) (int, []byte) {
		req, err := http.NewRequest(method, fgaURL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, out
	}
	_, list := do(http.MethodGet, "/stores?page_size=100", nil)
	var stores struct {
		Stores []struct{ ID, Name string } `json:"stores"`
	}
	if err := json.Unmarshal(list, &stores); err != nil {
		t.Fatalf("list stores: %v", err)
	}
	for _, s := range stores.Stores {
		if s.Name == "bex" {
			code, out := do(http.MethodPost, "/stores/"+s.ID+"/write", []byte(body))
			return code, string(out)
		}
	}
	t.Fatal(`openfga store "bex" not found`)
	return 0, ""
}

// A revoke of a tuple that is not there, a second revoke, and a grant of a
// tuple that is already there all succeed, for every relation a member can
// hold and for an agent session's parent edge.
func TestOpenFGAMembershipWritesAreIdempotentE2E(t *testing.T) {
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_OPENFGA_URL not set")
	}
	ctx := context.Background()
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	revoker := checker.(interface {
		RevokeWorkspaceMember(ctx context.Context, tenantID, subject, relation string) error
	})
	fresh := checker.(core.FreshChecker)
	full := ids.New(ids.Workspace)
	run := full[len(full)-8:]

	for _, relation := range []string{"admin", "developer", "viewer"} {
		t.Run(relation, func(t *testing.T) {
			workspace, subject := "tea-idem-"+relation+"-"+run, "user:idem-"+relation+"-"+run
			holds := func() bool {
				ok, err := fresh.CheckFresh(ctx, subject, relation, "workspace:"+workspace)
				if err != nil {
					t.Fatalf("check: %v", err)
				}
				return ok
			}

			// A tuple that was never written (the tuple of a key whose bind was
			// rolled back, an invitee whose grant is still queued).
			if err := revoker.RevokeWorkspaceMember(ctx, workspace, subject, relation); err != nil {
				t.Errorf("revoke of a tuple that was never written: %v", err)
			}

			if err := roles.GrantWorkspaceRole(ctx, workspace, subject, relation); err != nil {
				t.Fatalf("grant: %v", err)
			}
			if err := roles.GrantWorkspaceRole(ctx, workspace, subject, relation); err != nil {
				t.Errorf("second grant of the same tuple (a retry whose first answer was lost): %v", err)
			}
			if !holds() {
				t.Fatal("the grants did not leave the tuple in place")
			}

			if err := revoker.RevokeWorkspaceMember(ctx, workspace, subject, relation); err != nil {
				t.Fatalf("revoke: %v", err)
			}
			if err := revoker.RevokeWorkspaceMember(ctx, workspace, subject, relation); err != nil {
				t.Errorf("second revoke of the same tuple: %v", err)
			}
			if holds() {
				t.Error("the revokes left the tuple in place")
			}
		})
	}

	t.Run("agent session parent edge", func(t *testing.T) {
		grant := checker.(interface {
			GrantAgentSessionWorkspace(ctx context.Context, sessionID, workspaceID string) error
		})
		for i := range 2 {
			if err := grant.GrantAgentSessionWorkspace(ctx, "ags-idem-"+run, "tea-idem-"+run); err != nil {
				t.Errorf("grant %d of the session's parent edge: %v", i+1, err)
			}
		}
	})
}

// DELETE /v1/api-keys/{id} of a key whose OpenFGA developer tuple is already
// gone (an earlier revoke that removed the tuple and then failed, or a bind
// whose tuple never landed) answers 204 and completes the revoke: the Hydra
// client is deleted, the binding is dropped and the shared revocation marker is
// set, so the key stops working on every replica. The second DELETE of the same
// key answers what the route has always answered for a key it does not know.
func TestRevokeAPIKeyWhoseTupleIsGoneE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()
	fgaToken := os.Getenv("BEX_TEST_OPENFGA_TOKEN")

	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	// Additive and unique per run, like the other e2e files.
	full := ids.New(ids.Workspace)
	run := full[len(full)-8:]
	dana := "dana-" + run

	checker := authz.NewOpenFGAChecker(fgaURL, fgaToken)
	roles := checker.(store.MembershipGranter)
	ws, err := st.CreateWorkspace(ctx, "keys-"+run, store.PlanHobby, dana)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+dana, "admin"); err != nil {
		t.Fatalf("grant admin: %v", err)
	}

	tenantSvc := tenancy.NewTenantService(st, roles)
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: checker, Workspace: tenantSvc}
	keyStore := newFakeKeyStore()
	// The fake store counts ids from 1 and the revocation marker is permanent
	// per id, so start where no other run has been.
	keyStore.n = int(time.Now().UnixMicro())
	srv := NewServer(base, Deps{Store: st, WorkspaceStore: st, APIKeys: keyStore, KeyBinder: tenantSvc})
	mux := srv.restHandler()
	call := func(method, path, body string) *http.Response {
		return e2eCall(mux, ctx, dana, method, path, body).Result()
	}
	mint := func(name string) string {
		t.Helper()
		resp := call("POST", "/v1/api-keys", `{"name":"`+name+`","ownerId":"`+ws.ID+`"}`)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create %s: %d", name, resp.StatusCode)
		}
		return decodeJSON[apikeys.APIKey](t, resp).ID
	}
	fresh := checker.(core.FreshChecker)
	holdsTuple := func(keyID string) bool {
		t.Helper()
		ok, err := fresh.CheckFresh(ctx, "user:"+keyID, "developer", "workspace:"+ws.ID)
		if err != nil {
			t.Fatalf("check %s: %v", keyID, err)
		}
		return ok
	}
	hydraHas := func(keyID string) bool {
		t.Helper()
		keys, _ := keyStore.List(ctx)
		for _, k := range keys {
			if k.ID == keyID {
				return true
			}
		}
		return false
	}
	markerSet := func(keyID string) bool {
		t.Helper()
		_, found, err := st.OAuthRevokedAt(ctx, keyID, keyID)
		if err != nil {
			t.Fatal(err)
		}
		return found
	}
	// From the table, not tenantSvc.TenantForKey: that one answers a positive
	// from its 30 s cache after the row is gone.
	bound := func(keyID string) bool {
		t.Helper()
		_, err := st.TenantForIdentity(ctx, keyID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			t.Fatal(err)
		}
		return err == nil
	}

	t.Run("tuple already gone", func(t *testing.T) {
		keyID := mint("tuple-gone")
		if !holdsTuple(keyID) || !hydraHas(keyID) || !bound(keyID) {
			t.Fatal("a fresh key must hold its developer tuple, its client and its binding")
		}
		// An earlier revoke removed the tuple and failed before the rest.
		code, body := fgaRawWrite(t, fgaURL, fgaToken, `{"deletes":{"tuple_keys":[{"user":"user:`+keyID+`","relation":"developer","object":"workspace:`+ws.ID+`"}]}}`)
		if code != http.StatusOK {
			t.Fatalf("raw delete of the tuple: %d %s", code, body)
		}
		if holdsTuple(keyID) {
			t.Fatal("the raw delete left the tuple in place")
		}

		if resp := call("DELETE", "/v1/api-keys/"+keyID, ""); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("DELETE of a key whose tuple is gone: %d, want 204", resp.StatusCode)
		}
		if hydraHas(keyID) {
			t.Error("the Hydra client survived the revoke")
		}
		if bound(keyID) {
			t.Error("the key is still bound to its workspace")
		}
		if !markerSet(keyID) {
			t.Error("no shared revocation marker: another replica would keep accepting the token")
		}

		// The route's answer for a key it no longer knows, whether it was
		// deleted a moment ago or never existed.
		gone := call("DELETE", "/v1/api-keys/"+keyID, "").StatusCode
		never := call("DELETE", "/v1/api-keys/key-never-"+run, "").StatusCode
		if gone != http.StatusNotFound || gone != never {
			t.Errorf("second DELETE = %d, never-existed DELETE = %d, want both 404", gone, never)
		}
	})

	t.Run("tuple present", func(t *testing.T) {
		keyID := mint("tuple-present")
		if resp := call("DELETE", "/v1/api-keys/"+keyID, ""); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("DELETE: %d, want 204", resp.StatusCode)
		}
		if holdsTuple(keyID) || hydraHas(keyID) || bound(keyID) || !markerSet(keyID) {
			t.Error("the revoke left the tuple, the client or the binding behind, or set no marker")
		}
		if resp := call("DELETE", "/v1/api-keys/"+keyID, ""); resp.StatusCode != http.StatusNotFound {
			t.Errorf("second DELETE: %d, want 404", resp.StatusCode)
		}
	})
}
