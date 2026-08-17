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
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	crbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	"github.com/bex-co/bex/lego/operator/internal/publish"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	bexruntime "github.com/bex-co/bex/lego/operator/internal/runtime"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	finalizer                    = "app.bex.co/finalizer"
	registryCredentialRotateTrue = "true"
	childHealthRequeue           = 30 * time.Second
	// settleRequeue is the short retry every reconciler in this package uses
	// while a dependency settles — an optimistic-conflict retry, a child still
	// being torn down, a Secret a controller has not derived yet.
	settleRequeue = 2 * time.Second

	// Ready-condition reasons for the two halves of a source build. They are a
	// shared vocabulary, not local strings: the control plane keys on them to
	// tell "waiting for capacity" from "building" and gives each its own phase
	// budget (lego/backend/internal/store/reconciler.go observedDeployStatus).
	// Changing either value silently re-times deploys, so they live here rather
	// than being written out at each call site.
	reasonBuildQueued = "BuildQueued"
	reasonBuilding    = "Building"
)

// generationOrDeletionPredicate adds App's explicit registry-credential
// rotation edge to the lifecycle-aware primary predicate shared with Database.
type generationOrDeletionPredicate struct {
	generationDeletionOrFinalizerPredicate
}

func (p generationOrDeletionPredicate) Update(e event.UpdateEvent) bool {
	rotationRequested := e.ObjectNew != nil &&
		e.ObjectNew.GetAnnotations()[annotRotateRegistryCreds] == registryCredentialRotateTrue &&
		(e.ObjectOld == nil || e.ObjectOld.GetAnnotations()[annotRotateRegistryCreds] != registryCredentialRotateTrue)
	return p.generationDeletionOrFinalizerPredicate.Update(e) ||
		rotationRequested
}

// labelApp marks the workloads bex creates for an App.
const labelApp = "app.bex.co/app"

// createContainerConfigError is the kubelet pod-condition reason the stuck-pod
// classifier surfaces as its own App condition (a referenced Secret/ConfigMap
// that cannot be resolved). Named once: it is matched and re-emitted.
const createContainerConfigError = "CreateContainerConfigError"

// defaultAppsNamespace mirrors BEX_APPS_NAMESPACE's default: the shared
// bootstrap apps namespace an unset field must never refuse.
const defaultAppsNamespace = "default"

// labelAppID is the immutable Render-shaped service id propagated from the App
// CR onto workload pods for node-local egress attribution. Kept in sync with
// backend/core.LabelAppID without importing the backend.
const labelAppID = "bex.co/app-id"

// labelRevision ties a pod to the App revision it was created for. The backend
// keeps the same string as core.PodLabelRevision without importing backend;
// SSH target discovery uses it to exclude a still-Ready old ReplicaSet during
// a rollout, including same-image restarts where image equality is insufficient.
const labelRevision = "app.bex.co/revision"

// labelHostRedirectOwner marks the per-App Traefik redirect middlewares so a
// level-triggered reconcile can remove entries whose source host stopped being
// an auto-paired sibling. The value is the App name (already a label-safe DNS
// label); owner references still provide lifecycle garbage collection.
const labelHostRedirectOwner = "app.bex.co/host-redirect-owner"

// labelWorkspace carries the owning workspace (tenant) id on pod templates so
// NetworkPolicy selectors can express "same-workspace" allow rules. Kept in
// sync by hand with core.LabelWorkspace in the backend — the operator never
// imports the backend (same pattern as labelApp / core.PodLabelApp).
const labelWorkspace = "app.bex.co/workspace"

// labelNetworkIsolation carries an App's environment id (w6/m19 protected-
// environment ACLs), present on the App CR ONLY when that environment has
// networkIsolationEnabled=true (environments.Service stamps/clears it —
// internal/environments/service.go's setAppEnvironmentLabel). Kept in sync by
// hand with core.LabelNetworkIsolation in the backend (same pattern as
// labelWorkspace / core.LabelWorkspace) — a distinct label from the backend's
// core.LabelEnvironment (w6/m20, unconditional Database/KeyValue environment
// membership; Apps don't carry that one at all, only their own environment_id
// store column). reconcileNetworkPolicy scopes the App's NetworkPolicy to
// same-environment peers instead of same-workspace ones when this label is
// present.
const labelNetworkIsolation = "app.bex.co/network-isolation"

// labelPlatformAliasPurpose marks the only ExternalName Services the operator
// may create in tenant namespaces. The API-server admission policy in
// deploy/gitops/base/operator-alias-admission.yaml requires this label, the
// exact platform target, and an App controller owner reference before admitting
// an ExternalName create/update from the manager ServiceAccount.
const labelPlatformAliasPurpose = "app.bex.co/platform-alias"

const (
	platformAliasStatic      = "static-server"
	platformAliasMaintenance = "maintenance"
)

// annotLastActive records when the app last served (or received) traffic.
// Set by the operator on first Running and reset on each wake; updated by the
// activator on each inbound request. Free-tier apps auto-hibernate after
// idleTTLSeconds past this timestamp.
const annotLastActive = "app.bex.co/last-active"

// annotTLSSecretHistory persists every cert-manager Secret name ever selected
// for an App. spec.hosts is mutable, so deletion cannot reconstruct removed
// custom-domain names from the final spec alone.
const annotTLSSecretHistory = "app.bex.co/tls-secret-history"

const annotStaticPurgeComplete = "app.bex.co/static-purge-complete"
const annotRegistryPurgeComplete = "app.bex.co/registry-purge-complete"

// Runtime modes.
const (
	ModeOpenSandbox = "opensandbox" // run revisions as OpenSandbox sandboxes (host)
	ModeKubernetes  = "kubernetes"  // run revisions as k8s Deployments (pods on cluster nodes)
	runtimeDocker   = "docker"
)

// AppReconciler reconciles an App: resolve an image (prebuilt or built from git)
// and run it as a revision on the selected runtime, recording status.
type AppReconciler struct {
	client.Client
	// BuildClient bypasses controller-runtime's shared cache for build-plane
	// objects. Secrets and ServiceAccounts are authorized only in build/app
	// namespaces; a cached client would try to establish forbidden cluster-wide
	// informers before performing a namespaced Get. Tests may leave it nil and
	// use Client.
	BuildClient client.Client
	Scheme      *runtime.Scheme
	// AppsNamespace is the shared/bootstrap tenant namespace (BEX_APPS_NAMESPACE,
	// default "default"). Together with each App's own workspace namespace it
	// defines the canonical set the reconciler will act on — see canonicalNamespace
	// and the codex #4 guard in Reconcile.
	AppsNamespace        string
	Mode                 string                  // ModeOpenSandbox | ModeKubernetes
	Registry             string                  // in-cluster registry, e.g. zot.bex-registry.svc:5000
	KpackRegistry        string                  // optional kpack alias for Registry (e.g. zot.local:5000 for plain HTTP)
	CNBBuilder           string                  // e.g. paketobuildpacks/builder-jammy-base
	BuildNamespace       string                  // namespace in-cluster build Jobs run in; empty => the App's namespace
	Runtime              *bexruntime.OpenSandbox // OpenSandbox client (ModeOpenSandbox)
	BaseDomain           string                  // optional: "<name>.<BaseDomain>" when Expose && Host=="" (e.g. bex.co)
	ClusterIssuer        string                  // cert-manager ClusterIssuer for App Ingresses (letsencrypt-staging|-prod)
	ActivatorService     string                  // k8s Service name of the wake activator; empty => auto-sleep disabled
	ActivatorPort        int                     // activator listen port (default 8888)
	MaintenanceService   string                  // shared public maintenance responder Service (default bex-activator)
	MaintenanceNamespace string                  // namespace of the shared responder Service (default bex-system)
	MaintenancePort      int                     // maintenance responder Service port (default 8888)
	// StaticStore is the object-store target for static_site publishing
	// (BEX_STATIC_S3_ENDPOINT/BUCKET + BEX_STATIC_PUBLISH_S3_SECRET).
	// Unconfigured => static_site Apps are rejected with a clear status, the way
	// unset backup vars disable backups.
	StaticStore publish.Store
	// StaticServerService is the k8s Service name of the shared static-server that
	// serves static_site content. A static-site host's Ingress backs onto an
	// App-owned ExternalName alias when the shared Service is in another
	// namespace. Empty => static_site serving is unavailable.
	StaticServerService string
	// StaticServerNamespace is the namespace of the shared static-server Service
	// (normally the operator's own namespace, default bex-system).
	StaticServerNamespace string
	// StaticServerPort is the static-server Service port (default 8080).
	StaticServerPort int
	// TenantSignKeySecret names a Secret (in the build namespace, keys
	// "cosign.key"+"cosign.password") that enables tenant-image signing in the
	// in-cluster build Job (w6/006). Empty => tenant images unsigned (the default).
	TenantSignKeySecret string
	// TenantSignImage overrides the cosign image used by the signing container.
	TenantSignImage string
	// RegistryPushSecret names the bootstrap/shared-fallback docker-config Secret
	// in the build namespace. PerAppRegistry supersedes it for tenant build
	// push/sign phases; the webhook may still use it to verify every tenant repo.
	// Empty => unauthenticated registry (dev default).
	RegistryPushSecret string
	// RegistryPullSecret names a docker-config Secret (in the apps namespace) the
	// operator attaches as an imagePullSecret to tenant Deployments/CronJobs so
	// kubelet pulls authenticate against an auth-enabled registry (w7/m8). Empty
	// => no imagePullSecret attached. Superseded by PerAppRegistry when set.
	RegistryPullSecret string
	// RegistryBuildPullSecret names a dockerconfigjson Secret in the BUILD namespace
	// used as the imagePullSecret for build-plane Jobs that pull the just-built
	// tenant image from Zot — today the static-site publish Job's extract
	// initContainer. The tenant pull secret (per-App reg-pull-<name> / shared
	// RegistryPullSecret) lives in the App namespace and is unreachable when the
	// build Job runs in a separate BEX_BUILD_NAMESPACE, so this is a distinct
	// build-ns credential (scripts/registry-secrets.sh's bex-registry-pull,
	// bex-builder with wildcard read). Empty => anonymous pull (dev default).
	RegistryBuildPullSecret string
	// PerAppRegistry, when non-nil, enables per-App Zot pull credentials (w7/m36,
	// docs/ADR022-tenant-isolation.md §Read policy). Each App gets its own
	// "app-<name>" htpasswd user scoped to its image repository, replacing the
	// shared bex-puller credential. Nil => shared RegistryPullSecret path (w7/m8).
	PerAppRegistry *registry.Creds
	// HTTPClient is the bounded shared client used for registry finalization.
	// Tests may inject a deterministic transport.
	HTTPClient *http.Client
	// MetricsReader reads live CPU/memory utilization from the metrics-server API.
	// When nil, spec.autoscaling is accepted in the CR but the autoscaling loop
	// is skipped (no replica adjustment). Tests inject a fake reader.
	MetricsReader MetricsReader
	// MaxConcurrentBuilds, when positive, caps how many active build Jobs a
	// workspace may have at once. When the workspace is at its limit, the
	// reconciler sets phase=Building/BuildQueued and requeues for 30s rather than
	// starting another build. 0 = unlimited (the default; byte-identical to before).
	// Has no effect for Apps without the workspace label (legacy/hand-applied).
	MaxConcurrentBuilds int
	// MaxConcurrentReconciles controls how many App reconcile loops may run at
	// once. Source builds wait synchronously for their Build Jobs, so the
	// controller-runtime default of one serializes otherwise independent Apps.
	// Values below 1 retain controller-runtime's safe default of one worker.
	MaxConcurrentReconciles int
}

// +kubebuilder:rbac:groups=app.bex.co,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bex.co,resources=apps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.bex.co,resources=apps/finalizers,verbs=update
// Per-App registry credential access (w7/m36): secrets in bex-registry namespace
// are granted via deploy/gitops/base/operator-registry-rbac.yaml (namespace-scoped
// Role, not this ClusterRole, because the kustomize namePrefix transform would
// rewrite the namespace field to bex-system).
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kpack.io,resources=images,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=kpack.io,resources=builds,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=traefik.io,resources=middlewares,verbs=get;list;watch;create;update;patch;delete

// traefikHTTPMiddlewareGVK is Traefik's HTTP middleware CRD (v3). Used to
// gate the App Ingress with an ipAllowList (source-range) middleware.
var traefikHTTPMiddlewareGVK = schema.GroupVersionKind{Group: "traefik.io", Version: "v1alpha1", Kind: "Middleware"}

const traefikRouterMiddlewaresAnnotation = "traefik.ingress.kubernetes.io/router.middlewares"

// legacyTraefikRouterMiddlewaresAnnotation is the pre-m30 nonstandard key. It is
// deleted on every Ingress write: Traefik's Kubernetes Ingress provider only
// reads the traefik.ingress.kubernetes.io namespace.
const legacyTraefikRouterMiddlewaresAnnotation = "traefik.io/router.middlewares"

const (
	ingressClassTraefik                = "traefik"
	certManagerClusterIssuerAnnotation = "cert-manager.io/cluster-issuer"
)

// traefikMiddlewareRef is how a Traefik Ingress annotation addresses a
// Middleware CR in a namespace.
func traefikMiddlewareRef(namespace, name string) string {
	return namespace + "-" + name + "@kubernetescrd"
}

// applyIngressRouting stamps the shape every App-owned Ingress shares: the
// cert-manager issuer, the Traefik middleware refs (or their removal), the
// legacy key's removal, the ingress class, and a cleared TLS/rule set ready for
// appendIngressHostRoute. The serving Ingress and the host-redirect Ingress both
// go through here so their routing contract cannot drift.
func (r *AppReconciler) applyIngressRouting(ing *networkingv1.Ingress, middlewareRefs []string) {
	if ing.Annotations == nil {
		ing.Annotations = map[string]string{}
	}
	if r.ClusterIssuer != "" {
		ing.Annotations[certManagerClusterIssuerAnnotation] = r.ClusterIssuer
	}
	if len(middlewareRefs) > 0 {
		ing.Annotations[traefikRouterMiddlewaresAnnotation] = strings.Join(middlewareRefs, ",")
	} else {
		delete(ing.Annotations, traefikRouterMiddlewaresAnnotation)
	}
	delete(ing.Annotations, legacyTraefikRouterMiddlewaresAnnotation)
	class := ingressClassTraefik
	ing.Spec.IngressClassName = &class
	ing.Spec.TLS = nil
	ing.Spec.Rules = nil
}

// appendIngressHostRoute adds one host's TLS entry plus its catch-all rule
// routing to svc:port. tlsIndex picks the host's TLS Secret name.
func appendIngressHostRoute(ing *networkingv1.Ingress, appName, host string, tlsIndex int, svc string, port int32) {
	pathType := networkingv1.PathTypePrefix
	ing.Spec.TLS = append(ing.Spec.TLS, networkingv1.IngressTLS{
		Hosts:      []string{host},
		SecretName: tlsSecretName(appName, tlsIndex, host),
	})
	ing.Spec.Rules = append(ing.Spec.Rules, networkingv1.IngressRule{
		Host: host,
		IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
			Paths: []networkingv1.HTTPIngressPath{{
				Path: "/", PathType: &pathType,
				Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
					Name: svc, Port: networkingv1.ServiceBackendPort{Number: port},
				}},
			}},
		}},
	})
}

// canonicalNamespace reports whether app lives in a namespace the reconciler is
// allowed to act on: the shared/bootstrap apps namespace (AppsNamespace, default
// "default") or its own per-workspace namespace. The control plane projects each
// App into WorkspaceNamespace(workspaceID)==workspaceID and stamps that same id as
// the app.bex.co/workspace label, so for every legitimate App the namespace equals
// one of those two. Any other namespace is a confused-deputy / cross-tenant write
// (codex #4).
func (r *AppReconciler) canonicalNamespace(app *appv1alpha1.App) bool {
	appsNS := r.AppsNamespace
	if appsNS == "" {
		appsNS = defaultAppsNamespace // an unset field never refuses bootstrap Apps
	}
	if app.Namespace == appsNS {
		return true
	}
	ws := app.Labels[labelWorkspace]
	return ws != "" && app.Namespace == ws
}

func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var app appv1alpha1.App
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Deletion: tear down external resources; the owned k8s Deployment/Service/
	// Ingress/CronJob/NetworkPolicy are garbage-collected via owner refs. The
	// finalizer cleans up artifacts that ownerRefs can't reach: cross-namespace
	// build artifacts, cert-manager TLS Secrets, static-site S3 content, and
	// registry images (w7/m12).
	if !app.DeletionTimestamp.IsZero() {
		return r.handleAppDeletion(ctx, &app)
	}

	// SECURITY (codex #4): the bex-api ServiceAccount can create App CRs
	// cluster-wide, so a compromised API pod could plant an App in a platform
	// namespace or another tenant's namespace and use this operator as a deputy to
	// run its image and resolve that namespace's Secrets. Fail closed: every
	// legitimately-projected App lives in its own workspace namespace (namespace ==
	// app.bex.co/workspace, since WorkspaceNamespace(id)==id) or the shared
	// bootstrap apps namespace. Anything else gets no finalizer, no child
	// workloads, and no requeue — the confused-deputy write is inert.
	if !r.canonicalNamespace(&app) {
		logf.FromContext(ctx).Info("refusing App outside a canonical tenant namespace (codex #4)",
			"appNamespace", app.Namespace, "workspaceLabel", app.Labels[labelWorkspace], "appsNamespace", r.AppsNamespace)
		return ctrl.Result{}, nil
	}

	if res, done, err := stampFinalizer(ctx, r.Client, &app, finalizer); done {
		return res, err
	}
	if err := r.recordTLSSecretHistory(ctx, &app); err != nil {
		return ctrl.Result{}, err
	}

	// codex F1: refuse to build any workload for an App that references an
	// operator-managed Secret marked protected-from-tenant-mount (the shared S3
	// backup credential the Database/KeyValue reconcilers project into a datastore
	// namespace). A co-located tenant App could otherwise mount it by name; kubelet
	// projection needs no pod-side API token, so pod hardening does not prevent it.
	if err := r.rejectProtectedSecretRefs(ctx, &app); err != nil {
		return r.fail(ctx, &app, "ProtectedSecretReference", err)
	}

	if res, halted, err := r.convergeRegistryCredentials(ctx, &app); halted {
		return res, err
	}

	// NOTE: intentionally NO early-return on ObservedGeneration==Generation here.
	// This controller is level-triggered: every reconcile re-applies desired state
	// (Deployment/Service/Ingress via CreateOrUpdate below) so operator-level config
	// changes — cluster issuer, base domain, tier — reach already-running Apps on the
	// next reconcile / operator restart, not only on a spec bump. Re-adding an
	// early-return here reintroduces the config-drift bug. The expensive build path is
	// gated separately (Status.Image reuse below), so a no-op pass never rebuilds.

	port := int(app.Spec.EffectivePort())

	// Classify the desired spec before resolving an image. Kubernetes generation
	// acknowledges every spec mutation; release identity records only mutations
	// that actually need a build/pre-deploy/rollout. Operational mutations such as
	// manual scale therefore keep using the active artifact and release.
	releaseDecision := prepareAppReleaseDecision(&app)
	if releaseDecision.canceled {
		return r.settleCanceledRelease(ctx, &app, port)
	}

	image, res, halted, err := r.resolveDeployImage(ctx, &app, releaseDecision, port)
	if halted {
		return res, err
	}
	return r.dispatchRuntime(ctx, &app, image, port)
}

