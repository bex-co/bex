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

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// activatorUser is the identity the admission policy confines: the kustomized
// name of config/activator's ServiceAccount.
const activatorUser = "system:serviceaccount:bex-system:bex-activator"

// policyCluster starts a real API server with the App CRD, the production
// policy applied and the activator's identity granted cluster-admin, so a denial
// is the policy's. It returns an admin client and one acting as the activator.
func policyCluster(t *testing.T) (ctx context.Context, admin, activator client.Client) {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join("..", "..", "bin", "k8s", "*"))
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" && len(matches) > 0 {
		assets = matches[0]
	}
	if assets == "" {
		t.Skip("no envtest binaries: run through `make test`")
	}
	env := &envtest.Environment{
		BinaryAssetsDirectory: assets,
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
	}
	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("envtest: %v", err)
	}
	t.Cleanup(func() { _ = env.Stop() })

	scheme := runtime.NewScheme()
	adders := []func(*runtime.Scheme) error{
		corev1.AddToScheme, appsv1.AddToScheme, rbacv1.AddToScheme, appv1alpha1.AddToScheme,
	}
	for _, add := range adders {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	ctx = context.Background()
	admin, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	applyPolicy(ctx, t, admin)
	must(t, admin.Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "activator-test-admin"},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: activatorUser}},
	}))
	impersonated := rest.CopyConfig(cfg)
	impersonated.Impersonate = rest.ImpersonationConfig{UserName: activatorUser}
	activator, err = client.New(impersonated, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	return ctx, admin, activator
}

// TestAdmissionPolicyConfinesTheActivator runs the production policy
// (deploy/gitops/base/activator-admission.yaml) in a real API server with the
// App CRD. The identity holds cluster-admin here, so every denial below is the
// policy's, not RBAC's: the activator may set its two wake annotations on an App
// and raise a Deployment from zero to one replica, and nothing else.
func TestAdmissionPolicyConfinesTheActivator(t *testing.T) {
	ctx, admin, guard := policyCluster(t)
	// Fixtures, created as admin: a hibernated App with its scaled-down
	// Deployment, and a running one.
	const ns = "default"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "web", Annotations: map[string]string{
			annotLastActive: "2026-11-01T00:00:00Z", "owner": "someone",
		}},
		Spec: appv1alpha1.AppSpec{Image: "example.invalid/app:1", Port: 8080},
	}
	must(t, admin.Create(ctx, app))
	asleep, running, scaled := deployment("web", 0), deployment("busy", 1), deployment("scaled", 3)
	must(t, admin.Create(ctx, asleep))
	must(t, admin.Create(ctx, running))
	must(t, admin.Create(ctx, scaled))

	merge := func(c client.Client, obj client.Object, patch string) func() error {
		return func() error {
			return c.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(patch)), client.DryRunAll)
		}
	}
	annotate := func(pairs string) string { return `{"metadata":{"annotations":{` + pairs + `}}}` }
	const (
		now        = `"2026-11-14T08:15:00Z"`
		lastActive = `"` + annotLastActive + `":`
		wakeStart  = `"` + annotWakeStarted + `":`
		image      = `"containers":[{"name":"app","image":"evil:1"}]`
		ownerRef   = `{"metadata":{"ownerReferences":[{"apiVersion":"v1","kind":"ConfigMap","name":"x",` +
			`"uid":"11111111-1111-1111-1111-111111111111"}]}}`
	)

	// The policy compiles asynchronously; wait until it denies a write.
	waitDenied(t, merge(guard, app, `{"spec":{"port":9090}}`))

	cases := []struct {
		name  string
		allow bool
		write func() error
	}{
		{"app: last-active", true, merge(guard, app, annotate(lastActive+now))},
		{"app: last-active and wake-started", true, merge(guard, app, annotate(lastActive+now+","+wakeStart+now))},
		{"app: last-active that is not a time", false, merge(guard, app, annotate(lastActive+`"yesterday"`))},
		{"app: wake-started with an offset zone", false,
			merge(guard, app, annotate(wakeStart+`"2026-11-14T08:15:00+01:00"`))},
		{"app: another annotation", false, merge(guard, app, annotate(`"x":"y"`))},
		{"app: removing another annotation", false, merge(guard, app, annotate(`"owner":null`))},
		{"app: changing another annotation", false, merge(guard, app, annotate(`"owner":"me"`))},
		{"app: a label", false, merge(guard, app, `{"metadata":{"labels":{"x":"y"}}}`)},
		{"app: a finalizer", false, merge(guard, app, `{"metadata":{"finalizers":["bex.co/hold"]}}`)},
		{"app: an owner reference", false, merge(guard, app, ownerRef)},
		{"app: a spec field", false, merge(guard, app, `{"spec":{"port":9090}}`)},
		{"app: its image", false, merge(guard, app, `{"spec":{"image":"evil:1"}}`)},
		{"app: delete", false, func() error { return guard.Delete(ctx, app, client.DryRunAll) }},
		{"app: create", false, func() error {
			other := app.DeepCopy()
			other.ObjectMeta = metav1.ObjectMeta{Namespace: ns, Name: "other"}
			return guard.Create(ctx, other, client.DryRunAll)
		}},
		{"deployment: replicas 0 to 1", true, merge(guard, asleep, `{"spec":{"replicas":1}}`)},
		{"deployment: replicas 0 to 3", false, merge(guard, asleep, `{"spec":{"replicas":3}}`)},
		{"deployment: replicas 1 to 2", false, merge(guard, running, `{"spec":{"replicas":2}}`)},
		{"deployment: replicas 1 to 0", false, merge(guard, running, `{"spec":{"replicas":0}}`)},
		{"deployment: replicas 3 down to 1", false, merge(guard, scaled, `{"spec":{"replicas":1}}`)},
		{"deployment: replicas 0 to 1 and the image", false,
			merge(guard, asleep, `{"spec":{"replicas":1,"template":{"spec":{`+image+`}}}}`)},
		{"deployment: replicas 0 to 1 and an annotation", false, merge(guard, asleep,
			`{"metadata":{"annotations":{"x":"y"}},"spec":{"replicas":1}}`)},
		{"deployment: replicas 0 to 1 and a label", false, merge(guard, asleep,
			`{"metadata":{"labels":{"x":"y"}},"spec":{"replicas":1}}`)},
		{"deployment: replicas 0 to 1 and paused", false, merge(guard, asleep, `{"spec":{"replicas":1,"paused":true}}`)},
		{"deployment: the image alone", false, merge(guard, asleep, `{"spec":{"template":{"spec":{`+image+`}}}}`)},
		{"deployment: delete", false, func() error { return guard.Delete(ctx, asleep, client.DryRunAll) }},
		{"deployment: create", false, func() error {
			other := deployment("other", 1)
			return guard.Create(ctx, other, client.DryRunAll)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.write()
			switch {
			case tc.allow && err != nil:
				t.Fatalf("denied, want allowed: %v", err)
			case !tc.allow && err == nil:
				t.Fatal("allowed, want denied by the policy")
			case !tc.allow && !deniedByPolicy(err):
				t.Fatalf("denied, but not by the activator policy: %v", err)
			}
		})
	}

	// The policy confines only the activator: another identity may still write.
	if err := merge(admin, app, `{"spec":{"port":9090}}`)(); err != nil {
		t.Fatalf("the policy denied another identity: %v", err)
	}
}

