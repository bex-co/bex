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

package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Exercise serialization as well as domain behavior: callers must be able to
// feed the actual returned token into their next save without another read.
func TestEnvironmentPatchRevisionAcrossSurfaces(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, mode := range []SaveMode{SaveModeOnly, SaveModeDeploy} {
			t.Run(surface+"/"+string(mode), func(t *testing.T) {
				store := newVersionedFakeSecretStore()
				store.m[envPath("web")] = map[string]string{"TOKEN": "initial"}
				svc := newService(store, sampleApp("web"))
				call := environmentPatchSurfaceCall(t, svc, surface)
				revision := encodeEnvRevision(0)
				for i := range 20 {
					svc.Clock = func() time.Time { return fixedNow().Add(time.Duration(i) * time.Second) }
					// Consecutive equal values cover the no-op CAS return path.
					value := fmt.Sprintf("value-%d", i/2)
					result := call(map[string]any{"saveMode": string(mode), "expectedEnvRevision": revision,
						"envVars": []map[string]any{{"key": "TOKEN", "value": value}}})
					next, ok := result["revision"].(string)
					if !ok || next == revision {
						t.Fatalf("iteration %d: missing/unchanged revision: %#v", i, result)
					}
					snapshot, err := store.GetVersioned(context.Background(), envPath("web"))
					if err != nil {
						t.Fatal(err)
					}
					if next != encodeEnvRevision(snapshot.Version) || snapshot.Data["TOKEN"] != value {
						t.Fatalf("iteration %d: response revision %q does not describe stored state %#v", i, next, snapshot)
					}
					if wantRoll := mode == SaveModeDeploy && i%2 == 0; result["rolledOut"] != wantRoll {
						t.Fatalf("iteration %d: rolledOut = %v, want %v", i, result["rolledOut"], wantRoll)
					}
					revision = next
				}
				result := call(map[string]any{"saveMode": string(mode), "envVars": []map[string]any{{"key": "TOKEN", "value": "sparse"}}})
				if revision, exists := result["revision"]; !exists || revision != nil {
					t.Fatalf("sparse revision must be explicit null: %#v", result)
				}
			})
		}
	}
}

func environmentPatchSurfaceCall(t *testing.T, svc *Service, surface string) func(map[string]any) map[string]any {
	t.Helper()
	var call func(map[string]any) map[string]any
	switch surface {
	case "REST":
		call = func(args map[string]any) map[string]any {
			payload, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			response := serveREST(svc, http.MethodPatch, "/v1/services/web/environment", string(payload))
			if response.Code != http.StatusOK {
				t.Fatalf("PATCH: %d %s", response.Code, response.Body.String())
			}
			var result map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			return result
		}
	case "GraphQL":
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{"ok": &graphql.Field{Type: graphql.Boolean}}}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		call = func(args map[string]any) map[string]any {
			variable := args["envVars"].([]map[string]any)[0]
			result := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
				RequestString:  `mutation($mode:String!,$revision:String,$value:String!) { patchServiceEnvironment(serviceId:"web", saveMode:$mode, expectedEnvRevision:$revision, envVars:[{key:"TOKEN",value:$value}]) { revision rolledOut } }`,
				VariableValues: map[string]any{"mode": args["saveMode"], "revision": args["expectedEnvRevision"], "value": variable["value"]},
			})
			if len(result.Errors) != 0 {
				t.Fatalf("GraphQL: %v", result.Errors)
			}
			return result.Data.(map[string]any)["patchServiceEnvironment"].(map[string]any)
		}
	case "MCP":
		session := mcpSession(t, svc)
		call = func(args map[string]any) map[string]any {
			args["serviceId"] = "web"
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "patch_service_environment", Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				t.Fatalf("MCP: %#v", result)
			}
			encoded, err := json.Marshal(result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			return decoded
		}
	}
	if call == nil {
		t.Fatalf("unknown surface %q", surface)
	}
	return call
}

func TestEnvironmentPatchStaleRevisionAcrossSurfaces(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			store := newVersionedFakeSecretStore()
			store.m[envPath("web")] = map[string]string{"TOKEN": "initial"}
			svc := newService(store, sampleApp("web"))
			stale := encodeEnvRevision(0)
			call := environmentPatchSurfaceCall(t, svc, surface)
			call(map[string]any{"saveMode": "save_only", "expectedEnvRevision": stale, "envVars": []map[string]any{{"key": "TOKEN", "value": "winner"}}})
			before, err := store.GetVersioned(context.Background(), envPath("web"))
			if err != nil {
				t.Fatal(err)
			}
			const code = "ENVIRONMENT_REVISION_CONFLICT"
			switch surface {
			case "REST":
				response := serveREST(svc, http.MethodPatch, "/v1/services/web/environment", `{"saveMode":"save_only","expectedEnvRevision":"`+stale+`","envVars":[{"key":"TOKEN","value":"loser"}]}`)
				var body map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if response.Code != http.StatusConflict || body["code"] != code {
					t.Fatalf("REST conflict: %d %s", response.Code, response.Body.String())
				}
			case "GraphQL":
				_, err := svc.GraphQLMutation()["patchServiceEnvironment"].Resolve(graphql.ResolveParams{Context: context.Background(), Args: map[string]any{
					"serviceId": "web", "saveMode": "save_only", "expectedEnvRevision": stale, "envVars": []any{map[string]any{"key": "TOKEN", "value": "loser"}},
				}})
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Code != code {
					t.Fatalf("GraphQL conflict: %v", err)
				}
			case "MCP":
				result, err := mcpSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "patch_service_environment", Arguments: map[string]any{
					"serviceId": "web", "saveMode": "save_only", "expectedEnvRevision": stale, "envVars": []map[string]any{{"key": "TOKEN", "value": "loser"}},
				}})
				if err != nil {
					t.Fatal(err)
				}
				body, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				if !result.IsError || !strings.Contains(string(body), code) {
					t.Fatalf("MCP conflict: %s", body)
				}
			}
			after, err := store.GetVersioned(context.Background(), envPath("web"))
			if err != nil {
				t.Fatal(err)
			}
			if after.Version != before.Version || !maps.Equal(after.Data, before.Data) {
				t.Fatalf("conflicted writer changed source: before=%#v after=%#v", before, after)
			}
		})
	}
}

func TestEnvironmentCASNoopConsumesRevisionWithoutAppWriteOrEvent(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"TOKEN": "unchanged"}
	svc := newService(store, sampleApp("web"))
	counting := &patchCountingClient{Client: svc.Client}
	sink := &recordingAuditSink{}
	svc.Client, svc.Audit = counting, sink
	revision := encodeEnvRevision(0)
	result, err := svc.PatchEnvironment(auditCtx(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, ExpectedEnvRevision: &revision,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "unchanged"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision == nil || *result.Revision != encodeEnvRevision(1) || store.versions[envPath("web")] != 1 {
		t.Fatal("no-op did not consume and return the new revision")
	}
	if result.RolledOut || counting.patches != 0 || len(sink.events) != 0 || getApp(t, counting, "web").Spec.RestartedAt != "" {
		t.Fatalf("no-op changed App or events: rolledOut=%v patches=%d events=%d", result.RolledOut, counting.patches, len(sink.events))
	}
}