// convergeRegistryCredentials mints this App's per-App pull credentials, mirrors
// them, and backfills them onto already-running workloads. Registry auth is
// operator-level configuration, so it converges before a source build can halt
// the pass: a failed newer build deliberately leaves the previous
// Deployment/CronJob serving, and without this early backfill that older
// workload could retain an unauthenticated pod template forever and fail as soon
// as Kubernetes reschedules it onto a fresh node. halted reports that the caller
// should return (res, err) as this reconcile's outcome.
func (r *AppReconciler) convergeRegistryCredentials(ctx context.Context, app *appv1alpha1.App) (res ctrl.Result, halted bool, err error) {
	if r.Mode != ModeKubernetes {
		return ctrl.Result{}, false, nil
	}
	// Per-App pull credentials (w7/m36): mint before the backfill so the
	// "reg-pull-<name>" Secret exists by the time backfill patches workloads.
	if err := r.ensurePerAppRegistryCreds(ctx, app); err != nil {
		if errors.Is(err, registry.ErrConflictRequeue) {
			return ctrl.Result{RequeueAfter: settleRequeue}, true, nil
		}
		res, err := r.fail(ctx, app, "PerAppRegistryCredsFailed", err)
		return res, true, err
	}
	if err := r.mirrorPerAppRegistryCredential(ctx, app); err != nil {
		res, err := r.fail(ctx, app, "PerAppRegistryCredsFailed", err)
		return res, true, err
	}
	if err := r.backfillWorkloadPullSecrets(ctx, app); err != nil {
		res, err := r.fail(ctx, app, "RegistryPullSecretFailed", err)
		return res, true, err
	}
	return ctrl.Result{}, false, nil
}

// settleCanceledRelease handles a generation whose build artifact the backend
// already deleted: preserve the last healthy release (if any) and acknowledge
// the current App generation without ever dispatching the canceled build again.
func (r *AppReconciler) settleCanceledRelease(ctx context.Context, app *appv1alpha1.App, port int) (ctrl.Result, error) {
	// Meter the user Cancel once. Reconciliation is level-triggered, so gate on
	// the first pass that processes this generation (ObservedGeneration is still
	// the previous release's until this pass acknowledges it below / in dispatch).
	// This keeps the canceled series a true count of user cancels — the tripwire
	// that must stay flat under supersede churn (ADR060 §D5).
	if app.Status.ObservedGeneration != app.Generation {
		recordBuildOutcome(buildOutcomeCanceled)
	}
	if app.Status.Image != "" {
		return r.dispatchRuntime(ctx, app, app.Status.Image, port)
	}
	app.Status.Phase = appv1alpha1.PhaseFailed
	app.Status.ObservedGeneration = app.Generation
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "BuildCanceled",
		Message: "build canceled before a release became available", ObservedGeneration: app.Generation,
	})
	return ctrl.Result{}, r.Status().Update(ctx, app)
}

// resolveDeployImage settles which image this pass deploys — prebuilt, a reused
// artifact, or one built from git — and stamps the resolved artifact identity on
// status. halted reports that the phase already produced this reconcile's
// terminal outcome (a direct static publish, or a build that halted the pass),
// so the caller must return (res, err) instead of dispatching.
func (r *AppReconciler) resolveDeployImage(ctx context.Context, app *appv1alpha1.App, decision appReleaseDecision, port int) (image string, res ctrl.Result, halted bool, err error) {
	image, artifactResolved := reusableArtifactImage(app, decision)
	// Direct static publish (w9/010): a repo-backed static_site with no
	// Dockerfile path, no build command, and no native runtime has nothing to
	// build — Render publishes the repo's publish directory as-is. Skip the
	// build plane entirely; reconcileStaticSite dispatches a clone-mode publish
	// Job (image stays ""). An explicit dockerfilePath/buildCommand/runtime/
	// builder keeps the build path (the built image's publishPath holds the
	// site).
	if directStaticPublish(app) {
		// Direct static publishing has no OCI image, but ActiveRevision remains the
		// success gate. Recording its source identity here is safe: a failed publish
		// keeps the old ActiveRevision and is retried on the next reconcile.
		app.Status.ArtifactFingerprint = decision.desired.artifact
		app.Status.ArtifactImage = ""
		if r.Mode != ModeKubernetes {
			res, err := r.fail(ctx, app, "BadSpec", fmt.Errorf("static_site requires the kubernetes runtime"))
			return "", res, true, err
		}
		res, err := r.reconcileKubernetes(ctx, app, "", port)
		return "", res, true, err
	}
	if image == "" {
		built, res, buildHalted, err := r.buildFromSource(ctx, app)
		if buildHalted {
			return "", res, true, err
		}
		image, artifactResolved = built, true
	}
	if artifactResolved {
		app.Status.ArtifactFingerprint = decision.desired.artifact
		app.Status.ArtifactImage = image
	}
	return image, ctrl.Result{}, false, nil
}

// dispatchRuntime hands the resolved image to the configured runtime.
func (r *AppReconciler) dispatchRuntime(ctx context.Context, app *appv1alpha1.App, image string, port int) (ctrl.Result, error) {
	if r.Mode == ModeKubernetes {
		return r.reconcileKubernetes(ctx, app, image, port)
	}
	return r.reconcileOpenSandbox(ctx, app, image, port)
}

// buildObserveRequeue is how often a reconcile re-reads an in-flight build's
// Job/Image while it runs — the non-blocking cadence internal/predeploy uses for
// pre-deploy Jobs (ADR060 §D1). A build takes minutes, so a few-second poll is
// cheap and frees the reconcile worker between reads.
const buildObserveRequeue = 5 * time.Second

// buildFromSource resolves the image for a repo-backed App by dispatching an
// in-cluster build and OBSERVING it without blocking (ADR060 §D1): one
// create-if-absent + one read per reconcile, requeuing while the build runs.
// halted=true means this reconcile pass must stop and return (res, err): the
// credential gate, the per-workspace cap, a still-running build (requeue), or a
// build failure has already recorded its status. The image is the only output on
// success.
//
// Supersede semantics (ADR060 §D1a): this never cancels a running build. The
// generation whose build is in flight is pinned by prepareAppReleaseDecision
// until it resolves, so releaseBuildRevision here is always the running build's
// revision — a newer push is coalesced into the pending slot and picked up once
// this build completes and the release advances.
func (r *AppReconciler) buildFromSource(ctx context.Context, app *appv1alpha1.App) (string, ctrl.Result, bool, error) {
	halt := func(res ctrl.Result, err error) (string, ctrl.Result, bool, error) {
		return "", res, true, err
	}
	if app.Spec.Repo == "" {
		return halt(r.fail(ctx, app, "BadSpec", fmt.Errorf("one of spec.image or spec.repo is required")))
	}
	ref := effectiveDeployRef(app.Spec)
	// Tag by release generation: a deploy trigger (including a webhook redeploy
	// that stamps spec.restartedAt) yields a new tag, while an operational spec
	// generation retains the current tag/artifact.
	//
	// A fresh tag makes the built image fresh and
	// the Deployment re-pulls it. An in-cluster BuildKit Job does the work — no
	// docker daemon on the node.
	buildNs := r.buildNamespace(app.Namespace)
	builder := effectiveBuilder(app.Spec)

	// The build's serial push phase uses this App's repository-scoped Zot
	// credential. Zot reads htpasswd and ACL Secrets only at startup, so a new
	// App's freshly minted credential may not be active yet. The workload gate
	// in reconcileKubernetes is necessarily too late for this path: the image
	// must be pushed before there is an image to roll out. Prove activation
	// before creating the build Job so a healthy build cannot fail at its final
	// push with a misleading BuildFailed/authentication-required error.
	if r.PerAppRegistry != nil && r.Registry != "" {
		if res, halted := r.awaitRegistryCredActive(ctx, app, appv1alpha1.PhaseBuilding,
			"registry credential probe failed before build; requeueing",
			"Waiting for the registry to accept this app's build credential"); halted {
			return halt(res, nil)
		}
	}

	buildClient := r.buildPlaneClient()

	// Per-workspace concurrent-build cap (w7/m9): hold off when the workspace is
	// at its limit — requeue until a slot opens rather than starting another.
	// The cap gates only a NEW dispatch: once builds are observed across many
	// reconciles (§D1), an App already building must never be stalled by the cap
	// (it would deadlock on its own build), so skip the gate when this App already
	// has an active build. Byte-identical when MaxConcurrentBuilds == 0 (unset) or
	// workspace absent.
	if r.MaxConcurrentBuilds > 0 {
		if workspace := app.Labels[labelWorkspace]; workspace != "" {
			mine, err := build.ActiveAppBuilds(ctx, app.Name, string(app.UID), buildNs, buildClient)
			if err != nil {
				return halt(r.fail(ctx, app, "BuildFailed", fmt.Errorf("counting app builds: %w", err)))
			}
			if mine == 0 {
				active, err := build.ActiveWorkspaceBuilds(ctx, workspace, buildNs, buildClient)
				if err != nil {
					return halt(r.fail(ctx, app, "BuildFailed", fmt.Errorf("counting workspace builds: %w", err)))
				}
				if active >= r.MaxConcurrentBuilds {
					r.setPhase(ctx, app, appv1alpha1.PhaseBuilding, reasonBuildQueued,
						fmt.Sprintf("workspace has %d/%d concurrent builds active; waiting for a slot", active, r.MaxConcurrentBuilds))
					return halt(ctrl.Result{RequeueAfter: 30 * time.Second}, nil)
				}
			}
		}
	}

	// The clone Secret (docs/ADR026-github-integration.md) that bex-api wrote lives
	// in the App's namespace, but the build Job runs in buildNs
	// (BEX_BUILD_NAMESPACE). When they differ, relocate it so BuildKit can read
	// GIT_AUTH_TOKEN — otherwise the build pod is CreateContainerConfigError
	// ("secret not found"). Mechanism-only: the operator just copies an opaque
	// Secret across namespaces; it never mints or inspects the token.
	if err := r.relocateBuildSecret(ctx, app, buildNs, app.Spec.CloneSecret, "clone secret"); err != nil {
		return halt(r.fail(ctx, app, "BuildFailed", err))
	}
	runtimeSecret := runtimeEnvSecret(app)
	if builder == build.BuilderNative {
		if err := r.relocateBuildSecret(ctx, app, buildNs, runtimeSecret, "native build env"); err != nil {
			return halt(r.fail(ctx, app, "BuildFailed", err))
		}
	}
	buildRegistryPullSecret, err := r.prepareBuildRegistrySecret(ctx, app, buildNs, builder)
	if err != nil {
		return halt(r.fail(ctx, app, "BuildFailed", fmt.Errorf("preparing Docker-build registry credential: %w", err)))
	}
	obs, err := build.EnsureBuild(ctx, build.Options{
		Repo: app.Spec.Repo, Ref: ref, RootDir: app.Spec.RootDir,
		DockerfilePath: app.Spec.DockerfilePath, DockerContext: app.Spec.DockerContext,
		Name: app.Name, AppUID: string(app.UID),
		Registry: r.Registry, KpackRegistry: r.KpackRegistry,
		Builder:          builder,
		Runtime:          app.Spec.Runtime,
		StaticSite:       app.Spec.Type == appv1alpha1.TypeStaticSite,
		BuildCommand:     app.Spec.BuildCommand,
		StartCommand:     app.Spec.StartCommand,
		BuildEnv:         buildEnv(builder, app.Spec.Env),
		RuntimeEnvSecret: runtimeSecret,
		Revision:         releaseBuildRevision(app),
		Namespace:        buildNs,
		AppNamespace:     app.Namespace,
		Workspace:        app.Labels[labelWorkspace],
		CloneSecret:      app.Spec.CloneSecret,
		SignKeySecret:    r.TenantSignKeySecret,
		SignImage:        r.TenantSignImage,
		PushSecret:       r.buildJobPushSecret(app),
		PullSecret:       buildRegistryPullSecret,
		RegistryConfig:   usesBuildRegistryConfig(app, builder),
		Client:           buildClient,
	})
	if err != nil {
		return halt(r.fail(ctx, app, "BuildFailed", fmt.Errorf("dispatching build: %w", err)))
	}
	switch obs.Phase {
	case build.PhaseSucceeded:
		recordBuildOutcome(buildOutcomeSucceeded)
		recordBuildRunSeconds(obs.RunSeconds)
		return r.pinBuiltImage(ctx, app, obs.Image), ctrl.Result{}, false, nil
	case build.PhaseFailed:
		recordBuildRunSeconds(obs.RunSeconds)
		if obs.Timeout {
			recordBuildOutcome(buildOutcomeTimeout)
			return halt(r.fail(ctx, app, "BuildFailed",
				fmt.Errorf("build exceeded its time limit: %s", obs.Message)))
		}
		recordBuildOutcome(buildOutcomeFailed)
		return halt(r.fail(ctx, app, "BuildFailed", fmt.Errorf("build failed: %s", obs.Message)))
	case build.PhaseWaiting:
		// Dispatched, but no pod placed yet — capacity wait. Report BuildQueued so
		// the control plane's build budget restarts on the queued→building edge
		// (a build that waits for a node must not spend that wait from its own
		// budget; the 2026-08-11 misattribution incident, ADR034 §3).
		r.setPhase(ctx, app, appv1alpha1.PhaseBuilding, reasonBuildQueued,
			"waiting for build capacity: "+obs.Message)
		return halt(ctrl.Result{RequeueAfter: buildObserveRequeue}, nil)
	default: // build.PhaseBuilding — a pod is placed and compiling
		r.setPhase(ctx, app, appv1alpha1.PhaseBuilding, reasonBuilding, "Building image from "+app.Spec.Repo)
		return halt(ctrl.Result{RequeueAfter: buildObserveRequeue}, nil)
	}
}

// pinBuiltImage rewrites a freshly built mutable-tag reference to its immutable
// `<repo>:<tag>@<digest>` form (w9/013). The per-release `gen-N` tag RESETS
// on delete+recreate of the same App name, so a node that cached the previous
// incarnation's `gen-1` could silently run stale code; pinning to the digest
// the registry serves right after this build's push makes every downstream
// consumer (Deployment, CronJob, static-site publish Job, rollback's
// resolvedImage) pull exactly the built bytes. Resolution failure falls back
// to the tag with a logged warning — behavior then matches the pre-w9/013
// state (dev clusters where the registry is unreachable from the operator).
func (r *AppReconciler) pinBuiltImage(ctx context.Context, app *appv1alpha1.App, image string) string {
	if strings.Contains(image, "@") {
		return image // kpack builds already return a digest-pinned reference
	}
	prefix := r.Registry + "/"
	if !strings.HasPrefix(image, prefix) {
		return image
	}
	repo, tag, ok := strings.Cut(strings.TrimPrefix(image, prefix), ":")
	if !ok {
		return image
	}
	digest, err := r.resolveBuiltDigest(ctx, app, repo, tag)
	if err != nil {
		logf.FromContext(ctx).Error(err, "resolving built image digest; using the mutable tag",
			"app", app.Name, "image", image)
		return image
	}
	return image + "@" + digest
}

// resolveBuiltDigest picks the credential the operator itself can use against
// the registry: the App's own per-App credential when per-App registry auth is
// on (the identity EnsureActive just proved accepted), else the shared pull
// secret's basic-auth pair, else anonymous (the dev default).
func (r *AppReconciler) resolveBuiltDigest(ctx context.Context, app *appv1alpha1.App, repo, tag string) (string, error) {
	if r.PerAppRegistry != nil {
		return r.PerAppRegistry.ResolveBuiltDigest(ctx, app.Name, app.Namespace, tag)
	}
	username, password := "", ""
	if r.RegistryPullSecret != "" {
		var sec corev1.Secret
		if err := r.buildPlaneClient().Get(ctx,
			client.ObjectKey{Namespace: app.Namespace, Name: r.RegistryPullSecret}, &sec); err == nil {
			username, password, _ = registry.BasicAuthFromDockerConfig(sec.Data[corev1.DockerConfigJsonKey], r.Registry)
		}
	}
	return registry.ResolveDigest(ctx, nil, r.Registry, repo, tag, username, password)
}

// pullPolicyFor sets the kubelet pull behavior to the image reference's
// immutability: a digest-pinned reference (image@sha256:…) is immutable so
// IfNotPresent suffices, but a mutable tag — the pinBuiltImage fallback when the
// registry is unreachable from the operator — MUST Always-pull, or a node can
// reuse a previous App lifetime's cached gen-N tag and run stale code
// (codex-security #29).
func pullPolicyFor(image string) corev1.PullPolicy {
	if strings.Contains(image, "@") {
		return corev1.PullIfNotPresent
	}
	return corev1.PullAlways
}

// directStaticPublish reports whether the App takes the no-build static publish
// path (w9/010): a repo-backed static_site whose spec declares no build input
// at all — no prebuilt image, no Dockerfile path, no build command, no native
// runtime, no explicit builder. Its publish Job clones the repo and uploads
// rootDir/publishPath as-is (Render's no-build static site); any declared
// build input keeps the build-then-extract path.
func directStaticPublish(app *appv1alpha1.App) bool {
	return app.Spec.Type == appv1alpha1.TypeStaticSite &&
		app.Spec.Image == "" && app.Spec.Repo != "" &&
		app.Spec.DockerfilePath == "" && app.Spec.BuildCommand == "" &&
		app.Spec.Runtime == "" &&
		(app.Spec.Builder == "" || app.Spec.Builder == build.BuilderAuto)
}

// effectiveBuilder keeps the App CR itself Render-compatible: direct CR users
// may set runtime/build/start without knowing bex's internal builder enum. The
// API performs the same projection before persistence, while this final mapping
// makes the mechanism robust for hand-applied Apps and older control planes.
func effectiveBuilder(spec appv1alpha1.AppSpec) string {
	// A static site with a declared buildCommand and no other strategy input
	// builds natively so the command actually runs — the auto/CNB path would
	// silently ignore it (ADR029; the API performs the same projection).
	if spec.Type == appv1alpha1.TypeStaticSite && strings.TrimSpace(spec.BuildCommand) != "" &&
		strings.TrimSpace(spec.DockerfilePath) == "" && spec.Runtime == "" &&
		(spec.Builder == "" || spec.Builder == build.BuilderAuto) {
		return build.BuilderNative
	}
	switch spec.Runtime {
	case runtimeDocker:
		return build.BuilderDockerfile
	case "elixir", "go", "node", "python", "ruby", "rust":
		return build.BuilderNative
	default:
		return spec.Builder
	}
}

// relocateBuildSecret copies one App-namespace Secret next to the build Job
// when the build runs in a different namespace. A missing name or a same-
// namespace build is a no-op, so callers need no guard of their own. label
// names the secret in the error a failure produces.
func (r *AppReconciler) relocateBuildSecret(ctx context.Context, app *appv1alpha1.App, buildNs, secretName, label string) error {
	if secretName == "" || buildNs == app.Namespace {
		return nil
	}
	if err := r.copyCloneSecret(ctx, app, app.Namespace, buildNs, secretName); err != nil {
		return fmt.Errorf("relocating %s to %s: %w", label, buildNs, err)
	}
	return nil
}

