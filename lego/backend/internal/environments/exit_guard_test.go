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

package environments

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m232: the round-22 fix gated membership changes on the environment being
// edited, but a write on ANOTHER environment that pulls a member out of an
// ACL-bearing one stripped its protection with no check on that source. A move
// out of an ACL-bearing environment needs its fresh can_manage on every path.

type exitFixture struct {
	st                *fakeStore
	admin             *Service
	prod, staging, qa store.Environment
}

// newExitFixture builds prj-1 with a protected, isolated prod and two bare
// environments, staging and qa, with "web" placed in prod by an admin.
func newExitFixture(t *testing.T) exitFixture {
	t.Helper()
	ctx := ctxAs("user-a")
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	admin, _ := newServiceWithClient(st, sampleApp("web"))
	prodView, err := admin.CreateWithACL(ctx, CreateEnvironmentRequest{
		ProjectID: "prj-1", Name: "prod", ProtectedStatus: ProtectedStatusProtected, NetworkIsolationEnabled: true,
	})
	if err != nil {
		t.Fatalf("admin CreateWithACL: %v", err)
	}
	env := func(name string) store.Environment {
		v, err := admin.Create(ctx, "prj-1", name)
		if err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
		e, _ := st.GetEnvironment(ctx, v.ID)
		return e
	}
	prod, _ := st.GetEnvironment(ctx, prodView.ID)
	f := exitFixture{st: st, admin: admin, prod: prod, staging: env("staging"), qa: env("qa")}
	if _, err := admin.SetServices(ctx, prod.ID, []string{"web"}); err != nil {
		t.Fatalf("admin SetServices prod: %v", err)
	}
	return f
}

func (f exitFixture) developer() *Service {
	return &Service{Base: &core.Base{Authz: developerChecker{}, Client: f.admin.Client, Namespace: "default"}, Store: f.st}
}

func (f exitFixture) servicesIn(t *testing.T, e store.Environment) []string {
	t.Helper()
	members, err := f.st.ListEnvironmentServices(ctxAs("user-a"), e.ID, e.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(members)
	return members
}

func TestPullingAServiceOutOfAnACLBearingEnvironmentNeedsItsAdmin(t *testing.T) {
	ctx := ctxAs("user-a")
	f := newExitFixture(t)

	if _, err := f.developer().SetServices(ctx, f.staging.ID, []string{"web"}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("developer SetServices(staging, web from prod) = %v, want ErrForbidden", err)
	}
	if got := f.servicesIn(t, f.prod); !slices.Equal(got, []string{"web"}) || len(f.servicesIn(t, f.staging)) != 0 {
		t.Fatalf("a refused move changed membership: prod=%v staging=%v", got, f.servicesIn(t, f.staging))
	}
	if app := getApp(t, f.admin.Client, "web"); app.Labels[core.LabelNetworkIsolation] != f.prod.ID {
		t.Fatalf("a refused move stripped the isolation label: %v", app.Labels)
	}

	// The admin may move it; once in a bare environment a developer moves it
	// between bare environments as before.
	if _, err := f.admin.SetServices(ctx, f.staging.ID, []string{"web"}); err != nil {
		t.Fatalf("admin SetServices(staging): %v", err)
	}
	if _, err := f.developer().SetServices(ctx, f.qa.ID, []string{"web"}); err != nil {
		t.Fatalf("developer move between bare environments: %v", err)
	}
	if got := f.servicesIn(t, f.qa); !slices.Equal(got, []string{"web"}) {
		t.Fatalf("qa = %v, want [web]", got)
	}
}

func TestPullingADatastoreOrGroupOutOfAnACLBearingEnvironmentNeedsItsAdmin(t *testing.T) {
	ctx := ctxAs("user-a")
	for _, kind := range []string{"database", "key value", "env group"} {
		t.Run(kind, func(t *testing.T) {
			f := newExitFixture(t)
			dbs, kvs := newDatabaseIndex(), newKeyValueIndex()
			groups := &fakeEnvGroupIndex{}
			var set func(*Service, string) error
			var placement func() string
			switch kind {
			case "database":
				dbs.add(postgres.PostgresView{ID: "dpg-one", OwnerID: "tea-a", ProjectID: "prj-1", EnvironmentID: f.prod.ID})
				set = func(s *Service, env string) error {
					_, err := s.SetDatabases(ctx, env, []string{"dpg-one"})
					return err
				}
				placement = func() string { return dbs.dbs["dpg-one"].EnvironmentID }
			case "key value":
				kvs.add(keyvalue.KeyValueView{ID: "red-one", OwnerID: "tea-a", ProjectID: "prj-1", EnvironmentID: f.prod.ID})
				set = func(s *Service, env string) error {
					_, err := s.SetKeyValues(ctx, env, []string{"red-one"})
					return err
				}
				placement = func() string { return kvs.kvs["red-one"].EnvironmentID }
			case "env group":
				groups.groups = []envgroups.EnvGroupView{{ID: "evg-one", OwnerID: "tea-a", EnvironmentID: f.prod.ID}}
				set = func(s *Service, env string) error {
					_, err := s.SetEnvGroups(ctx, env, []string{"evg-one"})
					return err
				}
				placement = func() string { return groups.groups[0].EnvironmentID }
			}
			dev := f.developer()
			for _, s := range []*Service{dev, f.admin} {
				s.Databases, s.KeyValues, s.EnvGroups = dbs, kvs, groups
			}

			if err := set(dev, f.staging.ID); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("developer pull from prod = %v, want ErrForbidden", err)
			}
			if got := id.EnvironmentStorageID(placement()); got != f.prod.ID {
				t.Fatalf("a refused move changed placement to %q", got)
			}
			if err := set(f.admin, f.staging.ID); err != nil {
				t.Fatalf("admin pull from prod: %v", err)
			}
			if err := set(dev, f.qa.ID); err != nil {
				t.Fatalf("developer move between bare environments: %v", err)
			}
			if got := id.EnvironmentStorageID(placement()); got != f.qa.ID {
				t.Fatalf("placement = %q, want qa", got)
			}
		})
	}
}

