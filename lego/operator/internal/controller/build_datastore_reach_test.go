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
	"encoding/json"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m129: Render's free-plan recipe runs `manage.py migrate` in the build
// against the fromDatabase DATABASE_URL. These tests pin the two halves bex
// needs and the boundary the user decision kept: the value reaches the build
// only through the mounted env Secret, and the build reaches its own
// workspace's datastores and nothing in another workspace.

const (
	reachBuildNs = "bex-build"
	wsA          = "tea-aaaaaaaaaaaaaaaaaaaa"
	wsB          = "tea-bbbbbbbbbbbbbbbbbbbb"
	dbURI        = "postgresql://app:s3cr3t-pw@dpg-one-rw.tea-aaaaaaaaaaaaaaaaaaaa.svc:5432/app"
)

func reachScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

// reachApp is a source-built service in ws's own namespace whose DATABASE_URL
// comes from a fromDatabase reference, as a Blueprint writes it.
func reachApp(name, ws string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ws, UID: types.UID("uid-" + name + "-" + ws),
			Labels: map[string]string{labelWorkspace: ws},
		},
		Spec: appv1alpha1.AppSpec{
			Repo: "https://github.com/render-examples/django", Runtime: "python",
			BuildCommand: "./build.sh", StartCommand: "gunicorn mysite.wsgi",
			Env: []appv1alpha1.EnvVar{
				{Name: "DATABASE_URL", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "dpg-one-app", Key: "uri"}}},
				{Name: "PYTHON_VERSION", Value: "3.12.11"},
			},
		},
	}
}

func reachReconciler(t *testing.T, objs ...client.Object) (*AppReconciler, client.Client) {
	t.Helper()
	cl := fake.NewClientBuilder().WithScheme(reachScheme(t)).WithObjects(objs...).Build()
	return &AppReconciler{Client: cl, BuildClient: cl, Scheme: cl.Scheme(), BuildNamespace: reachBuildNs}, cl
}

// DATABASE_URL arrives through the merged env Secret the build mounts; neither
// the native nor the Dockerfile build Job carries the value anywhere in its
// spec (env, args, scripts, generated Dockerfile).
func TestBuildEnvCarriesFromDatabaseThroughTheMountOnly(t *testing.T) {
	app := reachApp("mysite", wsA)
	r, cl := reachReconciler(t, envSecret(wsA, "dpg-one-app", map[string]string{"uri": dbURI, "password": "s3cr3t-pw"}))
	ctx := context.Background()

	for _, builder := range []string{build.BuilderNative, build.BuilderDockerfile} {
		t.Run(builder, func(t *testing.T) {
			literals := buildEnv(builder, app.Spec.Env)
			for _, e := range literals {
				if e.Name == "DATABASE_URL" {
					t.Fatalf("DATABASE_URL must not be a build literal: %+v", e)
				}
			}
			projectLiterals := literals
			if builder != build.BuilderNative {
				projectLiterals = nil
			}
			inputs, err := r.projectBuildInputs(ctx, app, reachBuildNs, projectLiterals)
			if err != nil {
				t.Fatal(err)
			}
			merged := readSecret(t, cl, reachBuildNs, inputs.envSecret)
			if got := string(merged.Data["DATABASE_URL"]); got != dbURI {
				t.Fatalf("merged build env DATABASE_URL = %q, want the connection Secret's uri", got)
			}
			if _, leaked := merged.Data["password"]; leaked {
				t.Fatal("only the referenced key may enter the build env, not the whole connection Secret")
			}

			job := build.BuildJob(build.Options{
				Repo: app.Spec.Repo, Ref: "main", Name: app.Name, AppUID: string(app.UID),
				Registry: "zot:5000", Revision: "gen-1", Namespace: reachBuildNs, AppNamespace: app.Namespace,
				Workspace: wsA, Builder: builder, Runtime: app.Spec.Runtime,
				BuildCommand: app.Spec.BuildCommand, StartCommand: app.Spec.StartCommand,
				BuildEnv: literals, RuntimeEnvSecret: inputs.envSecret, NativeEnvRevision: inputs.revision,
			}, "zot:5000/mysite:gen-1")
			raw, err := json.Marshal(job)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{dbURI, "s3cr3t-pw"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("the build Job spec carries a connection-string literal (%q)", secret)
				}
			}
			mounted := false
			for _, v := range job.Spec.Template.Spec.Volumes {
				if v.Secret != nil && v.Secret.SecretName == inputs.envSecret {
					mounted = true
				}
			}
			if !mounted {
				t.Fatalf("the build Job does not mount the merged env Secret %s", inputs.envSecret)
			}
		})
	}
}

