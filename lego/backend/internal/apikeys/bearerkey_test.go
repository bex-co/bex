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

package apikeys

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A client id as Hydra mints it, and a secret using every punctuation mark of
// Hydra's secret alphabet (letters, digits, `-._~`), underscore included.
const (
	sampleKeyID     = "0b1c2d3e-4f50-4a61-8b72-93a4b5c6d7e8"
	sampleKeySecret = "ab_CD-ef.gh~0123456789_xyz"
)

func TestBearerKeyRoundTrips(t *testing.T) {
	key := BearerKey(sampleKeyID, sampleKeySecret)
	if want := "bex_" + sampleKeyID + "_" + sampleKeySecret; key != want {
		t.Fatalf("BearerKey = %q, want %q", key, want)
	}
	id, secret, ok := ParseBearerKey(key)
	if !ok || id != sampleKeyID || secret != sampleKeySecret {
		t.Fatalf("ParseBearerKey(%q) = %q, %q, %v: the secret's own underscores must stay in the secret", key, id, secret, ok)
	}
	if !IsBearerKey(key) {
		t.Fatal("IsBearerKey refuses a key")
	}
}

// TestParseBearerKeyRefusesWhatIsNotAKey: each of these claims the prefix or
// comes close to it, and none may reach Hydra's token endpoint as a key.
func TestParseBearerKeyRefusesWhatIsNotAKey(t *testing.T) {
	for name, bearer := range map[string]string{
		"empty":                        "",
		"a Hydra access token":         "ory_at_" + sampleKeySecret,
		"a JWT":                        "eyJhbGciOiJSUzI1NiJ9.e30.sig",
		"the prefix alone":             bearerKeyPrefix,
		"no secret":                    bearerKeyPrefix + sampleKeyID,
		"an empty secret":              BearerKey(sampleKeyID, ""),
		"an empty id":                  BearerKey("", sampleKeySecret),
		"a space in the secret":        BearerKey(sampleKeyID, "se cret"),
		"a slash in the id":            BearerKey("../admin", sampleKeySecret),
		"a form separator in the key":  BearerKey(sampleKeyID, "x&client_id=other"),
		"a non-ASCII letter":           BearerKey(sampleKeyID, "sécret"),
		"an over-long id":              BearerKey(strings.Repeat("a", maxBearerKeyIDLen+1), sampleKeySecret),
		"an over-long secret":          BearerKey(sampleKeyID, strings.Repeat("a", maxBearerKeySecretLen+1)),
		"another case of the prefix":   "BEX_" + sampleKeyID + "_" + sampleKeySecret,
		"the prefix after other text":  "x" + BearerKey(sampleKeyID, sampleKeySecret),
		"a newline that splits a line": BearerKey(sampleKeyID, "secret\nInjected: 1"),
	} {
		if id, secret, ok := ParseBearerKey(bearer); ok {
			t.Errorf("%s: ParseBearerKey(%q) accepted it as id %q, secret %q", name, bearer, id, secret)
		}
	}
	if IsBearerKey("ory_at_" + sampleKeySecret) {
		t.Error("IsBearerKey claims a Hydra access token")
	}
	// The longest parts a key may have are still a key.
	longest := BearerKey(strings.Repeat("a", maxBearerKeyIDLen), strings.Repeat("b", maxBearerKeySecretLen))
	if _, _, ok := ParseBearerKey(longest); !ok {
		t.Error("ParseBearerKey refuses a key at the length limits")
	}
}

// TestCreateAnswersTheBearerKeyOnEverySurface: REST, GraphQL and MCP each
// answer a create with the same three things (the id, the secret, and the one
// string that carries both), and no list on any surface ever carries the key
// or the secret.
func TestCreateAnswersTheBearerKeyOnEverySurface(t *testing.T) {
	svc := newService(newFakeKeyStore())
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	callTool := func(name string, args map[string]any) string {
		t.Helper()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %+v", name, err, res)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		return string(raw)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	runGraphQL := func(query, field string) string {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: ctx})
		if len(res.Errors) > 0 {
			t.Fatalf("%s: %v", query, res.Errors)
		}
		raw, _ := json.Marshal(res.Data.(map[string]any)[field])
		return string(raw)
	}

	creates := map[string]string{
		"REST":    serveREST(svc, "POST", "/v1/api-keys", `{"name":"ci"}`).Body.String(),
		"GraphQL": runGraphQL(`mutation { createApiKey(name:"ci") { id name secret key } }`, "createApiKey"),
		"MCP":     callTool("create_api_key", map[string]any{"name": "ci"}),
	}
	for surface, body := range creates {
		var created struct{ ID, Secret, Key string }
		if err := json.Unmarshal([]byte(body), &created); err != nil {
			t.Fatalf("%s create answered %s: %v", surface, body, err)
		}
		if created.ID == "" || created.Secret == "" {
			t.Errorf("%s create dropped the id or the secret the exchange needs: %s", surface, body)
		}
		if created.Key != BearerKey(created.ID, created.Secret) {
			t.Errorf("%s create answered key %q, want the bearer form of its own id and secret", surface, created.Key)
		}
		if id, secret, ok := ParseBearerKey(created.Key); !ok || id != created.ID || secret != created.Secret {
			t.Errorf("%s create answered a key that does not parse back to its id and secret: %q", surface, created.Key)
		}
	}

	// No caller minted them, so they are in the workspace-wide list only.
	reads := map[string]string{
		"REST":    serveREST(svc, "GET", "/v1/api-keys?allMembers=true", "").Body.String(),
		"GraphQL": runGraphQL(`{ apiKeys(allMembers: true) { id name secret key createdAt } }`, "apiKeys"),
		"MCP":     callTool("list_api_keys", map[string]any{"allMembers": true}),
	}
	for surface, body := range reads {
		if !strings.Contains(body, "key-1") || !strings.Contains(body, "key-3") {
			t.Fatalf("%s list is missing the keys just created: %s", surface, body)
		}
		if strings.Contains(body, "s3cret") || strings.Contains(body, bearerKeyPrefix) {
			t.Errorf("%s list carries a key or a secret: %s", surface, body)
		}
	}
}
