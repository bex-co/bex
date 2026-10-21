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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Render applies a group's PORT to every linked service (w5/m149, replacing
// w2/m95's refusal), so every env-group writer stores it.
func TestEveryEnvGroupWriterStoresPORT(t *testing.T) {
	ctx := testCtx()
	writers := map[string]func(*Service, EnvGroupView) (string, error){
		"CreateEnvGroup": func(s *Service, _ EnvGroupView) (string, error) {
			g, err := s.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name:    "with-port",
				EnvVars: []CreateEnvVarInput{{Key: "PORT", Value: "8080", ValueSet: true}},
			})
			return g.ID, err
		},
		"SetEnvGroupVars": func(s *Service, g EnvGroupView) (string, error) {
			_, err := s.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "PORT", Value: "8080"}})
			return g.ID, err
		},
		"SetEnvGroupVar": func(s *Service, g EnvGroupView) (string, error) {
			_, err := s.SetEnvGroupVar(ctx, g.ID, "PORT", "8080")
			return g.ID, err
		},
		"PatchEnvironment": func(s *Service, g EnvGroupView) (string, error) {
			_, err := s.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
				ExpectedRevision: &g.Revision,
				SaveMode:         SaveModeOnly,
				EnvVars:          []core.EnvVarPatch{{Key: "PORT", Value: "8080", ValueSet: true}},
			})
			return g.ID, err
		},
	}
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			svc := newService(newFakeStore(), sampleApp("web"))
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatalf("fixture group: %v", err)
			}
			gid, err := write(svc, group)
			if err != nil {
				t.Fatalf("%s = %v, want PORT stored", name, err)
			}
			if keys := envVarKeysOf(t, svc, gid); !slices.Contains(keys, "PORT") {
				t.Fatalf("%s did not store PORT: %v", name, keys)
			}
		})
	}
}

func envVarKeysOf(t *testing.T, svc *Service, gid string) []string {
	t.Helper()
	group, err := svc.GetEnvGroup(testCtx(), gid)
	if err != nil {
		t.Fatalf("get group: %v", err)
	}
	out := make([]string, 0, len(group.EnvVars))
	for _, v := range group.EnvVars {
		out = append(out, v.Key)
	}
	return out
}
