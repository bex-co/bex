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
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m63: the one-time initialDeployHook used to be injected as the
// preDeployCommand, so a later manual deploy without a Blueprint re-sync ran
// it again. The hook now has its own role: it runs until a release completes
// it, and every later release runs the regular command.
func TestPreDeployRunsTheInitialHookOnlyUntilItCompletes(t *testing.T) {
	const ns = "default"
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	newApp := func(gen int64, hook, regular string, completed bool, prior *appv1alpha1.PreDeployStatus) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: ns, Generation: gen, UID: "app-uid"},
			Spec:       appv1alpha1.AppSpec{Image: "registry.example/app:1", Port: 3000, PreDeployCommand: regular, InitialDeployHook: hook},
			Status:     appv1alpha1.AppStatus{ReleaseGeneration: gen, PreDeploy: prior, InitialDeployHookCompleted: completed},
		}
	}
	reconcile := func(t *testing.T, app *appv1alpha1.App, objs ...client.Object) (client.Client, *appv1alpha1.App) {
		t.Helper()
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(append(objs, app)...).WithStatusSubresource(&appv1alpha1.App{}).Build()
		r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: scheme, Mode: ModeKubernetes}
		if _, _, err := r.reconcilePreDeploy(context.Background(), app, "registry.example/app:1", 3000); err != nil {
			t.Fatalf("reconcilePreDeploy: %v", err)
		}
		return cl, app
	}
	jobCommand := func(t *testing.T, cl client.Client, gen int64) (string, bool) {
		t.Helper()
		var job batchv1.Job
		err := cl.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: predeploy.JobName("web", appv1alpha1.BuildRevision(gen))}, &job)
		if err != nil {
			return "", false
		}
		return job.Spec.Template.Spec.Containers[0].Command[2], true
	}
	succeeded := func(gen int64) *appv1alpha1.PreDeployStatus {
		return &appv1alpha1.PreDeployStatus{Generation: gen, Status: appv1alpha1.PreDeploySucceeded}
	}

	t.Run("first release runs the hook", func(t *testing.T) {
		cl, _ := reconcile(t, newApp(1, "echo initial", "echo regular", false, nil))
		if got, ok := jobCommand(t, cl, 1); !ok || got != "echo initial" {
			t.Fatalf("gen-1 Job command = %q (exists %v), want the hook", got, ok)
		}
	})
	t.Run("hook success completes it", func(t *testing.T) {
		done := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: predeploy.JobName("web", appv1alpha1.BuildRevision(1)), Namespace: ns,
				Labels: map[string]string{predeploy.LabelService: "web", execution.LabelApp: "web", execution.LabelAppUID: "app-uid", predeploy.LabelComponent: predeploy.ComponentValue}},
			Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "pre-deploy", Command: []string{"/bin/sh", "-c", "echo initial"}}}}}},
			Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			}},
		}
		app := newApp(1, "echo initial", "echo regular", false, &appv1alpha1.PreDeployStatus{Job: done.Name, Generation: 1, Status: appv1alpha1.PreDeployRunning})
		cl, _ := reconcile(t, app, done)
		var stored appv1alpha1.App
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: "web"}, &stored); err != nil {
			t.Fatal(err)
		}
		if !stored.Status.InitialDeployHookCompleted || stored.Status.PreDeploy == nil || stored.Status.PreDeploy.Status != appv1alpha1.PreDeploySucceeded {
			t.Fatalf("status after hook success = completed %v, preDeploy %+v; want the hook completed", stored.Status.InitialDeployHookCompleted, stored.Status.PreDeploy)
		}
	})
	t.Run("later release runs the regular command", func(t *testing.T) {
		cl, _ := reconcile(t, newApp(2, "echo initial", "echo regular", true, succeeded(1)))
		if got, ok := jobCommand(t, cl, 2); !ok || got != "echo regular" {
			t.Fatalf("gen-2 Job command = %q (exists %v), want the regular command", got, ok)
		}
	})
	t.Run("identical text keeps the regular role", func(t *testing.T) {
		cl, _ := reconcile(t, newApp(2, "echo same", "echo same", true, succeeded(1)))
		if got, ok := jobCommand(t, cl, 2); !ok || got != "echo same" {
			t.Fatalf("gen-2 Job command = %q (exists %v), want the regular command to run", got, ok)
		}
	})
	t.Run("empty regular command runs nothing", func(t *testing.T) {
		cl, _ := reconcile(t, newApp(2, "echo initial", "", true, succeeded(1)))
		if got, ok := jobCommand(t, cl, 2); ok {
			t.Fatalf("gen-2 Job ran %q, want no pre-deploy step", got)
		}
	})
	t.Run("a failed hook retries on the next release", func(t *testing.T) {
		failed := &appv1alpha1.PreDeployStatus{Generation: 1, Status: appv1alpha1.PreDeployFailed}
		cl, _ := reconcile(t, newApp(2, "echo initial", "echo regular", false, failed))
		if got, ok := jobCommand(t, cl, 2); !ok || got != "echo initial" {
			t.Fatalf("gen-2 Job command = %q (exists %v), want the uncompleted hook again", got, ok)
		}
	})
}
