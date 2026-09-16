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

package controller

import (
	"sort"
	"sync"
	"time"
)

// The bounded production trial of the registry build cache (docs/ADR060 D3,
// .pm/w7/047) is defined by stop rules — a hard stop at 72 hours, a push-latency
// ceiling, and cache-attributable push failures. Until this file those rules
// lived only in a drill document and depended on a human watching a dashboard
// for three days and running `kubectl set env` at the right moment. That is not
// a guardrail; it is a promise, and it is why the trial never started.
//
// buildCacheTrial makes the stop rules a property of the operator instead. It
// holds the gate that BEX_BUILD_CACHE opens and withdraws it on its own, from
// signals the operator already collects, so the window needs no observer and no
// human hand on a rollback command.
//
// What it deliberately does NOT do is decide the trial's OUTCOME. Withdrawing
// the cache is the safe direction — a trip costs a slow build, nothing more —
// but retaining the feature is a judgement about measured benefit that belongs
// to a person reading the numbers. A tripped breaker ends the experiment; it
// does not record a verdict.
//
// Interaction with GitOps is the reason this is an in-process breaker rather
// than a controller that edits its own Deployment. BEX_BUILD_CACHE lives in the
// production overlay, so Argo owns it: an operator that removed its own env var
// would be reverted on the next sync, and the two would fight for as long as
// the trial ran. Withdrawing the cache *behind* an unchanged env var has no such
// conflict — Git keeps saying "trial requested", the operator says "trial over",
// and the second one wins where it matters, at build dispatch.
type buildCacheTrial struct {
	// deadline is the hard stop. Absolute rather than a duration from process
	// start, because a duration would restart the clock on every manager
	// rollout: a trial that survived three restarts would quietly run far past
	// 72 hours, which is the exact failure the hard stop exists to prevent.
	// Zero means no deadline — the shape a retain decision leaves behind.
	deadline time.Time

	// maxPushSeconds is the push-latency ceiling, compared against the p95 of
	// the observations still inside window. The drill sets it at a 60s floor
	// over a measured ~27s baseline.
	maxPushSeconds float64
	// minPushSamples is how many observations must be in window before the p95
	// is allowed to trip anything. A percentile over two data points is noise,
	// and tripping on noise would end the trial without evidence.
	minPushSamples int
	// maxPushFailures is how many failed pushes inside window withdraw the
	// cache. The drill's rule for a human reads "a cache-related push failure",
	// but the operator cannot attribute a push failure to the cache — skopeo
	// reports an exit status, not a cause. Tripping on the first unattributable
	// failure would withdraw the cache for causes the trial is not testing (a
	// registry restart, a node's disk pressure), so the automated analogue is a
	// small count inside the window. The measured baseline is zero push errors
	// in 24 hours, so this stays a sensitive rule, not a permissive one.
	maxPushFailures int
	// window bounds every rolling judgement above.
	window time.Duration

	mu sync.Mutex
	// tripped is the reason the cache was withdrawn, empty while it is live.
	// Once set it is never cleared: a breaker that re-armed itself could
	// oscillate the cache on and off across a trial and make the resulting
	// measurements unreadable. Re-enabling is a deploy, which is also the point
	// at which a human has looked.
	tripped string
	pushes  []pushObservation
}

// pushObservation is one finished build's push phase.
type pushObservation struct {
	at      time.Time
	seconds float64
	failed  bool
}

// Trial stop-rule defaults, from docs/drills/2026-09-09-build-cache-enablement.md.
const (
	// The drill's stop rule is "push p95 persistently above 60 seconds", where
	// 60s is a floor chosen to sit well above the ~27s production baseline —
	// the rule fires on a regression, not on normal variance.
	defaultTrialMaxPushSeconds = 60.0
	// Five samples is the drill's own minimum for reading the percentile.
	defaultTrialMinPushSamples = 5
	// See maxPushFailures: the automated stand-in for "a cache-related push
	// failure", which the operator cannot identify as cache-related.
	defaultTrialMaxPushFailures = 3
	// The drill evaluates its push rule over 15 minutes.
	defaultTrialWindow = 15 * time.Minute
)

