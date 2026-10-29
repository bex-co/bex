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
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's free-plan recipe runs migrations at the end of the build command
// against the internal DATABASE_URL a fromDatabase reference supplies
// (w8/m129, docs/ADR022-tenant-isolation.md § Build reach to the owning
// workspace's datastores). Two things make it work on bex:
//
//   - the build receives every spec.env SecretKeyRef (fromDatabase, Key Value
//     fromService) through the App-owned merged env Secret it already mounts,
//     never as a literal in the Job spec (readBuildEnvRefs);
//   - a pair of operator-owned NetworkPolicies opens exactly one path: this
//     App's build pods → datastore ports in the App's own workspace namespace
//     (reconcileBuildDatastoreReach).

// errBuildEnvRefPending marks a referenced connection Secret that does not
// exist yet — a Blueprint's database is still being created. The build waits
// for it rather than running a migration with no DATABASE_URL.
var errBuildEnvRefPending = errors.New("linked datastore connection Secret not found yet")

// buildEnvRef is one spec.env variable sourced from a Secret key.
type buildEnvRef struct{ env, secret, key string }

// buildEnvRefs lists the App's Secret-sourced variables in spec order, so a
// later duplicate wins as it does in a Kubernetes container's env.
func buildEnvRefs(app *appv1alpha1.App) []buildEnvRef {
	var out []buildEnvRef
	for _, e := range app.Spec.Env {
		if e.Name == "" || e.Name == portEnvName || e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			continue
		}
		ref := e.ValueFrom.SecretKeyRef
		out = append(out, buildEnvRef{env: e.Name, secret: ref.Name, key: ref.Key})
	}
	return out
}

// readBuildEnvRefs resolves refs from the App's own namespace into env name →
// value. Values never leave this map except into the merged build Secret, and
// errors name the variable and Secret, never a value. A missing Secret is
// errBuildEnvRefPending; a missing key or a protected operator Secret fails the
// build, as the runtime container would fail to start.
func (r *AppReconciler) readBuildEnvRefs(ctx context.Context, app *appv1alpha1.App, refs []buildEnvRef) (map[string][]byte, error) {
	out := make(map[string][]byte, len(refs))
	secrets := map[string]*corev1.Secret{}
	for _, ref := range refs {
		src, ok := secrets[ref.secret]
		if !ok {
			src = &corev1.Secret{}
			if err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: ref.secret}, src); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("%s: %w", ref.env, errBuildEnvRefPending)
				}
				return nil, fmt.Errorf("reading build env %s from Secret %s/%s: %w", ref.env, app.Namespace, ref.secret, err)
			}
			if src.Labels[execution.LabelProtectedFromTenantMount] == execution.ProtectedFromTenantMount {
				return nil, fmt.Errorf("refusing to project protected operator Secret %s/%s into a build", app.Namespace, ref.secret)
			}
			secrets[ref.secret] = src
		}
		value, ok := src.Data[ref.key]
		if !ok {
			return nil, fmt.Errorf("build env %s: Secret %s/%s has no key %q", ref.env, app.Namespace, ref.secret, ref.key)
		}
		out[ref.env] = value
	}
	return out, nil
}

// buildDatabasePending names a linked Postgres that is still being created, so
// the build waits for it instead of running `migrate` against a server that is
// not accepting connections yet. A fromDatabase Secret is "<db>-app" or
// "<db>-pooler-app"; a Secret that names no Database (a Key Value, a hand-
// written reference) never holds the build.
func (r *AppReconciler) buildDatabasePending(ctx context.Context, app *appv1alpha1.App, refs []buildEnvRef) (string, error) {
	for _, ref := range refs {
		name, ok := strings.CutSuffix(ref.secret, "-pooler-app")
		if !ok {
			if name, ok = strings.CutSuffix(ref.secret, "-app"); !ok {
				continue
			}
		}
		var db appv1alpha1.Database
		if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, &db); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return "", err
		}
		switch db.Status.Phase {
		case "", appv1alpha1.DBPhasePending, appv1alpha1.DBPhaseProvisioning:
			return name, nil
		}
	}
	return "", nil
}

// buildReachPorts are the datastore ports a build may open: Postgres (and its
// PgBouncer pooler) and Key Value's plain and TLS listeners.
func buildReachPorts() []networkingv1.NetworkPolicyPort {
	tcp := corev1.ProtocolTCP
	ports := make([]networkingv1.NetworkPolicyPort, 0, 3)
	for _, port := range []int{pgPort, kvPort, kvTLSPort} {
		ports = append(ports, networkingv1.NetworkPolicyPort{Protocol: &tcp, Port: new(intstr.FromInt(port))})
	}
	return ports
}

// buildReachTargets selects the workspace pods a build may reach, honoring a
// protected environment exactly as allow-same-namespace does: an ordinary
// App's build reaches only pods outside every isolated environment, and an
// isolated App's build only pods of its own environment.
func buildReachTargets(app *appv1alpha1.App) metav1.LabelSelector {
	if env := app.Labels[labelNetworkIsolation]; env != "" {
		return metav1.LabelSelector{MatchLabels: map[string]string{labelNetworkIsolation: env}}
	}
	return metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
		Key: labelNetworkIsolation, Operator: metav1.LabelSelectorOpDoesNotExist,
	}}}
}

