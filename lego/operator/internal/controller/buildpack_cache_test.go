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
	"strconv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestAClearStartsABuildpackCacheTheNextReleaseKeeps (w5/m134): kpack restores
// whatever its one cache tag holds, so a clear-cache release builds on a tag
// the releases before it never used, and the App records it before that build
// starts. bex-api drops the clear marker on the next deploy, yet that release
// builds on the clear's cache, not on the one it cleared.
func TestAClearStartsABuildpackCacheTheNextReleaseKeeps(t *testing.T) {
	ctx := context.Background()
	app := activeApp("tea-kpack-cache")
	app.UID = "uid-web" // the fake client assigns none; build artifacts are keyed on it
	recordedAtDispatch := map[string]int64{}
	r, cl, nn := lifecycleFixture(t, app, withInterceptor(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if image, ok := obj.(*unstructured.Unstructured); ok && image.GetKind() == "Image" {
				var live appv1alpha1.App
				if err := c.Get(ctx, types.NamespacedName{Namespace: "tea-kpack-cache", Name: "web"}, &live); err != nil {
					return err
				}
				recordedAtDispatch[image.GetLabels()[appv1alpha1.LabelBuildRevision]] = live.Status.BuildCacheGeneration
			}
			return c.Create(ctx, obj, opts...)
		},
	}))
	cacheTag := func(generation int64) string {
		t.Helper()
		image := dispatchedKpackImage(t, r, cl, nn, generation)
		tag, _, _ := unstructured.NestedString(image.Object, "spec", "cache", "registry", "tag")
		if tag == "" {
			t.Fatalf("release %d's kpack Image configures no registry cache", generation)
		}
		return tag
	}
	stored := func(generation int64) {
		t.Helper()
		image := dispatchedKpackImage(t, r, cl, nn, generation)
		markKpackImageBuilt(image)
		if err := cl.Update(ctx, image); err != nil {
			t.Fatal(err)
		}
		reconcileOnce(t, r, nn)
	}
	deploy := func(generation int64, clearCache bool) {
		t.Helper()
		updateApp(t, cl, nn, func(a *appv1alpha1.App) {
			a.Generation = generation
			a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(generation, 10)
			a.Spec.BuildCommit = strings.Repeat(strconv.FormatInt(generation, 10), 40)
			delete(a.Annotations, appv1alpha1.AnnotationClearCacheReleaseGeneration)
			if clearCache {
				a.Annotations[appv1alpha1.AnnotationClearCacheReleaseGeneration] = strconv.FormatInt(generation, 10)
			}
		})
		reconcileOnce(t, r, nn)
	}

	serveReleaseOne(t, r, cl, nn)
	releaseTwoFromSource(t, cl, nn)
	updateApp(t, cl, nn, func(a *appv1alpha1.App) { a.Spec.Builder = build.BuilderBuildpack })
	reconcileOnce(t, r, nn)
	before := cacheTag(2)
	stored(2)

	deploy(3, true)
	cleared := cacheTag(3)
	if cleared == before {
		t.Fatalf("the clear-cache release caches in %q, the tag it was asked to clear", cleared)
	}
	if got := recordedAtDispatch[appv1alpha1.BuildRevision(3)]; got != 3 {
		t.Fatalf("status.buildCacheGeneration = %d when the clear release's build started, want 3", got)
	}
	stored(3)

	deploy(4, false)
	if got := cacheTag(4); got != cleared {
		t.Fatalf("release 4 caches in %q, want the clear's %q rather than the cache it cleared", got, cleared)
	}
}

// releaseKpackImage is the kpack Image the operator builds nn's release
// generation with, for an App whose UID is "uid-web".
func releaseKpackImage(r *AppReconciler, nn types.NamespacedName, generation int64) *unstructured.Unstructured {
	return build.KpackImage(build.Options{
		Name: nn.Name, AppUID: "uid-web", Namespace: r.buildNamespace(nn.Namespace), Revision: appv1alpha1.BuildRevision(generation),
		Repo: "https://example.invalid/repo.git",
	})
}

// dispatchedKpackImage reads back the kpack Image the operator dispatched for
// nn's release generation.
func dispatchedKpackImage(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName, generation int64) *unstructured.Unstructured {
	t.Helper()
	image := releaseKpackImage(r, nn, generation)
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(image), image); err != nil {
		t.Fatalf("release %d's kpack Image was never dispatched: %v", generation, err)
	}
	return image
}

// markKpackImageBuilt gives image the status kpack reports once its build
// pushed an image.
func markKpackImageBuilt(image *unstructured.Unstructured) {
	image.Object["status"] = map[string]any{
		"conditions":  []any{map[string]any{"type": kpackReadyType, "status": "True"}},
		"latestImage": "zot.bex-registry.svc:5000/web@sha256:" + image.GetLabels()[appv1alpha1.LabelBuildRevision],
	}
}
