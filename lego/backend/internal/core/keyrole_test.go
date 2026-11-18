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

package core

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"
)

// The workspace relations each role holds, restated from
// deploy/gitops/authz/model.fga.
var modelRoleRelations = map[string][]string{
	"admin":       {RelCanView, RelCanViewLogs, RelCanOperate, RelCanCreate, RelCanViewSensitive, RelCanManageKeys, RelCanManageSSHKeys, RelCanManage, RelCanManageBilling},
	"developer":   {RelCanView, RelCanViewLogs, RelCanOperate, RelCanCreate, RelCanViewSensitive, RelCanManageKeys, RelCanManageSSHKeys},
	"contributor": {RelCanView, RelCanViewLogs, RelCanOperate, RelCanManageSSHKeys},
	"viewer":      {RelCanView, RelCanManageSSHKeys},
	"billing":     {RelCanView, RelCanManageSSHKeys, RelCanManageBilling},
}

var modelRelations = []string{RelCanView, RelCanViewLogs, RelCanOperate, RelCanCreate, RelCanViewSensitive, RelCanManageKeys, RelCanManageSSHKeys, RelCanManage, RelCanManageBilling}

// roleModel is OpenFGA's workspace type as role tuples: a subject holds a
// relation on a workspace when any role tuple it has there grants it. cached
// stands in for a positive cache that still answers from tuples that have
// since been revoked (nil: the cache is the store).
type roleModel struct {
	mu          sync.Mutex
	tuples      map[string]map[string]bool // "user:<subject>|workspace:<id>" -> roles
	cached      map[string]map[string]bool
	freshChecks []string // the subjects asked uncached, in order
}

func newRoleModel() *roleModel { return &roleModel{tuples: map[string]map[string]bool{}} }

func (m *roleModel) grant(subject, workspace, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "user:" + subject + "|" + WorkspaceObject(workspace)
	if m.tuples[key] == nil {
		m.tuples[key] = map[string]bool{}
	}
	m.tuples[key][role] = true
}

func (m *roleModel) revoke(subject, workspace, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tuples["user:"+subject+"|"+WorkspaceObject(workspace)], role)
}

func holds(roles map[string]bool, relation string) bool {
	for role := range roles {
		if slices.Contains(modelRoleRelations[role], relation) {
			return true
		}
	}
	return false
}

func (m *roleModel) Check(_ context.Context, subject, relation, object string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cached != nil {
		return holds(m.cached[subject+"|"+object], relation), nil
	}
	return holds(m.tuples[subject+"|"+object], relation), nil
}

func (m *roleModel) CheckFresh(_ context.Context, subject, relation, object string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.freshChecks = append(m.freshChecks, subject)
	return holds(m.tuples[subject+"|"+object], relation), nil
}

// keyCreators is a KeyCreatorResolver: a client id in the map is a bex API
// key minted by the mapped subject ("" for one with no stamp); anything else
// is not a key.
type keyCreators struct {
	creators map[string]string
	err      error
}

func (k keyCreators) APIKeyCreator(_ context.Context, clientID string) (string, bool, error) {
	if k.err != nil {
		return "", false, k.err
	}
	creator, ok := k.creators[clientID]
	return creator, ok, nil
}

func keyContext(clientID string) context.Context {
	return WithIdentity(context.Background(), Identity{Subject: clientID, ClientID: clientID, Method: MethodOAuth2})
}

// A key's tuple still says admin because its revoke failed after its
// creator's demotion to Developer (I1): every Admin-only relation is refused
// on every path, fresh and cached, and the Developer relations still pass.
func TestKeyIsRefusedWhatItsDemotedCreatorMayNotDo(t *testing.T) {
	m := newRoleModel()
	m.grant("person-a", "tea-a", "developer") // demoted
	m.grant("key-a", "tea-a", "admin")        // the revoke that failed
	b := &Base{Authz: m, Workspace: fakeWorkspace{"key-a": "tea-a"}, KeyCreators: keyCreators{creators: map[string]string{"key-a": "person-a"}}}
	ctx := keyContext("key-a")

	for _, rel := range []string{RelCanManage, RelCanManageBilling} {
		if err := b.Authorize(ctx, rel); !errors.Is(err, ErrForbidden) {
			t.Errorf("Authorize(%s) = %v, want ErrForbidden: the creator is a Developer now", rel, err)
		}
		if err := b.AuthorizeFreshOn(ctx, rel, WorkspaceObject("tea-a")); !errors.Is(err, ErrForbidden) {
			t.Errorf("AuthorizeFreshOn(%s) = %v, want ErrForbidden", rel, err)
		}
		if d := b.CanDecision(ctx, rel); d.Allowed() {
			t.Errorf("CanDecision(%s) allowed, want refused on the cached path too", rel)
		}
		if ok, err := b.HoldsFresh(ctx, rel, WorkspaceObject("tea-a")); ok || err != nil {
			t.Errorf("HoldsFresh(%s) = %v, %v, want false, nil", rel, ok, err)
		}
	}
	for _, rel := range modelRoleRelations["developer"] {
		if err := b.Authorize(ctx, rel); err != nil {
			t.Errorf("Authorize(%s) = %v, want allowed: the creator is still a Developer", rel, err)
		}
	}
}

