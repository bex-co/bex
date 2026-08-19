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
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

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
		AppUID: "uid-hello", Registry: "zot.bex-registry.svc:5000", Revision: "gen-7", Namespace: "default",
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
	maxName := strings.Repeat("x", 55)
	if a, b := JobName(maxName, "gen-1"), JobName(maxName, "gen-2"); a == b {
		t.Fatalf("adjacent revisions collided after truncation: %q", a)
	}
}

func TestImageRefWorkspaceScoped(t *testing.T) {
	o := opts()
	o.Workspace = "tea-aaaaaaaaaaaaaaaaaaaa"
	if got, want := o.ImageRef(), "zot.bex-registry.svc:5000/tea-aaaaaaaaaaaaaaaaaaaa/hello:gen-7"; got != want {
		t.Errorf("ImageRef = %q, want %q", got, want)
	}
	o.Workspace = ""
	if got, want := o.ImageRef(), "zot.bex-registry.svc:5000/hello:gen-7"; got != want {
		t.Errorf("unlabeled ImageRef = %q, want %q", got, want)
	}
}

func TestKpackNamesBindUIDRevisionAndPurpose(t *testing.T) {
	o := opts()
	o.Name = strings.Repeat("x", 55)
	otherRevision := o
	otherRevision.Revision = "gen-8"
	otherUID := o
	otherUID.AppUID = "uid-sibling"

	names := []string{
		kpackImageName(o),
		kpackImageName(otherRevision),
		kpackImageName(otherUID),
		kpackServiceAccountName(o),
		kpackArtifactName(o, "bld-"+o.Name+"-kpack-registry", kpackRegistrySecretPurpose),
		kpackArtifactName(o, "bld-"+o.Name+"-kpack-git", kpackGitSecretPurpose),
	}
	seen := map[string]bool{}
	for _, name := range names {
		if len(name) > 63 || name != strings.ToLower(name) {
			t.Fatalf("invalid kpack name %q", name)
		}
		if seen[name] {
			t.Fatalf("UID/revision/purpose collision for %q", name)
		}
		seen[name] = true
	}
}

func TestCredentialBearingPlatformImagesAreDigestPinned(t *testing.T) {
	for name, image := range map[string]string{
		"buildkit": defaultBuildkitImage,
		"git":      defaultGitImage,
		"push":     defaultPushImage,
		"sign":     defaultSignImage,
	} {
		if !strings.Contains(image, "@sha256:") {
			t.Errorf("%s image is mutable: %q", name, image)
		}
	}
}

func TestBuildJobShape(t *testing.T) {
	o := opts()
	j := BuildJob(o, o.ImageRef())

	assertBuildJobMeta(t, o, j)
	assertBuildJobContainers(t, o, j)
}

func assertBuildJobMeta(t *testing.T, o Options, j *batchv1.Job) {
	t.Helper()
	if j.Namespace != "default" || j.Name != "bld-hello-gen-7" {
		t.Fatalf("job meta = %s/%s", j.Namespace, j.Name)
	}
	if j.Labels["app.bex.co/app-uid"] != o.AppUID || j.Spec.Template.Labels["app.bex.co/app-uid"] != o.AppUID {
		t.Fatalf("build artifact missing App UID labels: job=%v pod=%v", j.Labels, j.Spec.Template.Labels)
	}
	// Deadline set, TTL reaps it. Retry policy INVERTED by w7/m82 t002: the old
	// pin asserted backoffLimit 1 as "a failed build is a user error, not a
	// flake". Production disproved that in both directions -- deterministic
	// tenant failures were retried anyway (two pods per failing build), while an
	// autoscaler eviction failed the deploy outright. The budget is now 2 and
	// applies ONLY to unclassified failures; see the podFailurePolicy tests.
	if j.Spec.BackoffLimit == nil || *j.Spec.BackoffLimit != 2 {
		t.Errorf("backoffLimit = %v, want 2 for unclassified failures", j.Spec.BackoffLimit)
	}
	if j.Spec.ActiveDeadlineSeconds == nil || *j.Spec.ActiveDeadlineSeconds != 30*60 || j.Spec.TTLSecondsAfterFinished == nil {
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
		t.Error("build pod must run in a Kubernetes user namespace (spec.hostUsers=false)")
	}
	if pod.NodeSelector["bex.co/pool"] != "tenant" {
		t.Errorf("node selector = %v, want tenant pool", pod.NodeSelector)
	}
	// Build pods (and only build pods) tolerate the dedicated build-pool taint
	// (docs/ADR060 § dedicated build pool): serving/pre-deploy/publish pods must
	// keep being repelled so they can never pin an elastic build node.
	tolerated := false
	for _, tol := range pod.Tolerations {
		if tol.Key == "bex.co/build-only" && tol.Value == "true" && tol.Effect == corev1.TaintEffectNoSchedule {
			tolerated = true
		}
	}
	if !tolerated {
		t.Errorf("build pod must tolerate the build-pool taint, tolerations = %v", pod.Tolerations)
	}
	// The safe-to-evict pin was REMOVED by w7/m82 t002 and its absence is now
	// asserted in TestBuildJobNoLongerPinsNodesAgainstTheAutoscaler: eviction is
	// absorbed by podFailurePolicy instead of prevented, so pinning the node only
	// blocked consolidation. The apparmor annotation must survive that removal.
	if j.Spec.Template.Annotations["container.apparmor.security.beta.kubernetes.io/buildkit"] != "unconfined" {
		t.Errorf("buildkit apparmor annotation lost: %v", j.Spec.Template.Annotations)
	}
}

