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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// buildingOverOutage is the 2026-10-24 beancount-cms-v2 shape (w1/m185): a new
// release builds (deploy open) while the operator observes that no pod of the
// live revision is ready.
func buildingOverOutage(serving metav1.ConditionStatus, servingReason string) *appv1alpha1.App {
	cr := &appv1alpha1.App{}
	cr.Generation = 7
	cr.Status.Phase = appv1alpha1.PhaseBuilding
	cr.Status.ActiveRevision = "gen-5"
	since := metav1.NewTime(time.Date(2026, 10, 24, 6, 31, 0, 0, time.UTC))
	cr.Status.Conditions = []metav1.Condition{
		{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "Building", ObservedGeneration: 7},
		{Type: appv1alpha1.ConditionServing, Status: serving, Reason: servingReason, ObservedGeneration: 7, LastTransitionTime: since},
	}
	return cr
}

func TestObservedServiceStateSeesAnOutageUnderABuild(t *testing.T) {
	obs := observedServiceStateFor("srv-1", buildingOverOutage(metav1.ConditionFalse, appv1alpha1.ReasonServingRevisionUnavailable), true)
	if !obs.AvailabilityObserved || obs.Availability != "unhealthy" || obs.ReasonCode != EventReasonReadinessFailed {
		t.Fatalf("observation = %+v, want unhealthy/readiness_failed: the live revision has no ready pod", obs)
	}
	if want := time.Date(2026, 10, 24, 6, 31, 0, 0, time.UTC); !obs.ReadyTransitionAt.Equal(want) {
		t.Errorf("edge time = %v, want Serving's transition %v", obs.ReadyTransitionAt, want)
	}
}

func TestObservedServiceStateBuildOverAServingReleaseIsNoOutage(t *testing.T) {
	cases := map[string]*appv1alpha1.App{
		"live revision still serving": buildingOverOutage(metav1.ConditionTrue, appv1alpha1.ReasonPriorReleaseServing),
		"serving unobserved":          buildingOverOutage(metav1.ConditionUnknown, appv1alpha1.ReasonServingUnobserved),
	}
	stale := buildingOverOutage(metav1.ConditionFalse, appv1alpha1.ReasonServingRevisionUnavailable)
	stale.Status.Conditions[1].ObservedGeneration = 6 // a verdict from before this release
	cases["stale serving verdict"] = stale
	disk := buildingOverOutage(metav1.ConditionFalse, appv1alpha1.ReasonServingRevisionUnavailable)
	disk.Spec.Disk = &appv1alpha1.DiskSpec{}
	cases["disk-backed (Recreate)"] = disk
	rolling := buildingOverOutage(metav1.ConditionFalse, appv1alpha1.ReasonServingRevisionUnavailable)
	rolling.Status.Phase = appv1alpha1.PhaseDeploying // new pods replace the active revision's
	cases["mid-rollout"] = rolling
	for name, cr := range cases {
		if obs := observedServiceStateFor("srv-1", cr, true); obs.Availability == "unhealthy" {
			t.Errorf("%s: observation = %+v, want no outage", name, obs)
		}
	}
}
