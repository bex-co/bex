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

package apikeys

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeOrgKeys is OrganizationKeyStore over maps: each person's org role, and
// each org key's row.
type fakeOrgKeys struct {
	roles map[string]string // subject -> role in org-a
	rows  map[string]store.OrganizationAPIKey
	dead  map[string]bool // revoked client ids
}

func newFakeOrgKeys() *fakeOrgKeys {
	return &fakeOrgKeys{
		roles: map[string]string{"owner": core.OrgRoleOwner, "member": core.OrgRoleMember, "guest": core.OrgRoleGuest},
		rows:  map[string]store.OrganizationAPIKey{}, dead: map[string]bool{},
	}
}

func (f *fakeOrgKeys) OrganizationRole(_ context.Context, orgID, subject string) (store.Organization, string, error) {
	role, ok := f.roles[subject]
	if orgID != "org-a" || !ok {
		return store.Organization{}, "", store.ErrNotFound
	}
	return store.Organization{ID: orgID}, role, nil
}

func (f *fakeOrgKeys) InsertOrganizationAPIKey(_ context.Context, k store.OrganizationAPIKey, limit int) error {
	if live, _ := f.ListOrganizationAPIKeys(context.Background(), k.OrganizationID); len(live) >= limit {
		return store.ErrOrganizationAPIKeyLimit
	}
	k.Type, k.CreatedAt = "AUDIT_LOG", time.Date(2026, 11, 26, 0, 0, len(f.rows), 0, time.UTC)
	f.rows[k.ClientID] = k
	return nil
}

