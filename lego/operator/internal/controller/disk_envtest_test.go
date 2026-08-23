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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The CRD's own rules are the first and only line of defence for the disk
// trade-offs: bex-api mirrors them as 400s, but a direct CR write (a Blueprint
// apply, kubectl, a future controller) reaches the API server without passing
// through bex-api at all.
var _ = Describe("Persistent disk admission", func() {
	newDiskApp := func(name string) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Image: "nginx:1",
				Tier:  "starter",
				Disk:  &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10},
			},
		}
	}

	It("accepts a disk on an eligible paid single-instance service and defaults its size", func() {
		app := newDiskApp("disk-ok")
		app.Spec.Disk.SizeGB = 0 // omitted by the client
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()

		stored := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(app), stored)).To(Succeed())
		Expect(stored.Spec.Disk.SizeGB).To(BeEquivalentTo(10), "Render's Blueprint default is 10GB")
	})

	DescribeTable("refuses a disk the platform cannot honestly serve",
		func(mutate func(*appv1alpha1.App), wantMessage string) {
			app := newDiskApp("disk-reject")
			mutate(app)
			err := k8sClient.Create(ctx, app)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(wantMessage))
		},
		Entry("on a cron job", func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeCronJob
			a.Spec.Schedule = "0 * * * *"
		}, "spec.disk is supported only on"),
		Entry("on a static site", func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeStaticSite
		}, "spec.disk is supported only on"),
		Entry("on the free tier", func(a *appv1alpha1.App) {
			a.Spec.Tier = tierFree
		}, "spec.disk requires a paid tier"),
		Entry("with more than one instance", func(a *appv1alpha1.App) {
			a.Spec.Replicas = 2
		}, "cannot run more than one instance"),
		Entry("with autoscaling enabled", func(a *appv1alpha1.App) {
			a.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{
				Enabled: true, MinReplicas: 1, MaxReplicas: 3, TargetCPUPercent: ptr.To(int32(70)),
			}
		}, "cannot use autoscaling"),
		Entry("mounted at a relative path", func(a *appv1alpha1.App) {
			a.Spec.Disk.MountPath = "data"
		}, "must be an absolute path"),
		Entry("mounted at the root directory", func(a *appv1alpha1.App) {
			a.Spec.Disk.MountPath = "/"
		}, "must not be the root directory"),
		Entry("mounted over the build output", func(a *appv1alpha1.App) {
			a.Spec.Disk.MountPath = "/opt/render/project/src"
		}, "reserved platform path"),
		Entry("mounted over the projected secrets", func(a *appv1alpha1.App) {
			a.Spec.Disk.MountPath = "/etc/secrets"
		}, "reserved platform path"),
		Entry("larger than a Hetzner volume can be", func(a *appv1alpha1.App) {
			a.Spec.Disk.SizeGB = 20000
		}, "should be less than or equal to 10000"),
	)

	It("allows a subdirectory of a reserved path", func() {
		app := newDiskApp("disk-subdir")
		app.Spec.Disk.MountPath = "/opt/render/project/src/uploads"
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		Expect(k8sClient.Delete(ctx, app)).To(Succeed())
	})

	It("lets a disk grow but never shrink", func() {
		app := newDiskApp("disk-resize")
		app.Spec.Disk.SizeGB = 20
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()

		app.Spec.Disk.SizeGB = 50
		Expect(k8sClient.Update(ctx, app)).To(Succeed())

		app.Spec.Disk.SizeGB = 20
		err := k8sClient.Update(ctx, app)
		Expect(err).To(HaveOccurred(), "shrinking would destroy the filesystem under a running service")
		Expect(err.Error()).To(ContainSubstring("can only grow"))
	})
})

