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
	"slices"

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
//   - operator-owned NetworkPolicies open exactly one path: this App's build
//     pods → its own workspace namespace, on datastore ports and, since
//     w9/m495, to the workspace's own paid web and private services on any
//     TCP port, so fromService host/hostport works in the build as at runtime
//     (reconcileBuildReach).

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
// not accepting connections yet. A fromDatabase Secret is "<db>-app",
// "<db>-default-app" (w5/m265) or "<db>-pooler-app"; a Secret that names no
// Database (a Key Value, a hand-written reference) never holds the build.
func (r *AppReconciler) buildDatabasePending(ctx context.Context, app *appv1alpha1.App, refs []buildEnvRef) (string, error) {
	for _, ref := range refs {
		name, ok := appv1alpha1.DatabaseIDFromSecretName(ref.secret)
		if !ok {
			continue
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

// buildServicePorts is every TCP port: a service listens where it chose (PORT,
// a configured port, or every port its image declares — ADR089), and Render's
// private network carries "almost any port" (w9/m495).
func buildServicePorts() []networkingv1.NetworkPolicyPort {
	tcp := corev1.ProtocolTCP
	return []networkingv1.NetworkPolicyPort{{Protocol: &tcp}}
}

// buildReachServices names the Apps in app's namespace its build may dial on
// any TCP port: those that receive private-network traffic
// (ReceivesPrivateTraffic: a paid web or private service). Workers, cron jobs,
// static sites and free web services receive none, as on Render. A static
// site's own build gets none either: Render's static sites are not on the
// private network.
func (r *AppReconciler) buildReachServices(ctx context.Context, app *appv1alpha1.App) ([]string, error) {
	if app.Spec.Type == appv1alpha1.TypeStaticSite {
		return nil, nil
	}
	var apps appv1alpha1.AppList
	if err := r.List(ctx, &apps, client.InNamespace(app.Namespace)); err != nil {
		return nil, fmt.Errorf("listing the workspace's services: %w", err)
	}
	var names []string
	for _, a := range apps.Items {
		if a.DeletionTimestamp.IsZero() && a.Spec.ReceivesPrivateTraffic() {
			names = append(names, a.Name)
		}
	}
	slices.Sort(names) // a stable spec, so an unchanged set writes nothing
	return names, nil
}

// buildServiceTargets narrows buildReachTargets to the serving pods of
// services: labelApp names the App, and labelAppID marks a runtime pod, as the
// stable Services select them (privateServiceProjection) — Job pods
// (pre-deploy, build, disk backup) carry labelApp alone.
func buildServiceTargets(app *appv1alpha1.App, services []string) metav1.LabelSelector {
	targets := buildReachTargets(app)
	targets.MatchExpressions = append(targets.MatchExpressions,
		metav1.LabelSelectorRequirement{Key: labelApp, Operator: metav1.LabelSelectorOpIn, Values: services},
		metav1.LabelSelectorRequirement{Key: labelAppID, Operator: metav1.LabelSelectorOpExists})
	return targets
}

// buildReachTargets selects the workspace pods a build may reach, honoring a
// protected environment exactly as allow-same-namespace does: an ordinary
// App's build reaches only pods outside every isolated environment, and an
// isolated App's build only pods of its own environment, by the environment-id
// every datastore pod of it carries (w4/m342).
func buildReachTargets(app *appv1alpha1.App) metav1.LabelSelector {
	if env := app.Labels[labelNetworkIsolation]; env != "" {
		return metav1.LabelSelector{MatchLabels: map[string]string{labelEnvironment: env}}
	}
	return metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
		Key: labelNetworkIsolation, Operator: metav1.LabelSelectorOpDoesNotExist,
	}}}
}

// buildEgressPolicyName is the bex-build half: this App's build pods may leave
// toward its workspace namespace (the name predates service reach and stays,
// so a running build's grant is revoked across an upgrade).
// buildIngressPolicyName is the workspace half for datastore ports, and
// buildServiceIngressPolicyName for the workspace's services: each admits
// exactly those pods.
func buildEgressPolicyName(app string) string         { return build.JobName(app, "datastore-egress") }
func buildIngressPolicyName(app string) string        { return app + "-build-ingress" }
func buildServiceIngressPolicyName(app string) string { return app + "-build-service-ingress" }

// buildReachEligible reports whether app may receive build reach. The
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

// buildReachGrant is one workspace-side half: the pods it opens and on which
// ports, admitted by the ingress policy named ingress.
type buildReachGrant struct {
	ingress string
	targets metav1.LabelSelector
	ports   []networkingv1.NetworkPolicyPort
}

