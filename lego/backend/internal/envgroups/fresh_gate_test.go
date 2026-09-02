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

package envgroups

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// staleAllowChecker models the codex round-8 #8 window: the cached path (Check)
// still answers a warm positive while the source of truth (CheckFresh) already
// says the membership is gone — a member revoked on another replica inside
// PositiveTTL.
type staleAllowChecker struct{}

func (staleAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (staleAllowChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	return false, nil
}

// codex round-8 #8: a per-key reveal must re-assert can_view_sensitive
// uncached against the group's owning workspace — a revoked member riding a
// stale positive must not reveal one last value.
func TestGetEnvGroupVarFailsClosedOnFreshRevocation(t *testing.T) {
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: multiWorkspace{"dana": {"tea-a"}}},
		Store: newFakeStore(),
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})

	// Seed a workspace-attributed group holding one secret value (authz nil
	// allows the writes), then revoke: Check keeps answering the warm positive
	// while CheckFresh says the membership is gone.
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if _, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "TOKEN", Value: "topsecret"}}); err != nil {
		t.Fatalf("seed vars: %v", err)
	}
	svc.Authz = staleAllowChecker{}

	_, err = svc.GetEnvGroupVar(ctx, g.ID, "TOKEN")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("GetEnvGroupVar on a stale positive: %v, want ErrForbidden", err)
	}
	if strings.Contains(err.Error(), "topsecret") {
		t.Errorf("denial leaked the value: %v", err)
	}
}

// codex round-9 #7: a group secret FILE is the same reveal sink as a var — a
// revoked member riding a stale positive must not read one last file either.
func TestGetEnvGroupFileFailsClosedOnFreshRevocation(t *testing.T) {
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: multiWorkspace{"dana": {"tea-a"}}},
		Store: newFakeStore(),
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})

	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if _, err := svc.SetEnvGroupFile(ctx, g.ID, "credentials.json", `{"token":"topsecret"}`); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	svc.Authz = staleAllowChecker{}

	_, err = svc.GetEnvGroupFile(ctx, g.ID, "credentials.json")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("GetEnvGroupFile on a stale positive: %v, want ErrForbidden", err)
	}
	if strings.Contains(err.Error(), "topsecret") {
		t.Errorf("denial leaked the file content: %v", err)
	}
}

type sensitiveRevokedChecker struct{}

func (sensitiveRevokedChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (sensitiveRevokedChecker) CheckFresh(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != core.RelCanViewSensitive, nil
}

func TestEnvGroupLinksRequireSensitiveCapability(t *testing.T) {
	for _, tc := range []struct {
		name string
		link func(*Service, context.Context, string) error
	}{
		{name: "direct", link: func(s *Service, ctx context.Context, gid string) error {
			return s.LinkService(ctx, gid, "web")
		}},
		{name: "blueprint", link: func(s *Service, ctx context.Context, _ string) error {
			return s.LinkEnvGroup(ctx, "shared", "web")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := multiWorkspace{"seed": {"tea-a"}, "writer": {"tea-a"}, "developer": {"tea-a"}}
			svc := &Service{
				Base:  &core.Base{Client: fakeClient(ownedApp("web", "tea-a")), Namespace: "default", Workspace: resolver},
				Store: newFakeStore(),
			}
			seed := core.WithIdentity(context.Background(), core.Identity{Subject: "seed", Method: "session"})
			group, err := svc.CreateEnvGroup(seed, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatalf("seed group: %v", err)
			}
			if _, err := svc.SetEnvGroupVars(seed, group.ID, []EnvVarView{{Key: "TOKEN", Value: "topsecret"}}); err != nil {
				t.Fatalf("seed vars: %v", err)
			}

			writeOnly := core.WithIdentity(context.Background(), core.Identity{
				Subject: "writer", Method: "oauth2", Human: true, CanonicalScopes: core.ScopeWrite,
			})
			if err := tc.link(svc, writeOnly, group.ID); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("write-only link: %v, want ErrForbidden", err)
			}
			if app := getApp(t, svc.Client, "web"); len(app.Spec.EnvFromSecrets) != 0 || len(app.Spec.FilesFromSecrets) != 0 {
				t.Fatalf("denied link materialized secrets: env=%v files=%v", app.Spec.EnvFromSecrets, app.Spec.FilesFromSecrets)
			}
			if view, err := svc.GetEnvGroup(context.Background(), group.ID); err != nil || len(view.ServiceLinks) != 0 {
				t.Fatalf("denied link changed group metadata: links=%v err=%v", view.ServiceLinks, err)
			}

			allowed := core.WithIdentity(context.Background(), core.Identity{
				Subject: "developer", Method: "oauth2", Human: true,
				CanonicalScopes: core.ScopeWrite + " " + core.ScopeSensitive,
			})
			if err := tc.link(svc, allowed, group.ID); err != nil {
				t.Fatalf("write+sensitive link: %v", err)
			}
			if app := getApp(t, svc.Client, "web"); len(app.Spec.EnvFromSecrets) != 1 || len(app.Spec.FilesFromSecrets) != 1 {
				t.Fatalf("allowed link did not materialize group secrets: env=%v files=%v", app.Spec.EnvFromSecrets, app.Spec.FilesFromSecrets)
			}
			if err := tc.link(svc, writeOnly, group.ID); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("write-only re-link: %v, want ErrForbidden before idempotent return", err)
			}
		})
	}
}

func TestEnvGroupLinksFailClosedOnFreshSensitiveRevocation(t *testing.T) {
	for _, tc := range []struct {
		name string
		link func(*Service, context.Context, string) error
	}{
		{name: "direct", link: func(s *Service, ctx context.Context, gid string) error {
			return s.LinkService(ctx, gid, "web")
		}},
		{name: "blueprint", link: func(s *Service, ctx context.Context, _ string) error {
			return s.LinkEnvGroup(ctx, "shared", "web")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(newFakeStore(), sampleApp("web"))
			group, err := svc.CreateEnvGroup(context.Background(), CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatalf("seed group: %v", err)
			}
			svc.Authz = sensitiveRevokedChecker{}
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "revoked", Method: "session"})

			if err := tc.link(svc, ctx, group.ID); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("fresh-sensitive denial: %v, want ErrForbidden", err)
			}
			if app := getApp(t, svc.Client, "web"); len(app.Spec.EnvFromSecrets) != 0 || len(app.Spec.FilesFromSecrets) != 0 {
				t.Fatalf("revoked link materialized secrets: env=%v files=%v", app.Spec.EnvFromSecrets, app.Spec.FilesFromSecrets)
			}
		})
	}
}
