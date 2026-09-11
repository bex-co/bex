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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Platform alias admission", func() {
	It("admits actual bounded constructors and denies routing escapes on create and update", func() {
		data, err := os.ReadFile("../../../../deploy/gitops/base/operator-alias-admission.yaml")
		Expect(err).NotTo(HaveOccurred())
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			object := &unstructured.Unstructured{}
			err := decoder.Decode(object)
			if err == io.EOF {
				break
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Create(ctx, object)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, object)).To(Succeed()) })
		}

		operatorName := "system:serviceaccount:bex-system:bex-controller-manager"
		otherName := "system:serviceaccount:bex-system:untrusted"
		// Equal RBAC permissions let the negative actor prove admission, not RBAC.
		binding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{GenerateName: "alias-test-"},
			RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "cluster-admin"},
			Subjects: []rbacv1.Subject{{Kind: "User", APIGroup: "rbac.authorization.k8s.io", Name: operatorName}, {Kind: "User", APIGroup: "rbac.authorization.k8s.io", Name: otherName}}}
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, binding)).To(Succeed()) })
		asUser := func(name string) client.Client {
			conf := rest.CopyConfig(cfg)
			conf.Impersonate.UserName = name
			c, err := client.New(conf, client.Options{Scheme: k8sClient.Scheme()})
			Expect(err).NotTo(HaveOccurred())
			return c
		}
		operator := asUser(operatorName)
		other := asUser(otherName)
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "alias-test-", Labels: map[string]string{
			"app.kubernetes.io/managed-by": "bex-controlplane", "app.kubernetes.io/part-of": "bex",
			"app.bex.co/regime": "hosting", "app.bex.co/workspace": "tea-alias-test",
		}}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, ns)).To(Succeed()) })
		for _, namespace := range []string{"default", ns.Name} {
			// Await each binding's activation without persisting a forbidden object.
			sentinel := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "alias-probe", Namespace: namespace}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "example.com"}}
			Eventually(func() bool {
				err := other.Create(ctx, sentinel.DeepCopy(), client.DryRunAll)
				return err != nil && strings.Contains(err.Error(), "bex-operator-platform-aliases")
			}, 20*time.Second, 100*time.Millisecond).Should(BeTrue())
			r := &AppReconciler{Client: operator, Scheme: k8sClient.Scheme(), StaticServerService: "bex-static-server", ActivatorService: "bex-activator", ActivatorPort: 8888}
			for _, family := range []struct {
				prefix    string
				reconcile func(*appv1alpha1.App) (string, error)
			}{
				{"bex-static-", func(a *appv1alpha1.App) (string, error) { return r.reconcileStaticServerAlias(ctx, a) }},
				{"bex-maintenance-", func(a *appv1alpha1.App) (string, error) { return r.reconcileMaintenanceAlias(ctx, a) }},
				{"bex-activator-", func(a *appv1alpha1.App) (string, error) { return r.reconcileActivatorAlias(ctx, a) }},
			} {
				for _, fullLength := range []int{62, 63, 64, 63 + len(family.prefix)} {
					appName := strings.Repeat("a", fullLength-len(family.prefix))
					app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: namespace, UID: types.UID("alias-app-" + appName)}}
					name, err := family.reconcile(app)
					Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("%s/%d/%s", family.prefix, fullLength, namespace))
					alias := &corev1.Service{}
					Expect(operator.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, alias)).To(Succeed())
					// Cleanup Service explicitly: envtest has no garbage collector.
					DeferCleanup(func() { Expect(k8sClient.Delete(ctx, alias)).To(Succeed()) })
					Expect(name).To(Equal(platformAliasName(family.prefix, appName)))
					update := alias.DeepCopy()
					update.Annotations = map[string]string{"test": "valid-update"}
					Expect(operator.Update(ctx, update)).To(Succeed())
					Expect(operator.Get(ctx, client.ObjectKeyFromObject(alias), alias)).To(Succeed())
					for _, mutation := range []struct {
						name   string
						change func(*corev1.Service)
					}{
						{"target", func(s *corev1.Service) { s.Spec.ExternalName = "example.com" }},
						{"port", func(s *corev1.Service) { s.Spec.Ports[0].Port = 9999 }},
						{"owner", func(s *corev1.Service) { s.OwnerReferences[0].Name = "foreign" }},
						{"owner-kind", func(s *corev1.Service) { s.OwnerReferences[0].Kind = "Deployment" }},
						{"ownerless", func(s *corev1.Service) { s.OwnerReferences = nil }},
						{"managed-by", func(s *corev1.Service) { s.Labels["app.kubernetes.io/managed-by"] = "foreign" }},
						{"purpose", func(s *corev1.Service) { s.Labels[labelPlatformAliasPurpose] = "foreign" }},
						{"selector", func(s *corev1.Service) { s.Spec.Selector = map[string]string{"app": "foreign"} }},
						{"type", func(s *corev1.Service) { s.Spec.Type = corev1.ServiceTypeClusterIP; s.Spec.ExternalName = "" }},
					} {
						bad := alias.DeepCopy()
						mutation.change(bad)
						err := operator.Update(ctx, bad, client.DryRunAll)
						Expect(err).To(HaveOccurred(), mutation.name)
						Expect(err.Error()).To(ContainSubstring("bex-operator-platform-aliases"), mutation.name)
						if mutation.name != "type" {
							bad.ResourceVersion = ""
							bad.UID = ""
							err = operator.Create(ctx, bad, client.DryRunAll)
							Expect(err).To(HaveOccurred(), mutation.name)
							Expect(err.Error()).To(ContainSubstring("bex-operator-platform-aliases"), mutation.name)
						}
					}
					for _, op := range []string{"create", "update"} {
						bad := alias.DeepCopy()
						if op == "create" {
							bad.ResourceVersion = ""
							bad.UID = ""
							err = other.Create(ctx, bad, client.DryRunAll)
						} else {
							err = other.Update(ctx, bad, client.DryRunAll)
						}
						Expect(apierrors.IsForbidden(err)).To(BeTrue(), op)
						Expect(err.Error()).To(ContainSubstring("only the bex operator"))
					}
					// Keep malformed names DNS-valid so these failures prove admission.
					for _, badName := range []string{"x" + name[1:], name[:len(name)-1] + "g", name[:len(name)-1]} {
						bad := alias.DeepCopy()
						bad.Name = badName
						bad.ResourceVersion = ""
						bad.UID = ""
						err := operator.Create(ctx, bad, client.DryRunAll)
						Expect(err).To(HaveOccurred(), badName)
						Expect(err.Error()).To(ContainSubstring("bex-operator-platform-aliases"), badName)
					}
				}
			}
		}
	})
})
