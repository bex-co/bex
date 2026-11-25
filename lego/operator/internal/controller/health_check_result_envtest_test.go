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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m634 t003: a live release whose instance fails its health check records
// what the kubelet's Unhealthy event says the check got back, ends Ready's
// reason with it, and clears it once the instance is ready again. The events
// are listed through the API reader, with the API server's own field
// selectors.
var _ = Describe("App health-check result", func() {
	run := func(name, eventMessage string) (types.NamespacedName, *corev1.Pod) {
		skipNameValidation := true
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: k8sClient.Scheme(), Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: controllerconfig.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&AppReconciler{Client: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Scheme: mgr.GetScheme(), Mode: ModeKubernetes}).
			SetupWithManager(mgr)).To(Succeed())
		managerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mgr.Start(managerCtx) }()
		DeferCleanup(func() {
			stop()
			Eventually(done, 10*time.Second).Should(Receive(BeNil()))
		})

		nn := types.NamespacedName{Name: name, Namespace: "default"}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), app) })
		dep := &appsv1.Deployment{}
		Eventually(func() error { return k8sClient.Get(ctx, nn, dep) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		dep.Status.ObservedGeneration = dep.Generation
		dep.Status.Replicas, dep.Status.UpdatedReplicas, dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 1, 1, 1, 1
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())

		probe := &corev1.Probe{ProbeHandler: healthProbeExecHandler("/healthz", 3000, ""), PeriodSeconds: 5, TimeoutSeconds: 5}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name + "-pod", Namespace: nn.Namespace, Labels: dep.Spec.Template.Labels},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Image: "nginx:1", ReadinessProbe: probe}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), pod) })
		setReady(pod, true)
		Eventually(func(g Gomega) {
			current := &appv1alpha1.App{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			g.Expect(current.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
			g.Expect(meta.IsStatusConditionTrue(current.Status.Conditions, appv1alpha1.ConditionReady)).To(BeTrue())
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		event := &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: pod.Name + ".unhealthy", Namespace: nn.Namespace},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", APIVersion: "v1", Namespace: nn.Namespace, Name: pod.Name, UID: pod.UID,
				FieldPath: "spec.containers{" + appContainerName + "}"},
			Reason: unhealthyEventReason, Message: eventMessage, Type: corev1.EventTypeWarning, Count: 3,
			FirstTimestamp: metav1.Now(), LastTimestamp: metav1.Now(), Source: corev1.EventSource{Component: "kubelet"},
		}
		Expect(k8sClient.Create(ctx, event)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), event) })
		// Another pod's event never speaks for this one.
		other := event.DeepCopy()
		other.ResourceVersion, other.UID = "", ""
		other.Name, other.InvolvedObject.Name, other.InvolvedObject.UID = pod.Name+"-other.unhealthy", pod.Name+"-other", ""
		other.Message = "Readiness probe failed: HTTP health check failed with status code 418"
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), other) })

		setReady(pod, false)
		return nn, pod
	}

	It("records the check's line, ends Ready with it and clears it once ready", func() {
		nn, pod := run("health-check-result", "Readiness probe failed: HTTP health check failed with status code 503\n")
		result := "HTTP health check failed with status code 503"
		Eventually(func(g Gomega) {
			current := &appv1alpha1.App{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			g.Expect(current.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
			ready := meta.FindStatusCondition(current.Status.Conditions, appv1alpha1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(reasonHealthCheckFailing))
			g.Expect(ready.Message).To(HaveSuffix(" Last result: " + result + "."))
			g.Expect(current.Status.HealthCheckFailure).To(Equal(&appv1alpha1.HealthCheckFailureStatus{Instance: pod.Name, Message: result}))
		}, 15*time.Second, 100*time.Millisecond).Should(Succeed())

		setReady(pod, true)
		Eventually(func(g Gomega) {
			current := &appv1alpha1.App{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			g.Expect(meta.IsStatusConditionTrue(current.Status.Conditions, appv1alpha1.ConditionReady)).To(BeTrue())
			g.Expect(current.Status.HealthCheckFailure).To(BeNil())
		}, 15*time.Second, 100*time.Millisecond).Should(Succeed())
	})

	It("bounds a long result to what the status holds", func() {
		nn, pod := run("health-check-result-long", "Readiness probe failed: HTTP health check failed: "+strings.Repeat("x", 600))
		Eventually(func(g Gomega) {
			current := &appv1alpha1.App{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			g.Expect(current.Status.HealthCheckFailure).NotTo(BeNil())
			g.Expect(current.Status.HealthCheckFailure.Instance).To(Equal(pod.Name))
			g.Expect(current.Status.HealthCheckFailure.Message).To(HaveLen(appv1alpha1.HealthCheckFailureMaxLength))
			g.Expect(current.Status.HealthCheckFailure.Message).To(HavePrefix("HTTP health check failed: xxx"))
		}, 15*time.Second, 100*time.Millisecond).Should(Succeed())
	})
})

// setReady writes pod's status as a running app container that is, or is not,
// passing its readiness check, started long enough ago for a probe to fail.
func setReady(pod *corev1.Pod, ready bool) {
	GinkgoHelper()
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, pod)).To(Succeed())
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	started := true
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: appContainerName, Image: "nginx:1", ImageID: "nginx:1", Ready: ready, Started: &started,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now().Add(-5 * time.Minute))}},
	}}
	Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
}
