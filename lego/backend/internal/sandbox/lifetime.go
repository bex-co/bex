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

package sandbox

import "time"

// EffectiveTimeoutSeconds maps a create request or stored metadata value onto
// the enforced maximum lifetime. timeoutSeconds 0 (CLI default / legacy "no
// expiry") means the documented default and maximum of maxSandboxTimeout
// (86400), matching the pinned Render CLI — never an immortal sandbox (w5/m99).
func EffectiveTimeoutSeconds(requested int) int {
	if requested <= 0 {
		return maxSandboxTimeout
	}
	return requested
}

// LifetimeDeadline is createdAt + the effective timeout. ok is false when
// createdAt is missing — callers must not terminate on an inconclusive clock.
func LifetimeDeadline(createdAt *time.Time, timeoutSeconds int) (time.Time, bool) {
	if createdAt == nil || createdAt.IsZero() {
		return time.Time{}, false
	}
	return createdAt.Add(time.Duration(EffectiveTimeoutSeconds(timeoutSeconds)) * time.Second), true
}

// LifetimeExpired reports whether now is at or past the sandbox's lifetime
// deadline. Missing createdAt never expires (conservative — leave for the next
// tick once OpenSandbox supplies a create time).
func LifetimeExpired(createdAt *time.Time, timeoutSeconds int, now time.Time) bool {
	deadline, ok := LifetimeDeadline(createdAt, timeoutSeconds)
	if !ok {
		return false
	}
	return !now.Before(deadline)
}
