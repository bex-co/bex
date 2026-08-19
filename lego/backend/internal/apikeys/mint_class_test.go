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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakePlatformResolver answers IsPlatformClient from a fixed map.
type fakePlatformResolver map[string]bool

func (f fakePlatformResolver) IsPlatformClient(_ context.Context, clientID string) (bool, error) {
	return f[clientID], nil
}

// codex round-7 F3 — minting an API key is reserved for direct human callers.
//
// The relation gates (can_manage_keys, checked fresh) say WHO may manage keys
// but nothing about the credential the request rides on. That let two classes
// mint an independent Hydra client that no longer depends on its parent: a
// compromised API key (client_credentials, legitimately holds can_manage_keys
// as a workspace developer) could self-replicate before revocation, and a
// consented third-party OAuth client could mint a key that persists after its
// consent is revoked. AuthorizeMintClass closes both: only a Kratos session or
// a human token from a bex-provisioned client (official CLI, bex-mobile) may
// mint.
func TestCreateAPIKeyRequiresMintCredentialClass(t *testing.T) {
	cases := []struct {
		name     string
		id       core.Identity
		platform core.PlatformClientResolver
		want     error // nil = the mint succeeds
	}{
		{
			name: "machine client_credentials token cannot self-replicate",
			id:   core.Identity{Subject: "key-abc", Method: "oauth2", ClientID: "key-abc"},
			want: core.ErrForbidden,
		},
		{
			name:     "third-party OAuth human token cannot persist past its consent",
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
			name: "platform OAuth human token (official CLI) may mint",
			id: core.Identity{
				Subject: "user-a", Method: "oauth2", ClientID: "platform-cli",
				Human: true, PlatformClient: true,
			},
			platform: fakePlatformResolver{"platform-cli": true},
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
			store := newFakeKeyStore()
			svc := &Service{Base: &core.Base{Namespace: "default", PlatformClients: tc.platform}, APIKeys: store} // Binding nil — the legacy unbound mint path
			ctx := core.WithIdentity(context.Background(), tc.id)

			_, err := svc.CreateAPIKey(ctx, "", "agent")
			if tc.want == nil && err != nil {
				t.Fatalf("mint => %v, want success", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("mint => %v, want %v", err, tc.want)
			}
			if tc.want != nil && len(store.keys) != 0 {
				t.Fatalf("denied credential class minted a Hydra client anyway: %d keys", len(store.keys))
			}
		})
	}
}
