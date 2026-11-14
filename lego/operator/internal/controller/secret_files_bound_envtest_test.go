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
	"maps"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fullFilesData is a secret-files map at bex-api's limits: keys maximal-length
// names (253 characters, Kubernetes' longest Secret key) and values whose raw
// bytes total exactly total, spread across them.
func fullFilesData(prefix string, keys, total int) map[string][]byte {
	data := make(map[string][]byte, keys)
	each := total / keys
	for i := range keys {
		size := each
		if i == keys-1 {
			size = total - each*(keys-1)
		}
		name := fmt.Sprintf("%s%03d", prefix, i)
		name += strings.Repeat("k", 253-len(name))
		data[name] = []byte(strings.Repeat("v", size))
	}
	return data
}

// w4/m363 t002: bex-api bounds a service's or group's secret files at
// corev1.MaxSecretSize of contents (maxSecretFilesBytes), with names excluded,
// because that is how the apiserver measures a Secret. This pins that against a
// real kube-apiserver and etcd: one Secret takes exactly that bound under 500
// maximal names, and refuses one byte more with the apiserver's own error.
var _ = Describe("secret files bound (w4/m363)", func() {
	ctx := context.Background()
	const ns = "files-bound"

	BeforeEach(func() {
		_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
	})

	It("takes exactly MaxSecretSize of values under 500 maximal names, and refuses one byte more", func() {
		atBound := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-files", Namespace: ns},
			Type: corev1.SecretTypeOpaque, Data: fullFilesData("f", 500, corev1.MaxSecretSize)}
		Expect(k8sClient.Create(ctx, atBound)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, atBound) })

		stored := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(atBound), stored)).To(Succeed())
		total := 0
		for _, v := range stored.Data {
			total += len(v)
		}
		Expect(total).To(Equal(corev1.MaxSecretSize))
		Expect(stored.Data).To(HaveLen(500))

		over := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-files-over", Namespace: ns},
			Type: corev1.SecretTypeOpaque, Data: fullFilesData("f", 500, corev1.MaxSecretSize+1)}
		err := k8sClient.Create(ctx, over)
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "one byte past MaxSecretSize = %v, want the apiserver's Invalid", err)
		Expect(err.Error()).To(ContainSubstring(strconv.Itoa(corev1.MaxSecretSize)))

		// An update that grows a full Secret by one byte is refused the same way.
		for k := range stored.Data {
			stored.Data[k] = append(stored.Data[k], 'x')
			break
		}
		Expect(apierrors.IsInvalid(k8sClient.Update(ctx, stored))).To(BeTrue())
	})

	It("packs a full service map and a full group map into build Secrets the apiserver takes", func() {
		const buildNS = "files-bound-builds"
		_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: buildNS}})
		own := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "packer-files", Namespace: ns},
			Data: fullFilesData("own", 2, corev1.MaxSecretSize)}
		group := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "evg-packer-files", Namespace: ns},
			Data: fullFilesData("grp", 3, corev1.MaxSecretSize)}
		Expect(k8sClient.Create(ctx, own)).To(Succeed())
		Expect(k8sClient.Create(ctx, group)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, own)
			_ = k8sClient.Delete(ctx, group)
		})
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "packer", Namespace: ns, UID: types.UID("uid-packer")}}
		app.Spec.FilesFromSecrets = []string{group.Name, own.Name}
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

		names, files, err := r.projectNativeBuildFiles(ctx, app, buildNS)
		Expect(err).NotTo(HaveOccurred(), "2 MiB of merged secret files must not overflow one build Secret")
		Expect(names).To(HaveLen(2))
		want := maps.Clone(own.Data)
		maps.Copy(want, group.Data)
		Expect(files).To(HaveLen(len(want)))
		seen := map[string][]byte{}
		for _, name := range names {
			packed := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: name}, packed)).To(Succeed())
			size := 0
			for k, v := range packed.Data {
				size += len(v)
				seen[k] = v
			}
			Expect(size).To(BeNumerically("<=", corev1.MaxSecretSize))
		}
		Expect(seen).To(Equal(want))

		// Unlinking the group packs into one Secret again, and the second goes.
		app.Spec.FilesFromSecrets = []string{own.Name}
		names, _, err = r.projectNativeBuildFiles(ctx, app, buildNS)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(Equal([]string{nativeFilesSecretName(app.Name)}))
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: nativeFilesShardName(app.Name, 1)}, &corev1.Secret{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "a stale packed Secret must be deleted, got %v", err)

		app.Spec.FilesFromSecrets = nil
		names, _, err = r.projectNativeBuildFiles(ctx, app, buildNS)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(BeEmpty())
	})
})