// newBuildCacheTrial builds a breaker with the drill's default stop rules. A
// zero deadline leaves only the latency and failure rules armed.
func newBuildCacheTrial(deadline time.Time) *buildCacheTrial {
	return &buildCacheTrial{
		deadline:        deadline,
		maxPushSeconds:  defaultTrialMaxPushSeconds,
		minPushSamples:  defaultTrialMinPushSamples,
		maxPushFailures: defaultTrialMaxPushFailures,
		window:          defaultTrialWindow,
	}
}

// allow reports whether a build dispatched now may carry cache phases.
//
// The nil receiver is a real case and means "unbounded": every existing test
// constructs an AppReconciler without a trial, and a development cluster that
// sets BEX_BUILD_CACHE has no trial to bound. Those callers get the behavior
// they had before this file existed.
func (t *buildCacheTrial) allow(now time.Time) bool {
	if t == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tripped != "" {
		return false
	}
	// Checked on the dispatch path rather than from a timer so the hard stop
	// holds even if the manager was asleep, throttled, or restarted across the
	// deadline: the question is always asked at the moment it matters.
	if !t.deadline.IsZero() && !now.Before(t.deadline) {
		t.trip(trialStopDeadline)
		return false
	}
	return true
}

// observePush feeds one finished build's push phase to the latency and failure
// rules. It is called from the same once-per-build path that meters the push
// histogram, so a level-triggered re-reconcile cannot double-count a push into
// the breaker's window.
func (t *buildCacheTrial) observePush(now time.Time, seconds float64, failed bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tripped != "" {
		return
	}
	// A push with no measured duration (the kpack path reports none) still
	// carries a usable failure signal, so failures are recorded either way and
	// only the latency sample is conditional.
	t.pushes = append(t.pushes, pushObservation{at: now, seconds: seconds, failed: failed})
	t.evictBefore(now.Add(-t.window))

	failures, latencies := 0, make([]float64, 0, len(t.pushes))
	for _, p := range t.pushes {
		if p.failed {
			failures++
		}
		if p.seconds > 0 {
			latencies = append(latencies, p.seconds)
		}
	}
	if failures >= t.maxPushFailures {
		t.trip(trialStopPushErrors)
		return
	}
	if len(latencies) >= t.minPushSamples && percentile(latencies, 0.95) > t.maxPushSeconds {
		t.trip(trialStopPushLatency)
	}
}

// evictBefore drops observations that have aged out of the window. Called under
// t.mu.
func (t *buildCacheTrial) evictBefore(cutoff time.Time) {
	keep := t.pushes[:0]
	for _, p := range t.pushes {
		if p.at.After(cutoff) {
			keep = append(keep, p)
		}
	}
	t.pushes = keep
}

// trip withdraws the cache for the rest of this process's life. Called under
// t.mu; the first reason wins, so the metric attributes the withdrawal to what
// actually ended the trial rather than to whatever was checked last.
func (t *buildCacheTrial) trip(reason string) {
	if t.tripped != "" {
		return
	}
	t.tripped = reason
	// Dropping the retained observations keeps a tripped breaker from holding a
	// window of samples it will never read again.
	t.pushes = nil
	recordBuildCacheWithdrawn(reason)
}

// stopReason reports why the cache was withdrawn, or "" while it is still live.
func (t *buildCacheTrial) stopReason() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.tripped
}

// percentile returns the linearly-interpolated q-quantile of vals. vals is
// sorted in place, which is safe because every caller passes a slice it built.
func percentile(vals []float64, q float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sort.Float64s(vals)
	if len(vals) == 1 {
		return vals[0]
	}
	pos := q * float64(len(vals)-1)
	lo := int(pos)
	if lo >= len(vals)-1 {
		return vals[len(vals)-1]
	}
	frac := pos - float64(lo)
	return vals[lo] + frac*(vals[lo+1]-vals[lo])
}