func (f *fakeOrgKeys) ListOrganizationAPIKeys(_ context.Context, orgID string) ([]store.OrganizationAPIKey, error) {
	var out []store.OrganizationAPIKey
	for id, k := range f.rows {
		if k.OrganizationID == orgID && !f.dead[id] {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeOrgKeys) RevokeOrganizationAPIKey(_ context.Context, orgID, clientID string) error {
	k, ok := f.rows[clientID]
	if !ok || k.OrganizationID != orgID || f.dead[clientID] {
		return store.ErrNotFound
	}
	f.dead[clientID] = true
	return nil
}

func (f *fakeOrgKeys) RevokeOrganizationAPIKeys(_ context.Context, orgID string) ([]string, error) {
	var ids []string
	for id, k := range f.rows {
		if k.OrganizationID == orgID && !f.dead[id] {
			f.dead[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// orgChecker is OpenFGA's org model: an Owner holds can_manage on org-a, an
// Owner, Member or Guest can_view.
type orgChecker map[string]string

func (c orgChecker) Check(_ context.Context, subject, relation, object string) (bool, error) {
	role := c[strings.TrimPrefix(subject, "user:")]
	if object != core.OrganizationObject("org-a") || role == "" {
		return false, nil
	}
	return relation != core.RelCanManage || role == core.OrgRoleOwner, nil
}

func orgKeyService() (*Service, *fakeKeyStore, *fakeOrgKeys) {
	keys, orgs := newFakeKeyStore(), newFakeOrgKeys()
	checker := orgChecker{}
	for subject, role := range orgs.roles {
		checker[subject] = role
	}
	return &Service{Base: &core.Base{Namespace: "default", Authz: checker}, APIKeys: keys, Organizations: orgs}, keys, orgs
}

func session(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: core.MethodSession})
}

// An Owner mints a key, sees it listed once without its secret, revokes it,
// and a second revoke is not found; the key's client goes with the revoke.
func TestOrganizationKeyLifecycle(t *testing.T) {
	s, keys, _ := orgKeyService()
	owner := session("owner")
	token, err := s.ProvisionOrganizationAuditLogToken(owner, "org-a", "  siem  ")
	if err != nil {
		t.Fatal(err)
	}
	id, secret, ok := ParseBearerKey(token)
	if !ok || keys.keys[id].Name != "siem" || keys.keys[id].CreatedBy != "owner" || secret == "" {
		t.Fatalf("minted %q: client %+v", token, keys.keys[id])
	}
	listed, err := s.ListOrganizationAPITokens(owner, "org-a")
	if err != nil || len(listed) != 1 || listed[0].ID != id || listed[0].Label != "siem" || listed[0].Type != "AUDIT_LOG" || listed[0].ExpiresAt != "" {
		t.Fatalf("list = %+v %v", listed, err)
	}
	if err := s.RevokeOrganizationAPIToken(owner, "org-a", id); err != nil {
		t.Fatal(err)
	}
	if _, ok := keys.keys[id]; ok {
		t.Error("the revoked key's client is still in Hydra")
	}
	if err := s.RevokeOrganizationAPIToken(owner, "org-a", id); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("a second revoke = %v, want not found", err)
	}
	if listed, _ := s.ListOrganizationAPITokens(owner, "org-a"); len(listed) != 0 {
		t.Errorf("after the revoke the list = %+v", listed)
	}
}

// A Member and a Guest are refused 403 FORBIDDEN, never told that an org they
// are in was not found (ORGANIZATION_NOT_FOUND is a 403 too, so the code is
// what tells them apart; production answered it to a Member, w1/m657 t007);
// someone outside the org, and an org that does not exist, answer
// ORGANIZATION_NOT_FOUND, so an org id reveals nothing; an API key cannot
// mint one. Nothing reaches Hydra.
func TestOrganizationKeyRefusals(t *testing.T) {
	s, keys, _ := orgKeyService()
	notFound := func(err error) bool {
		var coded *core.CodedError
		return errors.As(err, &coded) && coded.Code == core.CodeOrganizationNotFound
	}
	forbidden := func(err error) bool { return errors.Is(err, core.ErrForbidden) && !notFound(err) }
	for _, who := range []string{"member", "guest"} {
		ctx := session(who)
		if _, err := s.ProvisionOrganizationAuditLogToken(ctx, "org-a", "x"); !forbidden(err) {
			t.Errorf("%s mint = %v", who, err)
		}
		if _, err := s.ListOrganizationAPITokens(ctx, "org-a"); !forbidden(err) {
			t.Errorf("%s list = %v", who, err)
		}
		if err := s.RevokeOrganizationAPIToken(ctx, "org-a", "key-1"); !forbidden(err) {
			t.Errorf("%s revoke = %v", who, err)
		}
	}
	for _, c := range []struct{ who, org string }{{"stranger", "org-a"}, {"owner", "org-missing"}} {
		ctx := session(c.who)
		if _, err := s.ProvisionOrganizationAuditLogToken(ctx, c.org, "x"); !notFound(err) {
			t.Errorf("%s mint in %s = %v", c.who, c.org, err)
		}
		if _, err := s.ListOrganizationAPITokens(ctx, c.org); !notFound(err) {
			t.Errorf("%s list in %s = %v", c.who, c.org, err)
		}
	}
	machine := core.WithIdentity(context.Background(), core.Identity{Subject: "owner", ClientID: "owner", Method: core.MethodOAuth2})
	if _, err := s.ProvisionOrganizationAuditLogToken(machine, "org-a", "x"); err == nil {
		t.Error("an API key minted an org key")
	}
	// A narrowed human OAuth token lacking can_manage must fail on capability
	// before OpenFGA is consulted, so it gets insufficient_scope (not
	// ORGANIZATION_NOT_FOUND) even for a stranger or an unknown org.
	readOnly := core.WithIdentity(context.Background(), core.Identity{
		Subject:         "stranger",
		Method:          core.MethodOAuth2,
		Human:           true,
		CanonicalScopes: core.ScopeRead,
	})
	var coded *core.CodedError
	if _, err := s.ListOrganizationAPITokens(readOnly, "org-missing"); !errors.As(err, &coded) || coded.Code != core.InsufficientScopeCode {
		t.Errorf("read-only OAuth token on unknown org = %v, want %s", err, core.InsufficientScopeCode)
	}
	// A write-only human OAuth token holds can_manage but lacks can_view: the
	// view gate's capability failure is insufficient_scope too, never an
	// existence answer — the uncovered cell of the same table.
	writeOnly := core.WithIdentity(context.Background(), core.Identity{
		Subject:         "owner",
		Method:          core.MethodOAuth2,
		Human:           true,
		CanonicalScopes: core.ScopeWrite,
	})
	for _, org := range []string{"org-a", "org-missing"} {
		if _, err := s.ListOrganizationAPITokens(writeOnly, org); !errors.As(err, &coded) || coded.Code != core.InsufficientScopeCode {
			t.Errorf("write-only OAuth token listing %s = %v, want %s", org, err, core.InsufficientScopeCode)
		}
		if err := s.RevokeOrganizationAPIToken(writeOnly, org, "key-1"); !errors.As(err, &coded) || coded.Code != core.InsufficientScopeCode {
			t.Errorf("write-only OAuth token revoking in %s = %v, want %s", org, err, core.InsufficientScopeCode)
		}
	}
	if _, err := s.ProvisionOrganizationAuditLogToken(session("owner"), "org-a", "   "); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("a blank label = %v", err)
	}
	if len(keys.keys) != 0 {
		t.Errorf("a refused mint left Hydra clients: %v", keys.keys)
	}
}

// Past the quota, and past the creation rate, the mint answers the one code
// the dashboard reads as Render's "You have created too many keys", and leaves
// no client behind.
func TestOrganizationKeyLimits(t *testing.T) {
	s, keys, _ := orgKeyService()
	owner := session("owner")
	for i := range defaultMaxOrganizationKeys {
		if _, err := s.ProvisionOrganizationAuditLogToken(owner, "org-a", "k"); err != nil {
			t.Fatalf("key %d: %v", i, err)
		}
	}
	limited := func(err error) bool {
		var coded *core.CodedError
		return errors.As(err, &coded) && coded.Code == OrganizationKeyLimitCode
	}
	if _, err := s.ProvisionOrganizationAuditLogToken(owner, "org-a", "one too many"); !limited(err) {
		t.Fatalf("past the quota = %v", err)
	}
	if len(keys.keys) != defaultMaxOrganizationKeys {
		t.Errorf("the refused mint left a client: %d clients", len(keys.keys))
	}

	s, _, _ = orgKeyService()
	s.CreationLimiter = core.NewKeyedRateLimiter[string](1, 1, time.Hour, time.Hour)
	if _, err := s.ProvisionOrganizationAuditLogToken(owner, "org-a", "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ProvisionOrganizationAuditLogToken(owner, "org-a", "second"); !limited(err) {
		t.Fatalf("past the rate = %v", err)
	}
}

// A key whose client is gone (its creator's account was deleted) is not
// listed; the org's end revokes every live key and deletes its client.
func TestOrganizationKeyListAndPurge(t *testing.T) {
	s, keys, orgs := orgKeyService()
	owner := session("owner")
	var ids []string
	for _, label := range []string{"a", "b", "c"} {
		token, err := s.ProvisionOrganizationAuditLogToken(owner, "org-a", label)
		if err != nil {
			t.Fatal(err)
		}
		id, _, _ := ParseBearerKey(token)
		ids = append(ids, id)
	}
	delete(keys.keys, ids[0])
	if listed, _ := s.ListOrganizationAPITokens(owner, "org-a"); len(listed) != 2 {
		t.Errorf("list = %+v, want the two keys whose client lives", listed)
	}
	if err := (OrganizationPurger{Store: keys, Keys: orgs}).PurgeOrganization(context.Background(), "org-a"); err != nil {
		t.Fatal(err)
	}
	if live, _ := orgs.ListOrganizationAPIKeys(context.Background(), "org-a"); len(live) != 0 || len(keys.keys) != 0 {
		t.Errorf("after the purge: rows %v, clients %v", live, keys.keys)
	}
}
