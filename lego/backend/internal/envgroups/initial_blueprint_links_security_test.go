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
	"fmt"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// linksOf is group gid's stored link set, or core.ErrNotFound.
func linksOf(svc *Service, gid string) ([]string, error) {
	row, ok := svc.fakeGroups().Row(gid)
	if !ok {
		return nil, core.ErrNotFound
	}
	return row.ServiceLinks, nil
}

func setGroupLinks(t *testing.T, svc *Service, gid string, links ...string) {
	t.Helper()
	if _, err := svc.mutateGroup(testCtx(), gid, func(cur *store.EnvGroup) error { cur.ServiceLinks = links; return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestInitialBlueprintGroupsRefuseUnauthorizedComposition(t *testing.T) {
	for _, tc := range []struct {
		name     string
		identity core.Identity
		revoked  bool
	}{
		{name: "write-only", identity: core.Identity{Subject: "writer", Method: "oauth2", Human: true, CanonicalScopes: core.ScopeWrite}},
		{name: "fresh revocation", identity: core.Identity{Subject: "writer", Method: "session"}, revoked: true},
	} {
		for _, alreadyLinked := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/alreadyLinked=%t", tc.name, alreadyLinked), func(t *testing.T) {
				ctx := testCtx()
				svc := newService(newFakeStore())
				group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
				if err != nil {
					t.Fatal(err)
				}
				if alreadyLinked {
					setGroupLinks(t, svc, group.ID, "web")
				}
				if tc.revoked {
					svc.Authz = sensitiveRevokedChecker{}
				}
				ctx = core.WithIdentity(ctx, tc.identity)
				called := false
				err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, sampleApp("web"), func() error { called = true; return nil }, nil)
				if !errors.Is(err, core.ErrForbidden) || called {
					t.Fatalf("denied composition: called=%v err=%v", called, err)
				}
				links, readErr := linksOf(svc, group.ID)
				if readErr != nil {
					t.Fatal(readErr)
				}
				want := []string{}
				if alreadyLinked {
					want = []string{"web"}
				}
				if !slices.Equal(links, want) {
					t.Fatalf("denial changed membership: %v", links)
				}
			})
		}
	}
}

func TestInitialBlueprintGroupsKeepWorkspaceAndEnvironmentBoundaries(t *testing.T) {
	ctx := core.WithIdentity(testCtx(), core.Identity{Subject: "owner", Method: "session"})
	for _, tc := range []struct {
		name, groupWorkspace, appWorkspace, groupEnvironment, appEnvironment string
		want                                                                 error
	}{
		{"foreign group name", "tea-b", "tea-a", "", "", core.ErrBadRequest},
		{"foreign app", "tea-a", "tea-b", "", "", core.ErrForbidden},
		{"environment mismatch", "tea-a", "tea-a", "env-a", "env-b", core.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(newFakeStore())
			svc.Workspace = coretest.Members{"owner": {"tea-a", "tea-b"}}
			svc.EnvironmentWorkspace = func(context.Context, string) (string, error) { return tc.groupWorkspace, nil }
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", OwnerID: tc.groupWorkspace, EnvironmentID: tc.groupEnvironment})
			if err != nil {
				t.Fatal(err)
			}
			app := ownedApp("web", tc.appWorkspace)
			app.Labels[core.LabelEnvironment] = tc.appEnvironment
			called := false
			err = svc.WithInitialEnvGroups(core.WithWorkspace(ctx, "tea-a"), []string{"shared"}, app, func() error { called = true; return nil }, nil)
			if !errors.Is(err, tc.want) || called {
				t.Fatalf("cross-boundary composition: called=%v err=%v want=%v", called, err, tc.want)
			}
			links, err := linksOf(svc, group.ID)
			if err != nil || len(links) != 0 {
				t.Fatalf("denial membership=%v err=%v", links, err)
			}
		})
	}
}

func TestInitialBlueprintGroupsUnavailableStoresDoNotCreate(t *testing.T) {
	for _, absent := range []string{"openbao", "groups"} {
		t.Run(absent, func(t *testing.T) {
			svc := newService(newFakeStore())
			ctx := testCtx()
			if _, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"}); err != nil {
				t.Fatal(err)
			}
			if absent == "openbao" {
				svc.Store = nil
			} else {
				svc.Groups = nil
			}
			called := false
			err := svc.WithInitialEnvGroups(ctx, []string{"shared"}, sampleApp("web"), func() error { called = true; return nil }, nil)
			if !errors.Is(err, core.ErrSecretsUnavailable) || called {
				t.Fatalf("unavailable composition: called=%v err=%v", called, err)
			}
		})
	}
}

func TestInitialBlueprintGroupsCompensateCreateAndCompletionFailures(t *testing.T) {
	for _, stage := range []string{"create", "complete"} {
		t.Run(stage, func(t *testing.T) {
			svc := newService(newFakeStore())
			ctx := testCtx()
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("initial write rejected")
			err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, sampleApp("web"), func() error {
				if stage == "create" {
					return failure
				}
				return nil
			}, func() error {
				links, err := linksOf(svc, group.ID)
				if err != nil || len(links) != 1 {
					t.Fatalf("completion before membership: %v %v", links, err)
				}
				return failure
			})
			if !errors.Is(err, failure) {
				t.Fatalf("failure lost: %v", err)
			}
			links, err := linksOf(svc, group.ID)
			if err != nil || len(links) != 0 {
				t.Fatalf("failed create retained links=%v err=%v", links, err)
			}
		})
	}
}

