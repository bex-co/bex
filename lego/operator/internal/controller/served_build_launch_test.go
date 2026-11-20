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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/operator/internal/build"
)

// The w32/048 repro: a native Node service built with the old start command
// (generation 5), a save-only start command change (6), then an environment
// group link that reruns the served build (7).
const (
	w32048Image = "zot.local:5000/tea-x/qa-env:gen-5@sha256:647e0e77"
	w32048Old   = `echo "QA8185 run QA_MARK=$QA_MARK"; exec perl serve.pl`
	w32048New   = `echo "QA8185 run2 QA_MARK=$QA_MARK QA_GRP=[$QA_GRP]"; exec perl serve.pl`
)

// servedBuildRerunApp is the native service at the link's release: an
// image-only selection of the served build, adopted, while generation 5's
// build still serves.
func servedBuildRerunApp(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	return projectionApp(append([]func(*appv1alpha1.App){func(a *appv1alpha1.App) {
		a.Namespace = snapshotTestNS
		a.UID = "qa-env-uid"
		a.Generation = 7
		a.Annotations = map[string]string{
			appv1alpha1.AnnotationReleaseGeneration:       "7",
			appv1alpha1.AnnotationPendingSourceGeneration: "6",
		}
		a.Spec = appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Repo: m202Repo, Runtime: "node", StartCommand: w32048New,
			ReleaseConfig: &appv1alpha1.ReleaseConfigReference{Generation: 7, Image: w32048Image}}
		a.Status.ReleaseGeneration = 7
		a.Status.ActiveRevision = "rev-5"
		a.Status.Image = w32048Image
	}}, mutate...)...)
}

// nativeArgsRecord is a release record whose app container passed
// startCommand to the native entrypoint as arguments.
func nativeArgsRecord(startCommand, image string) *runtimeConfigRecord {
	rec := m202Record(startCommand, image, nil)
	rec.template.Spec.Containers[0].Args = nativeStartArgs(startCommand)
	return rec
}

