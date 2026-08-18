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
	buildOutcomeSucceeded = "succeeded"
	// The user/infra split is what makes an infrastructure-success SLO definable:
	// a tenant's broken Dockerfile must not consume the platform's error budget.
	// buildOutcomeFailed remains for genuinely unclassifiable failures (kpack).
	buildOutcomeUserFailed  = "user_failed"
	buildOutcomeInfraFailed = "infra_failed"
	buildOutcomeFailed      = "failed"
	buildOutcomeTimeout     = "timeout"
	buildOutcomeSuperseded  = "superseded"
	buildOutcomeCanceled    = "canceled"
)

var (
	buildOutcomesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bex_build_outcomes_total",
		Help: "Count of build terminal outcomes by kind (succeeded, user_failed, infra_failed, failed, timeout, superseded, canceled).",
	}, []string{"outcome"})

	// A separate workspace-labelled series rather than a workspace label on the
	// outcome counter: the correlated-failure alert must count DISTINCT
	// workspaces, and this way cardinality stays proportional to real incidents
	// rather than to the size of the estate.
	buildInfraFailuresTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bex_build_infra_failures_total",
		Help: "Builds that failed for a platform-owned reason, by workspace. Feeds the correlated-failure detector: infra causes hit many workspaces at once, tenant errors do not.",
	}, []string{"workspace"})

	buildRunSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "bex_build_run_seconds",
		Help: "Wall-clock seconds a build Job ran (status.startTime → status.completionTime), for succeeded and failed builds.",
		// Builds run minutes; cover ~15s to the 30-minute activeDeadlineSeconds.
		Buckets: []float64{15, 30, 60, 120, 300, 600, 900, 1800},
	})
)

func init() {
	ctrlmetrics.Registry.MustRegister(buildOutcomesTotal, buildInfraFailuresTotal, buildRunSeconds)
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

// recordBuildInfraFailure counts a platform-owned build failure for one
// workspace. Called only for FaultInfra: a tenant error must never appear here,
// or the correlated-failure alert would page on a tenant pushing bad code.
func recordBuildInfraFailure(workspace string) {
	// An unlabelled App (the shared bootstrap namespace admits them) is dropped
	// rather than bucketed into a shared "unknown" series. Bucketing would fold
	// N distinct tenants into one label value, and the correlated-failure alert
	// counts DISTINCT workspaces — so it would under-report during exactly the
	// fleet-wide incident it exists to catch.
	if workspace == "" {
		return
	}
	buildInfraFailuresTotal.WithLabelValues(workspace).Inc()
}