// The ceiling asks about the creator through the same path the key's own
// decision took: uncached before a write, so a creator demoted on another
// replica is seen at once, exactly as the creator's own write would see it.
func TestKeyCeilingChecksTheCreatorAsFreshlyAsTheKey(t *testing.T) {
	m := newRoleModel()
	m.grant("person-a", "tea-a", "admin")
	m.grant("key-a", "tea-a", "admin")
	b := &Base{Authz: m, Workspace: fakeWorkspace{"key-a": "tea-a"}, KeyCreators: keyCreators{creators: map[string]string{"key-a": "person-a"}}}
	ctx := keyContext("key-a")

	if err := b.Authorize(ctx, RelCanManage); err != nil {
		t.Fatalf("an Admin's key: %v", err)
	}
	if want := []string{"user:key-a", "user:person-a"}; !slices.Equal(m.freshChecks, want) {
		t.Fatalf("uncached checks = %v, want %v", m.freshChecks, want)
	}
	// A positive cache that still remembers the creator as Admin cannot carry
	// the key's write: the creator's tuple is gone from the store.
	m.cached = map[string]map[string]bool{
		"user:person-a|workspace:tea-a": {"admin": true},
		"user:key-a|workspace:tea-a":    {"admin": true},
	}
	m.revoke("person-a", "tea-a", "admin")
	m.grant("person-a", "tea-a", "viewer")
	if err := b.Authorize(ctx, RelCanManage); !errors.Is(err, ErrForbidden) {
		t.Fatalf("write after the creator's demotion = %v, want ErrForbidden", err)
	}
	// A read rides the cache for the creator exactly as it does for the key,
	// and as the creator's own read would (the same staleness, never more).
	if err := b.Authorize(ctx, RelCanView); err != nil {
		t.Fatalf("cached read = %v, want the cached answer the creator would get", err)
	}
}

// I2: a key acts only in the workspace it was minted in. Its creator being an
// Admin of workspace B gives the key nothing there, on any verb.
func TestKeyActsOnlyInItsOwnWorkspace(t *testing.T) {
	m := newRoleModel()
	m.grant("person-a", "tea-a", "admin")
	m.grant("person-a", "tea-b", "admin")
	m.grant("key-a", "tea-a", "admin")
	b := &Base{Authz: m, Workspace: fakeWorkspace{"key-a": "tea-a"}, KeyCreators: keyCreators{creators: map[string]string{"key-a": "person-a"}}}
	ctx := keyContext("key-a")

	for _, rel := range modelRelations {
		if err := b.AuthorizeOn(ctx, rel, WorkspaceObject("tea-b")); !errors.Is(err, ErrForbidden) {
			t.Errorf("AuthorizeOn(%s, tea-b) = %v, want ErrForbidden", rel, err)
		}
		if ok, _ := b.HoldsFresh(ctx, rel, WorkspaceObject("tea-b")); ok {
			t.Errorf("HoldsFresh(%s, tea-b) = true, want false", rel)
		}
		// Naming the other workspace is refused before any check: the key is
		// no member of it.
		if err := b.Authorize(WithWorkspace(ctx, "tea-b"), rel); !errors.Is(err, ErrForbidden) {
			t.Errorf("Authorize(%s) naming tea-b = %v, want ErrForbidden", rel, err)
		}
		if err := b.Authorize(ctx, rel); err != nil {
			t.Errorf("Authorize(%s) in its own workspace = %v, want allowed", rel, err)
		}
	}
}

