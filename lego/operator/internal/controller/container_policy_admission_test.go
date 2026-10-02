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

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var _ = Describe("Application container policy admission", func() {
	It("admits real workload projections and confines image compatibility on create, update, and delete", func() {
		// Use the complete deployed policy, including its namespace/identity match
		// conditions. A separate API server keeps its default-namespace binding
		// out of sibling specs and avoids admission-cache cleanup races.
		admissionEnv := &envtest.Environment{BinaryAssetsDirectory: getFirstFoundEnvTestBinaryDir()}
		DeferCleanup(admissionEnv.Stop)
		config, err := admissionEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		config.QPS, config.Burst = 100, 200
		admin, err := client.New(config, client.Options{Scheme: k8sClient.Scheme()})
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile("../../../../deploy/gitops/base/operator-workload-admission.yaml")
		Expect(err).NotTo(HaveOccurred())
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			object := &unstructured.Unstructured{}
			if err := decoder.Decode(object); err == io.EOF {
				break
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(admin.Create(ctx, object)).To(Succeed())
		}

		const operatorName = "system:serviceaccount:bex-system:bex-controller-manager"
		role := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "workload-admission-test"}, Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{"apps"}, Resources: []string{"deployments", "statefulsets"}, Verbs: []string{"get", "create", "update", "delete"}},
			{APIGroups: []string{"batch"}, Resources: []string{"jobs", "cronjobs"}, Verbs: []string{"get", "create", "update", "delete"}},
		}}
		Expect(admin.Create(ctx, role)).To(Succeed())
		Expect(admin.Create(ctx, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: role.Name},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: role.Name},
			Subjects:   []rbacv1.Subject{{Kind: "User", APIGroup: "rbac.authorization.k8s.io", Name: operatorName}},
		})).To(Succeed())
		asUser := func(name string) client.Client {
			conf := rest.CopyConfig(config)
			conf.Impersonate.UserName = name
			cl, err := client.New(conf, client.Options{Scheme: admin.Scheme()})
			Expect(err).NotTo(HaveOccurred())
			return cl
		}
		operator := asUser(operatorName)
		// The policy intentionally matches only the operator; an unrelated
		// identity must be refused by RBAC, not a fictitious admission rule.
		untrusted := asUser("system:serviceaccount:bex-system:untrusted")
		const workspace = "tea-00000000000000000000"
		for _, name := range []string{workspace, "workload-outside"} {
			Expect(admin.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
				"app.kubernetes.io/managed-by": "bex-controlplane", "app.kubernetes.io/part-of": "bex",
				"app.bex.co/regime": "hosting", "app.bex.co/workspace": workspace,
			}}})).To(Succeed())
		}

		expectDenied := func(err error, scenario string) {
			Expect(apierrors.IsForbidden(err)).To(BeTrue(), "%s: %v", scenario, err)
			Expect(err.Error()).To(ContainSubstring("bex-operator-workloads"), scenario)
		}
		for _, namespace := range []string{"default", workspace} {
			probe := project(projectionApp(func(a *appv1alpha1.App) { a.Namespace = namespace }), webParams())
			probe.Spec.Template.Spec.AutomountServiceAccountToken = new(true)
			Eventually(func() bool {
				err := operator.Create(ctx, probe.DeepCopy(), client.DryRunAll)
				return apierrors.IsForbidden(err) && strings.Contains(err.Error(), "bex-operator-workloads")
			}, 20*time.Second, 100*time.Millisecond).Should(BeTrue(), "workload binding must become active")

			workloads := make([]client.Object, 0, 23)
			for _, policy := range []string{appv1alpha1.ContainerPolicyImageV1, appv1alpha1.ContainerPolicyStrictV1, ""} {
				workloads = append(workloads, admissionApplicationWorkloads(namespace, policy)...)
			}
			workloads = append(workloads, admissionPlatformWorkloads(namespace)...)
			for _, projected := range workloads {
				workload := admissionFreshWorkload(projected)
				scenario := fmt.Sprintf("%T %s/%s", workload, namespace, workload.GetName())
				By("admitting " + scenario)
				Expect(operator.Create(ctx, workload)).To(Succeed(), scenario)
				workload.SetAnnotations(map[string]string{"admission-test": "valid-update"})
				Expect(operator.Update(ctx, workload)).To(Succeed(), scenario)

				for _, mutation := range []struct {
					name   string
					change func(*corev1.PodTemplateSpec)
				}{
					{"extra-capability", func(p *corev1.PodTemplateSpec) {
						p.Spec.Containers[0].SecurityContext.Capabilities.Add = append(p.Spec.Containers[0].SecurityContext.Capabilities.Add, "SYS_ADMIN")
					}},
					{"privilege-escalation", func(p *corev1.PodTemplateSpec) {
						p.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation = new(true)
					}},
					{"privileged", func(p *corev1.PodTemplateSpec) {
						p.Spec.Containers[0].SecurityContext.Privileged = new(true)
						p.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation = new(true)
					}},
					{"seccomp-override", func(p *corev1.PodTemplateSpec) {
						p.Spec.SecurityContext = &corev1.PodSecurityContext{SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}
						p.Spec.Containers[0].SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
					}},
					{"host-network", func(p *corev1.PodTemplateSpec) { p.Spec.HostNetwork = true; p.Spec.HostUsers = new(true) }},
					{"host-pid", func(p *corev1.PodTemplateSpec) { p.Spec.HostPID = true; p.Spec.HostUsers = new(true) }},
					{"host-ipc", func(p *corev1.PodTemplateSpec) { p.Spec.HostIPC = true; p.Spec.HostUsers = new(true) }},
					{"host-path", func(p *corev1.PodTemplateSpec) {
						p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{Name: "host", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/"}}})
					}},
					{"token-automount", func(p *corev1.PodTemplateSpec) { p.Spec.AutomountServiceAccountToken = new(true) }},
					{"privileged-service-account", func(p *corev1.PodTemplateSpec) { p.Spec.ServiceAccountName = "bex-controller-manager" }},
					{"init-capability", func(p *corev1.PodTemplateSpec) {
						init := corev1.Container{Name: "injected-init", Image: "busybox:latest", SecurityContext: tenantSecCtx()}
						init.SecurityContext.Capabilities.Add = []corev1.Capability{"CHOWN"}
						p.Spec.InitContainers = append(p.Spec.InitContainers, init)
					}},
					{"init-seccomp-override", func(p *corev1.PodTemplateSpec) {
						p.Spec.SecurityContext = &corev1.PodSecurityContext{SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}
						init := corev1.Container{Name: "injected-init", Image: "busybox:latest", SecurityContext: tenantSecCtx()}
						init.SecurityContext.SeccompProfile.Type = corev1.SeccompProfileTypeUnconfined
						p.Spec.InitContainers = append(p.Spec.InitContainers, init)
					}},
				} {
					bad := workload.DeepCopyObject().(client.Object)
					pod := admissionPodTemplate(bad)
					// Some strict helpers inherit pod-level seccomp and do not
					// supply a container SecurityContext until it is mutated.
					if pod.Spec.Containers[0].SecurityContext == nil {
						pod.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{}
					}
					if pod.Spec.Containers[0].SecurityContext.Capabilities == nil {
						pod.Spec.Containers[0].SecurityContext.Capabilities = &corev1.Capabilities{}
					}
					mutation.change(pod)
					label := scenario + "/" + mutation.name
					expectDenied(operator.Create(ctx, admissionFreshWorkload(bad), client.DryRunAll), label+" create")
					if _, job := bad.(*batchv1.Job); job {
						// Job templates are immutable: the API refuses the update
						// before validating admission can inspect the new body.
						Expect(apierrors.IsInvalid(operator.Update(ctx, bad, client.DryRunAll))).To(BeTrue(), label+" immutable Job update")
					} else {
						expectDenied(operator.Update(ctx, bad, client.DryRunAll), label+" update")
					}
				}

				if admissionPodTemplate(workload).Labels["app.bex.co/container-policy"] == appv1alpha1.ContainerPolicyImageV1 {
					for _, mutation := range []struct {
						name         string
						metadataOnly bool
						change       func(client.Object, *corev1.PodTemplateSpec)
					}{
						{"missing-marker", false, func(_ client.Object, p *corev1.PodTemplateSpec) { delete(p.Labels, "app.bex.co/container-policy") }},
						{"forged-marker", false, func(_ client.Object, p *corev1.PodTemplateSpec) { p.Labels["app.bex.co/container-policy"] = "image-v2" }},
						{"missing-app-uid", false, func(_ client.Object, p *corev1.PodTemplateSpec) { delete(p.Labels, "app.bex.co/app-uid") }},
						{"forged-app-uid", false, func(_ client.Object, p *corev1.PodTemplateSpec) { p.Labels["app.bex.co/app-uid"] = "foreign-uid" }},
						{"forged-owner-name", true, func(o client.Object, _ *corev1.PodTemplateSpec) { o.GetOwnerReferences()[0].Name = "foreign-app" }},
						{"forged-owner-uid", true, func(o client.Object, _ *corev1.PodTemplateSpec) { o.GetOwnerReferences()[0].UID = "foreign-uid" }},
						{"missing-owner", true, func(o client.Object, _ *corev1.PodTemplateSpec) { o.SetOwnerReferences(nil) }},
						{"non-app-owner", true, func(o client.Object, _ *corev1.PodTemplateSpec) { o.GetOwnerReferences()[0].Kind = "Database" }},
						{"wrong-owner-api", true, func(o client.Object, _ *corev1.PodTemplateSpec) {
							o.GetOwnerReferences()[0].APIVersion = "foreign.example/v1"
						}},
						{"non-controller-owner", true, func(o client.Object, _ *corev1.PodTemplateSpec) { o.GetOwnerReferences()[0].Controller = new(false) }},
						{"missing-drop-all", false, func(_ client.Object, p *corev1.PodTemplateSpec) {
							p.Spec.Containers[0].SecurityContext.Capabilities.Drop = nil
						}},
						{"missing-no-escalation", false, func(_ client.Object, p *corev1.PodTemplateSpec) {
							p.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation = nil
						}},
						{"init-missing-drop-all", false, func(_ client.Object, p *corev1.PodTemplateSpec) {
							init := corev1.Container{Name: "injected-init", Image: "busybox:latest", SecurityContext: tenantSecCtx()}
							init.SecurityContext.Capabilities.Drop = nil
							p.Spec.InitContainers = append(p.Spec.InitContainers, init)
						}},
						{"init-missing-no-escalation", false, func(_ client.Object, p *corev1.PodTemplateSpec) {
							init := corev1.Container{Name: "injected-init", Image: "busybox:latest", SecurityContext: tenantSecCtx()}
							init.SecurityContext.AllowPrivilegeEscalation = nil
							p.Spec.InitContainers = append(p.Spec.InitContainers, init)
						}},
						{"sidecar-capability", false, func(_ client.Object, p *corev1.PodTemplateSpec) {
							sidecar := p.Spec.Containers[0].DeepCopy()
							sidecar.Name = "injected-sidecar"
							sidecar.Ports = nil
							p.Spec.Containers = append(p.Spec.Containers, *sidecar)
						}},
					} {
						bad := workload.DeepCopyObject().(client.Object)
						mutation.change(bad, admissionPodTemplate(bad))
						label := scenario + "/" + mutation.name
						expectDenied(operator.Create(ctx, admissionFreshWorkload(bad), client.DryRunAll), label+" create")
						_, job := bad.(*batchv1.Job)
						if job && !mutation.metadataOnly {
							Expect(apierrors.IsInvalid(operator.Update(ctx, bad, client.DryRunAll))).To(BeTrue(), label+" immutable Job update")
						} else {
							expectDenied(operator.Update(ctx, bad, client.DryRunAll), label+" update")
						}
					}
					// RuntimeDefault inherited from the pod is valid; an explicit
					// container Unconfined override was refused above.
					inherited := admissionFreshWorkload(workload)
					inherited.SetName(workload.GetName() + "-pod-seccomp")
					pod := &admissionPodTemplate(inherited).Spec
					pod.SecurityContext = &corev1.PodSecurityContext{SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}
					pod.Containers[0].SecurityContext.SeccompProfile = nil
					pod.InitContainers = []corev1.Container{{Name: "strict-init", Image: "busybox:latest", SecurityContext: tenantSecCtx()}}
					Expect(operator.Create(ctx, inherited, client.DryRunAll)).To(Succeed(), scenario+" inherited seccomp")
				}
				if _, stateful := workload.(*appsv1.StatefulSet); stateful {
					// Even a forged, internally consistent App profile cannot
					// widen the StatefulSet resource family.
					forged := workload.DeepCopyObject().(client.Object)
					pod := admissionPodTemplate(forged)
					pod.Labels["app.bex.co/container-policy"] = appv1alpha1.ContainerPolicyImageV1
					pod.Labels["app.bex.co/app"] = "forged-app"
					pod.Labels["app.bex.co/app-uid"] = "forged-uid"
					forged.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "App", Name: "forged-app", UID: "forged-uid", Controller: new(true)}})
					pod.Spec.Containers = pod.Spec.Containers[:1]
					pod.Spec.Containers[0].Name = "app"
					pod.Spec.Containers[0].SecurityContext = appSecCtx(appv1alpha1.AppSpec{ContainerPolicy: appv1alpha1.ContainerPolicyImageV1})
					expectDenied(operator.Create(ctx, admissionFreshWorkload(forged), client.DryRunAll), scenario+" forged StatefulSet profile")
					expectDenied(operator.Update(ctx, forged, client.DryRunAll), scenario+" forged StatefulSet profile update")
				}

				outside := admissionFreshWorkload(workload)
				outside.SetNamespace("workload-outside")
				expectDenied(operator.Create(ctx, outside, client.DryRunAll), scenario+" wrong namespace")
				Expect(admin.Create(ctx, outside)).To(Succeed())
				expectDenied(operator.Update(ctx, outside, client.DryRunAll), scenario+" wrong namespace update")
				expectDenied(operator.Delete(ctx, outside, client.DryRunAll), scenario+" wrong namespace delete")
				Expect(admin.Delete(ctx, outside, client.PropagationPolicy(metav1.DeletePropagationBackground))).To(Succeed())
				for _, forbidden := range []error{
					untrusted.Create(ctx, admissionFreshWorkload(workload), client.DryRunAll),
					untrusted.Update(ctx, workload.DeepCopyObject().(client.Object), client.DryRunAll),
					untrusted.Delete(ctx, workload.DeepCopyObject().(client.Object), client.DryRunAll),
				} {
					Expect(apierrors.IsForbidden(forbidden)).To(BeTrue(), scenario)
					Expect(forbidden.Error()).To(ContainSubstring("untrusted"))
					Expect(forbidden.Error()).NotTo(ContainSubstring("bex-operator-workloads"))
				}
				Expect(operator.Delete(ctx, workload, client.PropagationPolicy(metav1.DeletePropagationBackground))).To(Succeed(), scenario+" cleanup")
			}
		}

		By("allowing deletion of legacy invalid bodies while retaining the namespace boundary")
		for _, workload := range admissionApplicationWorkloads("default", appv1alpha1.ContainerPolicyImageV1) {
			legacy := admissionFreshWorkload(workload)
			delete(admissionPodTemplate(legacy).Labels, "app.bex.co/container-policy")
			Expect(admin.Create(ctx, legacy)).To(Succeed())
			Expect(operator.Delete(ctx, legacy, client.PropagationPolicy(metav1.DeletePropagationBackground))).To(Succeed())
		}
	})
})

