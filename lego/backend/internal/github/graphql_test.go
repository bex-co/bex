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

package github

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// jsonFields collects the (non-"-") JSON field names of a struct — the REST
// wire shape.
func jsonFields(v any) map[string]bool {
	t := reflect.TypeOf(v)
	out := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			out[name] = true
		}
	}
	return out
}

func gqlFields(obj *graphql.Object) map[string]bool {
	out := map[string]bool{}
	for name := range obj.Fields() {
		out[name] = true
	}
	return out
}

// The GraphQL objects must expose exactly the REST JSON fields — the one-core,
// identical-shape rule across surfaces (MCP reuses the same Repo/Connection
// structs, so it matches by construction).
func TestGraphQLFieldsMatchRESTShape(t *testing.T) {
	if a, b := jsonFields(Repo{}), gqlFields(repoGQLType); !reflect.DeepEqual(a, b) {
		t.Errorf("Repo field drift: REST=%v GraphQL=%v", a, b)
	}
	if a, b := jsonFields(Connection{}), gqlFields(gitConnectionGQLType); !reflect.DeepEqual(a, b) {
		t.Errorf("Connection field drift: REST=%v GraphQL=%v", a, b)
	}
}

func TestGraphQLResolversRoundTrip(t *testing.T) {
	st := newFakeStore()
	st.conns["default"] = store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"}
	svc := &Service{
		Base:   &core.Base{Namespace: "default"},
		GitHub: &fakeClient{login: "octo", repos: []Repo{{ID: 1, FullName: "octo/app", Private: true}}},
		Store:  st,
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: `{ gitConnection { connected accountLogin installUrl } repos { fullName private } }`,
	})
	if len(res.Errors) != 0 {
		t.Fatalf("graphql errors: %v", res.Errors)
	}
	data := res.Data.(map[string]any)
	conn := data["gitConnection"].(map[string]any)
	if conn["connected"] != true || conn["accountLogin"] != "octo" {
		t.Errorf("gitConnection = %v", conn)
	}
	repos := data["repos"].([]any)
	if len(repos) != 1 || repos[0].(map[string]any)["private"] != true {
		t.Errorf("repos = %v", repos)
	}
}

func TestGraphQLResolverErrorReturnsNullData(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default"}}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}

	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: `{ gitConnection { connected accountLogin installUrl } }`,
	})
	if len(res.Errors) != 1 {
		t.Fatalf("graphql errors = %v, want one unavailable error", res.Errors)
	}
	data := res.Data.(map[string]any)
	if data["gitConnection"] != nil {
		t.Errorf("gitConnection = %#v, want null when its root resolver fails", data["gitConnection"])
	}
}
