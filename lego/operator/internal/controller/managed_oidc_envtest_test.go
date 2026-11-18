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
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	oidcTrigger  = "AWS_ROLE_ARN"
	oidcFileEnv  = "AWS_WEB_IDENTITY_TOKEN_FILE"
	oidcFilePath = "/var/lib/render/oidc/aws-token"
	oidcRoleARN  = "arn:aws:iam::123456789012:role/bex-qa"
)

func TestWithManagedOIDCReplacesTheServicesOwnValueAndKeepsPortLast(t *testing.T) {
	aws, _ := appv1alpha1.ManagedOIDCProviderNamed("aws")
	env := withManagedOIDC([]corev1.EnvVar{
		{Name: "A", Value: "1"}, {Name: oidcFileEnv, Value: "/tmp/mine"}, {Name: portEnvName, Value: "10000"},
	}, []appv1alpha1.ManagedOIDCProvider{aws})
	want := []corev1.EnvVar{{Name: "A", Value: "1"}, {Name: oidcFileEnv, Value: oidcFilePath}, {Name: portEnvName, Value: "10000"}}
	if !equality.Semantic.DeepEqual(env, want) {
		t.Fatalf("env = %+v\nwant %+v", env, want)
	}
	mine := []corev1.EnvVar{{Name: oidcFileEnv, Value: "/tmp/mine"}, {Name: portEnvName, Value: "10000"}}
	if got := withManagedOIDC(mine, nil); !equality.Semantic.DeepEqual(got, mine) {
		t.Fatalf("with no mount, the service's own value must stay: %+v", got)
	}
}

// TestTemplateManagedOIDCReadsWhatTheProjectionWrote: a restored template's
// mounts are recognized from the variable the projection sets, so a cancel
// that restores a release keeps its tokens minted.
func TestTemplateManagedOIDCReadsWhatTheProjectionWrote(t *testing.T) {
	aws, _ := appv1alpha1.ManagedOIDCProviderNamed("aws")
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web"}}
	var spec corev1.PodSpec
	spec.Containers = []corev1.Container{{Name: appContainerName, Env: []corev1.EnvVar{{Name: portEnvName, Value: "10000"}}}}
	withManagedOIDCPod(&spec, app, []appv1alpha1.ManagedOIDCProvider{aws})
	if got := templateManagedOIDC(corev1.PodTemplateSpec{Spec: spec}); len(got) != 1 || got[0] != "aws" {
		t.Fatalf("providers read back = %v, want [aws]", got)
	}
	user := corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Env: []corev1.EnvVar{{Name: oidcFileEnv, Value: "/tmp/mine"}}}}}}
	if got := templateManagedOIDC(user); len(got) != 0 {
		t.Fatalf("a service's own %s is no mount: %v", oidcFileEnv, got)
	}
}

