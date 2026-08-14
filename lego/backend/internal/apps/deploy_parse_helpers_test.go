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

package apps

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestBlueprintCreateAPIFieldRejectWording pins the exact error bytes of the
// unified environmentId/secretFiles reject (rejectCreateAPIFields) through
// each parser, for all three nouns — the messages predate the shared helper
// and must never drift.
func TestBlueprintCreateAPIFieldRejectWording(t *testing.T) {
	for _, tt := range []struct {
		name  string
		parse func() error
		want  string
	}{
		{
			name: "service environmentId",
			parse: func() error {
				_, _, err := parseService(blueprintParseOverrides{}, bexService{Name: "web", EnvironmentID: "evm-1"})
				return err
			},
			want: `service "web" uses environmentId, which is a create-API field, not a Render Blueprint field; nest the service under projects[].environments[].services instead`,
		},
		{
			name: "service secretFiles",
			parse: func() error {
				_, _, err := parseService(blueprintParseOverrides{}, bexService{Name: "web", SecretFiles: []secretFileInput{{}}})
				return err
			},
			want: `service "web" uses secretFiles, which Render's Blueprint schema does not support; use createService secretFiles or the secret-files API`,
		},
		{
			name: "database environmentId",
			parse: func() error {
				_, err := parseDatabase(bexDatabase{Name: "db", EnvironmentID: "evm-1"})
				return err
			},
			want: `database "db" uses environmentId, which is a create-API field, not a Render Blueprint field; nest the database under projects[].environments[].databases instead`,
		},
		{
			name: "key-value environmentId",
			parse: func() error {
				_, err := parseKeyValue(bexService{Name: "cache", Type: "keyvalue", EnvironmentID: "evm-1"})
				return err
			},
			want: `key-value "cache" uses environmentId, which is a create-API field, not a Render Blueprint field; nest it under projects[].environments[].services instead`,
		},
		{
			name: "key-value secretFiles",
			parse: func() error {
				_, err := parseKeyValue(bexService{Name: "cache", Type: "keyvalue", SecretFiles: []secretFileInput{{}}})
				return err
			},
			want: `key-value "cache" uses secretFiles, which Render's Blueprint schema does not support`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parse()
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("want named ErrBadRequest, got %v", err)
			}
			if got, want := err.Error(), core.ErrBadRequest.Error()+": "+tt.want; got != want {
				t.Errorf("error = %q, want %q", got, want)
			}
		})
	}
}

// TestParseStackDuplicateEnvGroupNameWording pins the exact prior error
// string a compiled stack with duplicate env-group names yields. The strict
// compiler's global resource registry rejects the duplicate before the
// parse-time dedup ever runs (its wording, its source location) — the
// refactor must not change which boundary answers or what it says.
func TestParseStackDuplicateEnvGroupNameWording(t *testing.T) {
	manifest := `
envVarGroups:
  - name: shared
    envVars:
      - key: A
        value: "1"
projects:
  - name: p
    environments:
      - name: staging
        envVarGroups:
          - name: shared
            envVars:
              - key: B
                value: "2"
`
	_, err := parseStack(DeployRequest{Manifest: manifest})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("duplicate env group name => named ErrBadRequest, got %v", err)
	}
	if got, want := err.Error(), core.ErrBadRequest.Error()+`: duplicate name "shared" for env_var_group (first declared at #/envVarGroups/0)`; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}
