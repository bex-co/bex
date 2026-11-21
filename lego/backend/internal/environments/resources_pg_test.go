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
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// gatedStore lines a round of concurrent requests up the way their writes
// can meet: every request has read the environment before any of them
// writes, and the writes then commit one after another, each starting after
// the last one committed. Each request waits at its exit-guard load, the last
// read before its membership write, until the whole round has arrived.
type gatedStore struct {
	*store.PGStore
	mu      sync.Mutex
	waiting int
	release chan struct{}
	writing sync.Mutex
}

// hold makes the next n exit-guard loads wait for one another.
func (g *gatedStore) hold(n int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.waiting, g.release = n, make(chan struct{})
}

func (g *gatedStore) ListWorkspaceEnvironments(ctx context.Context, tenantID string) ([]store.Environment, error) {
	g.mu.Lock()
	release := g.release
	if release != nil {
		if g.waiting--; g.waiting == 0 {
			close(release)
			g.release = nil
		}
	}
	g.mu.Unlock()
	if release != nil {
		select {
		case <-release:
		case <-time.After(10 * time.Second): // a request that never arrives must not hang the suite
		}
	}
	return g.PGStore.ListWorkspaceEnvironments(ctx, tenantID)
}

func (g *gatedStore) SetEnvironmentServices(ctx context.Context, environmentID, projectID, tenantID string, serviceIDs []string, exit core.EnvironmentExitGuard) ([]core.ServicePlacementChange, error) {
	g.writing.Lock()
	defer g.writing.Unlock()
	return g.PGStore.SetEnvironmentServices(ctx, environmentID, projectID, tenantID, serviceIDs, exit)
}

func (g *gatedStore) ChangeEnvironmentServices(ctx context.Context, environmentID, projectID, tenantID string, serviceIDs []string, add bool, exit core.EnvironmentExitGuard) ([]core.ServicePlacementChange, error) {
	g.writing.Lock()
	defer g.writing.Unlock()
	return g.PGStore.ChangeEnvironmentServices(ctx, environmentID, projectID, tenantID, serviceIDs, add, exit)
}

// TestChangeResourcesConcurrentRequestsChangeOnlyTheirOwnMembers runs Render's
// add and remove resource requests side by side against the real store: each
// changes only the services it names, so every add lands, the one removal
// takes out only its own service, and every member keeps the environment's
// isolation label and inbound-IP layer.
// resourcesWorkspace opens the real store and creates a fresh workspace for
// one test, removed when it ends; it returns the store, the workspace id and
// a stamp that keeps the test's names unique.
func resourcesWorkspace(t *testing.T) (*store.PGStore, string, string) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	pg := store.NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	workspace, err := pg.CreateWorkspace(context.Background(), "env-resources-"+stamp, store.PlanHobby, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), workspace.ID) })
	return pg, workspace.ID, stamp
}

