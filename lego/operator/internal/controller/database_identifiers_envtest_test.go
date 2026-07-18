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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Database physical identifier admission", func() {
	ctx := context.Background()
	created := make([]types.NamespacedName, 0, 2)

	AfterEach(func() {
		for _, nn := range created {
			db := &appv1alpha1.Database{}
			if k8sClient.Get(ctx, nn, db) == nil {
				Expect(k8sClient.Delete(ctx, db)).To(Succeed())
			}
		}
		created = created[:0]
	})

	It("validates create-time names and rejects every post-create identity change", func() {
		invalid := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-invalid-physical", Namespace: "default"},
			Spec: appv1alpha1.DatabaseSpec{
				Name:         "invalid-physical",
				DatabaseName: "Orders-Data",
			},
		}
		Expect(k8sClient.Create(ctx, invalid)).NotTo(Succeed())

		customNN := types.NamespacedName{Name: "dpg-custom-physical", Namespace: "default"}
		custom := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: customNN.Name, Namespace: customNN.Namespace},
			Spec: appv1alpha1.DatabaseSpec{
				Name:         "custom-physical",
				DatabaseName: "orders_data",
				DatabaseUser: "orders_owner",
			},
		}
		Expect(k8sClient.Create(ctx, custom)).To(Succeed())
		created = append(created, customNN)

		custom.Spec.DatabaseName = "renamed_data"
		Expect(k8sClient.Update(ctx, custom)).NotTo(Succeed())
		Expect(k8sClient.Get(ctx, customNN, custom)).To(Succeed())
		custom.Spec.DatabaseUser = "renamed_owner"
		Expect(k8sClient.Update(ctx, custom)).NotTo(Succeed())

		legacyNN := types.NamespacedName{Name: "dpg-default-physical", Namespace: "default"}
		legacy := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: legacyNN.Name, Namespace: legacyNN.Namespace},
			Spec:       appv1alpha1.DatabaseSpec{Name: "default-physical"},
		}
		Expect(k8sClient.Create(ctx, legacy)).To(Succeed())
		created = append(created, legacyNN)
		legacy.Spec.DatabaseName = "late_identity"
		Expect(k8sClient.Update(ctx, legacy)).NotTo(Succeed(),
			"an omitted/defaulted physical name must not become mutable after CNPG initdb")
	})
})