// reconcileBuildReach converges the policies that let this App's build pods
// reach its own workspace namespace, and nothing else:
//
//   - datastore ports (5432/6379/6380) on any pod in the build's isolation
//     scope (w8/m129);
//   - every TCP port on the serving pods of the workspace's paid web and
//     private services in that scope (w9/m495).
//
// In buildNs, one egress policy selects only this App lifetime's build pods
// (name, UID, component and workspace — all operator-set) with one rule per
// grant, toward the App namespace matched by the API-server-owned
// kubernetes.io/metadata.name label. In the App namespace, one ingress policy
// per grant, owned by the App so it goes with it, admits only those same pods
// from buildNs.
//
// Both sides must allow a connection, so a build can reach another workspace
// only if both policies named it — and neither can: every egress peer is a
// single namespace equal to the workspace, and the ingress policies live in
// that namespace. allow=false (or an ineligible App) removes them all.
func (r *AppReconciler) reconcileBuildReach(ctx context.Context, app *appv1alpha1.App, buildNs string, allow bool) error {
	if !allow || !buildReachEligible(app, buildNs) {
		return r.revokeBuildReach(ctx, app, buildNs)
	}
	services, err := r.buildReachServices(ctx, app)
	if err != nil {
		return err
	}
	sources := build.AppBuildPodLabels(app.Name, string(app.UID), app.Labels[labelWorkspace])
	grants := []buildReachGrant{{ingress: buildIngressPolicyName(app.Name), targets: buildReachTargets(app), ports: buildReachPorts()}}
	if len(services) > 0 {
		grants = append(grants, buildReachGrant{ingress: buildServiceIngressPolicyName(app.Name),
			targets: buildServiceTargets(app, services), ports: buildServicePorts()})
	}

	egress := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: buildEgressPolicyName(app.Name), Namespace: buildNs}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.buildPlaneClient(), egress, func() error {
		if err := checkOwnedArtifact(egress, app); err != nil {
			return err
		}
		egress.Labels = artifactLabels(app, "execution-network-policy")
		rules := make([]networkingv1.NetworkPolicyEgressRule, 0, len(grants))
		for _, g := range grants {
			rules = append(rules, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: app.Namespace}},
					PodSelector:       &g.targets,
				}},
				Ports: g.ports,
			})
		}
		egress.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: sources},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress:      rules,
		}
		return nil // cross-namespace owner references are invalid; the finalizer reclaims it
	}); err != nil {
		return fmt.Errorf("build reach egress policy: %w", err)
	}

	for _, g := range grants {
		ingress := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: g.ingress, Namespace: app.Namespace}}
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ingress, func() error {
			ingress.Labels = artifactLabels(app, "build-network-policy")
			ingress.Spec = networkingv1.NetworkPolicySpec{
				PodSelector: g.targets,
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From: []networkingv1.NetworkPolicyPeer{{
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: buildNs}},
						PodSelector:       &metav1.LabelSelector{MatchLabels: sources},
					}},
					Ports: g.ports,
				}},
			}
			// Refuses an object another owner controls rather than adopting it.
			return controllerutil.SetControllerReference(app, ingress, r.Scheme)
		}); err != nil {
			return fmt.Errorf("build reach ingress policy %s: %w", g.ingress, err)
		}
	}
	if len(services) == 0 {
		return r.deleteBuildIngress(ctx, app, buildServiceIngressPolicyName(app.Name))
	}
	return nil
}

// endBuildReach revokes the grant once the observed build is over (succeeded
// or failed). Best-effort: the policies select only this App lifetime's build
// pods, so a leftover grants nothing while no build runs, and the next
// terminal pass or the finalizer retries.
func (r *AppReconciler) endBuildReach(ctx context.Context, app *appv1alpha1.App, buildNs string, phase build.Phase) {
	if phase != build.PhaseSucceeded && phase != build.PhaseFailed {
		return
	}
	if err := r.reconcileBuildReach(ctx, app, buildNs, false); err != nil {
		logf.FromContext(ctx).Error(err, "revoking build reach", "app", app.Name)
	}
}

// buildReachIdentity is the App lifetime every build-reach policy belongs to.
func buildReachIdentity(app *appv1alpha1.App) execution.ArtifactIdentity {
	return execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
		Workspace: app.Labels[labelWorkspace], Namespace: app.Namespace}
}

// deleteBuildIngress removes one workspace-side half this App lifetime owns.
func (r *AppReconciler) deleteBuildIngress(ctx context.Context, app *appv1alpha1.App, name string) error {
	if _, err := deleteOwnedObject(ctx, r.Client, app.Namespace, name, &networkingv1.NetworkPolicy{}, buildReachIdentity(app)); err != nil {
		return fmt.Errorf("removing build reach ingress policy %s: %w", name, err)
	}
	return nil
}

// revokeBuildReach removes every half. Each delete is ownership-gated: the
// build namespace is shared, and a same-named App in another workspace must
// keep its own policy.
func (r *AppReconciler) revokeBuildReach(ctx context.Context, app *appv1alpha1.App, buildNs string) error {
	var errs []error
	if buildNs != "" && buildNs != app.Namespace {
		if _, err := deleteOwnedObject(ctx, r.buildPlaneClient(), buildNs, buildEgressPolicyName(app.Name), &networkingv1.NetworkPolicy{}, buildReachIdentity(app)); err != nil {
			errs = append(errs, fmt.Errorf("removing build reach egress policy: %w", err))
		}
	}
	for _, name := range []string{buildIngressPolicyName(app.Name), buildServiceIngressPolicyName(app.Name)} {
		errs = append(errs, r.deleteBuildIngress(ctx, app, name))
	}
	return errors.Join(errs...)
}
