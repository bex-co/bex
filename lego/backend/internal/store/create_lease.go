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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// WorkspaceCreateBusyCode refuses a create that waited too long for another
// create in the same workspace; the caller may retry.
const WorkspaceCreateBusyCode = "WORKSPACE_CREATE_BUSY"

const (
	createLeaseTTL  = 2 * time.Minute
	createLeaseWait = 30 * time.Second
)

// WithWorkspaceCreateLease runs fn while this caller holds tenantID's create
// lease (ADR097 quota residual, w9/m180 t005). The lease is a row, taken and
// released by short statements, so no pooled connection is held while fn runs
// or while a waiter polls: a Postgres advisory lock would pin one connection
// per holder and per waiter while fn opens more, and concurrent creates could
// exhaust the pool. A holder that crashes leaves a row that expires; a long
// fn renews it. A waiter gives up after 30s with a retryable 503.
func (s *PGStore) WithWorkspaceCreateLease(ctx context.Context, tenantID string, fn func() error) error {
	return s.withWorkspaceCreateLease(ctx, tenantID, createLeaseWait, createLeaseTTL, fn)
}

func (s *PGStore) withWorkspaceCreateLease(ctx context.Context, tenantID string, wait, ttl time.Duration, fn func() error) error {
	token := make([]byte, 12)
	if _, err := rand.Read(token); err != nil {
		return err
	}
	holder := hex.EncodeToString(token)
	deadline := time.Now().Add(wait)
	backoff := 25 * time.Millisecond
	for {
		taken, err := s.takeCreateLease(ctx, tenantID, holder, ttl)
		if err != nil {
			return fmt.Errorf("take the workspace create lease: %w", err)
		}
		if taken {
			break
		}
		if time.Now().After(deadline) {
			return core.NewUnavailableError(WorkspaceCreateBusyCode,
				"another create in this workspace is still running; retry", nil)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 500*time.Millisecond)
	}
	renewed := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(renewed)
		tick := time.NewTicker(ttl / 3)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				if _, err := s.Pool.Exec(context.WithoutCancel(ctx),
					`UPDATE workspace_create_leases SET expires_at = now() + make_interval(secs => $3)
					 WHERE tenant_id = $1 AND holder = $2`, tenantID, holder, ttl.Seconds()); err != nil {
					log.Printf("store: renew the create lease of %s: %v", tenantID, err)
				}
			}
		}
	}()
	defer func() {
		close(done)
		<-renewed
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if _, err := s.Pool.Exec(releaseCtx,
			`DELETE FROM workspace_create_leases WHERE tenant_id = $1 AND holder = $2`, tenantID, holder); err != nil {
			log.Printf("store: release the create lease of %s: %v", tenantID, err)
		}
	}()
	return fn()
}

// takeCreateLease inserts tenantID's lease for holder, or takes over one
// whose holder let it expire. False while another holder's lease is live.
func (s *PGStore) takeCreateLease(ctx context.Context, tenantID, holder string, ttl time.Duration) (bool, error) {
	var got string
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO workspace_create_leases (tenant_id, holder, expires_at)
		 VALUES ($1, $2, now() + make_interval(secs => $3))
		 ON CONFLICT (tenant_id) DO UPDATE SET holder = EXCLUDED.holder, expires_at = EXCLUDED.expires_at
		 WHERE workspace_create_leases.expires_at < now()
		 RETURNING holder`, tenantID, holder, ttl.Seconds()).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil && got == holder, err
}
