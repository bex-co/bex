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
)

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
				ctx := context.Background()
				svc := newService(newFakeStore())
				group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
				if err != nil {
					t.Fatal(err)
				}
				if alreadyLinked {
					_, err = svc.mutateMetaCAS(ctx, group.ID, "", func(m meta) (meta, error) { m.links = []string{"web"}; return m, nil })
					if err != nil {
						t.Fatal(err)
					}
				}
				if tc.revoked {
					svc.Authz = sensitiveRevokedChecker{}
				}
				ctx = core.WithIdentity(ctx, tc.identity)
				called := false
				err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, "web", sampleApp("web"), func() error { called = true; return nil }, nil)
				if !errors.Is(err, core.ErrForbidden) || called {
					t.Fatalf("denied composition: called=%v err=%v", called, err)
				}
				m, readErr := svc.requireGroup(context.Background(), group.ID)
				if readErr != nil {
					t.Fatal(readErr)
				}
				want := []string(nil)
				if alreadyLinked {
					want = []string{"web"}
				}
				if !slices.Equal(m.links, want) {
					t.Fatalf("denial changed membership: %v", m.links)
				}
			})
		}
	}
}

func TestInitialBlueprintGroupsKeepWorkspaceAndEnvironmentBoundaries(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "owner", Method: "session"})
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
			svc.Workspace = multiWorkspace{"owner": {"tea-a", "tea-b"}}
			svc.EnvironmentWorkspace = func(context.Context, string) (string, error) { return tc.groupWorkspace, nil }
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", OwnerID: tc.groupWorkspace, EnvironmentID: tc.groupEnvironment})
			if err != nil {
				t.Fatal(err)
			}
			app := ownedApp("web", tc.appWorkspace)
			app.Labels[core.LabelEnvironment] = tc.appEnvironment
			called := false
			err = svc.WithInitialEnvGroups(core.WithWorkspace(ctx, "tea-a"), []string{"shared"}, "web", app, func() error { called = true; return nil }, nil)
			if !errors.Is(err, tc.want) || called {
				t.Fatalf("cross-boundary composition: called=%v err=%v want=%v", called, err, tc.want)
			}
			m, err := svc.requireGroup(ctx, group.ID)
			if err != nil || len(m.links) != 0 {
				t.Fatalf("denial membership=%v err=%v", m.links, err)
			}
		})
	}
}

func TestInitialBlueprintGroupsUnavailableStoresDoNotCreate(t *testing.T) {
	for _, absent := range []bool{false, true} {
		t.Run(fmt.Sprintf("absent=%t", absent), func(t *testing.T) {
			svc := newService(newFakeStore())
			ctx := context.Background()
			if _, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"}); err != nil {
				t.Fatal(err)
			}
			if absent {
				svc.Store = nil
			} else {
				svc.Store = struct{ core.SecretKV }{svc.Store}
			}
			called := false
			err := svc.WithInitialEnvGroups(ctx, []string{"shared"}, "web", sampleApp("web"), func() error { called = true; return nil }, nil)
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
			ctx := context.Background()
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("initial write rejected")
			err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, "web", sampleApp("web"), func() error {
				if stage == "create" {
					return failure
				}
				return nil
			}, func() error {
				m, err := svc.requireGroup(ctx, group.ID)
				if err != nil || len(m.links) != 1 {
					t.Fatalf("completion before membership: %v %v", m.links, err)
				}
				return failure
			})
			if !errors.Is(err, failure) {
				t.Fatalf("failure lost: %v", err)
			}
			m, err := svc.requireGroup(ctx, group.ID)
			if err != nil || len(m.links) != 0 {
				t.Fatalf("failed create retained links=%v err=%v", m.links, err)
			}
		})
	}
}

func TestInitialBlueprintGroupsRevalidateAfterCreation(t *testing.T) {
	for _, change := range []string{"delete", "environment", "revoke"} {
		t.Run(change, func(t *testing.T) {
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "owner", Method: "session"})
			svc := newService(newFakeStore())
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			want := core.ErrNotFound
			err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, "web", sampleApp("web"), func() error {
				switch change {
				case "delete":
					_, err = svc.clearMetaCAS(ctx, group.ID, "")
				case "environment":
					want = core.ErrConflict
					_, err = svc.mutateMetaCAS(ctx, group.ID, "", func(m meta) (meta, error) { m.environment = "env-other"; return m, nil })
				case "revoke":
					want = core.ErrForbidden
					svc.Authz = sensitiveRevokedChecker{}
				}
				return err
			}, nil)
			if !errors.Is(err, want) {
				t.Fatalf("changed group accepted: err=%v want=%v", err, want)
			}
			m, readErr := svc.requireGroup(context.Background(), group.ID)
			if change == "delete" {
				if !errors.Is(readErr, core.ErrNotFound) {
					t.Fatalf("deleted group resurrected: %+v %v", m, readErr)
				}
			} else if readErr != nil || len(m.links) != 0 {
				t.Fatalf("refused finalization retained links=%v err=%v", m.links, readErr)
			}
		})
	}
}

// Fail one membership write after a sibling group was committed, allowing the
// compensation writes to proceed. A concurrent unrelated link must survive.
type initialMembershipFailureStore struct {
	*fakeStore
	failPath      string
	failure       error
	beforeFailure func()
	failLocator   bool
}

func (s *initialMembershipFailureStore) PutCAS(ctx context.Context, path string, data map[string]string, version uint64) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if path == s.failPath {
		s.failPath = ""
		if s.beforeFailure != nil {
			s.beforeFailure()
		}
		return 0, s.failure
	}
	return s.fakeStore.PutCAS(ctx, path, data, version)
}
func (s *initialMembershipFailureStore) Put(ctx context.Context, path string, data map[string]string) error {
	if s.failLocator && data["locator"] == "1" {
		s.failLocator = false
		return s.failure
	}
	return s.fakeStore.Put(ctx, path, data)
}

