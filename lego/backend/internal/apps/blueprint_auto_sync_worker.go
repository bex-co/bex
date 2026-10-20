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

package apps

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// blueprint_auto_sync_worker.go (w8/m38) consumes durable auto-sync intents
// accepted at the signed git webhook. Replaces the fire-and-forget goroutine:
// acknowledgment means the intent row exists, and a restart resumes claimed
// work after the lease expires.

const (
	blueprintAutoSyncBatch = 50
	blueprintAutoSyncLease = 5 * time.Minute
)

// blueprintAutoSyncRetry marks a refused pre-apply attempt. Accepted intent
// remains leased for recovery; no additional webhook is needed.
type blueprintAutoSyncRetry struct{ err error }

func (e *blueprintAutoSyncRetry) Error() string { return e.err.Error() }
func (e *blueprintAutoSyncRetry) Unwrap() error { return e.err }

// BlueprintAutoSyncWorker periodically claims and executes pending Blueprint
// auto-sync intents. Run is a no-op when the store is unwired.
type BlueprintAutoSyncWorker struct {
	Svc *Service
	// Interval between claim ticks; non-positive means 15s.
	Interval time.Duration
}

// Run drives the loop until ctx is canceled.
func (w *BlueprintAutoSyncWorker) Run(ctx context.Context) {
	if w == nil || w.Svc == nil || w.Svc.Blueprints == nil {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	core.Poll(ctx, "blueprint auto-sync", interval, w.tick)
}

func (w *BlueprintAutoSyncWorker) tick(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	intents, err := w.Svc.Blueprints.ClaimBlueprintAutoSyncIntents(ctx, blueprintAutoSyncBatch, blueprintAutoSyncLease)
	if err != nil {
		log.Printf("blueprint auto-sync: claim: %v", err)
		return nil
	}
	for _, intent := range intents {
		if ctx.Err() != nil {
			return nil
		}
		w.process(ctx, intent)
	}
	return nil
}

func (w *BlueprintAutoSyncWorker) process(ctx context.Context, intent store.BlueprintAutoSyncIntent) {
	b, err := w.Svc.Blueprints.GetBlueprint(ctx, intent.BlueprintID, intent.TenantID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			_ = w.Svc.Blueprints.CompleteBlueprintAutoSyncIntent(ctx, intent.ID, intent.ClaimGeneration, "Blueprint disconnected or removed")
			return
		}
		log.Printf("blueprint auto-sync: get %s: %v", intent.BlueprintID, err)
		// Storage failure is not a terminal policy decision; recover the lease.
		return
	}
	// Policy is rechecked at execution: Auto Sync may have been disabled or
	// the row disconnected between acceptance and the claim.
	if !b.AutoSync || b.Path != intent.Path {
		_ = w.Svc.Blueprints.CompleteBlueprintAutoSyncIntent(ctx, intent.ID, intent.ClaimGeneration, "Auto Sync disabled or Blueprint path changed")
		return
	}
	runCtx := core.WithActingTenant(core.WithWorkspace(ctx, b.TenantID), b.TenantID)
	_, syncErr := w.Svc.runSync(runCtx, b, "", "", nil, &intent)
	if syncErr != nil {
		var retry *blueprintAutoSyncRetry
		var coded *core.CodedError
		if errors.As(syncErr, &retry) || errors.Is(syncErr, context.Canceled) || errors.Is(syncErr, context.DeadlineExceeded) ||
			(errors.As(syncErr, &coded) && coded.Code == "BLUEPRINT_SYNC_BUSY") {
			// Leave claimed; lease expiry lets another tick retry without
			// marking a transient busy as terminal failure.
			return
		}
		_ = w.Svc.Blueprints.FailBlueprintAutoSyncIntent(ctx, intent.ID, intent.ClaimGeneration, "Automatic Blueprint sync failed; see sync history for the failure reason.")
		return
	}
	if err := w.Svc.Blueprints.CompleteBlueprintAutoSyncIntent(ctx, intent.ID, intent.ClaimGeneration, "Applied current branch; see sync history for accepted and consumed source commits"); err != nil {
		log.Printf("blueprint auto-sync: complete %s: %v", intent.ID, err)
	}
}
