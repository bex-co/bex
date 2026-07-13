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

// TestBuildJobSigningMovesBuildToInitAndSignsAfterPush pins the w6/006 tenant-
// image-signing path: with a signing key Secret set, build+push becomes an
// initContainer and a cosign container signs the pushed image as the main
// container (k8s runs init → containers sequentially, so signing only fires on a
// successful push). Unset (the default) stays a single buildkit container.
func TestBuildJobSigningMovesBuildToInitAndSignsAfterPush(t *testing.T) {
	image := opts().ImageRef()

	// Default (no signing): one buildkit container, no init/volumes — unchanged.
	def := BuildJob(opts(), image).Spec.Template.Spec
	if len(def.Containers) != 1 || def.Containers[0].Name != "buildkit" || len(def.InitContainers) != 0 {
		t.Fatalf("unsigned job = %d containers (%v) + %d init; want single buildkit, no init",
			len(def.Containers), contNames(def.Containers), len(def.InitContainers))
	}

	// Signing enabled: buildkit is an initContainer, cosign is the main container.
	o := opts()
	o.SignKeySecret = "bex-tenant-cosign"
	signed := BuildJob(o, image).Spec.Template.Spec
	if len(signed.InitContainers) != 1 || signed.InitContainers[0].Name != "buildkit" {
		t.Fatalf("signed job init = %v; want [buildkit]", contNames(signed.InitContainers))
	}
	if len(signed.Containers) != 1 || signed.Containers[0].Name != "sign" {
		t.Fatalf("signed job containers = %v; want [sign]", contNames(signed.Containers))
	}
	sign := signed.Containers[0]
	// Signs the exact pushed ref, keyless-disabled (key from the mounted Secret),
	// insecure-registry for the in-cluster Zot over HTTP.
	joined := strings.Join(sign.Args, " ")
	if !strings.Contains(joined, "sign --yes --allow-insecure-registry --key /keys/cosign.key "+image) {
		t.Fatalf("sign args = %q; want cosign sign of %s", joined, image)
	}
	if sign.Image != defaultSignImage {
		t.Errorf("sign image = %s, want default %s", sign.Image, defaultSignImage)
	}
	// Key Secret mounted read-only at /keys + COSIGN_PASSWORD from the same Secret.
	if len(sign.VolumeMounts) != 1 || sign.VolumeMounts[0].MountPath != "/keys" || !sign.VolumeMounts[0].ReadOnly {
		t.Errorf("sign volumeMounts = %+v; want /keys ro", sign.VolumeMounts)
	}
	if len(signed.Volumes) != 1 || signed.Volumes[0].Secret == nil ||
		signed.Volumes[0].Secret.SecretName != "bex-tenant-cosign" {
		t.Errorf("volumes = %+v; want the cosign-key Secret", signed.Volumes)
	}
	gotPW := false
	for _, e := range sign.Env {
		if e.Name == "COSIGN_PASSWORD" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil &&
			e.ValueFrom.SecretKeyRef.Name == "bex-tenant-cosign" {
			gotPW = true
		}
	}
	if !gotPW {
		t.Error("sign container must pull COSIGN_PASSWORD from the signing-key Secret")
	}
}

func contNames(cs []corev1.Container) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func TestBuildJobCloneSecret(t *testing.T) {
	// Unset: no GIT_AUTH_TOKEN env, no --secret arg — byte-identical to a public clone.
	c := BuildJob(opts(), opts().ImageRef()).Spec.Template.Spec.Containers[0]
	if strings.Contains(strings.Join(c.Args, " "), "GIT_AUTH_TOKEN") {
		t.Error("no clone secret => no GIT_AUTH_TOKEN build secret")
	}
	for _, e := range c.Env {
		if e.Name == "GIT_AUTH_TOKEN" {
			t.Error("no clone secret => no GIT_AUTH_TOKEN env")
		}
	}

	// Set: BuildKit gets the token as its GIT_AUTH_TOKEN secret sourced from the
	// named Secret's "token" key.
	o := opts()
	o.CloneSecret = "hello-clone"
	c = BuildJob(o, o.ImageRef()).Spec.Template.Spec.Containers[0]
	if !strings.Contains(strings.Join(c.Args, " "), "id=GIT_AUTH_TOKEN,env=GIT_AUTH_TOKEN") {
		t.Errorf("missing --secret GIT_AUTH_TOKEN arg: %v", c.Args)
	}
	var found *corev1.EnvVar
	for i := range c.Env {
		if c.Env[i].Name == "GIT_AUTH_TOKEN" {
			found = &c.Env[i]
		}
	}
	if found == nil || found.ValueFrom == nil || found.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("GIT_AUTH_TOKEN env not sourced from a Secret: %+v", c.Env)
	}
	if found.ValueFrom.SecretKeyRef.Name != "hello-clone" || found.ValueFrom.SecretKeyRef.Key != "token" {
		t.Errorf("secret ref = %+v, want hello-clone/token", found.ValueFrom.SecretKeyRef)
	}
}

