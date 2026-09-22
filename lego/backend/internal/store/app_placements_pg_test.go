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

package store

import (
	"context"
	"errors"
	"maps"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestAppPlacementsEmptyNeedsNoDatabase(t *testing.T) {
	for _, input := range [][]string{nil, {}} {
		placements, err := (&PGStore{}).GetAppPlacements(context.Background(), input)
		if err != nil || placements == nil || len(placements) != 0 {
			t.Fatalf("empty read = %#v, %v", placements, err)
		}
	}
}

func TestPGAppPlacementsFollowCommittedMembership(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	createApp := func() App {
		t.Helper()
		tenant, err := st.CreateWorkspace(ctx, "placement-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
		app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "nginx:1", Type: "web_service", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatal(err)
		}
		return app
	}
	app, other := createApp(), createApp()
	missing := ids.New(ids.Service)
	assertPlacement := func(want AppPlacement) {
		t.Helper()
		got, err := st.GetAppPlacements(ctx, []string{app.ID, missing, app.ID})
		if err != nil || !maps.Equal(got, map[string]AppPlacement{app.ID: want}) {
			t.Fatalf("bounded placements = %#v, %v; want %#v", got, err, want)
		}
	}
	assertPlacement(AppPlacement{TenantID: app.TenantID})
	both, err := st.GetAppPlacements(ctx, []string{app.ID, other.ID})
	if err != nil || !maps.Equal(both, map[string]AppPlacement{app.ID: {TenantID: app.TenantID}, other.ID: {TenantID: other.TenantID}}) {
		t.Fatalf("ownership snapshot = %#v, %v", both, err)
	}
	project, err := st.CreateProject(ctx, app.TenantID, "placement")
	if err != nil {
		t.Fatal(err)
	}
	first, err := st.CreateEnvironment(ctx, project.ID, app.TenantID, "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.CreateEnvironment(ctx, project.ID, app.TenantID, "second")
	if err != nil {
		t.Fatal(err)
	}
	assign := func(environment Environment) {
		t.Helper()
		if _, err := st.SetEnvironmentServices(ctx, environment.ID, project.ID, app.TenantID, []string{app.ID}); err != nil {
			t.Fatal(err)
		}
		assertPlacement(AppPlacement{TenantID: app.TenantID, ProjectID: project.ID, EnvironmentID: environment.ID})
	}
	assign(first)
	assign(second)
	if _, err := st.SetEnvironmentServices(ctx, second.ID, project.ID, app.TenantID, nil); err != nil {
		t.Fatal(err)
	}
	assertPlacement(AppPlacement{TenantID: app.TenantID, ProjectID: project.ID})
	assign(first)
	if err := st.DeleteEnvironment(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	assertPlacement(AppPlacement{TenantID: app.TenantID, ProjectID: project.ID})
	assign(second)
	if err := st.DeleteProject(ctx, project.ID); err != nil {
		t.Fatal(err)
	}
	assertPlacement(AppPlacement{TenantID: app.TenantID})
	if err := st.DeleteApp(ctx, app.ID); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetAppPlacements(ctx, []string{app.ID, missing})
	if err != nil || len(got) != 0 {
		t.Fatalf("deleted placements = %#v, %v", got, err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := st.GetAppPlacements(canceled, []string{other.ID}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("failed query = %#v, %v", got, err)
	}
}
