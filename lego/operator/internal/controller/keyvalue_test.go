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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestResolveKVPlan(t *testing.T) {
	// known plan (from the shared lego/types/tiers valkey family)
	if p, gb := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "standard"}); p.Memory != "1Gi" || gb != 5 {
		t.Errorf("standard => mem %q storage %d, want 1Gi/5", p.Memory, gb)
	}
	// unknown plan falls back to free
	if p, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "nonsense"}); p.Memory != "128Mi" {
		t.Errorf("unknown plan should default to free (128Mi), got %q", p.Memory)
	}
	// storage only grows: a request below the plan floor is raised to the floor
	if _, gb := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "standard", StorageGB: 2}); gb != 5 {
		t.Errorf("storage below plan floor should be raised to 5, got %d", gb)
	}
	// a larger request is honored
	if _, gb := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "free", StorageGB: 10}); gb != 10 {
		t.Errorf("larger storage request should be honored, got %d", gb)
	}
}

func TestValkeyImage(t *testing.T) {
	if got := valkeyImage(""); got != kvDefaultImage {
		t.Errorf("empty version => %q, want default %q", got, kvDefaultImage)
	}
	if got := valkeyImage("7"); got != "valkey/valkey:7-alpine" {
		t.Errorf("version 7 => %q, want valkey/valkey:7-alpine", got)
	}
}

// TestKeyValueIPAllowListProjection drives the full reconcile against a fake
// client with the Traefik kinds registered (envtest has no Traefik CRDs) and
// pins the allowlist contract: public + non-empty ipAllowList => a MiddlewareTCP
// with those CIDRs, referenced by the SNI route; emptying the list removes the
// middleware and the route reference; going private removes route + middleware.
func TestKeyValueIPAllowListProjection(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(traefikIngressRouteTCPGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(traefikMiddlewareTCPGVK, &unstructured.Unstructured{})

	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "acl-kv", Namespace: "default"},
		Spec: appv1alpha1.KeyValueSpec{
			Plan: "free", Public: true,
			IPAllowList: []string{"203.0.113.0/24", "10.0.0.0/8"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).
		WithStatusSubresource(&appv1alpha1.KeyValue{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, KvDomain: "kv.example.test"}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "acl-kv", Namespace: "default"}
	mwNN := types.NamespacedName{Name: "acl-kv-kv-allow", Namespace: "default"}
	routeNN := types.NamespacedName{Name: "acl-kv-kv", Namespace: "default"}
	reconcile1 := func() {
		t.Helper()
		if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}
	getObj := func(gvk types.NamespacedName, kind string) (*unstructured.Unstructured, error) {
		o := &unstructured.Unstructured{}
		if kind == "route" {
			o.SetGroupVersionKind(traefikIngressRouteTCPGVK)
		} else {
			o.SetGroupVersionKind(traefikMiddlewareTCPGVK)
		}
		return o, cl.Get(ctx, gvk, o)
	}

	// Public + allowlist => middleware with the CIDRs, referenced by the route.
	reconcile1()
	mw, err := getObj(mwNN, "middleware")
	if err != nil {
		t.Fatalf("middleware not created: %v", err)
	}
	ranges, _, _ := unstructured.NestedSlice(mw.Object, "spec", "ipAllowList", "sourceRange")
	if len(ranges) != 2 || ranges[0] != "203.0.113.0/24" || ranges[1] != "10.0.0.0/8" {
		t.Fatalf("sourceRange = %v", ranges)
	}
	route, err := getObj(routeNN, "route")
	if err != nil {
		t.Fatalf("route not created: %v", err)
	}
	routes, _, _ := unstructured.NestedSlice(route.Object, "spec", "routes")
	mws, ok := routes[0].(map[string]any)["middlewares"].([]any)
	if !ok || len(mws) != 1 {
		t.Fatalf("route middlewares = %v, want 1 ref", routes[0].(map[string]any)["middlewares"])
	}
	if ref := mws[0].(map[string]any); ref["name"] != "acl-kv-kv-allow" || ref["namespace"] != "default" {
		t.Fatalf("middleware ref = %v", ref)
	}

	// Emptying the list removes the middleware and the route reference.
	if err := cl.Get(ctx, nn, kv); err != nil {
		t.Fatalf("get kv: %v", err)
	}
	kv.Spec.IPAllowList = nil
	if err := cl.Update(ctx, kv); err != nil {
		t.Fatalf("clear allowlist: %v", err)
	}
	reconcile1()
	if _, err := getObj(mwNN, "middleware"); !apierrors.IsNotFound(err) {
		t.Fatalf("emptied allowlist must delete the middleware, got %v", err)
	}
	route, err = getObj(routeNN, "route")
	if err != nil {
		t.Fatalf("route must survive an emptied allowlist: %v", err)
	}
	routes, _, _ = unstructured.NestedSlice(route.Object, "spec", "routes")
	if _, has := routes[0].(map[string]any)["middlewares"]; has {
		t.Fatalf("emptied allowlist must drop the route's middleware ref: %v", routes[0])
	}

	// Going private removes the route and any middleware.
	if err := cl.Get(ctx, nn, kv); err != nil {
		t.Fatalf("get kv: %v", err)
	}
	kv.Spec.Public = false
	kv.Spec.IPAllowList = []string{"203.0.113.0/24"}
	if err := cl.Update(ctx, kv); err != nil {
		t.Fatalf("make private: %v", err)
	}
	reconcile1()
	if _, err := getObj(routeNN, "route"); !apierrors.IsNotFound(err) {
		t.Fatalf("private store must have no route, got %v", err)
	}
	if _, err := getObj(mwNN, "middleware"); !apierrors.IsNotFound(err) {
		t.Fatalf("private store must have no allowlist middleware, got %v", err)
	}
}

func TestKvResources(t *testing.T) {
	plan, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "starter"})
	got := kvResources(plan)
	// Guaranteed QoS: requests == limits, from the catalog.
	if !got.Requests.Cpu().Equal(*got.Limits.Cpu()) || !got.Requests.Memory().Equal(*got.Limits.Memory()) {
		t.Errorf("requests must equal limits (Guaranteed QoS): req=%+v lim=%+v", got.Requests, got.Limits)
	}
	if got.Requests.Memory().String() != "256Mi" {
		t.Errorf("starter memory request = %s, want 256Mi", got.Requests.Memory())
	}
}