// w2/m477, docs/ADR116-managed-oidc.md D9: a release whose environment sets
// AWS_ROLE_ARN, in a Pro-or-higher workspace, reads its token from a
// read-only file named by AWS_WEB_IDENTITY_TOKEN_FILE in every container bex
// runs for it. Below Pro nothing is mounted and the release records why; a
// release without the variable has the pod template it always had.
var _ = Describe("Managed OIDC token mounts (w2/m477)", func() {
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	pass := func(nn types.NamespacedName) {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	}
	passN := func(nn types.NamespacedName) {
		GinkgoHelper()
		for range 3 {
			pass(nn)
		}
	}
	stampRelease := func(app *appv1alpha1.App) {
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(app.Generation, 10))
	}
	update := func(nn types.NamespacedName, mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := envtestApp(nn)
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	create := func(name string, pro bool, mutate func(*appv1alpha1.App)) types.NamespacedName {
		GinkgoHelper()
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Replicas: 1},
		}
		if pro {
			app.Labels = map[string]string{appv1alpha1.LabelManagedOIDC: appv1alpha1.ManagedOIDCEnabled}
		}
		if mutate != nil {
			mutate(app)
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		DeferCleanup(func() {
			if got := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, got) == nil {
				Expect(k8sClient.Delete(ctx, got)).To(Succeed())
				passN(nn)
			}
		})
		return nn
	}
	envSecret := func(name string, data map[string]string) {
		GinkgoHelper()
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, StringData: data}
		Expect(k8sClient.Create(ctx, s)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, s) })
	}
	// appContainerOf is the container running the release's image: "app",
	// or a step Job's single container.
	appContainerOf := func(spec corev1.PodSpec) corev1.Container {
		GinkgoHelper()
		for _, c := range spec.Containers {
			if c.Name == appContainerName || len(spec.Containers) == 1 {
				return c
			}
		}
		Fail("no app container")
		return corev1.Container{}
	}
	// expectMounted checks the token directory and its variable on spec.
	expectMounted := func(spec corev1.PodSpec, app string) {
		GinkgoHelper()
		var vol *corev1.Volume
		for i := range spec.Volumes {
			if spec.Volumes[i].Name == managedOIDCVolumeName {
				vol = &spec.Volumes[i]
			}
		}
		Expect(vol).NotTo(BeNil(), "the token volume")
		Expect(vol.Secret).NotTo(BeNil())
		Expect(vol.Secret.SecretName).To(Equal(app + "-oidc"))
		Expect(vol.Secret.Optional).To(BeNil(), "required: the container waits for its token")
		Expect(vol.Secret.Items).To(Equal([]corev1.KeyToPath{{Key: "aws-token", Path: "aws-token"}}))
		c := appContainerOf(spec)
		Expect(c.VolumeMounts).To(ContainElement(corev1.VolumeMount{Name: managedOIDCVolumeName, MountPath: "/var/lib/render/oidc", ReadOnly: true}))
		Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: oidcFileEnv, Value: oidcFilePath}))
		Expect(c.Env[len(c.Env)-1].Name).To(Equal(portEnvName), "PORT stays last")
	}
	expectNotMounted := func(spec corev1.PodSpec) {
		GinkgoHelper()
		for _, v := range spec.Volumes {
			Expect(v.Name).NotTo(Equal(managedOIDCVolumeName))
		}
		for _, e := range appContainerOf(spec).Env {
			Expect(e.Name).NotTo(Equal(oidcFileEnv))
		}
	}

	It("mounts the token in a Pro web service that sets AWS_ROLE_ARN in its own env, a linked group or a literal", func() {
		envSecret("oidc-own-env", map[string]string{oidcTrigger: oidcRoleARN})
		envSecret("evg-oidc-group-env", map[string]string{oidcTrigger: oidcRoleARN})
		for name, mutate := range map[string]func(*appv1alpha1.App){
			"oidc-own":   func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "oidc-own-env" },
			"oidc-group": func(a *appv1alpha1.App) { a.Spec.EnvFromSecrets = []string{"evg-oidc-group-env"} },
			"oidc-literal": func(a *appv1alpha1.App) {
				// The service's own token-file value gives way, as on Render.
				a.Spec.Env = []appv1alpha1.EnvVar{{Name: oidcTrigger, Value: oidcRoleARN}, {Name: oidcFileEnv, Value: "/tmp/mine"}}
			},
		} {
			nn := create(name, true, mutate)
			passN(nn)
			expectMounted(envtestDeployment(nn).Spec.Template.Spec, name)
			st := envtestApp(nn).Status.ManagedOIDC
			Expect(st).NotTo(BeNil(), name)
			Expect(st.Providers).To(Equal([]string{"aws"}), name)
			Expect(st.PlanAllowed).To(BeTrue(), name)
			Expect(st.Generation).To(Equal(envtestApp(nn).Status.ReleaseGeneration), name)
		}
	})

	It("keeps AWS_ROLE_ARN but mounts nothing below Pro, and records the refusal for the deploy log", func() {
		nn := create("oidc-hobby", false, func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: oidcTrigger, Value: oidcRoleARN}}
		})
		passN(nn)
		spec := envtestDeployment(nn).Spec.Template.Spec
		expectNotMounted(spec)
		Expect(appContainerOf(spec).Env).To(ContainElement(corev1.EnvVar{Name: oidcTrigger, Value: oidcRoleARN}))
		st := envtestApp(nn).Status.ManagedOIDC
		Expect(st).NotTo(BeNil())
		Expect(st.Refused()).To(Equal([]string{"aws"}))
		Expect(st.Mounted()).To(BeEmpty())
	})

	It("leaves a service without the variable with the pod template it had without Managed OIDC", func() {
		// A blank value is no trigger either.
		withLabel := create("oidc-none-pro", true, func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: "GREETING", Value: "hi"}, {Name: oidcTrigger, Value: " "}}
		})
		without := create("oidc-none", false, func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: "GREETING", Value: "hi"}, {Name: oidcTrigger, Value: " "}}
		})
		passN(withLabel)
		passN(without)
		st := envtestApp(withLabel).Status.ManagedOIDC
		Expect(st).NotTo(BeNil(), "every release records its decision")
		Expect(st.Providers).To(BeEmpty())
		a, b := envtestDeployment(withLabel).Spec.Template.Spec, envtestDeployment(without).Spec.Template.Spec
		expectNotMounted(a)
		// Only the App's own name differs between the two templates.
		rename := func(spec corev1.PodSpec, from, to string) corev1.PodSpec {
			out := spec.DeepCopy()
			for i, e := range out.Containers[0].Env {
				out.Containers[0].Env[i].Value = strings.ReplaceAll(e.Value, from, to)
			}
			return *out
		}
		Expect(equality.Semantic.DeepEqual(rename(a, "oidc-none-pro", "oidc-none"), b)).To(BeTrue(),
			"the labeled App's pod spec differs from the unlabeled one:\n%+v\n%+v", a, b)
	})

	It("keeps a release's plan decision until the next deploy", func() {
		nn := create("oidc-frozen", true, func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: oidcTrigger, Value: oidcRoleARN}}
		})
		passN(nn)
		serveEnvtestRelease(nn, func() { passN(nn) })
		first := envtestApp(nn).Status.ReleaseGeneration
		// A downgrade reaches the label; the running release keeps its mount.
		update(nn, func(a *appv1alpha1.App) { delete(a.Labels, appv1alpha1.LabelManagedOIDC) })
		passN(nn)
		expectMounted(envtestDeployment(nn).Spec.Template.Spec, nn.Name)
		Expect(envtestApp(nn).Status.ManagedOIDC.PlanAllowed).To(BeTrue())
		// The next deploy decides afresh, and drops the mount.
		update(nn, func(a *appv1alpha1.App) {
			a.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			stampRelease(a)
		})
		passN(nn)
		app := envtestApp(nn)
		Expect(app.Status.ReleaseGeneration).To(BeNumerically(">", first))
		Expect(app.Status.ManagedOIDC.Refused()).To(Equal([]string{"aws"}))
		expectNotMounted(envtestDeployment(nn).Spec.Template.Spec)
	})

	It("leaves a release that served before Managed OIDC alone until the next deploy", func() {
		nn := create("oidc-upgrade", false, func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: oidcTrigger, Value: oidcRoleARN}}
		})
		passN(nn)
		serveEnvtestRelease(nn, func() { passN(nn) })
		// An operator without Managed OIDC rolled this release out: no
		// decision is recorded. The workspace is (or becomes) Pro.
		app := envtestApp(nn)
		app.Status.ManagedOIDC = nil
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		update(nn, func(a *appv1alpha1.App) {
			a.Labels = map[string]string{appv1alpha1.LabelManagedOIDC: appv1alpha1.ManagedOIDCEnabled}
		})
		before := envtestDeployment(nn).Spec.Template.DeepCopy()
		passN(nn)
		Expect(equality.Semantic.DeepEqual(*before, envtestDeployment(nn).Spec.Template)).To(BeTrue(), "the serving release must not roll")
		Expect(envtestApp(nn).Status.ManagedOIDC.Mounted()).To(BeEmpty())
		// Its next deploy mounts the token.
		update(nn, func(a *appv1alpha1.App) {
			a.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			stampRelease(a)
		})
		passN(nn)
		Expect(envtestApp(nn).Status.ManagedOIDC.Mounted()).To(Equal([]string{"aws"}))
		expectMounted(envtestDeployment(nn).Spec.Template.Spec, nn.Name)
	})

	It("mounts the token in every cron run", func() {
		nn := create("oidc-cron", true, func(a *appv1alpha1.App) {
			a.Spec.Type, a.Spec.Schedule, a.Spec.Command = appv1alpha1.TypeCronJob, "* * * * *", "aws sts get-caller-identity"
			a.Spec.Port = 0
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: oidcTrigger, Value: oidcRoleARN}}
		})
		passN(nn)
		cron := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: appv1alpha1.CronJobName(nn.Name), Namespace: nn.Namespace}, cron)).To(Succeed())
		expectMounted(cron.Spec.JobTemplate.Spec.Template.Spec, nn.Name)
		// The template compares equal to itself on the next pass: nothing churns.
		before := cron.Spec.JobTemplate.Spec.Template.DeepCopy()
		passN(nn)
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: appv1alpha1.CronJobName(nn.Name), Namespace: nn.Namespace}, cron)).To(Succeed())
		Expect(equality.Semantic.DeepEqual(*before, cron.Spec.JobTemplate.Spec.Template)).To(BeTrue())
	})

	It("mounts the token in the pre-deploy step", func() {
		nn := create("oidc-predeploy", true, func(a *appv1alpha1.App) {
			a.Spec.PreDeployCommand = "migrate"
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: oidcTrigger, Value: oidcRoleARN}}
		})
		pass(nn) // the finalizer
		update(nn, stampRelease)
		release := envtestApp(nn).Generation
		pass(nn)
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: predeploy.JobName(nn.Name, appv1alpha1.BuildRevision(release)), Namespace: nn.Namespace,
		}, job)).To(Succeed(), "the release's pre-deploy step started")
		expectMounted(job.Spec.Template.Spec, nn.Name)
		// The decision is on the App before the step's pod can start, so
		// bex-api's minter is woken while the pod waits for its token.
		Expect(envtestApp(nn).Status.ManagedOIDC.Mounted()).To(Equal([]string{"aws"}))
	})
})
