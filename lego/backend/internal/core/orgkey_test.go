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
	"testing"
)

// orgKeyStore is OrganizationKeyAuthority and KeyGrantResolver over maps: the
// org each key belongs to, which keys are live, and each workspace's org.
type orgKeyStore struct {
	keyOrg     map[string]string
	live       map[string]bool
	workspaces map[string]string
	err        error
}

func (s orgKeyStore) APIKeyGrant(_ context.Context, clientID string) (KeyGrant, error) {
	if org, ok := s.keyOrg[clientID]; ok {
		return OrganizationKeyGrant(org), nil
	}
	return AllKeyGrant(), nil
}

func (s orgKeyStore) LiveOrganizationAPIKey(_ context.Context, clientID string) (string, bool, error) {
	if s.err != nil {
		return "", false, s.err
	}
	return s.keyOrg[clientID], s.live[clientID], nil
}

func (s orgKeyStore) WorkspaceOrganizationID(_ context.Context, workspaceID string) (string, error) {
	return s.workspaces[workspaceID], s.err
}

// orgTwoFactor refuses every check on the objects it names.
type orgTwoFactor map[string]bool

func (g orgTwoFactor) Check(_ context.Context, workspaceID string, _ Identity) error {
	if g[WorkspaceObject(workspaceID)] {
		return NewForbiddenError(CodeWorkspaceTwoFactorRequired, "2FA", nil)
	}
	return nil
}

func (g orgTwoFactor) CheckOrganization(_ context.Context, orgID string, _ Identity) error {
	if g[OrganizationObject(orgID)] {
		return NewForbiddenError(CodeWorkspaceTwoFactorRequired, "2FA", nil)
	}
	return nil
}

