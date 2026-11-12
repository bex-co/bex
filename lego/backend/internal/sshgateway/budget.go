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

package sshgateway

import (
	"sync"
	"time"
)

// exchangeBudgetSweepInterval / exchangeBudgetIdleTTL keep an ExchangeBudget's
// key set bounded: agent sessions are short-lived (minutes to hours), so a key
// that has seen no request — charged or refused — for a day belongs to a gone
// session and is pruned.
const (
	exchangeBudgetSweepInterval = 30 * time.Minute
	exchangeBudgetIdleTTL       = 24 * time.Hour
)

// ExchangeBudget is one credential proxy's cumulative exchange counter (ADR068
// round-13 #8), charged before the credential mint. Sessions are keyed by the
// verified namespace+session pair, workspaces by the verified workspace. The
// model and git proxies each build their own, so their caps are independent. A
// cap <= 0 disables that dimension.
type ExchangeBudget struct {
	perSession, perWorkspace int

	mu        sync.Mutex
	entries   map[string]budgetEntry
	nextSweep time.Time
}

type budgetEntry struct {
	charged  int
	lastSeen time.Time
}

// NewExchangeBudget returns a budget capping each session at perSession and
// each workspace at perWorkspace exchanges.
func NewExchangeBudget(perSession, perWorkspace int) *ExchangeBudget {
	return &ExchangeBudget{
		perSession:   perSession,
		perWorkspace: perWorkspace,
		entries:      make(map[string]budgetEntry),
	}
}

// ChargeSession counts one exchange for the session and reports whether it
// fits the per-session cap.
func (b *ExchangeBudget) ChargeSession(namespace, sessionID string) bool {
	return b.charge("session|"+namespace+"|"+sessionID, b.perSession, time.Now())
}

// ChargeWorkspace counts one exchange for the workspace and reports whether it
// fits the per-workspace cap.
func (b *ExchangeBudget) ChargeWorkspace(workspace string) bool {
	return b.charge("workspace|"+workspace, b.perWorkspace, time.Now())
}

// charge counts one exchange against key. A refusal still marks the key as
// seen: an exhausted key that keeps getting requests (a runaway loop retrying
// its 429s) must stay exhausted, not age out of the idle sweep and come back
// with a fresh budget.
func (b *ExchangeBudget) charge(key string, limit int, now time.Time) bool {
	if limit <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sweepIdle(now)
	entry := b.entries[key]
	entry.lastSeen = now
	admitted := entry.charged < limit
	if admitted {
		entry.charged++
	}
	b.entries[key] = entry
	return admitted
}

// sweepIdle drops keys unseen for exchangeBudgetIdleTTL, at most once per
// exchangeBudgetSweepInterval, so the map cannot become its own unbounded
// state. Sweeping inline means a budget owns no goroutine and is collected with
// its broker. Callers hold mu.
func (b *ExchangeBudget) sweepIdle(now time.Time) {
	if now.Before(b.nextSweep) {
		return
	}
	b.nextSweep = now.Add(exchangeBudgetSweepInterval)
	horizon := now.Add(-exchangeBudgetIdleTTL)
	for key, entry := range b.entries {
		if entry.lastSeen.Before(horizon) {
			delete(b.entries, key)
		}
	}
}
