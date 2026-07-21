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
	"errors"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
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
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Error("build pod must disable the Kubernetes API token")
	}
	if pod.HostUsers == nil || *pod.HostUsers {
		t.Error("build pod must run in a Kubernetes user namespace")
	}
	if pod.NodeSelector["bex.co/pool"] != "tenant" {
		t.Errorf("node selector = %v, want tenant pool", pod.NodeSelector)
	}
	if got := contNames(pod.InitContainers); strings.Join(got, ",") != "clone,buildkit" {
		t.Fatalf("init containers = %v, want clone,buildkit", got)
	}
	if got := contNames(pod.Containers); strings.Join(got, ",") != "push" {
		t.Fatalf("containers = %v, want push", got)
	}
	bk := containerByName(pod.InitContainers, "buildkit")
	joined := strings.Join(bk.Args, " ")
	if !strings.Contains(joined, "context=/source") || !strings.Contains(joined, "type=oci,dest=/output/image.tar") {
		t.Errorf("buildkit must use local source and OCI output: %q", joined)
	}
	if strings.Contains(joined, o.Repo) || strings.Contains(joined, "push=true") {
		t.Errorf("buildkit must neither clone nor push: %q", joined)
	}
	push := pod.Containers[0]
	if !strings.Contains(strings.Join(push.Args, " "), "docker://"+o.ImageRef()) {
		t.Errorf("push args = %v", push.Args)
	}
	if bk.Command[0] != "buildctl-daemonless.sh" {
		t.Errorf("command = %v, want buildctl-daemonless.sh", bk.Command)
	}
	// BuildKit gets only Pod-user-namespace capabilities and keeps its default
	// OCI process sandbox. It must never become a host-privileged container.
	if bk.SecurityContext == nil || bk.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeUnconfined {
		t.Error("BuildKit needs an unconfined container seccomp profile")
	}
	if bk.SecurityContext.Privileged != nil && *bk.SecurityContext.Privileged {
		t.Error("build container must not be privileged")
	}
	if flags := envValue(bk.Env, "BUILDKITD_FLAGS"); strings.Contains(flags, "no-process-sandbox") {
		t.Errorf("BuildKit process sandbox disabled: %q", flags)
	}
}

func TestBuildJobCustomDockerfilePath(t *testing.T) {
	o := opts()
	o.DockerfilePath = "docker/Dockerfile.prod"
	args := containerByName(BuildJob(o, o.ImageRef()).Spec.Template.Spec.InitContainers, "buildkit").Args
	if !containsPair(args, "--opt", "filename=docker/Dockerfile.prod") {
		t.Fatalf("buildkit args missing Dockerfile path: %v", args)
	}
}

