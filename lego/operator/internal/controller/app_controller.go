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
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	"github.com/bex-co/bex/lego/operator/internal/publish"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	bexruntime "github.com/bex-co/bex/lego/operator/internal/runtime"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const finalizer = "app.bex.co/finalizer"

// generationOrDeletionPredicate keeps the normal generation-only reconcile
// filter while admitting the metadata-only update Kubernetes emits when an App
// is deleted. DeletionTimestamp does not bump generation; filtering that event
// strands the App's finalizer and every owned workload indefinitely.
type generationOrDeletionPredicate struct {
	predicate.GenerationChangedPredicate
}

func (p generationOrDeletionPredicate) Update(e event.UpdateEvent) bool {
	return p.GenerationChangedPredicate.Update(e) ||
		(e.ObjectNew != nil && !e.ObjectNew.GetDeletionTimestamp().IsZero())
}

// labelApp marks the workloads bex creates for an App.
const labelApp = "app.bex.co/app"

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

// annotLastActive records when the app last served (or received) traffic.
// Set by the operator on first Running and reset on each wake; updated by the
// activator on each inbound request. Free-tier apps auto-hibernate after
// idleTTLSeconds past this timestamp.
const annotLastActive = "app.bex.co/last-active"

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
	BuildClient          client.Client
	Scheme               *runtime.Scheme
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
	// (BEX_STATIC_S3_ENDPOINT/BUCKET/SECRET). Unconfigured => static_site Apps are
	// rejected with a clear status, the way unset backup vars disable backups.
	StaticStore publish.Store
	// StaticServerService is the k8s Service name of the shared static-server that
	// serves static_site content; a static-site host's Ingress backs onto it (the
	// same by-name backend wiring the activator uses). Empty => static_site
	// serving is unavailable.
	StaticServerService string
	// StaticServerPort is the static-server Service port (default 8080).
	StaticServerPort int
	// TenantSignKeySecret names a Secret (in the build namespace, keys
	// "cosign.key"+"cosign.password") that enables tenant-image signing in the
	// in-cluster build Job (w6/006). Empty => tenant images unsigned (the default).
	TenantSignKeySecret string
	// TenantSignImage overrides the cosign image used by the signing container.
	TenantSignImage string
	// RegistryPushSecret names a docker-config Secret (in the build namespace,
	// key "config.json") the build Job mounts to authenticate its push against an
	// auth-enabled registry (w7/m8). Empty => unauthenticated push (dev default).
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
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kpack.io,resources=images,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=kpack.io,resources=builds,verbs=get;list;watch
// +kubebuilder:rbac:groups=traefik.io,resources=middlewares,verbs=get;list;watch;create;update;patch;delete

// traefikHTTPMiddlewareGVK is Traefik's HTTP middleware CRD (v3). Used to
// gate the App Ingress with an ipAllowList (source-range) middleware.
var traefikHTTPMiddlewareGVK = schema.GroupVersionKind{Group: "traefik.io", Version: "v1alpha1", Kind: "Middleware"}

const traefikRouterMiddlewaresAnnotation = "traefik.ingress.kubernetes.io/router.middlewares"

