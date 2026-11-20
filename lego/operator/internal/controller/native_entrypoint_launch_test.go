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
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/350 fixtures: a native cron job with its own command and a secret file.
const (
	w350Repo    = "https://github.com/acme/jobs"
	w350Native  = "zot.local:5000/jobs:gen-3@sha256:abc"
	w350Docker  = "zot.local:5000/jobs:gen-1@sha256:def"
	w350Start   = "node main.js"
	w350Command = "node report.js"
)

func w350Cron(mutate func(*appv1alpha1.AppSpec)) *appv1alpha1.App {
	return projectionApp(func(a *appv1alpha1.App) {
		a.Spec = appv1alpha1.AppSpec{Type: appv1alpha1.TypeCronJob, Schedule: "0 * * * *", Repo: w350Repo,
			Runtime: "node", BuildCommand: "npm ci", StartCommand: w350Start, Command: w350Command,
			FilesFromSecrets: []string{"hello-files"}}
		if mutate != nil {
			mutate(&a.Spec)
		}
	})
}

func w350CronContainer(app *appv1alpha1.App, image string, launch launchMode) corev1.Container {
	return (&AppReconciler{}).cronPodSpec(app, image, 0, nil, launch, "").Spec.Containers[0]
}

// TestNativeCronCommandRunsUnderItsEntrypoint is w8/350. A native cron job's
// own command replaced the image's entrypoint, native-init, so a run never
// linked the secret files mounted for it into the service root and saw only
// the names its image was built with. It now runs as bash arguments to that
// entrypoint, as the image's start command and a cron linked to a native
// build source do; every other image keeps the shell Command.
func TestNativeCronCommandRunsUnderItsEntrypoint(t *testing.T) {
	native := w350Cron(nil)
	c := w350CronContainer(native, w350Native, launchUnknown)
	if len(c.Command) != 0 || !slices.Equal(c.Args, nativeStartArgs(w350Command)) {
		t.Errorf("native cron command %q args %q, want no command and args %q", c.Command, c.Args, nativeStartArgs(w350Command))
	}
	if len(c.VolumeMounts) != 1 || c.VolumeMounts[0].MountPath != secretFilesMountPath {
		t.Errorf("native cron mounts = %+v, want its secret files at %s", c.VolumeMounts, secretFilesMountPath)
	}

	for name, app := range map[string]*appv1alpha1.App{
		"docker":    w350Cron(func(s *appv1alpha1.AppSpec) { s.Runtime = "docker" }),
		"buildpack": w350Cron(func(s *appv1alpha1.AppSpec) { s.Runtime, s.Builder = "", build.BuilderBuildpack }),
		"the saved prebuilt image with native settings": w350Cron(func(s *appv1alpha1.AppSpec) { s.Repo, s.Image = "", w350Docker }),
	} {
		c := w350CronContainer(app, w350Docker, launchUnknown)
		if !slices.Equal(c.Command, shellCommand(w350Command)) || len(c.Args) != 0 {
			t.Errorf("%s cron command %q args %q, want the shell Command", name, c.Command, c.Args)
		}
	}

	baked := w350CronContainer(w350Cron(func(s *appv1alpha1.AppSpec) { s.Command = "" }), w350Native, launchUnknown)
	if len(baked.Command) != 0 || len(baked.Args) != 0 {
		t.Errorf("native cron without its own command: command %q args %q, want the image's CMD", baked.Command, baked.Args)
	}
}