// A Blueprint creates the database and the service together: until CNPG has
// written the connection Secret, the build waits rather than running a migrate
// with no DATABASE_URL. A protected Secret or a missing key fails the build.
func TestReadBuildEnvRefsContract(t *testing.T) {
	ctx := context.Background()
	app := reachApp("mysite", wsA)

	r, _ := reachReconciler(t)
	if _, err := r.readBuildEnvRefs(ctx, app, buildEnvRefs(app)); !errors.Is(err, errBuildEnvRefPending) {
		t.Fatalf("missing connection Secret: err = %v, want errBuildEnvRefPending", err)
	}

	protected := envSecret(wsA, "dpg-one-app", map[string]string{"uri": dbURI})
	protected.Labels = map[string]string{execution.LabelProtectedFromTenantMount: execution.ProtectedFromTenantMount}
	r, _ = reachReconciler(t, protected)
	if _, err := r.readBuildEnvRefs(ctx, app, buildEnvRefs(app)); err == nil || errors.Is(err, errBuildEnvRefPending) {
		t.Fatalf("protected Secret: err = %v, want a hard refusal", err)
	}

	r, _ = reachReconciler(t, envSecret(wsA, "dpg-one-app", map[string]string{"host": "x"}))
	_, err := r.readBuildEnvRefs(ctx, app, buildEnvRefs(app))
	if err == nil || errors.Is(err, errBuildEnvRefPending) {
		t.Fatalf("missing key: err = %v, want a hard failure", err)
	}

	// A later duplicate wins, like a container's env list; PORT stays the
	// operator's.
	app.Spec.Env = append(app.Spec.Env,
		appv1alpha1.EnvVar{Name: "DATABASE_URL", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "dpg-one-app", Key: "host"}}},
		appv1alpha1.EnvVar{Name: "PORT", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "dpg-one-app", Key: "host"}}},
	)
	r, _ = reachReconciler(t, envSecret(wsA, "dpg-one-app", map[string]string{"uri": dbURI, "host": "dpg-one-rw"}))
	got, err := r.readBuildEnvRefs(ctx, app, buildEnvRefs(app))
	if err != nil {
		t.Fatal(err)
	}
	if string(got["DATABASE_URL"]) != "dpg-one-rw" || got["PORT"] != nil {
		t.Fatalf("refs = %q", got)
	}
}

func TestBuildDatabasePendingHoldsOnlyWhileProvisioning(t *testing.T) {
	ctx := context.Background()
	app := reachApp("mysite", wsA)
	app.Spec.Env = append(app.Spec.Env, appv1alpha1.EnvVar{Name: "REDIS_URL", ValueFrom: &appv1alpha1.EnvVarSource{
		SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "red-one", Key: "uri"}}})
	for phase, wantHold := range map[appv1alpha1.DatabasePhase]bool{
		"": true, appv1alpha1.DBPhasePending: true, appv1alpha1.DBPhaseProvisioning: true,
		appv1alpha1.DBPhaseReady: false, appv1alpha1.DBPhaseUpgrading: false, appv1alpha1.DBPhaseFailed: false,
	} {
		db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-one", Namespace: wsA}}
		db.Status.Phase = phase
		r, _ := reachReconciler(t, db)
		got, err := r.buildDatabasePending(ctx, app, buildEnvRefs(app))
		if err != nil {
			t.Fatal(err)
		}
		if (got == "dpg-one") != wantHold {
			t.Errorf("phase %q: pending = %q, want hold=%v", phase, got, wantHold)
		}
	}
	// The pooled Secret names the same Database; no Database at all never holds.
	app.Spec.Env[0].ValueFrom.SecretKeyRef.Name = "dpg-one-pooler-app"
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-one", Namespace: wsA}}
	r, _ := reachReconciler(t, db)
	if got, _ := r.buildDatabasePending(ctx, app, buildEnvRefs(app)); got != "dpg-one" {
		t.Errorf("pooler Secret: pending = %q, want dpg-one", got)
	}
	r, _ = reachReconciler(t)
	if got, _ := r.buildDatabasePending(ctx, app, buildEnvRefs(app)); got != "" {
		t.Errorf("no Database: pending = %q, want none", got)
	}
}