func assertBuildJobContainers(t *testing.T, o Options, j *batchv1.Job) {
	t.Helper()
	pod := j.Spec.Template.Spec
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
	// Since w7/m82 t001 buildkit runs under the classifier wrapper, so buildctl
	// is invoked from the script rather than being argv[0]. What still matters
	// is that buildctl is what actually runs, and that it receives the build
	// arguments positionally rather than spliced into the script.
	bkCmd := strings.Join(bk.Command, " ")
	if bk.Command[0] != "sh" || !strings.Contains(bkCmd, `bex_run buildctl-daemonless.sh "$@"`) {
		t.Errorf("command = %v, want sh running buildctl-daemonless.sh via bex_run", bk.Command)
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
		t.Errorf("BuildKit process sandbox must be enabled (pod user namespace is active): %q", flags)
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

func TestBuildReusesOwnedCompletedJobAndReturnsImage(t *testing.T) {
	o := opts()
	o.Client = fakeClient(completedJob(o, batchv1.JobComplete)) // pre-seeded, already Complete
	obs, err := EnsureBuild(context.Background(), o)
	if err != nil {
		t.Fatalf("EnsureBuild: %v", err)
	}
	if obs.Phase != PhaseSucceeded {
		t.Fatalf("phase = %v, want PhaseSucceeded", obs.Phase)
	}
	if obs.Image != "zot.bex-registry.svc:5000/hello:gen-7" {
		t.Errorf("image = %q", obs.Image)
	}
}

func TestBuildRequiresAppUID(t *testing.T) {
	o := opts()
	o.AppUID = ""
	o.Client = fakeClient()
	if _, err := EnsureBuild(context.Background(), o); err == nil || !strings.Contains(err.Error(), "empty App UID") {
		t.Fatalf("EnsureBuild error = %v, want missing-identity rejection", err)
	}
}

func TestBuildRejectsExistingJobWithoutAppUID(t *testing.T) {
	o := opts()
	job := completedJob(o, batchv1.JobComplete)
	delete(job.Labels, "app.bex.co/app-uid")
	o.Client = fakeClient(job)

	if _, err := EnsureBuild(context.Background(), o); err == nil || !strings.Contains(err.Error(), "different App lifetime") {
		t.Fatalf("EnsureBuild error = %v, want strict lifetime ownership rejection", err)
	}
	var got batchv1.Job
	if err := o.Client.Get(context.Background(), client.ObjectKeyFromObject(job), &got); err != nil {
		t.Fatal(err)
	}
	if got.Labels["app.bex.co/app-uid"] != "" {
		t.Fatalf("existing Job was adopted: labels=%v", got.Labels)
	}
}

func TestBuildReportsFailedJob(t *testing.T) {
	o := opts()
	o.Client = fakeClient(completedJob(o, batchv1.JobFailed))
	obs, err := EnsureBuild(context.Background(), o)
	if err != nil {
		t.Fatalf("EnsureBuild: %v", err)
	}
	if obs.Phase != PhaseFailed {
		t.Fatalf("phase = %v, want PhaseFailed", obs.Phase)
	}
	// A generic failure is not a timeout — the caller meters it as `failed`.
	if obs.Fault == FaultTimeout {
		t.Error("a generic build failure must not be classified as a timeout")
	}
}

// TestBuildTimeoutClassification pins the timeout branch (ADR060 §D1a/§D5): a Job
// failed via activeDeadlineSeconds reports FaultTimeout so the caller meters it
// as `timeout`, distinct from a tenant or infra failure.
func TestBuildTimeoutClassification(t *testing.T) {
	o := opts()
	j := BuildJob(o, o.ImageRef())
	j.Status.Conditions = []batchv1.JobCondition{{
		Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
		Reason: "DeadlineExceeded", Message: "Job was active longer than specified deadline",
	}}
	o.Client = fakeClient(j)
	obs, err := EnsureBuild(context.Background(), o)
	if err != nil {
		t.Fatalf("EnsureBuild: %v", err)
	}
	if obs.Phase != PhaseFailed || obs.Fault != FaultTimeout {
		t.Fatalf("observation = %+v, want PhaseFailed with FaultTimeout", obs)
	}
}

func TestBuildCreatesJobWhenAbsentAndReportsNonTerminal(t *testing.T) {
	// No pre-seeded Job: EnsureBuild creates it and returns immediately (ADR060
	// §D1 non-blocking — no wait loop). The fresh Job has no pod, so it reports a
	// non-terminal phase and the caller requeues.
	o := opts()
	o.Client = fakeClient()
	obs, err := EnsureBuild(context.Background(), o)
	if err != nil {
		t.Fatalf("EnsureBuild: %v", err)
	}
	if obs.Phase == PhaseSucceeded || obs.Phase == PhaseFailed {
		t.Fatalf("phase = %v, want a non-terminal phase for a just-created build", obs.Phase)
	}
	var j batchv1.Job
	key := client.ObjectKey{Namespace: o.Namespace, Name: JobName(o.Name, o.Revision)}
	if err := o.Client.Get(context.Background(), key, &j); err != nil {
		t.Fatalf("EnsureBuild did not create the build Job: %v", err)
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
	pod := BuildJob(opts(), opts().ImageRef()).Spec.Template.Spec
	c := containerByName(pod.InitContainers, "buildkit")
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
	if got := r.StorageEphemeral().String(); got != buildEphemeralRequest {
		t.Errorf("build Job ephemeral-storage request = %s, want %s", got, buildEphemeralRequest)
	}
	if got := l.StorageEphemeral().String(); got != buildEphemeralLimit {
		t.Errorf("build Job ephemeral-storage limit = %s, want %s", got, buildEphemeralLimit)
	}

	push := containerByName(pod.Containers, "push")
	if got := push.Resources.Requests.StorageEphemeral().String(); got != pushEphemeralRequest {
		t.Errorf("push ephemeral-storage request = %s, want %s", got, pushEphemeralRequest)
	}
	if got := push.Resources.Limits.StorageEphemeral().String(); got != pushEphemeralLimit {
		t.Errorf("push ephemeral-storage limit = %s, want %s", got, pushEphemeralLimit)
	}
	for _, name := range []string{"source", "output"} {
		volume := volByName(pod.Volumes, name)
		if volume == nil || volume.EmptyDir == nil || volume.EmptyDir.SizeLimit == nil {
			t.Fatalf("%s volume is missing its emptyDir size limit", name)
		}
		if got := volume.EmptyDir.SizeLimit.String(); got != emptyDirSizeLimit {
			t.Errorf("%s emptyDir size limit = %s, want %s", name, got, emptyDirSizeLimit)
		}
	}

	signedOpts := opts()
	signedOpts.SignKeySecret = "bex-tenant-cosign"
	signed := BuildJob(signedOpts, signedOpts.ImageRef()).Spec.Template.Spec
	signedPush := containerByName(signed.InitContainers, "push")
	signer := containerByName(signed.Containers, "sign")
	if got := signedPush.Resources.Limits.StorageEphemeral().String(); got != pushEphemeralLimit {
		t.Errorf("signed push ephemeral-storage limit = %s, want %s", got, pushEphemeralLimit)
	}
	if got := signer.Resources.Limits.StorageEphemeral().String(); got != lightEphemeralLimit {
		t.Errorf("sign ephemeral-storage limit = %s, want lightweight %s", got, lightEphemeralLimit)
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
	if image.GetName() != kpackImageName(o) || image.GetLabels()["app.bex.co/component"] != "build" || image.GetLabels()["app.bex.co/app-uid"] != o.AppUID {
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
	// Round-13 #4: kpack buildpack steps execute tenant-controlled source with
	// no bex-build LimitRange/Quota behind them — the per-workload bound is the
	// only disk cap, so it must match the Dockerfile/native build bound.
	if requests["ephemeral-storage"] != buildEphemeralRequest {
		t.Errorf("kpack build ephemeral-storage request = %q, want %q", requests["ephemeral-storage"], buildEphemeralRequest)
	}
	if limits["ephemeral-storage"] != buildEphemeralLimit {
		t.Errorf("kpack build ephemeral-storage limit = %q, want %q", limits["ephemeral-storage"], buildEphemeralLimit)
	}
	nodes, _, _ := unstructured.NestedStringMap(image.Object, "spec", "build", "nodeSelector")
	if nodes["bex.co/pool"] != "tenant" {
		t.Errorf("kpack node selector = %#v", nodes)
	}
	tolerations, _, _ := unstructured.NestedSlice(image.Object, "spec", "build", "tolerations")
	kpackTolerated := false
	for _, item := range tolerations {
		tol, ok := item.(map[string]any)
		if ok && tol["key"] == "bex.co/build-only" && tol["value"] == "true" && tol["effect"] == "NoSchedule" {
			kpackTolerated = true
		}
	}
	if !kpackTolerated {
		t.Errorf("kpack build must tolerate the build-pool taint, tolerations = %#v", tolerations)
	}

	ready := kpackImageWithCondition(o, corev1.ConditionTrue, "BuildSuccess", "", "zot.local:5000/hello@sha256:abc")
	o.Client = fakeClient(ready)
	obs, err := EnsureBuild(context.Background(), o)
	if err != nil {
		t.Fatalf("EnsureBuild: %v", err)
	}
	if obs.Phase != PhaseSucceeded {
		t.Fatalf("phase = %v, want PhaseSucceeded", obs.Phase)
	}
	if obs.Image != "zot.bex-registry.svc:5000/hello@sha256:abc" {
		t.Errorf("resolved image = %q", obs.Image)
	}
	var job batchv1.Job
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: JobName(o.Name, o.Revision)}, &job); !apierrors.IsNotFound(err) {
		t.Errorf("buildpack dispatch must not create a BuildKit Job, got %v", err)
	}
	var sa corev1.ServiceAccount
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: kpackServiceAccountName(o)}, &sa); err != nil {
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
	obs, err := EnsureBuild(context.Background(), o)
	if err != nil {
		t.Fatalf("EnsureBuild: %v", err)
	}
	if obs.Phase != PhaseFailed || !strings.Contains(obs.Message, "BuildpackDetectFailed: no buildpack groups passed detection") {
		t.Fatalf("observation = %+v", obs)
	}
}

func TestBuildpackCreatesImageWhenAbsentAndReportsNonTerminal(t *testing.T) {
	// EnsureBuild creates the kpack Image and returns immediately (ADR060 §D1
	// non-blocking); a fresh Image has no Ready condition, so it reports building.
	o := opts()
	o.Builder = BuilderBuildpack
	o.Client = fakeClient()
	obs, err := EnsureBuild(context.Background(), o)
	if err != nil {
		t.Fatalf("EnsureBuild: %v", err)
	}
	if obs.Phase != PhaseBuilding {
		t.Fatalf("phase = %v, want PhaseBuilding for a just-created kpack Image", obs.Phase)
	}
	image := newKpackImage()
	key := client.ObjectKey{Namespace: o.Namespace, Name: kpackImageName(o)}
	if err := o.Client.Get(context.Background(), key, image); err != nil {
		t.Fatalf("EnsureBuild did not create the kpack Image: %v", err)
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
	registryName := kpackArtifactName(o, "bld-"+o.Name+"-kpack-registry", kpackRegistrySecretPurpose)
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: registryName}, &registry); err != nil {
		t.Fatal(err)
	}
	if registry.Type != corev1.SecretTypeDockerConfigJson || !strings.Contains(string(registry.Data[corev1.DockerConfigJsonKey]), "zot.local:5000") {
		t.Errorf("adapted registry secret = type %s data %s", registry.Type, registry.Data[corev1.DockerConfigJsonKey])
	}
	var git corev1.Secret
	gitName := kpackArtifactName(o, "bld-"+o.Name+"-kpack-git", kpackGitSecretPurpose)
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: gitName}, &git); err != nil {
		t.Fatal(err)
	}
	if git.Type != corev1.SecretTypeBasicAuth || git.Annotations["kpack.io/git"] != "https://github.com" || string(git.Data[corev1.BasicAuthPasswordKey]) != "redacted-token" {
		t.Errorf("adapted git secret = %#v", git)
	}
	var sa corev1.ServiceAccount
	if err := o.Client.Get(context.Background(), client.ObjectKey{Namespace: o.Namespace, Name: kpackServiceAccountName(o)}, &sa); err != nil {
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

func TestKpackCredentialObjectsRejectMismatchedIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]string)
	}{
		{name: "uid", mutate: func(labels map[string]string) { labels["app.bex.co/app-uid"] = "uid-other" }},
		{name: "revision", mutate: func(labels map[string]string) { labels[kpackRevisionLabel] = "gen-other" }},
		{name: "purpose", mutate: func(labels map[string]string) { labels[kpackPurposeLabel] = kpackGitSecretPurpose }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := opts()
			labels := kpackArtifactLabels(o, kpackServiceAccountPurpose)
			tc.mutate(labels)
			existing := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
				Name: kpackServiceAccountName(o), Namespace: o.Namespace, Labels: labels,
			}}
			o.Client = fakeClient(existing)
			if err := ensureKpackCredentials(context.Background(), o); err == nil {
				t.Fatal("mismatched deterministic object was adopted")
			}
		})
	}
}

