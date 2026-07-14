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

package registrycreds

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

func testSchema(s *Service) graphql.Schema {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: s.GraphQLMutation()}),
	})
	if err != nil {
		panic(err)
	}
	return schema
}

func TestGraphQLCreateNeverReturnsSecretAndReadsBack(t *testing.T) {
	s, _, _ := newTestService()
	schema := testSchema(s)
	ctx := context.Background()

	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `mutation { createRegistryCredential(host: "ghcr.io", username: "alice", authToken: "hunter2") { id host username status } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createRegistryCredential: %v", res.Errors)
	}
	created := res.Data.(map[string]any)["createRegistryCredential"].(map[string]any)
	if created["host"] != "ghcr.io" || created["username"] != "alice" || created["status"] != "active" {
		t.Errorf("created = %+v", created)
	}
	id := created["id"].(string)

	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ registryCredential(id: "` + id + `") { id host username } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("registryCredential: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["registryCredential"].(map[string]any)
	if got["id"] != id || got["host"] != "ghcr.io" {
		t.Errorf("get = %+v", got)
	}
}

func TestGraphQLListNeverIncludesSecrets(t *testing.T) {
	s, _, _ := newTestService()
	schema := testSchema(s)
	ctx := context.Background()

	for _, q := range []string{
		`mutation { createRegistryCredential(host: "ghcr.io", username: "alice", authToken: "hunter2") { id } }`,
		`mutation { createRegistryCredential(host: "docker.io", username: "bob", authToken: "hunter3") { id } }`,
	} {
		if res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: q}); len(res.Errors) > 0 {
			t.Fatalf("create: %v", res.Errors)
		}
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ registryCredentials { id host username } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("registryCredentials: %v", res.Errors)
	}
	list := res.Data.(map[string]any)["registryCredentials"].([]any)
	if len(list) != 2 {
		t.Fatalf("list = %+v, want 2", list)
	}
	// The schema simply has no secret field to leak — assert the raw JSON
	// response text never carries either plaintext secret, belt-and-suspenders.
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(b), "hunter2") || strings.Contains(string(b), "hunter3") {
		t.Fatalf("response leaked a secret: %s", b)
	}
}

func TestGraphQLUpdateRotatesSecretAndThreeStateExpiresAt(t *testing.T) {
	s, _, kv := newTestService()
	schema := testSchema(s)
	ctx := context.Background()

	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `mutation { createRegistryCredential(host: "ghcr.io", username: "alice", authToken: "hunter2", expiresAt: "2027-01-01T00:00:00Z") { id expiresAt } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("create: %v", res.Errors)
	}
	created := res.Data.(map[string]any)["createRegistryCredential"].(map[string]any)
	id := created["id"].(string)
	if created["expiresAt"] == "" {
		t.Fatal("create did not set expiresAt")
	}

	// Rotate username + secret; omit expiresAt => unchanged.
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `mutation { updateRegistryCredential(id: "` + id + `", username: "alice2", authToken: "hunter3") { username expiresAt } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("update: %v", res.Errors)
	}
	updated := res.Data.(map[string]any)["updateRegistryCredential"].(map[string]any)
	if updated["username"] != "alice2" || updated["expiresAt"] != created["expiresAt"] {
		t.Errorf("updated = %+v, want username=alice2 and expiresAt unchanged", updated)
	}
	secret, _ := kv.Get(ctx, secretPath(s.workspaceID(ctx), id))
	if secret["password"] != "hunter3" {
		t.Errorf("secret not rotated: %+v", secret)
	}

	// Explicit empty expiresAt clears it.
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `mutation { updateRegistryCredential(id: "` + id + `", expiresAt: "") { expiresAt status } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("clear expiresAt: %v", res.Errors)
	}
	cleared := res.Data.(map[string]any)["updateRegistryCredential"].(map[string]any)
	if cleared["expiresAt"] != "" || cleared["status"] != "active" {
		t.Errorf("cleared = %+v", cleared)
	}
}

func TestGraphQLDeleteRemovesCredential(t *testing.T) {
	s, _, _ := newTestService()
	schema := testSchema(s)
	ctx := context.Background()

	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `mutation { createRegistryCredential(host: "ghcr.io", username: "alice", authToken: "hunter2") { id } }`})
	id := res.Data.(map[string]any)["createRegistryCredential"].(map[string]any)["id"].(string)

	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `mutation { deleteRegistryCredential(id: "` + id + `") }`})
	if len(res.Errors) > 0 || res.Data.(map[string]any)["deleteRegistryCredential"] != true {
		t.Fatalf("delete: data=%+v errors=%v", res.Data, res.Errors)
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ registryCredential(id: "` + id + `") { id } }`})
	if len(res.Errors) == 0 {
		t.Fatal("get after delete should error")
	}
}
