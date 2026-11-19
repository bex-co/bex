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

package webhooks

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// w4/079 — createWebhookEndpoint mints a show-once signing secret; delegated
// OAuth callers must not satisfy AuthorizeMintClass (same bar as API keys).
func TestCreateRequiresMintCredentialClass(t *testing.T) {
	cases := []struct {
		name     string
		id       core.Identity
		platform core.PlatformClientResolver
		want     error
	}{
		{
			name: "machine client_credentials token cannot mint webhook secret",
			id:   core.Identity{Subject: "key-abc", Method: "oauth2", ClientID: "key-abc"},
			want: core.ErrForbidden,
		},
		{
			name:     "third-party OAuth human token cannot mint webhook secret",
			id:       core.Identity{Subject: "user-a", Method: "oauth2", ClientID: "dcr-agent-7", Human: true, CanonicalScopes: core.ScopeWrite},
			platform: coretest.PlatformClients{"platform-cli": true},
			want:     core.ErrForbidden,
		},
		{
			name: "platform OAuth human token may mint",
			id: core.Identity{
				Subject: "user-a", Method: "oauth2", ClientID: "platform-cli",
				Human: true, PlatformClient: true, CanonicalScopes: core.ScopeWrite,
			},
			platform: coretest.PlatformClients{"platform-cli": true},
			want:     nil,
		},
		{
			name: "direct Kratos session may mint",
			id:   core.Identity{Subject: "user-a", Method: "session"},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeEndpointStore()
			s := &Service{
				Base:  &core.Base{Namespace: "default", PlatformClients: tc.platform},
				Store: st,
			}
			ctx := core.WithIdentity(context.Background(), tc.id)
			_, err := s.Create(ctx, CreateRequest{
				Name: "hook", URL: "https://example.com/h", EventTypes: []string{TypeDeployEnded}, Enabled: true,
			})
			if tc.want == nil && err != nil {
				t.Fatalf("Create => %v, want success", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Create => %v, want %v", err, tc.want)
			}
			if tc.want != nil && len(st.rows) != 0 {
				t.Fatalf("denied credential class minted a webhook endpoint anyway: %d rows", len(st.rows))
			}
		})
	}
}

// Round 24 (hardening of H-1's class): repointing an endpoint exports the
// activity stream to a new destination under its durable secret, so a
// delegated third-party token may rename or disable an endpoint but not move
// it, and reads its URL and secret redacted.
func TestRepointAndSecretReadRequireMintCredentialClass(t *testing.T) {
	st := newFakeEndpointStore()
	// user-a is the workspace admin, so only the credential class can decide.
	s := &Service{
		Base:  &core.Base{Namespace: "default", PlatformClients: coretest.PlatformClients{"platform-cli": true}, Authz: manageChecker{admin: "user-a"}},
		Store: st,
	}
	session := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})
	created, err := s.Create(session, CreateRequest{
		Name: "hook", URL: "https://example.com/h", EventTypes: []string{TypeDeployEnded}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("session Create: %v", err)
	}
	thirdParty := core.WithIdentity(context.Background(), core.Identity{
		Subject: "user-a", Method: "oauth2", ClientID: "dcr-agent-7", Human: true, CanonicalScopes: core.ScopeWrite,
	})
	elsewhere := "https://attacker.example/collect"
	if _, err := s.Update(thirdParty, "", created.ID, UpdateRequest{URL: &elsewhere}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("third-party repoint => %v, want ErrForbidden", err)
	}
	renamed := "renamed"
	if _, err := s.Update(thirdParty, "", created.ID, UpdateRequest{Name: &renamed}); err != nil {
		t.Fatalf("third-party rename => %v, want success: only the destination is gated", err)
	}
	for id, row := range st.rows {
		if row.URL != "https://example.com/h" {
			t.Fatalf("endpoint %s repointed to %q by a refused token", id, row.URL)
		}
	}
	if !s.mayManageWorkspace(session, "tea-1") {
		t.Fatal("the admin's own session is not a manager; the check below would prove nothing")
	}
	if s.mayManageWorkspace(thirdParty, "tea-1") {
		t.Fatal("a delegated third-party token is treated as a manager, so it reads the signing secret")
	}
}

// adminsChecker grants can_manage to every subject in admins and can_view to
// everyone, so the key ceiling (a key is allowed only what its creator is)
// has a creator to consult.
type adminsChecker map[string]bool