// Collect actual producer output through fake storage, then submit those exact
// objects to the real API server. This keeps unrelated release/status machinery
// outside this admission test without replacing any workload constructor.
func admissionApplicationWorkloads(namespace, policy string) []client.Object {
	suffix := policy
	if suffix == "" {
		suffix = "legacy"
	}
	newApp := func(name, kind string) (*AppReconciler, *appv1alpha1.App) {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-" + suffix, Namespace: namespace, UID: types.UID(name + "-" + suffix + "-uid"), Generation: 1},
			Spec:       appv1alpha1.AppSpec{Type: kind, ContainerPolicy: policy, Tier: "free"},
		}
		cl := fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
		return &AppReconciler{Client: cl, BuildClient: cl, Scheme: cl.Scheme()}, app
	}
	var workloads []client.Object
	for _, kind := range []struct{ name, value string }{
		{"web", appv1alpha1.TypeWebService}, {"private", appv1alpha1.TypePrivateService}, {"worker", appv1alpha1.TypeBackgroundWorker},
	} {
		r, app := newApp(kind.name, kind.value)
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		params := webParams()
		params.worker = kind.value == appv1alpha1.TypeBackgroundWorker
		Expect(r.applyServingDeployment(ctx, app, dep, params, nil)).To(Succeed())
		workloads = append(workloads, dep)
	}
	r, app := newApp("cron", appv1alpha1.TypeCronJob)
	app.Spec.Schedule = "*/5 * * * *"
	app.Spec.RunAt = "2026-10-02T12:00:00Z"
	Expect(r.Update(ctx, app)).To(Succeed())
	template := r.cronPodSpec(app, "busybox:latest", 8080, map[string]string{labelApp: app.Name})
	_, err := r.convergeCronRuntime(ctx, app, template)
	Expect(err).NotTo(HaveOccurred())
	cron := &batchv1.CronJob{}
	Expect(r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: appv1alpha1.CronJobName(app.Name)}, cron)).To(Succeed())
	manual := &batchv1.Job{}
	Expect(r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: manualRunJobName(app.Name, app.Spec.RunAt)}, manual)).To(Succeed())
	workloads = append(workloads, cron, manual)
	r, app = newApp("migration", appv1alpha1.TypeWebService)
	app.Spec.PreDeployCommand = "true"
	Expect(r.Update(ctx, app)).To(Succeed())
	_, _, err = r.reconcilePreDeploy(ctx, app, "busybox:latest", 8080)
	Expect(err).NotTo(HaveOccurred())
	var migrations batchv1.JobList
	Expect(r.List(ctx, &migrations)).To(Succeed())
	Expect(migrations.Items).To(HaveLen(1))
	return append(workloads, &migrations.Items[0])
}

