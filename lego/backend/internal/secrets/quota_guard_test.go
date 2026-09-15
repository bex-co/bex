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
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestEveryServiceMapWriterRefusesAnOverQuotaWrite is w1/m147 t002's guard: no
// write path can bypass the per-service secret-map quota again.
//
// Mechanism: behavioural, not structural. Each exported verb that can grow a
// service's env or file map is driven with an over-quota input and must return
// core.ErrBadRequest with nothing written. A reflection sweep then fails when an
// exported verb whose name marks it as a writer is missing from the table, so a
// new writer cannot land without a quota case. A structural sweep over
// updateMapCAS/storeMap call sites was the alternative. It would have had to
// allowlist every compensation and restore path — calls that shrink a map back
// and must not be quota-checked — which is exactly the kind of exemption list
// that let batch.go's writes go unguarded for a month after ADR066 #6.
func TestEveryServiceMapWriterRefusesAnOverQuotaWrite(t *testing.T) {
	big := strings.Repeat("x", maxSecretMapBytes+1)
	ctx := context.Background()

	writers := map[string]func(*Service) error{
		"SetEnvVars": func(s *Service) error {
			_, err := s.SetEnvVars(ctx, "web", []EnvVarView{{Key: "BIG", Value: big}})
			return err
		},
		"SetEnvVar": func(s *Service) error {
			_, err := s.SetEnvVar(ctx, "web", "BIG", EnvVarWrite{Value: big})
			return err
		},
		"SeedEnvVars": func(s *Service) error {
			return s.SeedEnvVars(ctx, "web", map[string]string{"BIG": big}, nil)
		},
		"SetSecretFile": func(s *Service) error {
			_, err := s.SetSecretFile(ctx, "web", "big.bin", big)
			return err
		},
		"SeedSecretFiles": func(s *Service) error {
			return s.SeedSecretFiles(ctx, "web", []core.SecretFile{{Name: "big.bin", Content: big}})
		},
		"PatchEnvironment": func(s *Service) error {
			_, envErr := s.PatchEnvironment(ctx, "web", EnvironmentPatch{
				SaveMode: SaveModeDeploy,
				EnvVars:  []EnvVarPatch{{Key: "BIG", Value: big}},
			})
			_, fileErr := s.PatchEnvironment(ctx, "web", EnvironmentPatch{
				SaveMode:    SaveModeDeploy,
				SecretFiles: []SecretFilePatch{{Name: "big.bin", Content: big}},
			})
			if !errors.Is(envErr, core.ErrBadRequest) {
				return envErr
			}
			return fileErr
		},
	}

	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			store := newVersionedFakeSecretStore()
			svc := newService(store, sampleApp("web"))
			if err := write(svc); !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("%s with an over-quota input = %v, want core.ErrBadRequest", name, err)
			}
			if len(store.m[envPath("web")]) != 0 || len(store.m[filesPath("web")]) != 0 {
				t.Fatalf("%s wrote an over-quota map to the store", name)
			}
		})
	}

	writerName := regexp.MustCompile(`^(Set|Seed|Patch|Replace|Create|Clone|Import|Apply|Add|Upsert|Put|Store|Write|Copy|Move|Merge)`)
	base := reflect.TypeOf(&core.Base{})
	svcType := reflect.TypeOf(&Service{})
	for i := 0; i < svcType.NumMethod(); i++ {
		method := svcType.Method(i).Name
		if _, promoted := base.MethodByName(method); promoted {
			continue // core.Base's own verbs never write a secret map
		}
		if writerName.MatchString(method) {
			if _, ok := writers[method]; !ok {
				t.Errorf("exported writer %s has no over-quota case in this guard; add one", method)
			}
		}
	}
}