// effectiveDeployRef is the git ref a source clone checks out: the tracked
// branch, overridden for one build by a commitId from the deploy API
// (spec.BuildCommit). The next trigger without a commitId resets BuildCommit to
// "" via the API patch, so Branch HEAD is the default for every subsequent
// deploy. Both the build path and the static-site direct publish resolve
// through here — they are required to agree, and drifted while they were two
// copies.
func effectiveDeployRef(spec appv1alpha1.AppSpec) string {
	if spec.BuildCommit != "" {
		return spec.BuildCommit
	}
	if spec.Branch == "" {
		return appv1alpha1.DefaultBranch
	}
	return spec.Branch
}

// buildEnv selects literal App.spec.env entries for the active source builder.
// Native builds receive every literal (the OpenBao-backed bulk Secret travels
// separately as a BuildKit secret); kpack receives only its documented BP_ and
// BPE_ configuration. SecretKeyRef entries never enter a world-readable Image
// CR or generated Dockerfile.
func buildEnv(builder string, env []appv1alpha1.EnvVar) []corev1.EnvVar {
	out := make([]corev1.EnvVar, 0, len(env))
	for _, item := range env {
		if item.ValueFrom != nil {
			continue
		}
		if builder == build.BuilderBuildpack && !strings.HasPrefix(item.Name, "BP_") && !strings.HasPrefix(item.Name, "BPE_") {
			continue
		}
		out = append(out, corev1.EnvVar{Name: item.Name, Value: item.Value})
	}
	return out
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
	return cmp.Or(r.BuildNamespace, appNS)
}

// imagePullSecrets returns the imagePullSecrets a tenant pod carries so kubelet
// pulls authenticate. Two independent sources, both additive (w2/m14 extends,
// doesn't replace, w7/m8's internal-registry path):
//   - the internal-registry pull secret — per-App (w7/m36) when PerAppRegistry
//     is set, otherwise the shared RegistryPullSecret (w7/m8); included only
//     when the image is hosted in our registry, so the credential is never sent
//     to a foreign registry (docs/ADR022-tenant-isolation.md § Registry access
//     control);
//   - app.Spec.ExternalRegistryPullSecret, when bex-api has materialized one for
//     this App's image host (w2/m14) — the operator just references it by name,
//     unaware of which registry or credential it corresponds to.
//
// registryHosted reports whether image lives in the platform registry.
func (r *AppReconciler) registryHosted(image string) bool {
	return r.Registry != "" && strings.HasPrefix(image, r.Registry+"/")
}

func (r *AppReconciler) imagePullSecrets(app *appv1alpha1.App, image string) []corev1.LocalObjectReference {
	var out []corev1.LocalObjectReference
	if r.registryHosted(image) {
		if r.PerAppRegistry != nil {
			// Per-App pull secret (w7/m36): "reg-pull-<name>" in the app namespace.
			out = append(out, corev1.LocalObjectReference{Name: registry.PullSecretName(app.Name)})
		} else if r.RegistryPullSecret != "" {
			// Shared pull secret (w7/m8, byte-identical when PerAppRegistry is nil).
			out = append(out, corev1.LocalObjectReference{Name: r.RegistryPullSecret})
		}
	}
	if app.Spec.ExternalRegistryPullSecret != "" {
		out = append(out, corev1.LocalObjectReference{Name: app.Spec.ExternalRegistryPullSecret})
	}
	return out
}

// predeployPullSecrets returns the imagePullSecrets for the pre-deploy Job,
// which runs in the BUILD namespace — where the App-namespace tenant pull
// secrets imagePullSecrets() names are unreachable when BEX_BUILD_NAMESPACE
// differs (the w9/012 publish-Job defect's sibling). Same namespace ⇒ exactly
// imagePullSecrets() (byte-identical). Different namespace: a platform-registry
// image uses the build-namespace credential (buildJobPullSecret), and a foreign
// image's ExternalRegistryPullSecret — minted by bex-api in the App namespace —
// is relocated beside the Job first (the clone-secret relocation seam).
func (r *AppReconciler) predeployPullSecrets(ctx context.Context, app *appv1alpha1.App, image string) ([]corev1.LocalObjectReference, error) {
	ns := r.buildNamespace(app.Namespace)
	if ns == app.Namespace {
		return r.imagePullSecrets(app, image), nil
	}
	var out []corev1.LocalObjectReference
	if r.registryHosted(image) {
		if s := r.buildJobPullSecret(app); s != "" {
			out = append(out, corev1.LocalObjectReference{Name: s})
		}
	}
	if app.Spec.ExternalRegistryPullSecret != "" {
		if err := r.copyCloneSecret(ctx, app, app.Namespace, ns, app.Spec.ExternalRegistryPullSecret); err != nil {
			return nil, fmt.Errorf("relocating external registry pull secret to %s: %w", ns, err)
		}
		out = append(out, corev1.LocalObjectReference{Name: app.Spec.ExternalRegistryPullSecret})
	}
	return out, nil
}

// mirrorPreDeploySecrets copies every App-namespace Secret the pre-deploy pod
// references — the env Secrets a migration reads (spec.envFromSecret[s]) and
// any secret-file projections (spec.filesFromSecrets) — into the build
// namespace the Job runs in (w9/012: those refs are namespace-local, so
// without the copy the pod either stalls on its required env Secret or
// silently runs the migration without its optional env). No-op when the build
// namespace IS the App namespace. A Secret absent from the App namespace is
// skipped: the pod then behaves exactly as it would in-namespace (optional
// refs tolerate it, the required one fails the pod the same way).
// rejectProtectedSecretRefs fails the reconcile when a tenant App names an
// operational Secret marked LabelProtectedFromTenantMount in any of its
// secret-mount fields (codex F1), or names an operator-configured operational
// Secret directly (round-11 #1). It covers every kubelet-projected reference
// the App pod (or its pre-deploy Job) would resolve by name: spec.envFromSecret,
// spec.envFromSecrets[], spec.filesFromSecrets[], per-variable
// env[].valueFrom.secretKeyRef, AND the two pending-projection annotations
// (pending-env-secret, pending-files-secret) — runtimeEnvSecret and
// secretFileMounts project those names exactly like the spec fields, so leaving
// them out of the validator was a bypass. Reads use the uncached build-plane
// client so the lookup is reliable in per-tenant namespaces the manager does
// not cache; a referenced Secret that does not exist is not our concern here
// (it resolves to nothing / fails the pod on its own). Zero secret references
// => no lookups.
func (r *AppReconciler) rejectProtectedSecretRefs(ctx context.Context, app *appv1alpha1.App) error {
	names := make(map[string]bool)
	add := func(n string) {
		if n != "" {
			names[n] = true
		}
	}
	for _, n := range app.Spec.EnvFromSecrets {
		add(n)
	}
	for _, n := range app.Spec.FilesFromSecrets {
		add(n)
	}
	add(app.Spec.EnvFromSecret)
	for _, e := range app.Spec.Env {
		if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			add(e.ValueFrom.SecretKeyRef.Name)
		}
	}
	// The pending annotations are runtime Secret references too (see godoc);
	// they join the validated set so neither can smuggle a protected name.
	add(app.Annotations[appv1alpha1.PendingEnvSecretAnnotation])
	add(app.Annotations[appv1alpha1.PendingFilesSecretAnnotation])
	if len(names) == 0 {
		return nil
	}
	// Operator-configured operational Secrets (shared pull/push credentials,
	// build-ns pull credential, tenant signing key) are created out of band by
	// scripts, so no code path stamps the protected label on them — deny them
	// by NAME, without a lookup, whether or not the Secret exists yet. They
	// normally live in the build namespace, but with BEX_BUILD_NAMESPACE unset
	// that IS the App namespace. Only an actual App-side reference is denied;
	// configuring the name alone changes nothing.
	for _, n := range []string{r.RegistryPullSecret, r.RegistryPushSecret, r.RegistryBuildPullSecret, r.TenantSignKeySecret} {
		if n != "" && names[n] {
			return fmt.Errorf("app references operational Secret %q which tenant workloads may not mount (round-11 #1)", n)
		}
	}
	cl := r.buildPlaneClient()
	for name := range names {
		var sec corev1.Secret
		if err := cl.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, &sec); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("validating secret reference %s/%s: %w", app.Namespace, name, err)
		}
		if sec.Labels[execution.LabelProtectedFromTenantMount] == execution.ProtectedFromTenantMount {
			return fmt.Errorf("app references protected operator Secret %q which tenant workloads may not mount (codex F1)", name)
		}
	}
	return nil
}

func (r *AppReconciler) mirrorPreDeploySecrets(ctx context.Context, app *appv1alpha1.App, ns string) error {
	if ns == app.Namespace {
		return nil
	}
	names := append([]string{}, app.Spec.EnvFromSecrets...)
	names = append(names, app.Spec.FilesFromSecrets...)
	if app.Spec.EnvFromSecret != "" {
		names = append(names, app.Spec.EnvFromSecret)
	}
	// Per-variable secretKeyRefs too — this is how a Blueprint's
	// fromDatabase/fromService link is injected (DATABASE_URL and friends), and
	// a pre-deploy command is most often a database migration, so it is the one
	// step that most needs them. Without this the Job is created and the pod
	// fails at container-config time with `secret "<dpg-id>-app" not found`,
	// which surfaces only as a failed deploy with no reason attached.
	// appEnv renders these refs unchanged, so the Job resolves them in ITS
	// namespace; the network path is already open (reconcileExecutionNetworkPolicy).
	for _, e := range app.Spec.Env {
		if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			names = append(names, e.ValueFrom.SecretKeyRef.Name)
		}
	}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		err := r.copyCloneSecret(ctx, app, app.Namespace, ns, name)
		if err == nil {
			continue
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("relocating secret %s to %s: %w", name, ns, err)
		}
		// codex F4: the tenant-referenced Secret is absent in the App namespace, so
		// NO tenant-owned copy landed in the build namespace. Silently skipping here
		// (the pre-fix behavior) is safe ONLY when the name resolves to nothing —
		// but the pre-deploy Job runs in a SHARED build namespace, so if a
		// same-named Secret already exists there the pod's unchanged EnvFrom/volume
		// reference binds THAT object, which can be a platform Secret (e.g. the
		// static publisher credential) — leaking it to a tenant-controlled
		// preDeployCommand. Fail closed unless the pre-existing build-namespace
		// Secret is one this App itself owns (a benign stale copy). A name absent
		// from BOTH namespaces stays the tolerated optional case.
		var existing corev1.Secret
		getErr := r.buildPlaneClient().Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &existing)
		if getErr == nil {
			identity := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
				Workspace: app.Labels[labelWorkspace], Namespace: app.Namespace}
			if ownErr := identity.CheckOwner(&existing); ownErr != nil {
				return fmt.Errorf("pre-deploy secret %q is absent in App namespace %s but a foreign Secret of that name occupies the shared build namespace %s; refusing to run the pre-deploy Job against a non-tenant Secret (codex F4): %w",
					name, app.Namespace, ns, ownErr)
			}
			continue
		}
		if !apierrors.IsNotFound(getErr) {
			return fmt.Errorf("checking build-namespace secret %s/%s: %w", ns, name, getErr)
		}
	}
	return nil
}

// buildJobPullSecret returns the single imagePullSecret name a build-plane Job
// (the static-site publish Job) uses to pull the just-built, always platform-
// registry-hosted tenant image from Zot. When the build Job runs in a separate
// BEX_BUILD_NAMESPACE, the tenant pull secret (per-App reg-pull-<name> or shared
// RegistryPullSecret) lives in the App namespace and is NOT reachable from the
// build namespace, so use the build-namespace credential (RegistryBuildPullSecret).
// When the build and App namespaces coincide, the tenant pull secret is right
// here. Empty => anonymous pull (dev's unauthenticated Zot). This is the pull
// counterpart to the build Job's own build-namespace RegistryPushSecret.
func (r *AppReconciler) buildJobPullSecret(app *appv1alpha1.App) string {
	if r.buildNamespace(app.Namespace) != app.Namespace {
		if r.PerAppRegistry != nil {
			return registry.PullSecretName(app.Name)
		}
		return r.RegistryBuildPullSecret
	}
	if r.PerAppRegistry != nil {
		return registry.PullSecretName(app.Name)
	}
	return r.RegistryPullSecret
}

func (r *AppReconciler) buildJobPushSecret(app *appv1alpha1.App) string {
	if r.PerAppRegistry != nil {
		return registry.PullSecretName(app.Name)
	}
	return r.RegistryPushSecret
}

// backfillWorkloadPullSecrets converges existing long-running pod templates
// before image resolution/building. It is intentionally update-only: the normal
// workload reconcilers remain the single creation seam, while this closes the
// migration gap for an old revision that stays live because a newer source
// build or pre-deploy step fails. Jobs are immutable and revision-scoped; every
// newly generated manual/pre-deploy/static-publish Job already receives the
// secret at construction time.
func (r *AppReconciler) backfillWorkloadPullSecrets(ctx context.Context, app *appv1alpha1.App) error {
	if (r.RegistryPullSecret == "" && r.PerAppRegistry == nil) || r.Registry == "" {
		return nil
	}

	patch := func(obj client.Object, podSpec *corev1.PodSpec) error {
		if !metav1.IsControlledBy(obj, app) || len(podSpec.Containers) == 0 {
			return nil
		}
		image := podSpec.Containers[0].Image
		if !r.registryHosted(image) {
			return nil
		}
		desired := r.imagePullSecrets(app, image)
		if localObjectReferencesEqual(podSpec.ImagePullSecrets, desired) {
			return nil
		}
		before := obj.DeepCopyObject().(client.Object)
		podSpec.ImagePullSecrets = desired
		return r.Patch(ctx, obj, client.MergeFrom(before))
	}

	key := client.ObjectKey{Name: app.Name, Namespace: app.Namespace}
	var dep appsv1.Deployment
	if err := r.Get(ctx, key, &dep); err == nil {
		if err := patch(&dep, &dep.Spec.Template.Spec); err != nil {
			return fmt.Errorf("backfill Deployment %s/%s imagePullSecrets: %w", app.Namespace, app.Name, err)
		}
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get Deployment %s/%s for imagePullSecrets backfill: %w", app.Namespace, app.Name, err)
	}

	var cron batchv1.CronJob
	if err := r.Get(ctx, key, &cron); err == nil {
		if err := patch(&cron, &cron.Spec.JobTemplate.Spec.Template.Spec); err != nil {
			return fmt.Errorf("backfill CronJob %s/%s imagePullSecrets: %w", app.Namespace, app.Name, err)
		}
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get CronJob %s/%s for imagePullSecrets backfill: %w", app.Namespace, app.Name, err)
	}
	return nil
}

// annotRotateRegistryCreds is the annotation key that triggers per-App registry
// credential rotation. Set to "true" on the App CR; the reconciler calls
// RotateCreds then clears the annotation.
const annotRotateRegistryCreds = "bex.co/rotate-registry-creds"

// appNeedsRegistryCred reports whether the App needs a platform-registry
// credential: it will build+push to our registry (Repo-based) or already has a
// registry-hosted image in spec or status.
func (r *AppReconciler) appNeedsRegistryCred(app *appv1alpha1.App) bool {
	return app.Spec.Repo != "" || r.registryHosted(app.Spec.Image) || r.registryHosted(app.Status.Image)
}

// ensurePerAppRegistryCreds mints per-App pull credentials (w7/m36) when
// PerAppRegistry is configured and the App needs a registry-hosted image. It is
// a no-op when PerAppRegistry is nil or the App is not registry-hosted.
func (r *AppReconciler) ensurePerAppRegistryCreds(ctx context.Context, app *appv1alpha1.App) error {
	if r.PerAppRegistry == nil || r.Registry == "" {
		return nil
	}
	if !r.appNeedsRegistryCred(app) {
		return nil
	}

	// Rotation: if the annotation is set, re-issue credentials first, then clear it.
	if app.Annotations[annotRotateRegistryCreds] == registryCredentialRotateTrue {
		if err := r.PerAppRegistry.RotateCreds(ctx, app.Name, app.Namespace); err != nil {
			return fmt.Errorf("rotate registry creds: %w", err)
		}
		patch := app.DeepCopy()
		delete(patch.Annotations, annotRotateRegistryCreds)
		if err := r.Patch(ctx, patch, client.MergeFrom(app)); err != nil {
			return fmt.Errorf("clear rotate annotation: %w", err)
		}
		// Update in-memory copy so downstream logic sees cleared annotation.
		delete(app.Annotations, annotRotateRegistryCreds)
		logf.FromContext(ctx).Info("rotated per-app registry credentials", "app", app.Name)
	}

	return r.PerAppRegistry.EnsureCreds(ctx, app.Name, app.Namespace)
}

// mirrorPerAppRegistryCredential copies the per-App, repository-scoped Zot
// credential beside build/publish/pre-deploy workloads. This removes the shared
// bex-builder credential from every tenant-execution Pod while preserving the
// kubelet imagePullSecret in the App namespace.
func (r *AppReconciler) mirrorPerAppRegistryCredential(ctx context.Context, app *appv1alpha1.App) error {
	if r.PerAppRegistry == nil || r.Registry == "" || r.buildNamespace(app.Namespace) == app.Namespace {
		return nil
	}
	if !r.appNeedsRegistryCred(app) {
		return nil
	}
	name := registry.PullSecretName(app.Name)
	if err := r.copyBuildRegistryCredential(ctx, app, app.Namespace, r.buildNamespace(app.Namespace), name); err != nil {
		return fmt.Errorf("relocating per-App registry credential to %s: %w", r.buildNamespace(app.Namespace), err)
	}
	return nil
}

func localObjectReferencesEqual(a, b []corev1.LocalObjectReference) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

// copyCloneSecret relocates an opaque git-credential Secret from srcNS to dstNS
// so a build Job running in a separate BEX_BUILD_NAMESPACE can read it (the
// Secret is created by bex-api in the App's namespace). Idempotent: overwritten
// on every build, so the token stays fresh. Mechanism-only — the operator never
// reads the token, only copies the bytes.
func (r *AppReconciler) copyCloneSecret(ctx context.Context, app *appv1alpha1.App, srcNS, dstNS, name string) error {
	buildClient := r.buildPlaneClient()
	var src corev1.Secret
	if err := buildClient.Get(ctx, client.ObjectKey{Namespace: srcNS, Name: name}, &src); err != nil {
		return err
	}
	identity := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
		Workspace: app.Labels[labelWorkspace], Namespace: app.Namespace}
	dst := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: dstNS}}
	_, err := controllerutil.CreateOrUpdate(ctx, buildClient, dst, func() error {
		// A tenant-controlled spec field names the destination, so a same-named
		// Secret belonging to another App (or the platform) may already occupy it.
		// Refuse to overwrite a foreign owner rather than clobbering its bytes under
		// the operator's build-namespace Secret CRUD (codex-security #14).
		if dst.UID != "" {
			if err := identity.CheckOwner(dst); err != nil {
				return err
			}
		}
		dst.Type = src.Type
		dst.Data = src.Data
		dst.Labels = artifactLabels(app, "copied-secret")
		return nil
	})
	return err
}

