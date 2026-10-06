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
	"strconv"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The wake, park and hold tests share one fake cluster (lifecycleFixture). The
// fake client runs no controllers, so a test reports the Deployment's status
// itself (setDeploymentStatus).

type fixtureConfig struct {
	scheme  *runtime.Scheme
	objs    []client.Object
	funcs   *interceptor.Funcs
	lagging bool
}

type fixtureOption func(*fixtureConfig)

// withScheme replaces the fixture's scheme, for a test that stores more kinds.
func withScheme(scheme *runtime.Scheme) fixtureOption {
	return func(c *fixtureConfig) { c.scheme = scheme }
}

// withObjects stores objs beside the App.
func withObjects(objs ...client.Object) fixtureOption {
	return func(c *fixtureConfig) { c.objs = append(c.objs, objs...) }
}

// withInterceptor routes the store's calls through funcs.
func withInterceptor(funcs interceptor.Funcs) fixtureOption {
	return func(c *fixtureConfig) { c.funcs = &funcs }
}

// withLaggingCache gives the reconciler a laggingClient over the store. Drive
// its passes with reconcileOnce, which starts each pass's snapshot.
func withLaggingCache() fixtureOption {
	return func(c *fixtureConfig) { c.lagging = true }
}

// lifecycleFixture stores app, with App, Deployment and Job status
// subresources, and returns a reconciler with an activator configured, the
// store and app's key. Tests read and write the store directly; the reconciler
// reads it through a laggingClient when asked to.
func lifecycleFixture(t *testing.T, app *appv1alpha1.App, opts ...fixtureOption) (*AppReconciler, client.Client, types.NamespacedName) {
	t.Helper()
	cfg := fixtureConfig{scheme: wakeScheme()}
	for _, opt := range opts {
		opt(&cfg)
	}
	// The fake client assigns no generation, and a release is recorded under its
	// own: start at 1 so release 1 has a record to restore.
	if app.Generation == 0 {
		app.Generation = 1
	}
	builder := fake.NewClientBuilder().WithScheme(cfg.scheme).WithObjects(append([]client.Object{app}, cfg.objs...)...).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}, &batchv1.Job{})
	if cfg.funcs != nil {
		builder = builder.WithInterceptorFuncs(*cfg.funcs)
	}
	store := builder.Build()
	r := wakeReconciler(store, cfg.scheme)
	if cfg.lagging {
		r.Client = &laggingClient{Client: store, seen: map[types.NamespacedName]*appv1alpha1.App{}}
	}
	return r, store, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
}

// laggingClient serves each App read of a pass from the App as it was at the
// pass's first read, as an informer cache does that has not yet observed the
// pass's own writes (w5/074). Writes reach the store; reconcileOnce starts each
// pass.
type laggingClient struct {
	client.Client
	seen map[types.NamespacedName]*appv1alpha1.App
}

func (c *laggingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	app, ok := obj.(*appv1alpha1.App)
	if !ok {
		return c.Client.Get(ctx, key, obj, opts...)
	}
	if seen, ok := c.seen[key]; ok {
		seen.DeepCopyInto(app)
		return nil
	}
	if err := c.Client.Get(ctx, key, app, opts...); err != nil {
		return err
	}
	c.seen[key] = app.DeepCopy()
	return nil
}

// reconcileOnce runs one pass of r over nn.
func reconcileOnce(t *testing.T, r *AppReconciler, nn types.NamespacedName) {
	t.Helper()
	if c, ok := r.Client.(*laggingClient); ok {
		clear(c.seen)
	}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

// Deployment states the deployment controller reports.
var (
	// statusRolledOut: the current template fully rolled out and ready, which
	// deploymentRolloutReady waits for before markRunning activates a release.
	statusRolledOut = appsv1.DeploymentStatus{Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	// statusServedPodReady: one ready pod that is not of the newest template, the
	// served release's while a newer release rolls or once it alone started.
	// Enough for routing, never for activating a release.
	statusServedPodReady = appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	// statusDrained: a parked Deployment with no pods.
	statusDrained = appsv1.DeploymentStatus{}
)

// statusRolloutFailedAt is the newest template past its progress deadline at at, with
// none of its pods updated and ready pods of the old ReplicaSet: 1 while the
// service stayed awake through the rollout, 0 when it rolled from parked.
func statusRolloutFailedAt(ready int32, at time.Time) appsv1.DeploymentStatus {
	return appsv1.DeploymentStatus{
		Replicas: ready, ReadyReplicas: ready, AvailableReplicas: ready,
		Conditions: []appsv1.DeploymentCondition{{
			Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded",
			LastTransitionTime: metav1.NewTime(at),
		}},
	}
}

// statusUnavailableSince is one pod asked for and none ready since at. failure, when
// set, is the ReplicaFailure the deployment controller copies from a
// ReplicaSet that cannot create its pods.
func statusUnavailableSince(at time.Time, failure string) appsv1.DeploymentStatus {
	status := appsv1.DeploymentStatus{
		Replicas: 1, UnavailableReplicas: 1,
		Conditions: []appsv1.DeploymentCondition{{
			Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse, Reason: "MinimumReplicasUnavailable",
			LastTransitionTime: metav1.NewTime(at),
		}},
	}
	if failure != "" {
		status.Replicas = 0
		status.Conditions = append(status.Conditions, appsv1.DeploymentCondition{
			Type: appsv1.DeploymentReplicaFailure, Status: corev1.ConditionTrue, Reason: "FailedCreate", Message: failure,
			LastTransitionTime: metav1.NewTime(at),
		})
	}
	return status
}

// setDeploymentStatus reports status on nn's Deployment, observed at its
// current generation.
func setDeploymentStatus(t *testing.T, cl client.Client, nn types.NamespacedName, status appsv1.DeploymentStatus) {
	t.Helper()
	dep := liveDeployment(t, cl, nn)
	dep.Status = status
	dep.Status.ObservedGeneration = dep.Generation
	if err := cl.Status().Update(context.Background(), &dep); err != nil {
		t.Fatal(err)
	}
}

func wakeScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return scheme
}

func wakeReconciler(cl client.Client, scheme *runtime.Scheme) *AppReconciler {
	return &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co",
		ActivatorService: "bex-activator", ActivatorNamespace: "bex-system", ActivatorPort: 8888,
	}
}

