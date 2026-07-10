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

package build

import (
	"context"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func opts() Options {
	return Options{
		Repo: "https://github.com/bex-co/hello", Ref: "main", Name: "hello",
		Registry: "zot.bex-registry.svc:5000", Revision: "gen-7", Namespace: "default",
	}
}

func TestImageRefAndJobName(t *testing.T) {
	o := opts()
	if got, want := o.ImageRef(), "zot.bex-registry.svc:5000/hello:gen-7"; got != want {
		t.Errorf("ImageRef = %q, want %q", got, want)
	}
	if got := JobName("hello", "gen-7"); got != "bld-hello-gen-7" {
		t.Errorf("JobName = %q, want bld-hello-gen-7", got)
	}
	// Long names are truncated to the 63-char DNS limit and lowercased.
	long := JobName(strings.Repeat("x", 40), "gen-123456789")
	if len(long) > 63 || long != strings.ToLower(long) {
		t.Errorf("JobName not clamped/lowercased: len=%d %q", len(long), long)
	}
}

func TestBuildJobShape(t *testing.T) {
	o := opts()
	j := BuildJob(o, o.ImageRef())

	if j.Namespace != "default" || j.Name != "bld-hello-gen-7" {
		t.Fatalf("job meta = %s/%s", j.Namespace, j.Name)
	}
	// One-shot build, deadline set, TTL reaps it.
	if j.Spec.BackoffLimit == nil || *j.Spec.BackoffLimit != 1 {
		t.Error("build must not retry (backoffLimit 1)")
	}
	if j.Spec.ActiveDeadlineSeconds == nil || j.Spec.TTLSecondsAfterFinished == nil {
		t.Error("deadline + TTL must be set")
	}
	pod := j.Spec.Template.Spec
	if pod.RestartPolicy != corev1.RestartPolicyNever {
		t.Error("restartPolicy must be Never")
	}
	c := pod.Containers[0]
	joined := strings.Join(c.Args, " ")
	// The git context (repo.git#ref — the .git suffix forces a git clone, not an
	// HTTP fetch) and a pushing, insecure image output must be present.
	if !strings.Contains(joined, "context=https://github.com/bex-co/hello.git#main") {
		t.Errorf("missing git context arg: %q", joined)
	}
	if !strings.Contains(joined, "type=image,name=zot.bex-registry.svc:5000/hello:gen-7,push=true,registry.insecure=true") {
		t.Errorf("missing pushing image output: %q", joined)
	}
	if c.Command[0] != "buildctl-daemonless.sh" {
		t.Errorf("command = %v, want buildctl-daemonless.sh", c.Command)
	}
	// Rootless: unconfined seccomp, no privileged escalation.
	if c.SecurityContext == nil || c.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeUnconfined {
		t.Error("rootless buildkit needs an unconfined seccomp profile")
	}
	if c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
		t.Error("build container must not be privileged")
	}
}

func TestGitContext(t *testing.T) {
	cases := map[[2]string]string{
		{"https://github.com/x/y", "main"}:     "https://github.com/x/y.git#main", // .git appended, ref added
		{"https://github.com/x/y.git", "main"}: "https://github.com/x/y.git#main", // already .git, not doubled
		{"https://github.com/x/y", ""}:         "https://github.com/x/y.git",      // no trailing # when ref empty
		{"git@github.com:x/y.git", "dev"}:      "git@github.com:x/y.git#dev",      // ssh scheme untouched
	}
	for in, want := range cases {
		if got := gitContext(in[0], in[1]); got != want {
			t.Errorf("gitContext(%q,%q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func fakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func completedJob(o Options, cond batchv1.JobConditionType) *batchv1.Job {
	j := BuildJob(o, o.ImageRef())
	j.Status.Conditions = []batchv1.JobCondition{{
		Type: cond, Status: corev1.ConditionTrue, Reason: "test", Message: "boom",
	}}
	return j
}

func TestBuildAdoptsCompletedJobAndReturnsImage(t *testing.T) {
	o := opts()
	o.Client = fakeClient(completedJob(o, batchv1.JobComplete)) // pre-seeded, already Complete
	res, err := Build(context.Background(), o)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if res.Image != "zot.bex-registry.svc:5000/hello:gen-7" {
		t.Errorf("image = %q", res.Image)
	}
}

func TestBuildReportsFailedJob(t *testing.T) {
	o := opts()
	o.Client = fakeClient(completedJob(o, batchv1.JobFailed))
	if _, err := Build(context.Background(), o); err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("want a build-failed error, got %v", err)
	}
}

func TestBuildCreatesJobWhenAbsent(t *testing.T) {
	// No pre-seeded Job: Build creates it. With a fake client the Job never
	// completes, so Build blocks in its wait loop — assert the Job got created,
	// then cancel to end the (otherwise 20-min) wait.
	o := opts()
	o.Client = fakeClient()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _, _ = Build(ctx, o) }()

	key := client.ObjectKey{Namespace: o.Namespace, Name: JobName(o.Name, o.Revision)}
	found := false
	for range 200 {
		var j batchv1.Job
		if err := o.Client.Get(ctx, key, &j); err == nil {
			found = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !found {
		t.Fatal("Build did not create the build Job")
	}
}

func TestBuildJobResourceLimits(t *testing.T) {
	c := BuildJob(opts(), opts().ImageRef()).Spec.Template.Spec.Containers[0]
	r, l := c.Resources.Requests, c.Resources.Limits
	// resource.Quantity.Cpu()/Memory() return zero-value Quantity, not nil, for
	// absent keys — no nil guards needed before IsZero().
	if r.Cpu().IsZero() {
		t.Error("build Job cpu request must not be zero")
	}
	if r.Memory().IsZero() {
		t.Error("build Job memory request must not be zero")
	}
	if l.Cpu().IsZero() {
		t.Error("build Job cpu limit must not be zero")
	}
	if l.Memory().IsZero() {
		t.Error("build Job memory limit must not be zero")
	}
	if l.Cpu().Cmp(*r.Cpu()) < 0 {
		t.Error("cpu limit must be >= cpu request")
	}
	if l.Memory().Cmp(*r.Memory()) < 0 {
		t.Error("memory limit must be >= memory request")
	}
}

func TestBuildRejectsBuildpackAndNilClient(t *testing.T) {
	o := opts()
	o.Client = fakeClient()
	o.Builder = BuilderBuildpack
	if _, err := Build(context.Background(), o); err == nil || !strings.Contains(err.Error(), "buildpack") {
		t.Errorf("buildpack should be rejected in-cluster, got %v", err)
	}
	o2 := opts() // nil client
	o2.Builder = BuilderDockerfile
	if _, err := Build(context.Background(), o2); err == nil || !strings.Contains(err.Error(), "nil client") {
		t.Errorf("nil client should error, got %v", err)
	}
}
