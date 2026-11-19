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
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const cacheTestBuildNamespace = "bex-build"

var cacheTestStore = build.CacheStore{
	Endpoint: "https://s3.example", Bucket: "bex-build-cache", Region: "eu-central-2", Secret: "bex-build-cache-s3",
}

func cacheDeletionApp(name string) *appv1alpha1.App {
	app := deletionApp(name)
	app.Labels = map[string]string{labelWorkspace: "tea-w1"}
	app.Spec.Repo = "https://github.com/bex-co/hello"
	return app
}

func cacheCredential() *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cacheTestStore.Secret, Namespace: cacheTestBuildNamespace}, Data: map[string][]byte{
		"AWS_ACCESS_KEY_ID":     []byte("test-access"),
		"AWS_SECRET_ACCESS_KEY": []byte("test-secret"),
	}}
}

func cacheDeletionReconciler(t *testing.T, objects ...client.Object) (*AppReconciler, client.Client) {
	t.Helper()
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(objects...).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	return &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes,
		BuildNamespace: cacheTestBuildNamespace, BuildCache: true, BuildCacheStore: cacheTestStore}, cl
}

func cachePurgeJobKey(app *appv1alpha1.App) types.NamespacedName {
	return types.NamespacedName{Namespace: cacheTestBuildNamespace, Name: cleanupJobName(buildCachePurgeJobPrefix, app.Name, app.UID)}
}

func TestSourceAppDeletionWaitsForItsBuildCachePurge(t *testing.T) {
	// Cache layers are built from the tenant's source, so they must not
	// outlive the App: the finalizer holds until the purge Job has run, and
	// releases only once the Job itself is gone.
	app := cacheDeletionApp("cached")
	r, cl := cacheDeletionReconciler(t, app, cacheCredential())
	nn := client.ObjectKeyFromObject(app)
	req := reconcile.Request{NamespacedName: nn}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	var job batchv1.Job
	if err := cl.Get(context.Background(), cachePurgeJobKey(app), &job); err != nil {
		t.Fatalf("build cache purge Job not created: %v", err)
	}
	if got := envValue(job.Spec.Template.Spec.Containers[0].Env, "BEX_CACHE_URI"); got != "s3://bex-build-cache/tea-w1/uid-cached/" {
		t.Errorf("purge targets %q, want the App lifetime's whole root", got)
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); err != nil {
		t.Fatalf("App released before its cache purge ran: %v", err)
	}

	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := cl.Status().Update(context.Background(), &job); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); !apierrors.IsNotFound(err) {
		t.Fatalf("App survived a completed cache purge: %v", err)
	}
	if err := cl.Get(context.Background(), cachePurgeJobKey(app), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("cache purge Job survived finalization: %v", err)
	}
}

func TestBuildCachePurgeFailsClosedWithoutItsCredential(t *testing.T) {
	app := cacheDeletionApp("no-cred")
	r, cl := cacheDeletionReconciler(t, app)
	_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(app)})
	if err == nil || !strings.Contains(err.Error(), "build cache") {
		t.Fatalf("deletion without the cache credential = %v, want a build-cache error", err)
	}
	if err := cl.Get(context.Background(), cachePurgeJobKey(app), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Errorf("a purge Job was dispatched without its credential: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), &appv1alpha1.App{}); err != nil {
		t.Errorf("the App was released with its cache unpurged: %v", err)
	}
}

func TestBuildCachePurgeOnlyWhereACacheCanExist(t *testing.T) {
	// An image-backed App never built, and the registry backend's cache leaves
	// with the App's image repository: neither waits on an S3 purge.
	image := cacheDeletionApp("image-app")
	image.Spec.Repo = ""
	image.Spec.Image = "nginx:1.27"
	registryApp := cacheDeletionApp("registry-app")
	for _, tc := range []struct {
		name  string
		app   *appv1alpha1.App
		store build.CacheStore
	}{
		{"image-backed App", image, cacheTestStore},
		{"registry backend", registryApp, build.CacheStore{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, cl := cacheDeletionReconciler(t, tc.app)
			r.BuildCacheStore = tc.store
			if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(tc.app)}); err != nil {
				t.Fatal(err)
			}
			if err := cl.Get(context.Background(), cachePurgeJobKey(tc.app), &batchv1.Job{}); !apierrors.IsNotFound(err) {
				t.Errorf("a cache purge Job was created: %v", err)
			}
			if err := cl.Get(context.Background(), client.ObjectKeyFromObject(tc.app), &appv1alpha1.App{}); !apierrors.IsNotFound(err) {
				t.Errorf("deletion waited on a cache that cannot exist: %v", err)
			}
		})
	}
}
