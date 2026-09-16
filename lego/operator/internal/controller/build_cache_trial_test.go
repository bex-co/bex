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
	"sync"
	"testing"
	"time"
)

// The trial breaker exists so the 48–72h production window needs no human
// watching it (.pm/w7/047). Each test below is one stop rule actually stopping
// something — a breaker that cannot be shown to trip is worth nothing, because
// the whole point is that nobody is looking when it matters.

func TestCacheTrialAllowsUntilTheDeadline(t *testing.T) {
	start := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	trial := newBuildCacheTrial(start.Add(72 * time.Hour))

	if !trial.allow(start) {
		t.Fatal("cache withdrawn at the start of its own trial")
	}
	if !trial.allow(start.Add(71*time.Hour + 59*time.Minute)) {
		t.Error("cache withdrawn a minute before the deadline")
	}
	if trial.allow(start.Add(72 * time.Hour)) {
		t.Error("cache still dispatched AT the 72h hard stop; the deadline is inclusive by design")
	}
	if got := trial.stopReason(); got != trialStopDeadline {
		t.Errorf("stop reason = %q, want %q", got, trialStopDeadline)
	}
}

// The hard stop has to hold across a manager restart's worth of elapsed time
// even though the breaker itself is in-process: an operator asleep, throttled,
// or simply not dispatching builds over the deadline must still refuse the
// cache the moment it is asked again. This is why allow() checks the clock on
// the dispatch path instead of a timer firing at the deadline.
func TestCacheTrialDeadlineHoldsWhenNothingRanDuringTheWindow(t *testing.T) {
	start := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	trial := newBuildCacheTrial(start.Add(72 * time.Hour))

	// First question asked a week after the deadline — no observations, no
	// intervening calls, exactly the shape of a manager that was rolled.
	if trial.allow(start.Add(7 * 24 * time.Hour)) {
		t.Fatal("cache dispatched a week past the hard stop")
	}
}

func TestCacheTrialWithoutDeadlineStillArmsTheOtherRules(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	trial := newBuildCacheTrial(time.Time{})

	if !trial.allow(now.Add(365 * 24 * time.Hour)) {
		t.Error("a zero deadline should mean unbounded in TIME, not an immediate stop")
	}
	for range defaultTrialMaxPushFailures {
		trial.observePush(now, 5, true)
	}
	if trial.allow(now) {
		t.Error("push-failure rule disarmed just because no deadline was set")
	}
}

func TestCacheTrialTripsOnSustainedPushLatency(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	trial := newBuildCacheTrial(now.Add(72 * time.Hour))

	// Four slow pushes are one short of the sample minimum: a percentile over
	// too little data must not end the trial.
	for range defaultTrialMinPushSamples - 1 {
		trial.observePush(now, 300, false)
	}
	if !trial.allow(now) {
		t.Fatalf("withdrew the cache on %d samples; the rule needs %d",
			defaultTrialMinPushSamples-1, defaultTrialMinPushSamples)
	}

	trial.observePush(now, 300, false)
	if trial.allow(now) {
		t.Error("push p95 of 300s over the sample minimum did not withdraw the cache")
	}
	if got := trial.stopReason(); got != trialStopPushLatency {
		t.Errorf("stop reason = %q, want %q", got, trialStopPushLatency)
	}
}

// The production baseline is a ~27s push p95 against a 60s ceiling. A trial that
// tripped on that baseline would be useless — it would end itself on day one
// and report a regression that never happened.
func TestCacheTrialToleratesTheMeasuredProductionBaseline(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	trial := newBuildCacheTrial(now.Add(72 * time.Hour))

	// The drill's measured distribution: p50 ~9s, p95 ~27s.
	for _, s := range []float64{7, 8, 9, 9, 10, 11, 12, 14, 19, 27} {
		trial.observePush(now, s, false)
	}
	if !trial.allow(now) {
		t.Errorf("withdrew the cache on the measured production baseline (p95 ~27s vs a %vs ceiling)",
			defaultTrialMaxPushSeconds)
	}
}

// Slow pushes that have aged out of the window must not accumulate into a trip
// hours later, or a single bad afternoon would end a trial that had since
// recovered.
func TestCacheTrialForgetsObservationsOutsideTheWindow(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	trial := newBuildCacheTrial(now.Add(72 * time.Hour))

	// Four slow pushes, then the window passes, then four more. Eight slow
	// pushes in total but never more than four in window, so the sample
	// minimum is never met and the cache survives.
	for range defaultTrialMinPushSamples - 1 {
		trial.observePush(now, 300, false)
	}
	later := now.Add(defaultTrialWindow + time.Minute)
	for range defaultTrialMinPushSamples - 1 {
		trial.observePush(later, 300, false)
	}
	if !trial.allow(later) {
		t.Error("aged-out observations still counted toward the latency rule")
	}
}

