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