func TestInitialBlueprintGroupsRevalidateAfterCreation(t *testing.T) {
	for _, change := range []string{"delete", "environment", "revoke"} {
		t.Run(change, func(t *testing.T) {
			ctx := core.WithIdentity(testCtx(), core.Identity{Subject: "owner", Method: "session"})
			svc := newService(newFakeStore())
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			want := core.ErrNotFound
			err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, sampleApp("web"), func() error {
				switch change {
				case "delete":
					_, err = svc.Groups.DeleteEnvGroup(ctx, group.ID)
				case "environment":
					want = core.ErrConflict
					_, err = svc.mutateGroup(ctx, group.ID, func(cur *store.EnvGroup) error { cur.EnvironmentID = "env-other"; return nil })
				case "revoke":
					want = core.ErrForbidden
					svc.Authz = sensitiveRevokedChecker{}
				}
				return err
			}, nil)
			if !errors.Is(err, want) {
				t.Fatalf("changed group accepted: err=%v want=%v", err, want)
			}
			links, readErr := linksOf(svc, group.ID)
			if change == "delete" {
				if !errors.Is(readErr, core.ErrNotFound) {
					t.Fatalf("deleted group resurrected: %+v %v", links, readErr)
				}
			} else if readErr != nil || len(links) != 0 {
				t.Fatalf("refused finalization retained links=%v err=%v", links, readErr)
			}
		})
	}
}

// One membership write fails after a sibling group's was committed, and a
// concurrent unrelated change lands on the sibling meanwhile. The rollback
// removes only this operation's link from the sibling.
func TestInitialBlueprintGroupsRollbackPreservesConcurrentMembership(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	first, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "second"})
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("membership write failed")
	groups := svc.fakeGroups()
	failed := false
	groups.UpdateErr = func(next store.EnvGroup) error {
		if next.ID != second.ID || failed {
			return nil
		}
		failed = true
		groups.Commit(first.ID, func(g *store.EnvGroup) {
			g.ServiceLinks = addString(g.ServiceLinks, "other-service")
			g.Name = "renamed"
		})
		return failure
	}
	called := false
	err = svc.WithInitialEnvGroups(ctx, []string{"first", "second"}, sampleApp("web"), func() error { called = true; return nil }, nil)
	if !errors.Is(err, failure) || called {
		t.Fatalf("partial membership failure: called=%v err=%v", called, err)
	}
	row, _ := groups.Row(first.ID)
	if row.Name != "renamed" || !slices.Equal(row.ServiceLinks, []string{"other-service"}) {
		t.Fatalf("rollback clobbered concurrent metadata: %+v", row)
	}
	if links, err := linksOf(svc, second.ID); err != nil || len(links) != 0 {
		t.Fatalf("failed group linked: %+v %v", links, err)
	}
}

func TestInitialBlueprintGroupsRollbackSurvivesCallerCancellation(t *testing.T) {
	svc := newService(newFakeStore())
	group, err := svc.CreateEnvGroup(testCtx(), CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(testCtx())
	defer cancel()
	err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, sampleApp("web"), func() error { cancel(); return ctx.Err() }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if links, err := linksOf(svc, group.ID); err != nil || len(links) != 0 {
		t.Fatalf("canceled create retained membership: %+v %v", links, err)
	}
}

func TestInitialBlueprintGroupsFailureRetainsPreexistingMembership(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	setGroupLinks(t, svc, group.ID, "web")
	failure := errors.New("create refused")
	err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, sampleApp("web"), func() error { return failure }, nil)
	if !errors.Is(err, failure) {
		t.Fatalf("failure lost: %v", err)
	}
	if links, err := linksOf(svc, group.ID); err != nil || !slices.Equal(links, []string{"web"}) {
		t.Fatalf("rollback removed preexisting link: %+v %v", links, err)
	}
}

func TestInitialBlueprintGroupsConflictCompensationRespectsCreationBoundary(t *testing.T) {
	for _, stage := range []string{"reservation", "finalization"} {
		t.Run(stage, func(t *testing.T) {
			ctx := testCtx()
			svc := newService(newFakeStore())
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			groups := svc.fakeGroups()
			injected := false
			// Commit a competing membership after this attempt's read, forcing
			// its write to conflict; the retry sees a membership owned by the
			// competing operation.
			injectCompetingLink := func(id string) {
				if injected || id != group.ID {
					return
				}
				injected = true
				groups.Commit(id, func(g *store.EnvGroup) {
					g.ServiceLinks = addString(g.ServiceLinks, "web")
					if stage == "finalization" {
						g.ServiceLinks = addString(g.ServiceLinks, "other-service")
					}
				})
			}
			if stage == "reservation" {
				groups.AfterGet = injectCompetingLink
			}
			failure := errors.New("creation cannot complete")
			err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, sampleApp("web"), func() error {
				if stage == "reservation" {
					return failure
				}
				// A group update can prune the reservation while the App is absent.
				if _, err := svc.mutateGroup(ctx, group.ID, func(cur *store.EnvGroup) error { cur.ServiceLinks = nil; return nil }); err != nil {
					return err
				}
				groups.AfterGet = injectCompetingLink
				return nil
			}, func() error { return failure })
			if !errors.Is(err, failure) || !injected {
				t.Fatalf("missing failed interleaving: injected=%v err=%v", injected, err)
			}
			links, err := linksOf(svc, group.ID)
			want := []string{"web"}
			if stage == "finalization" {
				// The caller removes the App this operation created on failure. Retaining
				// a link to that doomed App would orphan it; unrelated links survive.
				want = []string{"other-service"}
			}
			if err != nil || !slices.Equal(links, want) {
				t.Fatalf("rollback memberships=%v want=%v err=%v", links, want, err)
			}
		})
	}
}
