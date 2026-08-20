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

package sshkeys

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakePlatformResolver answers IsPlatformClient from a fixed map.
type fakePlatformResolver map[string]bool

func (f fakePlatformResolver) IsPlatformClient(_ context.Context, clientID string) (bool, error) {
	return f[clientID], nil
}

func (f fakePlatformResolver) IsPlatformClientFresh(ctx context.Context, clientID string) (bool, error) {
	return f.IsPlatformClient(ctx, clientID)
}

// codex round-7 F3 — SSH-key enrollment is durable-credential minting.
//
// The enrolled key binds to the caller's subject with no provenance, and the
// gateway later authenticates it by fingerprint alone — zero dependence on the
// OAuth consent that minted it. A consented third-party client or a machine
// token must therefore not enroll one (AuthorizeMintClass), and the
// can_manage_ssh_keys relation is re-asserted uncached (the round-5 finding-4
// symmetry CreateAPIKey already had) so a just-revoked member cannot ride a
// stale cached positive into a new key.
func TestCreateRequiresMintCredentialClass(t *testing.T) {
	cases := []struct {
		name     string
		id       core.Identity
		platform core.PlatformClientResolver
		want     error // nil = the enrollment succeeds
	}{
		{
			name: "machine client_credentials token",
			id:   core.Identity{Subject: "key-abc", Method: "oauth2", ClientID: "key-abc"},
			want: core.ErrForbidden,
		},
		{
			name:     "third-party OAuth human token",
			id:       core.Identity{Subject: "user-a", Method: "oauth2", ClientID: "dcr-agent-7", Human: true},
			platform: fakePlatformResolver{"platform-cli": true},
			want:     core.ErrForbidden,
		},
		{
			name:     "human OAuth token with no resolver wired fails closed",
			id:       core.Identity{Subject: "user-a", Method: "oauth2", ClientID: "platform-cli", Human: true},
			platform: nil,
			want:     core.ErrForbidden,
		},
		{
			name: "platform OAuth human token (official CLI)",
			id: core.Identity{
				Subject: "user-a", Method: "oauth2", ClientID: "platform-cli",
				Human: true, PlatformClient: true, CanonicalScopes: core.ScopeWrite,
			},
			platform: fakePlatformResolver{"platform-cli": true},
			want:     nil,
		},
		{
			name: "direct Kratos session",
			id:   core.Identity{Subject: "user-a", Method: "session"},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := &memoryStore{keys: map[string]store.SSHKey{}}
			svc := &Service{Base: &core.Base{PlatformClients: tc.platform}, Store: st}
			ctx := core.WithIdentity(context.Background(), tc.id)

			_, err := svc.Create(ctx, "laptop", publicKey(t))
			if tc.want == nil && err != nil {
				t.Fatalf("enroll => %v, want success", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("enroll => %v, want %v", err, tc.want)
			}
			if tc.want != nil && len(st.keys) != 0 {
				t.Fatalf("denied credential class enrolled a key anyway: %d keys", len(st.keys))
			}
		})
	}
}

// freshRelationChecker records which relations were re-asserted uncached.
type freshRelationChecker struct{ fresh []string }

func (c *freshRelationChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (c *freshRelationChecker) CheckFresh(_ context.Context, _, relation, _ string) (bool, error) {
	c.fresh = append(c.fresh, relation)
	return true, nil
}

// TestCreateFreshAssertsMembership pins the AuthorizeFresh half of the gate:
// enrollment is a durable-credential issuance verb, so can_manage_ssh_keys is
// consulted uncached even on a checker whose cached Check would say yes.
func TestCreateFreshAssertsMembership(t *testing.T) {
	chk := &freshRelationChecker{}
	svc := &Service{
		Base:  &core.Base{Authz: chk},
		Store: &memoryStore{keys: map[string]store.SSHKey{}},
	}
	if _, err := svc.Create(identity("user-a"), "laptop", publicKey(t)); err != nil {
		t.Fatal(err)
	}
	for _, rel := range chk.fresh {
		if rel == core.RelCanManageSSHKeys {
			return
		}
	}
	t.Errorf("Create never re-asserted %s uncached; fresh relations seen: %v", core.RelCanManageSSHKeys, chk.fresh)
}