// buildPlaneClient returns the uncached client used for build-plane resources.
// Tests and embedders that do not provide one retain the historical client.
func (r *AppReconciler) buildPlaneClient() client.Client {
	if r.BuildClient != nil {
		return r.BuildClient
	}
	return r.Client
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

// maintenanceEnabled reports whether the App has requested maintenance mode,
// nil-safe (unset spec.maintenanceMode == {enabled: false}).
func maintenanceEnabled(app *appv1alpha1.App) bool {
	return app.Spec.MaintenanceMode != nil && app.Spec.MaintenanceMode.Enabled
}

// reconcileKubernetes runs the revision as a Deployment (+ ClusterIP k8s Service)
// owned by the App — pods are scheduled onto the cluster's nodes (machines).
func (r *AppReconciler) reconcileKubernetes(ctx context.Context, app *appv1alpha1.App, image string, port int) (ctrl.Result, error) {
	if res, halted := r.deployRegistryGate(ctx, app, image); halted {
		return res, nil
	}

	// The slug-named Service converges bidirectionally for every type BEFORE the
	// per-type dispatch: an addressable App (web/private) gets the alias, a
	// non-addressable type (cron/static/worker — including one hand-edited away
	// from web/private) gets it removed. One seam owns both halves, so no type
	// path can forget the delete (docs/ADR041-service-addresses.md D2).
	if err := r.reconcileSlugService(ctx, app, port); err != nil {
		return r.fail(ctx, app, "ServiceFailed", err)
	}

	// A cron_job diverges entirely: a batch/v1 CronJob, no Deployment/Service/Ingress.
	if app.Spec.Type == appv1alpha1.TypeCronJob {
		return r.reconcileCronJob(ctx, app, image, port)
	}
	// A static_site diverges too: build → object store, then serve from the shared
	// static-server (no Deployment/Service for the served content).
	if app.Spec.Type == appv1alpha1.TypeStaticSite {
		return r.reconcileStaticSite(ctx, app, image)
	}
	// A background_worker runs the image with no HTTP port: a bare Deployment, no
	// Service, no Ingress, no URL, no auto-sleep (nothing routes traffic to wake it).
	worker := app.Spec.Type == appv1alpha1.TypeBackgroundWorker

	if rolloutPending(app, image) {
		r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Deploying", "Reconciling Deployment for "+image)
	}

	replicas, autoscaleRequeue, autoHibernating := r.desiredReplicas(ctx, app, worker)

	// Pre-deploy gate (w1/m33): run spec.preDeployCommand to completion against
	// the new revision's image before rolling the Deployment to it. A non-zero
	// exit fails the deploy and leaves the previous revision serving (the
	// Deployment below is never touched). Skipped when there is no rollout to
	// gate — suspended or auto-hibernating both scale to 0. When the step is still
	// running or has failed, reconcilePreDeploy halts this reconcile before the
	// Deployment update, which is what keeps the old revision live.
	if !app.Spec.Suspended && !autoHibernating {
		if res, halt, err := r.reconcilePreDeploy(ctx, app, image, port); halt || err != nil {
			return res, err
		}
	}

	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		applyDeploymentSpec(dep, app, deploymentParams{
			image:       image,
			port:        port,
			replicas:    replicas,
			worker:      worker,
			verifyImage: r.TenantSignKeySecret != "",
			// Authenticate kubelet pulls against the auth-enabled registry
			// (w7/m8). nil when no pull secret is configured or the image isn't
			// registry-hosted, so a prebuilt public image is left untouched.
			pullSecrets: r.imagePullSecrets(app, image),
		})
		return controllerutil.SetControllerReference(app, dep, r.Scheme)
	}); err != nil {
		return r.fail(ctx, app, "DeployFailed", err)
	}

	// Clean up any per-App NetworkPolicy a pre-ADR043 reconcile left behind
	// (docs/ADR022-tenant-isolation.md, superseded) — see reconcileNetworkPolicy.
	if err := r.reconcileNetworkPolicy(ctx, app); err != nil {
		return r.fail(ctx, app, "NetworkPolicyFailed", err)
	}

	// A worker skips the Service, the Ingress, and the URL entirely; any left over
	// from a type change are cleaned up so nothing dangles.
	if worker {
		return r.reconcileWorkerStatus(ctx, app, image, dep, replicas)
	}

	// the k8s core Service (ClusterIP) that fronts the App's pods
	if err := r.applyClusterIPService(ctx, app, app.Name, port); err != nil {
		return r.fail(ctx, app, "ServiceFailed", err)
	}

	// Optional external exposure: when any host resolves (spec.host, the computed
	// platform hostname, or spec.hosts custom domains), front the Service with one
	// Ingress carrying a rule + TLS certificate per host. No hosts => in-cluster
	// only, exactly as before. The operator emits a standard networking.k8s.io
	// Ingress, so the ingress controller (traefik today) stays swappable.
	hosts := effectiveHosts(app, r.BaseDomain)

	r.setPublicRoutingCondition(app, hosts)

	ingressSvc, ingressPort, err := r.ingressBackend(ctx, app, port, autoHibernating)
	if err != nil {
		return r.fail(ctx, app, "MaintenanceRoutingFailed", err)
	}

	if reason, err := r.reconcileIngressWithMiddlewares(ctx, app, hosts, ingressSvc, ingressPort); err != nil {
		return r.fail(ctx, app, reason, err)
	}

	if app.Spec.Suspended || autoHibernating {
		return r.parkKubernetes(ctx, app, image, hosts)
	}

	if res, halt, err := r.reportKubernetesRunning(ctx, app, dep, image, hosts, replicas, port); halt || err != nil {
		return res, err
	}

	return r.runningRequeue(ctx, app, autoscaleRequeue)
}

// deployRegistryGate holds a rollout until zot accepts this App's pull
// credential (w9/m43): zot only loads per-App credentials at process start (see
// registry/verify.go), so before rolling any workload (Deployment, CronJob,
// static-site publish Job) to a registry-hosted image, the credential must be
// proven live. Suspended Apps skip the gate: they scale to zero and pull nothing.
func (r *AppReconciler) deployRegistryGate(ctx context.Context, app *appv1alpha1.App, image string) (ctrl.Result, bool) {
	if r.PerAppRegistry == nil || app.Spec.Suspended || !r.registryHosted(image) {
		return ctrl.Result{}, false
	}
	return r.awaitRegistryCredActive(ctx, app, appv1alpha1.PhaseDeploying,
		"registry credential probe failed; requeueing",
		"Waiting for the registry to accept this app's pull credential")
}

// rolloutPending reports whether something is actually rolling out. A settled
// Running app on a steady-state pass (childHealthRequeue, pod events) must not
// flap Running→Deploying→Running, so the transitional phase is stamped only
// when this is true.
func rolloutPending(app *appv1alpha1.App, image string) bool {
	return app.Status.Phase != appv1alpha1.PhaseRunning ||
		app.Status.ObservedGeneration != app.Generation ||
		app.Status.Image != image
}

// parkKubernetes settles an App that serves nothing this pass.
// Suspended: parked at 0 replicas with Service/Ingress/TLS (and spec.replicas)
// all kept — resume is just scaling back. Report Hibernated and stop.
// Auto-hibernated: idle free-tier app scaled to 0, Ingress now points at the
// activator. The next inbound request will wake it; no further requeue needed.
func (r *AppReconciler) parkKubernetes(ctx context.Context, app *appv1alpha1.App, image string, hosts []string) (ctrl.Result, error) {
	reason, message := "Suspended", "suspended (scaled to 0; config, host and certs kept)"
	if !app.Spec.Suspended {
		reason = "AutoHibernated"
		message = fmt.Sprintf("idle ≥%ds on free tier; wakes on next request", app.Spec.IdleTTLSeconds)
	}
	res, err := r.hibernated(ctx, app, image, hosts, reason, message)
	if err == nil && reason == "AutoHibernated" {
		logf.FromContext(ctx).Info("app auto-hibernated", "name", app.Name, "idleTTL", app.Spec.IdleTTLSeconds)
	}
	return res, err
}

// desiredReplicas resolves the replica count reconcileKubernetes rolls the
// Deployment to, plus the flags for the two policies that can shape it
// (autoscale polling, auto-hibernation).
//
// Auto-hibernate: idle free-tier app past its TTL → scale to 0 without
// touching spec.suspended, so manual-suspend semantics are preserved. A worker
// never auto-hibernates — it has no Ingress, so a request could never wake it.
func (r *AppReconciler) desiredReplicas(ctx context.Context, app *appv1alpha1.App, worker bool) (replicas int32, autoscaleRequeue, autoHibernating bool) {
	autoHibernating = !worker && r.ActivatorService != "" && shouldAutoHibernate(app)

	replicas = effectiveReplicas(app)
	// Seed from the autoscaler annotation so a metrics-failure pass doesn't revert
	// to spec.replicas (the user's static count). applyAutoscaling writes the
	// annotation instead of spec.replicas to avoid bumping generation (see annotAutoscaleReplicas).
	if app.Spec.Autoscaling != nil && app.Spec.Autoscaling.Enabled && !worker && !app.Spec.Suspended {
		if raw := app.Annotations[annotAutoscaleReplicas]; raw != "" {
			if n, err := strconv.ParseInt(raw, 10, 32); err == nil && n > 0 {
				replicas = int32(n)
			}
		}
		replicas, autoscaleRequeue = r.applyAutoscaling(ctx, app, replicas)
	}
	if autoHibernating {
		replicas = 0
	}
	return replicas, autoscaleRequeue, autoHibernating
}

// ingressBackend picks the Service/port the public Ingress routes to.
// Maintenance has public-routing precedence over both auto-hibernation and
// manual suspension. It changes only the public Ingress backend; the workload
// follows its independent replica/suspension policy.
func (r *AppReconciler) ingressBackend(ctx context.Context, app *appv1alpha1.App, port int, autoHibernating bool) (string, int32, error) {
	if maintenanceEnabled(app) {
		maintenanceSvc, err := r.reconcileMaintenanceAlias(ctx, app)
		if err != nil {
			return "", 0, err
		}
		return maintenanceSvc, int32(r.maintenancePort()), nil
	}
	if autoHibernating {
		return r.ActivatorService, int32(r.ActivatorPort), nil
	}
	return app.Name, int32(port), nil
}

// reconcileIngressWithMiddlewares assembles the App's Traefik middlewares (IP
// allow-list + websocket meter) and applies the Ingress fronting svc:port. On
// error it returns the failure reason for the caller's r.fail.
func (r *AppReconciler) reconcileIngressWithMiddlewares(ctx context.Context, app *appv1alpha1.App, hosts []string, svc string, port int32) (string, error) {
	mwNames, err := r.reconcileIPAllowListMiddleware(ctx, app)
	if err != nil {
		return "MiddlewareFailed", err
	}
	wsMeter, err := r.reconcileWebsocketMeterMiddleware(ctx, app, len(hosts) > 0)
	if err != nil {
		return "MiddlewareFailed", err
	}
	middlewareNames := mwNames
	if wsMeter != "" {
		middlewareNames = append(middlewareNames, wsMeter)
	}
	if err := r.reconcileIngress(ctx, app, hosts, svc, port, middlewareNames); err != nil {
		return "IngressFailed", err
	}
	return "", nil
}

// reportKubernetesRunning reports the web/private path's rollout readiness and,
// once ready, stamps the Running status. halt=true means the caller must stop
// this reconcile and return (res, err) — the rollout is still progressing (or a
// status write failed); halt=false falls through to the running requeue policy.
func (r *AppReconciler) reportKubernetesRunning(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, image string, hosts []string, replicas int32, port int) (ctrl.Result, bool, error) {
	// Readiness: requeue until this Deployment revision, rather than retained
	// ready replicas from the previous ReplicaSet, has completed its rollout.
	_ = r.Get(ctx, client.ObjectKeyFromObject(dep), dep)
	completeAutoscalingTransition(app, dep.Status.Replicas, dep.Status.ReadyReplicas, time.Now())
	if !deploymentRolloutReady(dep, replicas) || !r.deploymentPodsReady(ctx, dep, replicas) {
		app.Status.Image = image
		res, err := r.reportRolloutProgress(ctx, app, dep, replicas, port,
			"waiting for the current Deployment revision and pods to become ready")
		return res, true, err
	}

	app.Status.Image = image
	if len(hosts) > 0 {
		setStatusURLs(app, hosts)
	} else {
		app.Status.URL = internalURL(app)
		app.Status.URLs = nil
	}
	app.Status.ObservedGeneration = app.Generation
	if err := r.markRunning(ctx, app, dep, replicas, "app running (kubernetes)",
		"name", app.Name, "image", image, "replicas", replicas); err != nil {
		return ctrl.Result{}, true, err
	}
	return ctrl.Result{}, false, nil
}

// runningRequeue picks the steady-state requeue for a Running app.
func (r *AppReconciler) runningRequeue(ctx context.Context, app *appv1alpha1.App, autoscaleRequeue bool) (ctrl.Result, error) {
	// Free-tier apps with an idle TTL: stamp last-active on first Running reconcile
	// and schedule a re-check after the TTL so the operator can auto-hibernate.
	if r.ActivatorService != "" && app.Spec.IdleTTLSeconds > 0 && isFreeApp(app) {
		now := time.Now().UTC()
		if lastActiveTime(app).IsZero() {
			base := app.DeepCopy()
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, annotLastActive, now.Format(time.RFC3339))
			if err := r.Patch(ctx, app, client.MergeFrom(base)); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: idleRequeueAfter(app, now)}, nil
	}
	// Autoscaling: always requeue at the poll interval so the loop keeps running.
	if autoscaleRequeue {
		return ctrl.Result{RequeueAfter: autoscaleInterval}, nil
	}
	return ctrl.Result{RequeueAfter: childHealthRequeue}, nil
}

// hibernated reports an App parked at 0 replicas with its Service, Ingress and
// certificates intact. Manual suspension and idle auto-hibernation reach the
// same resting state and differ only in why, so they share one terminal status
// write rather than two that can drift apart.
func (r *AppReconciler) hibernated(ctx context.Context, app *appv1alpha1.App, image string, hosts []string, reason, message string) (ctrl.Result, error) {
	app.Status.Phase = appv1alpha1.PhaseHibernated
	app.Status.Image = image
	if len(hosts) > 0 {
		app.Status.URL = "https://" + hosts[0]
	}
	app.Status.ObservedGeneration = app.Generation
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason,
		Message: message, ObservedGeneration: app.Generation,
	})
	if r.statusSettled(ctx, app) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Update(ctx, app)
}

// markRunning stamps the Running terminal shared by the web and worker paths:
// phase Running, the release revision as active, and Ready=True with the
// replica tally. URL/Image bookkeeping stays with each caller, as does the log
// line, so the two paths keep their distinct messages.
func (r *AppReconciler) markRunning(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, replicas int32, logMsg string, logKVs ...any) error {
	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.ActiveRevision = releaseRevision(app)
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Deployed",
		Message:            fmt.Sprintf("%d/%d replicas ready", dep.Status.ReadyReplicas, replicas),
		ObservedGeneration: app.Generation,
	})
	// A settled Running app re-stamps identical status on every steady-state
	// pass (childHealthRequeue): skip the no-op PUT and its log line.
	if !r.statusSettled(ctx, app) {
		if err := r.Status().Update(ctx, app); err != nil {
			return err
		}
		logf.FromContext(ctx).Info(logMsg, logKVs...)
	}
	return nil
}

// deleteStaleChildren removes leftover children a type change orphaned.
func (r *AppReconciler) deleteStaleChildren(ctx context.Context, objs ...client.Object) error {
	for _, obj := range objs {
		// Cached existence check first: both objects are absent on every
		// steady-state pass, and a blind Delete is a live API round trip.
		if err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func terminationGracePeriodSeconds(seconds *int32) *int64 {
	if seconds == nil {
		return nil
	}
	value := int64(*seconds)
	return &value
}

// applyClusterIPService creates/updates one ClusterIP Service named name over
// the App's pods (the stable labelApp selector) on the App's port. Shared by
// the primary CR-named Service and the slug alias so the two cannot drift.
// Protocol is set explicitly so the mutate matches the server-defaulted object
// and steady-state reconciles stay read-only (no perpetual no-op PUT).
func (r *AppReconciler) applyClusterIPService(ctx context.Context, app *appv1alpha1.App, name string, port int) error {
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Selector = map[string]string{labelApp: app.Name}
		svc.Spec.Ports = []corev1.ServicePort{{
			Port: int32(port), TargetPort: intstr.FromInt(port), Protocol: corev1.ProtocolTCP,
		}}
		return controllerutil.SetControllerReference(app, svc, r.Scheme)
	})
	return err
}

// reconcileSlugService converges the slug-named ClusterIP Service that gives an
// addressable App (web/private) its Render-shaped internal address
// `<slug>:<port>` (docs/ADR041-service-addresses.md D2): present exactly when
// the App is addressable and its slug differs from the CR name, absent
// otherwise — including after a hand-edit to a non-addressable type, so the
// delete half cannot be forgotten by a type path. The CR-named Service stays
// the primary object; this one only adds the slug DNS identity over the same
// pods. If the slug name is squatted by an object owned by someone else (a
// hand-applied CR whose *name* equals this App's slug — the store cannot see
// such CRs when minting slugs), the alias is skipped with a log rather than
// failing the deploy; deletion likewise touches only an object this App
// controls.
func (r *AppReconciler) reconcileSlugService(ctx context.Context, app *appv1alpha1.App, port int) error {
	slug := app.Spec.PlatformSubdomain(app.Name)
	if slug == app.Name {
		return nil // the CR-named Service already answers this hostname
	}
	if !app.Spec.InternallyAddressable() {
		svc := &corev1.Service{}
		if err := r.Get(ctx, client.ObjectKey{Name: slug, Namespace: app.Namespace}, svc); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if !metav1.IsControlledBy(svc, app) {
			return nil
		}
		return r.Delete(ctx, svc)
	}
	err := r.applyClusterIPService(ctx, app, slug, port)
	var owned *controllerutil.AlreadyOwnedError
	if errors.As(err, &owned) {
		logf.FromContext(ctx).Info("slug Service name already owned by another object; skipping the alias",
			"app", app.Name, "slug", slug)
		return nil
	}
	return err
}

// internalURL is the address surfaced when an App has no public host (private
// services). An App whose slug differs from its CR name (store-minted, w4/m19)
// gets the Render-shaped `http://` + spec.InternalAddress (the contract-level
// `<slug>:<port>` in types/v1alpha1, shared with bex-api's surfaced field) —
// resolvable via the slug Service; otherwise (legacy and adopted Apps, where
// the slug falls back to or equals the CR name) the fully-qualified
// in-namespace form is kept, byte-identical to prior behavior (ADR041 D1).
// The predicate is deliberately the same `slug != app.Name` its sibling
// reconcileSlugService uses, so the surfaced URL flips to the slug form only
// when the slug Service actually exists.
func internalURL(app *appv1alpha1.App) string {
	if slug := app.Spec.PlatformSubdomain(app.Name); slug != app.Name {
		return "http://" + app.Spec.InternalAddress(app.Name)
	}
	return fmt.Sprintf("http://%s.%s.svc:%d", app.Name, app.Namespace, app.Spec.EffectivePort())
}

// setStatusURLs projects a serving App's public hosts onto status: the first
// host is the canonical URL, the full list lands in URLs. hosts must be
// non-empty; the no-host cases (internal URL, no URL at all) stay per-type.
func setStatusURLs(app *appv1alpha1.App, hosts []string) {
	app.Status.URL = "https://" + hosts[0]
	app.Status.URLs = nil
	for _, h := range hosts {
		app.Status.URLs = append(app.Status.URLs, "https://"+h)
	}
}

