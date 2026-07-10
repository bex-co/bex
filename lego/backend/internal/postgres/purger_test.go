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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestWorkspacePurger_DeletesOnlyTheGivenTenantsDatabases(t *testing.T) {
	mine := sampleDatabase("mine")
	mine.Labels = map[string]string{core.LabelTenant: "tea-a"}
	other := sampleDatabase("other")
	other.Labels = map[string]string{core.LabelTenant: "tea-b"}
	svc, cl := newService(mine, other)
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("PurgeWorkspace: %v", err)
	}

	var list appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatalf("list Databases: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Name != "other" {
		t.Fatalf("Databases after purge = %+v, want only tea-b's \"other\" left", list.Items)
	}
}

func TestWorkspacePurger_NoMatchingDatabasesIsANoOp(t *testing.T) {
	svc, _ := newService(sampleDatabase("unrelated"))
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-never-existed"); err != nil {
		t.Fatalf("PurgeWorkspace on an unowned tenant should be a no-op, got: %v", err)
	}
}

func TestWorkspacePurger_SecondPurgeIsANoOp(t *testing.T) {
	d := sampleDatabase("mine")
	d.Labels = map[string]string{core.LabelTenant: "tea-a"}
	svc, cl := newService(d)
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("first purge: %v", err)
	}
	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("second purge (already-gone) should be a no-op, got: %v", err)
	}
	var list appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &list, client.MatchingLabels{core.LabelTenant: "tea-a"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("want no tea-a Databases left, got %+v", list.Items)
	}
}
