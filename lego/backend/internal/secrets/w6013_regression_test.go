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

package secrets

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w6013_regression_test.go is the set-env-vars leg of the w6/013 permanent
// regression: an invited-viewer-by-default user must still be able to write
// env vars on a service in a workspace they separately own and administer.
// See apps/w6013_regression_test.go for the fuller writeup (restart/suspend/
// delete) — this is the same bug, same fix (core.Base.AuthorizeApp), the one
// remaining verb the source note names explicitly.

// twoWorkspaces is a multi-workspace caller: subject -> workspaces, oldest
// (default) first.
type twoWorkspaces map[string][]string

func (w twoWorkspaces) Tenant(_ context.Context, id core.Identity) (string, bool) {
	ws := w[id.Subject]
	if len(ws) == 0 {
		return "", false
	}
	return ws[0], true
}

func (w twoWorkspaces) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	for _, t := range w[id.Subject] {
		if t == tenantID {
			return true, nil
		}
	}
	return false, nil
}

// denyWorkspaceChecker denies one relation on one workspace object and allows
// everything else — a caller who is, say, admin of one workspace but only a
// viewer of another.
type denyWorkspaceChecker struct{ relation, object string }

func (c denyWorkspaceChecker) Check(_ context.Context, _, relation, object string) (bool, error) {
	return !(relation == c.relation && object == c.object), nil
}

func TestW6013_InvitedViewerCanSetEnvVarsOnTheirOwnWorkspacesService(t *testing.T) {
	store := newFakeSecretStore()
	svc := &Service{
		Base: &core.Base{
			Client:    fakeClient(tenantApp("mine-web", "tea-mine")),
			Namespace: "default",
			// bob's oldest (default) membership is tea-team — where he was
			// invited as a viewer; he separately owns tea-mine, where
			// "mine-web" actually lives.
			Workspace: twoWorkspaces{"bob": {"tea-team", "tea-mine"}},
			// bob is a viewer of tea-team (can_create denied there) and
			// admin of tea-mine (can_create allowed there).
			Authz: denyWorkspaceChecker{relation: core.RelCanCreate, object: core.WorkspaceObject("tea-team")},
		},
		Store: store,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "bob", Method: "session"})

	if _, err := svc.SetEnvVars(ctx, "mine-web", []EnvVarView{{Key: "FOO", Value: "bar"}}); err != nil {
		t.Fatalf("SetEnvVars on bob's own service: got %v, want it served — "+
			"the service's own workspace (tea-mine) must decide, not bob's default (tea-team)", err)
	}
}
