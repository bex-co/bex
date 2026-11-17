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
	"strconv"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The pre-deploy step and the one-time initialDeployHook are two commands
// (w33/m435). From w8/m63 until then the operator ran the hook as the step
// until a release completed it, so the first release never ran its own
// preDeployCommand. These tests pin the step's side: it runs the release's
// command and nothing else, and a release an older operator left mid-step
// carries over without the hook being lost or run twice.

const (
	stepNamespace = "default"
	stepImage     = "registry.example/app:1"
)

// stepApp is release gen of a service with the hook and the regular command
// given, whose pre-deploy step last recorded prior.
func stepApp(gen int64, hook, regular string, completed bool, prior *appv1alpha1.PreDeployStatus) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: stepNamespace, Generation: gen, UID: "app-uid"},
		Spec:       appv1alpha1.AppSpec{Image: stepImage, Port: 3000, PreDeployCommand: regular, InitialDeployHook: hook},
		Status:     appv1alpha1.AppStatus{ReleaseGeneration: gen, PreDeploy: prior, InitialDeployHookCompleted: completed},
	}
}

// runStep runs the pre-deploy gate once over app and objs, and reports whether
// it held the rollout.
func runStep(t *testing.T, app *appv1alpha1.App, objs ...client.Object) (client.Client, bool, error) {
	t.Helper()
	scheme := reachScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(append(objs, app)...).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: scheme, Mode: ModeKubernetes}
	_, halt, err := r.reconcilePreDeploy(context.Background(), app, stepImage, 3000)
	return cl, halt, err
}

func stepJobScript(t *testing.T, cl client.Client, gen int64) (string, bool) {
	t.Helper()
	job, ok := storedJob(t, cl, client.ObjectKey{Namespace: stepNamespace, Name: predeploy.JobName("web", appv1alpha1.BuildRevision(gen))})
	if !ok {
		return "", false
	}
	return jobScript(job), true
}

func TestPreDeployStepRunsTheReleasesOwnCommandNeverTheHook(t *testing.T) {
	succeeded := func(gen int64) *appv1alpha1.PreDeployStatus {
		return &appv1alpha1.PreDeployStatus{Generation: gen, Status: appv1alpha1.PreDeploySucceeded}
	}
	t.Run("the first release runs its pre-deploy command", func(t *testing.T) {
		cl, halt, err := runStep(t, stepApp(1, "echo initial", "echo regular", false, nil))
		if got, ok := stepJobScript(t, cl, 1); err != nil || !halt || !ok || got != "echo regular" {
			t.Fatalf("gen-1 Job = %q (exists %v, halt %v, err %v), want the regular command gating the first rollout", got, ok, halt, err)
		}
	})
	t.Run("a first release with only a hook has no step", func(t *testing.T) {
		cl, halt, err := runStep(t, stepApp(1, "echo initial", "", false, nil))
		if err != nil || halt || len(jobsIn(t, cl, stepNamespace)) != 0 {
			t.Fatalf("halt %v, err %v, %d Jobs; want the rollout to proceed with no pre-deploy Job", halt, err, len(jobsIn(t, cl, stepNamespace)))
		}
	})
	t.Run("a later release runs the regular command", func(t *testing.T) {
		cl, _, _ := runStep(t, stepApp(2, "echo initial", "echo regular", true, succeeded(1)))
		if got, ok := stepJobScript(t, cl, 2); !ok || got != "echo regular" {
			t.Fatalf("gen-2 Job command = %q (exists %v), want the regular command", got, ok)
		}
	})
	t.Run("text equal to the hook's still runs as the step", func(t *testing.T) {
		cl, _, _ := runStep(t, stepApp(1, "echo same", "echo same", false, nil))
		job, ok := storedJob(t, cl, client.ObjectKey{Namespace: stepNamespace, Name: predeploy.JobName("web", "gen-1")})
		if !ok || jobScript(job) != "echo same" || job.Labels[predeploy.LabelStep] != string(predeploy.StepPreDeploy) {
			t.Fatalf("gen-1 Job = %+v (exists %v), want the pre-deploy step", job, ok)
		}
	})
	t.Run("a pre-deploy success never completes the hook", func(t *testing.T) {
		app := stepApp(1, "echo initial", "echo regular", false, nil)
		cl, _, _ := runStep(t, app)
		endJob(t, cl, client.ObjectKey{Namespace: stepNamespace, Name: predeploy.JobName("web", "gen-1")}, true)
		r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
		if _, halt, err := r.reconcilePreDeploy(context.Background(), app, stepImage, 3000); halt || err != nil {
			t.Fatalf("after the step passed: halt %v, err %v", halt, err)
		}
		if stored := storedApp(t, cl, app); stored.Status.InitialDeployHookCompleted || stored.Status.PreDeploy.Status != appv1alpha1.PreDeploySucceeded {
			t.Fatalf("status = completed %v, preDeploy %+v; want the step passed and the hook still owed", stored.Status.InitialDeployHookCompleted, stored.Status.PreDeploy)
		}
	})
}