func admissionPlatformWorkloads(namespace string) []client.Object {
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "strict-valkey", Namespace: namespace, UID: "strict-valkey-uid"}}
	plan, _ := resolveKVPlan(kv.Spec)
	intent := keyValueIntentFor(kv, plan, 1, "")
	intent.authSecretName = "strict-valkey-auth"
	cl := fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).Build()
	kvReconciler := &KeyValueReconciler{Client: cl, Scheme: cl.Scheme()}
	sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: namespace}}
	Expect(kvReconciler.reconcileKeyValueWorkload(ctx, kv, sts, intent)).To(Succeed())
	db := exportTestDatabase(time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC))
	db.Namespace = namespace
	export := exportJob(db, db.Spec.Exports[0], testStore)
	static := publish.PublishJob(publish.Options{
		AppID: "strict-static", AppUID: "strict-static-uid", Namespace: namespace, AppNamespace: namespace,
		Image: "busybox:latest", PublishPath: "/site", Revision: "rev-1",
		Store: publish.Store{Bucket: "static", Endpoint: "https://s3.example.invalid", Secret: "static-upload"},
	})
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "disk", Namespace: namespace, UID: "disk-uid"}}
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), BackupHelperImage: "ghcr.io/bex-co/bex:test",
		DiskSnapshots: DiskSnapshotStore{Endpoint: "https://s3.example.invalid", Bucket: "disks", S3Secret: "disk-upload", AgePublicKey: "test-key", AgeSecret: "disk-age"}}
	backup := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "dskbak-disk", Namespace: namespace}, Spec: r.diskBackupCronJobSpec(app)}
	Expect(r.createDiskRestoreJob(ctx, app, "disk/snapshot.tar.gz.age")).To(Succeed())
	restore := &batchv1.Job{}
	Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: diskRestoreName(app.Name)}, restore)).To(Succeed())
	return []client.Object{sts, export, static, backup, restore}
}