// A cron's own command is read back from its release record, so a Restart or
// rollback runs the selected image the way that release ran it, across a
// runtime change too.
func TestCronCommandRecordKeepsItsLaunch(t *testing.T) {
	record := func(c corev1.Container) *runtimeConfigRecord {
		c.Name = appContainerName
		return &runtimeConfigRecord{
			spec:     appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: w350Start, Command: new(w350Command)},
			template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{c}}},
		}
	}
	native := w350Cron(nil)
	docker := w350Cron(func(s *appv1alpha1.AppSpec) { s.Runtime, s.StartCommand = "docker", "" })
	cases := []struct {
		name  string
		saved *appv1alpha1.App
		rec   *runtimeConfigRecord
		image string
		want  launchMode
	}{
		{"a native release", native, record(w350CronContainer(native, w350Native, launchUnknown)), w350Native, launchNativeArgs},
		{"a Docker release selected after a switch to native", native, record(w350CronContainer(docker, w350Docker, launchUnknown)), w350Docker, launchShell},
		{"a native release selected after a switch to Docker", docker, record(w350CronContainer(native, w350Native, launchUnknown)), w350Native, launchNativeArgs},
		{"a native release recorded before w8/350", native, record(corev1.Container{Command: shellCommand(w350Command)}), w350Native, launchShell},
		{"a record with no template", native, &runtimeConfigRecord{spec: appv1alpha1.ReleaseRecordSpec{Version: 1, Command: new(w350Command)}}, w350Native, launchUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := tc.saved.DeepCopy()
			app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 9, SourceGeneration: 4, Image: tc.image}
			app.Status.ReleaseGeneration = 9
			r := m202Reconciler(t, app, 9, tc.rec)
			runtime, launch, err := r.selectedRuntime(context.Background(), app)
			if err != nil {
				t.Fatal(err)
			}
			if launch != tc.want {
				t.Fatalf("launch = %q, want %q", launch, tc.want)
			}
			c := w350CronContainer(runtime, tc.image, launch)
			if tc.want == launchNativeArgs {
				if len(c.Command) != 0 || !slices.Equal(c.Args, nativeStartArgs(w350Command)) {
					t.Errorf("command %q args %q, want the native entrypoint's args", c.Command, c.Args)
				}
			} else if tc.want == launchShell && (!slices.Equal(c.Command, shellCommand(w350Command)) || len(c.Args) != 0) {
				t.Errorf("command %q args %q, want the shell Command", c.Command, c.Args)
			}
		})
	}
}

// The initial deploy hook starts as the live instance does: under a native
// image's entrypoint, or in place of the program a Docker Command overrides.
func TestInitialDeployHookStartsAsItsInstance(t *testing.T) {
	const hook = "node seed.js"
	shell := []string{"/bin/sh", "-c", hook}
	native := projectionApp(func(a *appv1alpha1.App) {
		a.Spec = appv1alpha1.AppSpec{Repo: w350Repo, Runtime: "node", BuildCommand: "npm ci", StartCommand: w350Start}
	})
	docker := projectionApp(func(a *appv1alpha1.App) {
		a.Spec = appv1alpha1.AppSpec{Repo: w350Repo, Runtime: "docker", StartCommand: "/server --port 10000"}
	})
	for _, tc := range []struct {
		name              string
		app               *appv1alpha1.App
		instance          corev1.Container
		wantCmd, wantArgs []string
	}{
		{"native image", native, corev1.Container{}, nil, shell},
		{"native rerun with its start command as args", native, corev1.Container{Args: nativeStartArgs(w350Start)}, nil, shell},
		{"Docker Command", docker, corev1.Container{Command: shellCommand(docker.Spec.StartCommand)}, []string{"node", "seed.js"}, nil},
		{"a Docker image served under a native runtime", native, corev1.Container{Command: shellCommand("/server")}, []string{"node", "seed.js"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.instance.Name, tc.instance.Image = appContainerName, w350Native
			dep := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{tc.instance}}}}}
			opts, ok := (&AppReconciler{}).hookJobOptions(tc.app, dep, w350Native, hook, 1)
			if !ok {
				t.Fatal("no app container")
			}
			if !slices.Equal(opts.Command, tc.wantCmd) || !slices.Equal(opts.Args, tc.wantArgs) {
				t.Errorf("command %q args %q, want command %q args %q", opts.Command, opts.Args, tc.wantCmd, tc.wantArgs)
			}
		})
	}
}
