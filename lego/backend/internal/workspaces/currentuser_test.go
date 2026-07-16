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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// currentuser_test.go covers CurrentUser (GET /v1/users, w4/m25): session
// callers named and nameless, machine callers resolved through the key→identity
// binding, and the earliest-admin fallback for keys with no owning human.

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
	if err != nil || u.Email != "a@example.com" || u.Name != "Ada Lovelace" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_SessionCallerNamelessLegacyIdentity(t *testing.T) {
	// An identity minted before the name trait existed: email only, name "".
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	u, err := svc.CurrentUser(ctxAsSession("user-a", "a@example.com", ""))
	if err != nil || u.Email != "a@example.com" || u.Name != "" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_APIKeyCallerResolvesThroughKeyBinding(t *testing.T) {
	// The machine caller's subject is a Hydra client_id; KeyOwners resolves it to
	// the minting human, whose traits come from the same Identities lookup the
	// owners/members surfaces use.
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	svc.Identities = fakeIdentities{"user-a": {Email: "a@example.com", Name: "Ada Lovelace"}}
	svc.KeyOwners = fakeKeyOwners{"key-1": "user-a"}

	u, err := svc.CurrentUser(ctxAsAPIKey("key-1"))
	if err != nil || u.Email != "a@example.com" || u.Name != "Ada Lovelace" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_OAuthUserTokenSubjectResolvesDirectly(t *testing.T) {
	// A user-consented OAuth token's subject IS a Kratos identity id
	// (ClientID differs) — resolved directly, no key binding involved.
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	svc.Identities = fakeIdentities{"user-a": {Email: "a@example.com", Name: "Ada Lovelace"}}

	u, err := svc.CurrentUser(ctxAsOAuthUser("user-a"))
	if err != nil || u.Email != "a@example.com" || u.Name != "Ada Lovelace" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_UnboundKeyFallsBackToEarliestAdminEmail(t *testing.T) {
	// A key with no resolvable owning human (no created-by stamp): the bound
	// workspace's earliest-admin email, no name — the documented honest subset.
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
	if err != nil || u.Email != "a@example.com" || u.Name != "" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}

func TestCurrentUser_NoIdentitySourcesIsHonestlyEmpty(t *testing.T) {
	// No Identities, no KeyOwners, no tenant: both fields empty, never faked.
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	u, err := svc.CurrentUser(ctxAsAPIKey("key-orphan"))
	if err != nil || u.Email != "" || u.Name != "" {
		t.Fatalf("CurrentUser = %+v, %v", u, err)
	}
}
