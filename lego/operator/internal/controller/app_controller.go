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
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/bex-co/bex/lego/operator/internal/build"
	bexruntime "github.com/bex-co/bex/lego/operator/internal/runtime"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const finalizer = "app.bex.co/finalizer"

// labelApp marks the workloads bex creates for an App.
const labelApp = "app.bex.co/app"

// labelWorkspace carries the owning workspace (tenant) id on pod templates so
// NetworkPolicy selectors can express "same-workspace" allow rules. Kept in
// sync by hand with core.LabelWorkspace in the backend — the operator never
// imports the backend (same pattern as labelApp / core.PodLabelApp).
const labelWorkspace = "app.bex.co/workspace"

// annotLastActive records when the app last served (or received) traffic.
// Set by the operator on first Running and reset on each wake; updated by the
// activator on each inbound request. Free-tier apps auto-hibernate after
// idleTTLSeconds past this timestamp.
const annotLastActive = "app.bex.co/last-active"

// Runtime modes.
const (
	ModeOpenSandbox = "opensandbox" // run revisions as OpenSandbox sandboxes (host)
	ModeKubernetes  = "kubernetes"  // run revisions as k8s Deployments (pods on cluster nodes)
)

// AppReconciler reconciles an App: resolve an image (prebuilt or built from git)
// and run it as a revision on the selected runtime, recording status.
type AppReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Mode             string                  // ModeOpenSandbox | ModeKubernetes
	Registry         string                  // in-cluster registry, e.g. zot.bex-registry.svc:5000
	CNBBuilder       string                  // e.g. paketobuildpacks/builder-jammy-base
	BuildNamespace   string                  // namespace in-cluster build Jobs run in; empty => the App's namespace
	Runtime          *bexruntime.OpenSandbox // OpenSandbox client (ModeOpenSandbox)
	BaseDomain       string                  // optional: "<name>.<BaseDomain>" when Expose && Host=="" (e.g. bex.co)
	ClusterIssuer    string                  // cert-manager ClusterIssuer for App Ingresses (letsencrypt-staging|-prod)
	ActivatorService string                  // k8s Service name of the wake activator; empty => auto-sleep disabled
	ActivatorPort    int                     // activator listen port (default 8888)
}

// +kubebuilder:rbac:groups=app.bex.co,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bex.co,resources=apps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.bex.co,resources=apps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var app appv1alpha1.App
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Deletion: tear down external resources (OpenSandbox sandbox); the owned k8s
	// Deployment/Service are garbage-collected via owner refs.
	if !app.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&app, finalizer) {
			if app.Status.SandboxID != "" {
				_ = r.Runtime.Delete(ctx, app.Status.SandboxID)
			}
			controllerutil.RemoveFinalizer(&app, finalizer)
			if err := r.Update(ctx, &app); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}
	if controllerutil.AddFinalizer(&app, finalizer) {
		if err := r.Update(ctx, &app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil // finalizer update doesn't bump generation
	}

	// NOTE: intentionally NO early-return on ObservedGeneration==Generation here.
	// This controller is level-triggered: every reconcile re-applies desired state
	// (Deployment/Service/Ingress via CreateOrUpdate below) so operator-level config
	// changes — cluster issuer, base domain, tier — reach already-running Apps on the
	// next reconcile / operator restart, not only on a spec bump. Re-adding an
	// early-return here reintroduces the config-drift bug. The expensive build path is
	// gated separately (Status.Image reuse below), so a no-op pass never rebuilds.

	port := int(app.Spec.Port)
	if port == 0 {
		port = 3000
	}

	// --- resolve the image: prebuilt, or build from git ---
	// Reuse the cached Status.Image (never rebuild) when the spec generation is
	// unchanged — this reconcile only re-applies desired state (e.g. an operator
	// config change), not a new revision — or when the App is suspended. Only a
	// genuine spec/revision bump (generation changed, not suspended) rebuilds.
	image := app.Spec.Image
	if image == "" && app.Status.Image != "" &&
		(app.Spec.Suspended || app.Status.ObservedGeneration == app.Generation) {
		image = app.Status.Image
	}
	if image == "" {
		if app.Spec.Repo == "" {
			return r.fail(ctx, &app, "BadSpec", fmt.Errorf("one of spec.image or spec.repo is required"))
		}
		branch := app.Spec.Branch
		if branch == "" {
			branch = "main"
		}
		r.setPhase(ctx, &app, appv1alpha1.PhaseBuilding, "Building", "Building image from "+app.Spec.Repo)
		// Tag by generation: a spec/revision bump (including a webhook redeploy that
		// stamps spec.restartedAt) yields a new tag, so the built image is fresh and
		// the Deployment re-pulls it. An in-cluster BuildKit Job does the work — no
		// docker daemon on the node.
		res, err := build.Build(ctx, build.Options{
			Repo: app.Spec.Repo, Ref: branch, Name: app.Name,
			Registry: r.Registry, CNBBuilder: r.CNBBuilder,
			Builder:   app.Spec.Builder,
			Revision:  fmt.Sprintf("gen-%d", app.Generation),
			Namespace: r.buildNamespace(app.Namespace),
			Client:    r.Client,
		})
		if err != nil {
			return r.fail(ctx, &app, "BuildFailed", err)
		}
		image = res.Image
	}

	if r.Mode == ModeKubernetes {
		return r.reconcileKubernetes(ctx, &app, image, port)
	}
	return r.reconcileOpenSandbox(ctx, &app, image, port)
}

