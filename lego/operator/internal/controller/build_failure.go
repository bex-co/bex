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
