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
	"strings"
	"sync"
	"time"
)

// BudgetSweepInterval / BudgetIdleTTL keep a RequestBudget's key set bounded:
// agent sessions are short-lived (minutes to hours), so entries idle for a day
// are gone sessions and are pruned. The janitor stops with the process.
const (
	BudgetSweepInterval = 30 * time.Minute
	BudgetIdleTTL       = 24 * time.Hour
)

// RequestBudget is the process-wide cumulative exchange counter shared by the
// model and git credential proxies (round-13 #8): per-session and
// per-workspace atomic counts. Sessions are keyed by namespace+session id (the
// verified pair), workspaces by the `<ws>` the `<ws>-sandbox` namespace derives
// from. One canonical implementation keeps the two brokers' budgets identical
// — this logic was copied verbatim between them.
type RequestBudget struct {
	perSession   int
	perWorkspace int
	mu           sync.Mutex
	charged      map[string]int
	lastUsed     map[string]time.Time
	janitorOnce  sync.Once
}

// NewRequestBudget builds a budget with the given per-session and
// per-workspace caps; a non-positive cap disables that dimension.
func NewRequestBudget(perSession, perWorkspace int) *RequestBudget {
	return &RequestBudget{perSession: perSession, perWorkspace: perWorkspace}
}

// Charge records one request against key, refusing past max. max <= 0 disables
// the dimension. The first charge starts the janitor that prunes idle keys.
func (b *RequestBudget) Charge(key string, max int) bool {
	if max <= 0 {
		return true // dimension disabled
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.charged == nil {
		b.charged = make(map[string]int)
		b.lastUsed = make(map[string]time.Time)
		b.janitorOnce.Do(func() {
			go func() {
				ticker := time.NewTicker(BudgetSweepInterval)
				defer ticker.Stop()
				for range ticker.C {
					b.Sweep(time.Now().Add(-BudgetIdleTTL))
				}
			}()
		})
	}
	if b.charged[key] >= max {
		return false
	}
	b.charged[key]++
	b.lastUsed[key] = time.Now()
	return true
}

// Sweep drops keys idle past the horizon so finished sessions do not accumulate
// forever (the budget map must not become its own unbounded state).
func (b *RequestBudget) Sweep(horizon time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for key, at := range b.lastUsed {
		if at.Before(horizon) {
			delete(b.charged, key)
			delete(b.lastUsed, key)
		}
	}
}

// ChargeSession charges one request against the given verified session pair.
func (b *RequestBudget) ChargeSession(namespace, sessionID string) bool {
	return b.Charge("session|"+namespace+"|"+sessionID, b.perSession)
}

// ChargeWorkspace charges one request against the workspace the sandbox
// namespace belongs to.
func (b *RequestBudget) ChargeWorkspace(namespace string) bool {
	return b.Charge("workspace|"+strings.TrimSuffix(namespace, "-sandbox"), b.perWorkspace)
}