// An org key reads its own org's log and the log of each workspace its org
// holds at request time, and nothing else; a revoked key, a lookup that fails
// and the org's login requirements refuse it, and every refusal is audited.
// OpenFGA is never asked about the key: it refuses everything.
func TestAuthorizeAuditLogForAnOrganizationKey(t *testing.T) {
	keys := orgKeyStore{
		keyOrg:     map[string]string{"key-a": "org-a", "key-revoked": "org-a"},
		live:       map[string]bool{"key-a": true},
		workspaces: map[string]string{"tea-a1": "org-a", "tea-a2": "org-a", "tea-b1": "org-b"},
	}
	sink := &fakeAuditSink{}
	twoFactor := orgTwoFactor{}
	b := &Base{Authz: fakeDenyChecker{}, KeyGrants: keys, OrganizationKeys: keys, TwoFactor: twoFactor, Audit: sink}
	as := func(clientID string) context.Context {
		return WithIdentity(context.Background(), Identity{Subject: clientID, ClientID: clientID, Method: MethodOAuth2})
	}
	key := as("key-a")

	if orgKey, err := b.AuthorizeOrganizationAuditLog(key, "org-a"); err != nil || !orgKey {
		t.Errorf("its own org's log: %v (org key %v)", err, orgKey)
	}
	for _, ws := range []string{"tea-a1", "tea-a2"} {
		if got, err := b.AuthorizeWorkspaceAuditLog(key, ws); err != nil || got != ws {
			t.Errorf("org workspace %s: %q %v", ws, got, err)
		}
	}
	denied := sink.len()
	for name, err := range map[string]error{
		"another org's log":         second(b.AuthorizeOrganizationAuditLog(key, "org-b")),
		"another org's workspace":   second(b.AuthorizeWorkspaceAuditLog(key, "tea-b1")),
		"a standalone workspace":    second(b.AuthorizeWorkspaceAuditLog(key, "tea-solo")),
		"no workspace named":        second(b.AuthorizeWorkspaceAuditLog(key, "")),
		"a revoked key, its org":    second(b.AuthorizeOrganizationAuditLog(as("key-revoked"), "org-a")),
		"a revoked key, workspaces": second(b.AuthorizeWorkspaceAuditLog(as("key-revoked"), "tea-a1")),
	} {
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: %v, want ErrForbidden", name, err)
		}
	}
	if got := sink.len() - denied; got != 6 {
		t.Errorf("%d refusals audited, want 6", got)
	}

	// A workspace the org gains is read at once; one it loses is not.
	keys.workspaces["tea-a3"] = "org-a"
	delete(keys.workspaces, "tea-a2")
	if _, err := b.AuthorizeWorkspaceAuditLog(key, "tea-a3"); err != nil {
		t.Errorf("a workspace that joined the org: %v", err)
	}
	if _, err := b.AuthorizeWorkspaceAuditLog(key, "tea-a2"); !errors.Is(err, ErrForbidden) {
		t.Errorf("a workspace that left the org: %v", err)
	}

	// The login requirements of what it reads hold it, as any key.
	twoFactor[OrganizationObject("org-a")] = true
	twoFactor[WorkspaceObject("tea-a1")] = true
	if _, err := b.AuthorizeOrganizationAuditLog(key, "org-a"); !IsLoginRefusal(err) {
		t.Errorf("an org requiring 2FA: %v", err)
	}
	if _, err := b.AuthorizeWorkspaceAuditLog(key, "tea-a1"); !IsLoginRefusal(err) {
		t.Errorf("a workspace requiring 2FA: %v", err)
	}

	// A lookup that cannot answer fails closed.
	failing := &Base{Authz: fakeDenyChecker{}, KeyGrants: keys, OrganizationKeys: orgKeyStore{keyOrg: keys.keyOrg, err: errors.New("db down")}}
	if _, err := failing.AuthorizeOrganizationAuditLog(key, "org-a"); !errors.Is(err, ErrAuthzUnavailable) {
		t.Errorf("an unreadable key: %v, want ErrAuthzUnavailable", err)
	}
	unwired := &Base{Authz: fakeDenyChecker{}, KeyGrants: keys}
	if _, err := unwired.AuthorizeOrganizationAuditLog(key, "org-a"); !errors.Is(err, ErrForbidden) {
		t.Errorf("no authority wired: %v, want ErrForbidden", err)
	}
}

// A person and a workspace key take the funnel every verb takes: OpenFGA's
// can_manage decides, exactly as before org keys.
func TestAuthorizeAuditLogForEveryoneElse(t *testing.T) {
	keys := orgKeyStore{keyOrg: map[string]string{}}
	person := WithIdentity(context.Background(), Identity{Subject: "admin", Method: MethodSession})
	workspaceKey := WithIdentity(context.Background(), Identity{Subject: "ws-key", ClientID: "ws-key", Method: MethodOAuth2})
	allow := &Base{Authz: &fakeAllowChecker{}, KeyGrants: keys, OrganizationKeys: keys}
	deny := &Base{Authz: fakeDenyChecker{}, KeyGrants: keys, OrganizationKeys: keys}
	for _, ctx := range []context.Context{person, workspaceKey} {
		if _, err := allow.AuthorizeWorkspaceAuditLog(ctx, "tea-a"); err != nil {
			t.Errorf("allowed by OpenFGA: %v", err)
		}
		if orgKey, err := allow.AuthorizeOrganizationAuditLog(ctx, "org-a"); err != nil || orgKey {
			t.Errorf("allowed by OpenFGA: %v (org key %v)", err, orgKey)
		}
		if _, err := deny.AuthorizeWorkspaceAuditLog(ctx, "tea-a"); !errors.Is(err, ErrForbidden) {
			t.Errorf("refused by OpenFGA: %v", err)
		}
	}
	if got := allow.Authz.(*fakeAllowChecker).lastObject; got != OrganizationObject("org-a") {
		t.Errorf("OpenFGA was asked about %q", got)
	}
}

func second[T any](_ T, err error) error { return err }
