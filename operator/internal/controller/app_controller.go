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
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appv1alpha1 "github.com/bex-co/bex/operator/api/v1alpha1"
	"github.com/bex-co/bex/operator/internal/build"
	bexruntime "github.com/bex-co/bex/operator/internal/runtime"
)

const finalizer = "app.bex.co/finalizer"

// labelApp marks the workloads bex creates for an App.
const labelApp = "app.bex.co/app"

// Runtime modes.
const (
	ModeOpenSandbox = "opensandbox" // run revisions as OpenSandbox sandboxes (host)
	ModeKubernetes  = "kubernetes"  // run revisions as k8s Deployments (pods on cluster nodes)
)

// AppReconciler reconciles an App: resolve an image (prebuilt or built from git)
// and run it as a revision on the selected runtime, recording status.
type AppReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Mode          string                  // ModeOpenSandbox | ModeKubernetes
	Registry      string                  // e.g. 127.0.0.1:5050
	CNBBuilder    string                  // e.g. paketobuildpacks/builder-jammy-base
	Runtime       *bexruntime.OpenSandbox // OpenSandbox client (ModeOpenSandbox)
	BaseDomain    string                  // optional: "<name>.<BaseDomain>" when Expose && Host=="" (e.g. bex.co)
	ClusterIssuer string                  // cert-manager ClusterIssuer for App Ingresses (letsencrypt-staging|-prod)
}

// +kubebuilder:rbac:groups=app.bex.co,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bex.co,resources=apps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.bex.co,resources=apps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

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
		res, err := build.Build(ctx, build.Options{
			Repo: app.Spec.Repo, Ref: branch, Name: app.Name,
			Registry: r.Registry, CNBBuilder: r.CNBBuilder,
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

// tierResources maps an App tier (plan) to a fixed pod allocation, set as
// requests == limits (Guaranteed). Empty/unknown tier => no constraints
// (best-effort, prior behavior); the control plane sets a tier explicitly.
// Ladder mirrors docs/control-plane.md.
var tierResources = map[string]struct{ cpu, mem string }{
	"free":      {"100m", "512Mi"},
	"starter":   {"500m", "512Mi"},
	"standard":  {"1", "2Gi"},
	"pro":       {"2", "4Gi"},
	"pro-plus":  {"4", "8Gi"},
	"pro-max":   {"4", "16Gi"},
	"pro-ultra": {"8", "32Gi"},
}

func resourcesForTier(tier string) corev1.ResourceRequirements {
	t, ok := tierResources[tier]
	if !ok {
		return corev1.ResourceRequirements{} // unset => best-effort, unchanged behavior
	}
	mk := func() corev1.ResourceList {
		return corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(t.cpu),
			corev1.ResourceMemory: resource.MustParse(t.mem),
		}
	}
	return corev1.ResourceRequirements{Requests: mk(), Limits: mk()}
}

// reconcileKubernetes runs the revision as a Deployment (+ ClusterIP k8s Service)
// owned by the App — pods are scheduled onto the cluster's nodes (machines).
func (r *AppReconciler) reconcileKubernetes(ctx context.Context, app *appv1alpha1.App, image string, port int) (ctrl.Result, error) {
	r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Deploying", "Reconciling Deployment for "+image)
	replicas := effectiveReplicas(app)
	labels := map[string]string{labelApp: app.Name}

	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		dep.Spec.Template.ObjectMeta.Labels = labels
		// restart = roll the template (same mechanism as kubectl rollout restart,
		// recorded in the contract). Never removed once set — removal would
		// itself roll the pods again.
		if app.Spec.RestartedAt != "" {
			if dep.Spec.Template.ObjectMeta.Annotations == nil {
				dep.Spec.Template.ObjectMeta.Annotations = map[string]string{}
			}
			dep.Spec.Template.ObjectMeta.Annotations["app.bex.co/restarted-at"] = app.Spec.RestartedAt
		}
		dep.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:      "app",
			Image:     image,
			Env:       []corev1.EnvVar{{Name: "PORT", Value: strconv.Itoa(port)}},
			Ports:     []corev1.ContainerPort{{ContainerPort: int32(port)}},
			Resources: resourcesForTier(app.Spec.Tier),
		}}
		return controllerutil.SetControllerReference(app, dep, r.Scheme)
	}); err != nil {
		return r.fail(ctx, app, "DeployFailed", err)
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
										Name: app.Name,
										Port: networkingv1.ServiceBackendPort{Number: int32(port)},
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
	return ctrl.Result{}, nil
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

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.App{}).
		Owns(&appsv1.Deployment{}).
		Owns(&networkingv1.Ingress{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("app").
		Complete(r)
}