// TestKeyValuePlanChangeReconcile confirms that updating spec.plan on an
// existing KeyValue CR results in the StatefulSet container resources being
// updated on the very next reconcile — the "plan/version bump rolls" contract
// documented in the reconciler (the pod template is reapplied every reconcile).
func TestKeyValuePlanChangeReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(traefikIngressRouteTCPGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(traefikMiddlewareTCPGVK, &unstructured.Unstructured{})

	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "plan-change-kv", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).
		WithStatusSubresource(&appv1alpha1.KeyValue{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "plan-change-kv", Namespace: "default"}

	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	var sts appsv1.StatefulSet
	if err := cl.Get(ctx, nn, &sts); err != nil {
		t.Fatalf("StatefulSet not created: %v", err)
	}
	freeMem := sts.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String()
	if freeMem != "128Mi" {
		t.Errorf("free plan memory = %s, want 128Mi", freeMem)
	}

	// Patch spec.plan to "standard" and reconcile again.
	if err := cl.Get(ctx, nn, kv); err != nil {
		t.Fatalf("get kv: %v", err)
	}
	kv.Spec.Plan = "standard"
	if err := cl.Update(ctx, kv); err != nil {
		t.Fatalf("update plan: %v", err)
	}
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("plan-change reconcile: %v", err)
	}
	if err := cl.Get(ctx, nn, &sts); err != nil {
		t.Fatalf("get StatefulSet after plan change: %v", err)
	}
	standardMem := sts.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String()
	if standardMem != "1Gi" {
		t.Errorf("standard plan memory after change = %s, want 1Gi (plan change must update pod resources)", standardMem)
	}
}