func TestInitialBlueprintGroupsRollbackPreservesConcurrentMembership(t *testing.T) {
	ctx := context.Background()
	st := &initialMembershipFailureStore{fakeStore: newFakeStore(), failure: errors.New("membership write failed")}
	svc := newService(st)
	first, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "second"})
	if err != nil {
		t.Fatal(err)
	}
	st.failPath = metaPath(second.ID)
	st.beforeFailure = func() {
		_, err := svc.mutateMetaCAS(ctx, first.ID, "", func(m meta) (meta, error) {
			m.links = addString(m.links, "other-service")
			m.name = "renamed"
			return m, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	called := false
	err = svc.WithInitialEnvGroups(ctx, []string{"first", "second"}, "web", sampleApp("web"), func() error { called = true; return nil }, nil)
	if !errors.Is(err, st.failure) || called {
		t.Fatalf("partial membership failure: called=%v err=%v", called, err)
	}
	m, err := svc.requireGroup(ctx, first.ID)
	if err != nil || m.name != "renamed" || !slices.Equal(m.links, []string{"other-service"}) {
		t.Fatalf("rollback clobbered concurrent metadata: %+v err=%v", m, err)
	}
	m, err = svc.requireGroup(ctx, second.ID)
	if err != nil || len(m.links) != 0 {
		t.Fatalf("failed group linked: %+v %v", m, err)
	}
}

func TestInitialBlueprintGroupsRollbackPostCommitLocatorFailure(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "owner", Method: "session"})
	st := &initialMembershipFailureStore{fakeStore: newFakeStore(), failure: errors.New("locator write failed")}
	svc := newService(st)
	svc.Workspace = multiWorkspace{"owner": {"tea-a"}}
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	st.failLocator = true
	called := false
	err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, "web", ownedApp("web", "tea-a"), func() error { called = true; return nil }, nil)
	if !errors.Is(err, st.failure) || called {
		t.Fatalf("post-commit failure: called=%v err=%v", called, err)
	}
	m, err := svc.requireGroup(ctx, group.ID)
	if err != nil || len(m.links) != 0 {
		t.Fatalf("post-commit failure retained membership: %+v err=%v", m, err)
	}
}

func TestInitialBlueprintGroupsRollbackSurvivesCallerCancellation(t *testing.T) {
	st := &initialMembershipFailureStore{fakeStore: newFakeStore()}
	svc := newService(st)
	group, err := svc.CreateEnvGroup(context.Background(), CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, "web", sampleApp("web"), func() error { cancel(); return ctx.Err() }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	m, err := svc.requireGroup(context.Background(), group.ID)
	if err != nil || len(m.links) != 0 {
		t.Fatalf("canceled create retained membership: %+v %v", m, err)
	}
}

func TestInitialBlueprintGroupsFailureRetainsPreexistingMembership(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore())
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.mutateMetaCAS(ctx, group.ID, "", func(m meta) (meta, error) { m.links = []string{"web"}; return m, nil })
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("create refused")
	err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, "web", sampleApp("web"), func() error { return failure }, nil)
	if !errors.Is(err, failure) {
		t.Fatalf("failure lost: %v", err)
	}
	m, err := svc.requireGroup(ctx, group.ID)
	if err != nil || !slices.Equal(m.links, []string{"web"}) {
		t.Fatalf("rollback removed preexisting link: %+v %v", m, err)
	}
}

func TestInitialBlueprintGroupsConflictCompensationRespectsCreationBoundary(t *testing.T) {
	for _, stage := range []string{"reservation", "finalization"} {
		t.Run(stage, func(t *testing.T) {
			ctx := context.Background()
			st := newFakeStore()
			svc := newService(st)
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			injected := false
			injectCompetingLink := func(path string, snapshot core.SecretKVSnapshot) {
				if injected || path != metaPath(group.ID) || !isEditableMeta(snapshot.Data) {
					return
				}
				injected = true
				competing := decodeMeta(snapshot.Data)
				competing.links = addString(competing.links, "web")
				if stage == "finalization" {
					competing.links = addString(competing.links, "other-service")
				}
				// Commit after this attempt's read, forcing its PutCAS to conflict. Its
				// retry sees a membership owned by the competing operation.
				if err := st.Put(ctx, path, encodeMeta(competing)); err != nil {
					t.Fatal(err)
				}
			}
			if stage == "reservation" {
				st.afterGetVersioned = injectCompetingLink
			}
			failure := errors.New("creation cannot complete")
			err = svc.WithInitialEnvGroups(ctx, []string{"shared"}, "web", sampleApp("web"), func() error {
				if stage == "reservation" {
					return failure
				}
				// A group update can prune the reservation while the App is absent.
				if _, err := svc.mutateMetaCAS(ctx, group.ID, "", func(m meta) (meta, error) { m.links = nil; return m, nil }); err != nil {
					return err
				}
				st.afterGetVersioned = injectCompetingLink
				return nil
			}, func() error { return failure })
			if !errors.Is(err, failure) || !injected {
				t.Fatalf("missing failed interleaving: injected=%v err=%v", injected, err)
			}
			m, err := svc.requireGroup(ctx, group.ID)
			want := []string{"web"}
			if stage == "finalization" {
				// The caller removes the App this operation created on failure. Retaining
				// a link to that doomed App would orphan it; unrelated links survive.
				want = []string{"other-service"}
			}
			if err != nil || !slices.Equal(m.links, want) {
				t.Fatalf("rollback memberships=%v want=%v err=%v", m.links, want, err)
			}
		})
	}
}