// hibernatingApp is an idle free-tier web service past its idle TTL: exactly
// the shape desiredReplicas auto-hibernates.
func hibernatingApp(namespace string) *appv1alpha1.App {
	// A tenant App is canonical only when its workspace label matches its
	// namespace (the confused-deputy guard in Reconcile); the bootstrap apps
	// namespace needs no label.
	labels := map[string]string{}
	if namespace != defaultAppsNamespace {
		labels[labelWorkspace] = namespace
	}
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: namespace, Labels: labels,
			Annotations: map[string]string{
				annotLastActive: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Image: "nginx:1", Tier: "free",
			Port: 3000, Replicas: 1, Expose: true, IdleTTLSeconds: 1,
		},
	}
}

// activeApp is hibernatingApp's awake sibling: same free-tier, auto-sleep
// eligible service, but last seen just now, so it is serving rather than idle.
func activeApp(namespace string) *appv1alpha1.App {
	app := hibernatingApp(namespace)
	app.Annotations[annotLastActive] = time.Now().UTC().Format(time.RFC3339)
	app.Spec.IdleTTLSeconds = 300
	return app
}

// heldWorkerApp is a background worker: no Service, Ingress or auto-sleep, and
// no plan instance cap, so a manual scale takes effect.
func heldWorkerApp(namespace string) *appv1alpha1.App {
	app := activeApp(namespace)
	app.Spec.Type = appv1alpha1.TypeBackgroundWorker
	app.Spec.Tier = ""
	app.Spec.Expose = false
	return app
}

func reconcileTwice(t *testing.T, r *AppReconciler, nn types.NamespacedName) {
	t.Helper()
	for range 2 {
		reconcileOnce(t, r, nn)
	}
}

// serveReleaseOne reconciles the fixture's prebuilt release until it is the
// active release, and returns its pod template revision and image.
func serveReleaseOne(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName) (string, string) {
	t.Helper()
	reconcileTwice(t, r, nn)
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	live := liveApp(t, cl, nn)
	if live.Status.ActiveRevision == "" || live.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("setup: release 1 never became active (phase %q, activeRevision %q)", live.Status.Phase, live.Status.ActiveRevision)
	}
	return deploymentTemplateRevision(t, cl, nn), live.Status.Image
}

// parkIdle lets the service go idle: the route moves to the activator, then the
// pods drain.
func parkIdle(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	reconcileOnce(t, r, nn)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("setup: replicas = %d, want 0 once hibernated", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
		t.Fatalf("parked phase = %q, want Hibernated", got)
	}
	setDeploymentStatus(t, cl, nn, statusDrained)
}

// updateApp applies change to the stored App.
func updateApp(t *testing.T, cl client.Client, nn types.NamespacedName, change func(*appv1alpha1.App)) {
	t.Helper()
	live := liveApp(t, cl, nn)
	change(&live)
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
}

// deployImageAt requests a new release that changes the image.
func deployImageAt(t *testing.T, cl client.Client, nn types.NamespacedName, image string, generation int64) {
	t.Helper()
	updateApp(t, cl, nn, func(a *appv1alpha1.App) {
		a.Spec.Image = image
		a.Generation = generation
		a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(generation, 10)
	})
}

func stampLastActiveAt(t *testing.T, cl client.Client, nn types.NamespacedName, at time.Time) {
	t.Helper()
	updateApp(t, cl, nn, func(a *appv1alpha1.App) { a.Annotations[annotLastActive] = at.UTC().Format(time.RFC3339) })
}

func setSuspendedAt(t *testing.T, cl client.Client, nn types.NamespacedName, suspended bool, generation int64) {
	t.Helper()
	updateApp(t, cl, nn, func(a *appv1alpha1.App) { a.Spec.Suspended, a.Generation = suspended, generation })
}

func liveApp(t *testing.T, cl client.Client, nn types.NamespacedName) appv1alpha1.App {
	t.Helper()
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	return live
}

func appPhase(t *testing.T, cl client.Client, nn types.NamespacedName) appv1alpha1.AppPhase {
	t.Helper()
	return liveApp(t, cl, nn).Status.Phase
}

func liveDeployment(t *testing.T, cl client.Client, nn types.NamespacedName) appsv1.Deployment {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	return dep
}

func deploymentReplicas(t *testing.T, cl client.Client, nn types.NamespacedName) int32 {
	t.Helper()
	if dep := liveDeployment(t, cl, nn); dep.Spec.Replicas != nil {
		return *dep.Spec.Replicas
	}
	return 1
}

func deploymentTemplate(t *testing.T, cl client.Client, nn types.NamespacedName) corev1.PodTemplateSpec {
	t.Helper()
	return liveDeployment(t, cl, nn).Spec.Template
}

func deploymentTemplateRevision(t *testing.T, cl client.Client, nn types.NamespacedName) string {
	t.Helper()
	return deploymentTemplate(t, cl, nn).Labels[labelRevision]
}

func ingressBackendName(t *testing.T, cl client.Client, nn types.NamespacedName) string {
	t.Helper()
	var ing networkingv1.Ingress
	if err := cl.Get(context.Background(), nn, &ing); err != nil {
		t.Fatal(err)
	}
	return ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name
}
