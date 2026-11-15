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

// PreDeployStepCommand is the command a release's pre-deploy step runs: the
// one-time spec.initialDeployHook until a release has run it to success, then
// spec.preDeployCommand (w8/m63). The roles stay separate even when the two
// commands are equal; initial reports which one was selected. The operator
// runs it and bex-api reads it to tell a release still awaiting its step from
// one whose rollout has begun (w4/670). Only services that roll a Deployment
// run the step: a cron job, static site or build source selects none.
func (a *App) PreDeployStepCommand() (command string, initial bool) {
	switch a.Spec.Type {
	case TypeCronJob, TypeStaticSite, TypeBuildSource:
		return "", false
	}
	if strings.TrimSpace(a.Spec.InitialDeployHook) != "" && !a.Status.InitialDeployHookCompleted {
		return a.Spec.InitialDeployHook, true
	}
	return a.Spec.PreDeployCommand, false
}