//nolint:gocyclo // Reconcile intentionally coordinates the App state machine's distinct phases.
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
	if controllerutil.AddFinalizer(&app, finalizer) {
		if err := r.Update(ctx, &app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil // finalizer update doesn't bump generation
	}

	// Registry auth is operator-level configuration, so converge it before a
	// source build can halt this pass. A failed newer build deliberately leaves
	// the previous Deployment/CronJob serving; without this early backfill that
	// older workload could retain an unauthenticated pod template forever and
	// fail as soon as Kubernetes reschedules it onto a fresh node.
	if r.Mode == ModeKubernetes {
		// Per-App pull credentials (w7/m36): mint before the backfill so the
		// "reg-pull-<name>" Secret exists by the time backfill patches workloads.
		if err := r.ensurePerAppRegistryCreds(ctx, &app); err != nil {
			if errors.Is(err, registry.ErrConflictRequeue) {
				return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
			}
			return r.fail(ctx, &app, "PerAppRegistryCredsFailed", err)
		}
		if err := r.backfillWorkloadPullSecrets(ctx, &app); err != nil {
			return r.fail(ctx, &app, "RegistryPullSecretFailed", err)
		}
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
	// Direct static publish (w9/010): a repo-backed static_site with no
	// Dockerfile path, no build command, and no native runtime has nothing to
	// build — Render publishes the repo's publish directory as-is. Skip the
	// build plane entirely; reconcileStaticSite dispatches a clone-mode publish
	// Job (image stays ""). An explicit dockerfilePath/buildCommand/runtime/
	// builder keeps the build path (the built image's publishPath holds the
	// site).
	if directStaticPublish(&app) {
		if r.Mode == ModeKubernetes {
			return r.reconcileKubernetes(ctx, &app, "", port)
		}
		return r.fail(ctx, &app, "BadSpec", fmt.Errorf("static_site requires the kubernetes runtime"))
	}
	if image == "" {
		if app.Spec.Repo == "" {
			return r.fail(ctx, &app, "BadSpec", fmt.Errorf("one of spec.image or spec.repo is required"))
		}
		ref := app.Spec.Branch
		if ref == "" {
			ref = "main"
		}
		// A commitId override from the deploy API (spec.BuildCommit) takes
		// precedence over the tracked branch for this single build. The next
		// trigger without a commitId resets BuildCommit to "" via the API patch,
		// so Branch HEAD is the default for every subsequent deploy.
		if app.Spec.BuildCommit != "" {
			ref = app.Spec.BuildCommit
		}
		// Tag by generation: a spec/revision bump (including a webhook redeploy that
		// stamps spec.restartedAt) yields a new tag, so the built image is fresh and
		// the Deployment re-pulls it. An in-cluster BuildKit Job does the work — no
		// docker daemon on the node.
		buildNs := r.buildNamespace(app.Namespace)
		builder := effectiveBuilder(app.Spec)

		// Newest-wins per App (w7/m9): cancel any active build Job for this service
		// so a push-spam burst never runs more than one build at a time per App,
		// matching Render's "Render cancels any in-progress build for the same service".
		buildClient := r.buildPlaneClient()
		if err := build.CancelActiveBuilds(ctx, app.Name, buildNs, buildClient); err != nil {
			return r.fail(ctx, &app, "BuildFailed", fmt.Errorf("cancelling superseded build: %w", err))
		}

		// Per-workspace concurrent-build cap (w7/m9): hold off when the workspace is
		// at its limit — requeue until a slot opens rather than starting another.
		// Byte-identical when MaxConcurrentBuilds == 0 (unset) or workspace absent.
		if r.MaxConcurrentBuilds > 0 {
			if workspace := app.Labels[labelWorkspace]; workspace != "" {
				active, err := build.ActiveWorkspaceBuilds(ctx, workspace, buildNs, buildClient)
				if err != nil {
					return r.fail(ctx, &app, "BuildFailed", fmt.Errorf("counting workspace builds: %w", err))
				}
				if active >= r.MaxConcurrentBuilds {
					r.setPhase(ctx, &app, appv1alpha1.PhaseBuilding, "BuildQueued",
						fmt.Sprintf("workspace has %d/%d concurrent builds active; waiting for a slot", active, r.MaxConcurrentBuilds))
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
			}
		}

		r.setPhase(ctx, &app, appv1alpha1.PhaseBuilding, "Building", "Building image from "+app.Spec.Repo)
		// The clone Secret (docs/ADR026-github-integration.md) that bex-api wrote lives
		// in the App's namespace, but the build Job runs in buildNs
		// (BEX_BUILD_NAMESPACE). When they differ, relocate it so BuildKit can read
		// GIT_AUTH_TOKEN — otherwise the build pod is CreateContainerConfigError
		// ("secret not found"). Mechanism-only: the operator just copies an opaque
		// Secret across namespaces; it never mints or inspects the token.
		if app.Spec.CloneSecret != "" && buildNs != app.Namespace {
			if err := r.copyCloneSecret(ctx, app.Namespace, buildNs, app.Spec.CloneSecret); err != nil {
				return r.fail(ctx, &app, "BuildFailed", fmt.Errorf("relocating clone secret to %s: %w", buildNs, err))
			}
		}
		runtimeSecret := runtimeEnvSecret(&app)
		if builder == build.BuilderNative && runtimeSecret != "" && buildNs != app.Namespace {
			if err := r.copyCloneSecret(ctx, app.Namespace, buildNs, runtimeSecret); err != nil {
				return r.fail(ctx, &app, "BuildFailed", fmt.Errorf("relocating native build env to %s: %w", buildNs, err))
			}
		}
		buildRegistrySecret, err := r.prepareBuildRegistrySecret(ctx, &app, buildNs, builder)
		if err != nil {
			return r.fail(ctx, &app, "BuildFailed", fmt.Errorf("preparing Docker-build registry credential: %w", err))
		}
		res, err := build.Build(ctx, build.Options{
			Repo: app.Spec.Repo, Ref: ref, RootDir: app.Spec.RootDir,
			DockerfilePath: app.Spec.DockerfilePath, Name: app.Name,
			Registry: r.Registry, KpackRegistry: r.KpackRegistry,
			Builder:          builder,
			Runtime:          app.Spec.Runtime,
			BuildCommand:     app.Spec.BuildCommand,
			StartCommand:     app.Spec.StartCommand,
			BuildEnv:         buildEnv(builder, app.Spec.Env),
			RuntimeEnvSecret: runtimeSecret,
			Revision:         fmt.Sprintf("gen-%d", app.Generation),
			Namespace:        buildNs,
			AppNamespace:     app.Namespace,
			Workspace:        app.Labels[labelWorkspace],
			CloneSecret:      app.Spec.CloneSecret,
			SignKeySecret:    r.TenantSignKeySecret,
			SignImage:        r.TenantSignImage,
			PushSecret:       buildRegistrySecret,
			RegistryConfig:   usesBuildRegistryConfig(&app, builder),
			Client:           buildClient,
		})
		if err != nil {
			return r.fail(ctx, &app, "BuildFailed", err)
		}
		image = r.pinBuiltImage(ctx, &app, res.Image)
	}

	if r.Mode == ModeKubernetes {
		return r.reconcileKubernetes(ctx, &app, image, port)
	}
	return r.reconcileOpenSandbox(ctx, &app, image, port)
}

// pinBuiltImage rewrites a freshly built mutable-tag reference to its immutable
// `<repo>:<tag>@<digest>` form (w9/013). The per-generation `gen-N` tag RESETS
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
	switch spec.Runtime {
	case runtimeDocker:
		return build.BuilderDockerfile
	case "elixir", "go", "node", "python", "ruby", "rust":
		return build.BuilderNative
	default:
		return spec.Builder
	}
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
	if r.BuildNamespace != "" {
		return r.BuildNamespace
	}
	return appNS
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
		if s := r.RegistryBuildPullSecret; s != "" {
			out = append(out, corev1.LocalObjectReference{Name: s})
		}
	}
	if app.Spec.ExternalRegistryPullSecret != "" {
		if err := r.copyCloneSecret(ctx, app.Namespace, ns, app.Spec.ExternalRegistryPullSecret); err != nil {
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
func (r *AppReconciler) mirrorPreDeploySecrets(ctx context.Context, app *appv1alpha1.App, ns string) error {
	if ns == app.Namespace {
		return nil
	}
	names := append([]string{}, app.Spec.EnvFromSecrets...)
	names = append(names, app.Spec.FilesFromSecrets...)
	if app.Spec.EnvFromSecret != "" {
		names = append(names, app.Spec.EnvFromSecret)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if err := r.copyCloneSecret(ctx, app.Namespace, ns, name); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("relocating secret %s to %s: %w", name, ns, err)
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
		return r.RegistryBuildPullSecret
	}
	if r.PerAppRegistry != nil {
		return registry.PullSecretName(app.Name)
	}
	return r.RegistryPullSecret
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

// ensurePerAppRegistryCreds mints per-App pull credentials (w7/m36) when
// PerAppRegistry is configured and the App needs a registry-hosted image. It is
// a no-op when PerAppRegistry is nil or the App is not registry-hosted.
func (r *AppReconciler) ensurePerAppRegistryCreds(ctx context.Context, app *appv1alpha1.App) error {
	if r.PerAppRegistry == nil || r.Registry == "" {
		return nil
	}
	// Mint credentials if the App will build+push to our registry (Repo-based)
	// or already has a registry-hosted image in spec or status.
	needsCred := app.Spec.Repo != "" ||
		strings.HasPrefix(app.Spec.Image, r.Registry+"/") ||
		strings.HasPrefix(app.Status.Image, r.Registry+"/")
	if !needsCred {
		return nil
	}

	// Rotation: if the annotation is set, re-issue credentials first, then clear it.
	if app.Annotations[annotRotateRegistryCreds] == "true" {
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
func (r *AppReconciler) copyCloneSecret(ctx context.Context, srcNS, dstNS, name string) error {
	buildClient := r.buildPlaneClient()
	var src corev1.Secret
	if err := buildClient.Get(ctx, client.ObjectKey{Namespace: srcNS, Name: name}, &src); err != nil {
		return err
	}
	dst := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: dstNS}}
	_, err := controllerutil.CreateOrUpdate(ctx, buildClient, dst, func() error {
		dst.Type = src.Type
		dst.Data = src.Data
		if dst.Labels == nil {
			dst.Labels = map[string]string{}
		}
		dst.Labels[labelApp] = name
		dst.Labels["app.bex.co/component"] = "clone-secret"
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
//
//nolint:gocyclo // one cohesive linear reconcile pass (Deployment → Service → Ingress → status); each step is a guarded CreateOrUpdate, and splitting it solely to satisfy the counter would fragment a coherent unit without aiding readability.
func (r *AppReconciler) reconcileKubernetes(ctx context.Context, app *appv1alpha1.App, image string, port int) (ctrl.Result, error) {
	// Registry credential activation gate (w9/m43): zot only loads per-App
	// credentials at process start (see registry/verify.go), so before rolling
	// any workload (Deployment, CronJob, static-site publish Job) to a
	// registry-hosted image, verify zot accepts this App's credential.
	// Suspended Apps skip the gate: they scale to zero and pull nothing.
	if r.PerAppRegistry != nil && !app.Spec.Suspended && r.registryHosted(image) {
		active, err := r.PerAppRegistry.EnsureActive(ctx, app.Name, app.Namespace)
		if err != nil {
			// Probe impossible (zot or the pull Secret unreachable) — a platform
			// hiccup, not an App failure; keep the previous revision serving and retry.
			logf.FromContext(ctx).Error(err, "registry credential probe failed; requeueing", "app", app.Name)
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
		if !active {
			r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "RegistryCredsPending",
				"Waiting for the registry to accept this app's pull credential")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
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

	r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Deploying", "Reconciling Deployment for "+image)

	// Auto-hibernate: idle free-tier app past its TTL → scale to 0 without
	// touching spec.suspended, so manual-suspend semantics are preserved. A worker
	// never auto-hibernates — it has no Ingress, so a request could never wake it.
	autoHibernating := !worker && r.ActivatorService != "" && shouldAutoHibernate(app)

	replicas := effectiveReplicas(app)
	// Seed from the autoscaler annotation so a metrics-failure pass doesn't revert
	// to spec.replicas (the user's static count). applyAutoscaling writes the
	// annotation instead of spec.replicas to avoid bumping generation (see annotAutoscaleReplicas).
	var autoscaleRequeue bool
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

	labels := map[string]string{labelApp: app.Name}
	appID := app.Labels[labelAppID]
	if appID == "" {
		appID = app.Name
	}
	// Propagate the workspace label to pod templates so NetworkPolicy selectors
	// can express "allow same-workspace" rules. The selector stays stable
	// (labelApp only) — adding labelWorkspace here would make it immutable and
	// break existing Deployments when the label is added later.
	podLabels := map[string]string{
		labelApp:      app.Name,
		labelAppID:    appID,
		labelRevision: fmt.Sprintf("rev-%d", app.Generation),
	}
	if ws := app.Labels[labelWorkspace]; ws != "" {
		podLabels[labelWorkspace] = ws
	}
	// Same mutability rationale as labelWorkspace above: labelNetworkIsolation
	// is only present on the App when its Environment has
	// networkIsolationEnabled (w6/m19) — reconcileNetworkPolicy's
	// scopeSelector needs it on the pod template to actually select these pods.
	if env := app.Labels[labelNetworkIsolation]; env != "" {
		podLabels[labelNetworkIsolation] = env
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
		container := corev1.Container{
			Name:            "app",
			Image:           image,
			Env:             appEnv(app, port),
			EnvFrom:         envFromSources(app),
			Ports:           ports,
			Resources:       resourcesForTier(app.Spec.Tier),
			SecurityContext: tenantSecCtx(),
		}
		// StartCommand overrides the running container's entrypoint whenever the
		// image comes from an opaque Dockerfile (or a prebuilt image) — bex has
		// no control over that image's own ENTRYPOINT/CMD. A native-runtime or
		// buildpack build instead bakes the command into the image's own CMD at
		// build time, so no Deployment-level override is needed there.
		if b := effectiveBuilder(app.Spec); b != build.BuilderNative && b != build.BuilderBuildpack && app.Spec.StartCommand != "" {
			container.Command = []string{"/bin/sh", "-c", app.Spec.StartCommand}
		}
		// Health-gating: a non-worker service speaks HTTP, so gate pod
		// readiness on GET spec.healthCheckPath — Render's health check. A
		// 2xx/3xx (k8s' default success range) makes the pod ready and routes
		// traffic; a failure pulls it out of the Service until it recovers.
		// Empty defaults to "/", matching the CRD's +kubebuilder:default.
		// Workers/cron/static_site have no HTTP port and diverge above.
		if !worker {
			path := app.Spec.HealthCheckPath
			if path == "" {
				path = "/"
			}
			container.ReadinessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromInt(port)},
			}}
		}
		// Secret files: one projected /etc/secrets volume (the service's own
		// "<name>-files" + each linked env group's files). Rebuilt every reconcile
		// so a removed source drops out cleanly.
		dep.Spec.Template.Spec.Volumes = nil
		if vol, mount := secretFileMounts(app); vol != nil {
			container.VolumeMounts = []corev1.VolumeMount{*mount}
			dep.Spec.Template.Spec.Volumes = []corev1.Volume{*vol}
		}
		dep.Spec.Template.Spec.Containers = []corev1.Container{container}
		// Render's maxShutdownDelaySeconds is Kubernetes' native pod termination
		// grace period. Keep nil when unset so existing Apps retain Kubernetes'
		// identical 30-second default without adding a field to their pod template.
		dep.Spec.Template.Spec.TerminationGracePeriodSeconds = terminationGracePeriodSeconds(app.Spec.MaxShutdownDelaySeconds)
		// Authenticate kubelet pulls against the auth-enabled registry (w7/m8). nil
		// when no pull secret is configured or the image isn't registry-hosted, so a
		// prebuilt public image is left untouched.
		dep.Spec.Template.Spec.ImagePullSecrets = r.imagePullSecrets(app, image)
		dep.Spec.Template.Spec.AutomountServiceAccountToken = ptr(false)
		return controllerutil.SetControllerReference(app, dep, r.Scheme)
	}); err != nil {
		return r.fail(ctx, app, "DeployFailed", err)
	}

	// Reconcile the per-App NetworkPolicy (docs/ADR022-tenant-isolation.md). Only when
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

	// Maintenance has public-routing precedence over both auto-hibernation and
	// manual suspension. It changes only the public Ingress backend; the workload
	// follows its independent replica/suspension policy.
	ingressSvc := app.Name
	ingressPort := int32(port)
	if maintenanceEnabled(app) {
		maintenanceSvc, err := r.reconcileMaintenanceAlias(ctx, app)
		if err != nil {
			return r.fail(ctx, app, "MaintenanceRoutingFailed", err)
		}
		ingressSvc = maintenanceSvc
		ingressPort = int32(r.maintenancePort())
	} else if autoHibernating {
		ingressSvc = r.ActivatorService
		ingressPort = int32(r.ActivatorPort)
	}

	mwNames, err := r.reconcileIPAllowListMiddleware(ctx, app)
	if err != nil {
		return r.fail(ctx, app, "MiddlewareFailed", err)
	}
	wsMeter, err := r.reconcileWebsocketMeterMiddleware(ctx, app, len(hosts) > 0)
	if err != nil {
		return r.fail(ctx, app, "MiddlewareFailed", err)
	}
	middlewareNames := mwNames
	if wsMeter != "" {
		middlewareNames = append(middlewareNames, wsMeter)
	}
	if err := r.reconcileIngress(ctx, app, hosts, ingressSvc, ingressPort, middlewareNames); err != nil {
		return r.fail(ctx, app, "IngressFailed", err)
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
		// Surface why the rollout is stuck when a pod is crash-looping or cannot
		// pull its image: the deploy would otherwise time out as an opaque
		// update_failed with the cause visible only in pod state no tenant
		// surface shows (w9/011).
		if reason, msg := r.stuckPodMessage(ctx, dep, port); msg != "" {
			meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
				Type: "Ready", Status: metav1.ConditionFalse, Reason: reason,
				Message: msg, ObservedGeneration: app.Generation,
			})
		}
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
	// Autoscaling: always requeue at the poll interval so the loop keeps running.
	if autoscaleRequeue {
		return ctrl.Result{RequeueAfter: autoscaleInterval}, nil
	}
	return ctrl.Result{}, nil
}

func terminationGracePeriodSeconds(seconds *int32) *int64 {
	if seconds == nil {
		return nil
	}
	value := int64(*seconds)
	return &value
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
		// Same stuck-rollout surfacing as the web path; port 0 — a worker has no
		// HTTP endpoint, so the $PORT hint is omitted (w9/011).
		if reason, msg := r.stuckPodMessage(ctx, dep, 0); msg != "" {
			meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
				Type: "Ready", Status: metav1.ConditionFalse, Reason: reason,
				Message: msg, ObservedGeneration: app.Generation,
			})
		}
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
		{"-ip-allow", app.Spec.IPAllowList},
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

func (r *AppReconciler) reconcileWebsocketMeterMiddleware(ctx context.Context, app *appv1alpha1.App, enabled bool) (string, error) {
	middlewareName := app.Name + "-ws-egress"
	if !enabled {
		if err := deleteOwned(ctx, r.Client, app, traefikHTTPMiddlewareGVK, middlewareName); err != nil {
			return "", err
		}
		return "", nil
	}
	appID := app.Labels[labelAppID]
	if appID == "" {
		appID = app.Name
	}
	spec := map[string]any{"plugin": map[string]any{
		"bexWebsocketEgress": map[string]any{"appId": appID, "metricsAddr": ":9101"},
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
		stale := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		if err := r.Delete(ctx, stale); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	ingressClass := "traefik"
	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, ing, func() error {
		if ing.Annotations == nil {
			ing.Annotations = map[string]string{}
		}
		if r.ClusterIssuer != "" {
			ing.Annotations["cert-manager.io/cluster-issuer"] = r.ClusterIssuer
		}
		if len(middlewareNames) > 0 {
			refs := make([]string, len(middlewareNames))
			for i, name := range middlewareNames {
				refs[i] = app.Namespace + "-" + name + "@kubernetescrd"
			}
			ing.Annotations[traefikRouterMiddlewaresAnnotation] = strings.Join(refs, ",")
		} else {
			delete(ing.Annotations, traefikRouterMiddlewaresAnnotation)
		}
		// Remove the pre-m30 nonstandard key. Traefik's Kubernetes Ingress
		// provider only reads the traefik.ingress.kubernetes.io namespace.
		delete(ing.Annotations, "traefik.io/router.middlewares")
		ing.Spec.IngressClassName = &ingressClass
		ing.Spec.TLS = nil
		ing.Spec.Rules = nil
		for i, host := range hosts {
			if redirectSources[host] {
				continue
			}
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
									Name: svc,
									Port: networkingv1.ServiceBackendPort{Number: port},
								},
							},
						}},
					},
				},
			})
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
		middlewareNames = append(middlewareNames, app.Namespace+"-"+name+"@kubernetescrd")
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
	if err := r.List(ctx, list, client.InNamespace(app.Namespace), client.MatchingLabels{labelHostRedirectOwner: app.Name}); err != nil {
		if !meta.IsNoMatchError(err) {
			return nil, err
		}
	} else {
		for i := range list.Items {
			if !desiredMiddleware[list.Items[i].GetName()] {
				if err := deleteOptionalObject(ctx, r.Client, &list.Items[i]); err != nil {
					return nil, err
				}
			}
		}
	}

	ingressName := hostRedirectIngressName(app.Name)
	if len(redirects) == 0 {
		stale := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: ingressName, Namespace: app.Namespace}}
		if err := r.Delete(ctx, stale); err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		return map[string]bool{}, nil
	}

	ingressClass := "traefik"
	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: ingressName, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ing, func() error {
		if ing.Annotations == nil {
			ing.Annotations = map[string]string{}
		}
		if r.ClusterIssuer != "" {
			ing.Annotations["cert-manager.io/cluster-issuer"] = r.ClusterIssuer
		}
		ing.Annotations[traefikRouterMiddlewaresAnnotation] = strings.Join(middlewareNames, ",")
		delete(ing.Annotations, "traefik.io/router.middlewares")
		ing.Labels = map[string]string{labelHostRedirectOwner: app.Name}
		ing.Spec.IngressClassName = &ingressClass
		ing.Spec.TLS = nil
		ing.Spec.Rules = nil
		for _, source := range sources {
			index := slices.Index(hosts, source)
			ing.Spec.TLS = append(ing.Spec.TLS, networkingv1.IngressTLS{
				Hosts: []string{source}, SecretName: tlsSecretName(app.Name, index, source),
			})
			ing.Spec.Rules = append(ing.Spec.Rules, networkingv1.IngressRule{
				Host: source,
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
		return controllerutil.SetControllerReference(app, ing, r.Scheme)
	}); err != nil {
		return nil, err
	}

	redirectSources := make(map[string]bool, len(redirects))
	for source := range redirects {
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
func (r *AppReconciler) reconcileStaticSite(ctx context.Context, app *appv1alpha1.App, image string) (ctrl.Result, error) {
	if app.Spec.PublishPath == "" {
		return r.fail(ctx, app, "BadSpec", fmt.Errorf("static_site requires spec.publishPath"))
	}
	if !r.StaticStore.Configured() || r.StaticServerService == "" {
		return r.fail(ctx, app, "StaticUnavailable",
			fmt.Errorf("static_site is unavailable: set BEX_STATIC_S3_ENDPOINT/BUCKET/SECRET and BEX_STATIC_SERVER_SERVICE"))
	}

	// Type-change cleanup: a static_site has no Deployment/Service of its own.
	for _, obj := range []client.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}},
	} {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return r.fail(ctx, app, "StaticCleanupFailed", err)
		}
	}

	rev := fmt.Sprintf("rev-%d", app.Generation)

	// Publish this revision's built output to the object store, unless it is
	// already the active revision (idempotent: the publish Job is named per
	// revision, so a retry adopts it rather than re-uploading).
	if app.Status.ActiveRevision != rev {
		r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Publishing", "Publishing static output to object store")
		opts := publish.Options{
			Image:       image,
			PublishPath: app.Spec.PublishPath,
			AppID:       app.Name,
			Revision:    rev,
			Store:       r.StaticStore,
			Namespace:   r.buildNamespace(app.Namespace),
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
			// repo and uploads rootDir/publishPath as-is. Same ref selection as
			// the build path (branch, with a one-shot commit override).
			ref := app.Spec.Branch
			if app.Spec.BuildCommit != "" {
				ref = app.Spec.BuildCommit
			}
			opts.Repo = app.Spec.Repo
			opts.Ref = ref
			opts.RootDir = app.Spec.RootDir
			opts.CloneSecret = app.Spec.CloneSecret
			opts.PullSecret = "" // clone mode pulls only public platform images
			// A private repo's clone Secret lives in the App namespace; relocate
			// it beside the publish Job when the build namespace differs (the
			// same seam the build plane uses).
			if app.Spec.CloneSecret != "" && opts.Namespace != app.Namespace {
				if err := r.copyCloneSecret(ctx, app.Namespace, opts.Namespace, app.Spec.CloneSecret); err != nil {
					return r.fail(ctx, app, "PublishFailed", fmt.Errorf("relocating clone secret to %s: %w", opts.Namespace, err))
				}
			}
		}
		if err := publish.Publish(ctx, opts); err != nil {
			return r.fail(ctx, app, "PublishFailed", err)
		}
	}

	// A static_site must be reachable at a host (there is no in-cluster-only mode
	// for served content — the static-server dispatches by Host).
	hosts := effectiveHosts(app, r.BaseDomain)
	if len(hosts) == 0 {
		return r.fail(ctx, app, "BadSpec",
			fmt.Errorf("static_site requires a host: set spec.expose (with BEX_BASE_DOMAIN), spec.host, or spec.hosts"))
	}

	// Suspended: stop serving by removing the Ingress; the published content stays
	// in the object store, so resume just recreates the Ingress.
	if app.Spec.Suspended {
		if _, err := r.reconcileWebsocketMeterMiddleware(ctx, app, false); err != nil {
			return r.fail(ctx, app, "MiddlewareCleanupFailed", err)
		}
		if err := r.reconcileIngress(ctx, app, nil, "", 0, nil); err != nil {
			return r.fail(ctx, app, "IngressCleanupFailed", err)
		}
		app.Status.Phase = appv1alpha1.PhaseHibernated
		app.Status.Image = image
		app.Status.ActiveRevision = rev
		app.Status.ObservedGeneration = app.Generation
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "Suspended",
			Message: "static site suspended (not served; published content kept)", ObservedGeneration: app.Generation,
		})
		if err := r.Status().Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	staticMWNames, err := r.reconcileIPAllowListMiddleware(ctx, app)
	if err != nil {
		return r.fail(ctx, app, "MiddlewareFailed", err)
	}
	if _, err := r.reconcileWebsocketMeterMiddleware(ctx, app, false); err != nil {
		return r.fail(ctx, app, "MiddlewareFailed", err)
	}
	if err := r.reconcileIngress(ctx, app, hosts, r.StaticServerService, int32(r.staticServerPort()), staticMWNames); err != nil {
		return r.fail(ctx, app, "IngressFailed", err)
	}

	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.Image = image
	app.Status.ActiveRevision = rev
	app.Status.ObservedGeneration = app.Generation
	app.Status.URL = "https://" + hosts[0]
	app.Status.URLs = nil
	for _, h := range hosts {
		app.Status.URLs = append(app.Status.URLs, "https://"+h)
	}
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Published",
		Message: "static site published and served from the object-store origin", ObservedGeneration: app.Generation,
	})
	if err := r.Status().Update(ctx, app); err != nil {
		return ctrl.Result{}, err
	}
	logf.FromContext(ctx).Info("static site running", "name", app.Name, "revision", rev, "image", image)
	return ctrl.Result{}, nil
}

