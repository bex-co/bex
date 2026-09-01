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
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// buildFaultView is everything a terminal build failure projects outward: the
// metrics series, the App condition reason, and the sentence the tenant reads.
//
// One table rather than three switches, because the three must agree. A build
// metered as infra_failed while its condition says the tenant's code broke is
// worse than either message alone, and keeping the mapping in one place makes
// that agreement structural instead of something a test has to police.
type buildFaultView struct {
	outcome string
	reason  string
	message string
}

var buildFaultViews = map[build.Fault]buildFaultView{
	build.FaultTenant: {
		outcome: buildOutcomeUserFailed,
		reason:  appv1alpha1.ReasonBuildFailedUserError,
		message: "build failed",
	},
	build.FaultInfra: {
		outcome: buildOutcomeInfraFailed,
		reason:  appv1alpha1.ReasonBuildFailedInfrastructure,
		message: "build failed for an infrastructure reason and was retried",
	},
	build.FaultTimeout: {
		outcome: buildOutcomeTimeout,
		reason:  appv1alpha1.ReasonBuildFailed,
		message: "build exceeded its time limit",
	},
}

// unclassifiedBuildFailure is the fallback for a build whose class could not be
// determined — a kpack build, or a failure reason the operator does not model.
// It is deliberately neither user nor infra: filing an unknown under either
// would corrupt the infrastructure-success SLO in one direction.
var unclassifiedBuildFailure = buildFaultView{
	outcome: buildOutcomeFailed,
	reason:  appv1alpha1.ReasonBuildFailed,
	message: "build failed",
}

func viewForBuildFault(f build.Fault) buildFaultView {
	if v, ok := buildFaultViews[f]; ok {
		return v
	}
	return unclassifiedBuildFailure
}

// buildFailureMessage composes the sentence a tenant reads for a terminal
// build failure (w6/m123). House style is the runtime crash-loop message (the
// CrashLoopBackOff branch in app_controller.go): name the symptom, say where
// to look, add the one hint that usually explains it — never internals. The
// raw Kubernetes Job text (build namespace, Job/pod names, exit codes,
// PodFailurePolicy rule indices) stays in the operator log; it must not enter
// the string this returns.
func buildFailureMessage(v buildFaultView, obs build.Observation) string {
	switch obs.Fault {
	case build.FaultInfra:
		// A tenant cannot act on an infrastructure tail (it names registry
		// endpoints and platform components), so this class carries guidance
		// instead of output — with or without a captured tail.
		return v.message + " — this was not caused by a change in your code; trigger a new deploy to retry, and contact support if it keeps happening"
	case build.FaultTimeout:
		// A deadline reap kills the pods rather than letting one exit, so
		// there is never a failing container's tail to quote here.
		return fmt.Sprintf("%s (%d minutes) — reduce work done at build time, or enable the build cache so unchanged layers are reused",
			v.message, int(build.BuildTimeout.Minutes()))
	}
	// FaultTenant and the unclassified fallback: quote the failing step's own
	// output when the capture survived, otherwise point at the build logs.
	step := ""
	if obs.FailedStep != "" {
		step = " in the " + obs.FailedStep + " step"
	}
	if obs.Tail == "" {
		return fmt.Sprintf("%s%s — check the build logs for the failing step's output", v.message, step)
	}
	return fmt.Sprintf("%s%s:\n%s", v.message, step, obs.Tail)
}

// stampBuildRun records a failed build's execution window on the App status
// (w6/m123). Called just before r.fail so it rides the same status update as
// the Build condition — the two can never be observed apart. A window the Job
// never reported writes nothing: bex-api treats an absent window as "start
// unknown" and leaves the deploy row's started_at honestly null rather than
// fabricating one at observation time.
func stampBuildRun(app *appv1alpha1.App, obs build.Observation) {
	if obs.StartedAt.IsZero() || obs.FinishedAt.IsZero() {
		return
	}
	app.Status.BuildRun = &appv1alpha1.BuildRunStatus{
		Generation: releaseGeneration(app),
		StartedAt:  obs.StartedAt.UTC().Format(time.RFC3339),
		FinishedAt: obs.FinishedAt.UTC().Format(time.RFC3339),
	}
}
