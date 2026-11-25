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

	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m633: a rolling App Deployment keeps every old instance until a new one
// is available, as Render keeps every original instance serving until its
// replacement is healthy (ADR004 § A rollout keeps every old instance).

// apiServerDefaultStrategy is what the API server stores for a Deployment
// written with no strategy, which every unheld diskless App Deployment carried
// before w6/m633.
func apiServerDefaultStrategy() appsv1.DeploymentStrategy {
	quarter := intstr.FromString("25%")
	return appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: &quarter, MaxUnavailable: new(quarter)}}
}

// rolloutFenceposts resolves a rolling strategy for n replicas as the
// Deployment controller does (deploymentutil.ResolveFenceposts): the surge
// rounds up, the unavailable count down, and when both come to 0 the
// controller makes one instance unavailable rather than stall.
func rolloutFenceposts(t interface {
	Helper()
	Fatalf(string, ...any)
}, s appsv1.DeploymentStrategy, n int32) (surge, unavailable int) {
	t.Helper()
	if s.Type != appsv1.RollingUpdateDeploymentStrategyType || s.RollingUpdate == nil {
		t.Fatalf("strategy %+v is not a rolling update", s)
	}
	surge, err := intstr.GetScaledValueFromIntOrPercent(s.RollingUpdate.MaxSurge, int(n), true)
	if err != nil {
		t.Fatalf("maxSurge: %v", err)
	}
	unavailable, err = intstr.GetScaledValueFromIntOrPercent(s.RollingUpdate.MaxUnavailable, int(n), false)
	if err != nil {
		t.Fatalf("maxUnavailable: %v", err)
	}
	if surge == 0 && unavailable == 0 {
		unavailable = 1
	}
	return surge, unavailable
}

// Every instance count the CRD admits, for each service type that rolls,
// projects a strategy under which a deploy, a restart or a release that never
// becomes healthy takes no old instance out of service, and still starts at
// least one new one.
func TestRollingAppDeploymentKeepsEveryOldInstance(t *testing.T) {
	for _, svc := range []struct {
		name   string
		typ    string
		worker bool
	}{
		{"web", appv1alpha1.TypeWebService, false},
		{"private", appv1alpha1.TypePrivateService, false},
		{"worker", appv1alpha1.TypeBackgroundWorker, true},
	} {
		for n := int32(1); n <= appv1alpha1.MaxReplicas; n++ {
			p := webParams()
			p.replicas, p.worker = n, svc.worker
			dep := project(projectionApp(func(a *appv1alpha1.App) { a.Spec.Type = svc.typ }), p)
			if *dep.Spec.Replicas != n {
				t.Fatalf("%s at %d: replicas %d", svc.name, n, *dep.Spec.Replicas)
			}
			if !apiequality.Semantic.DeepEqual(dep.Spec.Strategy, defaultRollingUpdateStrategy()) {
				t.Fatalf("%s at %d: strategy %+v, want %+v", svc.name, n, dep.Spec.Strategy, defaultRollingUpdateStrategy())
			}
			surge, unavailable := rolloutFenceposts(t, dep.Spec.Strategy, n)
			if unavailable != 0 {
				t.Fatalf("%s at %d instances: a rollout may take %d old instances out before a new one is available, want 0", svc.name, n, unavailable)
			}
			if want := int((n + 3) / 4); surge != want {
				t.Fatalf("%s at %d instances: surge %d, want a quarter rounded up (%d)", svc.name, n, surge, want)
			}
		}
	}

	// What w6/m633 replaced: the API server's default dropped a quarter of the
	// instances from 4 up, and nothing below.
	for n, want := range map[int32]int{1: 0, 3: 0, 4: 1, 7: 1, 8: 2, 100: 25} {
		if _, unavailable := rolloutFenceposts(t, apiServerDefaultStrategy(), n); unavailable != want {
			t.Errorf("the API server's default at %d instances: %d unavailable, want %d", n, unavailable, want)
		}
	}
}

