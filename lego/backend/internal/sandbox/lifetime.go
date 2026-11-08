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
	"context"
	"errors"
	"fmt"
	"log"
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
	// its exact deadline by the inventory reconcile (lifetimeExpired).
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

// Runtime timeout across suspension (w2/m278 t010). Render restarts the
// runtime timeout when a suspended sandbox resumes, does not run it while the
// sandbox is suspended, and keeps a suspended sandbox resumable for 30 days
// (render.com/docs/sandboxes-limits, checked 2026-11-07). bex stamps each
// transition on the sandbox's own OpenSandbox metadata, so the inventory
// reconcile reads it from the list it already makes, with or without a store.
const (
	// metadataResumedAt is the Unix second of the sandbox's last resume. The
	// runtime timeout runs from max(createdAt, resumedAt).
	metadataResumedAt = "bex.co/resumed-at"
	// metadataSuspendedAt is the Unix second of the sandbox's last suspension.
	// While it is suspended, it is reaped at suspendedAt + suspendedRetention.
	metadataSuspendedAt = "bex.co/suspended-at"
	// suspendedRetention is how long a suspended sandbox stays resumable.
	suspendedRetention = 30 * 24 * time.Hour
)

func stampMetadataTime(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }

// metadataTime reads a stamp written by stampMetadataTime; missing or
// malformed reads as nil.
func metadataTime(v string) *time.Time {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return nil
	}
	t := time.Unix(n, 0).UTC()
	return &t
}

// lifetimeDeadline is when the inventory reconcile reaps raw; ok false means
// never. A suspended (or suspending) sandbox stamped with its suspension is
// kept for suspendedRetention from it, whatever its timeout, including no
// timeout. Otherwise the timeout (Render semantics: 0 never expires) runs
// from the later of createdAt and the last resume. A sandbox with neither
// time never expires (conservative: leave it for a tick once OpenSandbox
// supplies a create time). An unstamped suspended sandbox (paused before
// t010, or an agent-session sandbox) keeps the running rule.
func lifetimeDeadline(raw osSandbox) (time.Time, bool) {
	switch mapOpenSandboxStatus(raw.Status.State) {
	case StatusSuspending, StatusSuspended:
		if at := metadataTime(raw.Metadata[metadataSuspendedAt]); at != nil {
			return at.Add(suspendedRetention), true
		}
	}
	timeout := timeoutFromMetadata(raw.Metadata[metadataTimeout])
	if timeout == 0 {
		return time.Time{}, false
	}
	start := raw.Created
	if start != nil && start.IsZero() {
		start = nil
	}
	if resumed := metadataTime(raw.Metadata[metadataResumedAt]); resumed != nil && (start == nil || resumed.After(*start)) {
		start = resumed
	}
	if start == nil {
		return time.Time{}, false
	}
	return start.Add(time.Duration(timeout) * time.Second), true
}

// lifetimeExpired reports whether now is at or past raw's lifetime deadline.
func lifetimeExpired(raw osSandbox, now time.Time) bool {
	deadline, ok := lifetimeDeadline(raw)
	return ok && !now.Before(deadline)
}

// beforeSuspend runs ahead of every pause of an ordinary running sandbox
// (idle sweeper or explicit verb). It stamps the suspension, then moves the
// OpenSandbox expiry to the end of the 30-day suspended window: the
// BatchSandbox controller deletes a sandbox at spec.expireTime whatever its
// pause state, so without this a paused sandbox would still be deleted at
// createdAt + timeout. Both writes come first so a failure leaves the
// sandbox running, never paused under the old expiry. A no-timeout sandbox
// has no expireTime to move (OpenSandbox answers 409); the reconcile reaps
// it at the end of the window.
func (s *Service) beforeSuspend(ctx context.Context, key string, raw osSandbox) error {
	if !idleTracked(raw) || mapOpenSandboxStatus(raw.Status.State) != StatusRunning {
		return nil
	}
	now := s.Now()
	if err := s.Client.PatchMetadata(ctx, key, raw.ID, map[string]string{metadataSuspendedAt: stampMetadataTime(now)}); err != nil {
		return fmt.Errorf("stamp sandbox suspension: %w", err)
	}
	if timeoutFromMetadata(raw.Metadata[metadataTimeout]) == 0 {
		return nil
	}
	if err := s.Client.RenewExpiration(ctx, key, raw.ID, now.Add(suspendedRetention)); err != nil && !errors.Is(err, errOpenSandboxConflict) {
		return fmt.Errorf("extend suspended sandbox expiry: %w", err)
	}
	return nil
}

// beforeResume stamps the resume of an ordinary suspended sandbox ahead of
// the call, so the reconcile never reads a resumed sandbox's timeout from an
// older start. A failed stamp refuses the resume.
func (s *Service) beforeResume(ctx context.Context, key string, raw osSandbox) error {
	if !idleTracked(raw) || mapOpenSandboxStatus(raw.Status.State) != StatusSuspended {
		return nil
	}
	if err := s.Client.PatchMetadata(ctx, key, raw.ID, map[string]string{metadataResumedAt: stampMetadataTime(s.Now())}); err != nil {
		return fmt.Errorf("stamp sandbox resume: %w", err)
	}
	return nil
}

// afterResume restarts the OpenSandbox expiry at resume time + timeout once
// a resume is accepted. A failure is logged: the reconcile still reaps the
// sandbox at its own deadline, and the old expiry is the end of the
// suspended window.
func (s *Service) afterResume(ctx context.Context, key string, raw osSandbox) {
	if !idleTracked(raw) || mapOpenSandboxStatus(raw.Status.State) != StatusSuspended {
		return
	}
	timeout := openSandboxTimeout(timeoutFromMetadata(raw.Metadata[metadataTimeout]))
	if timeout == 0 {
		return
	}
	at := s.Now().Add(time.Duration(timeout) * time.Second)
	if err := s.Client.RenewExpiration(ctx, key, raw.ID, at); err != nil && !errors.Is(err, errOpenSandboxConflict) {
		log.Printf("sandbox lifetime: renew expiry on resume sandbox=%s: %v", raw.ID, err)
	}
}