func TestCacheTrialTripsOnRepeatedPushFailures(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	trial := newBuildCacheTrial(now.Add(72 * time.Hour))

	for range defaultTrialMaxPushFailures - 1 {
		trial.observePush(now, 5, true)
	}
	if !trial.allow(now) {
		t.Fatalf("withdrew the cache on %d push failures; the rule allows up to %d",
			defaultTrialMaxPushFailures-1, defaultTrialMaxPushFailures)
	}

	trial.observePush(now, 5, true)
	if trial.allow(now) {
		t.Error("repeated push failures did not withdraw the cache")
	}
	if got := trial.stopReason(); got != trialStopPushErrors {
		t.Errorf("stop reason = %q, want %q", got, trialStopPushErrors)
	}
}

// A failed push with no measured duration (the kpack path reports none) still
// has to reach the failure rule: dropping it would make the breaker blind to
// exactly the builds whose push died before it could be timed.
func TestCacheTrialCountsFailuresWithNoMeasuredDuration(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	trial := newBuildCacheTrial(now.Add(72 * time.Hour))

	for range defaultTrialMaxPushFailures {
		trial.observePush(now, 0, true)
	}
	if trial.allow(now) {
		t.Error("push failures with no timing were dropped instead of counted")
	}
}

// Once withdrawn the cache stays withdrawn for the process's life. A breaker
// that re-armed itself would oscillate the cache across a trial and make the
// measurements it exists to protect unreadable.
func TestCacheTrialDoesNotReArmAfterTripping(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	trial := newBuildCacheTrial(now.Add(72 * time.Hour))

	for range defaultTrialMaxPushFailures {
		trial.observePush(now, 5, true)
	}
	// A clean hour of fast, successful pushes must not bring it back.
	for range 50 {
		trial.observePush(now.Add(time.Hour), 3, false)
	}
	if trial.allow(now.Add(time.Hour)) {
		t.Error("breaker re-armed itself after recovery; re-enabling is a deploy, not a self-decision")
	}
	if got := trial.stopReason(); got != trialStopPushErrors {
		t.Errorf("first stop reason should win, got %q", got)
	}
}

// Nil is the shape every existing test and every development cluster uses, and
// it must behave exactly as the code did before the breaker existed.
func TestNilCacheTrialIsUnbounded(t *testing.T) {
	var trial *buildCacheTrial

	if !trial.allow(time.Now()) {
		t.Error("nil trial withdrew the cache; it means unbounded")
	}
	trial.observePush(time.Now(), 9000, true) // must not panic
	if got := trial.stopReason(); got != "" {
		t.Errorf("nil trial stop reason = %q, want empty", got)
	}
}

// The reconciler consults the breaker on the dispatch path, so a trip has to be
// visible through the same predicate the build dispatcher uses — not just on
// the trial object.
func TestBuildCacheAllowedFollowsTheBreaker(t *testing.T) {
	now := time.Now()
	r := &AppReconciler{BuildCache: true}
	r.EnableBuildCacheTrial(now.Add(-time.Minute)) // already past its deadline

	if r.buildCacheAllowed() {
		t.Error("dispatch still requests cache phases after the trial deadline passed")
	}

	off := &AppReconciler{BuildCache: false}
	off.EnableBuildCacheTrial(now.Add(72 * time.Hour))
	if off.buildCacheAllowed() {
		t.Error("an unset BEX_BUILD_CACHE must stay off regardless of the trial")
	}

	unbounded := &AppReconciler{BuildCache: true}
	if !unbounded.buildCacheAllowed() {
		t.Error("a reconciler with no trial armed should cache as it always did")
	}
}

// observePush runs from reconcile, which is concurrent across Apps. The race
// detector makes this meaningful; without the mutex it reports a data race on
// the observation slice.
func TestCacheTrialIsSafeUnderConcurrentObservations(t *testing.T) {
	now := time.Now()
	trial := newBuildCacheTrial(now.Add(72 * time.Hour))

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 64 {
				trial.observePush(time.Now(), 5, false)
				trial.allow(time.Now())
				trial.stopReason()
			}
		})
	}
	wg.Wait()
}

func TestPercentileInterpolates(t *testing.T) {
	for _, tc := range []struct {
		name string
		vals []float64
		q    float64
		want float64
	}{
		{"empty", nil, 0.95, 0},
		{"single", []float64{42}, 0.95, 42},
		{"max at q=1", []float64{1, 2, 3, 4}, 1, 4},
		{"median of an even set interpolates", []float64{1, 2, 3, 4}, 0.5, 2.5},
		{"unsorted input", []float64{4, 1, 3, 2}, 0.5, 2.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := percentile(tc.vals, tc.q); got != tc.want {
				t.Errorf("percentile(%v, %v) = %v, want %v", tc.vals, tc.q, got, tc.want)
			}
		})
	}
}
