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
	"maps"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w2/m95 t003: a user PORT used to be accepted by every write path, stored, and
// then dropped by the operator at runtime (appEnv appends its own container
// PORT, which beats every envFrom key) — so the dashboard showed a variable the
// process never saw. bex now refuses the key instead of lying about it.

const reservedSentence = `environment variable "PORT" is reserved`

// namesTheReservedKey accepts the sentence either as Go text or as it appears
// once a wire adapter has JSON-escaped its inner quotes.
func namesTheReservedKey(body string) bool {
	return strings.Contains(body, reservedSentence) ||
		strings.Contains(body, `environment variable \"PORT\" is reserved`)
}

func TestServiceWritesRefuseAReservedEnvKey(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(*Service) error
	}{
		{
			name: "batch patch",
			write: func(s *Service) error {
				_, err := s.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
					SaveMode: SaveModeDeploy,
					EnvVars:  []EnvVarPatch{{Key: "PORT", Value: "8080", ValueSet: true}},
				})
				return err
			},
		},
		{
			name: "single key",
			write: func(s *Service) error {
				_, err := s.SetEnvVar(context.Background(), "web", "PORT", EnvVarWrite{Value: "8080"})
				return err
			},
		},
		{
			name: "replace-all",
			write: func(s *Service) error {
				_, err := s.SetEnvVars(context.Background(), "web", []EnvVarView{{Key: "PORT", Value: "8080"}})
				return err
			},
		},
		{
			name: "seed (Blueprint sync)",
			write: func(s *Service) error {
				return s.SeedEnvVars(context.Background(), "web", map[string]string{"PORT": "8080"}, nil)
			},
		},
		{
			name: "a rename onto the reserved key",
			write: func(s *Service) error {
				_, err := s.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
					SaveMode: SaveModeDeploy,
					EnvVars:  []EnvVarPatch{{Key: "PORT", FromKey: "KEEP"}},
				})
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newVersionedFakeSecretStore()
			store.m[envPath("web")] = map[string]string{"KEEP": "original"}
			svc, counting := quotaService(store)

			err := tc.write(svc)
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), reservedSentence) {
				t.Fatalf("write = %v, want a bad request naming the reserved key", err)
			}
			if strings.Contains(err.Error(), "8080") {
				t.Fatal("the refusal leaked the value")
			}
			if want := map[string]string{"KEEP": "original"}; !maps.Equal(store.m[envPath("web")], want) {
				t.Fatalf("a refused write changed the store: %v", store.m[envPath("web")])
			}
			if counting.patches != 0 {
				t.Fatalf("a refused write patched the App %d times", counting.patches)
			}
		})
	}
}

// The escape hatch: a PORT stored before the rule existed must still be
// removable, or the refusal would strand it forever.
func TestAStoredReservedEnvKeyCanStillBeDeleted(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"PORT": "8080", "KEEP": "original"}
	svc, _ := quotaService(store)

	if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy,
		EnvVars:  []EnvVarPatch{{Key: "PORT", Delete: true}},
	}); err != nil {
		t.Fatalf("deleting a stored reserved key = %v, want it to succeed", err)
	}
	if want := map[string]string{"KEEP": "original"}; !maps.Equal(store.m[envPath("web")], want) {
		t.Fatalf("store after delete = %v, want %v", store.m[envPath("web")], want)
	}
}

// Case-sensitive and exact: only the name the operator actually injects is
// reserved. "port" and "PORTAL" are ordinary application variables.
func TestOnlyTheExactReservedNameIsRefused(t *testing.T) {
	store := newVersionedFakeSecretStore()
	svc, _ := quotaService(store)

	if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy,
		EnvVars: []EnvVarPatch{
			{Key: "port", Value: "8080", ValueSet: true},
			{Key: "PORTAL", Value: "on", ValueSet: true},
			{Key: "APP_PORT", Value: "8080", ValueSet: true},
		},
	}); err != nil {
		t.Fatalf("near-miss names = %v, want them accepted", err)
	}
	want := map[string]string{"port": "8080", "PORTAL": "on", "APP_PORT": "8080"}
	if !maps.Equal(store.m[envPath("web")], want) {
		t.Fatalf("store = %v, want %v", store.m[envPath("web")], want)
	}
}

// The refusal is the same sentence on every surface a client reaches the
// service environment through (t004's per-surface table).
func TestReservedEnvKeyRefusalIsIdenticalAcrossAdapters(t *testing.T) {
	t.Run("REST batch", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		response := serveREST(newService(store, sampleApp("web")), "PATCH", "/v1/services/web/environment",
			`{"saveMode":"deploy","envVars":[{"key":"PORT","value":"8080"}]}`)
		if response.Code != 400 || !namesTheReservedKey(response.Body.String()) {
			t.Fatalf("REST = %d %s, want 400 naming the reserved key", response.Code, response.Body.String())
		}
		if len(store.m[envPath("web")]) != 0 {
			t.Fatal("REST refusal wrote the key")
		}
	})

	t.Run("REST single key", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		response := serveREST(newService(store, sampleApp("web")), "PUT", "/v1/services/web/env-vars/PORT",
			`{"value":"8080"}`)
		if response.Code != 400 || !namesTheReservedKey(response.Body.String()) {
			t.Fatalf("REST = %d %s, want 400 naming the reserved key", response.Code, response.Body.String())
		}
		if len(store.m[envPath("web")]) != 0 {
			t.Fatal("REST refusal wrote the key")
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		_, err := newService(store, sampleApp("web")).GraphQLMutation()["patchServiceEnvironment"].Resolve(graphql.ResolveParams{
			Context: context.Background(),
			Args: map[string]any{
				"serviceId": "web", "saveMode": "deploy",
				"envVars": []any{map[string]any{"key": "PORT", "value": "8080"}},
			},
		})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), reservedSentence) {
			t.Fatalf("GraphQL = %v, want a bad request naming the reserved key", err)
		}
		if len(store.m[envPath("web")]) != 0 {
			t.Fatal("GraphQL refusal wrote the key")
		}
	})

	t.Run("MCP", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		cs := mcpSession(t, newService(store, sampleApp("web")))
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "patch_service_environment", Arguments: map[string]any{
			"serviceId": "web", "saveMode": "deploy",
			"envVars": []map[string]any{{"key": "PORT", "value": "8080"}},
		}})
		encoded, _ := json.Marshal(res)
		if err == nil && (res == nil || !res.IsError) {
			t.Fatalf("MCP accepted the reserved key: %s", encoded)
		}
		if !namesTheReservedKey(string(encoded)) && (err == nil || !namesTheReservedKey(err.Error())) {
			t.Fatalf("MCP refusal lost the sentence: err=%v result=%s", err, encoded)
		}
		if len(store.m[envPath("web")]) != 0 {
			t.Fatal("MCP refusal wrote the key")
		}
	})
}