func TestGitContext(t *testing.T) {
	cases := map[[3]string]string{
		{"https://github.com/x/y", "main", ""}:             "https://github.com/x/y.git#main",              // .git appended, ref added
		{"https://github.com/x/y.git", "main", ""}:         "https://github.com/x/y.git#main",              // already .git, not doubled
		{"https://github.com/x/y", "", ""}:                 "https://github.com/x/y.git",                   // no trailing # when ref and rootDir empty
		{"git@github.com:x/y.git", "dev", ""}:              "git@github.com:x/y.git#dev",                   // ssh scheme untouched
		{"https://github.com/x/y", "main", "services/api"}: "https://github.com/x/y.git#main:services/api", // rootDir suffix after ref
		{"https://github.com/x/y", "", "services/api"}:     "https://github.com/x/y.git#:services/api",     // rootDir with default (empty) ref: bare "#" still introduces it
		{"git@github.com:x/y.git", "dev", "apps/web"}:      "git@github.com:x/y.git#dev:apps/web",          // ssh scheme + rootDir
	}
	for in, want := range cases {
		if got := gitContext(in[0], in[1], in[2]); got != want {
			t.Errorf("gitContext(%q,%q,%q) = %q, want %q", in[0], in[1], in[2], got, want)
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

// TestBuildJobPushSecret pins w7/m8: with a push Secret set, the buildkit
// container mounts the docker-config at /docker-config (DOCKER_CONFIG pointing
// there) so buildkitd authenticates the push — and the credential is reachable
// ONLY through that mount: the Secret name and any credential material never
// appear in buildctl args or the build container's env (a tenant Dockerfile RUN
// step could read those). Unset ⇒ no mount, no DOCKER_CONFIG env (byte-identical
// dev default).
func TestBuildJobPushSecret(t *testing.T) {
	const secret = "bex-registry-push"

	// Unset: byte-identical — no registry-cred volume, no DOCKER_CONFIG env.
	def := BuildJob(opts(), opts().ImageRef()).Spec.Template.Spec
	if volByName(def.Volumes, "registry-cred") != nil {
		t.Error("no push secret => no registry-cred volume")
	}
	if dockerConfigValue(def.Containers[0].Env) != "" {
		t.Error("no push secret => no DOCKER_CONFIG env on buildkit")
	}

	// Set: buildkit gets the mount + DOCKER_CONFIG; the volume references the Secret.
	o := opts()
	o.PushSecret = secret
	set := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	bk := containerByName(set.Containers, "buildkit")
	if bk == nil {
		t.Fatal("buildkit container missing")
	}
	if vm := mountByName(bk.VolumeMounts, "registry-cred"); vm == nil || vm.MountPath != dockerConfigMount || !vm.ReadOnly {
		t.Errorf("buildkit registry-cred mount = %+v; want ro %s", vm, dockerConfigMount)
	}
	if dockerConfigValue(bk.Env) != dockerConfigMount {
		t.Errorf("buildkit DOCKER_CONFIG = %q; want %q", dockerConfigValue(bk.Env), dockerConfigMount)
	}
	vol := volByName(set.Volumes, "registry-cred")
	if vol == nil || vol.Secret == nil || vol.Secret.SecretName != secret {
		t.Errorf("registry-cred volume = %+v; want Secret %s", vol, secret)
	}

	// NEGATIVE SPACE — the load-bearing security invariant: the credential never
	// appears anywhere a tenant RUN step can read it. BuildKit RUN steps can see
	// buildctl args (no) and declared --secret mounts (none here), but NOT the
	// container's own volume mounts — still, assert the Secret name leaks nowhere
	// in args or env, so a future refactor can't accidentally inline it.
	for _, a := range bk.Args {
		if strings.Contains(a, secret) || strings.Contains(a, "config.json") {
			t.Errorf("credential leaked into buildctl args: %q", a)
		}
	}
	for _, e := range bk.Env {
		if strings.Contains(e.Value, secret) {
			t.Errorf("credential leaked into buildkit env %s=%q", e.Name, e.Value)
		}
	}
}

// TestBuildJobPushSecretWithSigning asserts the w7/m8 mount lands on BOTH the
// buildkit initContainer and the cosign container when push + signing are both
// enabled (cosign pushes a signature artifact, so it authenticates too).
func TestBuildJobPushSecretWithSigning(t *testing.T) {
	o := opts()
	o.PushSecret = "bex-registry-push"
	o.SignKeySecret = "bex-tenant-cosign"
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec

	bk := containerByName(spec.InitContainers, "buildkit")
	sign := containerByName(spec.Containers, "sign")
	if bk == nil || sign == nil {
		t.Fatalf("want buildkit init + sign containers; got init=%v containers=%v",
			contNames(spec.InitContainers), contNames(spec.Containers))
	}
	for _, c := range []*corev1.Container{bk, sign} {
		if mountByName(c.VolumeMounts, "registry-cred") == nil {
			t.Errorf("%s container missing the registry-cred mount", c.Name)
		}
		if dockerConfigValue(c.Env) != dockerConfigMount {
			t.Errorf("%s container DOCKER_CONFIG = %q; want %q", c.Name, dockerConfigValue(c.Env), dockerConfigMount)
		}
	}
	// Both volumes present (registry-cred + cosign-key).
	if volByName(spec.Volumes, "registry-cred") == nil || volByName(spec.Volumes, "cosign-key") == nil {
		t.Errorf("want both registry-cred + cosign-key volumes; got %+v", spec.Volumes)
	}
}

func containerByName(cs []corev1.Container, name string) *corev1.Container {
	for i := range cs {
		if cs[i].Name == name {
			return &cs[i]
		}
	}
	return nil
}

func mountByName(mounts []corev1.VolumeMount, name string) *corev1.VolumeMount {
	for i := range mounts {
		if mounts[i].Name == name {
			return &mounts[i]
		}
	}
	return nil
}

func volByName(vols []corev1.Volume, name string) *corev1.Volume {
	for i := range vols {
		if vols[i].Name == name {
			return &vols[i]
		}
	}
	return nil
}

func dockerConfigValue(envs []corev1.EnvVar) string {
	for _, e := range envs {
		if e.Name == "DOCKER_CONFIG" {
			return e.Value
		}
	}
	return ""
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

// TestBuildJobWorkspaceLabel pins w7/m9: the workspace label is stamped when
// Options.Workspace is set, omitted otherwise (byte-identical legacy path).
func TestBuildJobWorkspaceLabel(t *testing.T) {
	// No workspace: label absent.
	j := BuildJob(opts(), opts().ImageRef())
	if _, ok := j.Labels["app.bex.co/workspace"]; ok {
		t.Error("no Workspace => workspace label must be absent")
	}

	// Workspace set: label present on Job and pod template.
	o := opts()
	o.Workspace = "tea-abc"
	j = BuildJob(o, o.ImageRef())
	if j.Labels["app.bex.co/workspace"] != "tea-abc" {
		t.Errorf("job workspace label = %q, want tea-abc", j.Labels["app.bex.co/workspace"])
	}
}

// TestCancelActiveBuilds pins w7/m9 newest-wins: active Jobs are deleted,
// complete/failed Jobs are left untouched, and a not-found on delete is tolerated.
func TestCancelActiveBuilds(t *testing.T) {
	o := opts()
	active := BuildJob(o, o.ImageRef()) // active: no conditions
	active.Name = JobName(o.Name, "gen-5")

	done := completedJob(o, batchv1.JobComplete)
	done.Name = JobName(o.Name, "gen-4")

	cl := fakeClient(active, done)
	ctx := context.Background()

	if err := CancelActiveBuilds(ctx, o.Name, o.Namespace, cl); err != nil {
		t.Fatalf("CancelActiveBuilds: %v", err)
	}

	// Active Job deleted.
	var j batchv1.Job
	if err := cl.Get(ctx, client.ObjectKey{Namespace: o.Namespace, Name: active.Name}, &j); err == nil {
		t.Error("active build Job should have been deleted")
	}
	// Completed Job untouched.
	if err := cl.Get(ctx, client.ObjectKey{Namespace: o.Namespace, Name: done.Name}, &j); err != nil {
		t.Errorf("completed build Job should not be deleted: %v", err)
	}
}

// TestActiveWorkspaceBuilds pins w7/m9 per-workspace build counting: only
// active (not complete/failed) Jobs with the workspace label are counted.
func TestActiveWorkspaceBuilds(t *testing.T) {
	activeA := BuildJob(opts(), opts().ImageRef())
	activeA.Name = "bld-a-gen-1"
	activeA.Labels["app.bex.co/workspace"] = "tea-x"

	activeB := BuildJob(opts(), opts().ImageRef())
	activeB.Name = "bld-b-gen-1"
	activeB.Labels["app.bex.co/workspace"] = "tea-x"

	doneC := completedJob(opts(), batchv1.JobComplete)
	doneC.Name = "bld-c-gen-1"
	doneC.Labels["app.bex.co/workspace"] = "tea-x"

	otherWS := BuildJob(opts(), opts().ImageRef())
	otherWS.Name = "bld-d-gen-1"
	otherWS.Labels["app.bex.co/workspace"] = "tea-y"

	cl := fakeClient(activeA, activeB, doneC, otherWS)
	ctx := context.Background()

	n, err := ActiveWorkspaceBuilds(ctx, "tea-x", "default", cl)
	if err != nil {
		t.Fatalf("ActiveWorkspaceBuilds: %v", err)
	}
	// 2 active for tea-x (done and other-workspace excluded).
	if n != 2 {
		t.Errorf("ActiveWorkspaceBuilds(tea-x) = %d, want 2", n)
	}

	// Empty workspace string is a no-op (returns 0, not an error).
	n, err = ActiveWorkspaceBuilds(ctx, "", "default", cl)
	if err != nil || n != 0 {
		t.Errorf("empty workspace: got (%d, %v), want (0, nil)", n, err)
	}
}