// staleDatabaseIndex lists a database where it was before a concurrent move,
// standing in for a move that lands between the listing and the write.
type staleDatabaseIndex struct {
	*fakeDatabaseIndex
	listedEnvironment string
}

func (s staleDatabaseIndex) ListPostgres(ctx context.Context, ownerID string) ([]postgres.PostgresView, error) {
	views, err := s.fakeDatabaseIndex.ListPostgres(ctx, ownerID)
	for i := range views {
		views[i].EnvironmentID = id.EnvironmentPublicID(s.listedEnvironment)
	}
	return views, err
}

// TestADatastoreMovedSinceItWasListedIsRefusedNotPulledOut: the check passes
// on the bare environment the listing showed, but the database is in prod by
// the time of the write; the write's compare refuses it.
func TestADatastoreMovedSinceItWasListedIsRefusedNotPulledOut(t *testing.T) {
	ctx := ctxAs("user-a")
	f := newExitFixture(t)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "dpg-one", OwnerID: "tea-a", ProjectID: "prj-1", EnvironmentID: f.prod.ID})
	dev := f.developer()
	dev.Databases = staleDatabaseIndex{fakeDatabaseIndex: dbs, listedEnvironment: f.qa.ID}

	_, err := dev.SetDatabases(ctx, f.staging.ID, []string{"dpg-one"})
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("move of a concurrently moved database = %v, want a conflict", err)
	}
	if got := dbs.dbs["dpg-one"].EnvironmentID; got != f.prod.ID {
		t.Fatalf("the database left prod: %q", got)
	}
}

// TestTheExitGuardFailsClosedOnAnEnvironmentItDidNotLoad: the guard is built
// before the write, so an environment created since is unknown to it and
// counts as ACL-bearing; bare environments it loaded stay open.
func TestTheExitGuardFailsClosedOnAnEnvironmentItDidNotLoad(t *testing.T) {
	ctx := ctxAs("user-a")
	f := newExitFixture(t)
	exit, err := f.developer().loadExitGuard(ctx, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := exit(f.staging.ID, id.EnvironmentPublicID(f.qa.ID), ""); err != nil {
		t.Fatalf("leaving bare environments = %v, want nil", err)
	}
	for _, left := range []string{f.prod.ID, "env-created-since"} {
		if err := exit(left); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("developer leaving %s = %v, want ErrForbidden", left, err)
		}
	}
	adminExit, err := f.admin.loadExitGuard(ctx, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := adminExit(f.prod.ID, "env-created-since"); err != nil {
		t.Fatalf("admin leaving prod = %v, want nil", err)
	}
}
