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

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m172: databaseActions answers "this database in this workspace" (ADR087),
// so a database in the caller's non-default workspace is reachable by naming
// that workspace with ownerId, as serverActions already allows.
func TestDatabaseActionsTakesOwnerID(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-b", Namespace: "tea-b", Labels: core.TenantLabels("tea-b")},
		Spec:       appv1alpha1.DatabaseSpec{Name: "bravo", Plan: "free"},
	}
	svc, _ := newService(db)
	svc.Workspace = memberships{"dana": {"tea-a", "tea-b"}}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	run := func(q string) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: q})
	}
	if res := run(`{ databaseActions(id: "dpg-b", ownerId: "tea-b") { action outcome } }`); len(res.Errors) != 0 {
		t.Fatalf("databaseActions with ownerId: %v", res.Errors)
	}
	// Without it the projection stays bound to the acting (default) workspace.
	if res := run(`{ databaseActions(id: "dpg-b") { action } }`); len(res.Errors) == 0 {
		t.Fatalf("databaseActions without ownerId answered for another workspace: %+v", res.Data)
	}
}
