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
	"encoding/json"
	"maps"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// static_build_environment_test.go pins what a static site's rollback
// comparison reads (w1/m660, bex-api's RollbackComparison): each static
// release's adoption record names the env var, secret file and environment
// group sources its build reads and keeps a copy of each as it was, beside the
// publication record and reclaimed with it; a static Rollback or Restart's
// record keeps the selection that names the publication it copies.

const staticEnvGroup = "evg-static0000000000000"

// staticBuildApp is a repo-backed static site with its own env var and secret
// file and one linked group, adopting release generation.
func staticBuildApp(generation int64) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: snapshotTestNS, UID: "site-uid", Generation: generation},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/site", BuildCommand: "npm run build", PublishPath: "dist",
			EnvFromSecret: "site-env", EnvFromSecrets: []string{staticEnvGroup + "-env"},
			FilesFromSecrets: []string{staticEnvGroup + "-files", "site-files"},
		},
		Status: appv1alpha1.AppStatus{ReleaseGeneration: generation},
	}
}

func staticSourceSecret(name, key, value string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: snapshotTestNS}, Data: map[string][]byte{key: []byte(value)}}
}

func TestAStaticReleaseRecordsTheEnvironmentItsBuildReads(t *testing.T) {
	scheme := deletionScheme(t)
	app := staticBuildApp(2)
	if !pinnableRelease(app) {
		t.Fatal("precondition: a repo-backed static release is recorded at adoption")
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(app).WithObjects(app,
		staticSourceSecret("site-env", "API_URL", "v1"),
		staticSourceSecret("site-files", "robots.txt", "allow-v1"),
		staticSourceSecret(staticEnvGroup+"-env", "GROUP_KEY", "group-v1"),
		staticSourceSecret(staticEnvGroup+"-files", "group.txt", "group-file-v1"),
	).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes}
	ctx := t.Context()
	if err := r.recordAdoptedRelease(ctx, app); err != nil {
		t.Fatal(err)
	}
	copyOf := func(source string, generation int64) map[string]string {
		t.Helper()
		s := &corev1.Secret{}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: snapshotTestNS, Name: appv1alpha1.AppReleaseSnapshotName("site", source, generation)}, s); err != nil {
			t.Fatalf("copy of %s at %d: %v", source, generation, err)
		}
		if !metav1.IsControlledBy(s, app) {
			t.Errorf("copy of %s at %d is not the site's own", source, generation)
		}
		out := map[string]string{}
		for k, v := range s.Data {
			out[k] = string(v)
		}
		return out
	}

	rec, err := r.readRuntimeConfigRecord(ctx, app, 2)
	if err != nil {
		t.Fatal(err)
	}
	if rec.spec.Version < 1 || rec.spec.EnvFromSecret != "site-env" ||
		!slices.Equal(rec.spec.EnvFromSecrets, []string{staticEnvGroup + "-env"}) ||
		!slices.Equal(rec.spec.FilesFromSecrets, []string{staticEnvGroup + "-files", "site-files"}) {
		t.Fatalf("record names %+v, want the site's env, group and file sources", rec.spec)
	}
	for source, want := range map[string]map[string]string{
		"site-env": {"API_URL": "v1"}, "site-files": {"robots.txt": "allow-v1"},
		staticEnvGroup + "-env": {"GROUP_KEY": "group-v1"}, staticEnvGroup + "-files": {"group.txt": "group-file-v1"},
	} {
		if got := copyOf(source, 2); !maps.Equal(got, want) {
			t.Errorf("copy of %s = %v, want %v", source, got, want)
		}
	}

	// A save after the build keeps the release's copy, and the next build
	// copies the new value.
	saved := staticSourceSecret("site-env", "API_URL", "v2")
	if err := cl.Update(ctx, saved); err != nil {
		t.Fatal(err)
	}
	if err := r.recordAdoptedRelease(ctx, app); err != nil {
		t.Fatal(err)
	}
	if got := copyOf("site-env", 2)["API_URL"]; got != "v1" {
		t.Errorf("release 2's copy became %q after a later save, want v1", got)
	}
	next := staticBuildApp(3)
	next.UID = app.UID
	if err := r.recordAdoptedRelease(ctx, next); err != nil {
		t.Fatal(err)
	}
	if got := copyOf("site-env", 3)["API_URL"]; got != "v2" {
		t.Errorf("release 3's copy = %q, want the saved v2", got)
	}

	// The record and copies leave the retained window with the publication
	// record.
	if err := r.writeStaticPublicationRecord(ctx, app, 2, "site/rev-2/", "", true); err != nil {
		t.Fatal(err)
	}
	served := staticBuildApp(22)
	served.UID, served.Status.ActiveRevision = app.UID, "rev-22"
	if err := r.gcReleaseConfigSnapshots(ctx, served); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		appv1alpha1.StaticPublicationRecordName("site", 2), appv1alpha1.ReleaseRecordName("site", 2),
		appv1alpha1.AppReleaseSnapshotName("site", "site-env", 2), appv1alpha1.AppReleaseSnapshotName("site", staticEnvGroup+"-env", 2),
		appv1alpha1.AppReleaseSnapshotName("site", "site-files", 2), appv1alpha1.AppReleaseSnapshotName("site", staticEnvGroup+"-files", 2),
	} {
		if err := cl.Get(ctx, client.ObjectKey{Namespace: snapshotTestNS, Name: name}, &corev1.Secret{}); err == nil {
			t.Errorf("%s outlived the retained window", name)
		}
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: snapshotTestNS, Name: appv1alpha1.ReleaseRecordName("site", 3)}, &corev1.Secret{}); err != nil {
		t.Errorf("release 3, inside the window, lost its record: %v", err)
	}
}

// A static Rollback copies the target's publication and builds nothing. Its
// record keeps the selection that names that publication, which bex-api
// follows to the build whose environment the copied files hold.
func TestAStaticCopyRecordsThePublicationItCopies(t *testing.T) {
	scheme := deletionScheme(t)
	app := staticBuildApp(4)
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 4, Image: "zot/site@sha256:aa", StaticPrefix: "site/rev-1/"}
	if !pinnableRelease(app) || staticRestoreSelection(app) == nil {
		t.Fatal("precondition: a static Rollback is recorded at adoption")
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(app).WithObjects(app,
		staticSourceSecret("site-env", "API_URL", "v2")).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes}
	if err := r.recordAdoptedRelease(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	rec, err := r.readReleaseRecordSecret(t.Context(), app, 4)
	if err != nil {
		t.Fatal(err)
	}
	var adopted appv1alpha1.AppSpec
	if err := json.Unmarshal(rec.Data[appv1alpha1.ReleaseRecordAdoptedSpecKey], &adopted); err != nil {
		t.Fatal(err)
	}
	ref := adopted.ReleaseConfig
	if ref == nil || ref.Generation != 4 {
		t.Fatalf("the copy's record keeps selection %+v, want generation 4's", ref)
	}
	if copied, ok := appv1alpha1.StaticPrefixGeneration(ref.StaticPrefix); !ok || copied != 1 {
		t.Fatalf("the recorded selection names release (%d, %v), want 1", copied, ok)
	}
}