// policyAllows evaluates Kubernetes NetworkPolicy semantics over policies for
// one TCP connection, with every namespace default-deny (bex-build and every
// tenant namespace are): it is allowed only if an egress policy selecting the
// source admits the destination AND an ingress policy selecting the
// destination admits the source.
type endpoint struct {
	ns     string
	labels map[string]string
}

func peerMatches(t *testing.T, peer networkingv1.NetworkPolicyPeer, policyNs string, other endpoint) bool {
	t.Helper()
	if peer.IPBlock != nil {
		t.Fatalf("build reach must never use ipBlock peers: %+v", peer)
	}
	if peer.NamespaceSelector == nil {
		if other.ns != policyNs {
			return false
		}
	} else if !selectorMatches(t, *peer.NamespaceSelector, map[string]string{corev1.LabelMetadataName: other.ns}) {
		return false
	}
	return peer.PodSelector == nil || selectorMatches(t, *peer.PodSelector, other.labels)
}

func selectorMatches(t *testing.T, sel metav1.LabelSelector, set map[string]string) bool {
	t.Helper()
	s, err := metav1.LabelSelectorAsSelector(&sel)
	if err != nil {
		t.Fatal(err)
	}
	return s.Matches(labels.Set(set))
}

func portMatches(ports []networkingv1.NetworkPolicyPort, port int) bool {
	if len(ports) == 0 {
		return true
	}
	for _, p := range ports {
		if p.Port == nil || p.Port.IntValue() == port {
			return true
		}
	}
	return false
}

func policyAllows(t *testing.T, policies []networkingv1.NetworkPolicy, src, dst endpoint, port int) bool {
	t.Helper()
	egress, ingress := false, false
	for _, np := range policies {
		for _, pt := range np.Spec.PolicyTypes {
			switch {
			case pt == networkingv1.PolicyTypeEgress && np.Namespace == src.ns && selectorMatches(t, np.Spec.PodSelector, src.labels):
				for _, rule := range np.Spec.Egress {
					for _, peer := range rule.To {
						if peerMatches(t, peer, np.Namespace, dst) && portMatches(rule.Ports, port) {
							egress = true
						}
					}
				}
			case pt == networkingv1.PolicyTypeIngress && np.Namespace == dst.ns && selectorMatches(t, np.Spec.PodSelector, dst.labels):
				for _, rule := range np.Spec.Ingress {
					for _, peer := range rule.From {
						if peerMatches(t, peer, np.Namespace, src) && portMatches(rule.Ports, port) {
							ingress = true
						}
					}
				}
			}
		}
	}
	return egress && ingress
}

