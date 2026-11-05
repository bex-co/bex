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
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ipScanner answers a scan with the listeners recorded for that pod IP.
type ipScanner struct {
	mu   sync.Mutex
	open map[string][]int
}

func (s *ipScanner) Scan(_ context.Context, ip string) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.open[ip], nil
}

func (s *ipScanner) set(ip string, ports ...int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.open[ip] = ports
}

// readyEnvtestPod stands in for the kubelet: a Running, Ready pod of the
// Deployment's current template revision at ip.
func readyEnvtestPod(nn types.NamespacedName, name, ip string) {
	GinkgoHelper()
	tmpl := envtestDeployment(nn).Spec.Template
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace, Labels: tmpl.Labels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Image: "nginx:1"}}},
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodRunning, PodIP: ip,
		Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
	}
	Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0)) })
}

// servicePortTargets maps a Service's published ports to their targets.
func servicePortTargets(name string) map[int32]intstr.IntOrString {
	GinkgoHelper()
	svc := &corev1.Service{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, svc)).To(Succeed())
	out := map[int32]intstr.IntOrString{}
	for _, p := range svc.Spec.Ports {
		out[p.Port] = p.TargetPort
	}
	return out
}

func portKeys(m map[int32]intstr.IntOrString) []int32 {
	out := make([]int32, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

var _ = Describe("Private network ports (w3/m273)", func() {
	It("publishes a private service's undeclared listener and keeps the old revision's through a rollout", func() {
		nn := types.NamespacedName{Name: "private-runtime-ports", Namespace: "default"}
		slug := "private-runtime-ports-slug"
		scanner := &ipScanner{open: map[string][]int{}}
		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			PortScanner: scanner, portDetectEvery: time.Nanosecond,
		}
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "docker.io/library/nginx:1", Type: appv1alpha1.TypePrivateService,
				Port: 8080, Replicas: 1, Subdomain: slug,
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })

		By("serving the configured port alone before any instance is Ready")
		reconcileToFixedPoint(run)
		Expect(portKeys(servicePortTargets(nn.Name))).To(ConsistOf(int32(8080)))

		By("publishing the listeners a Ready instance binds beyond its port, never a reserved one")
		scanner.set("10.1.0.1", 8080, 9187, 18012, 6000)
		readyEnvtestPod(nn, nn.Name+"-a", "10.1.0.1")
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
		app := envtestApp(nn)
		Expect(app.Status.RuntimePorts).NotTo(BeNil())
		Expect(app.Status.RuntimePorts.Revision).To(Equal(releaseRevision(app)))
		Expect(app.Status.RuntimePorts.Ports).To(Equal([]int32{6000, 8080, 9187}))
		Expect(app.OpenPorts()).To(Equal([]int32{6000, 8080, 9187}))
		for _, name := range []string{nn.Name, slug} {
			targets := servicePortTargets(name)
			Expect(portKeys(targets)).To(ConsistOf(int32(8080), int32(6000), int32(9187)), name)
			Expect(targets[9187]).To(Equal(intstr.FromInt32(9187)))
			Expect(targets[6000]).To(Equal(intstr.FromInt32(6000)))
		}

		By("rolling to a revision that binds 9200 instead of 9187")
		oldRevision := app.Status.RuntimePorts.Revision
		app.Spec.RestartedAt = "2026-11-05T01:00:00Z"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		Expect(envtestDeployment(nn).Spec.Template.Labels[labelRevision]).NotTo(Equal(oldRevision))
		for _, name := range []string{nn.Name, slug} {
			Expect(portKeys(servicePortTargets(name))).To(ConsistOf(int32(8080), int32(6000), int32(9187)),
				"%s: nothing is dropped before the new revision has a Ready instance", name)
		}
		scanner.set("10.1.0.2", 8080, 9200, 6000)
		readyEnvtestPod(nn, nn.Name+"-b", "10.1.0.2")
		dep := envtestDeployment(nn)
		dep.Status = appsv1.DeploymentStatus{ObservedGeneration: dep.Generation, Replicas: 2, UpdatedReplicas: 1, ReadyReplicas: 2, AvailableReplicas: 2}
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
		reconcileToFixedPoint(run)
		app = envtestApp(nn)
		Expect(app.Status.RuntimePorts.Ports).To(Equal([]int32{6000, 8080, 9200}))
		Expect(app.Status.RuntimePorts.Retained).To(Equal([]int32{9187}), "the old instance still serves 9187")
		for _, name := range []string{nn.Name, slug} {
			Expect(portKeys(servicePortTargets(name))).To(ConsistOf(int32(8080), int32(6000), int32(9187), int32(9200)), name)
		}
		Expect(app.OpenPorts()).To(Equal([]int32{6000, 8080, 9187, 9200}))

		By("converging on the new revision's set once the rollout completes")
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
		app = envtestApp(nn)
		Expect(app.Status.RuntimePorts.Retained).To(BeEmpty())
		Expect(app.OpenPorts()).To(Equal([]int32{6000, 8080, 9200}))
		for _, name := range []string{nn.Name, slug} {
			Expect(portKeys(servicePortTargets(name))).To(ConsistOf(int32(8080), int32(6000), int32(9200)), name)
		}

		By("writing nothing when a rescan finds the same listeners")
		before := envtestApp(nn).ResourceVersion
		_, err := run()
		Expect(err).NotTo(HaveOccurred())
		Expect(envtestApp(nn).ResourceVersion).To(Equal(before))
	})

	It("answers a web service's private port 10000 with its primary listener", func() {
		nn := types.NamespacedName{Name: "web-private-10000", Namespace: "default"}
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "docker.io/library/nginx:1", Type: appv1alpha1.TypeWebService, Port: 8080, Replicas: 1,
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })
		reconcileToFixedPoint(run)
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports).To(HaveLen(2))
		Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)), "the primary stays first")
		Expect(svc.Spec.Ports[1].Port).To(Equal(appv1alpha1.PrivateHTTPPort))
		Expect(svc.Spec.Ports[1].TargetPort).To(Equal(svc.Spec.Ports[0].TargetPort))
		Expect(envtestApp(nn).OpenPorts()).To(Equal([]int32{8080}), "the alias is not a listener")

		By("adding nothing when the primary is already 10000")
		app := envtestApp(nn)
		app.Spec.Port = appv1alpha1.PrivateHTTPPort
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].Port).To(Equal(appv1alpha1.PrivateHTTPPort))
	})
})
