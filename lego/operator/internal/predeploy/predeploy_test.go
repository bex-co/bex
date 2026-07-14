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

package predeploy

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestJobName(t *testing.T) {
	if got := JobName("api", "gen-7"); got != "predeploy-api-gen-7" {
		t.Errorf("JobName = %q, want predeploy-api-gen-7", got)
	}
	// Long names clamp to the 63-char DNS limit and lowercase.
	long := JobName(strings.Repeat("X", 60), "gen-123")
	if len(long) > 63 || long != strings.ToLower(long) {
		t.Errorf("JobName not clamped/lowercased: len=%d %q", len(long), long)
	}
}

func TestJobShape(t *testing.T) {
	j := Job(Options{
		Name: "api", Namespace: "builds", Workspace: "tea-1",
		Image: "zot/api:gen-3", Command: "npm run migrate", Revision: "gen-3", Generation: 3,
		Env:              []corev1.EnvVar{{Name: "FOO", Value: "bar"}},
		EnvFrom:          []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "api-env"}}}},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "pull"}},
		SecurityContext:  &corev1.SecurityContext{RunAsNonRoot: ptr(true)},
		Volumes:          []corev1.Volume{{Name: "secrets"}},
		VolumeMounts:     []corev1.VolumeMount{{Name: "secrets", MountPath: "/etc/secrets"}},
	})

	if j.Name != "predeploy-api-gen-3" || j.Namespace != "builds" {
		t.Fatalf("job meta = %s/%s", j.Namespace, j.Name)
	}
	// Never retry a migration; a bounded deadline + TTL reap it.
	if j.Spec.BackoffLimit == nil || *j.Spec.BackoffLimit != 0 {
		t.Error("pre-deploy must never retry (backoffLimit 0)")
	}
	if j.Spec.ActiveDeadlineSeconds == nil || j.Spec.TTLSecondsAfterFinished == nil {
		t.Error("deadline + TTL must be set")
	}
	// Labels: service + component (for the backend's log selector) + workspace.
	if j.Labels[LabelService] != "api" || j.Labels[LabelComponent] != ComponentValue || j.Labels[labelWorkspace] != "tea-1" {
		t.Errorf("job labels = %v", j.Labels)
	}
	if j.Spec.Template.Labels[LabelService] != "api" {
		t.Error("pod must carry the predeploy service label for log selection")
	}

	pod := j.Spec.Template.Spec
	if pod.RestartPolicy != corev1.RestartPolicyNever {
		t.Error("restartPolicy must be Never")
	}
	if len(pod.Containers) != 1 {
		t.Fatalf("want 1 container, got %d", len(pod.Containers))
	}
	c := pod.Containers[0]
	if c.Image != "zot/api:gen-3" {
		t.Errorf("container image = %q, want the new revision's image", c.Image)
	}
	// The command is shell-wrapped so an arbitrary string works.
	if got, want := c.Command, []string{"sh", "-c", "npm run migrate"}; strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("command = %v, want %v", got, want)
	}
	// The app pod's env/secrets/pull secrets are threaded through unchanged.
	if len(c.Env) != 1 || c.Env[0].Name != "FOO" {
		t.Errorf("env not threaded: %v", c.Env)
	}
	if len(c.EnvFrom) != 1 {
		t.Errorf("envFrom not threaded: %v", c.EnvFrom)
	}
	if len(pod.ImagePullSecrets) != 1 || pod.ImagePullSecrets[0].Name != "pull" {
		t.Errorf("imagePullSecrets not threaded: %v", pod.ImagePullSecrets)
	}
	if c.SecurityContext == nil || c.SecurityContext.RunAsNonRoot == nil || !*c.SecurityContext.RunAsNonRoot {
		t.Error("security context not threaded")
	}
	if len(pod.Volumes) != 1 || len(c.VolumeMounts) != 1 {
		t.Error("secret-file volume/mount not threaded")
	}
}

func TestJobOmitsWorkspaceLabelWhenAbsent(t *testing.T) {
	j := Job(Options{Name: "api", Namespace: "builds", Command: "echo", Revision: "gen-1"})
	if _, ok := j.Labels[labelWorkspace]; ok {
		t.Error("workspace label must be omitted when Workspace is empty")
	}
}

func TestObserve(t *testing.T) {
	running := &batchv1.Job{}
	if s, _ := Observe(running); s != StatePending {
		t.Errorf("no terminal condition => %s, want Pending", s)
	}

	done := &batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
		{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
	}}}
	if s, _ := Observe(done); s != StateSucceeded {
		t.Errorf("JobComplete => %s, want Succeeded", s)
	}

	failed := &batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded", Message: "boom"},
	}}}
	s, msg := Observe(failed)
	if s != StateFailed {
		t.Errorf("JobFailed => %s, want Failed", s)
	}
	if !strings.Contains(msg, "boom") || !strings.Contains(msg, "BackoffLimitExceeded") {
		t.Errorf("failure message = %q, want the condition reason+message", msg)
	}
}

func fakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func activeJob(name string) *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "builds", Labels: map[string]string{LabelService: "api"},
	}}
}

func TestCancelSupersededDeletesOtherActiveRunsButKeepsCurrent(t *testing.T) {
	ctx := context.Background()
	old := activeJob("predeploy-api-gen-1")
	cur := activeJob("predeploy-api-gen-2")
	done := activeJob("predeploy-api-gen-0")
	done.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	cl := fakeClient(old, cur, done)

	if err := CancelSuperseded(ctx, "api", "builds", "predeploy-api-gen-2", cl); err != nil {
		t.Fatalf("CancelSuperseded: %v", err)
	}

	// The superseded active run is deleted...
	if err := cl.Get(ctx, client.ObjectKeyFromObject(old), &batchv1.Job{}); err == nil {
		t.Error("superseded run predeploy-api-gen-1 should have been deleted")
	}
	// ...the current run is kept...
	if err := cl.Get(ctx, client.ObjectKeyFromObject(cur), &batchv1.Job{}); err != nil {
		t.Errorf("current run predeploy-api-gen-2 must be kept: %v", err)
	}
	// ...and a finished run is left alone (not active).
	if err := cl.Get(ctx, client.ObjectKeyFromObject(done), &batchv1.Job{}); err != nil {
		t.Errorf("finished run should not be touched: %v", err)
	}
}
