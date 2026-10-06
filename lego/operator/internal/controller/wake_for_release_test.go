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
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/076: a deploy to a sleeping free service wakes it, once per release.

func TestDeployWhileAsleepGoesLiveWithoutARequest(t *testing.T) {
	app := activeApp("tea-076")
	r, cl, nn := lifecycleFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	parkIdle(t, r, cl, nn)
	deployImageAt(t, cl, nn, "nginx:2", 2)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("replicas = %d, want the deploy to wake the service", got)
	}

	// The served pod is ready, then release 2 rolls over it and goes live.
	setDeploymentStatus(t, cl, nn, statusServedPodReady)
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-2" {
		t.Fatalf("template revision = %q, want release 2 rolling", got)
	}
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	live := liveApp(t, cl, nn)
	if live.Status.ActiveRevision != "rev-2" || live.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("phase %q, activeRevision %q; want release 2 Running", live.Status.Phase, live.Status.ActiveRevision)
	}
}

func TestDeployWhileSuspendedDoesNotWake(t *testing.T) {
	app := activeApp("tea-076")
	r, cl, nn := lifecycleFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	live := liveApp(t, cl, nn)
	live.Spec.Suspended = true
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	deployImageAt(t, cl, nn, "nginx:2", 2)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d, want a suspended service to stay parked", got)
	}
	if got := liveApp(t, cl, nn).Annotations[annotReleaseWakeGeneration]; got != "" {
		t.Fatalf("release-wake generation = %q on a suspended service", got)
	}
}

// One wake per release: a release that never reaches the Deployment (here the
// served pod never becomes ready inside the budget and the service idles out
// again) cannot keep a free service awake by waking it on every park.
func TestDeployWhileAsleepWakesOncePerRelease(t *testing.T) {
	app := activeApp("tea-076")
	r, cl, nn := lifecycleFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	parkIdle(t, r, cl, nn)
	deployImageAt(t, cl, nn, "nginx:2", 2)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("replicas = %d, want the first wake", got)
	}
	if got := liveApp(t, cl, nn).Annotations[annotReleaseWakeGeneration]; got != "2" {
		t.Fatalf("release-wake generation = %q, want 2", got)
	}

	parkIdle(t, r, cl, nn)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d, want release 2 not to wake the service twice", got)
	}

	// The next release wakes it again.
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	deployImageAt(t, cl, nn, "nginx:3", 3)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("replicas = %d, want release 3 to wake the service", got)
	}
}

// w6/m147 t003: a parking pass of a sleeping service must not write Deploying.
// rolloutPending is true for any phase but Running, so every reconcile of a
// Hibernated service stamped Deploying before parkKubernetes wrote Hibernated
// against the cached App, which skipped it as unchanged: the header read
// Deploying while the service slept. The reconciler reads through a cache that
// lags its own writes (w5/074): what is stored after every pass is Hibernated.
func TestParkingPassNeverWritesDeploying(t *testing.T) {
	app := activeApp("tea-m147")
	var written []appv1alpha1.AppPhase
	r, cl, nn := lifecycleFixture(t, app, withLaggingCache(), withInterceptor(interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		if a, ok := obj.(*appv1alpha1.App); ok {
			written = append(written, a.Status.Phase)
		}
		return c.SubResource(sub).Update(ctx, obj, opts...)
	}}))

	serveReleaseOne(t, r, cl, nn)
	parkIdle(t, r, cl, nn)
	written = nil
	for pass := range 6 {
		reconcileOnce(t, r, nn)
		if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
			t.Fatalf("after parking pass %d the stored phase is %q, want Hibernated", pass, got)
		}
	}
	for _, phase := range written {
		if phase == appv1alpha1.PhaseDeploying {
			t.Fatalf("a parking pass wrote phase Deploying (writes %v)", written)
		}
	}
}
