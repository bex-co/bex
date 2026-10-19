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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// firstBuildApp is a public repo-backed web service just created: nothing has
// built, so its first reconcile dispatches a build and halts on it.
func firstBuildApp(tier string) *appv1alpha1.App {
	app := hibernatingApp("tea-abc123")
	app.Annotations = nil
	app.UID = "uid-first"
	app.Spec.Image = ""
	app.Spec.Repo = "https://github.com/acme/first"
	app.Spec.Branch = "main"
	app.Spec.Tier = tier
	return app
}

// TestAFirstBuildRoutesItsHostToTheActivator is w4/m235: the first build halts
// the reconcile before the release writes its Ingress, so the host the
// dashboard shows from creation answered Traefik's raw 404 for the whole build.
// It now routes to the activator from the first pass, on every plan.
func TestAFirstBuildRoutesItsHostToTheActivator(t *testing.T) {
	for _, tier := range []string{"free", "starter"} {
		t.Run(tier, func(t *testing.T) {
			app := firstBuildApp(tier)
			r, cl, nn := lifecycleFixture(t, app)
			r.Registry, r.BuildNamespace, r.BuildClient = "zot.example", "builds", cl
			reconcileTwice(t, r, nn)
			live := liveApp(t, cl, nn)
			if live.Status.Phase != appv1alpha1.PhaseBuilding || live.Status.ActiveRevision != "" {
				t.Fatalf("setup: phase %q active %q, want a first build in progress", live.Status.Phase, live.Status.ActiveRevision)
			}
			if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
				t.Fatalf("first-build backend = %q, want the activator alias", got)
			}
		})
	}
}

// TestAPaidFirstReleaseWaitsOnTheActivatorUntilItServes: a paid service is
// never sleep-eligible, so its first release used to route to its own Service
// with no ready endpoint, where Traefik answers "no available server". It
// waits on the activator until a pod is ready, then takes its own Service.
func TestAPaidFirstReleaseWaitsOnTheActivatorUntilItServes(t *testing.T) {
	app := hibernatingApp("tea-abc123")
	app.Annotations = nil
	app.Spec.Tier = "starter"
	r, cl, nn := lifecycleFixture(t, app)
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("first release starting: backend %q, want the activator alias", got)
	}
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	if live := liveApp(t, cl, nn); live.Status.ActiveRevision == "" {
		t.Fatal("setup: the first release never became active")
	}
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("serving: backend %q, want the App's own Service", got)
	}
}

// TestAnImageFirstReleaseHeldAtItsPreDeployGateIsRouted: an image-backed
// first release halts at its pre-deploy command before the release writes its
// Ingress, so a migration-length command used to serve Traefik's 404 for its
// whole run. It routes to the activator from the first pass too.
func TestAnImageFirstReleaseHeldAtItsPreDeployGateIsRouted(t *testing.T) {
	app := hibernatingApp("tea-abc123")
	app.Annotations = nil
	app.UID = "uid-predeploy"
	app.Spec.Tier = "starter"
	app.Spec.PreDeployCommand = "./migrate"
	r, cl, nn := lifecycleFixture(t, app)
	reconcileTwice(t, r, nn)
	if live := liveApp(t, cl, nn); live.Status.ActiveRevision != "" {
		t.Fatalf("setup: active %q, want the first release held", live.Status.ActiveRevision)
	}
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("pre-deploy gate backend = %q, want the activator alias", got)
	}
}