// resourcesForTier maps an App tier (plan) to a fixed pod allocation, set as
// requests == limits (Guaranteed) — one ladder, lego/types/tiers' compute
// family, shared with the backend store's tier/plan validation. Empty/unknown
// tier => no constraints (best-effort, prior behavior); the control plane
// sets a tier explicitly.
func resourcesForTier(tier string) corev1.ResourceRequirements {
	cpu, mem, ok := tiers.Compute.Resources(tier)
	if !ok {
		return corev1.ResourceRequirements{} // unset => best-effort, unchanged behavior
	}
	return guaranteedResources(cpu, mem)
}

// guaranteedResources builds a Guaranteed-QoS allocation (requests == limits)
// from two k8s quantity strings — the mechanism every tier family (compute for
// Apps, valkey for KeyValue) shares. Catalog quantities are validated at load,
// so MustParse cannot panic.
func guaranteedResources(cpu, memory string) corev1.ResourceRequirements {
	list := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse(memory),
	}
	return corev1.ResourceRequirements{Requests: list, Limits: list}
}

// isFreeApp reports whether the app is on the free tier (empty or "free").
// Paid tiers are always-on and never auto-hibernate.
func isFreeApp(app *appv1alpha1.App) bool {
	return app.Spec.Tier == "" || app.Spec.Tier == "free"
}

// buildNamespace is where an App's in-cluster build Job runs: the configured
// BuildNamespace, or the App's own namespace when unset (co-located; the
// registry is reached over cluster DNS either way).
func (r *AppReconciler) buildNamespace(appNS string) string {
	if r.BuildNamespace != "" {
		return r.BuildNamespace
	}
	return appNS
}

