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

package v1alpha1

import "strings"

// AnnotationInitialDeployHookRan marks an App whose initialDeployHook already
// ran to success: bex-api set it for services created before the operator
// owned completion (w2/m45), and still sets it once
// status.initialDeployHookCompleted is true. Either marker means the hook is
// never selected again.
const AnnotationInitialDeployHookRan = "bex.co/initial-deploy-hook-ran"

// LabelPreDeployStep says which of the plane's two commands a pre-deploy Job
// and its pod run. The operator stamps it (w33/m435); bex-api reads it to
// attribute the step's logs. The spelling lives here so the stamp and the
// readers can never drift apart.
const LabelPreDeployStep = "app.bex.co/predeploy-step"

// PreDeployStepInitialDeployHook is the LabelPreDeployStep value for the
// service's one-time initialDeployHook, run after the first release is live.
const PreDeployStepInitialDeployHook = "initial-deploy-hook"

// RollsDeployment reports a service that runs as a Deployment: web and private
// services and background workers. A cron job, static site or build source has
// none, and runs neither a pre-deploy step nor an initial deploy hook.
func (a *App) RollsDeployment() bool {
	switch a.Spec.Type {
	case TypeCronJob, TypeStaticSite, TypeBuildSource:
		return false
	}
	return true
}

// PreDeployStepCommand is the command a release's pre-deploy step runs before
// its rollout: the release's spec.preDeployCommand, on every release, the
// first included (w33/m435). The operator runs it and bex-api reads it to tell
// a release still awaiting its step from one whose rollout has begun (w4/670).
func (a *App) PreDeployStepCommand() string {
	if !a.RollsDeployment() {
		return ""
	}
	// A Restart or rollback runs its selected release's command (w4/m433).
	return a.RuntimeSpec().PreDeployCommand
}

// OwedInitialDeployHook is spec.initialDeployHook while the service still owes
// its one run, "" once it does not: Render runs it once, after the service's
// first successful deploy. The operator starts it only after a release is
// live; a failed run leaves it owed (w33/m435, docs/ADR004-app-deployment.md
// § Initial deploy hook).
func (a *App) OwedInitialDeployHook() string {
	if !a.RollsDeployment() || strings.TrimSpace(a.Spec.InitialDeployHook) == "" ||
		a.Status.InitialDeployHookCompleted || a.Annotations[AnnotationInitialDeployHookRan] != "" {
		return ""
	}
	return a.Spec.InitialDeployHook
}
