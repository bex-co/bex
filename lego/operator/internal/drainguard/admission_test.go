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

package drainguard

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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// drainGuardUser is the identity the admission policy confines.
const drainGuardUser = "system:serviceaccount:bex-system:bex-drain-guard"

// TestAdmissionPolicyConfinesDrainGuard runs the production policy
// (deploy/gitops/base/drain-guard-admission.yaml) in a real API server. The
// identity holds cluster-admin here, so every denial below is the policy's,
// not RBAC's: drain-guard may set or clear its own hook, cordon, and mark an
// App for evacuation, and nothing else.
func TestAdmissionPolicyConfinesDrainGuard(t *testing.T) {
	env := &envtest.Environment{BinaryAssetsDirectory: envtestAssets(t), CRDs: []*apiextensionsv1.CustomResourceDefinition{machineCRD()}}
	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("envtest: %v", err)
	}
	t.Cleanup(func() { _ = env.Stop() })
	ctx := context.Background()
	admin, err := client.New(cfg, client.Options{Scheme: testScheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	applyPolicy(ctx, t, admin)
	must(t, admin.Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "drain-guard-test-admin"},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: drainGuardUser}},
	}))

	// Fixtures, created as admin.
	must(t, admin.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: capiNS}}))
	must(t, admin.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tenantNS}}))
	plain, hookedM := testMachine("plain", false), testMachine("hooked", true)
	must(t, admin.Create(ctx, plain))
	must(t, admin.Create(ctx, hookedM))
	nodeA := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: doomed}}
	must(t, admin.Create(ctx, nodeA))
	dep := deployment("web", 0, appsv1.RollingUpdateDeploymentStrategyType)
	dep.ObjectMeta = metav1.ObjectMeta{Namespace: tenantNS, Name: "web"}
	dep.Status = appsv1.DeploymentStatus{}
	dep.Spec.Template.Labels = map[string]string{appLabel: "web"}
	dep.Spec.Template.Spec.Containers = []corev1.Container{{Name: "app", Image: "example.invalid/app:1"}}
	must(t, admin.Create(ctx, dep))

	impersonated := rest.CopyConfig(cfg)
	impersonated.Impersonate = rest.ImpersonationConfig{UserName: drainGuardUser}
	guard, err := client.New(impersonated, client.Options{Scheme: testScheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	merge := func(obj client.Object, patch string) func() error {
		return func() error {
			return guard.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(patch)), client.DryRunAll)
		}
	}
	annotation := func(key, value string) string {
		return `{"metadata":{"annotations":{"` + key + `":` + value + `}}}`
	}
	templateAnnotation := func(key, value string) string {
		return `{"spec":{"template":{"metadata":{"annotations":{"` + key + `":"` + value + `"}}}}}`
	}

	// The policy compiles asynchronously; wait until it denies a write.
	waitDenied(t, merge(nodeA, `{"metadata":{"labels":{"bex.co/pool":"platform"}}}`))

	cases := []struct {
		name  string
		allow bool
		write func() error
	}{
		{"machine: add its own hook", true, merge(plain, annotation(HookAnnotation, `"`+hookOwner+`"`))},
		{"machine: clear its own hook", true, merge(hookedM, annotation(HookAnnotation, "null"))},
		{"machine: hook with another owner", false, merge(plain, annotation(HookAnnotation, `"someone-else"`))},
		{"machine: another owner's hook", false, merge(plain, annotation("pre-drain.delete.hook.machine.cluster.x-k8s.io/other", `"x"`))},
		{"machine: any other annotation", false, merge(plain, annotation("x", `"y"`))},
		{"machine: a label", false, merge(plain, `{"metadata":{"labels":{"x":"y"}}}`)},
		{"machine: a spec field", false, merge(plain, `{"spec":{"clusterName":"other"}}`)},
		{"machine: delete", false, func() error { return guard.Delete(ctx, plain, client.DryRunAll) }},
		{"node: cordon", true, merge(nodeA, `{"spec":{"unschedulable":true}}`)},
		{"node: uncordon", false, merge(nodeA, `{"spec":{"unschedulable":false}}`)},
		{"node: cordon plus a label", false, merge(nodeA, `{"metadata":{"labels":{"x":"y"}},"spec":{"unschedulable":true}}`)},
		{"node: cordon plus a taint", false, merge(nodeA, `{"spec":{"unschedulable":true,"taints":[{"key":"x","effect":"NoSchedule"}]}}`)},
		{"node: an annotation", false, merge(nodeA, annotation("x", `"y"`))},
		{"node: delete", false, func() error { return guard.Delete(ctx, nodeA, client.DryRunAll) }},
		{"deployment: evacuation annotation", true, merge(dep, templateAnnotation(EvacuatedFromAnnotation, doomed))},
		{"deployment: evacuation to a non-node value", false, merge(dep, templateAnnotation(EvacuatedFromAnnotation, "Not A Node!"))},
		{"deployment: another template annotation", false, merge(dep, templateAnnotation("kubectl.kubernetes.io/restartedAt", "now"))},
		{"deployment: replicas", false, merge(dep, `{"spec":{"replicas":3}}`)},
		{"deployment: image", false, merge(dep, `{"spec":{"template":{"spec":{"containers":[{"name":"app","image":"evil:1"}]}}}}`)},
		{"deployment: an annotation", false, merge(dep, annotation("x", `"y"`))},
		{"deployment: create", false, func() error {
			other := dep.DeepCopy()
			other.ObjectMeta = metav1.ObjectMeta{Namespace: tenantNS, Name: "other"}
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
				t.Fatalf("denied, but not by the drain-guard policy: %v", err)
			}
		})
	}

	// The policy confines only drain-guard: another identity may still label.
	if err := admin.Patch(ctx, nodeA, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"labels":{"x":"y"}}}`)), client.DryRunAll); err != nil {
		t.Fatalf("the policy denied another identity: %v", err)
	}
}

// envtestAssets finds the API server binaries `make test` downloads, or skips.
func envtestAssets(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("KUBEBUILDER_ASSETS"); dir != "" {
		return dir
	}
	matches, _ := filepath.Glob(filepath.Join("..", "..", "bin", "k8s", "*"))
	if len(matches) == 0 {
		t.Skip("no envtest binaries: run through `make test`")
	}
	return matches[0]
}

// machineCRD is just enough of CAPI's Machine for admission to see one.
func machineCRD() *apiextensionsv1.CustomResourceDefinition {
	preserve := true
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "machines.cluster.x-k8s.io"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: MachineGVK.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{Plural: "machines", Singular: "machine", Kind: "Machine", ListKind: "MachineList"},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name: MachineGVK.Version, Served: true, Storage: true,
				Schema:       &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object", XPreserveUnknownFields: &preserve}},
				Subresources: &apiextensionsv1.CustomResourceSubresources{Status: &apiextensionsv1.CustomResourceSubresourceStatus{}},
			}},
		},
	}
}

func testMachine(name string, hooked bool) *unstructured.Unstructured {
	m := NewMachine()
	m.SetNamespace(capiNS)
	m.SetName(name)
	m.SetLabels(map[string]string{"nodepool": "bex-tenant-0"})
	_ = unstructured.SetNestedField(m.Object, "bex", "spec", "clusterName")
	if hooked {
		withHook(m)
	}
	return m
}

func applyPolicy(ctx context.Context, t *testing.T, c client.Client) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "gitops", "base", "drain-guard-admission.yaml"))
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
			t.Fatal("the drain-guard admission policy never took effect")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func deniedByPolicy(err error) bool {
	return err != nil && strings.Contains(err.Error(), "bex-drain-guard-writes")
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