// lastActiveTime parses the annotLastActive annotation, returning zero if absent or invalid.
func lastActiveTime(app *appv1alpha1.App) time.Time {
	raw := app.Annotations[annotLastActive]
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

// shouldAutoHibernate reports whether the app should scale to zero now:
// free tier, idleTTLSeconds > 0, not manually suspended, and last-active
// timestamp is older than the TTL.
func shouldAutoHibernate(app *appv1alpha1.App) bool {
	if app.Spec.Suspended {
		return false
	}
	if app.Spec.IdleTTLSeconds <= 0 {
		return false
	}
	if !isFreeApp(app) {
		return false
	}
	last := lastActiveTime(app)
	if last.IsZero() {
		return false // not yet stamped; operator will stamp on first Running reconcile
	}
	return time.Since(last) >= time.Duration(app.Spec.IdleTTLSeconds)*time.Second
}

// idleRequeueAfter returns how long until the idle TTL elapses from now.
func idleRequeueAfter(app *appv1alpha1.App, now time.Time) time.Duration {
	ttl := time.Duration(app.Spec.IdleTTLSeconds) * time.Second
	last := lastActiveTime(app)
	if last.IsZero() {
		return ttl
	}
	remaining := last.Add(ttl).Sub(now)
	if remaining < 5*time.Second {
		return 5 * time.Second
	}
	return remaining
}

// reconcileKubernetes runs the revision as a Deployment (+ ClusterIP k8s Service)
// owned by the App — pods are scheduled onto the cluster's nodes (machines).
//
//nolint:gocyclo // one cohesive linear reconcile pass (Deployment → Service → Ingress → status); each step is a guarded CreateOrUpdate, and splitting it solely to satisfy the counter would fragment a coherent unit without aiding readability.
func (r *AppReconciler) reconcileKubernetes(ctx context.Context, app *appv1alpha1.App, image string, port int) (ctrl.Result, error) {
	// A cron_job diverges entirely: a batch/v1 CronJob, no Deployment/Service/Ingress.
	if app.Spec.Type == appv1alpha1.TypeCronJob {
		return r.reconcileCronJob(ctx, app, image, port)
	}
	// A background_worker runs the image with no HTTP port: a bare Deployment, no
	// Service, no Ingress, no URL, no auto-sleep (nothing routes traffic to wake it).
	worker := app.Spec.Type == appv1alpha1.TypeBackgroundWorker

	r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Deploying", "Reconciling Deployment for "+image)

	// Auto-hibernate: idle free-tier app past its TTL → scale to 0 without
	// touching spec.suspended, so manual-suspend semantics are preserved. A worker
	// never auto-hibernates — it has no Ingress, so a request could never wake it.
	autoHibernating := !worker && r.ActivatorService != "" && shouldAutoHibernate(app)

	replicas := effectiveReplicas(app)
	if autoHibernating {
		replicas = 0
	}
	labels := map[string]string{labelApp: app.Name}
	// Propagate the workspace label to pod templates so NetworkPolicy selectors
	// can express "allow same-workspace" rules. The selector stays stable
	// (labelApp only) — adding labelWorkspace here would make it immutable and
	// break existing Deployments when the label is added later.
	podLabels := map[string]string{labelApp: app.Name}
	if ws := app.Labels[labelWorkspace]; ws != "" {
		podLabels[labelWorkspace] = ws
	}

	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		dep.Spec.Template.Labels = podLabels
		// restart = roll the template (same mechanism as kubectl rollout restart,
		// recorded in the contract). Never removed once set — removal would
		// itself roll the pods again.
		if app.Spec.RestartedAt != "" {
			if dep.Spec.Template.Annotations == nil {
				dep.Spec.Template.Annotations = map[string]string{}
			}
			dep.Spec.Template.Annotations["app.bex.co/restarted-at"] = app.Spec.RestartedAt
		}
		// A worker has no HTTP port, so it declares no container port.
		var ports []corev1.ContainerPort
		if !worker {
			ports = []corev1.ContainerPort{{ContainerPort: int32(port)}}
		}
		dep.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:      "app",
			Image:     image,
			Env:       appEnv(app, port),
			EnvFrom:   envFromSources(app),
			Ports:     ports,
			Resources: resourcesForTier(app.Spec.Tier),
		}}
		return controllerutil.SetControllerReference(app, dep, r.Scheme)
	}); err != nil {
		return r.fail(ctx, app, "DeployFailed", err)
	}

	// Reconcile the per-App NetworkPolicy (docs/tenant-isolation.md). Only when
	// the App carries a workspace label — legacy/hand-applied Apps run without
	// a policy (consistent with prior behavior). Workers get a policy too: they
	// need egress to same-workspace services even though they expose no port.
	if err := r.reconcileNetworkPolicy(ctx, app); err != nil {
		return r.fail(ctx, app, "NetworkPolicyFailed", err)
	}

	// A worker skips the Service, the Ingress, and the URL entirely; any left over
	// from a type change are cleaned up so nothing dangles.
	if worker {
		return r.reconcileWorkerStatus(ctx, app, image, dep, replicas)
	}

	// the k8s core Service (ClusterIP) that fronts the App's pods
	ksvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ksvc, func() error {
		ksvc.Spec.Selector = labels
		ksvc.Spec.Ports = []corev1.ServicePort{{Port: int32(port), TargetPort: intstr.FromInt(port)}}
		return controllerutil.SetControllerReference(app, ksvc, r.Scheme)
	}); err != nil {
		return r.fail(ctx, app, "ServiceFailed", err)
	}

	// Optional external exposure: when any host resolves (spec.host, the computed
	// platform hostname, or spec.hosts custom domains), front the Service with one
	// Ingress carrying a rule + TLS certificate per host. No hosts => in-cluster
	// only, exactly as before. The operator emits a standard networking.k8s.io
	// Ingress, so the ingress controller (traefik today) stays swappable.
	hosts := effectiveHosts(app, r.BaseDomain)

	// When auto-hibernating, route all traffic to the activator so it can wake
	// the app on the next request; restore the app's own service when running.
	ingressSvc := app.Name
	ingressPort := int32(port)
	if autoHibernating {
		ingressSvc = r.ActivatorService
		ingressPort = int32(r.ActivatorPort)
	}

	if len(hosts) > 0 {
		ingressClass := "traefik"
		pathType := networkingv1.PathTypePrefix
		ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ing, func() error {
			if ing.Annotations == nil {
				ing.Annotations = map[string]string{}
			}
			if r.ClusterIssuer != "" {
				ing.Annotations["cert-manager.io/cluster-issuer"] = r.ClusterIssuer
			}
			ing.Spec.IngressClassName = &ingressClass
			ing.Spec.TLS = nil
			ing.Spec.Rules = nil
			for i, host := range hosts {
				ing.Spec.TLS = append(ing.Spec.TLS, networkingv1.IngressTLS{
					Hosts:      []string{host},
					SecretName: tlsSecretName(app.Name, i, host),
				})
				ing.Spec.Rules = append(ing.Spec.Rules, networkingv1.IngressRule{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: ingressSvc,
										Port: networkingv1.ServiceBackendPort{Number: ingressPort},
									},
								},
							}},
						},
					},
				})
			}
			return controllerutil.SetControllerReference(app, ing, r.Scheme)
		}); err != nil {
			return r.fail(ctx, app, "IngressFailed", err)
		}
	} else {
		// Exposure turned off (all hosts cleared): remove any Ingress we previously created.
		stale := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		if err := r.Delete(ctx, stale); err != nil && !apierrors.IsNotFound(err) {
			return r.fail(ctx, app, "IngressCleanupFailed", err)
		}
	}

	// Suspended: parked at 0 replicas with Service/Ingress/TLS (and spec.replicas)
	// all kept — resume is just scaling back. Report Hibernated and stop.
	if app.Spec.Suspended {
		app.Status.Phase = appv1alpha1.PhaseHibernated
		app.Status.Image = image
		if len(hosts) > 0 {
			app.Status.URL = "https://" + hosts[0]
		}
		app.Status.ObservedGeneration = app.Generation
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "Suspended",
			Message:            "suspended (scaled to 0; config, host and certs kept)",
			ObservedGeneration: app.Generation,
		})
		if err := r.Status().Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Auto-hibernated: idle free-tier app scaled to 0, Ingress now points at the
	// activator. The next inbound request will wake it; no further requeue needed.
	if autoHibernating {
		app.Status.Phase = appv1alpha1.PhaseHibernated
		app.Status.Image = image
		if len(hosts) > 0 {
			app.Status.URL = "https://" + hosts[0]
		}
		app.Status.ObservedGeneration = app.Generation
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type:   "Ready",
			Status: metav1.ConditionFalse,
			Reason: "AutoHibernated",
			Message: fmt.Sprintf("idle ≥%ds on free tier; wakes on next request",
				app.Spec.IdleTTLSeconds),
			ObservedGeneration: app.Generation,
		})
		if err := r.Status().Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		logf.FromContext(ctx).Info("app auto-hibernated", "name", app.Name, "idleTTL", app.Spec.IdleTTLSeconds)
		return ctrl.Result{}, nil
	}

	// Readiness: requeue until the Deployment has its replicas ready.
	_ = r.Get(ctx, client.ObjectKeyFromObject(dep), dep)
	if dep.Status.ReadyReplicas < replicas {
		app.Status.Phase = appv1alpha1.PhaseDeploying
		app.Status.Image = image
		_ = r.Status().Update(ctx, app)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.Image = image
	if len(hosts) > 0 {
		app.Status.URL = "https://" + hosts[0]
		app.Status.URLs = nil
		for _, h := range hosts {
			app.Status.URLs = append(app.Status.URLs, "https://"+h)
		}
	} else {
		app.Status.URL = fmt.Sprintf("http://%s.%s.svc:%d", app.Name, app.Namespace, port)
		app.Status.URLs = nil
	}
	app.Status.ActiveRevision = fmt.Sprintf("rev-%d", app.Generation)
	app.Status.ObservedGeneration = app.Generation
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Deployed",
		Message:            fmt.Sprintf("%d/%d replicas ready", dep.Status.ReadyReplicas, replicas),
		ObservedGeneration: app.Generation,
	})
	if err := r.Status().Update(ctx, app); err != nil {
		return ctrl.Result{}, err
	}
	logf.FromContext(ctx).Info("app running (kubernetes)", "name", app.Name, "image", image, "replicas", replicas)

	// Free-tier apps with an idle TTL: stamp last-active on first Running reconcile
	// and schedule a re-check after the TTL so the operator can auto-hibernate.
	if r.ActivatorService != "" && app.Spec.IdleTTLSeconds > 0 && isFreeApp(app) {
		now := time.Now().UTC()
		if lastActiveTime(app).IsZero() {
			base := app.DeepCopy()
			if app.Annotations == nil {
				app.Annotations = map[string]string{}
			}
			app.Annotations[annotLastActive] = now.Format(time.RFC3339)
			if err := r.Patch(ctx, app, client.MergeFrom(base)); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: idleRequeueAfter(app, now)}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileWorkerStatus finishes the background_worker reconcile: a worker's
// Deployment is already applied by reconcileKubernetes, so here we tear down any
// Service/Ingress left from a prior type, then report status with no URL (a
// worker has no HTTP endpoint). It shares the Deployment/readiness shape with the
// web path but skips exposure and the auto-sleep machinery entirely.
func (r *AppReconciler) reconcileWorkerStatus(ctx context.Context, app *appv1alpha1.App, image string, dep *appsv1.Deployment, replicas int32) (ctrl.Result, error) {
	for _, obj := range []client.Object{
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}},
	} {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return r.fail(ctx, app, "WorkerCleanupFailed", err)
		}
	}

	app.Status.Image = image
	app.Status.URL = "" // a worker has no HTTP endpoint
	app.Status.URLs = nil
	app.Status.ObservedGeneration = app.Generation

	if app.Spec.Suspended {
		app.Status.Phase = appv1alpha1.PhaseHibernated
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "Suspended",
			Message: "worker suspended (scaled to 0)", ObservedGeneration: app.Generation,
		})
		if err := r.Status().Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	_ = r.Get(ctx, client.ObjectKeyFromObject(dep), dep)
	if dep.Status.ReadyReplicas < replicas {
		app.Status.Phase = appv1alpha1.PhaseDeploying
		_ = r.Status().Update(ctx, app)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.ActiveRevision = fmt.Sprintf("rev-%d", app.Generation)
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Deployed",
		Message:            fmt.Sprintf("%d/%d replicas ready", dep.Status.ReadyReplicas, replicas),
		ObservedGeneration: app.Generation,
	})
	if err := r.Status().Update(ctx, app); err != nil {
		return ctrl.Result{}, err
	}
	logf.FromContext(ctx).Info("worker running (kubernetes)", "name", app.Name, "image", image, "replicas", replicas)
	return ctrl.Result{}, nil
}

