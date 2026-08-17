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
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Build-plane SLIs (ADR060 §D5, the subset w2/m72 needs). The operator emitted
// no build metrics before this — diagnosis was kubectl archaeology. Two series:
// an outcome counter whose whole point is to separate a supersede (a newer push
// coalesced into the pending slot — expected, cheap) from a user Cancel (human
// intent) from a real build failure/timeout, and a run-duration histogram. If
// outcome="canceled" ever climbs outside the explicit Cancel verb after w2/m72
// ships, a supersede path was missed — that is the tripwire this exists for.
const (
	buildOutcomeSucceeded  = "succeeded"
	buildOutcomeFailed     = "failed"
	buildOutcomeTimeout    = "timeout"
	buildOutcomeSuperseded = "superseded"
	buildOutcomeCanceled   = "canceled"
)

var (
	buildOutcomesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bex_build_outcomes_total",
		Help: "Count of build terminal outcomes by kind (succeeded, failed, timeout, superseded, canceled).",
	}, []string{"outcome"})

	buildRunSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "bex_build_run_seconds",
		Help: "Wall-clock seconds a build Job ran (status.startTime → status.completionTime), for succeeded and failed builds.",
		// Builds run minutes; cover ~15s to the 30-minute activeDeadlineSeconds.
		Buckets: []float64{15, 30, 60, 120, 300, 600, 900, 1800},
	})
)

func init() {
	ctrlmetrics.Registry.MustRegister(buildOutcomesTotal, buildRunSeconds)
}

// recordBuildOutcome increments the outcome counter. Called once per terminal
// build event; superseded and canceled are distinct series by construction.
func recordBuildOutcome(outcome string) {
	buildOutcomesTotal.WithLabelValues(outcome).Inc()
}

// recordBuildRunSeconds records a finished build's run duration, computed by the
// build plane from the Job's own status timestamps (never an operator-side clock,
// which a manager restart would reset — the wart ADR060 §D1 removes). A
// non-positive value (unknown timestamps, or the kpack path, which does not
// surface them) is skipped rather than metered as a zero.
func recordBuildRunSeconds(seconds float64) {
	if seconds > 0 {
		buildRunSeconds.Observe(seconds)
	}
}
