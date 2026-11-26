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
	"context"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m671: the operator's ImageAvailable condition turning False is one
// image_pull_failed event with no deploy, however many passes see it; a
// recovery and a later outage is a second. The message names the cause and
// says the running instances are unaffected.
func TestAnUnavailableImageIsOneEventPerOutage(t *testing.T) {
	ctx := context.Background()
	rec, st, _ := newTestReconciler(t)
	tenant, err := st.CreateTenant(ctx, "image-check", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "ttl.sh/m671:1h", Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	const image = "ttl.sh/m671@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cause := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageNotFound, appv1alpha1.DeployReasonParam{Name: "image", Value: image})
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: tenant.ID, Generation: 3},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning, Image: image, ActiveRevision: "rev-3",
			ImageCheck: &appv1alpha1.ImageCheckStatus{Image: image, Refusals: 1},
		},
	}
	d := DesiredApp{App: row}
	setAvailable := func(status metav1.ConditionStatus, at time.Time) {
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionImageAvailable, Status: status, Reason: appv1alpha1.ReasonImageNotFound,
			Message: cause, LastTransitionTime: metav1.NewTime(at),
		})
	}
	unavailable := func() []ServiceEventFact {
		var facts []ServiceEventFact
		for _, f := range st.eventFacts {
			if f.Type == EventFactImagePullFailed {
				facts = append(facts, f)
			}
		}
		return facts
	}

	// Served, or one refusal that has not turned the condition: no event.
	setAvailable(metav1.ConditionTrue, time.Date(2026, 11, 26, 8, 0, 0, 0, time.UTC))
	rec.recordObservations(ctx, d, app, nil)
	if got := unavailable(); len(got) != 0 {
		t.Fatalf("an available image recorded %+v", got)
	}

	first := time.Date(2026, 11, 26, 14, 15, 0, 0, time.UTC)
	setAvailable(metav1.ConditionFalse, first)
	for range 3 {
		rec.recordObservations(ctx, d, app, nil)
	}
	got := unavailable()
	if len(got) != 1 {
		t.Fatalf("one outage seen three times = %d events, want 1", len(got))
	}
	fact := got[0]
	want := cause + ". Running instances are unaffected, but any action that pulls the image again, such as a restart or a scale-up, will fail until the image is reachable again"
	if fact.DeployID != "" || fact.Image != image || fact.Message != want || !fact.At.Equal(first) || !IsImageAvailabilityFact("fact:"+fact.SourceKey) {
		t.Fatalf("event = %+v, want no deploy, image %q, message %q at %s", fact, image, want, first)
	}

	// The operator recovers it, then finds a second outage.
	setAvailable(metav1.ConditionTrue, first.Add(6*time.Hour))
	rec.recordObservations(ctx, d, app, nil)
	second := first.Add(30 * time.Hour)
	setAvailable(metav1.ConditionFalse, second)
	rec.recordObservations(ctx, d, app, nil)
	rec.recordObservations(ctx, d, app, nil)
	if got := unavailable(); len(got) != 2 {
		t.Fatalf("two outages = %d events, want 2", len(got))
	}
}

func TestImageUnavailableFactNeedsAFalseConditionWithACause(t *testing.T) {
	at := metav1.NewTime(time.Date(2026, 11, 26, 14, 15, 0, 0, time.UTC))
	check := &appv1alpha1.ImageCheckStatus{Image: "nginx:1", Refusals: 2}
	for name, c := range map[string]metav1.Condition{
		"true":      {Type: appv1alpha1.ConditionImageAvailable, Status: metav1.ConditionTrue, Message: "served", LastTransitionTime: at},
		"no time":   {Type: appv1alpha1.ConditionImageAvailable, Status: metav1.ConditionFalse, Message: "gone"},
		"no cause":  {Type: appv1alpha1.ConditionImageAvailable, Status: metav1.ConditionFalse, LastTransitionTime: at},
		"elsewhere": {Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Message: "gone", LastTransitionTime: at},
	} {
		app := &appv1alpha1.App{Status: appv1alpha1.AppStatus{Image: "nginx:1", ImageCheck: check, Conditions: []metav1.Condition{c}}}
		if fact, ok := imageUnavailableFact("srv-1", app); ok {
			t.Errorf("%s: recorded %+v", name, fact)
		}
	}
	// The image named is the one the check read, not one a deploy in flight
	// already serves; a condition without the check's record is not its.
	gone := metav1.Condition{Type: appv1alpha1.ConditionImageAvailable, Status: metav1.ConditionFalse, Message: "gone", LastTransitionTime: at}
	app := &appv1alpha1.App{Status: appv1alpha1.AppStatus{Image: "nginx:2", ImageCheck: check, Conditions: []metav1.Condition{gone}}}
	if fact, ok := imageUnavailableFact("srv-1", app); !ok || fact.Image != "nginx:1" || !strings.HasPrefix(fact.Message, "gone. ") {
		t.Fatalf("fact = %+v (%v)", fact, ok)
	}
	app.Status.ImageCheck = nil
	if fact, ok := imageUnavailableFact("srv-1", app); ok {
		t.Fatalf("recorded without the check's record: %+v", fact)
	}
	if IsImageAvailabilityFact("fact:deploy:dep-1:image_pull_failed") || IsImageAvailabilityFact("fact:release:srv-1:3:image_pull_failed:1") {
		t.Fatal("a deploy's pull failure reads as the availability check's")
	}
}

// The image check's True is written on every newly served image and records
// nothing, so it wakes no projector pass; its False does (w4/m671).
func TestStatusFingerprintCountsAnUnavailableImageOnly(t *testing.T) {
	base := &appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning}
	with := func(status metav1.ConditionStatus) *appv1alpha1.AppStatus {
		s := base.DeepCopy()
		s.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionImageAvailable, Status: status, Reason: "r"}}
		return s
	}
	if statusFingerprint(base) != statusFingerprint(with(metav1.ConditionTrue)) {
		t.Error("ImageAvailable True moved the fingerprint")
	}
	if statusFingerprint(base) == statusFingerprint(with(metav1.ConditionFalse)) {
		t.Error("ImageAvailable False did not move the fingerprint")
	}
}
