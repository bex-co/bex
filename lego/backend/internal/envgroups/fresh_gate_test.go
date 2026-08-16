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
