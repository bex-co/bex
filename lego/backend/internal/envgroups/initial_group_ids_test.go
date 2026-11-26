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
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// The create form names its groups by id (w4/731). The first App write
// already carries each group's refs, in creation order, and the group lists
// the service, whatever order the ids came in; an id named twice links once.
func TestInitialGroupIDsExistOnFirstAppWrite(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	var ids []string
	for _, name := range []string{"older", "newer"} {
		group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, group.ID)
	}
	app := managedApp("web")
	wantEnv := []string{envSecretName(ids[0]), envSecretName(ids[1])}
	created := false
	err := svc.WithInitialEnvGroupIDs(ctx, []string{ids[1], ids[0], ids[1]}, app, func() error {
		if !reflect.DeepEqual(app.Spec.EnvFromSecrets, wantEnv) {
			t.Fatalf("first App write envFromSecrets = %v, want %v", app.Spec.EnvFromSecrets, wantEnv)
		}
		created = true
		return svc.Client.Create(ctx, app)
	}, nil)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	for _, gid := range ids {
		links, err := linksOf(svc, gid)
		if err != nil || !reflect.DeepEqual(links, []string{core.AppPublicID(app)}) {
			t.Fatalf("group %s links = %v (%v), want the new service once", gid, links, err)
		}
	}
}

// An id the acting workspace does not hold reads as a group that does not
// exist, whether it is malformed, unknown or another workspace's, so the
// answer names nothing of another workspace; a scope mismatch is the link's
// own 409. Each refusal comes before any write, from the plan's check as from
// the link itself.
func TestInitialGroupIDsKeepWorkspaceAndEnvironmentBoundaries(t *testing.T) {
	ctx := core.WithIdentity(testCtx(), core.Identity{Subject: "owner", Method: "session"})
	for _, tc := range []struct {
		name, groupWorkspace, groupEnvironment, appEnvironment, ref string
		want                                                        error
	}{
		{name: "malformed id", groupWorkspace: "tea-a", ref: "not-an-id", want: core.ErrBadRequest},
		{name: "unknown id", groupWorkspace: "tea-a", ref: "evg-d0000000000000000000", want: core.ErrBadRequest},
		{name: "another workspace's group", groupWorkspace: "tea-b", want: core.ErrBadRequest},
		{name: "environment mismatch", groupWorkspace: "tea-a", groupEnvironment: "env-a", appEnvironment: "env-b", want: core.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(newFakeStore())
			svc.Workspace = coretest.Members{"owner": {"tea-a", "tea-b"}}
			svc.EnvironmentWorkspace = func(context.Context, string) (string, error) { return tc.groupWorkspace, nil }
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", OwnerID: tc.groupWorkspace, EnvironmentID: tc.groupEnvironment})
			if err != nil {
				t.Fatal(err)
			}
			ref := group.ID
			if tc.ref != "" {
				ref = tc.ref
			}
			app := ownedApp("web", "tea-a")
			app.Labels[core.LabelEnvironment] = tc.appEnvironment
			actCtx := core.WithWorkspace(ctx, "tea-a")
			if err := svc.CheckInitialEnvGroupIDs(actCtx, []string{ref}, app); !errors.Is(err, tc.want) {
				t.Fatalf("plan check err = %v, want %v", err, tc.want)
			}
			called := false
			err = svc.WithInitialEnvGroupIDs(actCtx, []string{ref}, app, func() error { called = true; return nil }, nil)
			if !errors.Is(err, tc.want) || called {
				t.Fatalf("link: called=%v err=%v, want %v", called, err, tc.want)
			}
			if errors.Is(tc.want, core.ErrBadRequest) && !strings.Contains(err.Error(), `env group "`+ref+`" does not exist`) {
				t.Fatalf("refusal %q does not name the id as missing", err)
			}
			if links, err := linksOf(svc, group.ID); err != nil || len(links) != 0 {
				t.Fatalf("refused link left membership %v (%v)", links, err)
			}
		})
	}
}

// Linking at create materializes the group's values in the new service, so
// it is the same sensitive read a later link is: a write-only grant is
// refused, at the plan's check as at the link.
func TestInitialGroupIDsRefuseAWriteOnlyGrant(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	ctx = core.WithIdentity(ctx, core.Identity{Subject: "writer", Method: "oauth2", Human: true, CanonicalScopes: core.ScopeWrite})
	if err := svc.CheckInitialEnvGroupIDs(ctx, []string{group.ID}, sampleApp("web")); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("plan check err = %v, want forbidden", err)
	}
	called := false
	err = svc.WithInitialEnvGroupIDs(ctx, []string{group.ID}, sampleApp("web"), func() error { called = true; return nil }, nil)
	if !errors.Is(err, core.ErrForbidden) || called {
		t.Fatalf("write-only link: called=%v err=%v", called, err)
	}
}
