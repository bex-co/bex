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

package deploys

import (
	"context"
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakePlatformResolver answers IsPlatformClient from a fixed map.
type fakePlatformResolver map[string]bool

func (f fakePlatformResolver) IsPlatformClient(_ context.Context, clientID string) (bool, error) {
	return f[clientID], nil
}

func (f fakePlatformResolver) IsPlatformClientFresh(ctx context.Context, clientID string) (bool, error) {
	return f.IsPlatformClient(ctx, clientID)
}

// codex round-7 F3, completing the fix — a deploy-hook URL is a durable bearer
// credential, so handing one out is a mint verb and is reserved for direct
// human callers.
//
// The round-7 remediation put AuthorizeMintClass on API-key creation and
// SSH-key enrollment but not here, even though the deploy-hook URL is the third
// credential in the codebase that authenticates INDEPENDENTLY of whatever
// obtained it: DeployHookHandler is an open, unauthenticated endpoint keyed
// only on the token in the path, and a replay carries ref/imgURL — i.e. deploy
// an arbitrary image to that service. So a client_credentials machine key or a
// consented third-party OAuth client could convert its revocable, time-bounded
// access into a capability that outlives revocation.
//
// Both verbs are gated, not just rotation: an attacker does not need a FRESH
// token, only a working one, so gating rotation alone would leave the identical
// capability one GET away. On a service that has no hook yet, the read is
// itself the mint (writeDeployHookToken lazily creates one).
func TestDeployHookVerbsRequireMintCredentialClass(t *testing.T) {
	verbs := []struct {
		name string
		call func(*Service, context.Context) error
	}{
		{"reveal", func(s *Service, ctx context.Context) error { _, err := s.GetDeployHook(ctx, "web"); return err }},
		{"rotate", func(s *Service, ctx context.Context) error { _, err := s.RegenerateDeployHook(ctx, "web"); return err }},
	}
	cases := []struct {
		name     string
		id       core.Identity
		platform core.PlatformClientResolver
		want     error // nil = the verb is allowed through the class gate
	}{
		{
			name: "machine client_credentials token cannot obtain a durable hook",
			id:   core.Identity{Subject: "key-abc", Method: "oauth2", ClientID: "key-abc"},
			want: core.ErrForbidden,
		},
		{
			name:     "third-party OAuth client cannot outlive its consent",
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
			name: "platform OAuth human token (official CLI) is allowed",
			id: core.Identity{
				Subject: "user-a", Method: "oauth2", ClientID: "platform-cli",
				Human: true, PlatformClient: true, CanonicalScopes: core.ScopeWrite + " " + core.ScopeSensitive,
			},
			platform: fakePlatformResolver{"platform-cli": true},
			want:     nil,
		},
		{
			name: "direct Kratos session (dashboard) is allowed",
			id:   core.Identity{Subject: "user-a", Method: "session"},
			want: nil,
		},
	}
	for _, verb := range verbs {
		for _, tc := range cases {
			t.Run(verb.name+"/"+tc.name, func(t *testing.T) {
				svc, cl := newService(newFakeStore(), sampleApp("web", "srv-1"))
				svc.Base.PlatformClients = tc.platform
				ctx := core.WithIdentity(context.Background(), tc.id)

				err := verb.call(svc, ctx)
				if tc.want == nil && err != nil {
					t.Fatalf("%s => %v, want the class gate to allow it", verb.name, err)
				}
				if tc.want != nil && !errors.Is(err, tc.want) {
					t.Fatalf("%s => %v, want %v", verb.name, err, tc.want)
				}

				// The decisive assertion: a refused class must not have written a
				// token. Returning an error while persisting one would still leak a
				// working credential to anyone who can read the App.
				var app appv1alpha1.App
				if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &app); err != nil {
					t.Fatal(err)
				}
				got := app.Annotations[DeployHookTokenAnnotation]
				if tc.want != nil && got != "" {
					t.Fatalf("%s: denied credential class still minted a deploy-hook token", verb.name)
				}
				if tc.want == nil && got == "" {
					t.Fatalf("%s: allowed caller got no deploy-hook token", verb.name)
				}
			})
		}
	}
}