// TestBuildJobSigningMovesBuildToInitAndSignsAfterPush pins the w6/006 tenant-
// image-signing path: with a signing key Secret set, build+push becomes an
// initContainer and a cosign container signs the pushed image as the main
// container (k8s runs init → containers sequentially, so signing only fires on a
// successful push).
func TestBuildJobSigningMovesBuildToInitAndSignsAfterPush(t *testing.T) {
	image := opts().ImageRef()

	// Default: clone + build init containers, then the credential-isolated push.
	def := BuildJob(opts(), image).Spec.Template.Spec
	if strings.Join(contNames(def.InitContainers), ",") != "clone,buildkit" || strings.Join(contNames(def.Containers), ",") != "push" {
		t.Fatalf("unsigned phases = init %v main %v", contNames(def.InitContainers), contNames(def.Containers))
	}

	// Signing enabled: push also becomes an initContainer, then cosign runs.
	o := opts()
	o.SignKeySecret = "bex-tenant-cosign"
	signed := BuildJob(o, image).Spec.Template.Spec
	if strings.Join(contNames(signed.InitContainers), ",") != "clone,buildkit,push" {
		t.Fatalf("signed job init = %v; want [clone buildkit push]", contNames(signed.InitContainers))
	}
	if len(signed.Containers) != 1 || signed.Containers[0].Name != "sign" {
		t.Fatalf("signed job containers = %v; want [sign]", contNames(signed.Containers))
	}
	sign := signed.Containers[0]
	// Signs the exact pushed ref, keyless-disabled (key from the mounted Secret),
	// without publishing private image metadata to Rekor. allow-http-registry is
	// required for the in-cluster Zot over HTTP.
	joined := strings.Join(sign.Args, " ")
	if !strings.Contains(joined, "sign --yes --tlog-upload=false --allow-http-registry --key /keys/cosign.key "+image) {
		t.Fatalf("sign args = %q; want cosign sign of %s", joined, image)
	}
	if sign.Image != defaultSignImage {
		t.Errorf("sign image = %s, want default %s", sign.Image, defaultSignImage)
	}
	// Key Secret mounted read-only at /keys + COSIGN_PASSWORD from the same Secret.
	if mount := mountByName(sign.VolumeMounts, "cosign-key"); mount == nil || mount.MountPath != "/keys" || !mount.ReadOnly {
		t.Errorf("sign volumeMounts = %+v; want /keys ro", sign.VolumeMounts)
	}
	if volume := volByName(signed.Volumes, "cosign-key"); volume == nil || volume.Secret == nil ||
		volume.Secret.SecretName != "bex-tenant-cosign" {
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
	// Unset: no GIT_AUTH_TOKEN env on the clone container.
	pod := BuildJob(opts(), opts().ImageRef()).Spec.Template.Spec
	c := containerByName(pod.InitContainers, "clone")
	for _, e := range c.Env {
		if e.Name == "GIT_AUTH_TOKEN" {
			t.Error("no clone secret => no GIT_AUTH_TOKEN env")
		}
	}

	// Set: only the completed-before-build clone init container gets the token.
	o := opts()
	o.CloneSecret = "hello-clone"
	pod = BuildJob(o, o.ImageRef()).Spec.Template.Spec
	c = containerByName(pod.InitContainers, "clone")
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
	for _, other := range append(pod.InitContainers[1:], pod.Containers...) {
		for _, env := range other.Env {
			if env.Name == "GIT_AUTH_TOKEN" {
				t.Errorf("clone token leaked into %s", other.Name)
			}
		}
	}
}

func fakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func kpackImageWithCondition(o Options, status corev1.ConditionStatus, reason, message, latest string) *unstructured.Unstructured {
	image := KpackImage(o)
	condition := map[string]any{
		"type": kpackReadyCondition, "status": string(status), "reason": reason, "message": message,
	}
	image.Object["status"] = map[string]any{
		"conditions":  []any{condition},
		"latestImage": latest,
	}
	return image
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

func TestBuildStopsWaitingWhenOwningAppIsDeleting(t *testing.T) {
	o := opts()
	o.AppNamespace = "apps"
	now := metav1.Now()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: o.Name, Namespace: o.AppNamespace, UID: "uid-current",
		Finalizers: []string{"app.bex.co/finalizer"}, DeletionTimestamp: &now,
	}}
	o.AppUID = string(app.UID)
	o.Client = fakeClient(app)

	if _, err := Build(context.Background(), o); !errors.Is(err, ErrAppDeleting) {
		t.Fatalf("Build error = %v, want ErrAppDeleting", err)
	}
	var job batchv1.Job
	key := client.ObjectKey{Namespace: o.Namespace, Name: JobName(o.Name, o.Revision)}
	if err := o.Client.Get(context.Background(), key, &job); err != nil {
		t.Fatalf("build artifact must remain for finalizer inventory: %v", err)
	}
}

