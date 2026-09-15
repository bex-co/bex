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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w1/m147: the batch environment patch (REST PATCH /v1/services/{id}/environment,
// GraphQL patchServiceEnvironment, MCP) ran no quota check, while every sibling
// write path did. The dashboard's Environment editor stored and deployed a
// 614,400-byte secret file past the 512 KiB cap.

func manyEntries(prefix string, n int) map[string]string {
	out := make(map[string]string, n)
	for i := 0; i < n; i++ {
		out[fmt.Sprintf("%s%d", prefix, i)] = "v"
	}
	return out
}

// quotaService wires a counting client so a refusal can be shown to patch no App.
func quotaService(store core.SecretKV) (*Service, *patchCountingClient) {
	svc := newService(store, sampleApp("web"))
	counting := &patchCountingClient{Client: svc.Client}
	svc.Client = counting
	return svc, counting
}

func TestPatchEnvironmentRefusesAPatchPastTheQuota(t *testing.T) {
	big := strings.Repeat("x", 614_400)
	for _, tc := range []struct {
		name    string
		env     map[string]string
		files   map[string]string
		patch   EnvironmentPatch
		message string
	}{
		{
			name:    "a secret file past 512 KiB (the milestone's upload)",
			patch:   EnvironmentPatch{SecretFiles: []SecretFilePatch{{Name: "big.bin", Content: big}}},
			message: "total secret file size limit of 524288 bytes exceeded",
		},
		{
			name:    "the 501st secret file",
			files:   manyEntries("f", maxSecretFiles),
			patch:   EnvironmentPatch{SecretFiles: []SecretFilePatch{{Name: "one-more.pem", Content: "x"}}},
			message: "secret file limit of 500 exceeded",
		},
		{
			name:    "env vars past 512 KiB",
			patch:   EnvironmentPatch{EnvVars: []EnvVarPatch{{Key: "BIG", Value: big}}},
			message: "total environment variable size limit of 524288 bytes exceeded",
		},
		{
			name:    "the 501st env var",
			env:     manyEntries("KEY_", maxEnvKeys),
			patch:   EnvironmentPatch{EnvVars: []EnvVarPatch{{Key: "ONE_MORE", Value: "x"}}},
			message: "environment variable limit of 500 exceeded",
		},
	} {
		for _, mode := range []SaveMode{SaveModeDeploy, SaveModeOnly} {
			t.Run(tc.name+"/"+string(mode), func(t *testing.T) {
				store := newVersionedFakeSecretStore()
				if tc.env != nil {
					store.m[envPath("web")] = maps.Clone(tc.env)
				}
				if tc.files != nil {
					store.m[filesPath("web")] = maps.Clone(tc.files)
				}
				svc, counting := quotaService(store)

				patch := tc.patch
				patch.SaveMode = mode
				_, err := svc.PatchEnvironment(context.Background(), "web", patch)
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tc.message) {
					t.Fatalf("over-quota patch = %v, want a bad request naming %q", err, tc.message)
				}
				if strings.Contains(err.Error(), "xxxx") {
					t.Fatal("the refusal leaked the value")
				}
				if !maps.Equal(store.m[envPath("web")], tc.env) || !maps.Equal(store.m[filesPath("web")], tc.files) {
					t.Fatal("a refused patch wrote to the secret store")
				}
				if counting.patches != 0 {
					t.Fatalf("a refused patch patched the App %d times", counting.patches)
				}
			})
		}
	}
}

// The refusal is the same sentence on every surface the dashboard, the Render
// CLI and agents reach the batch patch through (t005).
func TestPatchEnvironmentQuotaRefusalIsIdenticalAcrossAdapters(t *testing.T) {
	const want = "total secret file size limit of 524288 bytes exceeded"
	big := strings.Repeat("x", 614_400)

	t.Run("REST", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		response := serveREST(newService(store, sampleApp("web")), "PATCH", "/v1/services/web/environment",
			`{"saveMode":"deploy","secretFiles":[{"name":"big.bin","content":"`+big+`"}]}`)
		if response.Code != 400 || !strings.Contains(response.Body.String(), want) {
			t.Fatalf("REST = %d %s, want 400 naming the quota", response.Code, response.Body.String())
		}
		if len(store.m[filesPath("web")]) != 0 {
			t.Fatal("REST refusal wrote the file")
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		_, err := newService(store, sampleApp("web")).GraphQLMutation()["patchServiceEnvironment"].Resolve(graphql.ResolveParams{
			Context: context.Background(),
			Args: map[string]any{
				"serviceId": "web", "saveMode": "deploy",
				"secretFiles": []any{map[string]any{"name": "big.bin", "content": big}},
			},
		})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), want) {
			t.Fatalf("GraphQL = %v, want a bad request naming the quota", err)
		}
		if len(store.m[filesPath("web")]) != 0 {
			t.Fatal("GraphQL refusal wrote the file")
		}
	})

	t.Run("MCP", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		cs := mcpSession(t, newService(store, sampleApp("web")))
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "patch_service_environment", Arguments: map[string]any{
			"serviceId": "web", "saveMode": "deploy",
			"secretFiles": []map[string]any{{"name": "big.bin", "content": big}},
		}})
		encoded, _ := json.Marshal(res)
		if err == nil && (res == nil || !res.IsError) {
			t.Fatalf("MCP accepted the over-quota patch: %s", encoded)
		}
		if !strings.Contains(string(encoded), want) && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Fatalf("MCP refusal lost the quota sentence: err=%v result=%s", err, encoded)
		}
		if len(store.m[filesPath("web")]) != 0 {
			t.Fatal("MCP refusal wrote the file")
		}
	})
}

