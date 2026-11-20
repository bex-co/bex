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
	"fmt"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// agreement/env_var_group refusals: the plan refuses what the apply refuses.
// ApplyEnvGroup mints a group on create and persistCreate enforces the
// aggregate env quota on the created map, so a manifest whose group already
// breaches it can never apply. The plan must refuse the create instead of
// previewing one the apply rejects — the same preview/perform contract
// CheckNewDatabase keeps for databases.
//
// Only the key-count half is reachable through a manifest: the manifest
// itself is capped at 512 KiB while the byte half caps the aggregate at the
// same 512 KiB, so YAML overhead alone keeps a declared group's byte total
// under the limit. Generated values mint at apply time, so the plan sizes
// them at their fixed minted length.

// groupManifest declares one env group with literalCount one-byte literals
// and generateCount generateValue entries.
func groupManifest(literalCount, generateCount int) string {
	var b strings.Builder
	b.WriteString("envVarGroups:\n  - name: cfg\n    envVars:\n")
	for i := 0; i < literalCount; i++ {
		fmt.Fprintf(&b, "      - {key: VAR_%04d, value: %q}\n", i, "v")
	}
	for i := 0; i < generateCount; i++ {
		fmt.Fprintf(&b, "      - {key: GEN_%04d, generateValue: true}\n", i)
	}
	return b.String()
}

func TestBlueprintPlanRefusesOverQuotaEnvGroupCreate(t *testing.T) {
	cases := []struct {
		name      string
		literals  int
		generates int
		refused   bool
	}{
		{"501 literals breach the 500-key quota", 501, 0, true},
		// 500 literals plus one minted key are 501 keys at apply time.
		{"literals at the cap plus a generate breach it", 500, 1, true},
		{"500 literals are exactly the cap", 500, 0, false},
		{"499 literals plus a generate are exactly the cap", 499, 1, false},
		{"a small group", 2, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, ctx, _ := groupingStackService(t)
			svc.EnvGroups = newFakeEnvGroups()
			v, err := svc.ValidateBlueprint(ctx, "tea-a", groupManifest(tc.literals, tc.generates), "")
			if err != nil {
				t.Fatalf("ValidateBlueprint: %v", err)
			}
			if v.Valid == tc.refused {
				t.Fatalf("Valid=%v, want refused=%v (errors: %+v)", v.Valid, tc.refused, v.Errors)
			}
			if !tc.refused {
				if len(v.Errors) != 0 {
					t.Fatalf("Valid plan carries errors: %+v", v.Errors)
				}
				return
			}
			if len(v.Errors) != 1 {
				t.Fatalf("want exactly the quota refusal, got %+v", v.Errors)
			}
			got := v.Errors[0]
			if got.Code != secrets.EnvVarLimitCode {
				t.Fatalf("refusal code = %q, want %q", got.Code, secrets.EnvVarLimitCode)
			}
			if got.Path == nil || !strings.Contains(*got.Path, "envVarGroups") {
				t.Fatalf("refusal %+v is not located at the group", got)
			}
			if !strings.Contains(got.Error, "environment variable limit of 500 exceeded") {
				t.Fatalf("refusal %q does not name the quota breach", got.Error)
			}
		})
	}
}

// The apply side of the same contract: persistCreate runs
// secrets.ValidateEnvMapQuota over the created map, so the declared entries
// of a refused manifest breach it too. This pins the apply half the plan
// mirrors; if the write path ever stops enforcing the quota, this fails
// instead of the plan silently over-refusing.
func TestBlueprintApplyQuotaRefusesOverQuotaEnvGroupCreate(t *testing.T) {
	parsed, err := parseStack(DeployRequest{Manifest: groupManifest(501, 0)})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	if len(parsed.envGroups) != 1 {
		t.Fatalf("%d groups parsed, want 1", len(parsed.envGroups))
	}
	err = secrets.ValidateEnvMapQuota(parsed.envGroups[0].literals)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != secrets.EnvVarLimitCode {
		t.Fatalf("apply-side err=%v, want code %q", err, secrets.EnvVarLimitCode)
	}
}

// generatedEnvValueSize sizes a generateValue entry for the plan's quota
// check. core.GenerateValue mints 32 random bytes as standard base64, which
// is always 44 chars; pin that so the plan's accounting cannot drift from
// what the apply writes.
func TestGeneratedEnvValueQuotaSize(t *testing.T) {
	minted, err := core.GenerateValue()
	if err != nil {
		t.Fatalf("GenerateValue: %v", err)
	}
	if len(minted) != generatedEnvValueSize {
		t.Fatalf("minted %d chars, want generatedEnvValueSize=%d", len(minted), generatedEnvValueSize)
	}
}
