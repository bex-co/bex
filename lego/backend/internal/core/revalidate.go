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

package core

import (
	"context"
	"time"
)

// DefaultStreamRevalidateInterval is how often a long-lived sensitive stream
// re-runs its fresh authorization (codex round-9 #6 for SSH/agent transports;
// extended to live log subscriptions in round 15). A membership or API-key
// revocation then ends the stream at the interval instead of at disconnect.
const DefaultStreamRevalidateInterval = time.Minute

// WithRevalidation derives a cancellable child of parent whose watchdog
// re-runs check every interval until the child is canceled; the first failed
// check (revocation, target gone, or the checker failing closed on an
// unreachable store) cancels the child, which is what ends the stream: every
// live transport pumps on its context, so canceling it tears down the exec,
// bridge, splice, or log follow from below. interval <= 0 disables the
// watchdog and the result degrades to a plain context.WithCancel.
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
