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
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// w8/092: the parser tested value != "", so an explicitly declared empty
// value was treated as omitted. A sync:false "" became a prompt (its sibling
// copy found "no such plainly-defined variable"), and "" beside
// generateValue:true passed as if no value were set. Presence now decides,
// as ADR049 D5 promises.
func TestBlueprintExplicitEmptyValueIsALiteral(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	const image = `"runtime":"image","image":{"url":"nginx:1"}`
	seededCopy := func(seed string) string {
		return `{"services":[
  {"name":"caller","type":"web",` + image + `,"envVars":[{"key":"COPY_VALUE","fromService":{"name":"target","type":"web","envVarKey":"SOURCE_VALUE"}}]},
  {"name":"target","type":"web",` + image + `,"envVars":[{"key":"SOURCE_VALUE"` + seed + `,"sync":false}]}]}`
	}
	for _, tc := range []struct {
		name, manifest, wantErr string
	}{
		{"explicit empty seed copies", seededCopy(`,"value":""`), ""},
		{"one-space seed copies", seededCopy(`,"value":" "`), ""},
		{"numeric seed copies", seededCopy(`,"value":0`), ""},
		{"omitted prompt cannot be copied", seededCopy(``), "no such plainly-defined variable"},
		{"null is refused by the schema", seededCopy(`,"value":null`), "got null, want string"},
		{"empty value beside generateValue", `{"services":[{"name":"api","type":"web",` + image + `,"envVars":[{"key":"V","value":"","generateValue":true}]}]}`, "sets both value and generateValue"},
		{"group empty value beside generateValue", `{"services":[{"name":"api","type":"web",` + image + `}],"envVarGroups":[{"name":"g","envVars":[{"key":"V","value":"","generateValue":true}]}]}`, "sets both value and generateValue"},
		{"generation alone", `{"services":[{"name":"api","type":"web",` + image + `,"envVars":[{"key":"V","generateValue":true}]}]}`, ""},
		{"empty literal with generation off", `{"services":[{"name":"api","type":"web",` + image + `,"envVars":[{"key":"V","value":"","generateValue":false}]}]}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.ValidateBlueprint(context.Background(), "", tc.manifest, "")
			if err != nil {
				t.Fatalf("ValidateBlueprint: %v", err)
			}
			joined := strings.Join(messagesOf(result), " | ")
			if tc.wantErr == "" && !result.Valid {
				t.Fatalf("validation = %s, want valid", joined)
			}
			if tc.wantErr != "" && (result.Valid || !strings.Contains(joined, tc.wantErr)) {
				t.Fatalf("validation = %+v, want %q", result, tc.wantErr)
			}
		})
	}

	// The explicit empty seed is seeded once, and known to the sibling copy.
	_, se, err := classifyServiceEnv(blueprintParseOverrides{}, bexServiceFromJSON(t, `{"name":"target","envVars":[{"key":"S","value":"","sync":false}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := se.seedLiterals["S"]; !ok || v != "" || len(se.promptKeys) != 0 {
		t.Fatalf("seed = %q (present %v), prompts %v; want an empty seed, no prompt", v, ok, se.promptKeys)
	}
	if v, ok := se.known["S"]; !ok || v != "" {
		t.Fatalf("known = %q (present %v), want the empty value", v, ok)
	}
}

func bexServiceFromJSON(t *testing.T, doc string) bexService {
	t.Helper()
	var s bexService
	if err := json.Unmarshal([]byte(doc), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// w8/092 acceptance: an explicitly empty sync:false value is seeded once as a
// present empty variable, and an unchanged re-sync keeps it (seed-once), through
// public DeployStack and the real secrets service.
func TestBlueprintEmptySeedIsSeededOnceAndRetained(t *testing.T) {
	base := &core.Base{Client: fakeClient(), Namespace: "default", Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}
	sec := &secrets.Service{Base: base, Store: newMemKV()}
	svc := &Service{Base: base, EnvSeeder: sec}
	ctx := context.Background()
	manifest := `{"services":[{"name":"web","type":"web","runtime":"image","image":{"url":"nginx:1"},"envVars":[{"key":"EMPTY_SEED","value":"","sync":false}]}]}`
	for i := 0; i < 2; i++ {
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
			t.Fatalf("apply %d: %v", i+1, err)
		}
		vars, err := sec.ListEnvVars(ctx, "web")
		if err != nil {
			t.Fatalf("list after apply %d: %v", i+1, err)
		}
		found := false
		for _, v := range vars {
			if v.Key == "EMPTY_SEED" {
				found = v.Value == ""
			}
		}
		if !found {
			t.Fatalf("after apply %d, EMPTY_SEED = %+v, want a present empty variable", i+1, vars)
		}
	}
}