func deployment(name string, replicas int32) *appsv1.Deployment {
	labels := map[string]string{"app.bex.co/app": name}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "example.invalid/app:1"}}},
			},
		},
	}
}

func applyPolicy(ctx context.Context, t *testing.T, c client.Client) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "gitops", "base", "activator-admission.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		obj := &unstructured.Unstructured{}
		if err := decoder.Decode(obj); errors.Is(err, io.EOF) {
			return
		} else if err != nil {
			t.Fatal(err)
		}
		must(t, c.Create(ctx, obj))
	}
}

func waitDenied(t *testing.T, write func() error) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if err := write(); deniedByPolicy(err) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the activator policy never started denying")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func deniedByPolicy(err error) bool {
	return err != nil && apierrors.IsForbidden(err) && strings.Contains(err.Error(), "bex-activator-writes")
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// The activator's own wake code, not a hand-written patch, passes the policy: a
// hibernated App is stamped with both wake annotations and its Deployment is
// raised to one replica, and an awake App only has last-active touched. wakeApp
// logs a refused write rather than returning it, so the result is read back.
func TestTheActivatorsWakePassesItsPolicy(t *testing.T) {
	ctx, admin, guard := policyCluster(t)
	const ns = "default"
	for _, name := range []string{"sleeper", "awake"} {
		must(t, admin.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
			Spec:       appv1alpha1.AppSpec{Image: "example.invalid/app:1", Port: 8080},
		}))
		must(t, admin.Create(ctx, deployment(name, 0)))
	}
	// The policy compiles asynchronously; wait until it denies a write.
	waitDenied(t, func() error {
		patch := client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":3}}`))
		return guard.Patch(ctx, deployment("awake", 0), patch, client.DryRunAll)
	})

	read := func(name string) (*appv1alpha1.App, *appsv1.Deployment) {
		app, dep := &appv1alpha1.App{}, &appsv1.Deployment{}
		must(t, admin.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, app))
		must(t, admin.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, dep))
		return app, dep
	}

	sleeper, _ := read("sleeper")
	sleeper.Status.Phase = appv1alpha1.PhaseHibernated
	wakeApp(ctx, guard, sleeper, "sleeper.example")
	app, dep := read("sleeper")
	for _, key := range []string{annotLastActive, annotWakeStarted} {
		if _, err := time.Parse(time.RFC3339, app.Annotations[key]); err != nil {
			t.Errorf("hibernated App: %s = %q after wakeApp: %v", key, app.Annotations[key], err)
		}
	}
	if replicas := ptrValue(dep.Spec.Replicas, -1); replicas != 1 {
		t.Errorf("hibernated App's Deployment replicas = %d after wakeApp, want 1", replicas)
	}

	awake, _ := read("awake")
	awake.Status.Phase = appv1alpha1.PhaseRunning
	wakeApp(ctx, guard, awake, "awake.example")
	app, _ = read("awake")
	if _, err := time.Parse(time.RFC3339, app.Annotations[annotLastActive]); err != nil {
		t.Errorf("awake App: %s = %q after wakeApp: %v", annotLastActive, app.Annotations[annotLastActive], err)
	}
	if _, ok := app.Annotations[annotWakeStarted]; ok {
		t.Errorf("awake App was stamped with %s", annotWakeStarted)
	}
}

func ptrValue(p *int32, orElse int32) int32 {
	if p == nil {
		return orElse
	}
	return *p
}
