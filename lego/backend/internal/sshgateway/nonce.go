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
	"context"
	"log"
	"sync"
	"time"
)

// NonceStore is the durable side of the exec-ticket single-use guard.
type NonceStore interface {
	// ClaimShellNonce atomically claims an exec-ticket nonce in the shared
	// store — true exactly once per nonce, across every gateway replica.
	ClaimShellNonce(context.Context, string, time.Time) (bool, error)
}

// NonceGuard enforces single-use of exec-ticket nonces ACROSS gateway replicas
// (w1/042 L7). The per-process map is the cheap first line (a same-replica
// replay never reaches the database, and expired nonces are pruned in
// passing), then Store.ClaimShellNonce is the authoritative atomic claim —
// INSERT … ON CONFLICT, so of N replicas presented the same ticket exactly
// one wins. A store error refuses the nonce (fail closed): the session-audit
// write moments later requires the same database, so a DB outage already
// means no exec sessions — this adds no new availability dependency. One
// instance is shared by every ticketed transport (Browser Web Shell, sandbox
// exec) so a nonce cannot be redeemed once per feature.
type NonceGuard struct {
	// Store is the durable cross-replica claim. Nil means best-effort
	// in-memory only (no cross-replica protection); the gap is logged.
	Store NonceStore

	mu   sync.Mutex
	used map[string]time.Time
}

// Consume claims nonce until exp. It returns true exactly once per nonce; an
// empty nonce, a replay, or a store error all refuse.
func (g *NonceGuard) Consume(ctx context.Context, nonce string, exp, now time.Time) bool {
	if nonce == "" {
		return false
	}
	g.mu.Lock()
	if g.used == nil {
		g.used = map[string]time.Time{}
	}
	for n, e := range g.used {
		if now.After(e) {
			delete(g.used, n)
		}
	}
	if _, seen := g.used[nonce]; seen {
		g.mu.Unlock()
		return false
	}
	g.used[nonce] = exp
	g.mu.Unlock()

	if g.Store == nil {
		log.Printf("exec ticket nonce: store unavailable, cross-replica replay not prevented")
		return true
	}
	claimCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	claimed, err := g.Store.ClaimShellNonce(claimCtx, nonce, exp)
	if err != nil {
		log.Printf("exec ticket nonce claim failed: %v", err)
		return false
	}
	return claimed
}
