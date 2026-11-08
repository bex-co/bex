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

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Render's timeoutSeconds contract (pinned CLI v2.35 `--timeout` help and the
// CreateSandbox 400 `invalid_timeout_seconds`): omitted means 86400 s; 0 means
// no timeout; the maximum is 2592000 s (30 days), or 86400 s on Hobby
// workspaces (w2/m263 D3=A, amending w5/m99's single 86400 s cap).
const (
	defaultSandboxTimeout  = 86400
	hobbySandboxMaxTimeout = 86400
	paidSandboxMaxTimeout  = 2592000
	// osMinTimeout is OpenSandbox's own floor (CreateSandboxRequest.timeout
	// ge=60). A shorter Render timeout is sent as this floor and enforced at
	// its exact deadline by the inventory reconcile (LifetimeExpired).
	osMinTimeout = 60
	// timeoutNone is the bex.co/timeout-seconds value of a sandbox created
	// with no timeout. A distinct marker, not "0": pre-w5/m99 metadata "0"
	// keeps meaning the 86400 s default, so no legacy sandbox becomes
	// immortal, and an older bex-api reading "none" falls back to 86400 s.
	timeoutNone = "none"
)

// resolveTimeout applies Render's timeoutSeconds contract for a workspace on
// plan. requested nil is an omitted timeout. The result uses Render's
// semantics: 0 is no timeout, otherwise the lifetime in seconds. On Hobby an
// explicit 0 is held to the Hobby maximum (86400 s, reported truthfully),
// since Render caps Hobby at 24 h and refuses only values below 0 or above
// the plan maximum. An unknown plan (store off) gets the paid maximum, the
// degrade every other plan gate uses.
func resolveTimeout(requested *int, plan string) (int, error) {
	if requested == nil {
		return defaultSandboxTimeout, nil
	}
	hobby := plan == store.PlanHobby
	maxTimeout := paidSandboxMaxTimeout
	if hobby {
		maxTimeout = hobbySandboxMaxTimeout
	}
	t := *requested
	switch {
	case t < 0 || t > maxTimeout:
		msg := fmt.Sprintf("sandbox timeoutSeconds must be 0 (no timeout) or between 1 and %d", maxTimeout)
		params := map[string]any{"field": "timeoutSeconds", "max": maxTimeout}
		if hobby {
			msg = fmt.Sprintf("sandbox timeoutSeconds must be between 0 and %d on a Hobby workspace; upgrade the workspace plan for up to %d (30 days) or no timeout", maxTimeout, paidSandboxMaxTimeout)
			params["plan"] = plan
		}
		return 0, core.NewBadRequestError("SANDBOX_TIMEOUT_INVALID", msg, params)
	case t == 0 && hobby:
		return hobbySandboxMaxTimeout, nil
	}
	return t, nil
}

// timeoutMetadata is the bex.co/timeout-seconds value for a resolved timeout.
func timeoutMetadata(timeout int) string {
	if timeout == 0 {
		return timeoutNone
	}
	return strconv.Itoa(timeout)
}

// timeoutFromMetadata reads bex.co/timeout-seconds back in Render's
// semantics (0 = no timeout). Missing, legacy "0" and malformed values mean
// the 86400 s default — never an immortal sandbox (w5/m99).
func timeoutFromMetadata(v string) int {
	if v == timeoutNone {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultSandboxTimeout
	}
	return n
}

// openSandboxTimeout is the substrate timeout for a resolved one: 0 (field
// omitted) leaves the BatchSandbox without an expireTime, and a positive
// value is raised to OpenSandbox's floor.
func openSandboxTimeout(timeout int) int {
	if timeout == 0 {
		return 0
	}
	return max(timeout, osMinTimeout)
}

// LifetimeExpired reports whether now is at or past createdAt + timeoutSeconds
// (Render semantics: 0 never expires). Missing createdAt never expires
// (conservative — leave for the next tick once OpenSandbox supplies a create
// time).
func LifetimeExpired(createdAt *time.Time, timeoutSeconds int, now time.Time) bool {
	if timeoutSeconds == 0 || createdAt == nil || createdAt.IsZero() {
		return false
	}
	return !now.Before(createdAt.Add(time.Duration(timeoutSeconds) * time.Second))
}
