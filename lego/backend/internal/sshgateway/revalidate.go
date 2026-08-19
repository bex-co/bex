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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// DefaultRevalidateInterval is how often a live gateway stream re-runs its
// authoritative admission checks (codex round-9 #6). The helper itself lives
// in core so live log subscriptions (round 15) share one watchdog.
const DefaultRevalidateInterval = core.DefaultStreamRevalidateInterval

// WithRevalidation is the gateway-facing name of core.WithRevalidation.
func WithRevalidation(parent context.Context, interval time.Duration, check func(context.Context) error) (context.Context, context.CancelFunc) {
	return core.WithRevalidation(parent, interval, check)
}
