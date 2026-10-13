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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/operator/internal/build"
)

const (
	m202Busybox  = "docker.io/library/busybox:1.37"
	m202Native   = "zot.local:5000/hello:gen-3@sha256:abc"
	m202Repo     = "https://github.com/bex-co/bex"
	m202Command  = `sh -c 'exec httpd -f -p 3000 -h /tmp/www'`
	m202Baked    = "node main.js"
	m202Explicit = "echo explicit"
)

var m202Shell = []string{shellBinary, "-c", m202Command}

// m202Commands returns what the Deployment's and the cron job's app container
// would execute for app running image with the given recorded launch.
func m202Commands(t *testing.T, app *appv1alpha1.App, image string, launch launchMode) (web, cron []string) {
	t.Helper()
	p := webParams()
	p.image, p.launch = image, launch
	web = appContainerOf(t, project(app, p)).Command
	cron = (&AppReconciler{}).cronPodSpec(app, image, 0, nil, launch).Spec.Containers[0].Command
	return web, cron
}

func m202Expect(t *testing.T, web, cron, want []string) {
	t.Helper()
	if !slices.Equal(web, want) {
		t.Errorf("Deployment command = %q, want %q", web, want)
	}
	if !slices.Equal(cron, want) {
		t.Errorf("cron fallback command = %q, want %q", cron, want)
	}
}

// TestPrebuiltImageCommandAfterNativeSourceSwitch is w4/m202. A Node service
// switched to the BusyBox image through Settings → Source keeps runtime node /
// builder native for a later switch back, and both command projections skipped
// its saved Docker Command — the container ran BusyBox's default and exited 0
// in a loop (QA a311). The saved prebuilt image now runs the command whatever
// build settings it retains; genuine native/buildpack artifacts keep their
// baked command.
func TestPrebuiltImageCommandAfterNativeSourceSwitch(t *testing.T) {
	cases := []struct {
		name  string
		spec  appv1alpha1.AppSpec
		image string
		want  []string
	}{
		{"image + retained node/native", appv1alpha1.AppSpec{Image: m202Busybox, Runtime: "node", Builder: build.BuilderNative, RootDir: "examples/hello-node", StartCommand: m202Command}, m202Busybox, m202Shell},
		{"image + retained buildpack", appv1alpha1.AppSpec{Image: m202Busybox, Builder: build.BuilderBuildpack, StartCommand: m202Command}, m202Busybox, m202Shell},
		{"image/auto control", appv1alpha1.AppSpec{Image: m202Busybox, Runtime: "image", Builder: build.BuilderAuto, StartCommand: m202Command}, m202Busybox, m202Shell},
		{"image + retained native, no command keeps the image default", appv1alpha1.AppSpec{Image: m202Busybox, Runtime: "node"}, m202Busybox, nil},
		{"genuine native repository keeps its baked command", appv1alpha1.AppSpec{Repo: m202Repo, Runtime: "node", StartCommand: m202Command}, m202Native, nil},
		{"genuine buildpack repository keeps its baked command", appv1alpha1.AppSpec{Repo: m202Repo, Builder: build.BuilderBuildpack, StartCommand: m202Command}, m202Native, nil},
		{"Dockerfile repository overrides", appv1alpha1.AppSpec{Repo: m202Repo, Runtime: "docker", StartCommand: m202Command}, m202Native, m202Shell},
		// The saved source names the image, but the release still runs the
		// served native artifact (no record to say so): the builder decides.
		{"image switch with the native artifact still running", appv1alpha1.AppSpec{Image: m202Busybox, Runtime: "node", StartCommand: m202Command}, m202Native, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := projectionApp(func(a *appv1alpha1.App) { a.Spec = tc.spec })
			web, cron := m202Commands(t, app, tc.image, launchUnknown)
			m202Expect(t, web, cron, tc.want)
		})
	}

	t.Run("explicit cron command still wins", func(t *testing.T) {
		app := projectionApp(func(a *appv1alpha1.App) {
			a.Spec = appv1alpha1.AppSpec{Type: appv1alpha1.TypeCronJob, Image: m202Busybox, Runtime: "node", StartCommand: m202Command, Command: m202Explicit}
		})
		for _, launch := range []launchMode{launchUnknown, launchImage} {
			got := (&AppReconciler{}).cronPodSpec(app, m202Busybox, 0, nil, launch).Spec.Containers[0].Command
			if want := shellCommand(m202Explicit); !slices.Equal(got, want) {
				t.Errorf("launch %q: cron command = %q, want %q", launch, got, want)
			}
		}
	})
}

// m202Record is a release record whose app container ran image with command.
func m202Record(startCommand, image string, command []string) *runtimeConfigRecord {
	return &runtimeConfigRecord{
		spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: startCommand},
		template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
			{Name: appContainerName, Image: image, Command: command},
		}}},
	}
}