// maxCronRuns caps how many recent runs the status carries — enough to show a
// history without unbounded status growth.
const maxCronRuns = 10

// reconcileCronJob materializes a cron_job as a batch/v1 CronJob (no
// Deployment/Service/Ingress), triggers a one-off run when spec.runAt asks for
// one, and records recent runs in status. Because the shared
// GenerationChangedPredicate filters owned Job/CronJob status events, it polls
// (RequeueAfter) to keep the run history fresh rather than watching Jobs.
func (r *AppReconciler) reconcileCronJob(ctx context.Context, app *appv1alpha1.App, image string, port int) (ctrl.Result, error) {
	if app.Spec.Schedule == "" {
		return r.fail(ctx, app, "BadSpec", fmt.Errorf("spec.schedule is required for a cron_job"))
	}
	r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Deploying", "Reconciling CronJob for "+image)
	labels := map[string]string{labelApp: app.Name}
	suspended := app.Spec.Suspended

	cj := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cj, func() error {
		cj.Spec.Schedule = app.Spec.Schedule
		// Suspend pauses scheduling without losing history — resume just clears it.
		cj.Spec.Suspend = &suspended
		cj.Spec.JobTemplate.Labels = labels // so the Jobs it creates carry labelApp
		cj.Spec.JobTemplate.Spec.Template = cronPodSpec(app, image, port, labels)
		return controllerutil.SetControllerReference(app, cj, r.Scheme)
	}); err != nil {
		return r.fail(ctx, app, "CronJobFailed", err)
	}

	// One-off run trigger (spec.runAt, from the API's cron run verb): materialize a
	// single Job from the same template, named deterministically from runAt so a
	// re-reconcile of the same value is a no-op. Skipped while suspended.
	if app.Spec.RunAt != "" && !suspended {
		if err := r.ensureManualRun(ctx, app, image, port, labels); err != nil {
			return r.fail(ctx, app, "CronRunFailed", err)
		}
	}

	runs, err := r.cronRuns(ctx, app)
	if err != nil {
		return r.fail(ctx, app, "CronRunFailed", err)
	}

	app.Status.Image = image
	app.Status.URL = "" // a cron_job has no serving URL
	app.Status.URLs = nil
	app.Status.Runs = runs
	app.Status.ActiveRevision = fmt.Sprintf("rev-%d", app.Generation)
	app.Status.ObservedGeneration = app.Generation
	if suspended {
		app.Status.Phase = appv1alpha1.PhaseHibernated
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "Suspended",
			Message: "cron schedule suspended", ObservedGeneration: app.Generation,
		})
	} else {
		app.Status.Phase = appv1alpha1.PhaseRunning
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionTrue, Reason: "Scheduled",
			Message: "cron scheduled: " + app.Spec.Schedule, ObservedGeneration: app.Generation,
		})
	}
	if err := r.Status().Update(ctx, app); err != nil {
		return ctrl.Result{}, err
	}
	logf.FromContext(ctx).Info("cron reconciled (kubernetes)", "name", app.Name, "schedule", app.Spec.Schedule, "runs", len(runs))
	if suspended {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// ensureManualRun creates the one-off Job for the current spec.runAt if it does
// not already exist. The Job carries labelApp (so it shows up in run history) and
// is owned by the App (so it is garbage-collected with it).
func (r *AppReconciler) ensureManualRun(ctx context.Context, app *appv1alpha1.App, image string, port int, labels map[string]string) error {
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: manualRunJobName(app.Name, app.Spec.RunAt), Namespace: app.Namespace,
	}}
	err := r.Get(ctx, client.ObjectKeyFromObject(job), job)
	if err == nil {
		return nil // already materialized this run
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	job.Labels = labels
	job.Spec.Template = cronPodSpec(app, image, port, labels)
	if err := controllerutil.SetControllerReference(app, job, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// cronRuns lists the Jobs backing an App's cron (by labelApp — both the CronJob's
// scheduled Jobs and one-off RunAt Jobs), newest first, capped at maxCronRuns.
func (r *AppReconciler) cronRuns(ctx context.Context, app *appv1alpha1.App) ([]appv1alpha1.CronRun, error) {
	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(app.Namespace), client.MatchingLabels{labelApp: app.Name}); err != nil {
		return nil, err
	}
	items := jobs.Items
	sort.Slice(items, func(i, j int) bool { return jobStartKey(&items[i]).After(jobStartKey(&items[j])) })
	if len(items) == 0 {
		return nil, nil
	}
	runs := make([]appv1alpha1.CronRun, 0, maxCronRuns)
	for i := range items {
		if len(runs) >= maxCronRuns {
			break
		}
		runs = append(runs, toCronRun(&items[i]))
	}
	return runs, nil
}

// jobStartKey orders runs by when they started, falling back to creation time for
// a Job the scheduler has not started yet.
func jobStartKey(job *batchv1.Job) time.Time {
	if job.Status.StartTime != nil {
		return job.Status.StartTime.Time
	}
	return job.CreationTimestamp.Time
}

// toCronRun projects a Job's status onto the CR's run-history shape.
func toCronRun(job *batchv1.Job) appv1alpha1.CronRun {
	run := appv1alpha1.CronRun{Name: job.Name, Status: "Running"}
	if job.Status.StartTime != nil {
		run.StartedAt = job.Status.StartTime.UTC().Format(time.RFC3339)
	}
	for _, c := range job.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		switch c.Type {
		case batchv1.JobComplete:
			run.Status = "Succeeded"
			run.FinishedAt = c.LastTransitionTime.UTC().Format(time.RFC3339)
		case batchv1.JobFailed:
			run.Status = "Failed"
			run.FinishedAt = c.LastTransitionTime.UTC().Format(time.RFC3339)
		}
	}
	if run.FinishedAt == "" && job.Status.CompletionTime != nil {
		run.FinishedAt = job.Status.CompletionTime.UTC().Format(time.RFC3339)
	}
	return run
}

