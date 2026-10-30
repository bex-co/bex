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
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Persistent disk snapshot finalization", func() {
	It("purges two detached disk lifecycles on the same App UID without reusing completion", func() {
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "disk-two-purge-cycles", Namespace: "default"},
			Spec: appv1alpha1.AppSpec{Image: "nginx:1", Tier: "starter",
				Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 1}}}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) })
		originalUID := app.UID
		credential := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "disk-two-cycles-s3", Namespace: app.Namespace},
			Data: map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("test"), "AWS_SECRET_ACCESS_KEY": []byte("test-secret")}}
		Expect(k8sClient.Create(ctx, credential)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, credential)).To(Succeed()) })
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), BackupHelperImage: "test-operator",
			DiskSnapshots: DiskSnapshotStore{Endpoint: "https://s3.example.invalid", Bucket: "snapshots", S3Secret: credential.Name, AgePublicKey: "test-recipient"}}
		var previousPurgeUID types.UID
		for cycle := range 2 {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(app), app)).To(Succeed())
			app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 1}
			if cycle == 1 {
				// Replay a stale completion after a clean detach. Starting the new
				// writer lifecycle must invalidate it before scheduling anything.
				metav1.SetMetaDataAnnotation(&app.ObjectMeta, annotDiskPurgeComplete, string(originalUID))
			}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			Expect(r.reconcileDisk(ctx, app)).To(Succeed())
			Expect(r.reconcileDiskBackup(ctx, app)).To(Succeed())
			Expect(app.Annotations).NotTo(HaveKey(annotDiskPurgeComplete))
			app.Spec.Disk = nil
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			Expect(r.cleanupDisk(ctx, app)).To(Succeed())
			cron := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: diskBackupName(app.Name)}, cron)).To(Succeed())
			Expect(cron.DeletionTimestamp.IsZero()).To(BeFalse())
			cron.Finalizers = nil // envtest has no garbage collector
			Expect(k8sClient.Update(ctx, cron)).To(Succeed())
			Expect(r.cleanupDisk(ctx, app)).To(Succeed())
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: diskPurgeName(app)}, job)).To(Succeed())
			Expect(job.UID).NotTo(Equal(previousPurgeUID))
			Expect(job.Status.Conditions).To(BeEmpty())
			previousPurgeUID = job.UID
			pvc := &corev1.PersistentVolumeClaim{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: diskPVCName(app.Name)}, pvc)).To(Succeed())
			Expect(pvc.DeletionTimestamp.IsZero()).To(BeTrue(), "detach must wait for purge before deleting the volume")

			app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 1}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			_, held, err := r.reconcileDiskLifecycle(ctx, app, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(held).To(BeTrue(), "reattachment must not start writers while the earlier purge runs")
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(cron), cron))).To(BeTrue())
			app.Spec.Disk = nil
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			completeEnvtestJob(job, metav1.Now())
			for range 3 {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(app), app)).To(Succeed())
				Expect(r.cleanupDisk(ctx, app)).To(Succeed())
			}
			Expect(app.UID).To(Equal(originalUID))
			for _, annotation := range []string{annotDiskProvisioned, annotDiskSnapshots, annotDiskPurgePending, annotDiskPurgeComplete} {
				Expect(app.Annotations).NotTo(HaveKey(annotation))
			}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)).To(Succeed())
			Expect(pvc.DeletionTimestamp.IsZero()).To(BeFalse())
			pvc.Finalizers = nil // simulate the PVC protection controller
			Expect(k8sClient.Update(ctx, pvc)).To(Succeed())
		}
	})

	It("stops legacy writers and holds deletion until purge completion and Pod absence", func() {
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
			Name: "disk-finalizer-barrier", Namespace: "default", Finalizers: []string{finalizer},
			Annotations: map[string]string{annotDiskProvisioned: diskProvisionedMarker},
		}, Spec: appv1alpha1.AppSpec{Image: "nginx:1", Tier: "starter",
			Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 1}}}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		credential := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "disk-finalizer-s3", Namespace: app.Namespace},
			Data: map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("test"), "AWS_SECRET_ACCESS_KEY": []byte("test-secret")}}
		Expect(k8sClient.Create(ctx, credential)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, credential)).To(Succeed()) })
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			BackupHelperImage: "test-operator", DiskSnapshots: DiskSnapshotStore{
				Endpoint: "https://s3.example.invalid", Bucket: "snapshots", S3Secret: credential.Name, AgePublicKey: "test-recipient",
			}}
		cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: diskBackupName(app.Name), Namespace: app.Namespace,
			Labels: diskBackupLabels(app)}, Spec: r.diskBackupCronJobSpec(app)}
		Expect(controllerutil.SetControllerReference(app, cron, r.Scheme)).To(Succeed())
		Expect(k8sClient.Create(ctx, cron)).To(Succeed())
		legacyLabels := diskBackupLabels(app)
		delete(legacyLabels, execution.LabelAppUID)
		writer := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "disk-finalizer-writer", Namespace: app.Namespace, Labels: legacyLabels},
			Spec: r.diskSnapshotJobSpec(app, legacyLabels, "backup", nil)}
		Expect(controllerutil.SetControllerReference(cron, writer, r.Scheme)).To(Succeed())
		Expect(k8sClient.Create(ctx, writer)).To(Succeed())
		writerPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "disk-finalizer-writer-pod", Namespace: app.Namespace,
			Labels: legacyLabels, Finalizers: []string{"test.bex.co/hold"}}, Spec: writer.Spec.Template.Spec}
		Expect(controllerutil.SetControllerReference(writer, writerPod, r.Scheme)).To(Succeed())
		Expect(k8sClient.Create(ctx, writerPod)).To(Succeed())
		Expect(k8sClient.Delete(ctx, app)).To(Succeed())
		request := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(app)}
		step := func() {
			_, err := r.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
		}
		assertHeld := func() {
			Expect(k8sClient.Get(ctx, request.NamespacedName, app)).To(Succeed())
			Expect(app.Finalizers).To(ContainElement(finalizer))
		}
		step()
		assertHeld()
		purgeKey := client.ObjectKey{Namespace: app.Namespace, Name: diskPurgeName(app)}
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, purgeKey, &batchv1.Job{}))).To(BeTrue())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(writerPod), writerPod)).To(Succeed())
		Expect(writerPod.DeletionTimestamp.IsZero()).To(BeFalse())
		step()
		assertHeld()
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, purgeKey, &batchv1.Job{}))).To(BeTrue())

		// Envtest has no garbage collector. Release its foreground-deletion
		// barriers only after the writer Pod is observed gone, as GC would.
		writerPod.Finalizers = nil
		Expect(k8sClient.Update(ctx, writerPod)).To(Succeed())
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(writerPod), writerPod))).To(BeTrue())
		for _, obj := range []client.Object{writer, cron} {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			obj.SetFinalizers(nil)
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())
		}
		step()
		assertHeld()
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, purgeKey, job)).To(Succeed())
		Expect(job.Labels[execution.LabelAppUID]).To(Equal(string(app.UID)))
		Expect(job.Spec.Template.Spec.Volumes).To(BeEmpty())
		failEnvtestJob(job, "StoreUnavailable", "S3 unavailable")
		_, err := r.Reconcile(ctx, request)
		Expect(err).To(HaveOccurred())
		assertHeld()
		step()
		Expect(k8sClient.Get(ctx, purgeKey, job)).To(Succeed())
		completeEnvtestJob(job, metav1.Now())
		purgePod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "disk-finalizer-purge-pod", Namespace: app.Namespace,
			Labels: job.Spec.Template.Labels, Finalizers: []string{"test.bex.co/hold"}}, Spec: job.Spec.Template.Spec}
		Expect(controllerutil.SetControllerReference(job, purgePod, r.Scheme)).To(Succeed())
		Expect(k8sClient.Create(ctx, purgePod)).To(Succeed())
		for range 4 {
			step()
			assertHeld()
		}
		Expect(app.Annotations[annotDiskPurgeComplete]).To(Equal(string(app.UID)))
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(purgePod), purgePod)).To(Succeed())
		Expect(purgePod.DeletionTimestamp.IsZero()).To(BeFalse())
		purgePod.Finalizers = nil
		Expect(k8sClient.Update(ctx, purgePod)).To(Succeed())
		step()
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, request.NamespacedName, app))).To(BeTrue())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(credential), credential)).To(Succeed())
	})
})