// I3: a key minted before the created-by stamp never acts above a Developer,
// whatever tuple it has been given.
func TestUnattributedKeyNeverPassesAnAdminRelation(t *testing.T) {
	m := newRoleModel()
	m.grant("key-old", "tea-a", "admin") // a hand-written grant
	b := &Base{Authz: m, Workspace: fakeWorkspace{"key-old": "tea-a"}, KeyCreators: keyCreators{creators: map[string]string{"key-old": ""}}}
	ctx := keyContext("key-old")

	for _, rel := range []string{RelCanManage, RelCanManageBilling, RelCanJoin, RelCanJoinAsAdmin} {
		if err := b.AuthorizeOn(ctx, rel, WorkspaceObject("tea-a")); !errors.Is(err, ErrForbidden) {
			t.Errorf("AuthorizeOn(%s) = %v, want ErrForbidden", rel, err)
		}
	}
	for _, rel := range modelRoleRelations["developer"] {
		if err := b.Authorize(ctx, rel); err != nil {
			t.Errorf("Authorize(%s) = %v, want the Developer binding it always had", rel, err)
		}
	}
}

// A creator lookup that cannot answer refuses every key decision as
// unavailable, never as an allow; a client that is not a bex API key is
// decided by its own tuples, as before.
func TestKeyCeilingFailsClosedAndLeavesOtherClientsAlone(t *testing.T) {
	m := newRoleModel()
	m.grant("key-a", "tea-a", "developer")
	m.grant("platform-client", "tea-a", "admin")
	b := &Base{Authz: m, Workspace: fakeWorkspace{"key-a": "tea-a", "platform-client": "tea-a"}, KeyCreators: keyCreators{err: errors.New("hydra down")}}
	if err := b.Authorize(keyContext("key-a"), RelCanView); !errors.Is(err, ErrAuthzUnavailable) {
		t.Fatalf("Hydra down = %v, want ErrAuthzUnavailable", err)
	}
	if _, err := b.HoldsFresh(keyContext("key-a"), RelCanView, WorkspaceObject("tea-a")); !errors.Is(err, ErrAuthzUnavailable) {
		t.Fatalf("HoldsFresh with Hydra down = %v, want ErrAuthzUnavailable", err)
	}

	b.KeyCreators = keyCreators{creators: map[string]string{}}
	if err := b.Authorize(keyContext("platform-client"), RelCanManage); err != nil {
		t.Fatalf("a platform client = %v, want its own tuple to decide", err)
	}
	// A person is never looked up as a key.
	session := WithIdentity(context.Background(), Identity{Subject: "platform-client", Method: MethodSession})
	b.KeyCreators = keyCreators{err: errors.New("must not be asked")}
	if err := b.Authorize(session, RelCanManage); err != nil {
		t.Fatalf("a session = %v, want no key lookup", err)
	}
}

// A key minted by another key (possible only before minting became
// human-only) is held to that key's decision, which is held to its person.
func TestKeyMintedByAKeyIsCappedThroughTheChain(t *testing.T) {
	m := newRoleModel()
	m.grant("person-a", "tea-a", "viewer")
	m.grant("key-1", "tea-a", "admin")
	m.grant("key-2", "tea-a", "admin")
	b := &Base{Authz: m, Workspace: fakeWorkspace{"key-2": "tea-a"}, KeyCreators: keyCreators{creators: map[string]string{"key-1": "person-a", "key-2": "key-1"}}}
	ctx := keyContext("key-2")
	if err := b.Authorize(ctx, RelCanCreate); !errors.Is(err, ErrForbidden) {
		t.Fatalf("key-2 can_create = %v, want ErrForbidden: its chain ends at a Viewer", err)
	}
	if err := b.Authorize(ctx, RelCanView); err != nil {
		t.Fatalf("key-2 can_view = %v, want allowed", err)
	}
	// A chain longer than any real one fails closed.
	b.KeyCreators = keyCreators{creators: map[string]string{"key-2": "key-1", "key-1": "key-0", "key-0": "key-2"}}
	m.grant("key-0", "tea-a", "admin")
	if err := b.Authorize(ctx, RelCanView); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a cyclic chain = %v, want ErrForbidden", err)
	}
}

