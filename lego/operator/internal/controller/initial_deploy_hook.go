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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The initial deploy hook (w33/m435, docs/ADR004-app-deployment.md § Initial
// deploy hook): Render runs a Blueprint's initialDeployHook once, after the
// service's first successful deploy. Here it is a one-time Job of the
// pre-deploy plane that starts only once a release is live and runs as that
// release's own instance does: its image, env, secrets and disk. It gates
// nothing. A run that fails leaves the deploy live and the hook owed; the next
// release to go live runs it again. One success sets
// status.initialDeployHookCompleted and nothing selects it again.

// initialDeployHookPoll is how soon a live pass looks at a running hook again.
// The hook pod's own events already bring a pass; this bounds a missed one.
const initialDeployHookPoll = 5 * time.Second

// ranHookAsStep reports a pre-deploy Job that an operator from before
// w33/m435 created: it ran the initial hook as the release's pre-deploy step
// whenever the hook was owed, and stamped no step label. A release caught
// mid-step by the upgrade keeps that rule to its end, so the hook is neither
// lost nor run twice.
func ranHookAsStep(app *appv1alpha1.App, job *batchv1.Job) bool {
	return app.OwedInitialDeployHook() != "" && job.Labels[predeploy.LabelStep] == ""
}

// legacyHookStep reports a release with no pre-deploy command whose
// generation nevertheless has a step on record while the hook is owed: the
// hook, run as the step by an older operator. Its verdict still stands
// (a failure keeps the release failed, a running Job is watched), and the next
// release runs under today's rule.
func legacyHookStep(app *appv1alpha1.App) bool {
	if app.OwedInitialDeployHook() == "" || app.PreDeployStepCommand() != "" {
		return false
	}
	pd := app.Status.PreDeploy
	return pd != nil && pd.Generation == releaseGeneration(app)
}

// observeInitialDeployHook settles a recorded running hook from its Job and
// reports whether it is still running. Every pass calls it, live or not: a
// hook left running through a suspend, a newer release's build or a held
// rollout must have its end read before the Job's TTL reaps it. A Job that is
// gone reads as not running; the live pass decides what that means.
func (r *AppReconciler) observeInitialDeployHook(ctx context.Context, app *appv1alpha1.App) bool {
	hook := app.Status.InitialDeployHook
	if hook == nil || hook.Status != appv1alpha1.PreDeployRunning || hook.Job == "" {
		return false
	}
	job := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: hook.Job}, job); err != nil {
		if apierrors.IsNotFound(err) {
			return false
		}
		logf.FromContext(ctx).Error(err, "read the initial deploy hook's Job", "app", app.Name, "job", hook.Job)
		return true
	}
	ended, ok := preDeployEnded(ctx, r.Client, job, *hook)
	if !ok {
		// A pull that cannot succeed never runs the command: it ends the run
		// now, as it ends a pre-deploy step.
		msg, err := r.endedByPullFailure(ctx, job)
		if err != nil || msg == "" {
			return true
		}
		ended = *failedStep(hook.Job, hook.Generation, hook.StartedAt, msg)
	}
	if err := r.patchAppStatus(ctx, app, func(status *appv1alpha1.AppStatus) {
		status.InitialDeployHook = &ended
		if ended.Status == appv1alpha1.PreDeploySucceeded {
			status.InitialDeployHookCompleted = true
		}
	}); err != nil {
		// Unrecorded: the Job is still there for the next pass to read. A
		// conflict is only a cache behind this App's last write.
		if !apierrors.IsConflict(err) {
			logf.FromContext(ctx).Error(err, "record the initial deploy hook's end", "app", app.Name, "job", hook.Job)
		}
		return true
	}
	logf.FromContext(ctx).Info("initial deploy hook ended", "name", app.Name, "job", hook.Job, "status", ended.Status)
	return false
}

