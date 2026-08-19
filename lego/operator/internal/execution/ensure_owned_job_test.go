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

package execution_test

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/operator/internal/execution"
)

func jobScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := batchv1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func testIdentity() execution.ArtifactIdentity {
	return execution.ArtifactIdentity{Name: "web", UID: "uid-1", Workspace: "tea-one", Namespace: "default"}
}

// jobFor builds the deterministic-name Job a constructor would emit for owned,
// carrying the identity labels CheckOwner validates.
func jobFor(owned execution.ArtifactIdentity) *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:      "bld-web-gen-1",
		Namespace: "bex-build",
		Labels:    owned.Labels("build"),
	}}
}

func TestEnsureOwnedJobCreatesWhenAbsent(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(jobScheme(t)).Build()
	owned := testIdentity()

	cur, created, err := execution.EnsureOwnedJob(context.Background(), cl, jobFor(owned), owned, "build")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("created = false on a fresh dispatch, want true")
	}
	if cur == nil || cur.Name != "bld-web-gen-1" || cur.Namespace != "bex-build" {
		t.Fatalf("cur = %#v, want the dispatched Job's server-side state", cur)
	}
	var stored batchv1.Job
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(cur), &stored); err != nil {
		t.Fatalf("the Job was not persisted: %v", err)
	}
}

// TestEnsureOwnedJobAdoptsExistingWithoutCreate covers the steady state (the
// idempotent re-invocation): the Job already exists from a prior pass, so the
// first Get hits and the EXISTING Job is adopted after its owner labels
// validate — and no Create ever reaches the client, since a blind Create would
// cost a live API-server 409 write on every reconcile of an App with a Job.
func TestEnsureOwnedJobAdoptsExistingWithoutCreate(t *testing.T) {
	owned := testIdentity()
	existing := jobFor(owned)
	existing.Annotations = map[string]string{"winner": "previous-reconcile"}
	creates := 0
	cl := fake.NewClientBuilder().WithScheme(jobScheme(t)).WithObjects(existing).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				creates++
				return c.Create(ctx, obj, opts...)
			},
		}).Build()

	cur, created, err := execution.EnsureOwnedJob(context.Background(), cl, jobFor(owned), owned, "build")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("created = true when the Job already existed, want false (adopted)")
	}
	if cur.Annotations["winner"] != "previous-reconcile" {
		t.Fatalf("cur = %#v, want the pre-existing Job, not this call's object", cur)
	}
	if creates != 0 {
		t.Fatalf("Create reached the client %d times for an existing Job, want 0 (cached Get first)", creates)
	}
}

// TestEnsureOwnedJobAdoptsOnCreateRace covers the create/create race: the
// first Get misses (a concurrent reconcile created the Job between this call's
// Get and Create, or the cache lags a prior dispatch), the Create hits
// AlreadyExists, and the WINNER's Job is fetched and adopted after its owner
// labels validate.
func TestEnsureOwnedJobAdoptsOnCreateRace(t *testing.T) {
	owned := testIdentity()
	existing := jobFor(owned)
	existing.Annotations = map[string]string{"winner": "concurrent-reconcile"}
	missed := false
	cl := fake.NewClientBuilder().WithScheme(jobScheme(t)).WithObjects(existing).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if !missed {
					missed = true
					return apierrors.NewNotFound(batchv1.Resource("jobs"), key.Name)
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).Build()

	cur, created, err := execution.EnsureOwnedJob(context.Background(), cl, jobFor(owned), owned, "build")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("created = true when a concurrent reconcile won the create, want false (adopted)")
	}
	if cur.Annotations["winner"] != "concurrent-reconcile" {
		t.Fatalf("cur = %#v, want the pre-existing Job (the race winner), not this call's object", cur)
	}
}

// TestEnsureOwnedJobRefusesForeignOwner pins the adoption gate: a same-named
// Job from ANOTHER App lifetime (deterministic-name reuse after delete +
// recreate, or a same-named App in another workspace sharing the build
// namespace) must be refused, with the caller package's error prefix intact.
func TestEnsureOwnedJobRefusesForeignOwner(t *testing.T) {
	foreign := execution.ArtifactIdentity{Name: "web", UID: "uid-OTHER", Workspace: "tea-two", Namespace: "default"}
	existing := jobFor(foreign)
	cl := fake.NewClientBuilder().WithScheme(jobScheme(t)).WithObjects(existing).Build()

	owned := testIdentity()
	cur, created, err := execution.EnsureOwnedJob(context.Background(), cl, jobFor(owned), owned, "publish")
	if err == nil {
		t.Fatal("a foreign-lifetime Job was adopted, want an owner-check error")
	}
	if cur != nil || created {
		t.Fatalf("cur = %v, created = %v on an owner-check failure, want nil/false", cur, created)
	}
	if !strings.HasPrefix(err.Error(), "publish: check job owner bld-web-gen-1: ") {
		t.Fatalf("err = %q, want the caller package's error prefix", err)
	}
	if !strings.Contains(err.Error(), "belongs to a different App lifetime") {
		t.Fatalf("err = %q, want the CheckOwner explanation", err)
	}
}