// staticServerPort is the static-server Service port, defaulting to 8080.
func (r *AppReconciler) staticServerPort() int {
	if r.StaticServerPort != 0 {
		return r.StaticServerPort
	}
	return 8080
}

// defaultMaintenanceService is the wake-activator Service name assumed when
// MaintenanceService is unset — the same default cmd/manager wires for
// ActivatorService via BEX_ACTIVATOR_SERVICE.
const defaultMaintenanceService = "bex-activator"

func (r *AppReconciler) maintenanceService() string {
	if r.MaintenanceService != "" {
		return r.MaintenanceService
	}
	return defaultMaintenanceService
}

func (r *AppReconciler) maintenanceNamespace() string {
	if r.MaintenanceNamespace != "" {
		return r.MaintenanceNamespace
	}
	return "bex-system"
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
	alias := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, alias, func() error {
		alias.Spec.Type = corev1.ServiceTypeExternalName
		alias.Spec.ExternalName = fmt.Sprintf("%s.%s.svc.cluster.local", r.maintenanceService(), r.maintenanceNamespace())
		alias.Spec.Selector = nil
		alias.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: int32(r.maintenancePort())}}
		return controllerutil.SetControllerReference(app, alias, r.Scheme)
	}); err != nil {
		return "", fmt.Errorf("reconciling maintenance responder alias: %w", err)
	}
	return name, nil
}

