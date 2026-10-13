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
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/m62: every env-group writer refuses a newly supplied NUL value, which no
// linked service's process environment could carry, and stores nothing.
func TestEveryEnvGroupWriterRefusesANULValue(t *testing.T) {
	ctx := context.Background()
	const value = "q\x00a"
	writers := map[string]func(*Service, EnvGroupView) error{
		"CreateEnvGroup": func(s *Service, _ EnvGroupView) error {
			_, err := s.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name: "nul", EnvVars: []CreateEnvVarInput{{Key: "QA", Value: value, ValueSet: true}},
			})
			return err
		},
		"SetEnvGroupVars": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "QA", Value: value}})
			return err
		},
		"SetEnvGroupVar": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupVar(ctx, g.ID, "QA", value)
			return err
		},
		"SetEnvGroupVarInput": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupVarInput(ctx, g.ID, EnvVarView{Key: "QA", Value: value, ValueSet: true})
			return err
		},
		"PatchEnvironment": func(s *Service, g EnvGroupView) error {
			_, err := s.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
				ExpectedRevision: &g.Revision, SaveMode: SaveModeOnly,
				EnvVars: []core.EnvVarPatch{{Key: "QA", Value: value, ValueSet: true}},
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
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), core.EnvValueSentence("QA")) {
				t.Fatalf("%s = %v, want a bad request naming the key", name, err)
			}
			if keys := envVarKeysOf(t, svc, group.ID); len(keys) != 0 {
				t.Fatalf("%s stored %v despite refusing", name, keys)
			}
		})
	}
}