// reconcileWorkerStatus finishes the background_worker reconcile: a worker's
// Deployment is already applied by reconcileKubernetes, so here we tear down any
// Service/Ingress left from a prior type, then report status with no URL (a
// worker has no HTTP endpoint). It shares the Deployment/readiness shape with the
// web path but skips exposure and the auto-sleep machinery entirely.
func (r *AppReconciler) reconcileWorkerStatus(ctx context.Context, app *appv1alpha1.App, image string, dep *appsv1.Deployment, replicas int32) (ctrl.Result, error) {
	if err := r.deleteStaleChildren(ctx,
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}},
	); err != nil {
		return r.fail(ctx, app, "WorkerCleanupFailed", err)
	}

	app.Status.Image = image
	app.Status.URL = "" // a worker has no HTTP endpoint
	app.Status.URLs = nil
	app.Status.ObservedGeneration = app.Generation

	if app.Spec.Suspended {
		app.Status.Phase = appv1alpha1.PhaseHibernated
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "Suspended",
			Message: "worker suspended (scaled to 0)", ObservedGeneration: app.Generation,
		})
		if err := r.Status().Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	_ = r.Get(ctx, client.ObjectKeyFromObject(dep), dep)
	if !deploymentRolloutReady(dep, replicas) || !r.deploymentPodsReady(ctx, dep, replicas) {
		// Port 0 — a worker has no HTTP endpoint, so the $PORT hint is omitted (w9/011).
		return r.reportRolloutProgress(ctx, app, dep, replicas, 0,
			"waiting for the current worker Deployment revision and pods to become ready")
	}

	if err := r.markRunning(ctx, app, dep, replicas, "worker running (kubernetes)",
		"name", app.Name, "image", image, "replicas", replicas); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: childHealthRequeue}, nil
}

// reportRolloutProgress records the shared not-ready shape of a Deployment
// rollout (the web and worker paths): phase Deploying, Ready=False with
// RolloutProgressing — upgraded to RolloutSettling when a live pod listing
// shows the current revision fully ready while the Deployment status is still
// converging — then overlaid with the stuck-pod diagnosis when a pod is
// crash-looping or cannot pull its image: the deploy would otherwise time out
// as an opaque update_failed with the cause visible only in pod state no
// tenant surface shows (w9/011).
func (r *AppReconciler) reportRolloutProgress(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, replicas int32, port int, notReadyMessage string) (ctrl.Result, error) {
	app.Status.Phase = appv1alpha1.PhaseDeploying
	notReadyReason := "RolloutProgressing"
	if r.currentRevisionFullyReady(ctx, dep, replicas) {
		notReadyReason, notReadyMessage = "RolloutSettling", "current revision fully ready; Deployment status still converging"
	}
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: notReadyReason,
		Message: notReadyMessage, ObservedGeneration: app.Generation,
	})
	if reason, msg := r.stuckPodMessage(ctx, dep, port); msg != "" {
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason,
			Message: msg, ObservedGeneration: app.Generation,
		})
	}
	_ = r.Status().Update(ctx, app)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// deploymentRolloutReady follows the Deployment controller's rollout-complete
// shape. ReadyReplicas alone is insufficient: during a failed rolling update,
// the old ReplicaSet can keep the desired number ready while every pod in the
// new revision is stuck in ErrImagePull.
func deploymentRolloutReady(dep *appsv1.Deployment, replicas int32) bool {
	return dep.Status.ObservedGeneration >= dep.Generation &&
		dep.Status.UpdatedReplicas == replicas &&
		dep.Status.Replicas == replicas &&
		dep.Status.AvailableReplicas >= replicas
}

// deploymentPodsReady closes the window where a Pod has already crashed but
// the Deployment controller has not yet lowered availableReplicas. A missing
// pod list is treated as inconclusive so startup/fake clients still rely on the
// authoritative Deployment status; once current-revision pods are visible,
// every desired replica must have PodReady=true.
func (r *AppReconciler) deploymentPodsReady(ctx context.Context, dep *appsv1.Deployment, replicas int32) bool {
	if replicas == 0 || dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return true
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return true
	}
	revision := dep.Spec.Template.Labels[labelRevision]
	var current, ready int32
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !pod.DeletionTimestamp.IsZero() || (revision != "" && pod.Labels[labelRevision] != revision) {
			continue
		}
		current++
		if podReady(pod) {
			ready++
		}
	}
	return current == 0 || ready >= replicas
}

// currentRevisionFullyReady reports that a LIVE pod listing shows the full
// desired complement of current-revision, non-terminating pods PodReady. It is
// the truthful counterpart to deploymentRolloutReady's status-based view: when
// this holds while the Deployment status has not converged (stale bookkeeping
// as kube-controller-manager catches up or reaps the old ReplicaSet), the
// service is serving, so Ready=False must carry RolloutSettling rather than
// RolloutProgressing — observed-state consumers exclude RolloutSettling from
// availability (w3/m78: a phantom server_failed page otherwise fires on every
// slow old-pod reap). Unlike deploymentPodsReady, a failed or empty listing is
// NOT inconclusive-true: no visible current pods means not fully ready.
func (r *AppReconciler) currentRevisionFullyReady(ctx context.Context, dep *appsv1.Deployment, replicas int32) bool {
	if replicas == 0 || dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return false
	}
	revision := dep.Spec.Template.Labels[labelRevision]
	if revision == "" {
		return false
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return false
	}
	var ready int32
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !pod.DeletionTimestamp.IsZero() || pod.Labels[labelRevision] != revision {
			continue
		}
		if podReady(pod) {
			ready++
		}
	}
	return ready >= replicas
}

// reconcileIPAllowListMiddleware creates or removes the Traefik HTTP
// Middlewares for the App's inbound-IP layers: its own ipAllowList
// ("<app>-ip-allow") and the environment-projected environmentIPAllowList
// ("<app>-env-ip-allow", w4/m28). Returns the middleware names to CHAIN on
// the Ingress annotation — Traefik ANDs chained middlewares, so a source must
// pass every present layer (Render's every-applicable-rule intersection) —
// or nil when no layer is set.
func (r *AppReconciler) reconcileIPAllowListMiddleware(ctx context.Context, app *appv1alpha1.App) ([]string, error) {
	var names []string
	for _, layer := range []struct {
		suffix string
		cidrs  []string
	}{
		{"-ip-allow", app.Spec.EffectiveIPAllowListCIDRs()},
		{"-env-ip-allow", app.Spec.EnvironmentIPAllowList},
	} {
		mwName := app.Name + layer.suffix
		if len(layer.cidrs) > 0 {
			if err := upsertOwned(ctx, r.Client, r.Scheme, app, traefikHTTPMiddlewareGVK, mwName, cidrMiddlewareSpec(layer.cidrs)); err != nil {
				return nil, err
			}
			names = append(names, mwName)
			continue
		}
		if err := deleteOwned(ctx, r.Client, app, traefikHTTPMiddlewareGVK, mwName); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// appIDOrName is the Render-shaped service id stamped on the App CR, falling
// back to the CR name for legacy/hand-applied Apps that carry no id label.
func appIDOrName(app *appv1alpha1.App) string {
	return cmp.Or(app.Labels[labelAppID], app.Name)
}

func (r *AppReconciler) reconcileWebsocketMeterMiddleware(ctx context.Context, app *appv1alpha1.App, enabled bool) (string, error) {
	middlewareName := app.Name + "-ws-egress"
	if !enabled {
		if err := deleteOwned(ctx, r.Client, app, traefikHTTPMiddlewareGVK, middlewareName); err != nil {
			return "", err
		}
		return "", nil
	}
	spec := map[string]any{"plugin": map[string]any{
		"bexWebsocketEgress": map[string]any{"appId": appIDOrName(app), "metricsAddr": ":9101"},
	}}
	if err := upsertOwned(ctx, r.Client, r.Scheme, app, traefikHTTPMiddlewareGVK, middlewareName, spec); err != nil {
		return "", err
	}
	return middlewareName, nil
}

// reconcileIngress applies one networking.k8s.io Ingress fronting the App's
// hosts, each with a rule + cert-manager TLS certificate, routed to backend
// svc:port. With no hosts it removes any Ingress previously created (exposure
// turned off). The backend is parameterized so a web/private service points at
// its own Service, an auto-hibernating app at the activator, and a static_site
// at the shared static-server — all through the same emitted Ingress shape, so
// the ingress controller (traefik today) stays swappable. middlewareNames,
// when non-empty, wire Traefik HTTP Middlewares (the ipAllowList layers and
// WebSocket egress meter) via
// the router.middlewares Ingress annotation — comma-joined, which Traefik
// applies as an AND chain.
func (r *AppReconciler) reconcileIngress(ctx context.Context, app *appv1alpha1.App, hosts []string, svc string, port int32, middlewareNames []string) error {
	redirectSources, err := r.reconcileHostRedirects(ctx, app, hosts, svc, port)
	if err != nil {
		return err
	}
	if len(hosts) == 0 {
		stale := &networkingv1.Ingress{}
		if err := r.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, stale); err != nil {
			return client.IgnoreNotFound(err) // cached read: absent on every steady-state pass
		}
		if err := r.Delete(ctx, stale); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	refs := make([]string, len(middlewareNames))
	for i, name := range middlewareNames {
		refs[i] = traefikMiddlewareRef(app.Namespace, name)
	}
	ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, ing, func() error {
		r.applyIngressRouting(ing, refs)
		for i, host := range hosts {
			if redirectSources[host] {
				continue
			}
			appendIngressHostRoute(ing, app.Name, host, i, svc, port)
		}
		return controllerutil.SetControllerReference(app, ing, r.Scheme)
	})
	return err
}

// effectiveHostRedirects returns only well-formed source->target entries whose
// hosts both exist in the effective host set. Invalid or stale intent degrades
// to direct serving instead of creating a redirect to an unreachable host.
func effectiveHostRedirects(app *appv1alpha1.App, hosts []string) map[string]string {
	present := make(map[string]bool, len(hosts))
	for _, host := range hosts {
		present[host] = true
	}
	out := map[string]string{}
	for source, target := range app.Spec.HostRedirects {
		if source != "" && target != "" && source != target && present[source] && present[target] {
			out[source] = target
		}
	}
	return out
}

// redirectRegexHTTPMiddlewareSpec builds the Traefik RedirectRegex that pins
// Render's captured sibling behavior: permanent 301, HTTPS canonical target,
// and the complete path/query suffix preserved by the single capture group.
func redirectRegexHTTPMiddlewareSpec(source, target string) map[string]any {
	return map[string]any{"redirectRegex": map[string]any{
		"regex":       `^https?://` + regexp.QuoteMeta(source) + `/(.*)`,
		"replacement": `https://` + target + `/${1}`,
		"permanent":   true,
	}}
}

func hostRedirectResourceName(appName, source string) string {
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(source)))[:10]
	suffix := "-redirect-" + sum
	if len(appName)+len(suffix) <= 63 {
		return appName + suffix
	}
	return strings.TrimRight(appName[:63-len(suffix)], "-") + suffix
}

func hostRedirectIngressName(appName string) string {
	const suffix = "-host-redirects"
	if len(appName)+len(suffix) <= 63 {
		return appName + suffix
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(appName)))[:8]
	return appName[:63-len(suffix)-len(sum)-1] + "-" + sum + suffix
}

// reconcileHostRedirects projects every valid auto-paired sibling into a
// per-host RedirectRegex middleware and one dedicated Ingress. A dedicated
// Ingress is necessary because middleware annotations apply to every rule in
// an Ingress: the canonical hosts must stay on the plain serving router. Each
// redirecting host retains its original per-host TLS secret.
func (r *AppReconciler) reconcileHostRedirects(ctx context.Context, app *appv1alpha1.App, hosts []string, svc string, port int32) (map[string]bool, error) {
	redirects := effectiveHostRedirects(app, hosts)
	desiredMiddleware := make(map[string]bool, len(redirects))
	middlewareNames := make([]string, 0, len(redirects))
	sources := make([]string, 0, len(redirects))
	for source := range redirects {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	for _, source := range sources {
		name := hostRedirectResourceName(app.Name, source)
		desiredMiddleware[name] = true
		middlewareNames = append(middlewareNames, traefikMiddlewareRef(app.Namespace, name))
		o := &unstructured.Unstructured{}
		o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
		o.SetName(name)
		o.SetNamespace(app.Namespace)
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, o, func() error {
			o.Object["spec"] = redirectRegexHTTPMiddlewareSpec(source, redirects[source])
			labels := o.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels[labelHostRedirectOwner] = app.Name
			o.SetLabels(labels)
			return controllerutil.SetControllerReference(app, o, r.Scheme)
		}); err != nil {
			return nil, err
		}
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: traefikHTTPMiddlewareGVK.Group, Version: traefikHTTPMiddlewareGVK.Version, Kind: "MiddlewareList"})
	// A NoMatchError means the Middleware CRD isn't installed, which leaves Items
	// empty — so the prune loop below is safe to run unconditionally.
	if err := r.List(ctx, list, client.InNamespace(app.Namespace), client.MatchingLabels{labelHostRedirectOwner: app.Name}); err != nil && !meta.IsNoMatchError(err) {
		return nil, err
	}
	for i := range list.Items {
		if desiredMiddleware[list.Items[i].GetName()] {
			continue
		}
		if err := deleteOptionalObject(ctx, r.Client, &list.Items[i]); err != nil {
			return nil, err
		}
	}

	ingressName := hostRedirectIngressName(app.Name)
	if len(redirects) == 0 {
		stale := &networkingv1.Ingress{}
		err := r.Get(ctx, client.ObjectKey{Name: ingressName, Namespace: app.Namespace}, stale)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		if err == nil { // cached read: absent on every steady-state pass
			if err := r.Delete(ctx, stale); err != nil && !apierrors.IsNotFound(err) {
				return nil, err
			}
		}
		return map[string]bool{}, nil
	}

	ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: ingressName, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ing, func() error {
		r.applyIngressRouting(ing, middlewareNames)
		ing.Labels = map[string]string{labelHostRedirectOwner: app.Name}
		for _, source := range sources {
			appendIngressHostRoute(ing, app.Name, source, slices.Index(hosts, source), svc, port)
		}
		return controllerutil.SetControllerReference(app, ing, r.Scheme)
	}); err != nil {
		return nil, err
	}

	redirectSources := make(map[string]bool, len(sources))
	for _, source := range sources {
		redirectSources[source] = true
	}
	return redirectSources, nil
}

// reconcileStaticSite materializes a static_site: build → object store (the
// publish plane), then serve the published revision from the shared
// static-server (no Deployment/Service for the served content). Routes/headers
// live on the CR and the static-server reads them live, so an edge-rule edit
// takes effect on the next resolver refresh without a republish. The image is
// already resolved (built from git or prebuilt) by Reconcile.
// publishStaticRevision uploads one revision's output to the object store —
// either extracted from the built image, or (direct publish, w9/010) cloned from
// the repo when no build ran. It reports its own failure through r.fail, so a
// non-nil error carries the reconcile result the caller must return.
func (r *AppReconciler) publishStaticRevision(ctx context.Context, app *appv1alpha1.App, image, rev string) (ctrl.Result, error) {
	publishNamespace := r.buildNamespace(app.Namespace)
	if err := validateStaticCredentialSecret(ctx, r.buildPlaneClient(), publishNamespace, r.StaticStore.Secret); err != nil {
		return r.fail(ctx, app, "StaticCredentialUnavailable", err)
	}
	r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Publishing", "Publishing static output to object store")
	opts := publish.Options{
		Image:        image,
		PublishPath:  app.Spec.PublishPath,
		AppID:        app.Name,
		AppUID:       string(app.UID),
		Revision:     rev,
		Store:        r.StaticStore,
		Namespace:    publishNamespace,
		Workspace:    app.Labels[labelWorkspace],
		AppNamespace: app.Namespace,
		VerifyImage:  r.TenantSignKeySecret != "",
		// The extract initContainer pulls the built image from authed Zot. Use
		// the BUILD-namespace pull credential: the per-App/shared tenant pull
		// secret lives in the App namespace, unreachable when the publish Job
		// runs in a separate BEX_BUILD_NAMESPACE (w9/m44 — static-site publish
		// failed on prod pulling with the apps-ns secret; docs/ADR029).
		PullSecret: r.buildJobPullSecret(app),
		Client:     r.Client,
	}
	if image == "" {
		// Direct publish (w9/010): no build ran — the publish Job clones the
		// repo and uploads rootDir/publishPath as-is.
		opts.Repo = app.Spec.Repo
		opts.Ref = effectiveDeployRef(app.Spec)
		opts.RootDir = app.Spec.RootDir
		opts.CloneSecret = app.Spec.CloneSecret
		opts.PullSecret = "" // clone mode pulls only public platform images
		// A private repo's clone Secret lives in the App namespace; relocate
		// it beside the publish Job when the build namespace differs (the
		// same seam the build plane uses).
		if app.Spec.CloneSecret != "" && opts.Namespace != app.Namespace {
			if err := r.copyCloneSecret(ctx, app, app.Namespace, opts.Namespace, app.Spec.CloneSecret); err != nil {
				return r.fail(ctx, app, "PublishFailed", fmt.Errorf("relocating clone secret to %s: %w", opts.Namespace, err))
			}
		}
	}
	if err := publish.Publish(ctx, opts); err != nil {
		return r.fail(ctx, app, "PublishFailed", err)
	}
	return ctrl.Result{}, nil
}

func (r *AppReconciler) reconcileStaticSite(ctx context.Context, app *appv1alpha1.App, image string) (ctrl.Result, error) {
	if app.Spec.PublishPath == "" {
		return r.fail(ctx, app, "BadSpec", fmt.Errorf("static_site requires spec.publishPath"))
	}
	if !r.StaticStore.Configured() || r.StaticServerService == "" {
		return r.fail(ctx, app, "StaticUnavailable",
			fmt.Errorf("static_site is unavailable: set BEX_STATIC_S3_ENDPOINT/BUCKET, BEX_STATIC_PUBLISH_S3_SECRET, and BEX_STATIC_SERVER_SERVICE"))
	}

	// Type-change cleanup: a static_site has no Deployment/Service of its own.
	// (The slug alias is converged by reconcileSlugService before dispatch.)
	if err := r.deleteStaleChildren(ctx,
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}},
	); err != nil {
		return r.fail(ctx, app, "StaticCleanupFailed", err)
	}

	rev := releaseRevision(app)

	// Publishing is skipped when rev is already active (idempotent: the publish
	// Job is named per revision, so a retry reuses the exact-lifetime Job rather
	// than re-uploading).
	if app.Status.ActiveRevision != rev {
		if res, err := r.publishStaticRevision(ctx, app, image, rev); err != nil {
			return res, err
		}
	}

	// A static_site must be reachable at a host (there is no in-cluster-only mode
	// for served content — the static-server dispatches by Host).
	hosts := effectiveHosts(app, r.BaseDomain)
	if len(hosts) == 0 {
		// A static site fails loudly here, and always has. That asymmetry is
		// what w7/m79 closes for web services, which produced the identical
		// no-host state in silence.
		return r.fail(ctx, app, "BadSpec",
			fmt.Errorf("static_site requires a host: set spec.expose (with BEX_BASE_DOMAIN), spec.host, or spec.hosts"))
	}
	r.setPublicRoutingCondition(app, hosts)

	// Suspended: stop serving by removing the Ingress; the published content stays
	// in the object store, so resume just recreates the Ingress.
	if app.Spec.Suspended {
		if _, err := r.reconcileWebsocketMeterMiddleware(ctx, app, false); err != nil {
			return r.fail(ctx, app, "MiddlewareCleanupFailed", err)
		}
		if err := r.reconcileIngress(ctx, app, nil, "", 0, nil); err != nil {
			return r.fail(ctx, app, "IngressCleanupFailed", err)
		}
		app.Status.ActiveRevision = rev
		return r.hibernated(ctx, app, image, nil, "Suspended",
			"static site suspended (not served; published content kept)")
	}

	staticMWNames, err := r.reconcileIPAllowListMiddleware(ctx, app)
	if err != nil {
		return r.fail(ctx, app, "MiddlewareFailed", err)
	}
	if _, err := r.reconcileWebsocketMeterMiddleware(ctx, app, false); err != nil {
		return r.fail(ctx, app, "MiddlewareFailed", err)
	}
	staticService, err := r.reconcileStaticServerAlias(ctx, app)
	if err != nil {
		return r.fail(ctx, app, "StaticAliasFailed", err)
	}
	if err := r.reconcileIngress(ctx, app, hosts, staticService, int32(r.staticServerPort()), staticMWNames); err != nil {
		return r.fail(ctx, app, "IngressFailed", err)
	}

	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.Image = image
	app.Status.ActiveRevision = rev
	app.Status.ObservedGeneration = app.Generation
	setStatusURLs(app, hosts)
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Published",
		Message: "static site published and served from the object-store origin", ObservedGeneration: app.Generation,
	})
	// Skip the write when nothing changed, the same guard markRunning and
	// hibernated already use — a re-published static site otherwise re-stamps a
	// byte-identical status (and logs) on every event.
	if r.statusSettled(ctx, app) {
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, app); err != nil {
		return ctrl.Result{}, err
	}
	logf.FromContext(ctx).Info("static site running", "name", app.Name, "revision", rev, "image", image)
	return ctrl.Result{}, nil
}