// TestBuildJobPushSecret proves the platform push credential is absent from the
// clone and BuildKit phases and mounted only after tenant code exits.
func TestBuildJobPushSecret(t *testing.T) {
	const secret = "bex-registry-push"

	// Unset: no push credential volume or env.
	def := BuildJob(opts(), opts().ImageRef()).Spec.Template.Spec
	if volByName(def.Volumes, "push-registry-cred") != nil {
		t.Error("no push secret => no push-registry-cred volume")
	}
	if dockerConfigValue(def.Containers[0].Env) != "" {
		t.Error("no push secret => no DOCKER_CONFIG env on buildkit")
	}

	// Set: only the pusher gets the mount + DOCKER_CONFIG.
	o := opts()
	o.PushSecret = secret
	set := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	bk := containerByName(set.InitContainers, "buildkit")
	push := containerByName(set.Containers, "push")
	if bk == nil || push == nil {
		t.Fatalf("phases missing: init=%v main=%v", contNames(set.InitContainers), contNames(set.Containers))
	}
	if mountByName(bk.VolumeMounts, "push-registry-cred") != nil || dockerConfigValue(bk.Env) != "" {
		t.Fatal("BuildKit must not receive platform push credentials")
	}
	if vm := mountByName(push.VolumeMounts, "push-registry-cred"); vm == nil || vm.MountPath != dockerConfigMount || !vm.ReadOnly {
		t.Errorf("pusher credential mount = %+v; want ro %s", vm, dockerConfigMount)
	}
	vol := volByName(set.Volumes, "push-registry-cred")
	if vol == nil || vol.Secret == nil || vol.Secret.SecretName != secret {
		t.Errorf("registry-cred volume = %+v; want Secret %s", vol, secret)
	}

	// The credential never appears in arguments or values visible to BuildKit.
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

// TestBuildJobPushSecretWithSigning asserts the credential lands on the serial
// push/sign phases, never BuildKit.
func TestBuildJobPushSecretWithSigning(t *testing.T) {
	o := opts()
	o.PushSecret = "bex-registry-push"
	o.SignKeySecret = "bex-tenant-cosign"
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec

	bk := containerByName(spec.InitContainers, "buildkit")
	push := containerByName(spec.InitContainers, "push")
	sign := containerByName(spec.Containers, "sign")
	if bk == nil || push == nil || sign == nil {
		t.Fatalf("want buildkit/push init + sign containers; got init=%v containers=%v",
			contNames(spec.InitContainers), contNames(spec.Containers))
	}
	if mountByName(bk.VolumeMounts, "push-registry-cred") != nil || dockerConfigValue(bk.Env) != "" {
		t.Fatal("BuildKit received push credential")
	}
	for _, c := range []*corev1.Container{push, sign} {
		if mountByName(c.VolumeMounts, "push-registry-cred") == nil {
			t.Errorf("%s container missing the push-registry-cred mount", c.Name)
		}
		if dockerConfigValue(c.Env) != dockerConfigMount {
			t.Errorf("%s container DOCKER_CONFIG = %q; want %q", c.Name, dockerConfigValue(c.Env), dockerConfigMount)
		}
	}
	if volByName(spec.Volumes, "push-registry-cred") == nil || volByName(spec.Volumes, "cosign-key") == nil {
		t.Errorf("want both push-registry-cred + cosign-key volumes; got %+v", spec.Volumes)
	}
}

func TestBuildJobPrivateBasePullCredentialIsReadOnlyAndSeparate(t *testing.T) {
	o := opts()
	o.PushSecret = "app-output"
	o.PullSecret = "private-base"
	o.RegistryConfig = true
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	bk := containerByName(spec.InitContainers, "buildkit")
	push := containerByName(spec.Containers, "push")
	if mountByName(bk.VolumeMounts, "pull-registry-cred") == nil || mountByName(bk.VolumeMounts, "push-registry-cred") != nil {
		t.Errorf("BuildKit mounts = %+v", bk.VolumeMounts)
	}
	if mountByName(push.VolumeMounts, "push-registry-cred") == nil || mountByName(push.VolumeMounts, "pull-registry-cred") != nil {
		t.Errorf("pusher mounts = %+v", push.VolumeMounts)
	}
	if flags := envValue(bk.Env, "BUILDKITD_FLAGS"); !strings.Contains(flags, "/docker-config/buildkitd.toml") {
		t.Errorf("BUILDKITD_FLAGS = %q", flags)
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
	return envValue(envs, "DOCKER_CONFIG")
}

func envValue(envs []corev1.EnvVar, name string) string {
	for _, e := range envs {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

func TestBuildJobResourceLimits(t *testing.T) {
	c := containerByName(BuildJob(opts(), opts().ImageRef()).Spec.Template.Spec.InitContainers, "buildkit")
	r, l := c.Resources.Requests, c.Resources.Limits
	if got := r.Cpu().String(); got != buildCPURequest {
		t.Errorf("build Job cpu request = %s, want %s", got, buildCPURequest)
	}
	if got := r.Memory().String(); got != buildMemoryRequest {
		t.Errorf("build Job memory request = %s, want %s", got, buildMemoryRequest)
	}
	if got := l.Cpu().String(); got != buildCPULimit {
		t.Errorf("build Job cpu limit = %s, want %s", got, buildCPULimit)
	}
	if got := l.Memory().String(); got != buildMemoryLimit {
		t.Errorf("build Job memory limit = %s, want %s", got, buildMemoryLimit)
	}
}

func TestBuildpackImageShapeAndSuccess(t *testing.T) {
	o := opts()
	o.Builder = BuilderBuildpack
	o.KpackRegistry = "zot.local:5000"
	o.RootDir = "services/api"
	o.BuildEnv = []corev1.EnvVar{
		{Name: "BP_GO_TARGETS", Value: "./cmd/api"},
		{Name: "IGNORED_SECRET", ValueFrom: &corev1.EnvVarSource{}},
	}
	image := KpackImage(o)
	if got, _, _ := unstructured.NestedString(image.Object, "spec", "tag"); got != "zot.local:5000/hello:gen-7" {
		t.Errorf("tag = %q", got)
	}
	if got, _, _ := unstructured.NestedString(image.Object, "spec", "source", "git", "url"); got != o.Repo {
		t.Errorf("repo = %q", got)
	}
	if got, _, _ := unstructured.NestedString(image.Object, "spec", "source", "git", "revision"); got != "main" {
		t.Errorf("revision = %q", got)
	}
	if got, _, _ := unstructured.NestedString(image.Object, "spec", "source", "subPath"); got != "services/api" {
		t.Errorf("subPath = %q", got)
	}
	env, _, _ := unstructured.NestedSlice(image.Object, "spec", "build", "env")
	if len(env) != 1 || env[0].(map[string]any)["name"] != "BP_GO_TARGETS" {
		t.Fatalf("build env = %#v", env)
	}
	if image.GetName() != "bld-hello-gen-7" || image.GetLabels()["app.bex.co/component"] != "build" {
		t.Errorf("image metadata = %s %#v", image.GetName(), image.GetLabels())
	}
	requests, _, _ := unstructured.NestedStringMap(image.Object, "spec", "build", "resources", "requests")
	limits, _, _ := unstructured.NestedStringMap(image.Object, "spec", "build", "resources", "limits")
	if requests["cpu"] != buildCPURequest || requests["memory"] != buildMemoryRequest {
		t.Errorf("kpack build requests = %#v", requests)
	}
	if limits["cpu"] != buildCPULimit || limits["memory"] != buildMemoryLimit {
		t.Errorf("kpack build limits = %#v", limits)
	}
	nodes, _, _ := unstructured.NestedStringMap(image.Object, "spec", "build", "nodeSelector")
	if nodes["bex.co/pool"] != "tenant" {
		t.Errorf("kpack node selector = %#v", nodes)
	}

	ready := kpackImageWithCondition(o, corev1.ConditionTrue, "BuildSuccess", "", "zot.local:5000/hello@sha256:abc")
	o.Client = fakeClient(ready)
	res, err := Build(context.Background(), o)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if res.Image != "zot.bex-registry.svc:5000/hello@sha256:abc" {
		t.Errorf("resolved image = %q", res.Image)
	}
	var job batchv1.Job
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: JobName(o.Name, o.Revision)}, &job); !apierrors.IsNotFound(err) {
		t.Errorf("buildpack dispatch must not create a BuildKit Job, got %v", err)
	}
	var sa corev1.ServiceAccount
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: kpackServiceAccountName(o.Name)}, &sa); err != nil {
		t.Fatalf("kpack service account: %v", err)
	}
	if sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken {
		t.Error("kpack ServiceAccount must disable token automount")
	}
}

