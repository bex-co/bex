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

package workspaces

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// currentuser_test.go covers CurrentUser (GET /v1/users, w4/m25 + w4/062): session
// callers named and nameless, machine callers resolved through the key→identity
// binding, the earliest-admin fallback for keys with no owning human, and the
// REST wire id the pinned CLI User type consumes.

// pinnedCLIUser is the field contract of render-oss/cli pkg/client.User
// (types_gen.go User: Id/Email/Name). Decoding the live response into this —
// not a self-mirroring mock of renderUser — fails if the producer omits id.
type pinnedCLIUser struct {
	Email string `json:"email"`
	Id    string `json:"id"`
	Name  string `json:"name"`
}

// fakeKeyOwners is an in-memory KeyOwnerReader: clientID → minting subject.
type fakeKeyOwners map[string]string

func (f fakeKeyOwners) KeyOwner(_ context.Context, clientID string) (string, bool) {
	s, ok := f[clientID]
	return s, ok
}

// ctxAsSession is ctxAs plus the email/name traits the auth gate resolves onto
// a session caller's Identity.
func ctxAsSession(subject, email, name string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{
		Subject: subject, Method: "session", Email: email, Name: name,
	})
}

// ctxAsAPIKey is a client_credentials caller: the auth gate stamps
// ClientID == Subject (the token was minted for the key itself).
func ctxAsAPIKey(clientID string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{
		Subject: clientID, Method: "oauth2", ClientID: clientID,
	})
}

// ctxAsOAuthUser is a user-consented OAuth caller: the subject is a Kratos
// identity id, distinct from the app's client id.
func ctxAsOAuthUser(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{
		Subject: subject, Method: "oauth2", ClientID: "agent-app",
	})
}

func TestCurrentUser_SessionCallerNamed(t *testing.T) {
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	u, err := svc.CurrentUser(ctxAsSession("user-a", "a@example.com", "Ada Lovelace"))
	if err != nil || u.ID != "user-a" || u.Email != "a@example.com" || u.Name != "Ada Lovelace" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_SessionCallerNamelessLegacyIdentity(t *testing.T) {
	// An identity minted before the name trait existed: email only, name "".
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	u, err := svc.CurrentUser(ctxAsSession("user-a", "a@example.com", ""))
	if err != nil || u.ID != "user-a" || u.Email != "a@example.com" || u.Name != "" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_APIKeyCallerResolvesThroughKeyBinding(t *testing.T) {
	// The machine caller's subject is a Hydra client_id; KeyOwners resolves it to
	// the minting human, whose traits come from the same Identities lookup the
	// owners/members surfaces use. Id is that human's subject — never the key.
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	svc.Identities = fakeIdentities{"user-a": {Email: "a@example.com", Name: "Ada Lovelace"}}
	svc.KeyOwners = fakeKeyOwners{"key-1": "user-a"}

	u, err := svc.CurrentUser(ctxAsAPIKey("key-1"))
	if err != nil || u.ID != "user-a" || u.Email != "a@example.com" || u.Name != "Ada Lovelace" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_OAuthUserTokenSubjectResolvesDirectly(t *testing.T) {
	// A user-consented OAuth token's subject IS a Kratos identity id
	// (ClientID differs) — resolved directly, no key binding involved.
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	svc.Identities = fakeIdentities{"user-a": {Email: "a@example.com", Name: "Ada Lovelace"}}

	u, err := svc.CurrentUser(ctxAsOAuthUser("user-a"))
	if err != nil || u.ID != "user-a" || u.Email != "a@example.com" || u.Name != "Ada Lovelace" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_UnboundKeyFallsBackToEarliestAdminEmail(t *testing.T) {
	// A key with no resolvable owning human (no created-by stamp): the bound
	// workspace's earliest-admin email, no name, no fabricated id — the
	// documented honest subset.
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	svc.Identities = fakeIdentities{"user-a": {Email: "a@example.com", Name: "Ada Lovelace"}}
	svc.KeyOwners = fakeKeyOwners{} // no binding for this key
	w, err := svc.Create(ctxAs("user-a"), "acme", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	// Bind the key into the workspace the way BindKey does: a developer
	// tenant_members row (the key subject itself has no Kratos identity).
	st.members[w.ID] = append(st.members[w.ID], store.TenantMember{
		TenantID: w.ID, Subject: "key-unbound", Role: "developer",
	})

	u, err := svc.CurrentUser(ctxAsAPIKey("key-unbound"))
	if err != nil || u.ID != "" || u.Email != "a@example.com" || u.Name != "" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_NoIdentitySourcesIsHonestlyEmpty(t *testing.T) {
	// No Identities, no KeyOwners, no tenant: all fields empty, never faked.
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	u, err := svc.CurrentUser(ctxAsAPIKey("key-orphan"))
	if err != nil || u.ID != "" || u.Email != "" || u.Name != "" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestRESTUsers_DecodesIntoPinnedCLIUserWithID(t *testing.T) {
	// Drive RegisterREST and decode with the pinned CLI User field contract so
	// omitting wire id cannot hide behind a self-mirroring renderUser mock.
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	req = req.WithContext(ctxAsSession("user-a", "a@example.com", "Ada Lovelace"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/users = %d body=%s", rec.Code, rec.Body.String())
	}

	var got pinnedCLIUser
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode pinned CLI User: %v body=%s", err, rec.Body.String())
	}
	if got.Id != "user-a" || got.Email != "a@example.com" || got.Name != "Ada Lovelace" {
		t.Fatalf("pinned CLI User = %+v", got)
	}
}

func TestToRenderUser_CarriesID(t *testing.T) {
	r := toRenderUser(UserView{ID: "user-a", Email: "a@example.com", Name: "Ada"})
	if r.ID != "user-a" || r.Email != "a@example.com" || r.Name != "Ada" {
		t.Fatalf("toRenderUser = %+v", r)
	}
}