func maintenanceAliasName(appName string) string {
	const prefix = "bex-maintenance-"
	if len(prefix)+len(appName) <= 63 {
		return prefix + appName
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(appName)))[:8]
	keep := 63 - len(prefix) - 1 - len(sum)
	return prefix + appName[:keep] + "-" + sum
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
	if appID := app.Labels[labelAppID]; appID != "" {
		labels[labelAppID] = appID
	} else {
		labels[labelAppID] = app.Name
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
	var hosts []string
	seen := map[string]bool{}
	add := func(h string) {
		if h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	add(app.Spec.Host)
	if app.Spec.Expose && baseDomain != "" && app.Spec.SubdomainPolicy != appv1alpha1.SubdomainPolicyDisabled {
		add(fmt.Sprintf("%s.%s", app.Spec.PlatformSubdomain(app.Name), baseDomain))
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
// Listed with the uncached client: pods are not in the manager cache and a
// cached List would establish a cluster-wide informer for them.
func (r *AppReconciler) stuckPodMessage(ctx context.Context, dep *appsv1.Deployment, port int) (string, string) {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return "", ""
	}
	var pods corev1.PodList
	if err := r.buildPlaneClient().List(ctx, &pods, client.InNamespace(dep.Namespace),
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
			}
		}
	}
	return "", ""
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

// reconcilePreDeploy runs spec.preDeployCommand to completion against image
// before the caller rolls the Deployment to it (w1/m33, docs/ADR004-deployment.md).
// It returns halt=true when the caller must stop this reconcile and return
// (res, err): either the step failed (err set — the previous revision keeps
// serving because the Deployment update below never runs) or it is still running
// (requeue). halt=false lets the rollout proceed — the step is unset or already
// succeeded for this revision.
//
// The step is gated per generation (bex treats every spec change as a new
// revision — a repo-backed App already rebuilds per generation), so it runs once
// per rollout, exactly like Render's Pre-Deploy Command, and never re-runs a
// migration that already passed or failed for the same revision. That terminal
// bookkeeping also guards against re-creating the Job after its TTL reap.
func (r *AppReconciler) reconcilePreDeploy(ctx context.Context, app *appv1alpha1.App, image string, port int) (ctrl.Result, bool, error) {
	if app.Spec.PreDeployCommand == "" {
		return ctrl.Result{}, false, nil
	}
	gen := app.Generation

	// firstForGen is the first reconcile that runs this revision's step; started
	// is preserved across the requeues that follow. Terminal per generation: a
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

	// Newest-wins: cancel any older-revision migration still in flight — but only
	// on the FIRST pass for this revision. Re-checking a running migration has
	// nothing newer to supersede (a newer generation would have reset firstForGen),
	// so this avoids a wasted List on every requeue.
	if firstForGen {
		if err := predeploy.CancelSuperseded(ctx, app.Name, ns, jobName, r.Client); err != nil {
			return r.failPreDeploy(ctx, app, &appv1alpha1.PreDeployStatus{
				Job: jobName, Generation: gen, Status: appv1alpha1.PreDeployFailed,
				StartedAt: started, FinishedAt: time.Now().UTC().Format(time.RFC3339), Message: err.Error(),
			}, fmt.Errorf("pre-deploy: %w", err))
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
		return r.failPreDeploy(ctx, app, &appv1alpha1.PreDeployStatus{
			Job: jobName, Generation: gen, Status: appv1alpha1.PreDeployFailed,
			StartedAt: started, FinishedAt: time.Now().UTC().Format(time.RFC3339), Message: err.Error(),
		}, fmt.Errorf("pre-deploy: %w", err))
	}
	job, err := predeploy.Ensure(ctx, predeploy.Options{
		Name:             app.Name,
		Namespace:        ns,
		Workspace:        app.Labels[labelWorkspace],
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
		return r.failPreDeploy(ctx, app, &appv1alpha1.PreDeployStatus{
			Job: job.Name, Generation: gen, Status: appv1alpha1.PreDeployFailed,
			StartedAt: started, FinishedAt: now, Message: msg,
		}, fmt.Errorf("pre-deploy command failed: %s", msg))
	default: // Pending/Running — keep the old revision serving and requeue
		app.Status.Phase = appv1alpha1.PhaseDeploying
		app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
			Job: job.Name, Generation: gen, Status: appv1alpha1.PreDeployRunning,
			StartedAt: started,
		}
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "PreDeploy",
			Message: "running pre-deploy command", ObservedGeneration: gen,
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

// reconcileNetworkPolicy creates or updates the per-App NetworkPolicy that
// enforces tenant isolation (docs/ADR022-tenant-isolation.md §per-app-networkpolicy).
// The policy is skipped for Apps without the workspace label (legacy/hand-applied
// Apps) — they communicate freely, consistent with prior behavior.
//
// Policy shape:
//   - Ingress: allow from same-scope pods + from the traefik namespace
//   - Egress: allow DNS (kube-system :53), same-scope pods/services,
//     and the public internet (not RFC1918/CGNAT — blocks in-cluster platforms)
//
// "same-scope" is same-workspace pods (labelWorkspace) normally, but when the
// App carries labelNetworkIsolation (w6/m19 protected-environment ACLs:
// networkIsolationEnabled=true on its Environment — internal/environments'
// setAppEnvironmentLabel stamps/clears it) it narrows to same-environment
// pods (labelNetworkIsolation) instead — denying traffic between that
// environment's Apps and everything else in the workspace, not just outside
// it. DNS, Traefik ingress, and public-internet egress are unaffected either
// way; those are infrastructural, not "other Apps".
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
	scopeSelector := map[string]string{labelWorkspace: ws}
	if env := app.Labels[labelNetworkIsolation]; env != "" {
		scopeSelector = map[string]string{labelNetworkIsolation: env}
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
						// same-scope pods (private services)
						{PodSelector: &metav1.LabelSelector{
							MatchLabels: scopeSelector,
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
				// same-scope pods and services
				{
					To: []networkingv1.NetworkPolicyPeer{
						{PodSelector: &metav1.LabelSelector{
							MatchLabels: scopeSelector,
						}},
					},
				},
				// public internet, minus RFC1918/CGNAT (in-cluster platforms) and
				// link-local 169.254.0.0/16 (cloud-metadata SSRF path; also
				// egressDeny-ed by the platform CNP — docs/ADR022-tenant-isolation.md)
				{
					To: []networkingv1.NetworkPolicyPeer{
						{IPBlock: &networkingv1.IPBlock{
							CIDR: "0.0.0.0/0",
							Except: []string{
								"10.0.0.0/8",
								"172.16.0.0/12",
								"192.168.0.0/16",
								"100.64.0.0/10",
								"169.254.0.0/16",
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
	workers := max(r.MaxConcurrentReconciles, 1)
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.App{}).
		Owns(&appsv1.Deployment{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&batchv1.CronJob{}).
		WithEventFilter(generationOrDeletionPredicate{}).
		WithOptions(crcontroller.Options{MaxConcurrentReconciles: workers}).
		Named("app").
		Complete(r)
}

// deleteTLSSecrets removes the cert-manager TLS Secrets for every host the App
// handleAppDeletion runs the finalizer teardown for an App being deleted:
// cleans up build artifacts, cert-manager TLS Secrets, static-site S3 content,
// and Zot registry images that ownerRefs can't cascade to. Extracted from
// Reconcile to keep its cyclomatic complexity in check.
func (r *AppReconciler) handleAppDeletion(ctx context.Context, app *appv1alpha1.App) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(app, finalizer) {
		log := logf.FromContext(ctx)
		if app.Status.SandboxID != "" {
			_ = r.Runtime.Delete(ctx, app.Status.SandboxID)
		}
		// Cross-namespace build/predeploy Jobs and kpack Images in the build
		// namespace — ownerRefs are invalid across namespaces so nothing cascades.
		ns := r.BuildNamespace
		if ns == "" {
			ns = app.Namespace
		}
		cl := r.Client
		if r.BuildClient != nil {
			cl = r.BuildClient
		}
		if err := build.DeleteAppArtifacts(ctx, app.Name, ns, cl); err != nil {
			log.Error(err, "delete build artifacts", "app", app.Name)
		}
		if err := r.deleteBuildRegistrySecret(ctx, app.Name, ns); err != nil {
			log.Error(err, "delete merged build registry credential", "app", app.Name)
		}
		// cert-manager TLS Secrets
		r.deleteTLSSecrets(ctx, app)
		// Static-site S3 prefix
		if app.Spec.Type == appv1alpha1.TypeStaticSite && r.StaticStore.Configured() {
			if err := publish.PurgeApp(ctx, app.Name, r.StaticStore, ns, cl,
				r.RegistryPullSecret, ""); err != nil {
				log.Error(err, "dispatch static purge job", "app", app.Name)
			}
		}
		// Registry images
		if r.Registry != "" && app.Status.Image != "" &&
			strings.HasPrefix(app.Status.Image, r.Registry+"/") {
			if err := r.deleteRegistryRepo(ctx, app); err != nil {
				log.Error(err, "delete registry images", "app", app.Name)
			}
		}
		// Per-App pull credential revocation (w7/m36): remove "app-<name>" from
		// zot-htpasswd and the per-repo ACL, then delete the pull Secret.
		if r.PerAppRegistry != nil {
			if err := r.PerAppRegistry.RevokeCreds(ctx, app.Name); err != nil {
				if errors.Is(err, registry.ErrConflictRequeue) {
					// Write conflict: defer cleanup to next reconcile. The finalizer
					// remains, so the App will be requeued automatically.
					return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
				}
				log.Error(err, "revoke per-app registry credentials", "app", app.Name)
			}
			pullSec := &corev1.Secret{}
			pullSec.Name = registry.PullSecretName(app.Name)
			pullSec.Namespace = app.Namespace
			if err := r.Delete(ctx, pullSec); err != nil && !apierrors.IsNotFound(err) {
				log.Error(err, "delete per-app pull secret", "app", app.Name)
			}
		}
		controllerutil.RemoveFinalizer(app, finalizer)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// served. cert-manager does not garbage-collect the Secret when the Certificate
// or Ingress is removed, so they would otherwise persist indefinitely. The
// Ingress is already cascaded via ownerRef; this cleans up the Secret it left.
func (r *AppReconciler) deleteTLSSecrets(ctx context.Context, app *appv1alpha1.App) {
	log := logf.FromContext(ctx)
	for i, h := range effectiveHosts(app, r.BaseDomain) {
		name := tlsSecretName(app.Name, i, h)
		sec := &corev1.Secret{}
		sec.Name = name
		sec.Namespace = app.Namespace
		if err := r.Delete(ctx, sec); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "delete TLS secret", "secret", name)
		}
	}
}

// deleteRegistryRepo removes all manifests for the app's image repository from
// the in-cluster OCI registry via the OCI Distribution Spec v2 HTTP API. Best-
// effort: errors are logged and do not block finalizer removal. After all
// manifests are deleted, Zot's periodic GC reclaims the unreferenced blobs.
func (r *AppReconciler) deleteRegistryRepo(ctx context.Context, app *appv1alpha1.App) error {
	registryHost := r.Registry
	base := "http://" + registryHost
	if strings.HasPrefix(registryHost, "http://") || strings.HasPrefix(registryHost, "https://") {
		base = registryHost
	}
	repo := app.Name

	authHdr, err := r.registryAuthHeader(ctx, app.Namespace)
	if err != nil {
		return fmt.Errorf("registry auth: %w", err)
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
		return http.DefaultClient.Do(req)
	}

	resp, err := do(http.MethodGet, fmt.Sprintf("%s/v2/%s/tags/list", base, repo))
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("list tags: status %d: %s", resp.StatusCode, body)
	}
	var tagList struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tagList); err != nil {
		return fmt.Errorf("decode tags: %w", err)
	}

	seen := map[string]bool{}
	for _, tag := range tagList.Tags {
		r2, err := do(http.MethodHead, fmt.Sprintf("%s/v2/%s/manifests/%s", base, repo, tag))
		if err != nil {
			return fmt.Errorf("head manifest %s: %w", tag, err)
		}
		_ = r2.Body.Close()
		if r2.StatusCode == http.StatusNotFound {
			continue
		}
		digest := r2.Header.Get("Docker-Content-Digest")
		if digest == "" || seen[digest] {
			continue
		}
		seen[digest] = true
		r3, err := do(http.MethodDelete, fmt.Sprintf("%s/v2/%s/manifests/%s", base, repo, digest))
		if err != nil {
			return fmt.Errorf("delete manifest %s: %w", digest, err)
		}
		body, _ := io.ReadAll(r3.Body)
		_ = r3.Body.Close()
		if r3.StatusCode != http.StatusAccepted && r3.StatusCode != http.StatusOK {
			return fmt.Errorf("delete manifest %s: status %d: %s", digest, r3.StatusCode, body)
		}
	}
	return nil
}

// registryAuthHeader reads the docker config push Secret and returns an HTTP
// Basic Authorization header for the registry, or "" when no auth is configured.
func (r *AppReconciler) registryAuthHeader(ctx context.Context, fallbackNS string) (string, error) {
	if r.RegistryPushSecret == "" {
		return "", nil
	}
	ns := r.BuildNamespace
	if ns == "" {
		ns = fallbackNS
	}
	cl := r.Client
	if r.BuildClient != nil {
		cl = r.BuildClient
	}
	var sec corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: r.RegistryPushSecret}, &sec); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	data, ok := sec.Data["config.json"]
	if !ok {
		return "", nil
	}
	var cfg struct {
		Auths map[string]struct{ Auth string } `json:"auths"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", nil // malformed docker config; skip auth
	}
	if e, ok := cfg.Auths[r.Registry]; ok && e.Auth != "" {
		return "Basic " + e.Auth, nil
	}
	return "", nil
}
