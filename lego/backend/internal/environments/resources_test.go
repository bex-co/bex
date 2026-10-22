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

package environments

import (
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/188: Render's POST/DELETE /environments/{id}/resources add and remove
// members by id (each kind by its prefix), leaving the other members as they
// are, and deleting a non-empty environment is a 409.
func TestChangeResourcesAddsAndRemovesByID(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	dbA, dbB := ids.New(ids.Postgres), ids.New(ids.Postgres)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: dbA, OwnerID: "tea-a"})
	dbs.add(postgres.PostgresView{ID: dbB, OwnerID: "tea-a"})
	svc := newService(st)
	svc.Databases = dbs
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	got, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, true)
	if err != nil || !slices.Equal(got.DatabaseIDs, []string{dbA}) {
		t.Fatalf("add dbA = %+v, %v", got.DatabaseIDs, err)
	}
	// Adding another keeps the first; adding a member again changes nothing.
	got, err = svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbB, dbA}, true)
	if err != nil || len(got.DatabaseIDs) != 2 {
		t.Fatalf("add dbB = %+v, %v", got.DatabaseIDs, err)
	}
	got, err = svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, false)
	if err != nil || !slices.Equal(got.DatabaseIDs, []string{dbB}) {
		t.Fatalf("remove dbA = %+v, %v", got.DatabaseIDs, err)
	}
	// Removing a non-member is a no-op.
	if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, false); err != nil {
		t.Fatalf("remove non-member: %v", err)
	}
}

func TestChangeResourcesRefusalsChangeNothing(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	dbA := ids.New(ids.Postgres)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: dbA, OwnerID: "tea-a"})
	svc := newService(st)
	svc.Databases = dbs
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	for name, resourceIDs := range map[string][]string{
		"empty":                  {},
		"not an id":              {"nope"},
		"an id of another kind":  {ids.New(ids.Project)},
		"one bad among good ids": {dbA, "nope"},
	} {
		if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, resourceIDs, true); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s: err = %v, want a bad request", name, err)
		}
	}
	if got, _ := svc.Get(ctxAs("user-a"), e.ID); len(got.DatabaseIDs) != 0 {
		t.Errorf("a refused request changed membership: %+v", got.DatabaseIDs)
	}
	// A database outside the workspace is refused like any link.
	foreign := ids.New(ids.Postgres)
	if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{foreign}, true); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("foreign id: err = %v, want forbidden", err)
	}
}

func TestDeleteIfEmptyRefusesAnOccupiedEnvironment(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	dbA := ids.New(ids.Postgres)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: dbA, OwnerID: "tea-a"})
	svc := newService(st)
	svc.Databases = dbs
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, true); err != nil {
		t.Fatal(err)
	}

	err := svc.DeleteIfEmpty(ctxAs("user-a"), e.ID)
	var coded *core.CodedError
	if !errors.Is(err, core.ErrConflict) || !errors.As(err, &coded) || coded.Code != codeEnvironmentNotEmpty {
		t.Fatalf("delete of an occupied environment = %v, want 409 %s", err, codeEnvironmentNotEmpty)
	}
	if _, err := svc.Get(ctxAs("user-a"), e.ID); err != nil {
		t.Fatalf("the refused delete removed the environment: %v", err)
	}
	if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteIfEmpty(ctxAs("user-a"), e.ID); err != nil {
		t.Fatalf("delete of the emptied environment: %v", err)
	}
	// The confirmed flow (dashboard, GraphQL) still detaches and deletes.
	e2, _ := svc.Create(ctxAs("user-a"), "prj-1", "prod")
	if _, err := svc.ChangeResources(ctxAs("user-a"), e2.ID, []string{dbA}, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctxAs("user-a"), e2.ID); err != nil {
		t.Fatalf("Delete of an occupied environment: %v", err)
	}
}
