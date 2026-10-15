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

import (
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// parkedApp is an App auto-hibernation parked at generation 3 after the
// operator recorded releaseGeneration/activeRevision.
func parkedApp(releaseGen int64, activeRevision string) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Generation = 3
	app.Status.Phase = appv1alpha1.PhaseHibernated
	app.Status.ObservedGeneration = 3
	app.Status.ReleaseGeneration = releaseGen
	app.Status.ActiveRevision = activeRevision
	return app
}

// A release that served and was then parked by auto-hibernation before the
// history loop observed Running settles Live, not in progress until the
// health-gate timeout fails it (w8/106).
func TestObservedDeployStatusServedThenParkedIsLive(t *testing.T) {
	open := Deploy{Generation: 3, Status: DeployUpdateInProgress}
	for _, timedOut := range []bool{false, true} {
		if got := observedDeployStatus(open, parkedApp(3, "rev-3"), timedOut); got != DeployLive {
			t.Errorf("timedOut=%v: status = %q, want live", timedOut, got)
		}
	}
}

// Parks that do not prove this release served keep their existing outcomes.
func TestObservedDeployStatusParkWithoutServedProofIsNotLive(t *testing.T) {
	open := Deploy{Generation: 3, Status: DeployUpdateInProgress}
	for name, app := range map[string]*appv1alpha1.App{
		"prior revision serving": parkedApp(3, "rev-2"),
		"older release":          parkedApp(2, "rev-2"),
		"legacy no release gen":  parkedApp(0, ""),
		"user suspend": func() *appv1alpha1.App {
			a := parkedApp(3, "rev-3")
			a.Spec.Suspended = true
			return a
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			if got := observedDeployStatus(open, app, false); got == DeployLive {
				t.Errorf("status = live, want the row left open")
			}
			if got := observedDeployStatus(open, app, true); got == DeployLive {
				t.Errorf("timed out: status = live, want a non-live verdict")
			}
		})
	}
}