func TestBuildpackFailureUsesBuildCondition(t *testing.T) {
	o := opts()
	o.Builder = BuilderBuildpack
	image := kpackImageWithCondition(o, corev1.ConditionFalse, "BuildFailed", "image fallback", "")
	image.Object["status"].(map[string]any)["latestBuildRef"] = "bld-hello-gen-7-build-1"
	build := newKpackBuild()
	build.SetName("bld-hello-gen-7-build-1")
	build.SetNamespace(o.Namespace)
	build.Object["status"] = map[string]any{"conditions": []any{map[string]any{
		"type": kpackSucceededCondition, "status": string(corev1.ConditionFalse),
		"reason": "BuildpackDetectFailed", "message": "no buildpack groups passed detection",
	}}}
	o.Client = fakeClient(image, build)
	if _, err := Build(context.Background(), o); err == nil || !strings.Contains(err.Error(), "BuildpackDetectFailed: no buildpack groups passed detection") {
		t.Fatalf("failure = %v", err)
	}
}

func TestBuildpackCreatesImageWhenAbsent(t *testing.T) {
	o := opts()
	o.Builder = BuilderBuildpack
	o.Client = fakeClient()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Build(ctx, o)
	}()

	key := client.ObjectKey{Namespace: o.Namespace, Name: JobName(o.Name, o.Revision)}
	found := false
	for range 200 {
		image := newKpackImage()
		if err := o.Client.Get(ctx, key, image); err == nil {
			found = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !found {
		t.Fatal("Build did not create the kpack Image")
	}
	cancel()
	<-done
}

func TestBuildpackStopsWaitingWhenOwningAppIsDeleting(t *testing.T) {
	o := opts()
	o.Builder = BuilderBuildpack
	o.AppNamespace = "apps"
	now := metav1.Now()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: o.Name, Namespace: o.AppNamespace, UID: "uid-current",
		Finalizers: []string{"app.bex.co/finalizer"}, DeletionTimestamp: &now,
	}}
	o.AppUID = string(app.UID)
	o.Client = fakeClient(app)

	if _, err := Build(context.Background(), o); !errors.Is(err, ErrAppDeleting) {
		t.Fatalf("Build error = %v, want ErrAppDeleting", err)
	}
	image := newKpackImage()
	key := client.ObjectKey{Namespace: o.Namespace, Name: JobName(o.Name, o.Revision)}
	if err := o.Client.Get(context.Background(), key, image); err != nil {
		t.Fatalf("kpack artifact must remain for finalizer inventory: %v", err)
	}
}

