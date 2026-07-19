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
	"slices"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// TestKeyValuePublicFrontDoorMigration pins the m15 cutover: the CR keeps its
// external hostname/allowlist intent for kv-sni-proxy, while any legacy
// Traefik TCP route and middleware are removed to prevent bypass/double count.
func TestKeyValuePublicFrontDoorMigration(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(traefikIngressRouteTCPGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(traefikMiddlewareTCPGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(certManagerCertificateGVK, &unstructured.Unstructured{})

	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "acl-kv", Namespace: "default"},
		Spec: appv1alpha1.KeyValueSpec{
			Plan: "free", Public: true,
			IPAllowList: []appv1alpha1.IPAllowEntry{
				{CIDR: "203.0.113.0/24", Description: "office"},
				{CIDR: "10.0.0.0/8"},
			},
		},
	}
	legacyRoute := &unstructured.Unstructured{}
	legacyRoute.SetGroupVersionKind(traefikIngressRouteTCPGVK)
	legacyRoute.SetName("acl-kv-kv")
	legacyRoute.SetNamespace("default")
	legacyRoute.Object["spec"] = map[string]any{}
	legacyMiddleware := &unstructured.Unstructured{}
	legacyMiddleware.SetGroupVersionKind(traefikMiddlewareTCPGVK)
	legacyMiddleware.SetName("acl-kv-kv-allow")
	legacyMiddleware.SetNamespace("default")
	legacyMiddleware.Object["spec"] = map[string]any{}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, legacyRoute, legacyMiddleware).
		WithStatusSubresource(&appv1alpha1.KeyValue{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, KvDomain: "kv.example.test", ClusterIssuer: "letsencrypt-prod"}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "acl-kv", Namespace: "default"}
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if err := cl.Get(ctx, nn, kv); err != nil {
		t.Fatal(err)
	}
	if kv.Status.ExternalHost != "acl-kv.kv.example.test" {
		t.Fatalf("external host = %q", kv.Status.ExternalHost)
	}
	if len(kv.Spec.IPAllowList) != 2 {
		t.Fatal("allowlist intent was mutated")
	}
	certificate := &unstructured.Unstructured{}
	certificate.SetGroupVersionKind(certManagerCertificateGVK)
	if err := cl.Get(ctx, types.NamespacedName{Name: "acl-kv-kv-tls", Namespace: "default"}, certificate); err != nil {
		t.Fatalf("public TLS Certificate not created: %v", err)
	}
	if dnsNames, _, _ := unstructured.NestedStringSlice(certificate.Object, "spec", "dnsNames"); len(dnsNames) != 1 || dnsNames[0] != "acl-kv.kv.example.test" {
		t.Fatalf("Certificate dnsNames = %v", dnsNames)
	}
	var service corev1.Service
	if err := cl.Get(ctx, nn, &service); err != nil {
		t.Fatal(err)
	}
	if len(service.Spec.Ports) != 2 || service.Spec.Ports[1].Port != kvTLSPort {
		t.Fatalf("public Service ports = %#v, want plaintext %d + TLS %d", service.Spec.Ports, kvPort, kvTLSPort)
	}
	var sts appsv1.StatefulSet
	if err := cl.Get(ctx, nn, &sts); err != nil {
		t.Fatal(err)
	}
	args := sts.Spec.Template.Spec.Containers[0].Args
	if !slices.Contains(args, "--tls-port") || !slices.Contains(args, strconv.Itoa(kvTLSPort)) {
		t.Fatalf("public Valkey args lack TLS port: %v", args)
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: "acl-kv-kv", Namespace: "default"}, legacyRoute); !apierrors.IsNotFound(err) {
		t.Fatalf("legacy route was not deleted: %v", err)
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: "acl-kv-kv-allow", Namespace: "default"}, legacyMiddleware); !apierrors.IsNotFound(err) {
		t.Fatalf("legacy middleware was not deleted: %v", err)
	}
}