// buildEgressPolicyName is the bex-build half: this App's build pods may leave
// toward its workspace namespace. buildIngressPolicyName is the workspace
// half: the workspace's datastores admit exactly those pods.
func buildEgressPolicyName(app string) string  { return build.JobName(app, "datastore-egress") }
func buildIngressPolicyName(app string) string { return app + "-build-ingress" }

// buildReachEligible reports whether app may receive build→datastore reach. The
// build must run outside the App namespace (a same-namespace build already has
// allow-same-namespace), and the App namespace must be the workspace's own
// canonical namespace — the one the operator admission policy also requires
// (namespace name == workspace label). The legacy shared namespace holds many
// workspaces, so it never qualifies: there "the App's namespace" is not "the
// App's workspace".
func buildReachEligible(app *appv1alpha1.App, buildNs string) bool {
	ws := app.Labels[labelWorkspace]
	return buildNs != "" && buildNs != app.Namespace && ws != "" && app.Namespace == ws && app.UID != ""
}

// reconcileBuildDatastoreReach converges the two policies that let this App's
// build pods reach datastore ports in the App's own workspace namespace, and
// nothing else:
//
//   - in buildNs, an egress policy selecting only this App lifetime's build
//     pods (name, UID, component and workspace — all operator-set) toward the
//     App namespace, matched by the API-server-owned kubernetes.io/metadata.name
//     label;
//   - in the App namespace, an ingress policy admitting only those same pods
//     from buildNs, owned by the App so it goes with it.
//
// Both sides must allow a connection, so a build can reach another workspace
// only if both policies named it — and neither can: the egress peer is a
// single namespace equal to the workspace, and the ingress policy lives in
// that namespace. allow=false (or an ineligible App) removes both.
func (r *AppReconciler) reconcileBuildDatastoreReach(ctx context.Context, app *appv1alpha1.App, buildNs string, allow bool) error {
	if !allow || !buildReachEligible(app, buildNs) {
		return r.revokeBuildDatastoreReach(ctx, app, buildNs)
	}
	sources := build.AppBuildPodLabels(app.Name, string(app.UID), app.Labels[labelWorkspace])
	targets := buildReachTargets(app)
	ports := buildReachPorts()

	egress := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: buildEgressPolicyName(app.Name), Namespace: buildNs}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.buildPlaneClient(), egress, func() error {
		if err := checkOwnedArtifact(egress, app); err != nil {
			return err
		}
		egress.Labels = artifactLabels(app, "execution-network-policy")
		egress.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: sources},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: app.Namespace}},
					PodSelector:       &targets,
				}},
				Ports: ports,
			}},
		}
		return nil // cross-namespace owner references are invalid; the finalizer reclaims it
	}); err != nil {
		return fmt.Errorf("build datastore egress policy: %w", err)
	}

	ingress := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: buildIngressPolicyName(app.Name), Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ingress, func() error {
		ingress.Labels = artifactLabels(app, "build-network-policy")
		ingress.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: targets,
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: buildNs}},
					PodSelector:       &metav1.LabelSelector{MatchLabels: sources},
				}},
				Ports: ports,
			}},
		}
		// Refuses an object another owner controls rather than adopting it.
		return controllerutil.SetControllerReference(app, ingress, r.Scheme)
	}); err != nil {
		return fmt.Errorf("build datastore ingress policy: %w", err)
	}
	return nil
}

// endBuildDatastoreReach revokes the grant once the observed build is over
// (succeeded or failed). Best-effort: the policies select only this App
// lifetime's build pods, so a leftover grants nothing while no build runs, and
// the next terminal pass or the finalizer retries.
func (r *AppReconciler) endBuildDatastoreReach(ctx context.Context, app *appv1alpha1.App, buildNs string, phase build.Phase) {
	if phase != build.PhaseSucceeded && phase != build.PhaseFailed {
		return
	}
	if err := r.reconcileBuildDatastoreReach(ctx, app, buildNs, false); err != nil {
		logf.FromContext(ctx).Error(err, "revoking build datastore reach", "app", app.Name)
	}
}

// revokeBuildDatastoreReach removes both halves. Each delete is ownership-gated:
// the build namespace is shared, and a same-named App in another workspace
// must keep its own policy.
func (r *AppReconciler) revokeBuildDatastoreReach(ctx context.Context, app *appv1alpha1.App, buildNs string) error {
	owned := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
		Workspace: app.Labels[labelWorkspace], Namespace: app.Namespace}
	var errs []error
	if buildNs != "" && buildNs != app.Namespace {
		if _, err := deleteOwnedObject(ctx, r.buildPlaneClient(), buildNs, buildEgressPolicyName(app.Name), &networkingv1.NetworkPolicy{}, owned); err != nil {
			errs = append(errs, fmt.Errorf("removing build datastore egress policy: %w", err))
		}
	}
	if _, err := deleteOwnedObject(ctx, r.Client, app.Namespace, buildIngressPolicyName(app.Name), &networkingv1.NetworkPolicy{}, owned); err != nil {
		errs = append(errs, fmt.Errorf("removing build datastore ingress policy: %w", err))
	}
	return errors.Join(errs...)
}