var _ = Describe("KeyValue Controller", func() {
	const name = "smoke-kv"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}

	// k8sClient is only set in BeforeSuite — build the reconciler lazily, never
	// in the container body. KvDomain is left empty so the reconcile takes the
	// internal-only path: the Traefik CRD is not installed in envtest, so the
	// public-route branch is covered by ingressRouteTCPSpec's pure-function test
	// (shared with the Database controller in database_test.go).
	var r *KeyValueReconciler
	BeforeEach(func() {
		r = &KeyValueReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	})
	reconcileN := func() {
		for range 2 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	AfterEach(func() {
		if kv := (&appv1alpha1.KeyValue{}); k8sClient.Get(ctx, nn, kv) == nil {
			Expect(k8sClient.Delete(ctx, kv)).To(Succeed())
		}
	})

	It("reconciles a tier-sized StatefulSet + headless Service + credentials Secret, owner-referenced", func() {
		By("creating a standard KeyValue")
		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.KeyValueSpec{Plan: "standard"},
		}
		Expect(k8sClient.Create(ctx, kv)).To(Succeed())
		reconcileN()

		By("projecting the tier's compute + storage onto the StatefulSet")
		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
		Expect(*sts.Spec.Replicas).To(Equal(int32(1)), "MVP plans are single-instance")
		c := sts.Spec.Template.Spec.Containers[0]
		Expect(c.Image).To(Equal(kvDefaultImage))
		Expect(c.Resources.Requests.Memory().String()).To(Equal("1Gi"), "standard memory")
		Expect(c.Resources.Limits.Memory().Equal(*c.Resources.Requests.Memory())).To(BeTrue(), "Guaranteed QoS")
		pvc := sts.Spec.VolumeClaimTemplates[0]
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("5Gi"), "standard storage floor")
		Expect(*pvc.Spec.StorageClassName).To(Equal(kvStorageClass))

		By("creating a headless Service giving the internal DNS name")
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone), "headless for StatefulSet identity")
		Expect(svc.Spec.Ports[0].Port).To(Equal(int32(kvPort)))

		By("creating a credentials Secret with a stable password + connection URI")
		sec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, nn, sec)).To(Succeed())
		firstPw := string(sec.Data["password"])
		Expect(firstPw).NotTo(BeEmpty())
		Expect(string(sec.Data["uri"])).To(ContainSubstring("redis://default:"), "internal URI form")
		Expect(string(sec.Data["host"])).To(Equal("smoke-kv.default.svc"))
		// a second reconcile must not rotate the password
		reconcileN()
		Expect(k8sClient.Get(ctx, nn, sec)).To(Succeed())
		Expect(string(sec.Data["password"])).To(Equal(firstPw), "password is reused, not regenerated")

		By("recording status coordinates")
		Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
		Expect(kv.Status.Host).To(Equal("smoke-kv.default.svc"))
		Expect(kv.Status.SecretName).To(Equal(name))
		Expect(kv.Status.Port).To(Equal(int32(kvPort)))

		// Delete cascade: envtest runs no garbage-collector controller, so it
		// can't collect the children here. The owner references asserted below
		// are exactly what k8s GC uses to cascade-delete in a real cluster —
		// controller=true pointing at the KeyValue — so this is the verifiable
		// half of "deleting the KeyValue removes the owned objects."
		By("owner-referencing every owned object to the KeyValue (drives delete cascade)")
		Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
		for _, obj := range []metav1.Object{sts, svc, sec} {
			Expect(metav1.IsControlledBy(obj, kv)).To(BeTrue())
		}
	})

	It("scales to zero when suspended and back on resume, preserving the Secret", func() {
		By("creating a running KeyValue")
		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.KeyValueSpec{Plan: "free"},
		}
		Expect(k8sClient.Create(ctx, kv)).To(Succeed())
		reconcileN()
		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
		Expect(*sts.Spec.Replicas).To(Equal(int32(1)), "running => one replica")
		sec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, nn, sec)).To(Succeed())
		pw := string(sec.Data["password"])

		By("suspending: the StatefulSet scales to zero, the Secret is kept")
		Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
		kv.Spec.Suspended = true
		Expect(k8sClient.Update(ctx, kv)).To(Succeed())
		reconcileN()
		Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
		Expect(*sts.Spec.Replicas).To(Equal(int32(0)), "suspended => zero replicas")
		Expect(k8sClient.Get(ctx, nn, sec)).To(Succeed())
		Expect(string(sec.Data["password"])).To(Equal(pw), "password preserved across suspend")
		// A suspended store settles Ready (it desires zero replicas) with a
		// Suspended reason — distinct from the running Provisioned reason.
		Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
		Expect(kv.Status.Phase).To(Equal(appv1alpha1.KVPhaseReady))
		Expect(meta.IsStatusConditionTrue(kv.Status.Conditions, "Ready")).To(BeTrue())
		Expect(meta.FindStatusCondition(kv.Status.Conditions, "Ready").Reason).To(Equal("Suspended"))

		By("resuming: the StatefulSet scales back to one replica with the same password")
		Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
		kv.Spec.Suspended = false
		Expect(k8sClient.Update(ctx, kv)).To(Succeed())
		reconcileN()
		Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
		Expect(*sts.Spec.Replicas).To(Equal(int32(1)), "resumed => one replica")
		Expect(k8sClient.Get(ctx, nn, sec)).To(Succeed())
		Expect(string(sec.Data["password"])).To(Equal(pw), "password preserved across resume")
	})

	It("treats a reconciled-then-deleted KeyValue as a clean no-op", func() {
		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.KeyValueSpec{Plan: "free"},
		}
		Expect(k8sClient.Create(ctx, kv)).To(Succeed())
		reconcileN()
		Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
		Expect(k8sClient.Delete(ctx, kv)).To(Succeed())

		// Once the KeyValue is gone, Reconcile's Get returns NotFound and is
		// ignored — no error, no re-creation attempt.
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	})
})
