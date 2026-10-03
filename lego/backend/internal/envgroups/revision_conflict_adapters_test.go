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

package envgroups

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/m161 t003: the dashboard fix sends a draft's original revision and relies
// on bex-api refusing it once another editor has saved. This pins the live
// sweep-49 controls — a stale token on REST, GraphQL, and MCP — to one
// ENV_GROUP_REVISION_CONFLICT meaning with no write: the winner's value and
// revision survive every attempt.
func TestStaleRevisionIsRefusedIdenticallyAcrossAdapters(t *testing.T) {
	ctx := context.Background()

	// fixture returns a group whose original revision is already stale: tab B
	// saved "winner" over it.
	fixture := func(t *testing.T) (svc *Service, gid, stale string, winner EnvironmentPatchResult) {
		t.Helper()
		svc = newService(newFakeStore())
		group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
			Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "QA_CONFLICT", Value: "initial", ValueSet: true}},
		})
		if err != nil {
			t.Fatalf("fixture group: %v", err)
		}
		winner, err = svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
			ExpectedRevision: &group.Revision, SaveMode: SaveModeOnly,
			EnvVars: []EnvVarPatch{{Key: "QA_CONFLICT", Value: "winner", ValueSet: true}},
		})
		if err != nil {
			t.Fatalf("winning patch: %v", err)
		}
		return svc, group.ID, group.Revision, winner
	}
	unchanged := func(t *testing.T, svc *Service, gid string, winner EnvironmentPatchResult) {
		t.Helper()
		if got, err := svc.GetEnvGroupVar(ctx, gid, "QA_CONFLICT"); err != nil || got.Value != "winner" {
			t.Fatalf("winner overwritten: %+v err=%v", got, err)
		}
		if refreshed, err := svc.GetEnvGroup(ctx, gid); err != nil || refreshed.Revision != winner.Revision {
			t.Fatalf("revision = %q err=%v, want %q", refreshed.Revision, err, winner.Revision)
		}
	}

	t.Run("REST", func(t *testing.T) {
		svc, gid, stale, winner := fixture(t)
		response := serveREST(svc, http.MethodPatch, "/v1/env-groups/"+gid+"/contents", `{
			"saveMode":"save_only","expectedRevision":"`+stale+`",
			"envVars":[{"key":"QA_CONFLICT","value":"stale-must-not-land"}]
		}`)
		var body map[string]any
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		if response.Code != http.StatusConflict || body["code"] != "ENV_GROUP_REVISION_CONFLICT" {
			t.Fatalf("REST = %d %s, want 409 ENV_GROUP_REVISION_CONFLICT", response.Code, response.Body.String())
		}
		unchanged(t, svc, gid, winner)
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, gid, stale, winner := fixture(t)
		_, err := svc.GraphQLMutation()["patchEnvGroupEnvironment"].Resolve(graphql.ResolveParams{Context: ctx, Args: map[string]any{
			"id": gid, "saveMode": "save_only", "expectedRevision": stale,
			"envVars": []any{map[string]any{"key": "QA_CONFLICT", "value": "stale-must-not-land"}},
		}})
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != "ENV_GROUP_REVISION_CONFLICT" || !errors.Is(err, core.ErrConflict) {
			t.Fatalf("GraphQL = %v, want ENV_GROUP_REVISION_CONFLICT", err)
		}
		unchanged(t, svc, gid, winner)
	})

	t.Run("MCP", func(t *testing.T) {
		svc, gid, stale, winner := fixture(t)
		srv := mcp.NewServer(&mcp.Implementation{Name: "bex", Version: "0"}, nil)
		svc.RegisterMCP(srv)
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
			t.Fatalf("server connect: %v", err)
		}
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatalf("client connect: %v", err)
		}
		t.Cleanup(func() { _ = cs.Close() })

		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "patch_env_group_environment", Arguments: map[string]any{
			"id": gid, "saveMode": "save_only", "expectedRevision": stale,
			"envVars": []map[string]any{{"key": "QA_CONFLICT", "value": "stale-must-not-land"}},
		}})
		encoded, _ := json.Marshal(res)
		if err != nil || res == nil || !res.IsError || !strings.Contains(string(encoded), "ENV_GROUP_REVISION_CONFLICT") {
			t.Fatalf("MCP = err=%v result=%s, want an ENV_GROUP_REVISION_CONFLICT tool error", err, encoded)
		}
		unchanged(t, svc, gid, winner)
	})
}