// reconcileInitialDeployHook starts the owed hook once dep serves the current
// release, and returns how soon to look again (zero when nothing is running).
// It never fails the pass: the release is live whatever the hook does.
//
// One Job per release generation: a hook that ended for this release is never
// started again for it, and one still running for an earlier release is left
// to finish before another starts.
func (r *AppReconciler) reconcileInitialDeployHook(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, image string) time.Duration {
	if r.observeInitialDeployHook(ctx, app) {
		return initialDeployHookPoll
	}
	command := app.OwedInitialDeployHook()
	// A cancel settling back onto the served release reaches here too, on that
	// release's restored template: no release went live, so nothing starts.
	if command == "" || canceledOverServed(app) || !runsActiveRelease(app, dep) || diskBusy(app) {
		return 0
	}
	gen := releaseGeneration(app)
	started := time.Now().UTC().Format(time.RFC3339)
	first := true
	if hook := app.Status.InitialDeployHook; hook != nil && hook.Generation == gen {
		if hook.Status != appv1alpha1.PreDeployRunning {
			return 0 // it ended for this release: the next one to go live runs it again
		}
		// Recorded running, yet its Job did not read: the record is ahead of
		// the cache, or the Job was reaped unread. Ensure settles which.
		first = false
		if hook.StartedAt != "" {
			started = hook.StartedAt
		}
	}
	opts, ok := r.hookJobOptions(app, dep, image, command, gen)
	if !ok {
		return 0
	}
	jobName := predeploy.HookJobName(app.Name, opts.Revision)
	// record writes the run to status. An unwritten record is asked for again
	// on the next pass, which finds the same Job by its name.
	record := func(hook *appv1alpha1.PreDeployStatus, site string) time.Duration {
		err := r.patchAppStatus(ctx, app, func(status *appv1alpha1.AppStatus) { status.InitialDeployHook = hook })
		if err == nil {
			return 0
		}
		if !apierrors.IsConflict(err) {
			logf.FromContext(ctx).Error(err, "record the initial deploy hook", "app", app.Name, "site", site)
		}
		return initialDeployHookPoll
	}
	if first {
		switch refused, err := r.outOfBuildMinutes(ctx, app, jobName); {
		case err != nil:
			return initialDeployHookPoll
		case refused:
			return record(failedStep(jobName, gen, started, appv1alpha1.BuildMinutesExhaustedMessage), "build-minutes")
		}
	}
	job, err := predeploy.Ensure(ctx, opts)
	if err != nil {
		// A refusal by the API server will not pass on a retry: it ends this
		// release's run, worded as the platform's fault (w9/090), instead of
		// being asked again every few seconds for as long as the release lives.
		if shown, refused := customerFailure(ctx, app, "InitialDeployHookFailed", err); refused {
			return record(failedStep(jobName, gen, started, "the initial deploy hook could not be started: "+shown), "refused")
		}
		logf.FromContext(ctx).Error(err, "start the initial deploy hook", "app", app.Name, "job", jobName)
		return initialDeployHookPoll
	}
	if first {
		logf.FromContext(ctx).Info("initial deploy hook started", "name", app.Name, "job", job.Name, "releaseGeneration", gen)
	}
	running := runningStep(job.Name, gen, started)
	if retry := record(&running, "started"); retry > 0 {
		return retry
	}
	// An adopted Job may already have ended: read it on this pass. One just
	// created is not in the cache yet, and reads as running all the same.
	r.observeInitialDeployHook(ctx, app)
	if app.Status.InitialDeployHook.Status == appv1alpha1.PreDeployRunning {
		return initialDeployHookPoll
	}
	return 0
}

// hookJobOptions is the hook's Job for release gen, built from the pod
// template dep serves that release with, so the command sees what the
// service's own instance sees: the same image, env, secret references, pull
// secrets, security context and resources, its secret files, and its
// persistent disk at the same mount path. ok is false when the template has no
// app container to copy.
func (r *AppReconciler) hookJobOptions(app *appv1alpha1.App, dep *appsv1.Deployment, image, command string, gen int64) (predeploy.Options, bool) {
	pod := dep.Spec.Template.Spec
	var instance *corev1.Container
	for i := range pod.Containers {
		if pod.Containers[i].Name == appContainerName {
			instance = &pod.Containers[i]
		}
	}
	if instance == nil {
		return predeploy.Options{}, false
	}
	opts := r.stepJobOptions(app, gen)
	opts.Step = predeploy.StepInitialDeployHook
	opts.Image = instance.Image
	opts.Command = preDeployArgv(app, image, command)
	opts.Env = instance.Env
	opts.EnvFrom = instance.EnvFrom
	opts.ImagePullSecrets = pod.ImagePullSecrets
	opts.SecurityContext = instance.SecurityContext
	opts.Resources = instance.Resources
	volumes := make(map[string]corev1.Volume, len(pod.Volumes))
	for _, v := range pod.Volumes {
		volumes[v.Name] = v
	}
	for _, mount := range instance.VolumeMounts {
		volume, ok := volumes[mount.Name]
		switch {
		case !ok:
			continue
		case volume.PersistentVolumeClaim != nil:
			// The service's disk. The claim is ReadWriteOnce, so the Job sits
			// on the instance's node and mounts it as the instance does.
			opts.HostUsers = pod.HostUsers
		case volume.Projected != nil, volume.Secret != nil:
			// Secret files.
		default:
			continue // the instance's own scratch (the health-probe helper)
		}
		opts.Volumes = append(opts.Volumes, volume)
		opts.VolumeMounts = append(opts.VolumeMounts, mount)
	}
	return opts, true
}

// diskBusy reports a disk the hook must stay off: a restore the owner asked
// for has not been served, or a purge of its snapshots is pending. Both own
// the volume's contents until they end.
func diskBusy(app *appv1alpha1.App) bool {
	return app.Spec.Disk != nil && (app.Annotations[annotDiskPurgePending] != "" || diskRestoreOwed(app))
}

// initialDeployHookHoldsDisk reports a hook Job of app that has not ended. It
// mounts the service's disk, so a restore waits for it instead of rewriting
// the volume under it. The Job is found by label, not by status: a Job whose
// status write was lost still holds the volume.
func (r *AppReconciler) initialDeployHookHoldsDisk(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(app.Namespace), client.MatchingLabels{
		predeploy.LabelService: app.Name,
		predeploy.LabelStep:    string(predeploy.StepInitialDeployHook),
	}); err != nil {
		return false, err
	}
	owner := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID)}
	for i := range jobs.Items {
		if owner.Owns(&jobs.Items[i]) && !execution.JobFinished(&jobs.Items[i]) {
			return true, nil
		}
	}
	return false, nil
}