func admissionPodTemplate(workload client.Object) *corev1.PodTemplateSpec {
	switch object := workload.(type) {
	case *appsv1.Deployment:
		return &object.Spec.Template
	case *appsv1.StatefulSet:
		return &object.Spec.Template
	case *batchv1.Job:
		return &object.Spec.Template
	case *batchv1.CronJob:
		return &object.Spec.JobTemplate.Spec.Template
	default:
		panic(fmt.Sprintf("unexpected workload type %T", workload))
	}
}

func admissionFreshWorkload(workload client.Object) client.Object {
	fresh := workload.DeepCopyObject().(client.Object)
	fresh.SetUID("")
	fresh.SetResourceVersion("")
	fresh.SetCreationTimestamp(metav1.Time{})
	fresh.SetManagedFields(nil)
	// Job UID selector labels are API defaults, not producer output. A new Job
	// receives new ones; retaining a prior server UID would fail API validation
	// before exercising the admission rule the test intends to check.
	if job, ok := fresh.(*batchv1.Job); ok {
		job.Spec.Selector = nil
		for _, key := range []string{"controller-uid", "batch.kubernetes.io/controller-uid", "job-name", "batch.kubernetes.io/job-name"} {
			delete(job.Spec.Template.Labels, key)
		}
	}
	return fresh
}
