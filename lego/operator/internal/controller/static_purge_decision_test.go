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

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/publish"
	"github.com/bex-co/bex/lego/types/k8sname"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The w2/076 purge decision: deleting a static_site whose purge cannot run
// (store unconfigured, or publish credential absent) completes only when the
// App never published; any publish trace keeps the finalizer failing closed.
// With the credential present the purge always runs, published or not.

var staticPurgeStore = publish.Store{Bucket: "static", Endpoint: "https://s3.example", Secret: "static-creds"}

func staticPurgeDecisionApp(name string, mutate func(*appv1alpha1.App)) *appv1alpha1.App {
	app := deletionApp(name)
	app.Spec.Type = appv1alpha1.TypeStaticSite
	if mutate != nil {
		mutate(app)
	}
	return app
}

// staticPurgeEvidence builds every independent publish trace for app.
func staticPurgeEvidence() map[string]func(*appv1alpha1.App) []client.Object {
	return map[string]func(*appv1alpha1.App) []client.Object{
		"dispatch marker": func(a *appv1alpha1.App) []client.Object {
			a.Annotations = map[string]string{annotStaticPublishDispatched: string(a.UID)}
			return nil
		},
		"served revision": func(a *appv1alpha1.App) []client.Object {
			a.Status.ActiveRevision = "rev-1"
			return nil
		},
		"recorded prefix": func(a *appv1alpha1.App) []client.Object {
			a.Status.StaticPrefix = a.Name + "/rev-1/"
			return nil
		},
		"publication record": func(a *appv1alpha1.App) []client.Object {
			rec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: appv1alpha1.StaticPublicationRecordName(a.Name, 1), Namespace: a.Namespace,
				Labels: map[string]string{snapshotOfLabel: k8sname.Fit(a.Name + "-static")},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "App", Name: a.Name, UID: a.UID,
					Controller: new(true),
				}},
			}, Data: map[string][]byte{appv1alpha1.StaticPublicationPrefixKey: []byte(a.Name + "/rev-1/")}}
			return []client.Object{rec}
		},
		"publish Job from a pre-marker operator": func(a *appv1alpha1.App) []client.Object {
			job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
				Name: "pub-" + a.Name + "-rev-1", Namespace: a.Namespace,
				Labels: execution.PodLabels(a.Name, string(a.UID), publish.Component, "", "", false),
			}}
			return []client.Object{job}
		},
	}
}

func reconcileStaticDeletion(t *testing.T, r *AppReconciler, app *appv1alpha1.App, rounds int) error {
	t.Helper()
	var err error
	for range rounds {
		_, err = r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(app)})
	}
	return err
}

func TestStaticDeletionWithoutCredentialFinalizesANeverPublishedSite(t *testing.T) {
	for name, store := range map[string]publish.Store{
		"credential Secret absent": staticPurgeStore,
		"store unconfigured":       {},
	} {
		t.Run(name, func(t *testing.T) {
			app := staticPurgeDecisionApp("static-never", nil)
			// A foreign same-named App's publish Job and an unowned record must
			// not read as this App's publish evidence.
			foreignJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
				Name: "pub-static-never-rev-1", Namespace: app.Namespace,
				Labels: execution.PodLabels(app.Name, "uid-other-lifetime", publish.Component, "", "", false),
			}}
			cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, foreignJob).
				WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
			r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, StaticStore: store}
			if err := reconcileStaticDeletion(t, r, app, 3); err != nil {
				t.Fatalf("never-published deletion failed: %v", err)
			}
			if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), &appv1alpha1.App{}); !apierrors.IsNotFound(err) {
				t.Fatalf("never-published static App stayed Terminating: %v", err)
			}
			purge := publish.PurgeJob(app.Name, string(app.UID), "", app.Namespace, staticPurgeStore, app.Namespace, "", "")
			if err := cl.Get(context.Background(), client.ObjectKeyFromObject(purge), &batchv1.Job{}); !apierrors.IsNotFound(err) {
				t.Fatalf("a purge Job was dispatched without a credential: %v", err)
			}
		})
	}
}

func TestStaticDeletionWithoutCredentialFailsClosedOnAnyPublishEvidence(t *testing.T) {
	for evidence, add := range staticPurgeEvidence() {
		for name, store := range map[string]publish.Store{
			"credential Secret absent": staticPurgeStore,
			"store unconfigured":       {},
		} {
			t.Run(evidence+"/"+name, func(t *testing.T) {
				var extra []client.Object
				app := staticPurgeDecisionApp("static-published", func(a *appv1alpha1.App) { extra = add(a) })
				status := app.Status
				cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(append(extra, app)...).
					WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
				if status.ActiveRevision != "" || status.StaticPrefix != "" {
					stored := &appv1alpha1.App{}
					if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), stored); err != nil {
						t.Fatal(err)
					}
					stored.Status = status
					if err := cl.Status().Update(context.Background(), stored); err != nil {
						t.Fatal(err)
					}
				}
				r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, StaticStore: store}
				err := reconcileStaticDeletion(t, r, app, 3)
				want := "credential Secret default/static-creds is unavailable"
				if !store.Configured() {
					want = "static purge configuration is unavailable"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("deletion error = %v, want it to name %q", err, want)
				}
				stored := &appv1alpha1.App{}
				if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), stored); err != nil {
					t.Fatalf("published static App finalized with its content unpurged: %v", err)
				}
				// A reaped Job or record must not turn a later pass into "never
				// published": the evidence is latched onto the App itself.
				if !staticPublishRecorded(stored) {
					t.Fatalf("publish evidence %q was not latched onto the App", evidence)
				}
			})
		}
	}
}

func TestStaticDeletionWithCredentialPurgesAPublishedSite(t *testing.T) {
	app := staticPurgeDecisionApp("static-purge-published", func(a *appv1alpha1.App) {
		a.Annotations = map[string]string{annotStaticPublishDispatched: string(a.UID)}
	})
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app, staticPurgeCreds(app.Namespace)).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, StaticStore: staticPurgeStore}
	_ = reconcileStaticDeletion(t, r, app, 1)
	purge := publish.PurgeJob(app.Name, string(app.UID), "", app.Namespace, staticPurgeStore, app.Namespace, "", "")
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(purge), &batchv1.Job{}); err != nil {
		t.Fatalf("published static App with a credential dispatched no purge Job: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), &appv1alpha1.App{}); err != nil {
		t.Fatalf("static App finalized before its purge completed: %v", err)
	}
}

func TestMarkStaticPublishDispatchedPersistsWithoutReplacingTheInMemoryApp(t *testing.T) {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: "default", UID: types.UID("uid-site")}}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}
	inMemory := app.DeepCopy()
	inMemory.Status.Phase = appv1alpha1.PhaseDeploying // unwritten status must survive
	if err := r.markStaticPublishDispatched(context.Background(), inMemory); err != nil {
		t.Fatal(err)
	}
	if inMemory.Status.Phase != appv1alpha1.PhaseDeploying {
		t.Fatalf("in-memory status replaced by the server copy: %q", inMemory.Status.Phase)
	}
	if got := inMemory.Annotations[annotStaticPublishDispatched]; got != "uid-site" {
		t.Fatalf("in-memory marker = %q", got)
	}
	stored := &appv1alpha1.App{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), stored); err != nil {
		t.Fatal(err)
	}
	if got := stored.Annotations[annotStaticPublishDispatched]; got != "uid-site" {
		t.Fatalf("persisted marker = %q", got)
	}
}
