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
			_ = w.Svc.Blueprints.CompleteBlueprintAutoSyncIntent(ctx, intent.ID)
			return
		}
		log.Printf("blueprint auto-sync: get %s: %v", intent.BlueprintID, err)
		_ = w.Svc.Blueprints.FailBlueprintAutoSyncIntent(ctx, intent.ID, "blueprint lookup failed")
		return
	}
	// Policy is rechecked at execution: Auto Sync may have been disabled or
	// the row disconnected between acceptance and the claim.
	if !b.AutoSync {
		_ = w.Svc.Blueprints.CompleteBlueprintAutoSyncIntent(ctx, intent.ID)
		return
	}
	reviewed := &ReviewedBlueprintSource{
		Repo:     b.Repo,
		Path:     intent.Path,
		CommitID: intent.CommitSHA,
	}
	runCtx := core.WithActingTenant(core.WithWorkspace(ctx, b.TenantID), b.TenantID)
	_, syncErr := w.Svc.runSync(runCtx, b, "", "", reviewed)
	if syncErr != nil {
		var coded *core.CodedError
		if errors.As(syncErr, &coded) && coded.Code == "BLUEPRINT_SYNC_BUSY" {
			// Leave claimed; lease expiry lets another tick retry without
			// marking a transient busy as terminal failure.
			return
		}
		msg := syncErr.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
		_ = w.Svc.Blueprints.FailBlueprintAutoSyncIntent(ctx, intent.ID, msg)
		return
	}
	if err := w.Svc.Blueprints.CompleteBlueprintAutoSyncIntent(ctx, intent.ID); err != nil {
		log.Printf("blueprint auto-sync: complete %s: %v", intent.ID, err)
	}
}