func TestKpackCredentialAdaptation(t *testing.T) {
	o := opts()
	o.KpackRegistry = "zot.local:5000"
	o.PushSecret = "bex-registry-push"
	o.CloneSecret = "hello-clone"
	o.SignKeySecret = "cosign-key"
	pushConfig := []byte(`{"auths":{"zot.bex-registry.svc:5000":{"username":"builder","password":"redacted"}}}`)
	push := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: o.PushSecret, Namespace: o.Namespace}, Data: map[string][]byte{"config.json": pushConfig}}
	clone := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: o.CloneSecret, Namespace: o.Namespace}, Data: map[string][]byte{"token": []byte("redacted-token")}}
	o.Client = fakeClient(push, clone)
	if err := ensureKpackCredentials(context.Background(), o); err != nil {
		t.Fatal(err)
	}

	var registry corev1.Secret
	registryName := JobName(o.Name, "kpack-registry")
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: registryName}, &registry); err != nil {
		t.Fatal(err)
	}
	if registry.Type != corev1.SecretTypeDockerConfigJson || !strings.Contains(string(registry.Data[corev1.DockerConfigJsonKey]), "zot.local:5000") {
		t.Errorf("adapted registry secret = type %s data %s", registry.Type, registry.Data[corev1.DockerConfigJsonKey])
	}
	var git corev1.Secret
	gitName := JobName(o.Name, "kpack-git")
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: gitName}, &git); err != nil {
		t.Fatal(err)
	}
	if git.Type != corev1.SecretTypeBasicAuth || git.Annotations["kpack.io/git"] != "https://github.com" || string(git.Data[corev1.BasicAuthPasswordKey]) != "redacted-token" {
		t.Errorf("adapted git secret = %#v", git)
	}
	var sa corev1.ServiceAccount
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: "bex-kpack-hello"}, &sa); err != nil {
		t.Fatal(err)
	}
	gotSecrets := make([]string, len(sa.Secrets))
	for i := range sa.Secrets {
		gotSecrets[i] = sa.Secrets[i].Name
	}
	if got := strings.Join(gotSecrets, ","); got != registryName+","+gitName+",cosign-key" {
		t.Errorf("service account secrets = %s", got)
	}
	if sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken {
		t.Error("kpack ServiceAccount must disable token automount")
	}
}

func TestBuildNilClient(t *testing.T) {
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
	if j.Spec.Template.Labels["app.bex.co/workspace"] != "tea-abc" ||
		j.Spec.Template.Labels["app.bex.co/app"] != o.Name {
		t.Errorf("pod identity labels = %v", j.Spec.Template.Labels)
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

// ---- DeleteAppArtifacts tests (w7/m12) ----

func buildJob(name, appName string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "build",
			Labels:    map[string]string{"app.bex.co/build": appName},
		},
	}
}

func predeployJob(name, appName, ns string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app.bex.co/predeploy": appName},
		},
	}
}

func artifactPod(name, label, appName, ns string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{label: appName},
		},
	}
}