// A Deployment stored before w6/m633 carries the API server's default. The
// next pass writes the new strategy and leaves the pod template, and so its
// pod-template hash, as it was: the upgrade rolls nothing.
func TestRollingStrategyIsAdoptedWithoutARoll(t *testing.T) {
	for _, n := range []int32{1, 4, 100} {
		p := webParams()
		p.replicas = n
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/healthz" })
		stored := project(app, p)
		stored.Spec.Strategy = apiServerDefaultStrategy()

		upgraded := stored.DeepCopy()
		applyDeploymentSpec(upgraded, app, p)
		if !apiequality.Semantic.DeepEqual(upgraded.Spec.Strategy, defaultRollingUpdateStrategy()) {
			t.Fatalf("%d instances: strategy after the upgrade %+v, want %+v", n, upgraded.Spec.Strategy, defaultRollingUpdateStrategy())
		}
		if !equalPodTemplate(upgraded.Spec.Template, stored.Spec.Template) {
			t.Fatalf("%d instances: the upgrade changed the pod template, which rolls every instance", n)
		}
	}
}

// heldWebImageApp is a web image service whose release may not take traffic
// yet (w4/m320), so projectRolloutHold holds its serving pods (w4/571).
func heldWebImageApp(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	app := webImageApp(2)
	app.Status.ActiveRevision = "rev-1"
	app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{HTTPSelection: appv1alpha1.HTTPSelectionDetected, Image: "nginx@sha256:a", Revision: "rev-1", PrimaryPort: 80, Ports: []int32{80}, ConfiguredPort: new(int32(0))}
	app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{HTTPSelection: appv1alpha1.HTTPSelectionDetected, Image: "nginx@sha256:b", Revision: "rev-2", PrimaryPort: 80, Ports: []int32{80}, ConfiguredPort: new(int32(0))}
	for _, m := range mutate {
		m(app)
	}
	return app
}

// Every move between the three strategies an App Deployment can carry lands
// on the one its App calls for, from whichever it held: the held release's
// 100%/0 (w4/571), a disk's Recreate (w6/m547) and the rolling default.
func TestRolloutStrategyTransitions(t *testing.T) {
	if !webImageReleaseHeld(heldWebImageApp()) || !webImageReleaseHeld(heldWebImageApp(withDisk)) {
		t.Fatal("fixture: the web image release is not held")
	}
	recreate := appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	stored := map[string]appsv1.DeploymentStrategy{
		"empty":              {},
		"API server default": apiServerDefaultStrategy(),
		"rolling":            defaultRollingUpdateStrategy(),
		"held":               heldReleaseStrategy(),
		"Recreate":           recreate,
	}
	for _, tc := range []struct {
		name   string
		app    *appv1alpha1.App
		params deploymentParams
		want   appsv1.DeploymentStrategy
	}{
		{"unheld, diskless", projectionApp(), webParams(), defaultRollingUpdateStrategy()},
		{"held", heldWebImageApp(), deploymentParams{image: "nginx@sha256:b", port: 80, replicas: 3}, heldReleaseStrategy()},
		{"with a disk", projectionApp(withDisk), diskParams(""), recreate},
		{"held, with a disk", heldWebImageApp(withDisk), deploymentParams{image: "nginx@sha256:b", port: 80, replicas: 1}, recreate},
	} {
		for from, strategy := range stored {
			dep := project(tc.app, tc.params)
			dep.Spec.Strategy = *strategy.DeepCopy()
			applyDeploymentSpec(dep, tc.app, tc.params)
			if !apiequality.Semantic.DeepEqual(dep.Spec.Strategy, tc.want) {
				t.Errorf("%s from %s: strategy %+v, want %+v", tc.name, from, dep.Spec.Strategy, tc.want)
			}
		}
	}
}

// The pass that promotes a held release lifts the hold with projectRolloutHold
// alone (releasePromotedHold, w4/672). It never touches a disk's Recreate,
// which the API server would refuse to combine with rolling parameters and
// which keeps two revisions off one volume.
func TestProjectRolloutHoldLeavesADisksRecreate(t *testing.T) {
	for name, app := range map[string]*appv1alpha1.App{
		"held":   heldWebImageApp(withDisk),
		"unheld": projectionApp(withDisk),
	} {
		dep := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}}}
		projectRolloutHold(dep, app)
		if dep.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType || dep.Spec.Strategy.RollingUpdate != nil {
			t.Errorf("%s disk service: strategy %+v after projectRolloutHold, want Recreate untouched", name, dep.Spec.Strategy)
		}
	}
}
