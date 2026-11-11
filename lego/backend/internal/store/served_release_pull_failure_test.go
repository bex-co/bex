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
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// served_release_pull_failure_test.go is w8/m156 t005. A wake, a restart or a
// reschedule of the live release opens no deploy row, so a pull failure there
// was recorded nowhere: no event, no alert, while the service could not start.

func servedPullFailureApp(servingGeneration, failedGeneration int64, reason string, at time.Time) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Name, app.Namespace = "web", "ws"
	app.Status.Image = "zot.bex-registry.svc:5000/ws/web:gen-8@sha256:" + strings.Repeat("a", 64)
	app.Status.ImageDigest = &appv1alpha1.ImageDigestStatus{Generation: servingGeneration, Digest: "sha256:" + strings.Repeat("a", 64)}
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: reason,
		Message:            "image pull failed: the build this release runs on (ws/web:gen-8) is no longer stored",
		ObservedGeneration: failedGeneration, LastTransitionTime: metav1.NewTime(at),
	}}
	return app
}

func TestServedReleasePullFailureIsRecordedOncePerIncident(t *testing.T) {
	at := time.Date(2026, 11, 11, 15, 52, 0, 0, time.UTC)
	app := servedPullFailureApp(8, 8, "ImagePullBackOff", at)

	fact, ok := servedReleasePullFailure("srv-1", app)
	if !ok {
		t.Fatal("the served release's pull failure produced no fact")
	}
	if fact.Type != EventFactImagePullFailed || fact.ReasonCode != EventReasonImagePullBackoff || fact.AppID != "srv-1" ||
		!fact.At.Equal(at) || fact.Image != app.Status.Image || fact.DeployID != "" {
		t.Errorf("fact = %+v", fact)
	}
	// Every later pass sees the same condition and writes the same key, which
	// the store's unique source key turns into a no-op.
	again, _ := servedReleasePullFailure("srv-1", app)
	if again.SourceKey != fact.SourceKey {
		t.Errorf("source key moved between passes: %q then %q", fact.SourceKey, again.SourceKey)
	}
	// The release failing to pull again later is a new incident.
	later := servedPullFailureApp(8, 8, "ImagePullBackOff", at.Add(3*time.Hour))
	if next, ok := servedReleasePullFailure("srv-1", later); !ok || next.SourceKey == fact.SourceKey {
		t.Errorf("a later incident reused the key %q (ok %v)", next.SourceKey, ok)
	}
}

func TestServedReleasePullFailureIgnoresWhatADeployRowRecords(t *testing.T) {
	at := time.Date(2026, 11, 11, 15, 52, 0, 0, time.UTC)
	for name, app := range map[string]*appv1alpha1.App{
		// A newer release failed to pull over the served one: its deploy row
		// recorded the failure when it closed.
		"a newer release failed over the served one": servedPullFailureApp(8, 9, "ImagePullBackOff", at),
		// The served release failed for another reason: not an image pull.
		"a crash, not a pull": servedPullFailureApp(8, 8, "CrashLoopBackOff", at),
		// Nothing has served yet.
		"nothing serving": func() *appv1alpha1.App {
			a := servedPullFailureApp(8, 8, "ImagePullBackOff", at)
			a.Status.ImageDigest = nil
			return a
		}(),
		// A verdict that passed.
		"a rollout that succeeded": func() *appv1alpha1.App {
			a := servedPullFailureApp(8, 8, "ImagePullBackOff", at)
			a.Status.Conditions[0].Status = metav1.ConditionTrue
			return a
		}(),
		"no rollout verdict": func() *appv1alpha1.App {
			a := servedPullFailureApp(8, 8, "ImagePullBackOff", at)
			a.Status.Conditions = nil
			return a
		}(),
	} {
		if fact, ok := servedReleasePullFailure("srv-1", app); ok {
			t.Errorf("%s: recorded %+v", name, fact)
		}
	}
}
