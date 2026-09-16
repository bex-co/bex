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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w2/m95 t003: an env-group PORT is worse than a service PORT — the operator
// drops it for *every* linked service at once, so the group page would promise
// a value none of them runs.

const reservedSentence = `environment variable "PORT" is reserved`

func namesTheReservedKey(body string) bool {
	return strings.Contains(body, reservedSentence) ||
		strings.Contains(body, `environment variable \"PORT\" is reserved`)
}

// Every exported env-group writer refuses the reserved key, and the group's
// maps are untouched afterwards. Mirrors the shape of
// TestEveryEnvGroupMapWriterRefusesAnOverQuotaWrite.
func TestEveryEnvGroupWriterRefusesAReservedEnvKey(t *testing.T) {
	ctx := context.Background()

	writers := map[string]func(*Service, EnvGroupView) error{
		"CreateEnvGroup": func(s *Service, _ EnvGroupView) error {
			_, err := s.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name:    "reserved",
				EnvVars: []CreateEnvVarInput{{Key: "PORT", Value: "8080", ValueSet: true}},
			})
			return err
		},
		"SetEnvGroupVars": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "PORT", Value: "8080"}})
			return err
		},
		"SetEnvGroupVar": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupVar(ctx, g.ID, "PORT", "8080")
			return err
		},
		"SetEnvGroupVarInput": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupVarInput(ctx, g.ID, EnvVarView{Key: "PORT", Value: "8080", ValueSet: true})
			return err
		},
		"PatchEnvironment": func(s *Service, g EnvGroupView) error {
			_, err := s.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
				ExpectedRevision: &g.Revision,
				SaveMode:         SaveModeOnly,
				EnvVars:          []core.EnvVarPatch{{Key: "PORT", Value: "8080", ValueSet: true}},
			})
			return err
		},
	}

	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			svc := newService(newFakeStore(), sampleApp("web"))
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatalf("fixture group: %v", err)
			}

			err = write(svc, group)
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), reservedSentence) {
				t.Fatalf("%s = %v, want a bad request naming the reserved key", name, err)
			}
			if strings.Contains(err.Error(), "8080") {
				t.Fatalf("%s leaked the value in its refusal", name)
			}
			if keys := envVarKeysOf(t, svc, group.ID); len(keys) != 0 {
				t.Fatalf("%s stored %v despite refusing", name, keys)
			}
		})
	}
}

func envVarKeysOf(t *testing.T, svc *Service, gid string) []string {
	t.Helper()
	group, err := svc.GetEnvGroup(context.Background(), gid)
	if err != nil {
		t.Fatalf("get group: %v", err)
	}
	out := make([]string, 0, len(group.EnvVars))
	for _, v := range group.EnvVars {
		out = append(out, v.Key)
	}
	return out
}

// The same sentence on all three machine surfaces (t004's env-group rows).
func TestEnvGroupReservedRefusalIsIdenticalAcrossAdapters(t *testing.T) {
	ctx := context.Background()

	fixture := func(t *testing.T) (*Service, EnvGroupView) {
		t.Helper()
		svc := newService(newFakeStore(), sampleApp("web"))
		group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
		if err != nil {
			t.Fatalf("fixture group: %v", err)
		}
		return svc, group
	}

	t.Run("REST batch", func(t *testing.T) {
		svc, group := fixture(t)
		response := serveREST(svc, "PATCH", "/v1/env-groups/"+group.ID+"/contents",
			`{"saveMode":"save_only","envVars":[{"key":"PORT","value":"8080"}]}`)
		if response.Code != 400 || !namesTheReservedKey(response.Body.String()) {
			t.Fatalf("REST = %d %s, want 400 naming the reserved key", response.Code, response.Body.String())
		}
		if keys := envVarKeysOf(t, svc, group.ID); len(keys) != 0 {
			t.Fatalf("REST refusal stored %v", keys)
		}
	})

	t.Run("REST single key", func(t *testing.T) {
		svc, group := fixture(t)
		response := serveREST(svc, "PUT", "/v1/env-groups/"+group.ID+"/env-vars/PORT", `{"value":"8080"}`)
		if response.Code != 400 || !namesTheReservedKey(response.Body.String()) {
			t.Fatalf("REST = %d %s, want 400 naming the reserved key", response.Code, response.Body.String())
		}
		if keys := envVarKeysOf(t, svc, group.ID); len(keys) != 0 {
			t.Fatalf("REST refusal stored %v", keys)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, group := fixture(t)
		_, err := svc.GraphQLMutation()["patchEnvGroupEnvironment"].Resolve(graphql.ResolveParams{Context: ctx, Args: map[string]any{
			"id": group.ID, "saveMode": "save_only",
			"envVars": []any{map[string]any{"key": "PORT", "value": "8080"}},
		}})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), reservedSentence) {
			t.Fatalf("GraphQL = %v, want a bad request naming the reserved key", err)
		}
		if keys := envVarKeysOf(t, svc, group.ID); len(keys) != 0 {
			t.Fatalf("GraphQL refusal stored %v", keys)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		svc, group := fixture(t)
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
			"id": group.ID, "saveMode": "save_only",
			"envVars": []map[string]any{{"key": "PORT", "value": "8080"}},
		}})
		encoded, _ := json.Marshal(res)
		if err == nil && (res == nil || !res.IsError) {
			t.Fatalf("MCP accepted the reserved key: %s", encoded)
		}
		if !namesTheReservedKey(string(encoded)) && (err == nil || !namesTheReservedKey(err.Error())) {
			t.Fatalf("MCP refusal lost the sentence: err=%v result=%s", err, encoded)
		}
		if keys := envVarKeysOf(t, svc, group.ID); len(keys) != 0 {
			t.Fatalf("MCP refusal stored %v", keys)
		}
	})
}
