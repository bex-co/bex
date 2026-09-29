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

package apps

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/graphql-go/graphql"
)

// undeployed_changes_test.go covers the API half of w1/m152 t003: after a deploy
// carrying saved changes is canceled, the service runs an earlier release than its
// saved spec, and every surface has to say so rather than imply the saved values are
// live. The operator owns the fact (status.undeployedChanges); these tests pin that
// the API carries it and does not invent it.

func TestViewCarriesUndeployedChanges(t *testing.T) {
	a := sampleApp("web")
	if view(a).UndeployedChanges {
		t.Fatal("an ordinary service reports undeployed changes")
	}
	a.Status.UndeployedChanges = true
	if !view(a).UndeployedChanges {
		t.Fatal("the operator's undeployedChanges did not reach the view")
	}
}

// REST and MCP share renderService. The field is a bex extension, so it must be
// absent — not `false` — on an ordinary service: a Render client then sees nothing
// it did not ask for.
func TestRenderServiceOmitsUndeployedChangesUnlessSet(t *testing.T) {
	a := sampleApp("web")
	raw, err := json.Marshal(toRenderService(view(a)))
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	_ = json.Unmarshal(raw, &wire)
	if _, present := wire["undeployedChanges"]; present {
		t.Errorf("ordinary service carries undeployedChanges on the wire: %s", raw)
	}

	a.Status.UndeployedChanges = true
	raw, _ = json.Marshal(toRenderService(view(a)))
	wire = map[string]any{}
	_ = json.Unmarshal(raw, &wire)
	if wire["undeployedChanges"] != true {
		t.Errorf("undeployedChanges = %v on the wire, want true: %s", wire["undeployedChanges"], raw)
	}
}

func TestGraphQLServiceUndeployedChanges(t *testing.T) {
	a := sampleApp("web")
	a.Status.UndeployedChanges = true
	svc, _ := newService(nil, a)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: `{ service(id:"web") { undeployedChanges } }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["service"].(map[string]any)["undeployedChanges"]
	if got != true {
		t.Errorf("GraphQL undeployedChanges = %v, want true", got)
	}
}
