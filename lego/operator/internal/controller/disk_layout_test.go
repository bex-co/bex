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
	"errors"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m558: a native service with a root directory runs from
// /opt/render/project/src/<root directory>, as on Render. A disk made before,
// when it ran from /opt/render/project/src itself, keeps landing where the
// service wrote through it.

func rootDirSpec(toolchain, rootDir string) appv1alpha1.AppSpec {
	return appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeWebService, Repo: "https://github.com/example/app",
		Runtime: toolchain, RootDir: rootDir, BuildCommand: "npm ci", StartCommand: "npm start",
	}
}

func TestMovedDiskMountIsTheOldPathInTheRootDirectory(t *testing.T) {
	const src = "/opt/render/project/src"
	for _, tc := range []struct {
		name      string
		source    appv1alpha1.AppSpec
		mountPath string
		want      diskMove
		ok        bool
	}{
		{"outside the root directory", rootDirSpec("node", "backend"), src + "/uploads", diskMove{path: src + "/backend/uploads"}, true},
		{"nested", rootDirSpec("python", "apps/api"), src + "/var/data", diskMove{path: src + "/apps/api/var/data"}, true},
		{"uncleaned", rootDirSpec("ruby", "/backend/"), src + "//uploads", diskMove{path: src + "/backend/uploads"}, true},
		{"the root directory itself would hide the source", rootDirSpec("node", "backend"), src + "/backend",
			diskMove{path: src + "/backend/backend", hides: true}, true},
		{"a parent of it would too", rootDirSpec("elixir", "apps/api"), src + "/apps",
			diskMove{path: src + "/apps/api/apps", hides: true}, true},
		{"a sibling with the same prefix", rootDirSpec("rust", "apps/api"), src + "/apps/apiary", diskMove{path: src + "/apps/api/apps/apiary"}, true},
		// Already where Render puts it: in the root directory's tree.
		{"in the root directory", rootDirSpec("node", "backend"), src + "/backend/uploads", diskMove{}, false},
		{"outside the source", rootDirSpec("node", "backend"), "/var/data", diskMove{}, false},
		{"a look-alike of the source", rootDirSpec("node", "backend"), "/opt/render/project/srcdata", diskMove{}, false},
		// The working directory did not move.
		{"no root directory", rootDirSpec("node", ""), src + "/uploads", diskMove{}, false},
		{"an unsafe root directory", rootDirSpec("node", "my app"), src + "/uploads", diskMove{}, false},
		{"go moved with w6/m546's link", rootDirSpec("go", "backend"), src + "/uploads", diskMove{}, false},
		{"docker", rootDirSpec("docker", "backend"), src + "/uploads", diskMove{}, false},
		{"a prebuilt image", appv1alpha1.AppSpec{Image: "nginx:1", RootDir: "backend"}, src + "/uploads", diskMove{}, false},
	} {
		if got, ok := movedDiskMount(tc.source, tc.mountPath); got != tc.want || ok != tc.ok {
			t.Errorf("%s: movedDiskMount = %+v, %v; want %+v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}

	for _, tc := range []struct {
		move diskMove
		want []string
	}{
		{diskMove{}, []string{src + "/uploads"}},
		{diskMove{path: src + "/backend/uploads"}, []string{src + "/uploads", src + "/backend/uploads"}},
		{diskMove{path: src + "/backend/uploads", hides: true}, []string{src + "/backend/uploads"}},
	} {
		if got := diskMountPaths(src+"/uploads", tc.move); !slices.Equal(got, tc.want) {
			t.Errorf("diskMountPaths(%+v) = %v, want %v", tc.move, got, tc.want)
		}
	}
}

// The disk mounts are adopted with the next deploy, which builds the image
// that runs in the root directory: the operator upgrade that ships them must
// not Recreate a disk service by itself.
func TestEarlierDiskMountMovesWithTheNextRoll(t *testing.T) {
	const old, moved = "/opt/render/project/src/uploads", "/opt/render/project/src/backend/uploads"
	app := func(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
		return diskProjectionApp(append([]func(*appv1alpha1.App){func(a *appv1alpha1.App) {
			a.Spec.Runtime, a.Spec.RootDir, a.Spec.Repo = "node", "backend", "https://github.com/example/app"
			a.Spec.Disk.MountPath = old
		}}, mutate...)...)
	}
	params := diskParams("")
	stored := project(app(), params)
	if got := storedDiskMountPaths(stored.Spec.Template.Spec); !slices.Equal(got, []string{old}) {
		t.Fatalf("a disk with no move mounts at %v, want only its path", got)
	}

	params.diskMove = diskMove{path: moved}
	upgraded := stored.DeepCopy()
	applyDeploymentSpec(upgraded, app(), params)
	if !equalPodTemplate(upgraded.Spec.Template, stored.Spec.Template) {
		t.Fatal("an unchanged service's template changed on upgrade; that would Recreate it")
	}

	rolled := stored.DeepCopy()
	applyDeploymentSpec(rolled, app(restartedAt("2026-11-21T12:00:00Z")), params)
	if got := storedDiskMountPaths(rolled.Spec.Template.Spec); !slices.Equal(got, []string{old, moved}) {
		t.Fatalf("the rolling pass mounts the disk at %v, want %v", got, []string{old, moved})
	}
	volumes := 0
	for _, v := range rolled.Spec.Template.Spec.Volumes {
		if v.Name == diskVolumeName {
			volumes++
		}
	}
	if volumes != 1 {
		t.Fatalf("the template carries %d disk volumes, want the one claim mounted twice", volumes)
	}

	// Once moved it stays moved, pass after pass.
	steady := rolled.DeepCopy()
	applyDeploymentSpec(steady, app(restartedAt("2026-11-21T12:00:00Z")), params)
	if !equalPodTemplate(steady.Spec.Template, rolled.Spec.Template) {
		t.Fatal("a moved disk's template changed on a pass that changes nothing")
	}

	// A path that would hide the source is mounted only at its new place.
	params.diskMove = diskMove{path: "/opt/render/project/src/backend/backend", hides: true}
	hidden := project(app(func(a *appv1alpha1.App) { a.Spec.Disk.MountPath = "/opt/render/project/src/backend" }), params)
	if got := storedDiskMountPaths(hidden.Spec.Template.Spec); !slices.Equal(got, []string{"/opt/render/project/src/backend/backend"}) {
		t.Fatalf("a disk at the root directory mounts at %v, want only its new place", got)
	}
}

// Only a claim with no layout mark predates w6/m558; a claim this operator
// makes is marked, and a service linked to a build source moves as the
// source's build does.
func TestEarlierDiskMoveReadsTheClaimsMark(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	const old = "/opt/render/project/src/uploads"
	newApp := func(name string, spec appv1alpha1.AppSpec) *appv1alpha1.App {
		a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Spec: spec}
		a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: old, SizeGB: 1}
		return a
	}
	claim := func(app string, marked bool) *corev1.PersistentVolumeClaim {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: diskPVCName(app), Namespace: "default"}}
		if marked {
			pvc.Annotations = map[string]string{annotDiskLayout: diskLayoutRootDirectory}
		}
		return pvc
	}
	earlier := newApp("earlier", rootDirSpec("node", "backend"))
	marked := newApp("marked", rootDirSpec("node", "backend"))
	unclaimed := newApp("unclaimed", rootDirSpec("node", "backend"))
	carrier := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "carrier", Namespace: "default"}, Spec: rootDirSpec("node", "backend")}
	carrier.Spec.Type = appv1alpha1.TypeBuildSource
	linked := newApp("linked", appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, BuildSource: &appv1alpha1.BuildSourceLink{Name: "carrier"}})
	unmoved := newApp("unmoved", rootDirSpec("node", ""))

	reads := 0
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(carrier, claim("earlier", false), claim("marked", true), claim("linked", false), claim("unmoved", false)).
		WithInterceptorFuncs(interceptor.Funcs{Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.PersistentVolumeClaim); ok {
				reads++
			}
			return cl.Get(ctx, key, obj, opts...)
		}}).Build()
	r := &AppReconciler{Client: c, Scheme: scheme}

	moved := diskMove{path: "/opt/render/project/src/backend/uploads"}
	for _, tc := range []struct {
		app  *appv1alpha1.App
		want diskMove
	}{
		{earlier, moved}, {marked, diskMove{}}, {unclaimed, diskMove{}}, {linked, moved}, {unmoved, diskMove{}},
	} {
		got, err := r.earlierDiskMove(context.Background(), tc.app)
		if err != nil || got != tc.want {
			t.Errorf("%s: earlierDiskMove = %+v, %v; want %+v", tc.app.Name, got, err, tc.want)
		}
	}
	if reads != 4 {
		t.Errorf("read %d claims, want 4: a disk whose path did not move needs no read", reads)
	}

	// A claim that cannot be read fails the pass rather than dropping a mount.
	failing := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim("earlier", false)).
		WithInterceptorFuncs(interceptor.Funcs{Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return errors.New("cache unavailable")
		}}).Build()
	if _, err := (&AppReconciler{Client: failing, Scheme: scheme}).earlierDiskMove(context.Background(), earlier); err == nil {
		t.Error("an unreadable claim answered no move")
	}
}