// cronPodSpec is the pod template shared by the CronJob's jobTemplate and any
// one-off RunAt Job: the built image run to completion (RestartPolicyNever), with
// the App's env. A cron container declares no port — it is not an HTTP server.
func cronPodSpec(app *appv1alpha1.App, image string, port int, labels map[string]string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: labels},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:      "app",
				Image:     image,
				Env:       appEnv(app, port),
				EnvFrom:   envFromSources(app),
				Resources: resourcesForTier(app.Spec.Tier),
			}},
		},
	}
}

// manualRunJobName derives a deterministic Job name from the App name + runAt, so
// reconciling the same runAt twice never spawns a duplicate run.
func manualRunJobName(appName, runAt string) string {
	sum := sha256.Sum256([]byte(runAt))
	return fmt.Sprintf("%s-run-%x", appName, sum[:4])
}

// reconcileOpenSandbox runs the revision as an OpenSandbox sandbox (host runtime).
func (r *AppReconciler) reconcileOpenSandbox(ctx context.Context, app *appv1alpha1.App, image string, port int) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Suspend: real pause (checkpoint snapshot) on this runtime.
	if app.Spec.Suspended {
		if app.Status.SandboxID != "" {
			if err := r.Runtime.Pause(ctx, app.Status.SandboxID); err != nil {
				return r.fail(ctx, app, "PauseFailed", err)
			}
		}
		app.Status.Phase = appv1alpha1.PhaseHibernated
		app.Status.ObservedGeneration = app.Generation
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "Suspended",
			Message: "sandbox paused (snapshot kept)", ObservedGeneration: app.Generation,
		})
		if err := r.Status().Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Resume a paused sandbox instead of creating a new one. The host port
	// changes across pause/resume, so re-read the endpoint from the Target.
	if app.Status.Phase == appv1alpha1.PhaseHibernated && app.Status.SandboxID != "" {
		if target, err := r.Runtime.Resume(ctx, app.Status.SandboxID, port); err == nil {
			app.Status.Phase = appv1alpha1.PhaseRunning
			app.Status.Endpoint = fmt.Sprintf("%s:%d%s", target.Host, target.Port, target.Prefix)
			app.Status.URL = target.URL()
			app.Status.ObservedGeneration = app.Generation
			meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
				Type: "Ready", Status: metav1.ConditionTrue, Reason: "Resumed",
				Message: "sandbox resumed", ObservedGeneration: app.Generation,
			})
			if err := r.Status().Update(ctx, app); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("app resumed (opensandbox)", "name", app.Name, "url", app.Status.URL)
			return ctrl.Result{}, nil
		}
		log.Info("resume failed; creating a fresh sandbox", "name", app.Name)
	}

	r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Deploying", "Starting revision for "+image)
	old := app.Status.SandboxID
	id, err := r.Runtime.Create(ctx, image, port, nil, string(app.UID))
	if err != nil {
		if id != "" {
			_ = r.Runtime.Delete(ctx, id)
		}
		return r.fail(ctx, app, "DeployFailed", err)
	}
	target, err := r.Runtime.Endpoint(ctx, id, port)
	if err != nil {
		_ = r.Runtime.Delete(ctx, id)
		return r.fail(ctx, app, "EndpointFailed", err)
	}

	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.Image = image
	app.Status.SandboxID = id
	app.Status.Endpoint = fmt.Sprintf("%s:%d%s", target.Host, target.Port, target.Prefix)
	app.Status.URL = target.URL()
	app.Status.ActiveRevision = fmt.Sprintf("rev-%d", app.Generation)
	app.Status.ObservedGeneration = app.Generation
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Deployed",
		Message: "revision running", ObservedGeneration: app.Generation,
	})
	if err := r.Status().Update(ctx, app); err != nil {
		return ctrl.Result{}, err
	}
	if old != "" && old != id {
		_ = r.Runtime.Delete(ctx, old)
	}
	log.Info("app running (opensandbox)", "name", app.Name, "image", image, "url", app.Status.URL)
	return ctrl.Result{}, nil
}

