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

	"github.com/bex-co/bex/lego/operator/internal/build"
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

	// QUEUE time, not run time (docs/ADR060 D5). The split is the point: a build
	// that waits for a node is the PLATFORM failing to supply capacity, while the
	// time it then spends compiling is mostly the tenant's own code. Conflating
	// them is what misattributed the 2026-08-11 incident, where a build sat
	// Pending 22 minutes and the deploy was failed by the control-plane gate while
	// the build itself was healthy — and, with no series at all, why every
	// incident in this class has presented as silence.
	buildQueueSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "bex_build_queue_seconds",
		Help: "Seconds a build waited between admission (Job creation) and the scheduler placing its pod on a node.",
		// The buckets have to resolve the two regimes this series exists to tell
		// apart, so they are not a copied default:
		//   • warm node — a free slot on an already-running build node places the
		//     pod in single-digit seconds (5/15/30 separate "instant" from "a
		//     moment", which is where the prewarm DaemonSet's effect shows up);
		//   • cold scale-from-zero — Hetzner provisioning + kubeadm join + Cilium
		//     readiness is minutes, never seconds (60/120/300 is the healthy band).
		// 600 is the "something is wrong" line, and 1200 exists so a build that
		// queued past the 18-minute deploy gate lands in a countable bucket rather
		// than disappearing into +Inf — that case is the one worth alerting on.
		Buckets: []float64{5, 15, 30, 60, 120, 300, 600, 1200},
	})

	// The live admission picture. All three are SET from a full recount of the
	// build namespace (build.CountBuilds), never incremented/decremented: a gauge
	// driven by edges strands above zero the first time the process restarts
	// mid-build or misses an event, and a stranded capacity gauge is worse than
	// none — it pages forever about a build that finished hours ago.
	buildsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bex_builds_active",
		Help: "Builds dispatched and not yet finished, cluster-wide (queued ones included). The quantity BEX_MAX_ACTIVE_BUILDS caps.",
	})
	buildsQueued = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bex_builds_queued",
		Help: "Builds whose pod the scheduler has not yet placed on a node — waiting for capacity, doing no work.",
	})
	// A histogram of FINISHED builds cannot answer "is a build stuck right now?",
	// and that is the question the deploy gate makes urgent. This gauge can, at
	// one series: it is the age of the longest-waiting queued build.
	buildQueueOldestSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bex_build_queue_oldest_seconds",
		Help: "Age of the longest-currently-queued build, in seconds (0 when nothing is queued).",
	})

	// The push phase is the one part of a build the platform owns end to end
	// (docs/ADR060 D4): skopeo, the platform's own credential, the platform's own
	// registry. Its duration and error rate are therefore platform signals even
	// when the build as a whole failed for a tenant reason. Deferred here from
	// w7/m82 t004 because both are read off the per-container states this file's
	// caller already fetches — emitting them there would have meant a second pod
	// read path or an always-zero counter.
	buildPushSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "bex_build_push_seconds",
		Help: "Seconds the skopeo push phase ran on a build's final attempt.",
		// A push moves one OCI archive to an in-cluster registry: sub-second for a
		// small image, minutes for a multi-GB one against a slow disk. The top
		// bucket is deliberately far below the 30-minute build deadline — a push
		// approaching 600s is already the incident, not the tail.
		Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600},
	})
	buildPushErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bex_build_push_errors_total",
		Help: "Build push phases that exited non-zero. A platform-owned failure: the credential, the archive, and the registry are all ours.",
	})
	// Labelled by the CLOSED reason set in internal/build (disruption, oom,
	// transient, tenant, unclassified) so cardinality is fixed. reason="disruption"
	// climbing is the podFailurePolicy absorbing node churn as designed;
	// reason="tenant" climbing at all means a phase stopped classifying, because
	// the policy fails those Jobs outright instead of retrying them.
	buildRetriesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bex_build_retries_total",
		Help: "Build attempts that were superseded by a retry, by classified reason.",
	}, []string{"reason"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(
		buildOutcomesTotal, buildInfraFailuresTotal, buildRunSeconds,
		buildQueueSeconds, buildsActive, buildsQueued, buildQueueOldestSeconds,
		buildPushSeconds, buildPushErrorsTotal, buildRetriesTotal,
	)
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

// recordBuildSignals meters the pod-derived series for one finished build
// (docs/ADR060 D5). Called under the same once-per-build gate as the outcome
// counter, so a level-triggered re-reconcile of an already-finished build cannot
// re-observe the same queue wait.
//
// UNMEASURED values are skipped rather than metered: a kpack build has no Job
// pods at all, and observing its absent queue wait as 0s would drag the
// platform's capacity percentile toward zero with data that was never taken. A
// genuinely-zero measurement is a different thing and IS recorded — see
// build.Signals.QueueMeasured.
func recordBuildSignals(sig build.Signals) {
	if sig.QueueMeasured {
		buildQueueSeconds.Observe(sig.QueueSeconds)
	}
	if sig.PushSeconds > 0 {
		buildPushSeconds.Observe(sig.PushSeconds)
	}
	if sig.PushFailed {
		buildPushErrorsTotal.Inc()
	}
	for _, reason := range sig.Retries {
		buildRetriesTotal.WithLabelValues(reason).Inc()
	}
}

// publishBuildCensus republishes the live admission gauges from a full recount.
func publishBuildCensus(c build.Census) {
	buildsActive.Set(float64(c.Active))
	buildsQueued.Set(float64(c.Queued))
	buildQueueOldestSeconds.Set(c.OldestQueued.Seconds())
}
