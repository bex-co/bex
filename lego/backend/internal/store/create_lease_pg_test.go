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
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// smallPoolStore is a PGStore on a 2-connection pool: fewer connections than
// the concurrent creates below, so a lease that held a connection while its
// holder opened another would deadlock.
func smallPoolStore(t *testing.T) *PGStore {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(uri)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return NewPGStore(pool)
}

// leaseTenant creates a workspace for a lease test and deletes it afterwards.
func leaseTenant(t *testing.T, s *PGStore) string {
	t.Helper()
	tenant, err := s.CreateTenant(context.Background(), "lease-"+ids.New(ids.Service), PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), tenant.ID) })
	return tenant.ID
}

// w9/m180 t005: the workspace create lease admits one holder per workspace,
// holds no connection while its holder queries, and lets other workspaces
// through.
func TestPGWorkspaceCreateLeaseSerializesWithoutPinningConnections(t *testing.T) {
	s := smallPoolStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tenants := []string{leaseTenant(t, s), leaseTenant(t, s)}
	var inside [2]atomic.Int32
	var overlap atomic.Bool
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ti := i % 2
			errs <- s.WithWorkspaceCreateLease(ctx, tenants[ti], func() error {
				if inside[ti].Add(1) > 1 {
					overlap.Store(true)
				}
				defer inside[ti].Add(-1)
				// The holder works through the pool, as a create does.
				var one int
				if err := s.Pool.QueryRow(ctx, `SELECT 1`).Scan(&one); err != nil {
					return err
				}
				time.Sleep(20 * time.Millisecond)
				return nil
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("lease: %v", err)
		}
	}
	if overlap.Load() {
		t.Fatal("two holders of one workspace's lease ran at once")
	}
	var left int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM workspace_create_leases WHERE tenant_id = ANY($1)`, tenants).Scan(&left); err != nil || left != 0 {
		t.Fatalf("leases left behind = %d (%v), want 0", left, err)
	}
}

// A waiter gives up with a retryable 503 while another holder works, and an
// expired lease (a crashed holder) is taken over.
func TestPGWorkspaceCreateLeaseTimesOutAndTakesOverAnExpiredOne(t *testing.T) {
	s := smallPoolStore(t)
	ctx := context.Background()
	tenant := leaseTenant(t, s)
	release := make(chan struct{})
	held := make(chan struct{})
	holderDone := make(chan struct{})
	go func() {
		defer close(holderDone)
		_ = s.WithWorkspaceCreateLease(ctx, tenant, func() error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	err := s.withWorkspaceCreateLease(ctx, tenant, 200*time.Millisecond, time.Minute, func() error { return nil })
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != WorkspaceCreateBusyCode || !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("waiting on a held lease: err = %v, want %s", err, WorkspaceCreateBusyCode)
	}
	close(release)
	<-holderDone

	if _, err := s.Pool.Exec(ctx, `INSERT INTO workspace_create_leases (tenant_id, holder, expires_at)
		VALUES ($1, 'crashed', now() - interval '1 second')
		ON CONFLICT (tenant_id) DO UPDATE SET holder = 'crashed', expires_at = now() - interval '1 second'`, tenant); err != nil {
		t.Fatal(err)
	}
	ran := false
	if err := s.withWorkspaceCreateLease(ctx, tenant, 2*time.Second, time.Minute, func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("taking over an expired lease: ran=%v err=%v", ran, err)
	}
}

// w9/m180: the fan-out's build write changes a linked row only when it is not
// already on the image, and reports a re-linked row as not found.
func TestPGSelectAppBuildSourceImageWritesOnlyANewBuild(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "select-build")
	ctx := context.Background()
	carrierID := ids.New(ids.BuildSource)
	if _, err := s.CreateApp(ctx, App{ID: carrierID, TenantID: tenant.ID, Name: carrierID, DisplayName: "mono", Type: TypeBuildSource, Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}); err != nil {
		t.Fatal(err)
	}
	web, err := s.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web-select-build", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetAppBuildSource(ctx, web.ID, carrierID, "zot/a@sha256:1"); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.SelectAppBuildSourceImage(ctx, web.ID, carrierID, "zot/a@sha256:2"); err != nil || !changed {
		t.Fatalf("a new build: changed=%v err=%v, want changed", changed, err)
	}
	if changed, err := s.SelectAppBuildSourceImage(ctx, web.ID, carrierID, "zot/a@sha256:2"); err != nil || changed {
		t.Fatalf("the same build again: changed=%v err=%v, want unchanged", changed, err)
	}
	if _, err := s.SelectAppBuildSourceImage(ctx, web.ID, "bds-other", "zot/a@sha256:3"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another source's build: err = %v, want ErrNotFound", err)
	}
}

// w9/m180: an old carrier with no display name is named by its public name,
// as the API reads it, so the locked name check compares the same way.
func TestPGBuildSourceNameCheckReadsAnEmptyDisplayNameAsThePublicName(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "bds-noname")
	ctx := context.Background()
	old := ids.New(ids.BuildSource)
	if _, err := s.CreateApp(ctx, App{ID: old, TenantID: tenant.ID, Name: old, Type: TypeBuildSource, Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}); err != nil {
		t.Fatal(err)
	}
	other := ids.New(ids.BuildSource)
	if _, err := s.CreateApp(ctx, App{ID: other, TenantID: tenant.ID, Name: other, DisplayName: old, Type: TypeBuildSource, Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}); !errors.Is(err, ErrBuildSourceNameTaken) {
		t.Fatalf("taking an unnamed carrier's public name: err = %v, want ErrBuildSourceNameTaken", err)
	}
}