func listPolicies(t *testing.T, cl client.Client) []networkingv1.NetworkPolicy {
	t.Helper()
	var list networkingv1.NetworkPolicyList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

// buildPod is the labelled pod a BuildKit Job creates for app — taken from the
// real Job template, so the policy and the pod cannot drift apart.
func buildPod(app *appv1alpha1.App) endpoint {
	job := build.BuildJob(build.Options{
		Repo: app.Spec.Repo, Ref: "main", Name: app.Name, AppUID: string(app.UID), Registry: "zot:5000",
		Revision: "gen-1", Namespace: reachBuildNs, AppNamespace: app.Namespace, Workspace: app.Labels[labelWorkspace],
		Builder: build.BuilderDockerfile,
	}, "zot:5000/x:gen-1")
	return endpoint{ns: reachBuildNs, labels: job.Spec.Template.Labels}
}

// The user-approved boundary (m129 unblock record): a build reaches only its
// own workspace's datastores. Two workspaces each run a build with the grant in
// place; every cross-workspace pairing must be denied by construction.
func TestBuildDatastoreReachIsOwnWorkspaceOnly(t *testing.T) {
	ctx := context.Background()
	// Service ids are globally unique, so two workspaces' Apps never share a
	// name in the build namespace (a collision fails closed — see
	// TestBuildDatastoreReachRevocationAndEligibility).
	appA, appB := reachApp("srv-a", wsA), reachApp("srv-b", wsB)
	isolated := reachApp("guarded", wsA)
	isolated.Labels[labelNetworkIsolation] = "evm-prod"
	r, cl := reachReconciler(t, appA, appB, isolated)
	for _, app := range []*appv1alpha1.App{appA, appB, isolated} {
		if err := r.reconcileBuildDatastoreReach(ctx, app, reachBuildNs, true); err != nil {
			t.Fatal(err)
		}
	}
	policies := listPolicies(t, cl)

	pgA := endpoint{ns: wsA, labels: map[string]string{"app.bex.co/component": "database", labelWorkspace: wsA, "cnpg.io/cluster": "dpg-one"}}
	poolerA := endpoint{ns: wsA, labels: map[string]string{"cnpg.io/poolerName": "dpg-one-pooler"}}
	kvA := endpoint{ns: wsA, labels: map[string]string{labelKeyValueInternalProxy: "red-one", labelWorkspace: wsA}}
	pgB := endpoint{ns: wsB, labels: map[string]string{"app.bex.co/component": "database", labelWorkspace: wsB}}
	prodPgA := endpoint{ns: wsA, labels: map[string]string{"app.bex.co/component": "database", labelWorkspace: wsA, labelNetworkIsolation: "evm-prod"}}
	platform := endpoint{ns: "bex-system", labels: map[string]string{"control-plane": "controller-manager"}}
	strangerBuild := endpoint{ns: reachBuildNs, labels: map[string]string{
		appv1alpha1.LabelBuild: "mysite", execution.LabelAppUID: "uid-forged", execution.LabelComponent: "build", labelWorkspace: wsA}}
	publish := endpoint{ns: reachBuildNs, labels: map[string]string{
		labelApp: appA.Name, execution.LabelAppUID: string(appA.UID), execution.LabelComponent: "publish", labelWorkspace: wsA}}

	for _, tc := range []struct {
		name      string
		src, dst  endpoint
		port      int
		wantAllow bool
	}{
		{"A build → A Postgres", buildPod(appA), pgA, pgPort, true},
		{"A build → A PgBouncer", buildPod(appA), poolerA, pgPort, true},
		{"A build → A Key Value", buildPod(appA), kvA, kvPort, true},
		{"A build → A Key Value TLS", buildPod(appA), kvA, kvTLSPort, true},
		{"B build → B Postgres", buildPod(appB), pgB, pgPort, true},
		{"A build → B Postgres (cross-workspace)", buildPod(appA), pgB, pgPort, false},
		{"B build → A Postgres (cross-workspace)", buildPod(appB), pgA, pgPort, false},
		{"A build → A Postgres on a non-datastore port", buildPod(appA), pgA, 8000, false},
		{"A build → A isolated environment's Postgres", buildPod(appA), prodPgA, pgPort, false},
		{"isolated build → its environment's Postgres", buildPod(isolated), prodPgA, pgPort, true},
		{"isolated build → the workspace's ordinary Postgres", buildPod(isolated), pgA, pgPort, false},
		{"A build → platform namespace", buildPod(appA), platform, pgPort, false},
		{"a build pod with another App lifetime's UID → A Postgres", strangerBuild, pgA, pgPort, false},
		{"A's publish pod → A Postgres", publish, pgA, pgPort, false},
	} {
		if got := policyAllows(t, policies, tc.src, tc.dst, tc.port); got != tc.wantAllow {
			t.Errorf("%s: allowed=%v, want %v", tc.name, got, tc.wantAllow)
		}
	}

	// Structural pins: the egress peer names exactly one namespace — the App's
	// own — by the API-server-owned metadata.name label, and nothing lands in
	// the other workspace's namespace on A's behalf.
	for _, np := range policies {
		if np.Namespace == reachBuildNs {
			for _, rule := range np.Spec.Egress {
				for _, peer := range rule.To {
					ns := peer.NamespaceSelector.MatchLabels[corev1.LabelMetadataName]
					if len(peer.NamespaceSelector.MatchLabels) != 1 || len(peer.NamespaceSelector.MatchExpressions) != 0 || ns != np.Labels[execution.LabelAppNamespace] {
						t.Errorf("%s: egress peer %+v must name exactly the App's own namespace", np.Name, peer.NamespaceSelector)
					}
				}
			}
			continue
		}
		if np.Labels[labelWorkspace] != np.Namespace {
			t.Errorf("%s/%s: an ingress grant must live in its own App's workspace namespace (labels %v)", np.Namespace, np.Name, np.Labels)
		}
	}
}

func TestBuildDatastoreReachRevocationAndEligibility(t *testing.T) {
	ctx := context.Background()
	app := reachApp("mysite", wsA)
	// A same-named App of another lifetime owns a same-named build policy in
	// the shared build namespace: revocation must leave it alone.
	foreign := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{
		Name: buildEgressPolicyName("mysite"), Namespace: reachBuildNs,
		Labels: map[string]string{labelApp: "mysite", execution.LabelAppUID: "uid-other"},
	}}
	r, cl := reachReconciler(t, app, foreign)
	if err := r.reconcileBuildDatastoreReach(ctx, app, reachBuildNs, true); err == nil {
		t.Fatal("the grant must refuse to take over another App lifetime's policy")
	}
	if err := cl.Delete(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	if err := r.reconcileBuildDatastoreReach(ctx, app, reachBuildNs, true); err != nil {
		t.Fatal(err)
	}
	if n := len(listPolicies(t, cl)); n != 2 {
		t.Fatalf("grant created %d policies, want the egress+ingress pair", n)
	}
	ingress := &networkingv1.NetworkPolicy{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: wsA, Name: buildIngressPolicyName(app.Name)}, ingress); err != nil {
		t.Fatal(err)
	}
	if ref := metav1.GetControllerOf(ingress); ref == nil || ref.UID != app.UID {
		t.Fatalf("the workspace-side policy must be owned by the App so it goes with it: %+v", ref)
	}

	if err := r.reconcileBuildDatastoreReach(ctx, app, reachBuildNs, false); err != nil {
		t.Fatal(err)
	}
	if n := len(listPolicies(t, cl)); n != 0 {
		t.Fatalf("revoke left %d policies", n)
	}
	// Idempotent once gone.
	if err := r.reconcileBuildDatastoreReach(ctx, app, reachBuildNs, false); err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*appv1alpha1.App){
		"legacy shared namespace": func(a *appv1alpha1.App) { a.Namespace = "default" },
		"no workspace label":      func(a *appv1alpha1.App) { delete(a.Labels, labelWorkspace) },
	} {
		a := reachApp("mysite", wsA)
		mutate(a)
		r, cl := reachReconciler(t)
		if err := r.reconcileBuildDatastoreReach(ctx, a, reachBuildNs, true); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n := len(listPolicies(t, cl)); n != 0 {
			t.Errorf("%s: an ineligible App got %d policies", name, n)
		}
	}
	// Same-namespace builds already have allow-same-namespace: nothing to add.
	r, cl = reachReconciler(t, app)
	if err := r.reconcileBuildDatastoreReach(ctx, app, wsA, true); err != nil {
		t.Fatal(err)
	}
	if n := len(listPolicies(t, cl)); n != 0 {
		t.Errorf("same-namespace build got %d policies", n)
	}
	var np networkingv1.NetworkPolicy
	if err := cl.Get(ctx, client.ObjectKey{Namespace: reachBuildNs, Name: buildEgressPolicyName(app.Name)}, &np); !apierrors.IsNotFound(err) {
		t.Fatalf("err = %v", err)
	}
}
