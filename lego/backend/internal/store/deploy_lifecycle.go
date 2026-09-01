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

package store

import "slices"

// deployStatusVocabulary is Render's eleven-state deploy vocabulary in phase
// order. A deploy does not have to visit every state: the control-plane
// reconciler records only states for which it observes evidence, so a fast
// build may legitimately move from created straight to update_in_progress.
var deployStatusVocabulary = []string{
	DeployCreated,
	DeployQueued,
	DeployBuildInProgress,
	DeployBuildFailed,
	DeployPreDeployInProgress,
	DeployPreDeployFailed,
	DeployUpdateInProgress,
	DeployUpdateFailed,
	DeployLive,
	DeployDeactivated,
	DeployCanceled,
}

var openDeployStatuses = []string{
	DeployCreated,
	DeployQueued,
	DeployBuildInProgress,
	DeployPreDeployInProgress,
	DeployUpdateInProgress,
}

// DeployStatuses returns a copy of the complete stored status vocabulary.
// Callers may use it to validate surface/filter parity without mutating the
// store's transition model.
func DeployStatuses() []string { return slices.Clone(deployStatusVocabulary) }

// IsOpenDeployStatus reports whether status may still advance or be canceled.
func IsOpenDeployStatus(status string) bool { return slices.Contains(openDeployStatuses, status) }

// DeployStatusStartsExecution reports whether reaching status means the deploy's
// work has actually begun. A queued deploy is waiting behind another build and
// has dispatched nothing; canceled/deactivated are exits that can be reached
// without ever executing. Shared with the build_started event fact so the
// timeline cannot disagree with the deploy row about when a build began
// (w6/035). Note it says nothing about WHEN the work began: a terminal failure
// reached by a phase skip executed at some earlier, unobserved time — see
// DeployStatusStampsDispatch for the stamping half (w6/m123).
func DeployStatusStartsExecution(status string) bool {
	return status != DeployQueued && status != DeployCanceled && status != DeployDeactivated
}

// DeployFailureStatus reports whether status is a terminal failure. These can
// be reached by a forward skip straight from queued (the backend samples
// Kubernetes state; a build can fail entirely between two passes), so the
// transition's own clock is not evidence of when their work began.
func DeployFailureStatus(status string) bool {
	switch status {
	case DeployBuildFailed, DeployPreDeployFailed, DeployUpdateFailed:
		return true
	}
	return false
}

// DeployStatusStampsDispatch reports whether TransitionDeploy may stamp
// started_at from its own clock on reaching status: entering an in-progress
// (or live) status IS the dispatch moment being observed, while a terminal
// failure arrived at by a skip is not — stamping it collapsed a 68-second
// failed build into a one-microsecond duration (w6/m123). Failure closes
// stamp only observed evidence (the operator's recorded build window), or
// honestly nothing.
func DeployStatusStampsDispatch(status string) bool {
	return DeployStatusStartsExecution(status) && !DeployFailureStatus(status)
}

// IsTerminalDeployStatus reports whether status is a known terminal state.
func IsTerminalDeployStatus(status string) bool {
	return slices.Contains(deployStatusVocabulary, status) && !IsOpenDeployStatus(status)
}

// CanTransitionDeploy is the evidence-backed transition table. It permits
// forward phase skips because the backend samples Kubernetes state rather
// than receiving every operator edge, but rejects phase regressions and every
// mutation out of a terminal state. live -> deactivated is the sole exception:
// a newer live deploy atomically makes the prior live revision historical.
func CanTransitionDeploy(from, to string) bool {
	if from == to {
		return false
	}
	switch to {
	case DeployQueued:
		return from == DeployCreated
	case DeployBuildInProgress:
		return from == DeployCreated || from == DeployQueued
	case DeployBuildFailed:
		return from == DeployCreated || from == DeployQueued || from == DeployBuildInProgress
	case DeployPreDeployInProgress:
		return from == DeployCreated || from == DeployQueued || from == DeployBuildInProgress
	case DeployPreDeployFailed:
		return from == DeployCreated || from == DeployQueued || from == DeployBuildInProgress || from == DeployPreDeployInProgress
	case DeployUpdateInProgress:
		return IsOpenDeployStatus(from)
	case DeployUpdateFailed, DeployLive, DeployCanceled:
		return IsOpenDeployStatus(from)
	case DeployDeactivated:
		return from == DeployLive
	default:
		return false
	}
}
