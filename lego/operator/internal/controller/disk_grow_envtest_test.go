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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/157 measured a grow on production: the claim waited for the kubelet's
// next pod sync for up to a minute, and DiskReady stayed False up to 30 s after
// the filesystem had grown, because nothing told the App controller the claim
// had moved. Both now follow the claim's own status, with no App change
// (w6/m557).
var _ = Describe("Persistent disk grow", func() {
	It("asks the node as soon as the claim waits for it, and is ready as soon as the claim has grown", func() {
		const className = "disk-expandable-grow"
		sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: className},
			Provisioner: "example.com/mock", AllowVolumeExpansion: new(true)}
		if err := k8sClient.Create(ctx, sc); err != nil {
			Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
		}

		skipNameValidation := true
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: k8sClient.Scheme(), Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: controllerconfig.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&AppReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Mode: ModeKubernetes,
			DiskStorageClass: className}).SetupWithManager(mgr)).To(Succeed())
		managerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mgr.Start(managerCtx) }()
		DeferCleanup(func() {
			stop()
			Eventually(done, 10*time.Second).Should(Receive(BeNil()))
		})

		nn := types.NamespacedName{Name: "disk-grow-watch", Namespace: "default"}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1, Tier: "starter",
				Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(context.Background(), app))).To(Succeed()) })

		claimKey := types.NamespacedName{Name: diskPVCName(nn.Name), Namespace: nn.Namespace}
		claim := &corev1.PersistentVolumeClaim{}
		Eventually(func() error { return k8sClient.Get(ctx, claimKey, claim) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		claim.Status.Phase = corev1.ClaimBound
		claim.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")}
		Expect(k8sClient.Status().Update(ctx, claim)).To(Succeed())

		// A served instance, so the App settles and nothing but the claim moves.
		dep := &appsv1.Deployment{}
		Eventually(func() error { return k8sClient.Get(ctx, nn, dep) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		dep.Status.ObservedGeneration = dep.Generation
		dep.Status.Replicas, dep.Status.UpdatedReplicas, dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 1, 1, 1, 1
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name + "-abc-1", Namespace: nn.Namespace, Labels: dep.Spec.Template.Labels,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: kindReplicaSet, Name: nn.Name + "-abc",
					UID: "rs-uid", Controller: new(true)}}},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: appContainerName, Image: "nginx:1"}},
				Volumes:    dep.Spec.Template.Spec.Volumes,
			},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), pod) })
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

		diskReady := func(g Gomega) *metav1.Condition {
			current := &appv1alpha1.App{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			condition := meta.FindStatusCondition(current.Status.Conditions, appv1alpha1.ConditionDiskReady)
			g.Expect(condition).NotTo(BeNil())
			return condition
		}
		Eventually(func(g Gomega) {
			g.Expect(diskReady(g).Status).To(Equal(metav1.ConditionTrue))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		template := dep.Spec.Template.DeepCopy()
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Spec.Disk.SizeGB = 11
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, claimKey, claim)).To(Succeed())
			g.Expect(claim.Spec.Resources.Requests.Storage().String()).To(Equal("11Gi"))
			g.Expect(diskReady(g).Reason).To(Equal("DiskResizePending"))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		grownGeneration := app.Generation

		// The controller step is done and the claim waits for the node. The
		// running instance is stamped within the 5 s budget, with no App change.
		Expect(k8sClient.Get(ctx, claimKey, claim)).To(Succeed())
		claim.Status.AllocatedResources = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("11Gi")}
		claim.Status.AllocatedResourceStatuses = map[corev1.ResourceName]corev1.ClaimResourceStatus{
			corev1.ResourceStorage: corev1.PersistentVolumeClaimNodeResizePending}
		Expect(k8sClient.Status().Update(ctx, claim)).To(Succeed())
		Eventually(func(g Gomega) {
			current := &corev1.Pod{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), current)).To(Succeed())
			g.Expect(current.Annotations).To(HaveKeyWithValue(annotDiskResizeRequested, "11"))
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

		// The node grew the filesystem: DiskReady follows the claim alone.
		Expect(k8sClient.Get(ctx, claimKey, claim)).To(Succeed())
		claim.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("11Gi")}
		claim.Status.AllocatedResourceStatuses = nil
		Expect(k8sClient.Status().Update(ctx, claim)).To(Succeed())
		Eventually(func(g Gomega) {
			condition := diskReady(g)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Message).To(HavePrefix("11GB disk mounted"))
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Generation).To(Equal(grownGeneration), "the claim alone must move DiskReady")
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(dep.Spec.Template).To(Equal(*template), "a grow must never roll the instance")
	})
})