func TestBuildNilClient(t *testing.T) {
	o2 := opts() // nil client
	o2.Builder = BuilderDockerfile
	if _, err := EnsureBuild(context.Background(), o2); err == nil || !strings.Contains(err.Error(), "nil client") {
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

// TestActiveAppBuilds pins the per-App active-build count that gates the
// workspace cap without stalling an App's own in-flight build (ADR060 §D1a): a
// running Job counts, a complete/failed one does not, and — the round-5 finding-5
// cross-tenant guard — a same-named App in ANOTHER workspace (same build label,
// different UID) is never counted.
func TestActiveAppBuilds(t *testing.T) {
	o := opts()
	active := BuildJob(o, o.ImageRef()) // active: no conditions
	active.Name = JobName(o.Name, "gen-5")

	done := completedJob(o, batchv1.JobComplete)
	done.Name = JobName(o.Name, "gen-4")

	// A same-named App in ANOTHER workspace carries the same "app.bex.co/build"
	// label value in the shared build namespace but a distinct UID.
	foreign := BuildJob(o, o.ImageRef())
	foreign.Name = JobName(o.Name, "gen-9")
	foreign.Labels["app.bex.co/app-uid"] = "uid-foreign"

	cl := fakeClient(active, done, foreign)
	ctx := context.Background()

	n, err := ActiveAppBuilds(ctx, cl, o.Namespace, o.Name, o.AppUID)
	if err != nil {
		t.Fatalf("ActiveAppBuilds: %v", err)
	}
	if n != 1 {
		t.Errorf("ActiveAppBuilds = %d, want 1 (only this App's one running build; the completed one and the foreign-UID one excluded)", n)
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

	n, err := ActiveWorkspaceBuilds(ctx, cl, "default", "tea-x")
	if err != nil {
		t.Fatalf("ActiveWorkspaceBuilds: %v", err)
	}
	// 2 active for tea-x (done and other-workspace excluded).
	if n != 2 {
		t.Errorf("ActiveWorkspaceBuilds(tea-x) = %d, want 2", n)
	}

	// Empty workspace string is a no-op (returns 0, not an error).
	n, err = ActiveWorkspaceBuilds(ctx, cl, "default", "")
	if err != nil || n != 0 {
		t.Errorf("empty workspace: got (%d, %v), want (0, nil)", n, err)
	}
}

// buildPod is a pod as the Job controller would create it: `job-name` is the
// label kubelet/the Job controller stamp and what buildQueued selects on.
func buildPod(o Options, name, nodeName string, cond *corev1.PodCondition) *corev1.Pod {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      name,
		Namespace: o.Namespace,
		Labels:    map[string]string{"job-name": JobName(o.Name, o.Revision)},
	}}
	pod.Spec.NodeName = nodeName
	pod.Status.Phase = corev1.PodPending
	if cond != nil {
		pod.Status.Conditions = []corev1.PodCondition{*cond}
	}
	return pod
}

// TestBuildQueuedSeparatesWaitingForCapacityFromBuilding is the distinction the
// whole start-point fix rests on. A build requests 2 CPU + 7Gi and is
// node-exclusive by design, so it can sit unschedulable for many minutes; that
// wait must be reported as QUEUED so the control plane does not spend it from
// the build's own budget (2026-08-11: 22 minutes Pending ate most of the
// 35-minute build gate and a healthy, still-running build was reported failed).
func TestBuildQueuedSeparatesWaitingForCapacityFromBuilding(t *testing.T) {
	o := opts()
	job := BuildJob(o, o.ImageRef())
	unschedulable := &corev1.PodCondition{
		Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
		Reason: "Unschedulable", Message: "0/10 nodes are available: 3 Insufficient memory.",
	}

	t.Run("unplaced pod is queued and carries the scheduler's reason", func(t *testing.T) {
		o.Client = fakeClient(job, buildPod(o, "b-1", "", unschedulable))
		queued, reason := buildQueued(context.Background(), o, job.Name)
		if !queued {
			t.Fatal("a pod the scheduler could not place must report queued")
		}
		if !strings.Contains(reason, "Insufficient memory") {
			t.Errorf("reason = %q, want the scheduler's own message", reason)
		}
	})

	// The regression guard for the subtle half: the build runs in
	// initContainers, so a pod actively compiling reports phase Pending. Keying
	// on phase instead of placement would call a healthy mid-build pod queued
	// and hold the clock at zero for the entire build.
	t.Run("scheduled pod still Pending is building, not queued", func(t *testing.T) {
		o.Client = fakeClient(job, buildPod(o, "b-2", "node-a", nil))
		if queued, _ := buildQueued(context.Background(), o, job.Name); queued {
			t.Error("a placed pod is consuming capacity and building, even while phase is Pending")
		}
	})

	t.Run("a spent attempt does not mask the live one", func(t *testing.T) {
		dead := buildPod(o, "b-3", "node-a", nil)
		dead.Status.Phase = corev1.PodFailed
		o.Client = fakeClient(job, dead, buildPod(o, "b-4", "", unschedulable))
		if queued, _ := buildQueued(context.Background(), o, job.Name); !queued {
			t.Error("the retry is unplaced, so the build is queued regardless of the failed attempt")
		}
	})

	// Fail toward "building": guessing queued on absent evidence would stall the
	// caller's clock on something it does not actually know.
	t.Run("no pods yet is not reported as queued", func(t *testing.T) {
		o.Client = fakeClient(job)
		if queued, _ := buildQueued(context.Background(), o, job.Name); queued {
			t.Error("the no-pod-yet window must not be reported as queued")
		}
	})
}

func TestBuildJobDockerContextMovesOnlyTheContext(t *testing.T) {
	o := Options{
		Repo: "https://github.com/example/mono", Ref: "main", Name: "api",
		Registry: "zot:5000", Revision: "gen-1", Namespace: "bex-system",
		RootDir: "apps/api", DockerfilePath: "Dockerfile",
		DockerContext: "apps/api/build-ctx",
	}
	joinedArgs := func(o Options) string {
		job := BuildJob(o, o.ImageRef())
		bk := containerByName(job.Spec.Template.Spec.InitContainers, "buildkit")
		return strings.Join(bk.Args, " ")
	}

	joined := joinedArgs(o)
	if !strings.Contains(joined, "context=/source/apps/api/build-ctx") {
		t.Errorf("dockerContext must move the build context: %q", joined)
	}
	// The dockerfile local stays RootDir-derived — Docker's own
	// context-vs-dockerfile split (Render's dockerContext semantics).
	if !strings.Contains(joined, "dockerfile=/source/apps/api") {
		t.Errorf("dockerfile dir must stay RootDir-derived: %q", joined)
	}

	// A traversal context cleans to a repo-root-bounded path (the same
	// containment RootDir uses) instead of escaping the source mount.
	o.DockerContext = "../../escape"
	if joined := joinedArgs(o); !strings.Contains(joined, "context=/source/escape") {
		t.Errorf("traversal context must clean inside the source mount: %q", joined)
	}

	// Empty keeps the prior RootDir-derived context byte-identical.
	o.DockerContext = ""
	if joined := joinedArgs(o); !strings.Contains(joined, "context=/source/apps/api ") && !strings.Contains(joined, "context=/source/apps/api") {
		t.Errorf("empty dockerContext must keep the RootDir context: %q", joined)
	}
}

// --- w7/m82 t001: reserved exit codes ---------------------------------------

func TestClassifyPreludeUsesReservedCodes(t *testing.T) {
	// The prelude must exit with the reserved codes, not with literals that
	// could drift from the constants podFailurePolicy matches on.
	if !strings.Contains(classifyPrelude, "exit "+strconv.Itoa(ExitTenantError)) {
		t.Errorf("prelude does not exit %d for the tenant class:\n%s", ExitTenantError, classifyPrelude)
	}
	if !strings.Contains(classifyPrelude, "exit "+strconv.Itoa(ExitTransient)) {
		t.Errorf("prelude does not exit %d for the transient class:\n%s", ExitTransient, classifyPrelude)
	}
}

func TestReservedExitCodesCannotCollideWithSignalsOrShellErrors(t *testing.T) {
	// 126/127 are the shell's own "not executable"/"not found"; 128+N is a
	// signal exit (137 OOM, 143 SIGTERM). A reserved code landing in either
	// range would make a classified failure indistinguishable from one of
	// those, which is exactly what the classification exists to prevent.
	for name, code := range map[string]int{"ExitTenantError": ExitTenantError, "ExitTransient": ExitTransient} {
		if code >= 126 {
			t.Errorf("%s = %d must stay below 126 (shell errors) and 128 (signal exits)", name, code)
		}
		if code <= 1 {
			t.Errorf("%s = %d must not collide with success or a generic failure", name, code)
		}
	}
	if ExitTenantError == ExitTransient {
		t.Fatal("the two classes must be distinguishable")
	}
}

func TestClassifyingPhasesPassTenantArgsAsPositionalParameters(t *testing.T) {
	// SECURITY: the whole point of the "$@" form is that tenant-controlled
	// values reach the phase as discrete argv entries and never as script text.
	// A regression here would be a shell-injection surface, so assert the
	// tenant inputs are absent from the script and present in Args.
	o := Options{
		Name: "app", Revision: "r1", Repo: "https://github.com/o/r", Ref: "main",
		RootDir: "svc/api", DockerfilePath: "build/Dockerfile.prod",
	}
	j := BuildJob(o, "zot.local:5000/app:r1")
	var bk *corev1.Container
	for i := range j.Spec.Template.Spec.InitContainers {
		if j.Spec.Template.Spec.InitContainers[i].Name == "buildkit" {
			bk = &j.Spec.Template.Spec.InitContainers[i]
		}
	}
	if bk == nil {
		t.Fatal("buildkit container not found")
	}
	script := strings.Join(bk.Command, " ")
	for _, tenant := range []string{"svc/api", "build/Dockerfile.prod"} {
		if strings.Contains(script, tenant) {
			t.Errorf("tenant value %q was interpolated into the script text:\n%s", tenant, script)
		}
	}
	if !strings.Contains(script, `bex_run buildctl-daemonless.sh "$@"`) {
		t.Errorf("buildkit does not invoke buildctl through the positional form:\n%s", script)
	}
	joined := strings.Join(bk.Args, " ")
	if !strings.Contains(joined, "svc/api") || !strings.Contains(joined, "build/Dockerfile.prod") {
		t.Errorf("tenant values should reach buildctl via Args, got %q", joined)
	}
}

func TestBuildPhasesCaptureFailureTail(t *testing.T) {
	// Without FallbackToLogsOnError the controller can only report an exit
	// code, so the tenant sees a number instead of the error they caused.
	j := BuildJob(Options{Name: "app", Revision: "r1", Repo: "https://github.com/o/r", SignKeySecret: "signing"}, "img:1")
	spec := j.Spec.Template.Spec
	all := append(append([]corev1.Container{}, spec.InitContainers...), spec.Containers...)
	if len(all) < 3 {
		t.Fatalf("expected the signing layout to have several phases, got %d", len(all))
	}
	for _, c := range all {
		if c.TerminationMessagePolicy != corev1.TerminationMessageFallbackToLogsOnError {
			t.Errorf("phase %q has TerminationMessagePolicy %q, want FallbackToLogsOnError", c.Name, c.TerminationMessagePolicy)
		}
	}
}

func TestOnlyTenantInputPhasesClassify(t *testing.T) {
	// push and sign are deliberately left with their natural exit codes: after
	// push retries, a failure there is the platform's, and an unclassified
	// non-zero exit is what the backoff budget should retry. If they ever start
	// emitting ExitTenantError, a registry outage would be blamed on the tenant.
	j := BuildJob(Options{Name: "app", Revision: "r1", Repo: "https://github.com/o/r", SignKeySecret: "signing", PushSecret: "push"}, "img:1")
	spec := j.Spec.Template.Spec
	for _, c := range append(append([]corev1.Container{}, spec.InitContainers...), spec.Containers...) {
		// clone carries its script in Args, buildkit in Command; scan both so
		// the assertion does not depend on which slot a phase happens to use.
		classifies := strings.Contains(strings.Join(append(append([]string{}, c.Command...), c.Args...), " "), "bex_run")
		switch c.Name {
		case "clone", "buildkit":
			if !classifies {
				t.Errorf("phase %q must classify its failures", c.Name)
			}
		default:
			if classifies {
				t.Errorf("phase %q must NOT classify: a failure there is not the tenant's", c.Name)
			}
		}
	}
}

// --- w7/m82 t002: podFailurePolicy ------------------------------------------

func TestBuildPodFailurePolicyAbsorbsDisruptionAndFailsTenantErrors(t *testing.T) {
	j := BuildJob(opts(), "img:1")
	p := j.Spec.PodFailurePolicy
	if p == nil {
		t.Fatal("build Job has no podFailurePolicy: a node drain would fail the tenant's deploy")
	}
	var sawIgnoreDisruption, sawFailTenant bool
	for _, r := range p.Rules {
		for _, c := range r.OnPodConditions {
			if c.Type == corev1.DisruptionTarget && r.Action == batchv1.PodFailurePolicyActionIgnore {
				sawIgnoreDisruption = true
			}
		}
		if r.OnExitCodes != nil && r.Action == batchv1.PodFailurePolicyActionFailJob {
			for _, v := range r.OnExitCodes.Values {
				if v == ExitTenantError {
					sawFailTenant = true
				}
				if v == ExitTransient {
					t.Error("a transient failure must NOT FailJob: a registry blip would be reported as the tenant's fault")
				}
				if v == 137 {
					t.Error("OOM must not be in the FailJob set; it is bounded by the backoff budget, not fast-failed by exit code")
				}
			}
		}
	}
	if !sawIgnoreDisruption {
		t.Error("DisruptionTarget must be Ignored, or an autoscaler reclaim still burns the tenant's attempt budget")
	}
	if !sawFailTenant {
		t.Errorf("exit %d (tenant error) must FailJob, or deterministic failures keep retrying", ExitTenantError)
	}
}

func TestBuildJobNoLongerPinsNodesAgainstTheAutoscaler(t *testing.T) {
	// safe-to-evict:"false" prevented eviction because eviction used to fail the
	// build. Now the policy absorbs eviction, so the pin only blocks legitimate
	// node consolidation for the build's whole duration.
	j := BuildJob(opts(), "img:1")
	if v, ok := j.Spec.Template.Annotations["cluster-autoscaler.kubernetes.io/safe-to-evict"]; ok {
		t.Errorf("safe-to-evict is still set to %q; podFailurePolicy makes it unnecessary and it blocks consolidation", v)
	}
}

func TestBuildBackoffAllowsRetryOnlyForUnclassifiedFailures(t *testing.T) {
	j := BuildJob(opts(), "img:1")
	if j.Spec.BackoffLimit == nil || *j.Spec.BackoffLimit != 2 {
		t.Fatalf("backoffLimit = %v, want 2 (unclassified failures only)", j.Spec.BackoffLimit)
	}
	// The budget is only safe because tenant errors never reach it.
	if j.Spec.PodFailurePolicy == nil {
		t.Fatal("backoffLimit 2 without a podFailurePolicy would retry every tenant error twice")
	}
}

// --- w7/m82 t003: fault classification --------------------------------------

func TestFaultFromJobReadsTheClassOffTheJobReason(t *testing.T) {
	// The classification is derived from the Job's own failure reason, which
	// costs no extra API call: buildPodFailurePolicy's only FailJob rule matches
	// ExitTenantError, so "PodFailurePolicy" IS the statement that a
	// tenant-classified phase failed.
	for _, tc := range []struct {
		reason string
		want   Fault
	}{
		{batchv1.JobReasonPodFailurePolicy, FaultTenant},
		{batchv1.JobReasonBackoffLimitExceeded, FaultInfra},
		{batchv1.JobReasonDeadlineExceeded, FaultTimeout}, // its own class, not a fault of either side
		{"", FaultNone},
	} {
		if got := faultFromJob(tc.reason); got != tc.want {
			t.Errorf("faultFromJob(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}

func TestFaultClassesAreDistinct(t *testing.T) {
	// If tenant and infra ever collapse to the same value, the SLO silently
	// starts counting tenant errors against the platform's error budget.
	seen := map[Fault]bool{}
	for _, f := range []Fault{FaultNone, FaultTenant, FaultInfra, FaultTimeout} {
		if seen[f] {
			t.Fatalf("fault class %q is duplicated; the SLO would count one class as another", f)
		}
		seen[f] = true
	}
}

// --- w7/m82 t004: registry hardening ----------------------------------------

func TestPushRetriesTransientRegistryFailures(t *testing.T) {
	j := BuildJob(opts(), "img:1")
	var push *corev1.Container
	for i := range j.Spec.Template.Spec.Containers {
		if j.Spec.Template.Spec.Containers[i].Name == "push" {
			push = &j.Spec.Template.Spec.Containers[i]
		}
	}
	if push == nil {
		t.Fatal("push container not found")
	}
	args := strings.Join(push.Args, " ")
	if !strings.Contains(args, "--retry-times 3") {
		t.Errorf("push must retry transient registry failures, args = %q", args)
	}
	// Whole-blob retry only: resume paths are where registries corrupt uploads.
	if strings.Contains(args, "resume") {
		t.Errorf("push must not use chunked-upload resume, args = %q", args)
	}
}

func TestPushTLSVerificationIsConditional(t *testing.T) {
	// .pm/w1/046.md F11: --dest-tls-verify=false used to be unconditional, so
	// pointing BEX_REGISTRY at a real TLS registry silently pushed tenant
	// images, with the push credential, over an unverified connection.
	pushArgsFor := func(registry string) string {
		o := opts()
		o.Registry = registry
		j := BuildJob(o, registry+"/app:1")
		for _, c := range j.Spec.Template.Spec.Containers {
			if c.Name == "push" {
				return strings.Join(c.Args, " ")
			}
		}
		t.Fatalf("no push container for registry %q", registry)
		return ""
	}

	// Cluster-local / dev endpoints legitimately speak HTTP — byte-identical
	// to the shipped default.
	for _, local := range []string{
		"zot.bex-registry.svc:5000", "zot.bex-registry.svc.cluster.local:5000",
		"zot.local:5000", "localhost:5000", "127.0.0.1:5000", "zot:5000",
	} {
		if !strings.Contains(pushArgsFor(local), "--dest-tls-verify=false") {
			t.Errorf("registry %q is cluster-local and must keep plain HTTP push", local)
		}
	}
	// A real registry must be verified.
	for _, remote := range []string{
		"registry.example.com:5000", "ghcr.io", "index.docker.io", "eu.gcr.io",
	} {
		if strings.Contains(pushArgsFor(remote), "--dest-tls-verify=false") {
			t.Errorf("registry %q is external — TLS verification must NOT be disabled", remote)
		}
	}
}

// TestClassifyPreludeExecutes runs the prelude through a real shell under the
// same `sh -eu` the build phases use. Every other test here string-matches the
// script; this one is the only thing that would catch the script being subtly
// wrong, which is exactly what happened: without `set +e` the brace group aborts
// on failure, `echo $?` never runs, and the captured exit code is lost.
func TestClassifyPreludeExecutes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase string
		want  int
	}{
		{"success passes through", "exit 0", 0},
		{"tenant error by default", `echo "Dockerfile parse error" >&2; exit 1`, ExitTenantError},
		{"network failure classified transient", `echo "dial tcp 10.0.0.1:5000: connect: connection refused" >&2; exit 1`, ExitTransient},
		{"registry 5xx classified transient", `echo "unexpected status: 503 Service Unavailable" >&2; exit 1`, ExitTransient},
		{"bad git ref stays tenant", `echo "fatal: couldn't find remote ref refs/heads/nope" >&2; exit 128`, ExitTenantError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := classifyPrelude + `bex_run sh -c "$1"`
			cmd := exec.Command("sh", "-eu", "-c", script, "bex-test", tc.phase)
			cmd.Env = append(os.Environ(), "TMPDIR="+t.TempDir())
			out, err := cmd.CombinedOutput()
			got := 0
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				got = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("running prelude: %v (output %q)", err, out)
			}
			if got != tc.want {
				t.Errorf("exit = %d, want %d\nphase: %s\noutput:\n%s", got, tc.want, tc.phase, out)
			}
		})
	}
}

// TestClassifyPreludeSurvivesRepeatedUse pins the failure mode the set +e bug
// would have caused: a successful first call must not leave a stale 0 that makes
// a later failing call report success.
func TestClassifyPreludeSurvivesRepeatedUse(t *testing.T) {
	script := classifyPrelude + `bex_run true
bex_run sh -c 'echo "Dockerfile parse error" >&2; exit 1'
echo REACHED_END_WITHOUT_FAILING`
	cmd := exec.Command("sh", "-eu", "-c", script, "bex-test")
	cmd.Env = append(os.Environ(), "TMPDIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if strings.Contains(string(out), "REACHED_END_WITHOUT_FAILING") {
		t.Fatalf("a failing phase after a successful one reported success:\n%s", out)
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != ExitTenantError {
		t.Errorf("second (failing) call exit = %v, want %d\noutput:\n%s", err, ExitTenantError, out)
	}
}

// TestTenantOutputCannotForgeAnInfraClassification closes an abuse vector the
// classification opened: the buildkit phase's log is tenant-authored, so a
// Dockerfile could print a network-error string and have its own deterministic
// failure classified as transient — earning a free retry and, once the budget
// is spent, landing in infra_failed where it moves the platform SLO and feeds
// the correlated-failure page. BuildKit prefixes tenant RUN output with a step
// marker, so classification excludes those lines.
func TestTenantOutputCannotForgeAnInfraClassification(t *testing.T) {
	run := func(phase string) int {
		script := classifyPrelude + `bex_run sh -c "$1"`
		cmd := exec.Command("sh", "-eu", "-c", script, "bex-test", phase)
		cmd.Env = append(os.Environ(), "TMPDIR="+t.TempDir())
		out, err := cmd.CombinedOutput()
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		if err != nil {
			t.Fatalf("running prelude: %v (%s)", err, out)
		}
		return 0
	}

	// A tenant RUN step printing a network error: BuildKit stamps it with a step
	// marker, so it must NOT buy the tenant a transient classification.
	forged := `echo '#12 0.482 connection refused'; echo '#12 ERROR: process did not complete successfully'; exit 1`
	if got := run(forged); got != ExitTenantError {
		t.Errorf("tenant-forged network text classified as %d, want %d (tenant): a Dockerfile can move the platform SLO", got, ExitTenantError)
	}

	// The toolchain's own unprefixed error must still classify as transient.
	real := `echo 'ERROR: failed to solve: failed to do request: dial tcp 10.0.0.1:5000: connect: connection refused' >&2; exit 1`
	if got := run(real); got != ExitTransient {
		t.Errorf("genuine registry failure classified as %d, want %d (transient)", got, ExitTransient)
	}
}

// TestClassifyBufferIsBounded pins the ephemeral-storage bound. An unbounded
// classification log shares the phase's ephemeral-storage limit, and exceeding
// that limit evicts the pod with DisruptionTarget set — which podFailurePolicy
// Ignores, turning a deterministic disk-filling build into a retry loop that
// ends only at activeDeadlineSeconds.
func TestClassifyBufferIsBounded(t *testing.T) {
	if classifyBufferBytes <= 0 || classifyBufferBytes > 4*1024*1024 {
		t.Fatalf("classifyBufferBytes = %d; must be positive and small next to the ephemeral limit", classifyBufferBytes)
	}
	if !strings.Contains(classifyPrelude, "tail -c") {
		t.Error("the prelude must bound what it retains for classification")
	}

	script := classifyPrelude + `bex_run sh -c "$1"`
	// Emit far more than the buffer, ending with a transient marker so we can
	// prove the tail (not the head) is what survives.
	phase := `i=0; while [ $i -lt 60000 ]; do echo "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"; i=$((i+1)); done; echo "dial tcp: connection refused" >&2; exit 1`
	cmd := exec.Command("sh", "-eu", "-c", script, "bex-test", phase)
	cmd.Env = append(os.Environ(), "TMPDIR="+t.TempDir())
	_, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != ExitTransient {
		t.Errorf("classification over a large log = %v, want exit %d from the retained tail", err, ExitTransient)
	}
}
