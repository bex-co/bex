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
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A refusal names Render's cause: "Image not found (404 error)", or an
// "Authentication error" that is the service's credential when the check
// carried it.
func TestImageRefusalReasonNamesRendersCause(t *testing.T) {
	const image = "ghcr.io/acme/api@sha256:" + sha64
	for _, tc := range []struct {
		status       int
		credentialed bool
		reason, code string
	}{
		{http.StatusNotFound, false, appv1alpha1.ReasonImageNotFound, appv1alpha1.DeployReasonImageNotFound},
		{http.StatusNotFound, true, appv1alpha1.ReasonImageNotFound, appv1alpha1.DeployReasonImageNotFound},
		{http.StatusUnauthorized, true, appv1alpha1.ReasonImageCredentialRefused, appv1alpha1.DeployReasonImageCredential},
		{http.StatusForbidden, true, appv1alpha1.ReasonImageCredentialRefused, appv1alpha1.DeployReasonImageCredential},
		{http.StatusUnauthorized, false, appv1alpha1.ReasonImageAccessRefused, appv1alpha1.DeployReasonImageAccess},
		{http.StatusForbidden, false, appv1alpha1.ReasonImageAccessRefused, appv1alpha1.DeployReasonImageAccess},
	} {
		reason, msg := imageRefusalReason(image, tc.status, tc.credentialed)
		code, _, ok := appv1alpha1.ParseDeployReason(msg)
		if reason != tc.reason || !ok || code != tc.code || !strings.Contains(msg, image) {
			t.Errorf("status %d credentialed=%v: %s %q (code %q)", tc.status, tc.credentialed, reason, msg, code)
		}
	}
	for _, status := range []int{http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusTooManyRequests, http.StatusBadGateway} {
		if imageGoneStatus(status) {
			t.Errorf("status %d counts as the image gone", status)
		}
	}
}

// The second refusal in a row turns the condition False; one served answer
// in between starts the count over; while it is False a further refusal keeps
// its transition.
func TestImageCheckCountsRefusalsInARow(t *testing.T) {
	const image = "nginx@sha256:" + sha64
	at := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	status := &appv1alpha1.AppStatus{Image: image}
	unavailable := func() bool { return imageUnavailable(status) }

	recordImageRefused(status, image, at, appv1alpha1.ReasonImageNotFound, "gone")
	if unavailable() || status.ImageCheck.Refusals != 1 {
		t.Fatalf("one refusal: %+v %+v", status.ImageCheck, status.Conditions)
	}
	recordImageServed(status, image, at.Add(time.Minute))
	if unavailable() || status.ImageCheck.Refusals != 0 {
		t.Fatalf("served after one refusal: %+v %+v", status.ImageCheck, status.Conditions)
	}
	recordImageRefused(status, image, at.Add(2*time.Minute), appv1alpha1.ReasonImageNotFound, "gone")
	recordImageRefused(status, image, at.Add(3*time.Minute), appv1alpha1.ReasonImageNotFound, "gone")
	if !unavailable() || status.Conditions[0].LastTransitionTime.Time != at.Add(3*time.Minute) {
		t.Fatalf("second refusal in a row: %+v", status.Conditions)
	}
	// While it is False, the next refusal keeps the transition, and a refused
	// credential replaces the cause it names.
	recordImageRefused(status, image, at.Add(4*time.Minute), appv1alpha1.ReasonImageAccessRefused, "refused")
	if c := status.Conditions[0]; c.LastTransitionTime.Time != at.Add(3*time.Minute) || c.Reason != appv1alpha1.ReasonImageAccessRefused || status.ImageCheck.Refusals != 3 {
		t.Fatalf("refusal during the outage: %+v %+v", c, status.ImageCheck)
	}
	recordImageServed(status, image, at.Add(5*time.Minute))
	if c := status.Conditions[0]; unavailable() || c.LastTransitionTime.Time != at.Add(5*time.Minute) {
		t.Fatalf("recovery: %+v", c)
	}
}