// recordsReconciler stores each record as app's release record for its
// generation.
func recordsReconciler(t *testing.T, app *appv1alpha1.App, records map[int64]*runtimeConfigRecord) *AppReconciler {
	t.Helper()
	scheme := deletionScheme(t)
	r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(app).WithObjects(app.DeepCopy()).Build(),
		Scheme: scheme, Mode: ModeKubernetes}
	// The stored copy's resourceVersion, for the pass's own status writes.
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(app), app); err != nil {
		t.Fatal(err)
	}
	for generation, rec := range records {
		if err := r.writeRuntimeConfigRecord(context.Background(), app, generation, rec); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// launchedContainer is the app container a Deployment of app runs.
func launchedContainer(t *testing.T, r *AppReconciler, app *appv1alpha1.App) (launchMode, corev1.Container) {
	t.Helper()
	runtime, launch, err := r.selectedLaunch(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	p := webParams()
	p.image, p.launch = w32048Image, launch
	return launch, appContainerOf(t, project(runtime, p))
}

// TestServedBuildRerunRunsTheSavedStartCommand is w32/048: an environment
// group link, or an environment "Save and deploy", reruns the served native
// build. That image's CMD is the start command it was built with, so the
// release passes its own start command to the native entrypoint. Before, the
// container carried neither command nor args, the deploy went live on the old
// command, and undeployedChanges read false.
func TestServedBuildRerunRunsTheSavedStartCommand(t *testing.T) {
	builtOld := m202Record(w32048Old, w32048Image, nil)

	t.Run("a rerun of a build passes the saved start command", func(t *testing.T) {
		app := servedBuildRerunApp()
		launch, c := launchedContainer(t, recordsReconciler(t, app, map[int64]*runtimeConfigRecord{5: builtOld}), app)
		if launch != launchNativeArgs || len(c.Command) != 0 || !slices.Equal(c.Args, nativeStartArgs(w32048New)) {
			t.Fatalf("launch %q, command %q, args %q; want the native entrypoint with args %q", launch, c.Command, c.Args, nativeStartArgs(w32048New))
		}
	})

	t.Run("a rerun of a rerun passes it too", func(t *testing.T) {
		app := servedBuildRerunApp()
		r := recordsReconciler(t, app, map[int64]*runtimeConfigRecord{5: nativeArgsRecord(w32048Old, w32048Image)})
		if launch, c := launchedContainer(t, r, app); launch != launchNativeArgs || !slices.Equal(c.Args, nativeStartArgs(w32048New)) {
			t.Fatalf("launch %q, args %q; want args %q", launch, c.Args, nativeStartArgs(w32048New))
		}
	})

	t.Run("the adoption record, with no template yet, still reads the served launch", func(t *testing.T) {
		app := servedBuildRerunApp()
		adopted := &runtimeConfigRecord{spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: w32048New}}
		r := recordsReconciler(t, app, map[int64]*runtimeConfigRecord{5: builtOld, 7: adopted})
		if launch, c := launchedContainer(t, r, app); launch != launchNativeArgs || !slices.Equal(c.Args, nativeStartArgs(w32048New)) {
			t.Fatalf("launch %q, args %q; want args %q", launch, c.Args, nativeStartArgs(w32048New))
		}
	})

	// An operator upgrade must not roll a running release: one an earlier
	// operator rendered without args keeps that template.
	t.Run("a release already rendered without args keeps its template", func(t *testing.T) {
		app := servedBuildRerunApp()
		r := recordsReconciler(t, app, map[int64]*runtimeConfigRecord{5: builtOld, 7: m202Record(w32048New, w32048Image, nil)})
		if launch, c := launchedContainer(t, r, app); len(c.Command) != 0 || len(c.Args) != 0 {
			t.Fatalf("launch %q, command %q, args %q; want the recorded command-less template", launch, c.Command, c.Args)
		}
	})

	// Controls: everything that is not a rerun of a natively launched build.
	controls := []struct {
		name    string
		mutate  func(*appv1alpha1.App)
		records map[int64]*runtimeConfigRecord
		command []string
	}{
		{"a Dockerfile service keeps its shell command", func(a *appv1alpha1.App) { a.Spec.Runtime = "docker" },
			map[int64]*runtimeConfigRecord{5: m202Record(w32048Old, w32048Image, shellCommand(w32048Old))}, shellCommand(w32048New)},
		{"a buildpack service keeps the image's own process", func(a *appv1alpha1.App) { a.Spec.Runtime, a.Spec.Builder = "", build.BuilderBuildpack },
			map[int64]*runtimeConfigRecord{5: builtOld}, nil},
		{"a served Docker image under a staged native runtime keeps the builder's decision", nil,
			map[int64]*runtimeConfigRecord{5: m202Record(w32048Old, w32048Image, shellCommand(w32048Old))}, nil},
		{"no served record keeps the builder's decision", nil, nil, nil},
		{"a build release reads no selection", func(a *appv1alpha1.App) { a.Spec.ReleaseConfig = nil },
			map[int64]*runtimeConfigRecord{5: builtOld}, nil},
	}
	for _, tc := range controls {
		t.Run(tc.name, func(t *testing.T) {
			app := servedBuildRerunApp()
			if tc.mutate != nil {
				tc.mutate(app)
			}
			_, c := launchedContainer(t, recordsReconciler(t, app, tc.records), app)
			if !slices.Equal(c.Command, tc.command) || len(c.Args) != 0 {
				t.Fatalf("command %q, args %q; want command %q and no args", c.Command, c.Args, tc.command)
			}
		})
	}

	// A Restart of the fixed release selects its record, which names the
	// command it ran, whatever has been saved since.
	t.Run("a Restart of the rerun keeps its command", func(t *testing.T) {
		app := servedBuildRerunApp(func(a *appv1alpha1.App) {
			a.Generation = 9
			a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "9"
			a.Spec.StartCommand = "echo saved later"
			a.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 9, SourceGeneration: 7, Image: w32048Image, PreserveGroupValues: true}
			a.Status.ReleaseGeneration = 9
			a.Status.ActiveRevision = "rev-7"
		})
		r := recordsReconciler(t, app, map[int64]*runtimeConfigRecord{9: nativeArgsRecord(w32048New, w32048Image)})
		if launch, c := launchedContainer(t, r, app); launch != launchNativeArgs || !slices.Equal(c.Args, nativeStartArgs(w32048New)) {
			t.Fatalf("launch %q, args %q; want the restarted release's args %q", launch, c.Args, nativeStartArgs(w32048New))
		}
	})
}

// TestServedBuildRerunTemplateIsStable: the first pass renders the args and
// records them, and every later pass of the same release renders the same
// template, so the rerun rolls once.
func TestServedBuildRerunTemplateIsStable(t *testing.T) {
	app := servedBuildRerunApp()
	r := recordsReconciler(t, app, map[int64]*runtimeConfigRecord{5: m202Record(w32048Old, w32048Image, nil)})
	ctx := context.Background()
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	p := deploymentParams{image: w32048Image, port: 10000, replicas: 1}
	if err := r.applyServingDeployment(ctx, app, dep, p, nil); err != nil {
		t.Fatal(err)
	}
	first := dep.Spec.Template.DeepCopy()
	if args := first.Spec.Containers[0].Args; !slices.Equal(args, nativeStartArgs(w32048New)) {
		t.Fatalf("first pass args %q, want %q", args, nativeStartArgs(w32048New))
	}
	rec, err := r.readRuntimeConfigRecord(ctx, app, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got := rec.launch(app); got != launchNativeArgs {
		t.Fatalf("recorded launch %q, want %q", got, launchNativeArgs)
	}

	// The release goes live: it is now the served one, and its own record
	// decides.
	app.Status.ActiveRevision = "rev-7"
	if err := r.applyServingDeployment(ctx, app, dep, p, nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(dep.Spec.Template.Spec.Containers[0].Args, first.Spec.Containers[0].Args) {
		t.Fatalf("second pass args %q, want the first pass's %q", dep.Spec.Template.Spec.Containers[0].Args, first.Spec.Containers[0].Args)
	}
}