// staticServerPort is the static-server Service port, defaulting to 8080.
func (r *AppReconciler) staticServerPort() int {
	return cmp.Or(r.StaticServerPort, 8080)
}

func (r *AppReconciler) staticServerNamespace() string {
	return cmp.Or(r.StaticServerNamespace, platformNamespace)
}

// Ingress backends must live in the Ingress namespace. Static sites normally
// live in tenant namespaces while the shared origin runs in bex-system, so
// route through an App-owned ExternalName alias there.
func (r *AppReconciler) reconcileStaticServerAlias(ctx context.Context, app *appv1alpha1.App) (string, error) {
	if app.Namespace == r.staticServerNamespace() {
		return r.StaticServerService, nil
	}
	name := staticServerAliasName(app.Name)
	if err := r.reconcilePlatformAlias(ctx, app, name, platformAliasStatic,
		r.StaticServerService, r.staticServerNamespace(), int32(r.staticServerPort())); err != nil {
		return "", fmt.Errorf("reconciling static-server alias: %w", err)
	}
	return name, nil
}

func staticServerAliasName(appName string) string {
	return platformAliasName("bex-static-", appName)
}

// platformAliasName derives the ≤63-char Service name for an App's platform
// ExternalName alias: prefix + app name, truncated with a sha256 suffix when
// the combination would exceed the DNS-label cap.
func platformAliasName(prefix, appName string) string {
	if len(prefix)+len(appName) <= 63 {
		return prefix + appName
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(appName)))[:8]
	keep := 63 - len(prefix) - 1 - len(sum)
	return prefix + appName[:keep] + "-" + sum
}

// defaultMaintenanceService is the wake-activator Service name assumed when
// MaintenanceService is unset — the same default cmd/manager wires for
// ActivatorService via BEX_ACTIVATOR_SERVICE.
const defaultMaintenanceService = "bex-activator"

// platformNamespace is where bex's own shared workloads (the static server, the
// wake activator) run when no override is configured — never a tenant namespace.
const platformNamespace = "bex-system"

func (r *AppReconciler) maintenanceService() string {
	return cmp.Or(r.MaintenanceService, defaultMaintenanceService)
}

func (r *AppReconciler) maintenanceNamespace() string {
	return cmp.Or(r.MaintenanceNamespace, platformNamespace)
}

func (r *AppReconciler) maintenancePort() int {
	if r.MaintenancePort > 0 {
		return r.MaintenancePort
	}
	return 8888
}

// Ingress backends must live in the Ingress namespace. When the shared
// responder is in the platform namespace, create an App-owned ExternalName
// alias without replacing the tenant's private ClusterIP Service.
func (r *AppReconciler) reconcileMaintenanceAlias(ctx context.Context, app *appv1alpha1.App) (string, error) {
	if app.Namespace == r.maintenanceNamespace() {
		return r.maintenanceService(), nil
	}
	name := maintenanceAliasName(app.Name)
	if err := r.reconcilePlatformAlias(ctx, app, name, platformAliasMaintenance,
		r.maintenanceService(), r.maintenanceNamespace(), int32(r.maintenancePort())); err != nil {
		return "", fmt.Errorf("reconciling maintenance responder alias: %w", err)
	}
	return name, nil
}

// reconcilePlatformAlias is the single constructor for tenant ExternalName
// Services. Keep its labels, owner reference, target, and one-port shape aligned
// with operator-alias-admission.yaml: admission treats every other ExternalName
// create/update in a tenant namespace as an attempted routing-authority escape.
func (r *AppReconciler) reconcilePlatformAlias(ctx context.Context, app *appv1alpha1.App, name, purpose, service, namespace string, port int32) error {
	alias := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, alias, func() error {
		alias.Labels = map[string]string{
			labelApp:                       app.Name,
			labelPlatformAliasPurpose:      purpose,
			"app.kubernetes.io/managed-by": "bex-operator",
		}
		alias.Spec.Type = corev1.ServiceTypeExternalName
		alias.Spec.ExternalName = fmt.Sprintf("%s.%s.svc.cluster.local", service, namespace)
		alias.Spec.Selector = nil
		// Protocol/TargetPort are set explicitly so the mutate matches the
		// server-defaulted object and steady-state reconciles stay read-only,
		// exactly as applyClusterIPService documents — otherwise CreateOrUpdate
		// diffs against the defaults and PUTs on every pass forever.
		alias.Spec.Ports = []corev1.ServicePort{{
			Name: "http", Port: port, TargetPort: intstr.FromInt32(port), Protocol: corev1.ProtocolTCP,
		}}
		return controllerutil.SetControllerReference(app, alias, r.Scheme)
	}); err != nil {
		return err
	}
	return nil
}

func maintenanceAliasName(appName string) string {
	return platformAliasName("bex-maintenance-", appName)
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
	// Same steady-state gate as the Deployment path: the 1-minute run-history
	// poll of a settled Running cron must not flap Running→Deploying→Running.
	if app.Status.Phase != appv1alpha1.PhaseRunning ||
		app.Status.ObservedGeneration != app.Generation || app.Status.Image != image {
		r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Deploying", "Reconciling CronJob for "+image)
	}
	labels := map[string]string{labelApp: app.Name, labelAppID: appIDOrName(app)}
	if r.TenantSignKeySecret != "" {
		labels[execution.LabelVerifyImage] = execution.VerifyImageEnabled
	}
	suspended := app.Spec.Suspended

	cj := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cj, func() error {
		cj.Spec.Schedule = app.Spec.Schedule
		// Render runs at most one cron execution at a time. Scheduled overlap is
		// skipped; a manual trigger explicitly cancels the active Job before its
		// replacement is created below.
		cj.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
		// Suspend pauses scheduling without losing history — resume just clears it.
		cj.Spec.Suspend = &suspended
		cj.Spec.JobTemplate.Labels = labels // so the Jobs it creates carry labelApp
		cj.Spec.JobTemplate.Spec.Template = r.cronPodSpec(app, image, port, labels)
		return controllerutil.SetControllerReference(app, cj, r.Scheme)
	}); err != nil {
		return r.fail(ctx, app, "CronJobFailed", err)
	}

	cancelPending, err := r.cancelRequestedCronRun(ctx, app)
	if err != nil {
		return r.fail(ctx, app, "CronRunCancelFailed", err)
	}

	// One-off run trigger (spec.runAt, from the API's cron run verb): materialize a
	// single Job from the same template, named deterministically from runAt so a
	// re-reconcile of the same value is a no-op. Skipped while suspended.
	if app.Spec.RunAt != "" && !suspended && !cancelPending && !cancelsManualRun(app) {
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
	app.Status.ActiveRevision = releaseRevision(app)
	app.Status.ObservedGeneration = app.Generation
	if suspended {
		app.Status.Phase = appv1alpha1.PhaseHibernated
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "Suspended",
			Message: "cron schedule suspended", ObservedGeneration: app.Generation,
		})
	} else {
		app.Status.Phase = appv1alpha1.PhaseRunning
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Scheduled",
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
	if cancelPending {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// cancelRequestedCronRun converges spec.cancelRun by foreground-deleting the
// exact backing Job. NotFound is success: a retry after deletion and a run the
// CronJob controller already removed are both already at the desired state.
// Status reconciliation below preserves and marks the run Canceled even after
// the Job object has disappeared. The bool reports that deletion is still in
// progress, which keeps a manual replacement from starting until the old run's
// pods are gone.
func (r *AppReconciler) cancelRequestedCronRun(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	if app.Spec.CancelRun == nil || app.Spec.CancelRun.Name == "" {
		return false, nil
	}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: app.Spec.CancelRun.Name, Namespace: app.Namespace}}
	if err := r.Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if !job.DeletionTimestamp.IsZero() {
		return true, nil
	}
	if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	return true, nil
}

// cancelsManualRun prevents the stable spec.runAt value from recreating the
// exact manual Job a cancellation just deleted. A later trigger changes runAt,
// producing a different Job name, so the replacement is created normally.
func cancelsManualRun(app *appv1alpha1.App) bool {
	return app.Spec.CancelRun != nil &&
		app.Spec.CancelRun.Name == appv1alpha1.ManualCronRunJobName(app.Name, app.Spec.RunAt)
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
	job.Spec.Template = r.cronPodSpec(app, image, port, labels)
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
	runs := make([]appv1alpha1.CronRun, 0, maxCronRuns)
	prior := make(map[string]appv1alpha1.CronRun, len(app.Status.Runs))
	for _, run := range app.Status.Runs {
		prior[run.Name] = run
	}
	current := make(map[string]appv1alpha1.CronRun, len(items))
	for i := range items {
		run := toCronRun(&items[i])
		if old, ok := prior[run.Name]; ok && run.StartedAt == "" {
			run.StartedAt = old.StartedAt
		}
		if app.Spec.CancelRun != nil && app.Spec.CancelRun.Name == run.Name {
			run.Status = appv1alpha1.CronRunCanceled
			run.FinishedAt = app.Spec.CancelRun.RequestedAt
		}
		current[run.Name] = run
		// A Job absent from the prior status is new, and the sorted Job list puts
		// these newest executions first. Existing entries are appended below in
		// their prior relative order, which remains stable when a newer canceled
		// Job disappears while an older successful Job still exists.
		if _, existed := prior[run.Name]; !existed && len(runs) < maxCronRuns {
			runs = append(runs, run)
		}
	}
	// Kubernetes may garbage-collect terminal Jobs (and cancellation deletes its
	// Job by definition). Retain terminal entries already captured in status so
	// the first-class run ids and cursor history remain stable across reads.
	for _, old := range app.Status.Runs {
		if len(runs) >= maxCronRuns {
			break
		}
		if run, ok := current[old.Name]; ok {
			runs = append(runs, run)
			continue
		}
		if app.Spec.CancelRun != nil && app.Spec.CancelRun.Name == old.Name {
			old.Status = appv1alpha1.CronRunCanceled
			old.FinishedAt = app.Spec.CancelRun.RequestedAt
		}
		switch old.Status {
		case appv1alpha1.CronRunSucceeded, appv1alpha1.CronRunFailed, appv1alpha1.CronRunCanceled:
			runs = append(runs, old)
		}
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
	run := appv1alpha1.CronRun{Name: job.Name, Status: appv1alpha1.CronRunRunning}
	if job.Status.StartTime != nil {
		run.StartedAt = job.Status.StartTime.UTC().Format(time.RFC3339)
	}
	for _, c := range job.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		switch c.Type {
		case batchv1.JobComplete:
			run.Status = appv1alpha1.CronRunSucceeded
			run.FinishedAt = c.LastTransitionTime.UTC().Format(time.RFC3339)
		case batchv1.JobFailed:
			run.Status = appv1alpha1.CronRunFailed
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
// spec.command, when set, overrides the image's default entrypoint/cmd via a
// shell (Render's cron "Command" field); empty leaves the image's own command.
func (r *AppReconciler) cronPodSpec(app *appv1alpha1.App, image string, port int, labels map[string]string) corev1.PodTemplateSpec {
	container := corev1.Container{
		Name:            "app",
		Image:           image,
		Env:             appEnv(app, port),
		EnvFrom:         envFromSources(app),
		Resources:       resourcesForTier(app.Spec.Tier),
		SecurityContext: tenantSecCtx(),
	}
	command := app.Spec.Command
	if b := effectiveBuilder(app.Spec); command == "" && b != build.BuilderNative && b != build.BuilderBuildpack {
		command = app.Spec.StartCommand
	}
	if command != "" {
		container.Command = []string{"/bin/sh", "-c", command}
	}
	spec := corev1.PodSpec{
		RestartPolicy:                corev1.RestartPolicyNever,
		AutomountServiceAccountToken: ptr(false),
	}
	// Secret files reach the cron run's container the same way as a Deployment's.
	if vol, mount := secretFileMounts(app); vol != nil {
		container.VolumeMounts = []corev1.VolumeMount{*mount}
		spec.Volumes = []corev1.Volume{*vol}
	}
	spec.Containers = []corev1.Container{container}
	// Authenticate the pull under an auth-enabled registry (w7/m8) — owned here so
	// every caller of cronPodSpec gets it without a post-hoc patch at each site.
	spec.ImagePullSecrets = r.imagePullSecrets(app, image)
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: labels},
		Spec:       spec,
	}
}

// manualRunJobName derives a deterministic Job name from the App name + runAt, so
// reconciling the same runAt twice never spawns a duplicate run.
func manualRunJobName(appName, runAt string) string {
	return appv1alpha1.ManualCronRunJobName(appName, runAt)
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
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "Suspended",
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
				Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Resumed",
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
	app.Status.ActiveRevision = releaseRevision(app)
	app.Status.ObservedGeneration = app.Generation
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Deployed",
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
// TLS secret name stable), then the platform hostname
// "<subdomain-or-name>.<BaseDomain>" when exposed, then spec.hosts (custom
// domains). Deduplicated, empties dropped.
//
// The platform hostname prefers spec.subdomain (w4/m19): the control plane
// mints it as a globally-unique slug, so two Apps sharing a workspace-scoped
// name (bex.co/m19's whole point) never claim the same host. spec.subdomain
// is empty on any App the control plane hasn't stamped — a pre-migration
// store-managed App, or one applied directly (examples/whoami-app.yaml) — so
// falling back to app.Name keeps those byte-identical to before this field
// existed.
func effectiveHosts(app *appv1alpha1.App, baseDomain string) []string {
	// The precedence rule lives on the CRD contract (types.AppSpec.EffectiveHosts,
	// w5/m48) so the static-server resolver and bex-api's pending-URL projection
	// cannot drift from the Ingress/status derivation.
	return app.Spec.EffectiveHosts(app.Name, baseDomain)
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
// A ValueFrom.SecretKeyRef entry (a bex.yml fromDatabase reference, w1/m24)
// becomes a corev1 EnvVar.ValueFrom.SecretKeyRef — non-optional, so a service
// waiting on its Database stays Pending until the CNPG connection Secret exists.
func appEnv(app *appv1alpha1.App, port int) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0, len(app.Spec.Env)+1)
	for _, e := range app.Spec.Env {
		if e.Name == portEnvName {
			continue // operator-owned; never let a user shadow it
		}
		ev := corev1.EnvVar{Name: e.Name, Value: e.Value}
		if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			ev.Value = ""
			ev.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: e.ValueFrom.SecretKeyRef.Name},
					Key:                  e.ValueFrom.SecretKeyRef.Key,
				},
			}
		}
		env = append(env, ev)
	}
	env = append(env, corev1.EnvVar{Name: portEnvName, Value: strconv.Itoa(port)})
	return env
}

// envFromSources wires the App's secret-backed env into the container: each linked
// environment group's "<evg-id>-env" Secret (spec.envFromSecrets) FIRST, then the
// service's own "<name>-env" Secret (active spec reference or save-only pending
// annotation) LAST. Kubernetes applies
// envFrom sources in order and the last wins on a key collision, so a service's own
// variable overrides a linked group's — matching Render (docs/ADR013-secrets.md). The
// group sources are marked optional so a briefly-absent group Secret can't wedge
// the pod; the service's own set keeps its original (non-optional) shape. No
// active or pending service reference => no service envFrom.
func envFromSources(app *appv1alpha1.App) []corev1.EnvFromSource {
	var out []corev1.EnvFromSource
	optional := true
	for _, name := range app.Spec.EnvFromSecrets {
		if name == "" {
			continue
		}
		out = append(out, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Optional:             &optional,
			},
		})
	}
	if name := runtimeEnvSecret(app); name != "" {
		out = append(out, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
			},
		})
	}
	return out
}

// secretFilesVolumeName is the single projected volume carrying every secret-file
// source, mounted read-only at secretFilesMountPath.
const (
	secretFilesVolumeName = "bex-secret-files"
	secretFilesMountPath  = "/etc/secrets"
)

// secretFileMounts projects spec.filesFromSecrets plus a save-only pending
// service-file annotation into one read-only /etc/secrets volume + mount
// (docs/ADR013-secrets.md — secret files): each named Secret's keys become
// files "/etc/secrets/<key>". A service's own files ("<name>-files") and each
// linked group's files ("<evg-id>-files") merge into the single projected volume,
// each source optional so an absent one contributes no files rather than failing
// the mount. Empty spec => (nil, nil): no volume, unchanged behavior.
func secretFileMounts(app *appv1alpha1.App) (*corev1.Volume, *corev1.VolumeMount) {
	var sources []corev1.VolumeProjection
	optional := true
	for _, name := range app.Spec.FilesFromSecrets {
		if name == "" {
			continue
		}
		sources = append(sources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Optional:             &optional,
			},
		})
	}
	if name := app.Annotations[appv1alpha1.PendingFilesSecretAnnotation]; name != "" && !containsSecretProjection(sources, name) {
		sources = append(sources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Optional:             &optional,
			},
		})
	}
	if len(sources) == 0 {
		return nil, nil
	}
	vol := &corev1.Volume{
		Name:         secretFilesVolumeName,
		VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: sources}},
	}
	mount := &corev1.VolumeMount{Name: secretFilesVolumeName, MountPath: secretFilesMountPath, ReadOnly: true}
	return vol, mount
}

// runtimeEnvSecret prefers the active spec reference and falls back to the
// metadata-only projection staged by a save-only Environment patch. Metadata
// updates do not enqueue this generation-filtered controller; a later restart
// or deploy does, and consumes the pending Secret without an intermediate roll.
func runtimeEnvSecret(app *appv1alpha1.App) string {
	if app.Spec.EnvFromSecret != "" {
		return app.Spec.EnvFromSecret
	}
	return app.Annotations[appv1alpha1.PendingEnvSecretAnnotation]
}

