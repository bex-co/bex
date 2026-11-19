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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/drainguard"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("App platform redeploy status (drain-guard handover)", func() {
	newReconciler := func() *AppReconciler {
		return &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	}

	makeNode := func(name string, taints ...corev1.Taint) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       corev1.NodeSpec{Unschedulable: true, Taints: taints},
		}
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), node) })
	}

	makeRunningApp := func(name string, spec appv1alpha1.AppSpec) (types.NamespacedName, *appsv1.Deployment) {
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), app) })

		r := newReconciler()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		dep.Status = appsv1.DeploymentStatus{
			ObservedGeneration: dep.Generation,
			Replicas:           1,
			UpdatedReplicas:    1,
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		}
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())

		_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		return nn, dep
	}

	evacuateDeployment := func(nn types.NamespacedName, nodeName string) *appsv1.Deployment {
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		before := dep.DeepCopy()
		metav1.SetMetaDataAnnotation(&dep.Spec.Template.ObjectMeta, drainguard.EvacuatedFromAnnotation, nodeName)
		Expect(k8sClient.Patch(ctx, dep, client.MergeFrom(before))).To(Succeed())
		return dep
	}

	It("records node_replacement Started then Ended, stays idempotent across operator restart and later deploys", func() {
		makeNode("zdr-node-replace")
		nn, _ := makeRunningApp("zdr-replace-app", appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1})

		dep := evacuateDeployment(nn, "zdr-node-replace")
		evacGen := dep.Generation

		r := newReconciler()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		var app appv1alpha1.App
		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy).NotTo(BeNil())
		Expect(app.Status.PlatformRedeploy.TransitionID).To(Equal(fmt.Sprintf("zdr-node-replace-%d", evacGen)))
		Expect(app.Status.PlatformRedeploy.Node).To(Equal("zdr-node-replace"))
		Expect(app.Status.PlatformRedeploy.Generation).To(Equal(evacGen))
		Expect(app.Status.PlatformRedeploy.Trigger).To(Equal(appv1alpha1.PlatformRedeployTriggerNodeReplacement))
		Expect(app.Status.PlatformRedeploy.State).To(Equal(appv1alpha1.PlatformRedeployStateStarted))
		Expect(app.Status.PlatformRedeploy.StartedAt).NotTo(BeEmpty())
		Expect(app.Status.PlatformRedeploy.FinishedAt).To(BeEmpty())
		startedAt := app.Status.PlatformRedeploy.StartedAt

		// Mid-rollout (old + new pod): still Started, and a fresh reconciler
		// (simulating an operator restart) keeps the same transition.
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		dep.Status = appsv1.DeploymentStatus{
			ObservedGeneration: evacGen,
			Replicas:           2,
			UpdatedReplicas:    1,
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		}
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())

		restarted := newReconciler()
		_, err = restarted.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy.TransitionID).To(Equal(fmt.Sprintf("zdr-node-replace-%d", evacGen)))
		Expect(app.Status.PlatformRedeploy.StartedAt).To(Equal(startedAt))
		Expect(app.Status.PlatformRedeploy.State).To(Equal(appv1alpha1.PlatformRedeployStateStarted))
		Expect(app.Status.PlatformRedeploy.FinishedAt).To(BeEmpty())

		// Rollout complete: updated == replicas == available == 1.
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		dep.Status = appsv1.DeploymentStatus{
			ObservedGeneration: evacGen,
			Replicas:           1,
			UpdatedReplicas:    1,
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		}
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())

		_, err = restarted.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy.State).To(Equal(appv1alpha1.PlatformRedeployStateEnded))
		Expect(app.Status.PlatformRedeploy.StartedAt).To(Equal(startedAt))
		Expect(app.Status.PlatformRedeploy.FinishedAt).NotTo(BeEmpty())
		finishedAt := app.Status.PlatformRedeploy.FinishedAt

		// A later user deploy keeps the old evacuated-from annotation on the
		// Deployment template and bumps Deployment generation: must not start
		// a second platform redeploy transition.
		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		app.Spec.Image = "nginx:2"
		Expect(k8sClient.Update(ctx, &app)).To(Succeed())
		_, err = restarted.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy.TransitionID).To(Equal(fmt.Sprintf("zdr-node-replace-%d", evacGen)))
		Expect(app.Status.PlatformRedeploy.State).To(Equal(appv1alpha1.PlatformRedeployStateEnded))
		Expect(app.Status.PlatformRedeploy.FinishedAt).To(Equal(finishedAt))

		// A subsequent evacuation from a different node starts a fresh transition.
		makeNode("zdr-node-replace-2")
		dep = evacuateDeployment(nn, "zdr-node-replace-2")
		evacGen2 := dep.Generation
		_, err = restarted.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy.TransitionID).To(Equal(fmt.Sprintf("zdr-node-replace-2-%d", evacGen2)))
		Expect(app.Status.PlatformRedeploy.Node).To(Equal("zdr-node-replace-2"))
		Expect(app.Status.PlatformRedeploy.State).To(Equal(appv1alpha1.PlatformRedeployStateStarted))
		Expect(app.Status.PlatformRedeploy.FinishedAt).To(BeEmpty())
	})

	It("classifies a node with ToBeDeletedByClusterAutoscaler as node_consolidation", func() {
		makeNode("zdr-node-ca", corev1.Taint{Key: drainguard.ScaleDownTaint, Effect: corev1.TaintEffectNoSchedule})
		nn, _ := makeRunningApp("zdr-ca-app", appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1})

		dep := evacuateDeployment(nn, "zdr-node-ca")
		evacGen := dep.Generation

		r := newReconciler()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		var app appv1alpha1.App
		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy).NotTo(BeNil())
		Expect(app.Status.PlatformRedeploy.Trigger).To(Equal(appv1alpha1.PlatformRedeployTriggerNodeConsolidation))
		Expect(app.Status.PlatformRedeploy.State).To(Equal(appv1alpha1.PlatformRedeployStateStarted))

		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		dep.Status = appsv1.DeploymentStatus{
			ObservedGeneration: evacGen,
			Replicas:           1,
			UpdatedReplicas:    1,
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		}
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
		_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy.State).To(Equal(appv1alpha1.PlatformRedeployStateEnded))
		Expect(app.Status.PlatformRedeploy.FinishedAt).NotTo(BeEmpty())
	})

	It("ends the transition when hold expiry removes the old pod before the replacement is ready", func() {
		makeNode("zdr-node-expire")
		nn, dep := makeRunningApp("zdr-expire-app", appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1})

		rs := &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name + "-rs", Namespace: nn.Namespace},
			Spec: appsv1.ReplicaSetSpec{
				Selector: dep.Spec.Selector,
				Template: dep.Spec.Template,
			},
		}
		Expect(k8sClient.Create(ctx, rs)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), rs) })
		controllerRef := metav1.NewControllerRef(rs, appsv1.SchemeGroupVersion.WithKind("ReplicaSet"))

		oldPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: nn.Name + "-old", Namespace: nn.Namespace,
				Labels: dep.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{*controllerRef},
			},
			Spec: corev1.PodSpec{
				NodeName:   "zdr-node-expire",
				Containers: []corev1.Container{{Name: "app", Image: "nginx:1"}},
			},
		}
		Expect(k8sClient.Create(ctx, oldPod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), oldPod) })
		oldPod.Status.Phase = corev1.PodRunning
		oldPod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		Expect(k8sClient.Status().Update(ctx, oldPod)).To(Succeed())

		newPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: nn.Name + "-new", Namespace: nn.Namespace,
				Labels: dep.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{*controllerRef},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app", Image: "nginx:1"}},
			},
		}
		Expect(k8sClient.Create(ctx, newPod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), newPod) })
		newPod.Status.Phase = corev1.PodPending
		Expect(k8sClient.Status().Update(ctx, newPod)).To(Succeed())

		dep = evacuateDeployment(nn, "zdr-node-expire")
		evacGen := dep.Generation
		dep.Status = appsv1.DeploymentStatus{
			ObservedGeneration: evacGen,
			Replicas:           2,
			UpdatedReplicas:    1,
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		}
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())

		r := newReconciler()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		var app appv1alpha1.App
		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy).NotTo(BeNil())
		Expect(app.Status.PlatformRedeploy.State).To(Equal(appv1alpha1.PlatformRedeployStateStarted))

		// Hold expires: old pod evicted and terminated from zdr-node-expire
		// while newPod is still Pending (not ready).
		Expect(k8sClient.Delete(ctx, oldPod, client.GracePeriodSeconds(0))).To(Succeed())
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		dep.Status = appsv1.DeploymentStatus{
			ObservedGeneration: evacGen,
			Replicas:           1,
			UpdatedReplicas:    1,
			ReadyReplicas:      0,
			AvailableReplicas:  0,
		}
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())

		_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy.State).To(Equal(appv1alpha1.PlatformRedeployStateEnded))
		Expect(app.Status.PlatformRedeploy.FinishedAt).NotTo(BeEmpty())
	})

	It("records no transition for a Recreate (disk-attached) Deployment", func() {
		makeNode("zdr-node-disk")
		nn, dep := makeRunningApp("zdr-disk-app", appv1alpha1.AppSpec{
			Image:    "nginx:1",
			Port:     3000,
			Replicas: 1,
			Disk:     &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 1},
		})
		Expect(dep.Spec.Strategy.Type).To(Equal(appsv1.RecreateDeploymentStrategyType))

		evacuateDeployment(nn, "zdr-node-disk")
		r := newReconciler()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		var app appv1alpha1.App
		Expect(k8sClient.Get(ctx, nn, &app)).To(Succeed())
		Expect(app.Status.PlatformRedeploy).To(BeNil())
	})
})
