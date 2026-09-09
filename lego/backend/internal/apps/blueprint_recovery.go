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
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// blueprint_recovery.go (w8/m37 t004) settles Blueprint sync runs abandoned by
// process loss: without it a Blueprint whose API replica died mid-apply would
// read running/syncing forever, and its active claim would fence every later
// sync and disconnect until an operator intervened.
//
// The sweep is bounded (indexed oldest-first batch per tick, short statement
// timeout) and generation-fenced: a demonstrably live run is never listed,
// and settling touches the Blueprint row only while the abandoned generation
// still owns it — disconnected rows and newer generations are preserved. A
// settled run carries an actionable retry instruction; partial application is
// never replayed, only retried through an explicit new sync.

// BlueprintRecoveryStore is the persisted surface the recovery sweep needs.
// *store.PGStore satisfies it; the apps fake backs the unit tests.
type BlueprintRecoveryStore interface {
	ListAbandonedBlueprintSyncs(ctx context.Context, before time.Time, limit int) ([]store.AbandonedBlueprintSync, error)
	AbandonBlueprintSync(ctx context.Context, runID string, now time.Time, reason string) (bool, error)
}

// blueprintRecoveryBatch bounds one sweep tick (w5/m85 precedent: 100).
const blueprintRecoveryBatch = 100

// BlueprintRecoverer periodically settles abandoned Blueprint sync runs. Run
// drives the loop until ctx is canceled; it returns immediately when the store
// is unwired, keeping store-off builds unchanged.
type BlueprintRecoverer struct {
	Store BlueprintRecoveryStore
	// Interval between sweeps; non-positive means one minute.
	Interval time.Duration
	// Now overrides the clock in tests.
	Now func() time.Time
}

func (r *BlueprintRecoverer) now() time.Time {
	if r == nil || r.Now == nil {
		return time.Now()
	}
	return r.Now()
}

// Run drives the loop until ctx is canceled. It is a no-op (returns
// immediately) when the store is unwired.
func (r *BlueprintRecoverer) Run(ctx context.Context) {
	if r == nil || r.Store == nil {
		return
	}
	interval := r.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.recoverOnce(ctx)
		}
	}
}

// recoverOnce settles one bounded batch of abandoned runs. A single run's
// failure is isolated; it never blocks the others. Independent cleanup
// contexts (per-call timeout) keep one slow store call from starving the tick.
func (r *BlueprintRecoverer) recoverOnce(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	now := r.now()
	due, err := r.Store.ListAbandonedBlueprintSyncs(ctx, now.Add(-store.BlueprintRunRecoveryBound), blueprintRecoveryBatch)
	if err != nil {
		log.Printf("blueprint recovery: list abandoned runs: %v", err)
		return
	}
	for _, d := range due {
		if ctx.Err() != nil {
			return
		}
		settled, err := r.Store.AbandonBlueprintSync(ctx, d.RunID, now, store.BlueprintRunInterruptedReason)
		if err != nil {
			log.Printf("blueprint recovery: abandon run %s: %v", d.RunID, err)
			continue
		}
		if settled {
			log.Printf("blueprint recovery: settled abandoned run %s (blueprint %s)", d.RunID, d.BlueprintID)
		}
	}
}