func containsSecretProjection(sources []corev1.VolumeProjection, name string) bool {
	for _, source := range sources {
		if source.Secret != nil && source.Secret.Name == name {
			return true
		}
	}
	return false
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

// tenantSecCtx returns the hardening SecurityContext stamped on every tenant
// container: no privilege escalation, all capabilities dropped, RuntimeDefault
// seccomp. runAsNonRoot is deliberately absent — tenant images may run as root
// (PSS baseline only, not restricted; see docs/ADR022-tenant-isolation.md).
func tenantSecCtx() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

// ptr returns a pointer to v. Needed because Go cannot take the address of a
// composite literal or constant — e.g. &false is a compile error.
func ptr[T any](v T) *T { return &v }

// stuckPodMessage inspects the pods behind a not-yet-ready tenant Deployment
// for a terminal-looking container state and returns an actionable Ready
// condition (reason, message). Empty when the rollout is merely progressing.
// Listed with the cached client: SetupWithManager watches pods, so the
// informer already exists and the read is free.
func (r *AppReconciler) stuckPodMessage(ctx context.Context, dep *appsv1.Deployment, port int) (string, string) {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return "", ""
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace),
		client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return "", ""
	}
	for _, p := range pods.Items {
		for _, cs := range p.Status.ContainerStatuses {
			w := cs.State.Waiting
			if w == nil {
				continue
			}
			switch w.Reason {
			case "CrashLoopBackOff":
				exit := ""
				if t := cs.LastTerminationState.Terminated; t != nil {
					exit = fmt.Sprintf(" (last exit code %d)", t.ExitCode)
				}
				msg := fmt.Sprintf(
					"container exited shortly after start and is restarting repeatedly%s — check the service logs for the crash output.", exit)
				if port > 0 {
					msg += fmt.Sprintf(
						" If the crash is a port bind: the process must listen on $PORT (%d), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped).", port)
				}
				return "CrashLoopBackOff", msg
			case "ImagePullBackOff", "ErrImagePull":
				return "ImagePullBackOff", "image pull is failing: " + w.Message
			case createContainerConfigError:
				// The pod's configuration cannot be resolved — almost always an
				// env var or file whose Secret/ConfigMap is absent from the
				// service's namespace, or present without the referenced key.
				// The kubelet already names the object precisely ("secret
				// \"dpg-…-app\" not found"), so pass its message through rather
				// than re-deriving a worse one.
				//
				// This class was the 2026-08-08 incident's first failure leg and
				// had no signal at all: the pod never starts, so it never crash-
				// loops and never pulls badly, and the deploy simply timed out as
				// an unexplained health-check failure minutes later (ADR043 D8).
				return createContainerConfigError,
					"container configuration cannot be resolved: " + w.Message +
						" — a referenced Secret or ConfigMap must exist in this service's own namespace and carry the referenced key. " +
						"A linked Postgres or Key Value provides its Secret once the instance is provisioned."
			}
		}
	}
	return "", ""
}

// setPublicRoutingCondition records whether this App has a derivable public
// host, and when it does not, why.
//
// This is w7/m79/t003, and it exists because of what w7/m78 found: production
// runs with BEX_BASE_DOMAIN unset (onbex.co is a registrable domain, so tenant
// JavaScript could set parent cookies a sibling tenant receives — w7/m54). With
// no base domain a service that brought no custom domain has zero effective
// hosts, so reconcileIngress correctly writes nothing, status.URL is never
// populated, and the API reports url: null. All of that is right; the silence
// is not. Nothing told the user their service would never be publicly
// reachable, or what to do about it.
//
// The condition is also the durable per-service trace for a platform-wide
// change: when BEX_BASE_DOMAIN was removed, every service that had only a
// platform host had its Ingress deleted on the next reconcile, fleet-wide and
// unrecorded. A Kubernetes Event would have been the obvious alternative, but
// Events expire (default ~1h) and would be gone long before anyone investigated
// — a condition persists on the CR for exactly as long as the state does.
func (r *AppReconciler) setPublicRoutingCondition(app *appv1alpha1.App, hosts []string) {
	// Only the types that carry a public URL at all. A worker or cron job has no
	// public route by definition and must never report a routing problem.
	switch app.Spec.Type {
	case appv1alpha1.TypeWebService, appv1alpha1.TypeStaticSite, "":
	default:
		meta.RemoveStatusCondition(&app.Status.Conditions, appv1alpha1.ConditionPublicRouting)
		return
	}
	if len(hosts) > 0 {
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionPublicRouting, Status: metav1.ConditionTrue, Reason: "Routed",
			Message:            "serving at " + strings.Join(hosts, ", "),
			ObservedGeneration: app.Generation,
		})
		return
	}
	// A service that did not ask to be exposed, or that switched its own
	// platform subdomain off without adding a custom domain, is in the state its
	// owner chose. Reporting it would train users to ignore the condition.
	if !app.Spec.Expose || app.Spec.SubdomainPolicy == appv1alpha1.SubdomainPolicyDisabled {
		meta.RemoveStatusCondition(&app.Status.Conditions, appv1alpha1.ConditionPublicRouting)
		return
	}
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionPublicRouting, Status: metav1.ConditionFalse, Reason: "NoPublicHost",
		Message: "this service has no public address: the platform subdomain is not available on this " +
			"installation, and no custom domain is attached. Add a custom domain to serve it publicly.",
		ObservedGeneration: app.Generation,
	})
}

// awaitRegistryCredActive proves zot accepts this App's per-App credential
// before a build pushes or a workload pulls (zot loads credentials only at
// process start — see registry/verify.go). It reports halted=true with the
// result to return while the credential is unusable: a probe failure is a
// platform hiccup (requeue, keep the previous state serving), a not-yet-active
// credential parks the App at the caller's phase.
func (r *AppReconciler) awaitRegistryCredActive(ctx context.Context, app *appv1alpha1.App, phase appv1alpha1.AppPhase, logMsg, waitMsg string) (ctrl.Result, bool) {
	active, err := r.PerAppRegistry.EnsureActive(ctx, app.Name, app.Namespace)
	if err != nil {
		logf.FromContext(ctx).Error(err, logMsg, "app", app.Name)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, true
	}
	if !active {
		r.setPhase(ctx, app, phase, "RegistryCredsPending", waitMsg)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, true
	}
	return ctrl.Result{}, false
}

// statusSettled reports whether the App's in-memory status already matches the
// stored object's, so a steady-state terminal pass can skip its no-op status
// PUT (the same diff-before-write discipline as setPhase; the Get is served by
// the informer cache).
func (r *AppReconciler) statusSettled(ctx context.Context, app *appv1alpha1.App) bool {
	var stored appv1alpha1.App
	if err := r.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		return false
	}
	return apiequality.Semantic.DeepEqual(stored.Status, app.Status)
}

// setPhase stamps a transitional phase + Ready=False condition. The write is
// skipped when the persisted phase and condition already match: a no-op pass
// re-stamping an unchanged state would otherwise issue a status write per
// reconcile, per App (a diff-before-write discipline that matters now that the
// build phase requeues every few seconds instead of blocking).
func (r *AppReconciler) setPhase(ctx context.Context, app *appv1alpha1.App, p appv1alpha1.AppPhase, reason, msg string) {
	samePhase := app.Status.Phase == p
	app.Status.Phase = p
	changed := meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: msg,
		ObservedGeneration: app.Generation,
	})
	if samePhase && !changed {
		return
	}
	_ = r.Status().Update(ctx, app)
}

func (r *AppReconciler) fail(ctx context.Context, app *appv1alpha1.App, reason string, err error) (ctrl.Result, error) {
	app.Status.Phase = appv1alpha1.PhaseFailed
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: err.Error(),
		ObservedGeneration: app.Generation,
	})
	_ = r.Status().Update(ctx, app)
	return ctrl.Result{}, err
}

// reconcilePreDeploy runs spec.preDeployCommand to completion against image
// before the caller rolls the Deployment to it (w1/m33, docs/ADR004-app-deployment.md).
// It returns halt=true when the caller must stop this reconcile and return
// (res, err): either the step failed (err set — the previous revision keeps
// serving because the Deployment update below never runs) or it is still running
// (requeue). halt=false lets the rollout proceed — the step is unset or already
// succeeded for this revision.
//
// The step is gated per release generation, so operational generations (scale,
// routing, suspension, autoscaling) never re-run it. It runs once per rollout,
// exactly like Render's Pre-Deploy Command, and never re-runs a migration that
// already passed or failed for the same revision. That terminal bookkeeping also
// guards against re-creating the Job after its TTL reap.
func (r *AppReconciler) reconcilePreDeploy(ctx context.Context, app *appv1alpha1.App, image string, port int) (ctrl.Result, bool, error) {
	if app.Spec.PreDeployCommand == "" {
		return ctrl.Result{}, false, nil
	}
	gen := releaseGeneration(app)

	// firstForGen is the first reconcile that runs this revision's step; started
	// is preserved across the requeues that follow. Terminal per release: a
	// completed step for this revision is decisive, so we never touch the Job
	// again (no re-run, no re-create after TTL reap).
	firstForGen := true
	started := time.Now().UTC().Format(time.RFC3339)
	if pd := app.Status.PreDeploy; pd != nil && pd.Generation == gen {
		firstForGen = false
		if pd.StartedAt != "" {
			started = pd.StartedAt
		}
		switch pd.Status {
		case appv1alpha1.PreDeploySucceeded:
			return ctrl.Result{}, false, nil // passed — proceed to the rollout
		case appv1alpha1.PreDeployFailed:
			return r.failPreDeploy(ctx, app, pd, fmt.Errorf("pre-deploy command failed: %s", pd.Message))
		}
		// Running/Pending: fall through to re-check the live Job.
	}

	ns := r.buildNamespace(app.Namespace)
	rev := fmt.Sprintf("gen-%d", gen)
	jobName := predeploy.JobName(app.Name, rev)
	failedPD := func(job, msg string) *appv1alpha1.PreDeployStatus {
		return &appv1alpha1.PreDeployStatus{
			Job: job, Generation: gen, Status: appv1alpha1.PreDeployFailed,
			StartedAt: started, FinishedAt: time.Now().UTC().Format(time.RFC3339), Message: msg,
		}
	}

	// Newest-wins: cancel any older-revision migration still in flight — but only
	// on the FIRST pass for this revision. Re-checking a running migration has
	// nothing newer to supersede (a newer generation would have reset firstForGen),
	// so this avoids a wasted List on every requeue.
	if firstForGen {
		if err := predeploy.CancelSuperseded(ctx, app.Name, string(app.UID), ns, jobName, r.Client); err != nil {
			return r.failPreDeploy(ctx, app, failedPD(jobName, err.Error()), fmt.Errorf("pre-deploy: %w", err))
		}
	}

	// The step's container mirrors the app pod: same env, envFrom (the app's
	// "<name>-env" Secret a migration reads DATABASE_URL from), pull secrets,
	// security context, tier resources, and secret-file volume.
	var vols []corev1.Volume
	var mounts []corev1.VolumeMount
	if vol, mount := secretFileMounts(app); vol != nil {
		vols = []corev1.Volume{*vol}
		mounts = []corev1.VolumeMount{*mount}
	}
	pullSecrets, err := r.predeployPullSecrets(ctx, app, image)
	if err == nil {
		err = r.mirrorPreDeploySecrets(ctx, app, ns)
	}
	if err != nil {
		return r.failPreDeploy(ctx, app, failedPD(jobName, err.Error()), fmt.Errorf("pre-deploy: %w", err))
	}
	if err := r.reconcileExecutionNetworkPolicy(ctx, app); err != nil {
		return r.failPreDeploy(ctx, app, failedPD(jobName, err.Error()), fmt.Errorf("pre-deploy network policy: %w", err))
	}
	job, err := predeploy.Ensure(ctx, predeploy.Options{
		Name:             app.Name,
		AppUID:           string(app.UID),
		Namespace:        ns,
		Workspace:        app.Labels[labelWorkspace],
		AppNamespace:     app.Namespace,
		VerifyImage:      r.TenantSignKeySecret != "",
		Image:            image,
		Command:          app.Spec.PreDeployCommand,
		Revision:         rev,
		Generation:       gen,
		Env:              appEnv(app, port),
		EnvFrom:          envFromSources(app),
		ImagePullSecrets: pullSecrets,
		SecurityContext:  tenantSecCtx(),
		Resources:        resourcesForTier(app.Spec.Tier),
		Volumes:          vols,
		VolumeMounts:     mounts,
		Client:           r.Client,
	})
	if err != nil {
		// A transient API error (create/get) — requeue without marking the deploy
		// failed; the migration itself hasn't reported an outcome.
		return ctrl.Result{}, true, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	switch state, msg := predeploy.Observe(job); state {
	case predeploy.StateSucceeded:
		app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
			Job: job.Name, Generation: gen, Status: appv1alpha1.PreDeploySucceeded,
			StartedAt: started, FinishedAt: now,
		}
		_ = r.Status().Update(ctx, app)
		logf.FromContext(ctx).Info("pre-deploy succeeded", "name", app.Name, "job", job.Name)
		return ctrl.Result{}, false, nil // proceed to the rollout
	case predeploy.StateFailed:
		return r.failPreDeploy(ctx, app, failedPD(job.Name, msg), fmt.Errorf("pre-deploy command failed: %s", msg))
	default: // Pending/Running — keep the old revision serving and requeue
		app.Status.Phase = appv1alpha1.PhaseDeploying
		app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
			Job: job.Name, Generation: gen, Status: appv1alpha1.PreDeployRunning,
			StartedAt: started,
		}
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "PreDeploy",
			Message: "running pre-deploy command", ObservedGeneration: app.Generation,
		})
		_ = r.Status().Update(ctx, app)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, true, nil
	}
}

// failPreDeploy records the failed pre-deploy step (status.preDeploy = pd, phase
// Failed) and blocks the rollout — the one place the three failure paths (a
// persisted-Failed re-check, the Job's non-zero exit, and a cancel-superseded
// error) converge, so the status shape can't drift between them.
func (r *AppReconciler) failPreDeploy(ctx context.Context, app *appv1alpha1.App, pd *appv1alpha1.PreDeployStatus, err error) (ctrl.Result, bool, error) {
	app.Status.PreDeploy = pd
	res, ferr := r.fail(ctx, app, "PreDeployFailed", err)
	return res, true, ferr
}

// reconcileNetworkPolicy deletes any per-App NetworkPolicy a pre-ADR043
// reconcile may have left behind (docs/ADR022-tenant-isolation.md's Option B,
// superseded by ADR043). Per-tenant namespace isolation is unconditional
// (ADR043): the workspace's `<ws>` namespace default-deny + allow policies
// (control-plane NamespaceReconciler) are the tenant boundary, so a per-App
// label-scoped policy is redundant — the namespace, not a label, is the
// boundary now. The transitional TenantNamespaces gate that made this
// conditional was retired in w3/m34 once every cluster had migrated.
func (r *AppReconciler) reconcileNetworkPolicy(ctx context.Context, app *appv1alpha1.App) error {
	np := &networkingv1.NetworkPolicy{}
	if err := r.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, np); err != nil {
		return client.IgnoreNotFound(err) // cached read: absent on every steady-state pass
	}
	if err := r.Delete(ctx, np); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// reconcileExecutionNetworkPolicy adds the one dynamic egress edge a static
// namespace policy cannot express: a pre-deploy Pod may reach only workloads in
// its own workspace (or protected environment) in the App namespace. DNS,
// source hosts, the registry, and public object storage are handled by the
// namespace-wide policy in deploy/gitops/base/build-boundary.yaml.
func (r *AppReconciler) reconcileExecutionNetworkPolicy(ctx context.Context, app *appv1alpha1.App) error {
	buildNS := r.buildNamespace(app.Namespace)
	if buildNS == app.Namespace {
		return nil
	}
	name := build.JobName(app.Name, "execution-egress")
	np := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: buildNS}}
	ws := app.Labels[labelWorkspace]
	if ws == "" {
		// Owner-gated like the finalizer's delete (round 12, finding 1): the
		// shared build namespace can hold a same-named App's policy, which a
		// bare Delete would tear down cross-workspace.
		identity := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
			Workspace: ws, Namespace: app.Namespace}
		_, err := deleteOwnedObject(ctx, r.buildPlaneClient(), buildNS, name, &networkingv1.NetworkPolicy{}, identity)
		return client.IgnoreNotFound(err)
	}
	scopeSelector := map[string]string{labelWorkspace: ws}
	if env := app.Labels[labelNetworkIsolation]; env != "" {
		scopeSelector = map[string]string{labelNetworkIsolation: env}
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.buildPlaneClient(), np, func() error {
		if err := checkOwnedArtifact(np, app); err != nil {
			return err
		}
		np.Labels = artifactLabels(app, "execution-network-policy")
		np.Spec = networkingv1.NetworkPolicySpec{
			// round-5 finding 8: the build namespace is shared, so selecting source
			// pods by name alone (labelApp) would apply this workspace's egress
			// grant to a SAME-NAMED App's build pods in ANOTHER workspace, letting
			// attacker build code reach the victim namespace. Scope the source
			// selector to this App's workspace too — the same tenant discriminator
			// the destination scopeSelector uses (ws is non-empty here). App name +
			// workspace uniquely identifies the App (a name is unique per workspace).
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{labelApp: app.Name, labelWorkspace: ws}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": app.Namespace,
					}},
					PodSelector: &metav1.LabelSelector{MatchLabels: scopeSelector},
				}},
			}},
		}
		return nil // cross-namespace ownerReferences are invalid; finalizer deletes it
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
	workers := max(r.MaxConcurrentReconciles, 1)
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.App{}, crbuilder.WithPredicates(generationOrDeletionPredicate{})).
		// Deployment readiness and rollout revisions live entirely in status, so
		// child resourceVersion updates must not inherit the App's generation-only
		// primary filter. Pod events close the shorter crash-loop window before the
		// Deployment controller has recomputed availableReplicas.
		Owns(&appsv1.Deployment{}, crbuilder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(appPodRequests),
			crbuilder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Owns(&networkingv1.Ingress{}, crbuilder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&networkingv1.NetworkPolicy{}, crbuilder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&batchv1.CronJob{}, crbuilder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		WithOptions(crcontroller.Options{MaxConcurrentReconciles: workers}).
		Named("app").
		Complete(r)
}

func appPodRequests(_ context.Context, object client.Object) []reconcile.Request {
	name := object.GetLabels()[labelApp]
	if name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: object.GetNamespace(), Name: name}}}
}