// Each wait is spread over ±10%, the same for an App and answer whenever the
// manager computes it; a first refusal is confirmed within minutes, and an
// outage is checked on the interval.
func TestNextImageCheckSpreadsTheInterval(t *testing.T) {
	r := &AppReconciler{ImageCheckInterval: DefaultImageCheckInterval}
	at := metav1.NewTime(time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC))
	if due := r.nextImageCheck("uid", &appv1alpha1.AppStatus{}); !due.IsZero() {
		t.Fatalf("an image never checked is due at %v, want at once", due)
	}
	seen := map[time.Duration]bool{}
	for i := range 50 {
		status := &appv1alpha1.AppStatus{ImageCheck: &appv1alpha1.ImageCheckStatus{Image: "i", CheckedAt: at}}
		uid := types.UID(fmt.Sprintf("app-%d", i))
		wait := r.nextImageCheck(uid, status).Sub(at.Time)
		if wait < DefaultImageCheckInterval*9/10 || wait >= DefaultImageCheckInterval*11/10 {
			t.Fatalf("wait %v outside ±10%% of %v", wait, DefaultImageCheckInterval)
		}
		if again := r.nextImageCheck(uid, status).Sub(at.Time); again != wait {
			t.Fatalf("wait moved from %v to %v", wait, again)
		}
		seen[wait] = true

		status.ImageCheck.Refusals = 1
		if wait := r.nextImageCheck(uid, status).Sub(at.Time); wait >= imageCheckConfirm*11/10 {
			t.Fatalf("a first refusal waits %v", wait)
		}
		status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionImageAvailable, Status: metav1.ConditionFalse}}
		if wait := r.nextImageCheck(uid, status).Sub(at.Time); wait < DefaultImageCheckInterval*9/10 {
			t.Fatalf("an outage is checked again after %v", wait)
		}
	}
	if len(seen) < 25 {
		t.Fatalf("50 services share %d waits", len(seen))
	}
	// A short interval (the dev walk's) bounds the confirmation too.
	r.ImageCheckInterval = 2 * time.Minute
	status := &appv1alpha1.AppStatus{ImageCheck: &appv1alpha1.ImageCheckStatus{Image: "i", CheckedAt: at, Refusals: 1}}
	if wait := r.nextImageCheck("uid", status).Sub(at.Time); wait >= 2*time.Minute*11/10 {
		t.Fatalf("confirmation waits %v under a 2m interval", wait)
	}
}

// Checks are spaced apart, and a canceled wait gives up.
func TestImageCheckPacerSpacesChecks(t *testing.T) {
	p := imageCheckPacer{spacing: 50 * time.Millisecond}
	if err := p.wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := p.wait(t.Context()); err != nil || time.Since(start) < 40*time.Millisecond {
		t.Fatalf("second check after %v (err %v), want the spacing", time.Since(start), err)
	}
	p.spacing = time.Hour
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := p.wait(ctx); err == nil {
		t.Fatal("a canceled wait went on")
	}
}

// The check wakes on what decides it, never on its own write.
func TestImageCheckPredicate(t *testing.T) {
	base := &appv1alpha1.App{Status: appv1alpha1.AppStatus{Image: "nginx:1", Phase: appv1alpha1.PhaseRunning}}
	update := func(mutate func(*appv1alpha1.App)) bool {
		next := base.DeepCopy()
		mutate(next)
		return imageCheckPredicate{}.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: next})
	}
	if !update(func(a *appv1alpha1.App) { a.Status.Image = "nginx:2" }) ||
		!update(func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseHibernated }) ||
		!update(func(a *appv1alpha1.App) { a.Spec.Suspended = true }) {
		t.Fatal("a change that decides the check did not wake it")
	}
	if update(func(a *appv1alpha1.App) {
		a.Status.ImageCheck = &appv1alpha1.ImageCheckStatus{Image: "nginx:1"}
		a.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionImageAvailable, Status: metav1.ConditionTrue}}
	}) {
		t.Fatal("the check's own write woke it")
	}
}

// Only a running web service, private service or background worker on a
// prebuilt image outside bex's registry is checked.
func TestImageCheckTarget(t *testing.T) {
	r := &AppReconciler{Mode: ModeKubernetes, Registry: "zot.bex-registry.svc:5000"}
	app := func(mutate func(*appv1alpha1.App)) *appv1alpha1.App {
		a := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Image: "nginx:1"},
			Status: appv1alpha1.AppStatus{Image: "nginx@sha256:" + sha64, Phase: appv1alpha1.PhaseRunning}}
		mutate(a)
		return a
	}
	for name, mutate := range map[string]func(*appv1alpha1.App){
		"web":     func(*appv1alpha1.App) {},
		"legacy":  func(a *appv1alpha1.App) { a.Spec.Type = "" },
		"private": func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypePrivateService },
		"worker": func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeBackgroundWorker
			a.Status.Image = "ghcr.io/acme/w:v1"
		},
		"sleeping":  func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseHibernated },
		"hub alias": func(a *appv1alpha1.App) { a.Status.Image = "docker.io/library/nginx:1.27" },
	} {
		a := app(mutate)
		if image, holder, err := r.imageCheckTarget(t.Context(), a); image == "" || holder != a || err != nil {
			t.Errorf("%s: not checked (%q, %v)", name, image, err)
		}
	}
	for name, mutate := range map[string]func(*appv1alpha1.App){
		"suspended": func(a *appv1alpha1.App) { a.Spec.Suspended = true; a.Status.Phase = appv1alpha1.PhaseHibernated },
		"cron":      func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeCronJob },
		"static":    func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeStaticSite },
		"deploying": func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseDeploying },
		"failed":    func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseFailed },
		"no image":  func(a *appv1alpha1.App) { a.Status.Image = "" },
		"bex build": func(a *appv1alpha1.App) { a.Status.Image = "zot.bex-registry.svc:5000/tea-x/api@sha256:" + sha64 },
		"git built": func(a *appv1alpha1.App) { a.Spec.Image = "" },
	} {
		if image, _, err := r.imageCheckTarget(t.Context(), app(mutate)); image != "" || err != nil {
			t.Errorf("%s: checked %q (%v)", name, image, err)
		}
	}
}

const sha64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