// A claim this operator makes carries the layout mark, so a disk made from
// now on stays at the path its owner gave it.
func TestNewDiskClaimIsMarkedWithTheRootDirectoryLayout(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	app := diskApp("web", &appv1alpha1.DiskSpec{Name: "data", MountPath: "/opt/render/project/src/uploads", SizeGB: 1})
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(app).Build()
	r := &AppReconciler{Client: c, BuildClient: c, Scheme: scheme}
	if err := r.reconcileDisk(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	var pvc corev1.PersistentVolumeClaim
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: app.Namespace, Name: diskPVCName(app.Name)}, &pvc); err != nil {
		t.Fatal(err)
	}
	if got := pvc.Annotations[annotDiskLayout]; got != diskLayoutRootDirectory {
		t.Errorf("new claim %s = %q, want %q", annotDiskLayout, got, diskLayoutRootDirectory)
	}
}

// The serving pass reads the claim's mark for the release it projects: a new
// Deployment of a service whose disk predates w6/m558 mounts it in the root
// directory as well.
func TestServingDeploymentMountsAnEarlierDiskInTheRootDirectory(t *testing.T) {
	const old, moved = "/opt/render/project/src/uploads", "/opt/render/project/src/backend/uploads"
	for name, marked := range map[string]bool{"an earlier disk": false, "a disk made since": true} {
		t.Run(name, func(t *testing.T) {
			app := servedBuildRerunApp(func(a *appv1alpha1.App) {
				a.Spec.RootDir, a.Spec.Replicas, a.Spec.Tier = "backend", 1, "starter"
				a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: old, SizeGB: 1}
			})
			r := recordsReconciler(t, app, map[int64]*runtimeConfigRecord{5: m202Record(w32048Old, w32048Image, nil)})
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: diskPVCName(app.Name), Namespace: app.Namespace}}
			if marked {
				pvc.Annotations = map[string]string{annotDiskLayout: diskLayoutRootDirectory}
			}
			if err := r.Create(context.Background(), pvc); err != nil {
				t.Fatal(err)
			}
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
			if _, err := r.applyServingDeployment(context.Background(), app, dep, deploymentParams{image: w32048Image, port: 10000, replicas: 1}, nil); err != nil {
				t.Fatal(err)
			}
			want := []string{old, moved}
			if marked {
				want = []string{old}
			}
			if got := storedDiskMountPaths(dep.Spec.Template.Spec); !slices.Equal(got, want) {
				t.Errorf("the disk is mounted at %v, want %v", got, want)
			}
		})
	}
}