func TestDeleteAppArtifacts_DeletesBuildAndPredeployJobs(t *testing.T) {
	ctx := context.Background()
	cl := fakeClient(
		buildJob("bld-hello-gen-1", "hello"),
		buildJob("bld-hello-gen-2", "hello"),
		predeployJob("pred-hello-gen-1", "hello", "build"),
		artifactPod("bld-hello-gen-1-pod", "app.bex.co/build", "hello", "build"),
		artifactPod("pred-hello-gen-1-pod", "app.bex.co/predeploy", "hello", "build"),
		// another app's jobs — must NOT be deleted
		buildJob("bld-other-gen-1", "other"),
		artifactPod("bld-other-gen-1-pod", "app.bex.co/build", "other", "build"),
	)

	if err := DeleteAppArtifacts(ctx, "hello", "build", cl); err != nil {
		t.Fatalf("DeleteAppArtifacts: %v", err)
	}

	// hello's build Jobs must be gone.
	var jobs batchv1.JobList
	if err := cl.List(ctx, &jobs, client.InNamespace("build"),
		client.MatchingLabels{"app.bex.co/build": "hello"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(jobs.Items) != 0 {
		t.Errorf("build Jobs for 'hello' still present: %d", len(jobs.Items))
	}

	// hello's predeploy Jobs must be gone.
	if err := cl.List(ctx, &jobs, client.InNamespace("build"),
		client.MatchingLabels{"app.bex.co/predeploy": "hello"}); err != nil {
		t.Fatalf("list predeploy: %v", err)
	}
	if len(jobs.Items) != 0 {
		t.Errorf("predeploy Jobs for 'hello' still present: %d", len(jobs.Items))
	}

	// The Job controller normally cascades these Pods, but the finalizer also
	// removes them explicitly so an orphaned completed Pod cannot survive.
	var pods corev1.PodList
	if err := cl.List(ctx, &pods, client.InNamespace("build"),
		client.MatchingLabels{"app.bex.co/build": "hello"}); err != nil {
		t.Fatalf("list build pods: %v", err)
	}
	if len(pods.Items) != 0 {
		t.Errorf("build Pods for 'hello' still present: %d", len(pods.Items))
	}
	if err := cl.List(ctx, &pods, client.InNamespace("build"),
		client.MatchingLabels{"app.bex.co/predeploy": "hello"}); err != nil {
		t.Fatalf("list predeploy pods: %v", err)
	}
	if len(pods.Items) != 0 {
		t.Errorf("predeploy Pods for 'hello' still present: %d", len(pods.Items))
	}

	// other's build Job must still be there.
	if err := cl.List(ctx, &jobs, client.InNamespace("build"),
		client.MatchingLabels{"app.bex.co/build": "other"}); err != nil {
		t.Fatalf("list other: %v", err)
	}
	if len(jobs.Items) != 1 {
		t.Errorf("other app's build Job was incorrectly deleted; want 1, got %d", len(jobs.Items))
	}
	if err := cl.List(ctx, &pods, client.InNamespace("build"),
		client.MatchingLabels{"app.bex.co/build": "other"}); err != nil {
		t.Fatalf("list other pods: %v", err)
	}
	if len(pods.Items) != 1 {
		t.Errorf("other app's build Pod was incorrectly deleted; want 1, got %d", len(pods.Items))
	}
}

func TestDeleteAppArtifacts_EmptyNamespaceReturnsNil(t *testing.T) {
	// No pre-existing objects — should be a no-op, not an error.
	cl := fakeClient()
	if err := DeleteAppArtifacts(context.Background(), "hello", "build", cl); err != nil {
		t.Fatalf("DeleteAppArtifacts on empty namespace: %v", err)
	}
}

func TestDeleteAppArtifacts_MissingKpackIsNotAnError(t *testing.T) {
	// kpack CRD is not installed; the fake client returns "no matches for kind".
	// DeleteAppArtifacts should tolerate that and return nil.
	ctx := context.Background()
	cl := fakeClient(buildJob("bld-hello-gen-1", "hello"))

	// Even without kpack registered in the scheme the function should succeed
	// (the build Job is deleted, the kpack list error is tolerated).
	if err := DeleteAppArtifacts(ctx, "hello", "build", cl); err != nil {
		t.Fatalf("DeleteAppArtifacts without kpack in scheme: %v", err)
	}
}