// handleAppDeletion runs the finalizer teardown for an App being deleted:
// cleans up build artifacts, cert-manager TLS Secrets, static-site S3 content,
// and Zot registry images that ownerRefs can't cascade to. Extracted from
// Reconcile to keep its cyclomatic complexity in check.
func (r *AppReconciler) handleAppDeletion(ctx context.Context, app *appv1alpha1.App) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(app, finalizer) {
		log := logf.FromContext(ctx)
		var errs []error
		if app.Status.SandboxID != "" && r.Runtime != nil {
			if err := r.Runtime.Delete(ctx, app.Status.SandboxID); err != nil {
				errs = append(errs, fmt.Errorf("delete sandbox: %w", err))
			}
		}
		// Cross-namespace build/predeploy Jobs and kpack Images in the build
		// namespace — ownerRefs are invalid across namespaces so nothing cascades.
		ns := r.buildNamespace(app.Namespace)
		cl := r.buildPlaneClient()
		executionPending, err := r.reclaimAppExecution(ctx, app, ns, cl)
		errs = append(errs, err)
		externalPending, err := r.reclaimAppExternalArtifacts(ctx, app, ns, cl)
		errs = append(errs, err)
		cleanupErr := errors.Join(errs...)
		if cleanupErr != nil {
			log.Error(cleanupErr, "App finalization cleanup blocked; preserving registry credentials",
				"app", app.Name, "executionPending", executionPending, "externalPending", externalPending)
			return ctrl.Result{}, cleanupErr
		}
		if executionPending || externalPending {
			log.Info("App finalization cleanup pending; preserving registry credentials",
				"app", app.Name, "executionPending", executionPending, "externalPending", externalPending)
			return ctrl.Result{RequeueAfter: settleRequeue}, nil
		}

		// Per-App registry credentials are the least-privilege authority used to
		// delete and then prove the repository absent. Revoke them only after all
		// execution and external artifacts have converged; otherwise a transient
		// registry/RBAC failure destroys the only credential that can retry safely.
		credentialsPending, err := r.revokeAppRegistryCredentials(ctx, app, ns, cl)
		if err != nil {
			log.Error(err, "App finalization credential revocation blocked", "app", app.Name)
			return ctrl.Result{}, err
		}
		if credentialsPending {
			log.Info("App finalization credential revocation pending", "app", app.Name)
			return ctrl.Result{RequeueAfter: settleRequeue}, nil
		}
		controllerutil.RemoveFinalizer(app, finalizer)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *AppReconciler) reclaimAppExecution(ctx context.Context, app *appv1alpha1.App, namespace string, cl client.Client) (bool, error) {
	var errs []error
	pending := false
	identity := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
		Workspace: app.Labels[labelWorkspace], Namespace: app.Namespace}
	done, _, err := build.ReclaimAppArtifacts(ctx, identity, namespace, cl)
	pending = pending || !done
	if err != nil {
		errs = append(errs, fmt.Errorf("reclaim build artifacts: %w", err))
	}
	if namespace != app.Namespace {
		for _, name := range r.knownBuildSecretNames(app) {
			// Delete only a Secret this App owns; a same-named foreign/platform
			// Secret in the shared build namespace must survive the finalizer
			// (codex-security #5 — the loop used to delete by name, no owner check).
			done, deleteErr := deleteOwnedObject(ctx, cl, namespace, name, &corev1.Secret{}, identity)
			pending = pending || !done
			if deleteErr != nil {
				errs = append(errs, fmt.Errorf("delete copied build Secret %s: %w", name, deleteErr))
			}
		}
	}
	// round 12, finding 1: the execution NetworkPolicy name derives from the
	// workspace-local App name and lives in the shared build namespace, so the
	// bare deleteAndWait below would also remove a SAME-NAMED App's policy in
	// another workspace. Gate on the artifact ownership labels like the Secrets.
	execPolicyName := build.JobName(app.Name, "execution-egress")
	if done, deleteErr := deleteOwnedObject(ctx, cl, namespace, execPolicyName, &networkingv1.NetworkPolicy{}, identity); deleteErr != nil {
		errs = append(errs, fmt.Errorf("delete execution NetworkPolicy: %w", deleteErr))
	} else {
		pending = pending || !done
	}
	return pending, errors.Join(errs...)
}

// deleteOwnedObject deletes the named object in namespace only if it belongs to
// the given App identity (the artifactLabels UID). A foreign or platform object
// that happens to share the App-controlled name — possible in the SHARED build
// namespace, where object names derive from the workspace-local App name — is
// left untouched. NotFound and not-owned are both "done" (true, nothing to wait
// on); false means a delete was issued and the caller should requeue to observe
// it settle — the same semantics as deleteAndWait, plus the ownership gate
// (codex-security #5; extended to the execution NetworkPolicy and the
// build-namespace reg-pull mirror in codex-security round 12, finding 1).
func deleteOwnedObject(ctx context.Context, cl client.Client, namespace, name string, obj client.Object, identity execution.ArtifactIdentity) (bool, error) {
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	if !identity.Owns(obj) {
		return true, nil // not ours — leave a foreign/platform object intact
	}
	if !obj.GetDeletionTimestamp().IsZero() {
		return false, nil
	}
	if err := cl.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	return false, nil
}

func (r *AppReconciler) reclaimAppExternalArtifacts(ctx context.Context, app *appv1alpha1.App, namespace string, cl client.Client) (bool, error) {
	var errs []error
	pending := false
	// Stop the Ingress/cert-manager producers before deleting their TLS Secret.
	// Otherwise ingress-shim can recreate the Secret on every finalizer pass and
	// make an otherwise residue-free image service wait indefinitely.
	tlsProducersDone, err := r.quiesceTLSProducers(ctx, app)
	pending = pending || !tlsProducersDone
	errs = append(errs, err)
	if tlsProducersDone {
		tlsDone, tlsErr := r.deleteTLSSecrets(ctx, app)
		pending = pending || !tlsDone
		errs = append(errs, tlsErr)
	}
	if app.Spec.Type == appv1alpha1.TypeStaticSite {
		if !r.StaticStore.Configured() {
			errs = append(errs, fmt.Errorf("static purge configuration is unavailable"))
		} else if credentialErr := validateStaticCredentialSecret(ctx, r.buildPlaneClient(), namespace, r.StaticStore.Secret); credentialErr != nil {
			errs = append(errs, credentialErr)
		} else {
			purgeJob := publish.PurgeJob(app.Name, string(app.UID), app.Labels[labelWorkspace],
				app.Namespace, r.StaticStore, namespace, r.RegistryBuildPullSecret, "")
			done, purgeErr := reconcileCleanupJob(ctx, cl, app, purgeJob, annotStaticPurgeComplete)
			pending = pending || !done
			if purgeErr != nil {
				errs = append(errs, fmt.Errorf("purge static content: %w", purgeErr))
			}
		}
	}
	registryOwned := r.registryHosted(app.Status.Image) ||
		r.registryHosted(app.Status.ArtifactImage) || app.Spec.Repo != ""
	if r.Registry != "" && registryOwned && app.Annotations[annotRegistryPurgeComplete] != registryCredentialRotateTrue {
		done, registryErr := r.deleteRegistryRepo(ctx, app)
		pending = pending || !done
		if registryErr != nil {
			errs = append(errs, fmt.Errorf("delete registry images: %w", registryErr))
		} else if done {
			before := client.MergeFrom(app.DeepCopy())
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, annotRegistryPurgeComplete, registryCredentialRotateTrue)
			if patchErr := r.Patch(ctx, app, before); patchErr != nil {
				errs = append(errs, fmt.Errorf("record registry purge completion: %w", patchErr))
			}
		}
	}
	return pending, errors.Join(errs...)
}

// validateStaticCredentialSecret fails before a publish/purge Pod is created,
// so a missing or malformed build-namespace credential becomes an actionable
// App condition instead of a Job stuck in CreateContainerConfigError. The
// operator's direct build client is RBAC-scoped to this namespace and never
// reads the static-server's separate bex-system read credential.
func validateStaticCredentialSecret(ctx context.Context, cl client.Client, namespace, name string) error {
	var secret corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &secret); err != nil {
		return fmt.Errorf("static publish credential Secret %s/%s is unavailable: %w", namespace, name, err)
	}
	for _, key := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
		if len(secret.Data[key]) == 0 {
			return fmt.Errorf("static publish credential Secret %s/%s is missing key %s", namespace, name, key)
		}
	}
	return nil
}

func (r *AppReconciler) revokeAppRegistryCredentials(ctx context.Context, app *appv1alpha1.App, namespace string, cl client.Client) (bool, error) {
	if r.PerAppRegistry == nil {
		return false, nil
	}
	var errs []error
	pending := false
	if err := r.PerAppRegistry.RevokeCreds(ctx, app.Name); err != nil {
		errs = append(errs, fmt.Errorf("revoke per-app registry credentials: %w", err))
	}
	pullSec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: registry.PullSecretName(app.Name), Namespace: app.Namespace}}
	if done, deleteErr := deleteAndWait(ctx, cl, pullSec); deleteErr != nil {
		errs = append(errs, fmt.Errorf("delete per-app pull Secret: %w", deleteErr))
	} else {
		pending = pending || !done
	}
	if namespace != app.Namespace {
		// round 12, finding 1: the build-namespace reg-pull mirror carries the
		// same workspace-local App name and a same-named App in another
		// workspace builds and pulls with the object mirrored here — the bare
		// deleteAndWait would revoke ITS credential too. Gate on ownership.
		identity := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
			Workspace: app.Labels[labelWorkspace], Namespace: app.Namespace}
		done, deleteErr := deleteOwnedObject(ctx, cl, namespace, pullSec.Name, &corev1.Secret{}, identity)
		if deleteErr != nil {
			errs = append(errs, fmt.Errorf("delete build-namespace registry Secret: %w", deleteErr))
		} else {
			pending = pending || !done
		}
	}
	return pending, errors.Join(errs...)
}

func (r *AppReconciler) knownBuildSecretNames(app *appv1alpha1.App) []string {
	names := []string{build.JobName(app.Name, "registry-auth")}
	for _, name := range []string{
		app.Spec.CloneSecret,
		runtimeEnvSecret(app),
		app.Spec.ExternalRegistryPullSecret,
		registry.PullSecretName(app.Name),
	} {
		if name != "" {
			names = append(names, name)
		}
	}
	names = append(names, app.Spec.EnvFromSecrets...)
	names = append(names, app.Spec.FilesFromSecrets...)
	if app.Spec.EnvFromSecret != "" {
		names = append(names, app.Spec.EnvFromSecret)
	}
	sort.Strings(names)
	return slices.Compact(names)
}

// recordTLSSecretHistory remembers every cert-manager TLS Secret ever selected
// for this App. cert-manager does not garbage-collect the Secret when the Certificate
// or Ingress is removed, so they would otherwise persist indefinitely. The
// Ingress is already cascaded via ownerRef; this cleans up the Secret it left.
func (r *AppReconciler) recordTLSSecretHistory(ctx context.Context, app *appv1alpha1.App) error {
	names := tlsSecretHistory(app)
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		seen[name] = struct{}{}
	}
	changed := false
	for idx, host := range effectiveHosts(app, r.BaseDomain) {
		name := tlsSecretName(app.Name, idx, host)
		if _, ok := seen[name]; !ok {
			names = append(names, name)
			seen[name] = struct{}{}
			changed = true
		}
	}
	if !changed {
		return nil
	}
	sort.Strings(names)
	raw, err := json.Marshal(names)
	if err != nil {
		return err
	}
	before := client.MergeFrom(app.DeepCopy())
	metav1.SetMetaDataAnnotation(&app.ObjectMeta, annotTLSSecretHistory, string(raw))
	return r.Patch(ctx, app, before)
}

func tlsSecretHistory(app *appv1alpha1.App) []string {
	var names []string
	if raw := app.Annotations[annotTLSSecretHistory]; raw != "" {
		_ = json.Unmarshal([]byte(raw), &names)
	}
	return names
}

// tlsSecretNameSet is every cert-manager TLS Secret name this App may own: the
// recorded history plus the names derived from the current effective hosts.
func (r *AppReconciler) tlsSecretNameSet(app *appv1alpha1.App) map[string]struct{} {
	names := make(map[string]struct{})
	for _, name := range tlsSecretHistory(app) {
		names[name] = struct{}{}
	}
	for idx, host := range effectiveHosts(app, r.BaseDomain) {
		names[tlsSecretName(app.Name, idx, host)] = struct{}{}
	}
	return names
}

// quiesceTLSProducers removes the current Ingress and its ingress-shim
// Certificates before TLS Secret cleanup. Kubernetes does not garbage-collect
// App-owned children while the App's finalizer is still present, so relying on
// owner references here creates a cycle: the finalizer waits for the Secret,
// while the live Certificate keeps recreating it. Each stage is observed absent
// on a later pass before the next one starts.
func (r *AppReconciler) quiesceTLSProducers(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	ingress := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	ingressDone, err := deleteAndWait(ctx, r.Client, ingress)
	if err != nil {
		return false, fmt.Errorf("delete TLS Ingress: %w", err)
	}
	if !ingressDone {
		return false, nil
	}
	if r.ClusterIssuer == "" {
		return true, nil
	}

	var errs []error
	pending := false
	for name := range r.tlsSecretNameSet(app) {
		certificate := &unstructured.Unstructured{}
		certificate.SetGroupVersionKind(certManagerCertificateGVK)
		certificate.SetName(name)
		certificate.SetNamespace(app.Namespace)
		done, deleteErr := deleteAndWait(ctx, r.Client, certificate)
		pending = pending || !done
		if deleteErr != nil {
			errs = append(errs, fmt.Errorf("delete TLS Certificate %s: %w", name, deleteErr))
		}
	}
	return !pending, errors.Join(errs...)
}

func (r *AppReconciler) deleteTLSSecrets(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	secretClient := r.buildPlaneClient()
	names := r.tlsSecretNameSet(app)
	// Migration safety for Apps whose custom host was removed before m61 wrote
	// the history annotation. These names are operator-reserved for this App.
	var secrets corev1.SecretList
	if err := secretClient.List(ctx, &secrets, client.InNamespace(app.Namespace)); err != nil {
		return false, fmt.Errorf("list TLS Secrets: %w", err)
	}
	for idx := range secrets.Items {
		name := secrets.Items[idx].Name
		if name == app.Name+"-tls" || strings.HasPrefix(name, app.Name+"-tls-") {
			names[name] = struct{}{}
		}
	}
	var errs []error
	pending := false
	for name := range names {
		sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
		done, err := deleteAndWait(ctx, secretClient, sec)
		pending = pending || !done
		if err != nil {
			errs = append(errs, fmt.Errorf("delete TLS Secret %s: %w", name, err))
		}
	}
	return !pending, errors.Join(errs...)
}

// deleteRegistryRepo removes all manifests for the app's image repository from
// the in-cluster OCI registry via the OCI Distribution Spec v2 HTTP API. Any
// error retains the finalizer; after manifest deletion a later list must prove
// the repository empty. Zot's periodic GC reclaims the unreferenced blobs.
func (r *AppReconciler) deleteRegistryRepo(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	requestCtx, cancel := boundedhttp.WithTimeout(ctx)
	defer cancel()
	ctx = requestCtx
	base := registry.NormalizeBase(r.Registry)
	repo := app.Name

	authHdr, ready, err := r.registryAuthHeader(ctx, app)
	if err != nil {
		return false, fmt.Errorf("registry auth: %w", err)
	}
	if !ready {
		return false, nil
	}

	do := func(method, url string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept",
			"application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json")
		if authHdr != "" {
			req.Header.Set("Authorization", authHdr)
		}
		httpClient := r.HTTPClient
		if httpClient == nil {
			httpClient = boundedhttp.Shared
		}
		return httpClient.Do(req)
	}

	resp, err := do(http.MethodGet, fmt.Sprintf("%s/v2/%s/tags/list", base, repo))
	if err != nil {
		return false, fmt.Errorf("list tags: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return true, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := boundedhttp.ReadAll(resp.Body)
		return false, fmt.Errorf("list tags: status %d: %s", resp.StatusCode, body)
	}
	var tagList struct {
		Tags []string `json:"tags"`
	}
	if err := boundedhttp.DecodeJSON(resp.Body, &tagList); err != nil {
		return false, fmt.Errorf("decode tags: %w", err)
	}
	if len(tagList.Tags) == 0 {
		return true, nil
	}

	seen := map[string]bool{}
	for _, tag := range tagList.Tags {
		r2, err := do(http.MethodHead, fmt.Sprintf("%s/v2/%s/manifests/%s", base, repo, tag))
		if err != nil {
			return false, fmt.Errorf("head manifest %s: %w", tag, err)
		}
		_ = r2.Body.Close()
		if r2.StatusCode == http.StatusNotFound {
			continue
		}
		if r2.StatusCode != http.StatusOK {
			return false, fmt.Errorf("head manifest %s: status %d", tag, r2.StatusCode)
		}
		digest := r2.Header.Get("Docker-Content-Digest")
		if digest == "" {
			return false, fmt.Errorf("head manifest %s: missing Docker-Content-Digest", tag)
		}
		if seen[digest] {
			continue
		}
		seen[digest] = true
		r3, err := do(http.MethodDelete, fmt.Sprintf("%s/v2/%s/manifests/%s", base, repo, digest))
		if err != nil {
			return false, fmt.Errorf("delete manifest %s: %w", digest, err)
		}
		body, _ := boundedhttp.ReadAll(r3.Body)
		_ = r3.Body.Close()
		if r3.StatusCode != http.StatusAccepted && r3.StatusCode != http.StatusOK {
			return false, fmt.Errorf("delete manifest %s: status %d: %s", digest, r3.StatusCode, body)
		}
	}
	return false, nil
}

// registryAuthHeader returns the least-privilege credential used to delete the
// App's own repository and a ready flag. In per-App mode it repairs credentials
// revoked by older finalizer code, then waits for Zot to activate them before
// cleanup proceeds. Shared-secret mode is retained as a fallback; a configured
// but missing/malformed Secret is an explicit retryable error, never an
// anonymous request that degrades into an opaque 401.
func (r *AppReconciler) registryAuthHeader(ctx context.Context, app *appv1alpha1.App) (string, bool, error) {
	if r.PerAppRegistry != nil {
		if err := r.PerAppRegistry.EnsureCreds(ctx, app.Name, app.Namespace); err != nil {
			return "", false, fmt.Errorf("restore per-app credential: %w", err)
		}
		active, err := r.PerAppRegistry.EnsureActive(ctx, app.Name, app.Namespace)
		if err != nil {
			return "", false, fmt.Errorf("activate per-app credential: %w", err)
		}
		if !active {
			return "", false, nil
		}
		var sec corev1.Secret
		if err := r.PerAppRegistry.Client.Get(ctx, client.ObjectKey{
			Namespace: app.Namespace, Name: registry.PullSecretName(app.Name),
		}, &sec); err != nil {
			return "", false, fmt.Errorf("read per-app pull Secret: %w", err)
		}
		header, err := dockerConfigAuthHeader(sec.Data, r.Registry)
		if err != nil {
			return "", false, fmt.Errorf("per-app pull Secret: %w", err)
		}
		return header, true, nil
	}

	if r.RegistryPushSecret == "" {
		return "", true, nil
	}
	ns := r.buildNamespace(app.Namespace)
	cl := r.buildPlaneClient()
	var sec corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: r.RegistryPushSecret}, &sec); err != nil {
		return "", false, fmt.Errorf("read configured push Secret %s/%s: %w", ns, r.RegistryPushSecret, err)
	}
	header, err := dockerConfigAuthHeader(sec.Data, r.Registry)
	if err != nil {
		return "", false, fmt.Errorf("push Secret %s/%s: %w", ns, r.RegistryPushSecret, err)
	}
	return header, true, nil
}

func dockerConfigAuthHeader(secretData map[string][]byte, registryHost string) (string, error) {
	data := secretData[corev1.DockerConfigJsonKey]
	if len(data) == 0 {
		data = secretData["config.json"]
	}
	if len(data) == 0 {
		return "", fmt.Errorf("has neither %s nor config.json", corev1.DockerConfigJsonKey)
	}
	var cfg struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse docker config: %w", err)
	}
	entry, ok := cfg.Auths[registryHost]
	if !ok {
		return "", fmt.Errorf("docker config has no auth entry for %s", registryHost)
	}
	auth := strings.TrimSpace(entry.Auth)
	if auth == "" && (entry.Username != "" || entry.Password != "") {
		auth = base64.StdEncoding.EncodeToString([]byte(entry.Username + ":" + entry.Password))
	}
	if auth == "" {
		return "", fmt.Errorf("docker config auth entry for %s is empty", registryHost)
	}
	return "Basic " + auth, nil
}