// A mixed patch whose env half fits but whose files half does not writes
// neither: the env map already written is restored.
func TestPatchEnvironmentRestoresTheEnvMapWhenTheFilesHalfIsOverQuota(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"KEEP": "original"}
	svc, counting := quotaService(store)

	_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode:    SaveModeDeploy,
		EnvVars:     []EnvVarPatch{{Key: "NEW", Value: "fits"}},
		SecretFiles: []SecretFilePatch{{Name: "big.bin", Content: strings.Repeat("x", maxSecretMapBytes+1)}},
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("mixed over-quota patch = %v, want bad request", err)
	}
	if want := map[string]string{"KEEP": "original"}; !maps.Equal(store.m[envPath("web")], want) {
		t.Fatalf("env map after a refused mixed patch = %v, want %v", store.m[envPath("web")], want)
	}
	if len(store.m[filesPath("web")]) != 0 || counting.patches != 0 {
		t.Fatalf("refused mixed patch left files %v and %d App patches", store.m[filesPath("web")], counting.patches)
	}
}

// The revision-aware path writes one existing key; it can still push the map
// past the byte cap.
func TestPatchEnvironmentCASRefusesAValuePastTheQuota(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"TOKEN": "small"}
	svc, counting := quotaService(store)
	keys, err := svc.EnvVarKeys(context.Background(), "web")
	if err != nil || len(keys) != 1 {
		t.Fatalf("EnvVarKeys = %v, %v", keys, err)
	}

	_, err = svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode:            SaveModeDeploy,
		ExpectedEnvRevision: &keys[0].Revision,
		EnvVars:             []EnvVarPatch{{Key: "TOKEN", Value: strings.Repeat("x", maxSecretMapBytes)}},
	})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "total environment variable size limit") {
		t.Fatalf("over-quota CAS patch = %v, want the size refusal", err)
	}
	if store.m[envPath("web")]["TOKEN"] != "small" || store.casCalls != 0 || counting.patches != 0 {
		t.Fatalf("refused CAS patch wrote: value %q, %d CAS calls, %d App patches",
			store.m[envPath("web")]["TOKEN"], store.casCalls, counting.patches)
	}
}

// The control: an in-quota batch save still writes and rolls out.
func TestPatchEnvironmentInQuotaSaveStillRollsOut(t *testing.T) {
	store := newVersionedFakeSecretStore()
	svc, _ := quotaService(store)

	result, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode:    SaveModeDeploy,
		SecretFiles: []SecretFilePatch{{Name: "small.pem", Content: "fits"}},
	})
	if err != nil || !result.RolledOut {
		t.Fatalf("in-quota deploy save = %+v, %v; want rolled out", result, err)
	}
	if store.m[filesPath("web")]["small.pem"] != "fits" {
		t.Fatalf("in-quota file not stored: %v", store.m[filesPath("web")])
	}
}

// t004's rule: a map already over quota (from before w1/m147) can be shrunk,
// or edited without growing, but not grown further.
func TestPatchEnvironmentOnAnAlreadyOverQuotaMap(t *testing.T) {
	overCount := manyEntries("f", maxSecretFiles+2)

	t.Run("deleting an entry is allowed", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		store.m[filesPath("web")] = maps.Clone(overCount)
		svc, _ := quotaService(store)
		if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
			SaveMode:    SaveModeOnly,
			SecretFiles: []SecretFilePatch{{Name: "f0", Delete: true}},
		}); err != nil {
			t.Fatalf("shrinking an over-quota map = %v, want allowed", err)
		}
		if _, ok := store.m[filesPath("web")]["f0"]; ok {
			t.Fatal("the delete was not applied")
		}
	})

	t.Run("an edit that does not grow it is allowed", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		store.m[filesPath("web")] = maps.Clone(overCount)
		svc, _ := quotaService(store)
		if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
			SaveMode:    SaveModeOnly,
			SecretFiles: []SecretFilePatch{{Name: "f1", Content: "w"}},
		}); err != nil {
			t.Fatalf("a same-size edit of an over-quota map = %v, want allowed", err)
		}
	})

	t.Run("adding an entry is refused", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		store.m[filesPath("web")] = maps.Clone(overCount)
		svc, _ := quotaService(store)
		_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
			SaveMode:    SaveModeOnly,
			SecretFiles: []SecretFilePatch{{Name: "new.pem", Content: "x"}},
		})
		if !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("growing an over-quota map = %v, want bad request", err)
		}
		if !maps.Equal(store.m[filesPath("web")], overCount) {
			t.Fatal("a refused grow wrote to the store")
		}
	})
}
