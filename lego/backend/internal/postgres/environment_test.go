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
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type fixedCreateEnvironment struct {
	assignment core.EnvironmentAssignment
	err        error
	calls      int
}

func (r *fixedCreateEnvironment) ResolveForCreate(_ context.Context, _, _ string) (core.EnvironmentAssignment, error) {
	r.calls++
	return r.assignment, r.err
}

func TestCreatePostgresEnvironmentResolution(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "unknown", err: core.ErrNotFound},
		{name: "foreign", err: core.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService()
			svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
			svc.Environments = &fixedCreateEnvironment{err: tc.err}
			_, err := svc.CreatePostgres(ctxAs("user-a"), CreatePostgresRequest{Name: "db", EnvironmentID: "env-x"})
			if !errors.Is(err, tc.err) {
				t.Fatalf("CreatePostgres error = %v, want %v", err, tc.err)
			}
			var databases appv1alpha1.DatabaseList
			if listErr := cl.List(context.Background(), &databases); listErr != nil || len(databases.Items) != 0 {
				t.Fatalf("failed resolution wrote databases: %v, err=%v", len(databases.Items), listErr)
			}
		})
	}

	resolver := &fixedCreateEnvironment{assignment: core.EnvironmentAssignment{ID: "env-staging", ProjectID: "prj-platform", WorkspaceID: "tea-a"}}
	svc, cl := newService()
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Environments = resolver
	view, err := svc.CreatePostgres(ctxAs("user-a"), CreatePostgresRequest{Name: "db", EnvironmentID: "env-staging"})
	if err != nil {
		t.Fatal(err)
	}
	if view.ProjectID != "prj-platform" || view.EnvironmentID != "env-staging" || resolver.calls != 1 {
		t.Fatalf("view = %+v, resolver calls = %d", view, resolver.calls)
	}
	var database appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: view.ID}, &database); err != nil {
		t.Fatal(err)
	}
	if database.Labels[core.LabelProject] != "prj-platform" || database.Labels[core.LabelEnvironment] != "env-staging" {
		t.Fatalf("labels = %v", database.Labels)
	}
}

// environment_test.go covers w6/m20's SetEnvironmentID: the internal/
// environments feature's write path onto a Database CR, mirroring
// ownerid_test.go's coverage of SetProjectID's sibling label.

// TestSetEnvironmentID_WritesAndClearsLabel is w6/m20/t001's regression test:
// SetEnvironmentID stamps core.LabelEnvironment on the underlying Database CR,
// and an empty environmentID clears it again — the same round-trip
// TestCreatePostgres_StampsBothLabels proves for the tenant/workspace labels.
func TestSetEnvironmentID_WritesAndClearsLabel(t *testing.T) {
	svc, cl := newService(sampleDatabase("db1"))
	svc.Authz = &fakeChecker{allow: true}

	if err := svc.SetEnvironmentID(ctxAs("user-a"), "db1", "env-1"); err != nil {
		t.Fatalf("SetEnvironmentID: %v", err)
	}
	var d appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "db1"}, &d); err != nil {
		t.Fatalf("get Database: %v", err)
	}
	if d.Labels[core.LabelEnvironment] != "env-1" {
		t.Fatalf("Database labels = %+v, want LabelEnvironment=env-1", d.Labels)
	}

	if err := svc.SetEnvironmentID(ctxAs("user-a"), "db1", ""); err != nil {
		t.Fatalf("SetEnvironmentID (clear): %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "db1"}, &d); err != nil {
		t.Fatalf("get Database: %v", err)
	}
	if _, ok := d.Labels[core.LabelEnvironment]; ok {
		t.Fatalf("Database labels = %+v, want LabelEnvironment cleared", d.Labels)
	}
}

// TestListPostgres_ReadsEnvironmentIDLabel proves ListPostgres/pgView surface
// the label ListPostgres.EnvironmentID needs for environments.Service's
// databaseIDsForEnvironment read path (w6/m20/t004) to find member Databases.
func TestListPostgres_ReadsEnvironmentIDLabel(t *testing.T) {
	d := sampleDatabase("db1")
	d.Labels = map[string]string{core.LabelEnvironment: "env-1"}
	svc, _ := newService(d)

	v, err := svc.GetPostgres(ctxAs("user-a"), "db1")
	if err != nil || v.EnvironmentID != "env-1" {
		t.Fatalf("GetPostgres = %+v, err=%v; want EnvironmentID=env-1", v, err)
	}

	list, err := svc.ListPostgres(ctxAs("user-a"), "")
	if err != nil || len(list) != 1 || list[0].EnvironmentID != "env-1" {
		t.Fatalf("ListPostgres = %+v, err=%v; want one instance with EnvironmentID=env-1", list, err)
	}
}
