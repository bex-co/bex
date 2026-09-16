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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakePlatformResolver answers IsPlatformClient from a fixed map.
type fakePlatformResolver map[string]bool

func (f fakePlatformResolver) IsPlatformClient(_ context.Context, clientID string) (bool, error) {
	return f[clientID], nil
}

func (f fakePlatformResolver) IsPlatformClientFresh(ctx context.Context, clientID string) (bool, error) {
	return f.IsPlatformClient(ctx, clientID)
}

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
			platform: fakePlatformResolver{"platform-cli": true},
			want:     core.ErrForbidden,
		},
		{
			name: "platform OAuth human token may mint",
			id: core.Identity{
				Subject: "user-a", Method: "oauth2", ClientID: "platform-cli",
				Human: true, PlatformClient: true, CanonicalScopes: core.ScopeWrite,
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
