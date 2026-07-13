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
	"testing"
)

// These exercise the in-memory Store (always on, no BEX_TEST_DB_URI needed) —
// PGStore's equivalent behavior is covered against real Postgres by
// assertDeployLifecycle in store_pg_test.go.

// openDeployFor is the test-side equivalent of the old per-app OpenDeploy:
// ListOpenDeploys (the batched call the reconciler now uses once per pass,
// not once per app) filtered to one app, for tests that only care about one.
func openDeployFor(ctx context.Context, s Store, appID string) (Deploy, bool, error) {
	open, err := s.ListOpenDeploys(ctx)
	if err != nil {
		return Deploy{}, false, err
	}
	for _, d := range open {
		if d.AppID == appID {
			return d, true, nil
		}
	}
	return Deploy{}, false, nil
}

func TestCreateAppOpensFirstDeploy(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	deploys, err := s.ListDeploys(ctx, app.ID)
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	if len(deploys) != 1 || deploys[0].Trigger != "create" || deploys[0].Image != "img:1" || deploys[0].Status != DeployUpdateInProgress {
		t.Fatalf("deploy #1 = %+v", deploys)
	}

	open, ok, err := openDeployFor(ctx, s, app.ID)
	if err != nil || !ok || open.ID != deploys[0].ID {
		t.Fatalf("open deploy = %+v ok=%v (err %v)", open, ok, err)
	}
}

func TestCloseDeployIsIdempotentAndListIsNewestFirst(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	first, _, err := openDeployFor(ctx, s, app.ID)
	if err != nil {
		t.Fatalf("open first deploy: %v", err)
	}

	if won, err := s.CloseDeploy(ctx, first.ID, DeployLive, "img:resolved"); err != nil || !won {
		t.Fatalf("close: won=%v err=%v", won, err)
	}
	// Re-closing an already-terminal deploy must not flip its status, and must
	// report it lost the race (already terminal).
	if won, err := s.CloseDeploy(ctx, first.ID, DeployUpdateFailed, ""); err != nil || won {
		t.Fatalf("re-close: won=%v err=%v, want won=false", won, err)
	}
	got, err := s.GetDeploy(ctx, app.ID, first.ID)
	if err != nil || got.Status != DeployLive || got.FinishedAt == nil || got.ResolvedImage != "img:resolved" {
		t.Fatalf("closed deploy = %+v (err %v), want status live with finished_at + resolved_image set", got, err)
	}
	if _, ok, err := openDeployFor(ctx, s, app.ID); err != nil || ok {
		t.Fatalf("open deploy after close: ok=%v (err %v), want none open", ok, err)
	}

	second, err := s.CreateDeploy(ctx, app.ID, "api", "img:2", 2)
	if err != nil {
		t.Fatalf("trigger deploy: %v", err)
	}
	deploys, err := s.ListDeploys(ctx, app.ID)
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	if len(deploys) != 2 || deploys[0].ID != second.ID || deploys[1].ID != first.ID {
		t.Fatalf("list not newest-first: %+v", deploys)
	}
}

// TestCreateRollbackDeployRecordsProvenanceAndResolvedImage covers w2/m10's
// t001: unlike a normal trigger, a rollback deploy already knows the exact
// image it restores (it ran live before), so ResolvedImage is set immediately
// — Rollback never has to wait on the reconciler's write-back.
func TestCreateRollbackDeployRecordsProvenanceAndResolvedImage(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	first, _, _ := openDeployFor(ctx, s, app.ID)

	rb, err := s.CreateRollbackDeploy(ctx, app.ID, "img:1", first.ID)
	if err != nil {
		t.Fatalf("create rollback deploy: %v", err)
	}
	if rb.Trigger != "rollback" || rb.Image != "img:1" || rb.ResolvedImage != "img:1" || rb.RollbackOf != first.ID {
		t.Fatalf("rollback deploy = %+v", rb)
	}
	got, err := s.GetDeploy(ctx, app.ID, rb.ID)
	if err != nil || got.RollbackOf != first.ID {
		t.Fatalf("get rollback deploy = %+v (err %v)", got, err)
	}
}

func TestGetDeployIsScopedToItsApp(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	a1, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	a2, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "api", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	d1, _, err := openDeployFor(ctx, s, a1.ID)
	if err != nil {
		t.Fatalf("open deploy for a1: %v", err)
	}

	// A deploy id that belongs to a1 must not resolve when scoped to a2 — no
	// cross-app leak through the id alone.
	if _, err := s.GetDeploy(ctx, a2.ID, d1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get a1's deploy scoped to a2: want ErrNotFound, got %v", err)
	}
	if got, err := s.GetDeploy(ctx, a1.ID, d1.ID); err != nil || got.ID != d1.ID {
		t.Errorf("get a1's deploy scoped to a1: got %+v (err %v)", got, err)
	}
	if _, err := s.GetDeploy(ctx, a1.ID, "dep-doesnotexist00000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get unknown deploy: want ErrNotFound, got %v", err)
	}
}
