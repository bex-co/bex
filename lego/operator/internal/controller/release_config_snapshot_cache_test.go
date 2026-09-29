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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// release_config_snapshot_cache_test.go is w1/m152 t011 (w4/171): production's
// manager caches Secrets for ONE namespace (NamespacedSecretCacheOptions), while
// every tenant App lives in its workspace namespace (ADR043). A cached Secret call
// there fails with "unknown namespace for the cache" — not NotFound — so the
// snapshot step failed every new service's first release. envtest could not see
// it: its Apps live in the cached namespace.

// productionShapedClients returns the manager's cached client as production
// builds it — Secrets outside cachedNS refused with the cache's own error — and
// the uncached client, both over one store.
func productionShapedClients(t *testing.T, objs ...client.Object) (cached, uncached client.WithWatch) {
	t.Helper()
	const cachedNS = "default"
	base := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(objs...).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	refuse := func(obj any, ns string) error {
		switch obj.(type) {
		case *corev1.Secret, *corev1.SecretList:
			if ns != cachedNS {
				return fmt.Errorf("unable to get: %s because of unknown namespace for the cache", ns)
			}
		}
		return nil
	}
	cached = interceptor.NewClient(base, interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if err := refuse(obj, key.Namespace); err != nil {
				return err
			}
			return c.Get(ctx, key, obj, opts...)
		},
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			lo := (&client.ListOptions{}).ApplyOptions(opts)
			if err := refuse(list, lo.Namespace); err != nil {
				return err
			}
			return c.List(ctx, list, opts...)
		},
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if err := refuse(obj, obj.GetNamespace()); err != nil {
				return err
			}
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if err := refuse(obj, obj.GetNamespace()); err != nil {
				return err
			}
			return c.Update(ctx, obj, opts...)
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if err := refuse(obj, obj.GetNamespace()); err != nil {
				return err
			}
			return c.Delete(ctx, obj, opts...)
		},
	})
	return cached, base
}

// Every snapshot operation — first-release copy, record, cancel settle, the
// cancel template read, and GC — works for an App in a workspace namespace.
func TestReleaseSnapshotsWorkOutsideTheCachedNamespace(t *testing.T) {
	ctx := context.Background()
	const ns = "tea-m152"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: ns, Generation: 1, UID: "uid-api"},
		Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, EnvFromSecret: "api-env", EnvFromSecrets: []string{"evg-g-env"}},
		Status:     appv1alpha1.AppStatus{ReleaseGeneration: 1},
	}
	own := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "api-env", Namespace: ns}, Data: map[string][]byte{"MESSAGE": []byte("v1")}}
	grp := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "evg-g-env", Namespace: ns}, Data: map[string][]byte{"G": []byte("g1")}}
	cached, uncached := productionShapedClients(t, app, own, grp)
	r := &AppReconciler{Client: cached, BuildClient: uncached, Scheme: cached.Scheme()}

	// A new service's first release — the w4/171 failure.
	if err := r.ensureReleaseConfigSnapshot(ctx, app); err != nil {
		t.Fatalf("first release: %v", err)
	}
	if app.Status.ConfigSnapshotGeneration != 1 {
		t.Fatalf("first release did not project its snapshot: %d", app.Status.ConfigSnapshotGeneration)
	}
	if err := r.recordReleasePodTemplate(ctx, app, corev1.PodTemplateSpec{}); err != nil {
		t.Fatalf("record: %v", err)
	}

	// Generation 1 serves; generation 2 is dispatched, then canceled.
	app.Status.ActiveRevision = "rev-1"
	app.Generation, app.Status.ReleaseGeneration = 2, 2
	if err := r.ensureReleaseConfigSnapshot(ctx, app); err != nil {
		t.Fatalf("second release: %v", err)
	}
	// The status write above refreshed app from the store; the cancel is for the
	// generation just dispatched.
	app.Generation = 2
	app.Annotations = map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "2"}
	if moved, err := r.settleConfigSnapshotTo(ctx, app, 1); err != nil || !moved {
		t.Fatalf("cancel settle = (%v, %v), want a move onto generation 1", moved, err)
	}
	if tmpl, err := r.servedPodTemplateForCancel(ctx, app); err != nil || tmpl == nil {
		t.Fatalf("cancel template = (%v, %v), want generation 1's record", tmpl, err)
	}

	app.Status.ReleaseGeneration = 1 + releaseSnapshotRetention
	if err := r.gcReleaseConfigSnapshots(ctx, app); err != nil {
		t.Fatalf("gc: %v", err)
	}
	var gone corev1.Secret
	if err := uncached.Get(ctx, client.ObjectKey{Namespace: ns, Name: "api-env-r1"}, &gone); err == nil {
		t.Fatal("gc left an expired snapshot")
	}
}

// The upgrade must not roll the fleet (decision 2): an App serving a release from
// before snapshots keeps its mutable names until a NEW release rolls out.
func TestServingAppWithoutSnapshotsMigratesAtItsNextRelease(t *testing.T) {
	ctx := context.Background()
	const ns = "tea-m152"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: ns, Generation: 3},
		Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, EnvFromSecret: "api-env"},
		Status:     appv1alpha1.AppStatus{ReleaseGeneration: 3, ActiveRevision: "rev-3"},
	}
	own := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "api-env", Namespace: ns}, Data: map[string][]byte{"MESSAGE": []byte("v1")}}
	cached, uncached := productionShapedClients(t, app, own)
	r := &AppReconciler{Client: cached, BuildClient: uncached, Scheme: cached.Scheme()}

	if err := r.ensureReleaseConfigSnapshot(ctx, app); err != nil {
		t.Fatal(err)
	}
	if app.Status.ConfigSnapshotGeneration != 0 || envNames(app)[0] != "api-env" {
		t.Fatalf("a serving App flipped with no release: gen %d, projecting %v", app.Status.ConfigSnapshotGeneration, envNames(app))
	}

	// A cancel over that release has nothing to restore to and stays put too.
	app.Generation = 4
	app.Annotations = map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "4"}
	app.Status.ReleaseGeneration = 4
	if err := r.ensureReleaseConfigSnapshot(ctx, app); err != nil || app.Status.ConfigSnapshotGeneration != 0 {
		t.Fatalf("a cancel flipped a pre-snapshot App: gen %d, err %v", app.Status.ConfigSnapshotGeneration, err)
	}

	// Its next real release snapshots, as part of a rollout that happens anyway.
	app.Annotations = nil
	app.Generation = 5
	app.Status.ReleaseGeneration = 5
	if err := r.ensureReleaseConfigSnapshot(ctx, app); err != nil {
		t.Fatal(err)
	}
	if app.Status.ConfigSnapshotGeneration != 5 || envNames(app)[0] != "api-env-r5" {
		t.Fatalf("the next release did not snapshot: gen %d, projecting %v", app.Status.ConfigSnapshotGeneration, envNames(app))
	}
}