// m202Reconciler stores rec as app's release record for generation.
func m202Reconciler(t *testing.T, app *appv1alpha1.App, generation int64, rec *runtimeConfigRecord) *AppReconciler {
	t.Helper()
	scheme := newTestScheme(t)
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Scheme: scheme}
	if rec != nil {
		if err := r.writeRuntimeConfigRecord(context.Background(), app, generation, rec); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// TestSelectedReleaseKeepsItsRecordedLaunch: Restart or rollback selects a
// release whose artifact can predate the saved source. Its record's app
// container shows how that artifact launched, which decides over the source.
func TestSelectedReleaseKeepsItsRecordedLaunch(t *testing.T) {
	imageSaved := appv1alpha1.AppSpec{Image: m202Busybox, Runtime: "node", StartCommand: m202Command}
	gitSaved := appv1alpha1.AppSpec{Repo: m202Repo, Runtime: "node", StartCommand: m202Baked}
	const olderBusybox = "docker.io/library/busybox:1.36"

	cases := []struct {
		name  string
		saved appv1alpha1.AppSpec
		rec   *runtimeConfigRecord
		image string
		shell bool // the selected StartCommand runs through the shell
	}{
		{"native artifact selected after the source switched to an image", imageSaved, m202Record(m202Baked, m202Native, nil), m202Native, false},
		{"older image release selected after a switch to another image", imageSaved, m202Record(m202Command, olderBusybox, m202Shell), olderBusybox, true},
		{"image release selected after a switch back to Git", gitSaved, m202Record(m202Command, m202Busybox, m202Shell), m202Busybox, true},
		{"native release selected while the saved source is native", gitSaved, m202Record(m202Baked, m202Native, nil), m202Native, false},
		// A pre-fix operator recorded the bug itself — no command on the saved
		// prebuilt image — which proves no baked launch: the source decides.
		{"pre-fix record of the saved prebuilt image", imageSaved, m202Record(m202Command, m202Busybox, nil), m202Busybox, true},
		// No template says nothing about the launch: the saved spec decides,
		// exactly as before records carried one.
		{"origin-unknown record, saved image source", imageSaved, &runtimeConfigRecord{spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: m202Command}}, m202Busybox, true},
		{"origin-unknown record, saved native source", gitSaved, &runtimeConfigRecord{spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: m202Command}}, m202Native, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := projectionApp(func(a *appv1alpha1.App) {
				a.Spec = tc.saved
				a.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 9, SourceGeneration: 4, Image: tc.image}
				a.Status.ReleaseGeneration = 9
			})
			r := m202Reconciler(t, app, 9, tc.rec)
			runtime, launch, err := r.selectedRuntime(context.Background(), app)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			if tc.shell {
				want = shellCommand(tc.rec.spec.StartCommand)
			}
			web, cron := m202Commands(t, runtime, tc.image, launch)
			m202Expect(t, web, cron, want)
		})
	}

	t.Run("a selected cron Command takes precedence", func(t *testing.T) {
		rec := m202Record(m202Command, m202Native, shellCommand(m202Explicit))
		rec.spec.Command = new(m202Explicit)
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec = gitSaved; a.Spec.Type = appv1alpha1.TypeCronJob })
		got := (&AppReconciler{}).cronPodSpec(rec.projectRuntimeApp(app, &appv1alpha1.ReleaseConfigReference{Generation: 9}), m202Native, 0, nil, rec.launch(app)).Spec.Containers[0].Command
		if want := shellCommand(m202Explicit); !slices.Equal(got, want) {
			t.Errorf("cron command = %q, want %q", got, want)
		}
	})
}

// TestStagedSourceSwitchKeepsTheServedLaunch: Update Source stages a new source
// without a deploy, and every pass until the next deploy keeps running the
// served artifact. Its pod must keep the launch it was served with, or an
// operational pass would roll it onto a different command.
func TestStagedSourceSwitchKeepsTheServedLaunch(t *testing.T) {
	staged := func(spec appv1alpha1.AppSpec) *appv1alpha1.App {
		return projectionApp(func(a *appv1alpha1.App) {
			a.Spec = spec
			a.Annotations = map[string]string{
				appv1alpha1.AnnotationPendingSourceGeneration: "5",
				appv1alpha1.AnnotationReleaseGeneration:       "4",
			}
			a.Status.ActiveRevision = "rev-4"
			a.Status.Image = m202Busybox
		})
	}
	gitStaged := appv1alpha1.AppSpec{Repo: m202Repo, Runtime: "node", StartCommand: m202Command}

	t.Run("image served with its command, Git switch staged", func(t *testing.T) {
		app := staged(gitStaged)
		r := m202Reconciler(t, app, 4, m202Record(m202Command, m202Busybox, m202Shell))
		runtime, launch, err := r.selectedRuntime(context.Background(), app)
		if err != nil {
			t.Fatal(err)
		}
		web, cron := m202Commands(t, runtime, m202Busybox, launch)
		m202Expect(t, web, cron, m202Shell)
	})

	t.Run("image served without its command keeps that pod unchanged", func(t *testing.T) {
		app := staged(gitStaged)
		r := m202Reconciler(t, app, 4, m202Record(m202Command, m202Busybox, nil))
		runtime, launch, err := r.selectedRuntime(context.Background(), app)
		if err != nil {
			t.Fatal(err)
		}
		web, cron := m202Commands(t, runtime, m202Busybox, launch)
		m202Expect(t, web, cron, nil)
	})

	t.Run("no pending source reads no record", func(t *testing.T) {
		app := staged(gitStaged)
		delete(app.Annotations, appv1alpha1.AnnotationPendingSourceGeneration)
		r := m202Reconciler(t, app, 4, m202Record(m202Command, m202Busybox, m202Shell))
		if _, launch, err := r.selectedRuntime(context.Background(), app); err != nil || launch != launchUnknown {
			t.Fatalf("launch = %q, %v; want the saved source to decide", launch, err)
		}
	})

	t.Run("served release without a record", func(t *testing.T) {
		app := staged(gitStaged)
		r := m202Reconciler(t, app, 0, nil)
		if _, launch, err := r.selectedRuntime(context.Background(), app); err != nil || launch != launchUnknown {
			t.Fatalf("launch = %q, %v; want the saved source to decide", launch, err)
		}
	})
}