// effectiveHosts resolves every external FQDN an App serves, in canonical order:
// spec.host first (the legacy single host — first position keeps existing Apps'
// TLS secret name stable), then the platform hostname "<name>.<BaseDomain>" when
// exposed, then spec.hosts (custom domains). Deduplicated, empties dropped.
func effectiveHosts(app *appv1alpha1.App, baseDomain string) []string {
	var hosts []string
	seen := map[string]bool{}
	add := func(h string) {
		if h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	add(app.Spec.Host)
	if app.Spec.Expose && baseDomain != "" {
		add(fmt.Sprintf("%s.%s", app.Name, baseDomain))
	}
	for _, h := range app.Spec.Hosts {
		add(h)
	}
	return hosts
}

// portEnvName is the operator-owned environment variable carrying the injected
// port — the one env invariant the CRD contract promises (see appEnv).
const portEnvName = "PORT"

// appEnv builds the container's environment: the user's spec.env (literal
// config) first, then the operator-owned PORT last. PORT is appended last and a
// user entry named PORT is dropped, so a user variable can never shadow the
// injected port — the one invariant the CRD contract promises. Secret-backed
// variables arrive separately through envFrom (envFromSources); a container-level
// Env entry overrides an envFrom key of the same name, so PORT wins over both.
func appEnv(app *appv1alpha1.App, port int) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0, len(app.Spec.Env)+1)
	for _, e := range app.Spec.Env {
		if e.Name == portEnvName {
			continue // operator-owned; never let a user shadow it
		}
		env = append(env, corev1.EnvVar{Name: e.Name, Value: e.Value})
	}
	env = append(env, corev1.EnvVar{Name: portEnvName, Value: strconv.Itoa(port)})
	return env
}