// legacyHookJob is the Job an operator from before w33/m435 created for
// release 1: the hook, run as the pre-deploy step, with no step label.
func legacyHookJob(conditions ...batchv1.JobCondition) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: predeploy.JobName("web", "gen-1"), Namespace: stepNamespace,
			Labels: map[string]string{
				predeploy.LabelService: "web", execution.LabelApp: "web", execution.LabelAppUID: "app-uid",
				predeploy.LabelComponent: predeploy.ComponentValue,
			},
		},
		Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "predeploy", Command: []string{"/bin/sh", "-c", "echo initial"}}},
		}}},
		Status: batchv1.JobStatus{Conditions: conditions},
	}
}

var (
	jobSucceeded = []batchv1.JobCondition{
		{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
		{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
	}
	jobFailed = []batchv1.JobCondition{
		{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue},
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
	}
)

// stepRunning is release 1's pre-deploy step, recorded running.
func stepRunning() *appv1alpha1.PreDeployStatus {
	running := runningStep(predeploy.JobName("web", "gen-1"), 1, "")
	return &running
}

// w33/m435 t006, case 3: a release whose hook an older operator is still
// running as its step. The Job is left to finish, no second one starts, and
// its success is the hook's completion.
func TestLegacyHookStepStillRunningCarriesOver(t *testing.T) {
	for _, regular := range []string{"echo regular", ""} {
		t.Run("regular="+strconv.Quote(regular), func(t *testing.T) {
			app := stepApp(1, "echo initial", regular, false, stepRunning())
			cl, halt, err := runStep(t, app, legacyHookJob())
			if err != nil || !halt {
				t.Fatalf("running legacy step: halt %v, err %v; want the rollout held as the older operator held it", halt, err)
			}
			if got, _ := stepJobScript(t, cl, 1); len(jobsIn(t, cl, stepNamespace)) != 1 || got != "echo initial" {
				t.Fatalf("%d Jobs, gen-1 command %q; want the one legacy Job left running, no second run", len(jobsIn(t, cl, stepNamespace)), got)
			}

			done := stepApp(1, "echo initial", regular, false, stepRunning())
			cl, halt, err = runStep(t, done, legacyHookJob(jobSucceeded...))
			if err != nil || halt {
				t.Fatalf("succeeded legacy step: halt %v, err %v; want the rollout to proceed", halt, err)
			}
			stored := storedApp(t, cl, done)
			if !stored.Status.InitialDeployHookCompleted || stored.Status.PreDeploy.Status != appv1alpha1.PreDeploySucceeded {
				t.Fatalf("status = completed %v, preDeploy %+v; want its success counted as the hook's completion", stored.Status.InitialDeployHookCompleted, stored.Status.PreDeploy)
			}
			if stored.OwedInitialDeployHook() != "" {
				t.Fatal("the hook is still owed after its legacy run succeeded: it would run a second time")
			}
		})
	}
}

// A legacy step whose Job was reaped unread gates nothing: the release rolls,
// and the hook, still owed, runs once it is live.
func TestLegacyHookStepReapedUnreadLeavesTheHookOwed(t *testing.T) {
	app := stepApp(1, "echo initial", "", false, stepRunning())
	cl, halt, err := runStep(t, app)
	if err != nil || halt || len(jobsIn(t, cl, stepNamespace)) != 0 {
		t.Fatalf("halt %v, err %v, %d Jobs; want the rollout to proceed without re-running the hook as a step", halt, err, len(jobsIn(t, cl, stepNamespace)))
	}
	if app.OwedInitialDeployHook() == "" || app.Status.PreDeploy != nil {
		t.Fatalf("owed %q, preDeploy %+v; want the hook owed and the stale step record dropped", app.OwedInitialDeployHook(), app.Status.PreDeploy)
	}
}

// w33/m435 t006, case 2: a first release that failed on the hook as its step
// stays failed, and the next release runs under today's rule: its own
// pre-deploy command before the rollout, the hook after it is live.
func TestReleaseFailedOnLegacyHookStepStaysFailedAndTheNextRunsItsOwnCommand(t *testing.T) {
	failed := func() *appv1alpha1.PreDeployStatus {
		return &appv1alpha1.PreDeployStatus{
			Job: predeploy.JobName("web", "gen-1"), Generation: 1, Status: appv1alpha1.PreDeployFailed,
			Message: "the pre-deploy command exited with code 1; check the pre-deploy logs",
		}
	}
	for _, regular := range []string{"echo regular", ""} {
		t.Run("regular="+strconv.Quote(regular), func(t *testing.T) {
			app := stepApp(1, "echo initial", regular, false, failed())
			cl, halt, _ := runStep(t, app, legacyHookJob(jobFailed...))
			if !halt || len(jobsIn(t, cl, stepNamespace)) != 1 {
				t.Fatalf("halt %v, %d Jobs; want the failed release held with no new run", halt, len(jobsIn(t, cl, stepNamespace)))
			}
			if !hasPreDeployStep(app) || preDeployPassed(app) {
				t.Fatal("the planner reads the failed legacy step as passed: an operator upgrade would roll a failed release")
			}

			next := stepApp(2, "echo initial", regular, false, failed())
			cl, halt, err := runStep(t, next)
			got, ok := stepJobScript(t, cl, 2)
			if regular == "" {
				if err != nil || halt || ok {
					t.Fatalf("release 2 with no command: halt %v, err %v, Job %q; want no step", halt, err, got)
				}
			} else if err != nil || !halt || got != "echo regular" {
				t.Fatalf("release 2: halt %v, err %v, Job %q; want its own pre-deploy command, not the hook", halt, err, got)
			}
			if next.OwedInitialDeployHook() == "" {
				t.Fatal("the hook is no longer owed: release 2 would go live without it")
			}
		})
	}
}

// A canceled release left to finish may be one an older operator was running
// the hook for: its success still completes the hook.
func TestCanceledLegacyHookStepStillCompletesTheHook(t *testing.T) {
	for name, tc := range map[string]struct {
		job  *batchv1.Job
		want bool
	}{
		"legacy Job": {legacyHookJob(jobSucceeded...), true},
		"today's pre-deploy Job": {func() *batchv1.Job {
			job := legacyHookJob(jobSucceeded...)
			job.Labels[predeploy.LabelStep] = string(predeploy.StepPreDeploy)
			return job
		}(), false},
	} {
		t.Run(name, func(t *testing.T) {
			app := stepApp(1, "echo initial", "echo regular", false, stepRunning())
			app.Annotations = map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "1"}
			scheme := reachScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, tc.job).WithStatusSubresource(&appv1alpha1.App{}).Build()
			r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: scheme, Mode: ModeKubernetes}
			r.observeCanceledPreDeploy(context.Background(), app)
			if app.Status.PreDeploy.Status != appv1alpha1.PreDeploySucceeded || app.Status.InitialDeployHookCompleted != tc.want {
				t.Fatalf("preDeploy %+v, completed %v; want the step's verdict and completed=%v", app.Status.PreDeploy, app.Status.InitialDeployHookCompleted, tc.want)
			}
		})
	}
}