func TestChangeResourcesConcurrentRequestsChangeOnlyTheirOwnMembers(t *testing.T) {
	pg, workspaceID, stamp := resourcesWorkspace(t)
	ctx := context.Background()
	project, err := pg.CreateProject(ctx, workspaceID, "stack-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	env, err := pg.CreateEnvironment(ctx, project.ID, workspaceID, "production")
	if err != nil {
		t.Fatal(err)
	}
	office := []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8", Description: "office"}}
	if err := pg.SetEnvironmentACL(ctx, env.ID, ProtectedStatusUnprotected, true, office); err != nil {
		t.Fatal(err)
	}
	const n = 6
	services := make([]string, n)
	apps := make([]*appv1alpha1.App, n)
	for i := range services {
		app, err := pg.CreateApp(ctx, store.App{TenantID: workspaceID, Name: fmt.Sprintf("svc%d-%s", i, stamp), Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatal(err)
		}
		services[i], apps[i] = app.ID, sampleApp(app.ID)
	}
	gated := &gatedStore{PGStore: pg}
	svc, cl := newServiceWithClient(gated, apps...)
	envID := ids.EnvironmentPublicID(env.ID)
	if _, err := svc.ChangeResources(ctxAs("user-a"), envID, services[:2], true); err != nil {
		t.Fatalf("seed members: %v", err)
	}

	// Four adds, each of one new service, and the removal of services[1].
	type request struct {
		id  string
		add bool
	}
	round := []request{{services[1], false}}
	for _, sid := range services[2:] {
		round = append(round, request{sid, true})
	}
	gated.hold(len(round))
	errs := make([]error, len(round))
	var wg sync.WaitGroup
	for i, r := range round {
		wg.Go(func() {
			_, errs[i] = svc.ChangeResources(ctxAs("user-a"), envID, []string{r.id}, r.add)
		})
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %+v: %v", round[i], err)
		}
	}

	want := append([]string{services[0]}, services[2:]...)
	got, err := pg.ListEnvironmentServices(ctx, env.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("members after the concurrent requests = %v, want %v (every add, without the one removed)", got, want)
	}
	cidrs := core.EnvironmentLayerCIDRs(office)
	for _, sid := range want {
		app := getApp(t, cl, sid)
		if app.Labels[core.LabelNetworkIsolation] != env.ID || !slices.Equal(app.Spec.EnvironmentIPAllowList, cidrs) {
			t.Errorf("member %s carries isolation %q and IP layer %v, want %q and %v",
				sid, app.Labels[core.LabelNetworkIsolation], app.Spec.EnvironmentIPAllowList, env.ID, cidrs)
		}
	}
	removed := getApp(t, cl, services[1])
	if removed.Labels[core.LabelNetworkIsolation] != "" || len(removed.Spec.EnvironmentIPAllowList) != 0 {
		t.Errorf("removed %s still carries isolation %q and IP layer %v",
			services[1], removed.Labels[core.LabelNetworkIsolation], removed.Spec.EnvironmentIPAllowList)
	}
}

// A failed add returns a service it took from an environment in another
// project to that environment and project, with its isolation label and
// inbound-IP layer, and the store's return moves only a service still where
// the add put it.
func TestChangeResourcesFailureReturnsAServiceToItsEnvironmentPG(t *testing.T) {
	pg, workspaceID, stamp := resourcesWorkspace(t)
	ctx := context.Background()
	left, err := pg.CreateProject(ctx, workspaceID, "left-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	right, err := pg.CreateProject(ctx, workspaceID, "right-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	staging, err := pg.CreateEnvironment(ctx, left.ID, workspaceID, "staging")
	if err != nil {
		t.Fatal(err)
	}
	production, err := pg.CreateEnvironment(ctx, right.ID, workspaceID, "production")
	if err != nil {
		t.Fatal(err)
	}
	office := []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8", Description: "office"}}
	if err := pg.SetEnvironmentACL(ctx, staging.ID, ProtectedStatusUnprotected, true, office); err != nil {
		t.Fatal(err)
	}
	app, err := pg.CreateApp(ctx, store.App{TenantID: workspaceID, Name: "web-" + stamp, Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	group := ids.New(ids.EnvGroup)
	groups := &failingEnvGroupIndex{refuse: group}
	groups.groups = []envgroups.EnvGroupView{{ID: group, OwnerID: workspaceID}}
	svc, cl := newServiceWithClient(pg, sampleApp(app.ID))
	svc.EnvGroups = groups
	if _, err := svc.ChangeResources(ctxAs("user-a"), ids.EnvironmentPublicID(staging.ID), []string{app.ID}, true); err != nil {
		t.Fatal(err)
	}

	// The service moves to production, then the group's join fails.
	_, err = svc.ChangeResources(ctxAs("user-a"), ids.EnvironmentPublicID(production.ID), []string{app.ID, group}, true)
	var coded *core.CodedError
	if err == nil || errors.As(err, &coded) {
		t.Fatalf("failed add = %v, want the group's own failure", err)
	}
	if got, err := pg.ListEnvironmentServices(ctx, staging.ID, left.ID); err != nil || !slices.Equal(got, []string{app.ID}) {
		t.Fatalf("staging after the reversal = %v, %v; want %s back in it and in its project", got, err, app.ID)
	}
	if got, err := pg.ListEnvironmentServices(ctx, production.ID, right.ID); err != nil || len(got) != 0 {
		t.Fatalf("production after the reversal = %v, %v; want it empty", got, err)
	}
	cidrs := core.EnvironmentLayerCIDRs(office)
	if cr := getApp(t, cl, app.ID); cr.Labels[core.LabelNetworkIsolation] != staging.ID || !slices.Equal(cr.Spec.EnvironmentIPAllowList, cidrs) {
		t.Errorf("returned %s carries isolation %q and IP layer %v, want staging's %q and %v",
			app.ID, cr.Labels[core.LabelNetworkIsolation], cr.Spec.EnvironmentIPAllowList, staging.ID, cidrs)
	}

	// A return naming a service that is no longer in production moves nothing.
	moves, err := pg.ReturnEnvironmentServices(ctx, production.ID, staging.ID, left.ID, workspaceID, []string{app.ID}, nil)
	if err != nil || len(moves) != 0 {
		t.Fatalf("return of a service not in production = %+v, %v; want no change", moves, err)
	}
}
