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

package postgres

import (
	"context"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestCreatePostgresLandsInTheWorkspaceNamespace pins ADR043 D8 on the DIRECT
// create path (the Blueprint path is covered in internal/apps). It asserts the
// relationship — the CR is where store.WorkspaceNamespace says the workspace's
// hosting workloads live — rather than a "tea-a" literal, so renaming the
// namespace scheme cannot silently pass while breaking every link.
func TestCreatePostgresLandsInTheWorkspaceNamespace(t *testing.T) {
	svc, cl := newService()
	svc.Authz = &fakeChecker{allow: true}
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}

	view, err := svc.CreatePostgres(ctxAs("user-a"), CreatePostgresRequest{Name: "db1"})
	if err != nil {
		t.Fatalf("CreatePostgres: %v", err)
	}

	want := store.WorkspaceNamespace("tea-a")
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: want, Name: view.ID}, &db); err != nil {
		t.Fatalf("Database not created in the workspace namespace %q: %v — a secretKeyRef from an App in that namespace cannot resolve across a namespace boundary", want, err)
	}
	if db.Labels[core.LabelTenant] != "tea-a" {
		t.Errorf("tenant label = %q, want tea-a", db.Labels[core.LabelTenant])
	}
}

// TestCreatePostgresWithoutAWorkspaceStaysInTheSharedNamespace pins the
// store-off / unbound-caller path as byte-identical: there is no workspace
// namespace to resolve, so the CR belongs where it always did.
func TestCreatePostgresWithoutAWorkspaceStaysInTheSharedNamespace(t *testing.T) {
	// No Authz and no Workspace: the pre-authorization / store-off mode.
	svc, cl := newService()

	view, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "db1"})
	if err != nil {
		t.Fatalf("CreatePostgres: %v", err)
	}
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: svc.Namespace, Name: view.ID}, &db); err != nil {
		t.Fatalf("unbound create did not land in the shared namespace %q: %v", svc.Namespace, err)
	}
}

// TestListPostgresCollapsesCutoverTwins pins the read side of the ADR043 D8
// cutover window: between Step 5 and Step 9 of
// docs/runbooks/datastore-namespace-cutover.md a workspace legitimately has TWO
// Database CRs with the same metadata.name — the live one in its own namespace,
// the stale one in the shared namespace — and the label-scoped list returns
// both. GET /v1/postgres must show the id once, carrying the LIVE copy's
// fields (the same which-twin rule the App projector's indexManagedApps
// applies).
func TestListPostgresCollapsesCutoverTwins(t *testing.T) {
	twin := func(namespace, plan string) *appv1alpha1.Database {
		return &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dpg-x",
				Namespace: namespace,
				Labels:    map[string]string{core.LabelTenant: "tea-a", core.LabelWorkspace: "tea-a"},
			},
			Spec: appv1alpha1.DatabaseSpec{Name: "forumdb", Plan: plan},
		}
	}
	svc, _ := newService(twin("default", "free"), twin("tea-a", "standard"))
	svc.Authz = &fakeChecker{allow: true}

	list, err := svc.ListPostgres(ctxAs("user-a"), "tea-a")
	if err != nil {
		t.Fatalf("ListPostgres: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListPostgres returned %d rows, want 1 — the stale shared-namespace twin leaked into the view", len(list))
	}
	if list[0].ID != "dpg-x" || list[0].Plan != "standard" {
		t.Errorf("ListPostgres kept id %q with plan %q, want dpg-x with the live tea-a copy's plan (standard)",
			list[0].ID, list[0].Plan)
	}
}
