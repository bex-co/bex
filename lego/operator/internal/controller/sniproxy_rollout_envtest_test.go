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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var _ = Describe("SNI proxy rollout annotation", func() {
	// w6/m518: Argo's DaemonSet health reads the annotation, so it must follow
	// the pods by itself: set while an old pod drains beside each node's Ready
	// new pod, gone as soon as a new pod is not Ready or a node joins.
	It("follows each node's new pod, ignores draining ones, and leaves other DaemonSets alone", func() {
		const namespace = "m518-sni"
		holdFinalizer := "bex.co/m518-test-hold"
		err := k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
		if !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		skipNameValidation := true
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: k8sClient.Scheme(), Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: controllerconfig.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&SNIProxyRolloutReconciler{Client: mgr.GetClient(), Namespace: namespace}).SetupWithManager(mgr)).To(Succeed())
		managerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mgr.Start(managerCtx) }()
		DeferCleanup(func() {
			stop()
			Eventually(done, 10*time.Second).Should(Receive(BeNil()))
		})

		daemonSet := func(name string, desired int32) *appsv1.DaemonSet {
			labels := map[string]string{"app.bex.co/component": name}
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: labels},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "proxy", Image: "proxy:1"}}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, ds)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), ds) })
			ds.Status = appsv1.DaemonSetStatus{ObservedGeneration: ds.Generation, DesiredNumberScheduled: desired}
			Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())
			return ds
		}
		setReady := func(pod *corev1.Pod, ready bool) {
			status := corev1.ConditionFalse
			if ready {
				status = corev1.ConditionTrue
			}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}
			Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		}
		podOn := func(ds *appsv1.DaemonSet, node, template string) *corev1.Pod {
			labels := map[string]string{"app.bex.co/component": ds.Name, extensionsv1beta1.DaemonSetTemplateGenerationKey: template}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: ds.Name + "-", Namespace: namespace, Labels: labels,
					Finalizers:      []string{holdFinalizer},
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ds, appsv1.SchemeGroupVersion.WithKind("DaemonSet"))},
				},
				Spec: corev1.PodSpec{NodeName: node, Containers: []corev1.Container{{Name: "proxy", Image: "proxy:1"}}},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Patch(context.Background(), pod, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":null}}`)))
				_ = k8sClient.Delete(context.Background(), pod)
			})
			setReady(pod, true)
			return pod
		}
		annotation := func(ds *appsv1.DaemonSet) func() (string, error) {
			return func() (string, error) {
				var live appsv1.DaemonSet
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ds), &live); err != nil {
					return "", err
				}
				value, ok := live.Annotations[NewPodsReadyAnnotation]
				if !ok {
					return "<none>", nil
				}
				return value, nil
			}
		}

		kv := daemonSet("bex-kv-sni-proxy", 2)
		template := kv.Annotations[appsv1.DeprecatedTemplateGeneration]
		Expect(template).NotTo(BeEmpty(), "the API server sets the template generation")
		want := fmt.Sprintf("%d/2", kv.Generation)

		// Node a: the old pod drains beside its Ready new pod. Node b: new only.
		old := podOn(kv, "node-a", "0")
		Expect(k8sClient.Delete(ctx, old)).To(Succeed())
		podOn(kv, "node-a", template)
		b := podOn(kv, "node-b", template)
		Eventually(annotation(kv), 10*time.Second, 100*time.Millisecond).Should(Equal(want))

		By("a new pod that is not Ready clears it")
		setReady(b, false)
		Eventually(annotation(kv), 10*time.Second, 100*time.Millisecond).Should(Equal("<none>"))
		setReady(b, true)
		Eventually(annotation(kv), 10*time.Second, 100*time.Millisecond).Should(Equal(want))

		By("a node that joins clears it until its pod is Ready")
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(kv), kv)).To(Succeed())
		kv.Status.DesiredNumberScheduled = 3
		Expect(k8sClient.Status().Update(ctx, kv)).To(Succeed())
		Eventually(annotation(kv), 10*time.Second, 100*time.Millisecond).Should(Equal("<none>"))
		podOn(kv, "node-c", template)
		Eventually(annotation(kv), 10*time.Second, 100*time.Millisecond).Should(Equal(fmt.Sprintf("%d/3", kv.Generation)))

		By("another DaemonSet whose pods would qualify gets no annotation")
		meter := daemonSet("bex-egress-meter", 1)
		podOn(meter, "node-a", meter.Annotations[appsv1.DeprecatedTemplateGeneration])
		Consistently(annotation(meter), 2*time.Second, 200*time.Millisecond).Should(Equal("<none>"))
	})
})
