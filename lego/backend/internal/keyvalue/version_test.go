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
	"encoding/json"
	"net/http"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// create_key_value has no version argument, as Render's MCP tool has none: a
// new store always runs Valkey 8 (w4/743). A call naming a version is refused
// and writes nothing, and a store the tool creates reads Valkey 8.
func TestMCPCreateKeyValueOffersNoVersion(t *testing.T) {
	svc, cl := newService()
	cs := kvMCPSession(t, svc)
	defer cs.Close()
	ctx := context.Background()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name != "create_key_value" {
			continue
		}
		found = true
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		if _, ok := schema.Properties["version"]; ok {
			t.Fatalf("create_key_value advertises a version argument: %s", raw)
		}
	}
	if !found {
		t.Fatal("create_key_value is not registered")
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "create_key_value", Arguments: map[string]any{"name": "mcp7", "version": "7"}})
	if err == nil && !res.IsError {
		t.Fatalf("create_key_value with version 7 succeeded: %+v", res.StructuredContent)
	}
	if n := countKeyValues(t, cl); n != 0 {
		t.Fatalf("a refused create wrote %d KeyValue(s)", n)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "create_key_value", Arguments: map[string]any{"name": "mcp8"}})
	if err != nil || res.IsError {
		t.Fatalf("create_key_value: err=%v isError=%v", err, res.IsError)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var created struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	if created.Version != "8" {
		t.Fatalf("a new store reads version %q, want 8", created.Version)
	}
	// The spec leaves the major to the operator's default, as every Blueprint
	// create does, and the operator runs that default as Valkey 8.
	if got := getKeyValueCR(t, cl, created.ID).Spec.Version; got != "" {
		t.Fatalf("a new store's spec.version = %q, want the operator default", got)
	}
}

// A store created on Valkey 7 before w4/743 keeps running it, and every read
// surface still says so.
func TestExistingValkey7StoreStillReadsValkey7(t *testing.T) {
	const id = "red-00000000000000000007"
	svc, _ := newService(&appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: id, Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Name: "legacy-cache", Plan: "starter", Version: "7"},
		Status:     appv1alpha1.KeyValueStatus{Phase: appv1alpha1.KVPhaseReady},
	})

	w := serveREST(svc, http.MethodGet, "/v1/key-value/"+id, "")
	var rest struct {
		Version string `json:"version"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &rest) != nil || rest.Version != "7" {
		t.Fatalf("REST read = %d %s, want version 7", w.Code, w.Body.String())
	}

	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, RequestString: `{ keyValue(id:"` + id + `") { version } }`, Context: context.Background()})
	if len(gql.Errors) > 0 {
		t.Fatalf("GraphQL read: %v", gql.Errors)
	}
	if got := gql.Data.(map[string]any)["keyValue"].(map[string]any)["version"]; got != "7" {
		t.Fatalf("GraphQL read version = %v, want 7", got)
	}

	call, cleanup := kvMCPClient(t, svc)
	defer cleanup()
	if got := call("get_key_value", map[string]any{"keyValueId": id})["version"]; got != "7" {
		t.Fatalf("MCP read version = %v, want 7", got)
	}
}
