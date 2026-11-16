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
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// w3/055: the operator's in-place relabel of a running Valkey pod must pass the
// production bex-operator-pod-labels policy for every key in inPlacePodLabels.
// The policy once admitted only the isolation label, so moving a Key Value into
// an environment was denied and the serving store read Failed.
var _ = Describe("Operator pod label admission", func() {
	It("admits the Key Value in-place relabel and the zot reload stamp, and denies every other pod change", func() {
		// The policy matches only the operator's identity, but give it its own API
		// server anyway: admission's compiled cache outlives a policy delete.
		admissionEnv := &envtest.Environment{BinaryAssetsDirectory: getFirstFoundEnvTestBinaryDir()}
		DeferCleanup(admissionEnv.Stop)
		admissionConfig, err := admissionEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		admin, err := client.New(admissionConfig, client.Options{Scheme: k8sClient.Scheme()})
		Expect(err).NotTo(HaveOccurred())

		data, err := os.ReadFile("../../../../deploy/gitops/base/operator-pod-label-admission.yaml")
		Expect(err).NotTo(HaveOccurred())
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			object := &unstructured.Unstructured{}
			err := decoder.Decode(object)
			if err == io.EOF {
				break
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(admin.Create(ctx, object)).To(Succeed())
		}

		operatorName := "system:serviceaccount:bex-system:bex-controller-manager"
		binding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{GenerateName: "pod-label-test-"},
			RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "cluster-admin"},
			Subjects: []rbacv1.Subject{{Kind: "User", APIGroup: "rbac.authorization.k8s.io", Name: operatorName}}}
		Expect(admin.Create(ctx, binding)).To(Succeed())
		conf := rest.CopyConfig(admissionConfig)
		conf.Impersonate.UserName = operatorName
		operator, err := client.New(conf, client.Options{Scheme: admin.Scheme()})
		Expect(err).NotTo(HaveOccurred())

		const kvName = "red-podlabeladmission"
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: kvName + "-0", Namespace: "default",
				Labels: map[string]string{labelKeyValue: kvName, "app.bex.co/workspace": "tea-podlabel"},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1", Kind: "StatefulSet", Name: kvName, UID: "sts-uid", Controller: new(true),
				}},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "valkey", Image: "valkey/valkey:8"}}},
		}
		Expect(admin.Create(ctx, pod)).To(Succeed())

		denied := func(change func(*corev1.Pod)) bool {
			current := &corev1.Pod{}
			Expect(operator.Get(ctx, client.ObjectKeyFromObject(pod), current)).To(Succeed())
			patch := client.MergeFrom(current.DeepCopy())
			change(current)
			err := operator.Patch(ctx, current, patch, client.DryRunAll)
			return err != nil && strings.Contains(err.Error(), "bex-operator-pod-labels")
		}
		// Await the binding's activation without persisting a forbidden change.
		Eventually(func() bool {
			return denied(func(p *corev1.Pod) { p.Labels["app.bex.co/workspace"] = "tea-other" })
		}, 20*time.Second, 100*time.Millisecond).Should(BeTrue())

		r := &KeyValueReconciler{Client: operator, Scheme: admin.Scheme()}
		relabel := func(labels map[string]string) map[string]string {
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: kvName, Namespace: "default", Labels: labels}}
			Expect(r.syncKeyValuePodIsolation(ctx, kv)).To(Succeed())
			got := &corev1.Pod{}
			Expect(admin.Get(ctx, client.ObjectKeyFromObject(pod), got)).To(Succeed())
			return got.Labels
		}
		// Join an environment, become isolated, move environments, then leave.
		Expect(relabel(map[string]string{labelEnvironment: "evm-a"})).To(HaveKeyWithValue(labelEnvironment, "evm-a"))
		got := relabel(map[string]string{labelEnvironment: "evm-a", labelNetworkIsolation: "evm-a"})
		Expect(got).To(HaveKeyWithValue(labelNetworkIsolation, "evm-a"))
		got = relabel(map[string]string{labelEnvironment: "evm-b"})
		Expect(got).To(HaveKeyWithValue(labelEnvironment, "evm-b"))
		Expect(got).NotTo(HaveKey(labelNetworkIsolation))
		got = relabel(nil)
		Expect(got).NotTo(HaveKey(labelEnvironment))
		Expect(got).To(HaveKeyWithValue(labelKeyValue, kvName))

		for name, change := range map[string]func(*corev1.Pod){
			"other label":  func(p *corev1.Pod) { p.Labels[labelKeyValue] = "red-foreign" },
			"drop label":   func(p *corev1.Pod) { delete(p.Labels, "app.bex.co/workspace") },
			"image":        func(p *corev1.Pod) { p.Spec.Containers[0].Image = "evil/valkey:8" },
			"annotation":   func(p *corev1.Pod) { p.Annotations = map[string]string{"x": "y"} },
			"finalizer":    func(p *corev1.Pod) { p.Finalizers = []string{"bex.co/hold"} },
			"owner rename": func(p *corev1.Pod) { p.OwnerReferences[0].Name = "foreign" },
		} {
			Expect(denied(change)).To(BeTrue(), name)
		}

		// w4/m347: the registry's hot-reload nudge (registry.nudgeZot) stamps one
		// annotation on bex-registry/zot-0. Production denied it until the policy
		// excepted exactly that key on exactly that pod, so a new App's credential
		// waited for the kubelet's periodic sync instead of activating in seconds.
		Expect(admin.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "bex-registry"}})).To(Succeed())
		zot := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "zot-0", Namespace: "bex-registry",
				Labels: map[string]string{registry.ZotPodLabelKey: registry.ZotPodLabelValue}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "zot", Image: "ghcr.io/project-zot/zot:v2.1.22"}}},
		}
		Expect(admin.Create(ctx, zot)).To(Succeed())
		stamp := func(target *corev1.Pod, key, value string) error {
			return operator.Patch(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: target.Name, Namespace: target.Namespace}},
				client.RawPatch(types.MergePatchType, fmt.Appendf(nil, `{"metadata":{"annotations":{%q:%q}}}`, key, value)))
		}
		Expect(stamp(zot, registry.ZotReloadAnnotation, "2026-11-16T21:40:00Z")).To(Succeed())
		Expect(stamp(zot, registry.ZotReloadAnnotation, "2026-11-16T21:40:05Z")).To(Succeed(), "a later nudge restamps")
		stamped := &corev1.Pod{}
		Expect(admin.Get(ctx, client.ObjectKeyFromObject(zot), stamped)).To(Succeed())
		Expect(stamped.Annotations).To(HaveKeyWithValue(registry.ZotReloadAnnotation, "2026-11-16T21:40:05Z"))

		for name, err := range map[string]error{
			"other annotation on zot":       stamp(zot, "x", "y"),
			"reload stamp on another pod":   stamp(pod, registry.ZotReloadAnnotation, "2026-11-16T21:40:00Z"),
			"zot image":                     operator.Patch(ctx, zot, client.RawPatch(types.StrategicMergePatchType, []byte(`{"spec":{"containers":[{"name":"zot","image":"evil/zot:1"}]}}`))),
			"reload stamp removed from zot": operator.Patch(ctx, zot, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"annotations":{"`+registry.ZotReloadAnnotation+`":null}}}`))),
		} {
			Expect(err).To(HaveOccurred(), name)
			Expect(err.Error()).To(ContainSubstring("bex-operator-pod-labels"), name)
		}
	})
})