var _ = Describe("Persistent disk reconcile", func() {
	var reconciler *AppReconciler

	BeforeEach(func() {
		reconciler = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	})

	newApp := func(name string) *appv1alpha1.App {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Image: "nginx:1", Tier: "starter",
				Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		return app
	}

	getPVC := func(app *appv1alpha1.App) (*corev1.PersistentVolumeClaim, error) {
		pvc := &corev1.PersistentVolumeClaim{}
		key := client.ObjectKey{Namespace: app.Namespace, Name: diskPVCName(app.Name)}
		return pvc, k8sClient.Get(ctx, key, pvc)
	}

	It("provisions a claim and a passphrase the App owns", func() {
		app := newApp("disk-provision")
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()

		Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())

		pvc, err := getPVC(app)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("10Gi"))
		Expect(metav1.GetControllerOf(pvc)).NotTo(BeNil(), "the claim must be garbage-collected with its App")
		Expect(metav1.GetControllerOf(pvc).Name).To(Equal(app.Name))

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: diskLUKSSecretName(app.Name)}, secret)).To(Succeed())
		Expect(secret.Data).To(HaveKey(diskLUKSSecretKey))
		Expect(metav1.GetControllerOf(secret)).NotTo(BeNil())

		Expect(app.Status.Disk).NotTo(BeNil())
		Expect(app.Status.Disk.AllocatedSizeGB).To(BeEquivalentTo(10))
	})

	// Rewriting the passphrase would lock the tenant out of their own volume,
	// so the mint has to be strictly once-only across every later pass.
	It("never rewrites an existing passphrase", func() {
		app := newApp("disk-passphrase")
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()

		Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())
		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: app.Namespace, Name: diskLUKSSecretName(app.Name)}
		Expect(k8sClient.Get(ctx, key, secret)).To(Succeed())
		first := string(secret.Data[diskLUKSSecretKey])

		for range 3 {
			Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())
		}
		Expect(k8sClient.Get(ctx, key, secret)).To(Succeed())
		Expect(string(secret.Data[diskLUKSSecretKey])).To(Equal(first))
	})

	It("keeps the volume while the App is suspended", func() {
		app := newApp("disk-suspend")
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()
		Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())

		app.Spec.Suspended = true
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())

		_, err := getPVC(app)
		Expect(err).NotTo(HaveOccurred(), "suspend parks a service; it does not discard its data")
	})

	It("removes the volume and passphrase when the disk is detached", func() {
		app := newApp("disk-detach")
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()
		Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())

		app.Spec.Disk = nil
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())

		// Gone, or at least on its way out: envtest runs the API server's
		// StorageObjectInUseProtection admission plugin, which stamps every PVC
		// with a pvc-protection finalizer, but not the controller-manager that
		// would clear it — so a deleted claim lingers here in Terminating.
		// What the operator controls, and what must hold, is that the delete
		// was issued; otherwise a detached disk keeps billing.
		pvc, err := getPVC(app)
		if err == nil {
			Expect(pvc.DeletionTimestamp).NotTo(BeNil(), "detaching a disk must not orphan a billable volume")
		} else {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
		secret := &corev1.Secret{}
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: diskLUKSSecretName(app.Name)}, secret)
		if err == nil {
			Expect(secret.DeletionTimestamp).NotTo(BeNil())
		} else {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
		Expect(app.Annotations).NotTo(HaveKey(annotDiskProvisioned))
		Expect(app.Status.Disk).To(BeNil())
		Expect(meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionDiskReady)).To(BeNil())
	})

	It("costs a stateless App nothing", func() {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "disk-none", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1"},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()

		Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())
		Expect(app.Status.Disk).To(BeNil())
		Expect(app.Status.Conditions).To(BeEmpty())
	})

	Context("when the class supports expansion", func() {
		const className = "disk-expandable"

		BeforeEach(func() {
			sc := &storagev1.StorageClass{
				ObjectMeta:           metav1.ObjectMeta{Name: className},
				Provisioner:          "example.com/mock",
				AllowVolumeExpansion: ptr.To(true),
			}
			if err := k8sClient.Create(ctx, sc); err != nil {
				Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
			}
			reconciler.DiskStorageClass = className
		})

		bind := func(app *appv1alpha1.App, capacity string) {
			pvc, err := getPVC(app)
			Expect(err).NotTo(HaveOccurred())
			pvc.Status.Phase = corev1.ClaimBound
			pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(capacity)}
			Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())
		}

		It("grows the claim and reports the filesystem catching up", func() {
			app := newApp("disk-grow")
			defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()
			Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())
			bind(app, "10Gi")

			app.Spec.Disk.SizeGB = 25
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())

			pvc, err := getPVC(app)
			Expect(err).NotTo(HaveOccurred())
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("25Gi"))
			Expect(app.Status.Disk.AllocatedSizeGB).To(BeEquivalentTo(25))

			// The volume is 25GB but the filesystem still reports 10GB: the
			// honest state while a grow finishes, not a healthy disk.
			condition := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionDiskReady)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Reason).To(Equal("DiskResizePending"))
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		})

		It("reports a ready disk once the filesystem has caught up", func() {
			app := newApp("disk-ready")
			defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()
			Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())
			bind(app, "10Gi")

			Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())

			condition := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionDiskReady)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(app.Status.Disk.CapacityGB).To(BeEquivalentTo(10))
		})
	})

	It("explains itself instead of failing when the class cannot expand", func() {
		const className = "disk-fixed"
		sc := &storagev1.StorageClass{
			ObjectMeta:           metav1.ObjectMeta{Name: className},
			Provisioner:          "example.com/mock",
			AllowVolumeExpansion: ptr.To(false),
		}
		if err := k8sClient.Create(ctx, sc); err != nil {
			Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
		}
		reconciler.DiskStorageClass = className

		app := newApp("disk-fixed-class")
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()
		Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())

		pvc, err := getPVC(app)
		Expect(err).NotTo(HaveOccurred())
		pvc.Status.Phase = corev1.ClaimBound
		pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")}
		Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())

		app.Spec.Disk.SizeGB = 25
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		// A local-path style class is a configuration limit, not a fault: the
		// running service keeps its volume and the reconcile stays green.
		Expect(reconciler.reconcileDisk(ctx, app)).To(Succeed())

		condition := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionDiskReady)
		Expect(condition).NotTo(BeNil())
		Expect(condition.Reason).To(Equal("StorageClassNotExpandable"))
		pvc, err = getPVC(app)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("10Gi"))
	})
})