// envFromSources wires spec.envFromSecret to a Secret envFrom source — how the
// env-vars API's materialized "<name>-env" Secret (docs/secrets.md) reaches the
// container. Empty => no envFrom, unchanged behavior.
func envFromSources(app *appv1alpha1.App) []corev1.EnvFromSource {
	if app.Spec.EnvFromSecret == "" {
		return nil
	}
	return []corev1.EnvFromSource{{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: app.Spec.EnvFromSecret},
		},
	}}
}

// effectiveReplicas derives the Deployment size: spec.replicas (default 1),
// overridden to 0 while suspended. spec.replicas itself is never rewritten —
// it keeps meaning "how many when running", so resume knows what to restore.
func effectiveReplicas(app *appv1alpha1.App) int32 {
	if app.Spec.Suspended {
		return 0
	}
	if app.Spec.Replicas == 0 {
		return 1
	}
	return app.Spec.Replicas
}

// tlsSecretName gives each host its own certificate secret so one domain's
// failed issuance/renewal (e.g. a customer's deleted CNAME) can't block the
// others. The first host keeps the legacy "<app>-tls" name — renaming it would
// point the Ingress at an empty secret until cert-manager re-issues.
func tlsSecretName(appName string, i int, host string) string {
	if i == 0 {
		return appName + "-tls"
	}
	name := appName + "-tls-" + strings.ReplaceAll(host, "*", "wildcard")
	if len(name) > 253 { // secret names are RFC 1123 subdomains, max 253 chars
		sum := sha256.Sum256([]byte(host))
		name = fmt.Sprintf("%s-tls-%x", appName, sum[:8])
	}
	return name
}