func (c adminsChecker) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	if relation == core.RelCanView {
		return true, nil
	}
	return relation == core.RelCanManage && c[strings.TrimPrefix(subject, "user:")], nil
}

// w9/m481: Render's API keys "Create, update, and delete webhooks". An
// Admin's bex API key creates an endpoint (and gets its secret), repoints it
// and reads it unredacted; a Developer's key is refused by the role, and a key
// with no recorded creator or any other machine client by the credential
// class, as before.
func TestAnAdminsAPIKeyManagesWebhooksAsTheAdminDoes(t *testing.T) {
	st := newFakeEndpointStore()
	s := &Service{
		Base: &core.Base{
			Namespace: "default",
			// key-dev's own tuple says admin (a revoke that failed after its
			// creator's demotion), so only the ceiling refuses it.
			Authz: adminsChecker{"user-a": true, "key-admin": true, "key-dev": true, "key-old": true, "svc": true},
			KeyCreators: coretest.KeyCreators{
				"key-admin": "user-a", "key-dev": "user-d", "key-old": "",
			},
		},
		Store: st,
	}
	keyCtx := func(id string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{Subject: id, ClientID: id, Method: core.MethodOAuth2})
	}
	create := func(ctx context.Context, name string) (EndpointView, error) {
		return s.Create(ctx, CreateRequest{Name: name, URL: "https://example.com/h", EventTypes: []string{TypeDeployEnded}, Enabled: true})
	}

	created, err := create(keyCtx("key-admin"), "hook")
	if err != nil {
		t.Fatalf("an Admin's key Create => %v, want success", err)
	}
	if created.Secret == "" {
		t.Fatal("an Admin's key got no signing secret back")
	}
	elsewhere := "https://example.com/elsewhere"
	if _, err := s.Update(keyCtx("key-admin"), "", created.ID, UpdateRequest{URL: &elsewhere}); err != nil {
		t.Fatalf("an Admin's key repoint => %v, want success", err)
	}
	if !s.mayManageWorkspace(keyCtx("key-admin"), "tea-1") {
		t.Fatal("an Admin's key reads the endpoint redacted, so Terraform loses its secret")
	}

	for _, refused := range []string{"key-dev", "key-old", "svc"} {
		if _, err := create(keyCtx(refused), "hook-"+refused); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s Create => %v, want ErrForbidden", refused, err)
		}
		away := "https://attacker.example/" + refused
		if _, err := s.Update(keyCtx(refused), "", created.ID, UpdateRequest{URL: &away}); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s repoint => %v, want ErrForbidden", refused, err)
		}
		if s.mayManageWorkspace(keyCtx(refused), "tea-1") {
			t.Errorf("%s reads the signing secret", refused)
		}
	}
	if len(st.rows) != 1 {
		t.Fatalf("refused callers created endpoints: %d rows", len(st.rows))
	}
}

// w9/m496: an Admin's key reads a webhook's signing secret only with
// Workspaces Read & write, the permission whose "Create, update, and delete
// webhooks" the secret belongs to; a read-only grant reads it redacted, and an
// unreadable grant hides it.
func TestAScopedKeyReadsTheSigningSecretOnlyWithTheWebhooksWrite(t *testing.T) {
	grant := core.KeyGrantFromEntries([]string{"workspaces:read"})
	var grantErr error
	s := &Service{Base: &core.Base{
		Namespace:   "default",
		Authz:       adminsChecker{"user-a": true, "key-admin": true},
		KeyCreators: coretest.KeyCreators{"key-admin": "user-a"},
		KeyGrants: coretest.KeyGrantsFunc(func(context.Context, string) (core.KeyGrant, error) {
			return grant, grantErr
		}),
	}}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "key-admin", ClientID: "key-admin", Method: core.MethodOAuth2})
	if s.mayManageWorkspace(ctx, "tea-1") {
		t.Error("a Workspaces Read key reads the signing secret")
	}
	grant = core.KeyGrantFromEntries([]string{"workspaces:read_write"})
	if !s.mayManageWorkspace(ctx, "tea-1") {
		t.Error("a Workspaces Read & write key reads the endpoint redacted")
	}
	grantErr = errors.New("postgres down")
	if s.mayManageWorkspace(ctx, "tea-1") {
		t.Error("an unreadable grant revealed the signing secret")
	}
}
