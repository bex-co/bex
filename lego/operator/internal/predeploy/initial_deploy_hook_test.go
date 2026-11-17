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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w33/m435: the one-time initialDeployHook is a second command of the
// pre-deploy plane. Its Job keeps everything the admission policy, the log
// shipper and the build-minutes meter key on (name prefix, component, service
// label, container name) and differs only where the hook does.

func hookOptions() Options {
	return Options{
		Name: "api", AppUID: "uid-api", Namespace: "tea-1", AppNamespace: "tea-1", Workspace: "tea-1",
		Step: StepInitialDeployHook, Image: "zot/api:gen-3", Command: []string{"/bin/sh", "-c", "npm run seed"},
		Revision: "gen-3", Generation: 3,
	}
}

func TestHookJobKeepsThePreDeployPlanesIdentity(t *testing.T) {
	hook, step := Job(hookOptions()), Job(func() Options { o := hookOptions(); o.Step = ""; return o }())

	if hook.Name != "predeploy-api-gen-3-hook" || step.Name != "predeploy-api-gen-3" {
		t.Fatalf("names = %q and %q, want the hook's own name beside the step's", hook.Name, step.Name)
	}
	if hook.Labels[LabelStep] != string(StepInitialDeployHook) ||
		hook.Spec.Template.Labels[LabelStep] != string(StepInitialDeployHook) ||
		step.Labels[LabelStep] != string(StepPreDeploy) {
		t.Fatalf("step labels: hook %v, pod %v, pre-deploy %v", hook.Labels, hook.Spec.Template.Labels, step.Labels)
	}
	// What the cluster keys on stays the pre-deploy plane's: the workload
	// admission policy admits a widened container policy only for a Job named
	// predeploy-* whose pod is component=predeploy with one container
	// "predeploy"; the log shipper and the build-minutes meter select the
	// component and service labels; the revision label is the release's.
	pod := hook.Spec.Template
	if !strings.HasPrefix(hook.Name, "predeploy-") || pod.Labels[LabelComponent] != ComponentValue ||
		pod.Labels[LabelService] != "api" || len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Name != containerName ||
		hook.Labels[appv1alpha1.LabelBuildRevision] != "gen-3" {
		t.Fatalf("hook Job %q labels %v pod labels %v containers %v", hook.Name, hook.Labels, pod.Labels, pod.Spec.Containers)
	}
	// One run, bounded like a pre-deploy command.
	if *hook.Spec.BackoffLimit != 0 || *hook.Spec.ActiveDeadlineSeconds != *step.Spec.ActiveDeadlineSeconds {
		t.Fatalf("hook backoffLimit %d deadline %d, want one bounded run", *hook.Spec.BackoffLimit, *hook.Spec.ActiveDeadlineSeconds)
	}
	assertExecutionPodDefaults(t, pod.Spec)
	if pod.Spec.Affinity != nil {
		t.Fatalf("a hook with no disk carries affinity %+v", pod.Spec.Affinity)
	}
	// Free to schedule anywhere on the pool, it keeps the tier's full size.
	o := hookOptions()
	tier := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("512Mi")}
	o.Resources = corev1.ResourceRequirements{Requests: tier, Limits: tier}
	if got := Job(o).Spec.Template.Spec.Containers[0].Resources; !got.Requests.Cpu().Equal(resource.MustParse("500m")) {
		t.Fatalf("a hook with no disk requests %+v, want the tier's size", got.Requests)
	}
}

func TestHookJobSharingTheDiskSitsBesideTheInstanceAndMountsAsItDoes(t *testing.T) {
	for name, hostUsers := range map[string]*bool{"host users, as a strict service runs": nil, "a pod user namespace, as a render-v1 service runs": new(false)} {
		t.Run(name, func(t *testing.T) {
			o := hookOptions()
			o.HostUsers = hostUsers
			o.ServingPool = "xl"
			o.Volumes = []corev1.Volume{{Name: "disk", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "disk-api"}}}}
			o.VolumeMounts = []corev1.VolumeMount{{Name: "disk", MountPath: "/var/data"}}
			tier := corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("512Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			}
			o.Resources = corev1.ResourceRequirements{Requests: tier, Limits: tier}
			pod := Job(o).Spec.Template.Spec

			// The one node it can run on is bin-packed: it reserves next to
			// nothing there and runs up to the tier's limits.
			got := pod.Containers[0].Resources
			if !got.Limits.Cpu().Equal(resource.MustParse("500m")) || !got.Limits.Memory().Equal(resource.MustParse("512Mi")) ||
				!got.Requests.Cpu().IsZero() || !got.Requests.Memory().Equal(resource.MustParse("64Mi")) ||
				!got.Requests.StorageEphemeral().IsZero() || len(got.Requests) != len(got.Limits) {
				t.Fatalf("disk-sharing hook resources = %+v, want the tier's limits over explicit near-zero requests", got)
			}

			if pod.HostUsers != hostUsers {
				t.Fatalf("hostUsers = %v, want the instance's own %v: a mismatch mounts the volume differently, and local-path refuses an idmapped mount", pod.HostUsers, hostUsers)
			}
			if len(pod.NodeSelector) != 0 {
				t.Fatalf("nodeSelector = %v, want none: the instance's node is the only one that can mount the volume", pod.NodeSelector)
			}
			if pod.Affinity == nil || pod.Affinity.PodAffinity == nil || len(pod.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 1 {
				t.Fatalf("affinity = %+v, want a required pod affinity", pod.Affinity)
			}
			// The dedicated serving pool's node affinity and toleration stay.
			if pod.Affinity.NodeAffinity == nil || len(pod.Tolerations) != 1 {
				t.Fatalf("serving pool placement lost: affinity %+v tolerations %v", pod.Affinity, pod.Tolerations)
			}
			term := pod.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0]
			selector, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
			if err != nil {
				t.Fatal(err)
			}
			instance := labels.Set{appv1alpha1.LabelApp: "api", appv1alpha1.LabelRevision: "rev-3"}
			sibling := labels.Set{appv1alpha1.LabelApp: "other", appv1alpha1.LabelRevision: "rev-3"}
			own := labels.Set(Job(o).Spec.Template.Labels)
			if term.TopologyKey != corev1.LabelHostname || !selector.Matches(instance) || selector.Matches(sibling) || selector.Matches(own) {
				t.Fatalf("affinity term %+v must select only this service's instances, never another service's pods or a Job's own", term)
			}
			if pod.Volumes[0].PersistentVolumeClaim.ClaimName != "disk-api" || pod.Containers[0].VolumeMounts[0].MountPath != "/var/data" {
				t.Fatalf("disk not threaded: %v %v", pod.Volumes, pod.Containers[0].VolumeMounts)
			}
		})
	}
}