var _ = Describe("Persistent disk snapshots", func() {
	store := DiskSnapshotStore{
		Endpoint: "https://s3.example.invalid", Bucket: "bex-disk-snapshots",
		Prefix: "disks", S3Secret: "bex-disk-snapshot-s3",
		AgePublicKey: "age1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqsxfa9d",
		AgeSecret:    "bex-disk-snapshot-age",
	}

	newSnapshotApp := func(name string) (*AppReconciler, *appv1alpha1.App) {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: "default",
				Labels:      map[string]string{"app.bex.co/workspace": "tea-snap"},
				Annotations: map[string]string{annotDiskProvisioned: diskProvisionedMarker},
			},
			Spec: appv1alpha1.AppSpec{
				Image: "nginx:1", Tier: "starter",
				Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			DiskSnapshots: store, BackupHelperImage: "ghcr.io/bex-co/bex:test",
		}, app
	}

	getCronJob := func(app *appv1alpha1.App) (*batchv1.CronJob, error) {
		cron := &batchv1.CronJob{}
		key := client.ObjectKey{Namespace: app.Namespace, Name: diskBackupName(app.Name)}
		return cron, k8sClient.Get(ctx, key, cron)
	}

	It("schedules a nightly encrypted snapshot of the volume", func() {
		reconciler, app := newSnapshotApp("snap-schedule")
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()

		Expect(reconciler.reconcileDiskBackup(ctx, app)).To(Succeed())

		cron, err := getCronJob(app)
		Expect(err).NotTo(HaveOccurred())
		Expect(cron.Spec.Schedule).To(MatchRegexp(`^\d+ 2 \* \* \*$`), "nightly, in the disk window")
		Expect(cron.Spec.ConcurrencyPolicy).To(Equal(batchv1.ForbidConcurrent))
		Expect(metav1.GetControllerOf(cron)).NotTo(BeNil(), "the schedule must die with its App")

		pod := cron.Spec.JobTemplate.Spec.Template.Spec
		Expect(pod.Containers).To(HaveLen(1))
		container := pod.Containers[0]
		Expect(container.Command).To(Equal([]string{"/disk-snapshot"}))
		Expect(container.Args).To(Equal([]string{"backup"}))

		// The volume is mounted READ-ONLY: a backup must not be able to modify
		// the data it is copying.
		Expect(container.VolumeMounts).To(HaveLen(1))
		Expect(container.VolumeMounts[0].ReadOnly).To(BeTrue())
		Expect(pod.Volumes[0].PersistentVolumeClaim.ClaimName).To(Equal(diskPVCName(app.Name)))

		// Encryption is not optional: the recipient must reach the Job, and the
		// DECRYPT half must not.
		env := map[string]string{}
		for _, e := range container.Env {
			env[e.Name] = e.Value
		}
		Expect(env).To(HaveKeyWithValue("AGE_PUBLIC_KEY", store.AgePublicKey))
		Expect(env).NotTo(HaveKey("AGE_PRIVATE_KEY"))
		Expect(env).To(HaveKeyWithValue("BEX_DISK_WORKSPACE", "tea-snap"))
		Expect(env).To(HaveKeyWithValue("BEX_DISK_SNAPSHOT_RETAIN", "7"), "Render keeps at least seven days")

		// The ReadWriteOnce volume can only be co-mounted on the node that has
		// it attached, so the snapshot pod prefers the service's own node.
		Expect(pod.Affinity).NotTo(BeNil())
		Expect(pod.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution).To(HaveLen(1))
	})

	It("takes no snapshot at all when the store is not configured", func() {
		_, app := newSnapshotApp("snap-unconfigured")
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()
		bare := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

		Expect(bare.reconcileDiskBackup(ctx, app)).To(Succeed())

		_, err := getCronJob(app)
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "a half-configured store must not produce a Job")
	})

	// An unencrypted snapshot in a third-party bucket is worse than none, and
	// indistinguishable from a good one once written.
	It("refuses to take snapshots without a recipient key", func() {
		noKey := store
		noKey.AgePublicKey = ""
		Expect(noKey.configured()).To(BeFalse())
		Expect(store.configured()).To(BeTrue())
		// Restore additionally needs the decrypt half.
		noIdentity := store
		noIdentity.AgeSecret = ""
		Expect(noIdentity.restorable()).To(BeFalse())
		Expect(store.restorable()).To(BeTrue())
	})

	It("removes the schedule when the disk is detached", func() {
		reconciler, app := newSnapshotApp("snap-detach")
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()
		Expect(reconciler.reconcileDiskBackup(ctx, app)).To(Succeed())
		Expect(getCronJob(app)).Error().NotTo(HaveOccurred())

		app.Spec.Disk = nil
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		Expect(reconciler.reconcileDiskBackup(ctx, app)).To(Succeed())

		cron, err := getCronJob(app)
		if err == nil {
			Expect(cron.DeletionTimestamp).NotTo(BeNil())
		} else {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
	})

	// A deleted disk's snapshots are a full copy of tenant data in a bucket;
	// leaving them behind keeps billing storage and keeps the data reachable.
	It("purges the snapshots when the disk is deleted", func() {
		reconciler, app := newSnapshotApp("snap-purge")
		defer func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) }()

		Expect(reconciler.purgeDiskSnapshots(ctx, app)).To(Succeed())

		job := &batchv1.Job{}
		key := client.ObjectKey{Namespace: app.Namespace, Name: diskPurgeName(app.Name)}
		Expect(k8sClient.Get(ctx, key, job)).To(Succeed())
		Expect(job.Spec.Template.Spec.Containers[0].Args).To(Equal([]string{"purge"}))
		// Purge touches only the object store, so it must not mount the volume.
		Expect(job.Spec.Template.Spec.Volumes).To(BeEmpty())
	})

	It("gives each disk its own snapshot prefix", func() {
		a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
			Name: "one", Namespace: "default", Labels: map[string]string{"app.bex.co/workspace": "tea-a"}}}
		b := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
			Name: "two", Namespace: "default", Labels: map[string]string{"app.bex.co/workspace": "tea-a"}}}
		Expect(snapshotPrefixFor(a)).NotTo(Equal(snapshotPrefixFor(b)))
		// Falls back to the namespace, which is still a per-tenant scope.
		c := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "one", Namespace: "tea-b"}}
		Expect(snapshotPrefixFor(c)).To(HavePrefix("tea-b/"))
	})
})
