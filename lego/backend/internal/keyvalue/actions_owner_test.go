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

package keyvalue

import (
	"context"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m172: keyValueActions answers "this store in this workspace" (ADR087),
// so a Key Value in the caller's non-default workspace is reachable by naming
// that workspace with ownerId, as serverActions already allows.
func TestKeyValueActionsTakesOwnerID(t *testing.T) {
	db := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-b", Namespace: "tea-b", Labels: core.TenantLabels("tea-b")},
		Spec:       appv1alpha1.KeyValueSpec{Name: "bravo", Plan: "free"},
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
	if res := run(`{ keyValueActions(id: "red-b", ownerId: "tea-b") { action outcome } }`); len(res.Errors) != 0 {
		t.Fatalf("keyValueActions with ownerId: %v", res.Errors)
	}
	// Without it the projection stays bound to the acting (default) workspace.
	if res := run(`{ keyValueActions(id: "red-b") { action } }`); len(res.Errors) == 0 {
		t.Fatalf("keyValueActions without ownerId answered for another workspace: %+v", res.Data)
	}
}