func TestKeyValuePublicFrontDoorFailsClosedWithoutIssuer(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(traefikIngressRouteTCPGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(traefikMiddlewareTCPGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(certManagerCertificateGVK, &unstructured.Unstructured{})

	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "no-issuer", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free", Public: true},
	}
	legacyRoute := &unstructured.Unstructured{}
	legacyRoute.SetGroupVersionKind(traefikIngressRouteTCPGVK)
	legacyRoute.SetName("no-issuer-kv")
	legacyRoute.SetNamespace("default")
	legacyMiddleware := &unstructured.Unstructured{}
	legacyMiddleware.SetGroupVersionKind(traefikMiddlewareTCPGVK)
	legacyMiddleware.SetName("no-issuer-kv-allow")
	legacyMiddleware.SetNamespace("default")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, legacyRoute, legacyMiddleware).
		WithStatusSubresource(&appv1alpha1.KeyValue{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, KvDomain: "kv.example.test"}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "no-issuer", Namespace: "default"}
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err == nil {
		t.Fatal("public Key Value unexpectedly reconciled without a TLS issuer")
	}
	if err := cl.Get(ctx, nn, kv); err != nil {
		t.Fatal(err)
	}
	if kv.Status.Phase != appv1alpha1.KVPhaseFailed || kv.Status.ExternalHost != "" {
		t.Fatalf("status = phase %q host %q, want failed with no public host", kv.Status.Phase, kv.Status.ExternalHost)
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: "no-issuer-kv", Namespace: "default"}, legacyRoute); !apierrors.IsNotFound(err) {
		t.Fatalf("legacy route survived fail-closed reconcile: %v", err)
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: "no-issuer-kv-allow", Namespace: "default"}, legacyMiddleware); !apierrors.IsNotFound(err) {
		t.Fatalf("legacy middleware survived fail-closed reconcile: %v", err)
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
	scheme.AddKnownTypeWithName(certManagerCertificateGVK, &unstructured.Unstructured{})

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

func TestStatefulSetRolloutReadyRejectsOldRevision(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Status: appsv1.StatefulSetStatus{
			ObservedGeneration: 2, CurrentRevision: "rev-old", UpdateRevision: "rev-new",
			CurrentReplicas: 1, UpdatedReplicas: 0, ReadyReplicas: 1, AvailableReplicas: 1,
		},
	}
	if statefulSetRolloutReady(sts, 1) {
		t.Fatal("old ready revision reported as the current StatefulSet rollout")
	}
	sts.Status.CurrentRevision = "rev-new"
	sts.Status.UpdatedReplicas = 1
	if !statefulSetRolloutReady(sts, 1) {
		t.Fatal("converged StatefulSet revision did not report ready")
	}
}

func TestKeyValuePVCExpansionAndStorageClassFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		expandable bool
		wantFailed bool
		wantGB     int32
	}{
		{name: "expandable", expandable: true, wantGB: 5},
		{name: "not-expandable", expandable: false, wantFailed: true, wantGB: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = appv1alpha1.AddToScheme(scheme)
			kv := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: "resize-kv", Namespace: "default", Generation: 2},
				Spec:       appv1alpha1.KeyValueSpec{Plan: "standard"},
				Status:     appv1alpha1.KeyValueStatus{AllocatedStorageGB: 1},
			}
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: keyValuePVCName(kv.Name), Namespace: kv.Namespace},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: ptr(kvStorageClass),
					Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					}},
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase:    corev1.ClaimBound,
					Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
				},
			}
			sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: kvStorageClass}, AllowVolumeExpansion: &tc.expandable}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, pvc, sc).Build()
			r := &KeyValueReconciler{Client: cl, Scheme: scheme}
			state, err := r.reconcileKeyValueStorage(context.Background(), kv, sts, 5)
			if err != nil {
				t.Fatal(err)
			}
			if state.failed != tc.wantFailed {
				t.Fatalf("state.failed = %t, want %t (state=%+v)", state.failed, tc.wantFailed, state)
			}
			got := &corev1.PersistentVolumeClaim{}
			if err := cl.Get(context.Background(), client.ObjectKeyFromObject(pvc), got); err != nil {
				t.Fatal(err)
			}
			if size := pvcRequestedStorageGB(got); size != tc.wantGB {
				t.Fatalf("PVC request = %d GB, want %d GB", size, tc.wantGB)
			}
			if tc.wantFailed && state.reason != "StorageClassNotExpandable" {
				t.Fatalf("failure reason = %q", state.reason)
			}
		})
	}
}

