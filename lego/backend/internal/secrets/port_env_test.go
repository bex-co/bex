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
	"testing"
)

// Render lets a service set PORT (w5/m149, replacing w2/m95's refusal), so
// every service env write stores it; the operator then serves on it.
func TestServiceWritesStorePORT(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(*Service) error
	}{
		{"batch patch", func(s *Service) error {
			_, err := s.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
				SaveMode: SaveModeDeploy,
				EnvVars:  []EnvVarPatch{{Key: "PORT", Value: "8080", ValueSet: true}},
			})
			return err
		}},
		{"single key", func(s *Service) error {
			_, err := s.SetEnvVar(context.Background(), "web", "PORT", EnvVarWrite{Value: "8080"})
			return err
		}},
		{"replace-all", func(s *Service) error {
			_, err := s.SetEnvVars(context.Background(), "web", []EnvVarView{{Key: "KEEP", Value: "original"}, {Key: "PORT", Value: "8080"}})
			return err
		}},
		{"seed (Blueprint sync)", func(s *Service) error {
			return s.SeedEnvVars(context.Background(), "web", map[string]string{"PORT": "8080"}, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newVersionedFakeSecretStore()
			store.m[envPath("web")] = map[string]string{"KEEP": "original"}
			svc, _ := quotaService(store)
			if err := tc.write(svc); err != nil {
				t.Fatalf("write = %v, want PORT stored", err)
			}
			if got := store.m[envPath("web")]["PORT"]; got != "8080" {
				t.Fatalf("stored PORT = %q, want 8080 (store %v)", got, store.m[envPath("web")])
			}
		})
	}
}