// A newer release's pre-deploy step cancels an older step still in flight. It
// never cancels the hook: a one-time seed stopped halfway is worse than one
// that finishes, and nothing waits on it.
func TestCancelSupersededNeverCancelsARunningHook(t *testing.T) {
	ctx := context.Background()
	hook := activeJob("predeploy-api-gen-1-hook")
	hook.Labels["app.bex.co/app-uid"] = "uid-api"
	hook.Labels[LabelStep] = string(StepInitialDeployHook)
	old := activeJob("predeploy-api-gen-1")
	old.Labels["app.bex.co/app-uid"] = "uid-api"
	old.Labels[LabelStep] = string(StepPreDeploy)
	cl := fakeClient(hook, old)

	if err := CancelSuperseded(ctx, "api", "uid-api", "builds", "predeploy-api-gen-2", cl); err != nil {
		t.Fatalf("CancelSuperseded: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(hook), &batchv1.Job{}); err != nil {
		t.Fatalf("the running hook was canceled: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(old), &batchv1.Job{}); err == nil {
		t.Fatal("the superseded pre-deploy step was kept")
	}
}

// A failed hook is worded as the hook: the deploy's log names what failed and
// its exit code, and sends nobody to a pre-deploy log filter.
func TestHookFailureMessageNamesTheHook(t *testing.T) {
	hookJob := func(reason string) *batchv1.Job {
		job := failedJob(reason)
		job.Name = "predeploy-web-gen-3-hook"
		job.Labels = map[string]string{LabelStep: string(StepInitialDeployHook)}
		return job
	}
	startError := corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
		ExitCode: 128, Reason: "StartError",
		Message: `failed to create containerd task: exec: "/bin/sh": stat /bin/sh: no such file or directory`,
	}}
	for name, tc := range map[string]struct {
		job   *batchv1.Job
		state *corev1.ContainerState
		want  string
	}{
		"exit code":     {hookJob("BackoffLimitExceeded"), new(terminated(3, "Error")), "the initial deploy hook exited with code 3"},
		"out of memory": {hookJob("BackoffLimitExceeded"), new(terminated(137, "OOMKilled")), "the initial deploy hook ran out of memory (exit code 137)"},
		"window":        {hookJob(batchv1.JobReasonDeadlineExceeded), nil, "the initial deploy hook did not finish within its 30-minute window"},
		"no exit code":  {hookJob("BackoffLimitExceeded"), nil, "the initial deploy hook failed"},
		"never started": {hookJob("BackoffLimitExceeded"), &startError,
			`the initial deploy hook could not start: the program "/bin/sh" does not exist in this image; the program never ran, so it wrote no output`},
		"image pull": {hookJob("BackoffLimitExceeded"), new(pullWaiting("ErrImagePull")), ""},
	} {
		t.Run(name, func(t *testing.T) {
			objs := []client.Object{tc.job}
			if tc.state != nil {
				objs = append(objs, jobPod(jobUID, tc.job.Namespace, *tc.state))
			}
			got := FailureMessage(context.Background(), fakeClient(objs...), tc.job)
			if strings.Contains(got, "pre-deploy") {
				t.Fatalf("message %q still names the pre-deploy command", got)
			}
			if tc.want != "" && got != tc.want {
				t.Fatalf("message = %q, want %q", got, tc.want)
			}
			if tc.want == "" && !strings.HasSuffix(got, "; the initial deploy hook never ran") {
				t.Fatalf("message = %q, want an image failure that says the hook never ran", got)
			}
		})
	}

	// The grace-window pull failure the controller reads while the Job is
	// still pending is worded the same way.
	job := hookJob("")
	job.Status.Conditions = nil
	pod := jobPod(jobUID, job.Namespace, pullWaiting("ImagePullBackOff"))
	pod.CreationTimestamp = metav1.NewTime(time.Now().Add(-10 * time.Minute))
	if got := PullFailure(context.Background(), fakeClient(job, pod), job, time.Now()); got == "" || strings.Contains(got, "pre-deploy") {
		t.Fatalf("pending pull failure = %q, want it to name the hook", got)
	}
}