func TestKeyValueStorageShrinkIsRejectedBeforePVCMutation(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "shrink-kv", Namespace: "default", Generation: 2},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free", StorageGB: 1},
		Status: appv1alpha1.KeyValueStatus{
			AllocatedStorageGB: 5, ObservedStorageGB: 5, StorageCapacityGB: 5,
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace},
		Spec: appsv1.StatefulSetSpec{VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
			Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("5Gi"),
			}}},
		}}},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: keyValuePVCName(kv.Name), Namespace: kv.Namespace},
		Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("5Gi"),
		}}},
		Status: corev1.PersistentVolumeClaimStatus{Capacity: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("5Gi"),
		}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, sts, pvc).
		WithStatusSubresource(&appv1alpha1.KeyValue{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme}
	nn := types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace}

	result, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("rejected shrink requeued immediately: %+v", result)
	}
	gotPVC := &corev1.PersistentVolumeClaim{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(pvc), gotPVC); err != nil {
		t.Fatal(err)
	}
	if got := pvcRequestedStorageGB(gotPVC); got != 5 {
		t.Fatalf("rejected shrink mutated PVC request to %d GB, want 5", got)
	}
	gotKV := &appv1alpha1.KeyValue{}
	if err := cl.Get(context.Background(), nn, gotKV); err != nil {
		t.Fatal(err)
	}
	ready := meta.FindStatusCondition(gotKV.Status.Conditions, "Ready")
	storage := meta.FindStatusCondition(gotKV.Status.Conditions, "StorageReady")
	if gotKV.Status.Phase != appv1alpha1.KVPhaseFailed || ready == nil || storage == nil ||
		ready.Reason != "StorageShrinkRejected" || storage.Reason != "StorageShrinkRejected" {
		t.Fatalf("shrink status = phase %q ready %+v storage %+v", gotKV.Status.Phase, ready, storage)
	}
}

var _ = Describe("KeyValue Controller", func() {
	const name = "smoke-kv"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}

	// k8sClient is only set in BeforeSuite — build the reconciler lazily, never
	// in the container body. KvDomain is left empty so the reconcile takes the
	// internal-only path.
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
		Expect(sts.Spec.Template.Annotations).To(HaveKeyWithValue("cluster-autoscaler.kubernetes.io/safe-to-evict", "false"),
			"singleton stateful pod must not be bin-packed away by the autoscaler")

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

	It("expands the live PVC and waits for capacity plus the current StatefulSet revision", func() {
		expand := true
		sc := &storagev1.StorageClass{
			ObjectMeta:  metav1.ObjectMeta{Name: kvStorageClass},
			Provisioner: "example.test/noop", AllowVolumeExpansion: &expand,
		}
		Expect(k8sClient.Create(ctx, sc)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), sc) })

		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.KeyValueSpec{Plan: "free"},
		}
		Expect(k8sClient.Create(ctx, kv)).To(Succeed())
		reconcileN()

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: keyValuePVCName(name), Namespace: "default", Labels: map[string]string{labelKeyValue: name}},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &sc.Name,
				VolumeName:       "pv-smoke-kv",
				Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				}},
			},
		}
		Expect(k8sClient.Create(ctx, pvc)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), pvc) })
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)).To(Succeed())
		pvc.Status.Phase = corev1.ClaimBound
		pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}
		Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())

		Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
		kv.Spec.Plan = "standard"
		Expect(k8sClient.Update(ctx, kv)).To(Succeed())
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)).To(Succeed())
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("5Gi"))
		Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
		Expect(kv.Status.Phase).To(Equal(appv1alpha1.KVPhaseProvisioning))
		Expect(meta.FindStatusCondition(kv.Status.Conditions, "StorageReady").Reason).To(Equal("PVCResizePending"))

		pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")}
		Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())
		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
		sts.Status.ObservedGeneration = sts.Generation
		sts.Status.CurrentRevision = "rev-2"
		sts.Status.UpdateRevision = "rev-2"
		sts.Status.Replicas = 1
		sts.Status.CurrentReplicas = 1
		sts.Status.UpdatedReplicas = 1
		sts.Status.ReadyReplicas = 1
		sts.Status.AvailableReplicas = 1
		Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
		Expect(kv.Status.Phase).To(Equal(appv1alpha1.KVPhaseReady))
		Expect(meta.IsStatusConditionTrue(kv.Status.Conditions, "StorageReady")).To(BeTrue())
		Expect(kv.Status.AllocatedStorageGB).To(Equal(int32(5)))
		Expect(kv.Status.StorageCapacityGB).To(Equal(int32(5)))
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