func (r *AppReconciler) setPhase(ctx context.Context, app *appv1alpha1.App, p appv1alpha1.AppPhase, reason, msg string) {
	app.Status.Phase = p
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: reason, Message: msg,
		ObservedGeneration: app.Generation,
	})
	_ = r.Status().Update(ctx, app)
}

func (r *AppReconciler) fail(ctx context.Context, app *appv1alpha1.App, reason string, err error) (ctrl.Result, error) {
	app.Status.Phase = appv1alpha1.PhaseFailed
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: reason, Message: err.Error(),
		ObservedGeneration: app.Generation,
	})
	_ = r.Status().Update(ctx, app)
	return ctrl.Result{}, err
}

// reconcileNetworkPolicy creates or updates the per-App NetworkPolicy that
// enforces tenant isolation (docs/tenant-isolation.md §per-app-networkpolicy).
// The policy is skipped for Apps without the workspace label (legacy/hand-applied
// Apps) — they communicate freely, consistent with prior behavior.
//
// Policy shape:
//   - Ingress: allow from same-workspace pods + from the traefik namespace
//   - Egress: allow DNS (kube-system :53), same-workspace pods/services,
//     and the public internet (not RFC1918/CGNAT — blocks in-cluster platforms)
func (r *AppReconciler) reconcileNetworkPolicy(ctx context.Context, app *appv1alpha1.App) error {
	ws := app.Labels[labelWorkspace]
	np := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if ws == "" {
		// No workspace label: remove any stale NetworkPolicy we may have left from
		// a prior reconcile that had the label, then skip.
		if err := r.Delete(ctx, np); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	udpProto := corev1.ProtocolUDP
	tcpProto := corev1.ProtocolTCP
	dnsPort := intstr.FromInt(53)
	ingressClass := networkingv1.PolicyTypeIngress
	egressClass := networkingv1.PolicyTypeEgress
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, np, func() error {
		np.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{labelApp: app.Name},
			},
			PolicyTypes: []networkingv1.PolicyType{ingressClass, egressClass},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						// same-workspace pods (private services)
						{PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{labelWorkspace: ws},
						}},
						// Traefik ingress controller
						{NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"kubernetes.io/metadata.name": "traefik"},
						}},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				// DNS
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: &udpProto, Port: &dnsPort},
						{Protocol: &tcpProto, Port: &dnsPort},
					},
					To: []networkingv1.NetworkPolicyPeer{
						{NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"kubernetes.io/metadata.name": "kube-system"},
						}},
					},
				},
				// same-workspace pods and services
				{
					To: []networkingv1.NetworkPolicyPeer{
						{PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{labelWorkspace: ws},
						}},
					},
				},
				// public internet (not RFC1918 / CGNAT — blocks in-cluster platform services)
				{
					To: []networkingv1.NetworkPolicyPeer{
						{IPBlock: &networkingv1.IPBlock{
							CIDR: "0.0.0.0/0",
							Except: []string{
								"10.0.0.0/8",
								"172.16.0.0/12",
								"192.168.0.0/16",
								"100.64.0.0/10",
							},
						}},
					},
				},
			},
		}
		return controllerutil.SetControllerReference(app, np, r.Scheme)
	})
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Config propagation on restart: operator-level settings (cluster issuer, base
	// domain, tier ladder) come from env, so a config change always arrives as an
	// operator restart — and reaches every running App because informer replay
	// enqueues a Create per App once the cache syncs (GenerationChangedPredicate
	// only filters Updates). That invariant is pinned by startup_requeue_test.go;
	// this runnable only makes the fleet-wide pass visible in the logs.
	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		if !mgr.GetCache().WaitForCacheSync(ctx) {
			return nil // manager shutting down before the cache synced
		}
		var apps appv1alpha1.AppList
		if err := mgr.GetClient().List(ctx, &apps); err != nil {
			mgr.GetLogger().Error(err, "listing Apps at startup failed")
			return nil
		}
		mgr.GetLogger().Info("operator start: all Apps re-reconcile via informer replay", "count", len(apps.Items))
		return nil
	})); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.App{}).
		Owns(&appsv1.Deployment{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&batchv1.CronJob{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("app").
		Complete(r)
}
