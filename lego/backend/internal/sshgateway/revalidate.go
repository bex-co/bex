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
	"time"
)

// DefaultRevalidateInterval is how often a live gateway stream re-runs its
// authoritative admission checks (codex round-9 #6): round-6/7/8 hardened the
// admission edge — fresh (uncached) authorization before a channel is accepted,
// a web shell upgraded, or an attach ticket redeemed — but an admitted stream
// then ran to its disconnect or the 4h session cap, so a key deletion,
// membership revocation, or session cancel only took effect at the NEXT
// admission. The watchdog closes that window to the interval.
const DefaultRevalidateInterval = time.Minute

// WithRevalidation derives a cancellable child of parent whose watchdog
// re-runs check every interval until the child is canceled; the first failed
// check (revocation, target gone, or the checker failing closed on an
// unreachable store) cancels the child, which is what ends the stream: every
// live transport pumps on its context, so canceling it tears down the exec,
// bridge, or splice from below. interval <= 0 disables the watchdog and the
// result degrades to a plain context.WithCancel — the pre-round-9 behavior.
//
// The returned Cancel stops the watchdog AND cancels the child; callers that
// already defer a wait-group on the child keep their existing defer order.
func WithRevalidation(parent context.Context, interval time.Duration, check func(context.Context) error) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	if interval <= 0 || check == nil {
		return ctx, cancel
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := check(ctx); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}