// The w4/m29 admission contract through a REAL apiserver (envtest): the
// pre-m24 bare-CIDR-string serialization was normalized fleet-wide
// (scripts/ipallowlist-normalize.sh) and the CRD field is structural again —
// a string entry (or an object missing cidr) is REJECTED at admission, the
// structured shape round-trips with descriptions preserved, and descriptions
// never influence the rendered middleware. This inverts the m24-era test that
// pinned legacy acceptance; that contract was retired deliberately.
var _ = Describe("KeyValue ipAllowList structural schema", func() {
	const name = "structural-acl-kv"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}

	AfterEach(func() {
		if kv := (&appv1alpha1.KeyValue{}); k8sClient.Get(ctx, nn, kv) == nil {
			Expect(k8sClient.Delete(ctx, kv)).To(Succeed())
		}
	})

	It("rejects the retired string shape at admission and round-trips structured entries", func() {
		By("refusing the pre-m24 serialization (bare CIDR strings) at the apiserver")
		legacy := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "app.bex.co/v1alpha1",
			"kind":       "KeyValue",
			"metadata":   map[string]any{"name": name, "namespace": "default"},
			"spec": map[string]any{
				"plan":        "free",
				"public":      true,
				"ipAllowList": []any{"203.0.113.0/24", "10.0.0.0/8"},
			},
		}}
		Expect(k8sClient.Create(ctx, legacy)).NotTo(Succeed(),
			"the w4/m29 structural schema must reject bare-string entries")

		By("refusing an object entry missing its required cidr")
		nocidr := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "app.bex.co/v1alpha1",
			"kind":       "KeyValue",
			"metadata":   map[string]any{"name": name, "namespace": "default"},
			"spec": map[string]any{
				"plan":        "free",
				"public":      true,
				"ipAllowList": []any{map[string]any{"description": "no cidr"}},
			},
		}}
		Expect(k8sClient.Create(ctx, nocidr)).NotTo(Succeed(),
			"cidr is required on every entry")

		By("accepting and round-tripping the structured shape, descriptions preserved")
		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.KeyValueSpec{
				Plan:   "free",
				Public: true,
				IPAllowList: []appv1alpha1.IPAllowEntry{
					{CIDR: "203.0.113.0/24", Description: "office"},
					{CIDR: "10.0.0.0/8"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, kv)).To(Succeed())
		got := &appv1alpha1.KeyValue{}
		Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
		Expect(got.Spec.IPAllowList).To(Equal(kv.Spec.IPAllowList))
		// Description-blind middleware rendering is a pure function pinned by
		// TestIPAllowListMiddlewareSpec (database_test.go) — not re-asserted
		// through the apiserver here.
	})
})