// AuthorizeExportClass (webhook create and repoint): the mint-class callers,
// plus a bex API key a person minted. A key with no recorded creator, any
// other machine client and a delegated third-party token stay refused.
func TestAuthorizeExportClassAdmitsOnlyAttributedKeysBeyondTheMintClass(t *testing.T) {
	b := &Base{KeyCreators: keyCreators{creators: map[string]string{"key-a": "person-a", "key-old": ""}}}
	cases := []struct {
		name string
		id   Identity
		want error
	}{
		{"session", Identity{Subject: "person-a", Method: MethodSession}, nil},
		{"attributed key", Identity{Subject: "key-a", ClientID: "key-a", Method: MethodOAuth2}, nil},
		{"key with no creator", Identity{Subject: "key-old", ClientID: "key-old", Method: MethodOAuth2}, ErrForbidden},
		{"other machine client", Identity{Subject: "svc", ClientID: "svc", Method: MethodOAuth2}, ErrForbidden},
		{"delegated third-party token", Identity{Subject: "person-a", ClientID: "third-party", Method: MethodOAuth2, Human: true, CanonicalScopes: "bex.read bex.write"}, ErrForbidden},
	}
	for _, tc := range cases {
		err := b.AuthorizeExportClass(WithIdentity(context.Background(), tc.id))
		if (tc.want == nil) != (err == nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Errorf("%s: AuthorizeExportClass = %v, want %v", tc.name, err, tc.want)
		}
		// The mint verbs themselves stay human-only (I4).
		if tc.name == "attributed key" {
			if err := b.AuthorizeMintClass(WithIdentity(context.Background(), tc.id)); !errors.Is(err, ErrForbidden) {
				t.Errorf("AuthorizeMintClass(attributed key) = %v, want ErrForbidden", err)
			}
		}
	}
	b.KeyCreators = keyCreators{err: errors.New("hydra down")}
	if err := b.AuthorizeExportClass(keyContext("key-a")); !errors.Is(err, ErrAuthzUnavailable) {
		t.Errorf("Hydra down = %v, want ErrAuthzUnavailable", err)
	}
	// With no resolver nothing can attribute a key, so none passes.
	b.KeyCreators = nil
	if err := b.AuthorizeExportClass(keyContext("key-a")); !errors.Is(err, ErrForbidden) {
		t.Errorf("no resolver = %v, want ErrForbidden", err)
	}
}

// The property of I1 and I2 over random histories: whatever sequence of role
// changes, failed or partial tuple writes and re-seats happens, a key's
// allowed relations on every workspace are a subset of its creator's at that
// moment, and empty outside the workspace it was minted in. I3 rides along:
// the unattributed key never holds more than a Developer.
func TestKeyNeverExceedsItsCreatorOverRandomHistories(t *testing.T) {
	roles := []string{"admin", "developer", "contributor", "viewer", "billing"}
	workspaces := []string{"tea-a", "tea-b"}
	rng := rand.New(rand.NewPCG(481, 1))
	for history := range 200 {
		m := newRoleModel()
		creators := keyCreators{creators: map[string]string{"key-a1": "person-a", "key-a2": "person-a", "key-b": "person-b", "key-old": ""}}
		home := map[string]string{"key-a1": "tea-a", "key-a2": "tea-a", "key-b": "tea-a", "key-old": "tea-a"}
		b := &Base{Authz: m, Workspace: fakeWorkspace(home), KeyCreators: creators}
		m.grant("person-a", "tea-a", "admin")
		m.grant("person-a", "tea-b", "admin")
		m.grant("person-b", "tea-a", "developer")
		m.grant("key-old", "tea-a", roles[rng.IntN(len(roles))])
		for step := range 30 {
			subject := []string{"person-a", "person-b", "key-a1", "key-a2", "key-b"}[rng.IntN(5)]
			workspace := workspaces[rng.IntN(2)]
			role := roles[rng.IntN(len(roles))]
			switch rng.IntN(4) {
			case 0: // a grant that landed (a re-seat, or a stale write)
				m.grant(subject, workspace, role)
			case 1: // a revoke that landed
				m.revoke(subject, workspace, role)
			case 2: // a role change: the person's tuple moves, their keys' may not
				for _, r := range roles {
					m.revoke(subject, workspace, r)
				}
				m.grant(subject, workspace, role)
			case 3: // nothing: a write that failed
			}
			for key, creator := range creators.creators {
				for _, ws := range workspaces {
					for _, rel := range modelRelations {
						ctx := keyContext(key)
						keyAllowed := b.AuthorizeOn(ctx, rel, WorkspaceObject(ws)) == nil
						if !keyAllowed {
							continue
						}
						switch {
						case ws != home[key]:
							t.Fatalf("history %d step %d: %s allowed %s in %s, outside its workspace %s", history, step, key, rel, ws, home[key])
						case creator == "" && !slices.Contains(modelRoleRelations["developer"], rel):
							t.Fatalf("history %d step %d: unattributed %s allowed %s", history, step, key, rel)
						case creator != "":
							person := WithIdentity(context.Background(), Identity{Subject: creator, Method: MethodSession})
							if b.AuthorizeOn(person, rel, WorkspaceObject(ws)) != nil {
								t.Fatalf("history %d step %d: %s allowed %s in %s, which its creator %s is not (tuples %v)",
									history, step, key, rel, ws, creator, describe(m))
							}
						}
					}
				}
			}
		}
	}
}

func describe(m *roleModel) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for key, roles := range m.tuples {
		for role := range roles {
			out = append(out, key+"="+role)
		}
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}
