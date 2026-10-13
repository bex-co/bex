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
	"maps"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/m62: a NUL byte cannot reach a process environment — the container
// runtime refuses the variable and the native loader drops the byte — so every
// service env writer refuses a newly supplied NUL value before any write.
var nulValues = map[string]string{"start": "\x00qa", "middle": "q\x00a", "end": "qa\x00", "whole": "\x00"}

func TestServiceEnvWritersRefuseANULValue(t *testing.T) {
	ctx := context.Background()
	writers := map[string]func(*Service, string) error{
		"batch patch": func(s *Service, v string) error {
			_, err := s.PatchEnvironment(ctx, "web", EnvironmentPatch{
				SaveMode: SaveModeDeploy, EnvVars: []EnvVarPatch{{Key: "QA", Value: v, ValueSet: true}},
			})
			return err
		},
		"single key": func(s *Service, v string) error {
			_, err := s.SetEnvVar(ctx, "web", "QA", EnvVarWrite{Value: v})
			return err
		},
		"replace-all": func(s *Service, v string) error {
			_, err := s.SetEnvVars(ctx, "web", []EnvVarView{{Key: "KEEP", Value: "original"}, {Key: "QA", Value: v}})
			return err
		},
		"seed": func(s *Service, v string) error {
			return s.SeedEnvVars(ctx, "web", map[string]string{"QA": v}, nil)
		},
	}
	for name, write := range writers {
		for where, value := range nulValues {
			t.Run(name+"/"+where, func(t *testing.T) {
				store := newVersionedFakeSecretStore()
				store.m[envPath("web")] = map[string]string{"KEEP": "original"}
				svc, counting := quotaService(store)

				err := write(svc, value)
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), core.EnvValueSentence("QA")) {
					t.Fatalf("write = %v, want a bad request naming the key", err)
				}
				if strings.Contains(err.Error(), "qa") {
					t.Fatal("the refusal leaked the value")
				}
				if want := map[string]string{"KEEP": "original"}; !maps.Equal(store.m[envPath("web")], want) {
					t.Fatalf("a refused write changed the store: %q", store.m[envPath("web")])
				}
				if counting.patches != 0 {
					t.Fatalf("a refused write patched the App %d times", counting.patches)
				}
			})
		}
	}
}

// Every representable value keeps its exact bytes, including the literal
// six-character text \u0000.
func TestServiceEnvAcceptsRepresentableValues(t *testing.T) {
	store := newVersionedFakeSecretStore()
	svc, _ := quotaService(store)
	want := map[string]string{
		"EMPTY": "", "UNICODE": "ι ü 漢", "NEWLINE": "a\nb\n", "TAB": "a\tb", "CRLF": "a\r\nb",
		"ESCAPED": `\u0000`, "NUMERIC": "0",
	}
	writes := make([]EnvVarPatch, 0, len(want))
	for k, v := range want {
		writes = append(writes, EnvVarPatch{Key: k, Value: v, ValueSet: true})
	}
	if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{SaveMode: SaveModeDeploy, EnvVars: writes}); err != nil {
		t.Fatalf("representable values = %v", err)
	}
	if !maps.Equal(store.m[envPath("web")], want) {
		t.Fatalf("store = %q, want %q", store.m[envPath("web")], want)
	}
}

// A NUL value stored before the rule stays repairable: it can be replaced or
// deleted, and an unrelated patch is not stranded by it.
func TestAStoredNULValueStaysRepairable(t *testing.T) {
	ctx := context.Background()
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"LEGACY": "q\x00a", "OTHER": "x"}
	svc, _ := quotaService(store)

	if _, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, EnvVars: []EnvVarPatch{{Key: "OTHER", Value: "y", ValueSet: true}},
	}); err != nil {
		t.Fatalf("unrelated patch beside a legacy value = %v", err)
	}
	if _, err := svc.SetEnvVar(ctx, "web", "LEGACY", EnvVarWrite{Value: "qa"}); err != nil {
		t.Fatalf("replacing the legacy value = %v", err)
	}
	store.m[envPath("web")]["LEGACY"] = "q\x00a"
	if _, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, EnvVars: []EnvVarPatch{{Key: "LEGACY", Delete: true}},
	}); err != nil {
		t.Fatalf("deleting the legacy value = %v", err)
	}
	if want := map[string]string{"OTHER": "y"}; !maps.Equal(store.m[envPath("web")], want) {
		t.Fatalf("store = %q, want %q", store.m[envPath("web")], want)
	}
}

// A secret file is not an environment value: its NUL bytes stay exact.
func TestSecretFileKeepsNULBytes(t *testing.T) {
	store := newVersionedFakeSecretStore()
	svc, _ := quotaService(store)
	content := "bin\x00ary\x00"
	if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, SecretFiles: []SecretFilePatch{{Name: "blob.bin", Content: content}},
	}); err != nil {
		t.Fatalf("binary secret file = %v", err)
	}
	if got := store.m[filesPath("web")]["blob.bin"]; got != content {
		t.Fatalf("file bytes = %q, want %q", got, content)
	}
}

// The REST batch refusal is the coded 400 every adapter maps; JSON "\u0000"
// decodes to a real NUL byte.
func TestRESTRefusesANULValueWithItsCode(t *testing.T) {
	store := newVersionedFakeSecretStore()
	response := serveREST(newService(store, sampleApp("web")), "PATCH", "/v1/services/web/environment",
		`{"saveMode":"deploy","envVars":[{"key":"QA","value":"q\u0000a"}]}`)
	if response.Code != 400 || !strings.Contains(response.Body.String(), "ENVIRONMENT_VARIABLE_VALUE_INVALID") {
		t.Fatalf("REST = %d %s, want 400 ENVIRONMENT_VARIABLE_VALUE_INVALID", response.Code, response.Body.String())
	}
	if len(store.m[envPath("web")]) != 0 {
		t.Fatal("REST refusal wrote the key")
	}
}
