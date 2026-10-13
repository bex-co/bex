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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m62: Blueprint validation reported valid:true for a value no process
// environment can carry. JSON is YAML, so "\u0000" decodes to a real NUL
// byte, as in the finding's three documents; "\n" is the valid control.
const nulServiceLiteral = `{"services":[{"name":"api","type":"web","runtime":"image","image":{"url":"nginx:1"},
  "envVars":[{"key":"QA_LITERAL","value":"qa-%s-marker"}]}]}`

const nulServiceSeed = `{"services":[{"name":"api","type":"web","runtime":"image","image":{"url":"nginx:1"},
  "envVars":[{"key":"QA_SEED","value":"qa-%s-marker","sync":false}]}]}`

const nulGroupLiteral = `{"services":[{"name":"api","type":"web","runtime":"image","image":{"url":"nginx:1"}}],
  "envVarGroups":[{"name":"shared","envVars":[{"key":"QA_GROUP","value":"qa-%s-marker"}]}]}`

func TestBlueprintValidationRefusesANULEnvValue(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	for _, tc := range []struct{ name, manifest, key, where string }{
		{"service literal", nulServiceLiteral, "QA_LITERAL", "api"},
		{"service seed", nulServiceSeed, "QA_SEED", "api"},
		{"group literal", nulGroupLiteral, "QA_GROUP", "shared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.ValidateBlueprint(context.Background(), "", strings.Replace(tc.manifest, "%s", `\u0000`, 1), "")
			if err != nil {
				t.Fatalf("ValidateBlueprint: %v", err)
			}
			joined := strings.Join(messagesOf(result), " | ")
			if result.Valid || !strings.Contains(joined, core.EnvValueSentence(tc.key)) || !strings.Contains(joined, tc.where) {
				t.Fatalf("validation = %+v, want valid:false naming %s in %s", result, tc.key, tc.where)
			}
			if strings.Contains(joined, "marker") {
				t.Fatalf("errors leaked the value: %s", joined)
			}

			control, err := svc.ValidateBlueprint(context.Background(), "", strings.Replace(tc.manifest, "%s", `\n`, 1), "")
			if err != nil || !control.Valid {
				t.Fatalf("newline control = %+v, %v; want valid", control, err)
			}
		})
	}
}

// A sync:false prompt answer supplied at deploy time is held to the same rule.
func TestBlueprintPromptValueRefusesNUL(t *testing.T) {
	sync := false
	_, _, err := classifyServiceEnv(blueprintParseOverrides{envVarValues: map[string]string{"QA_PROMPT": "q\x00a"}},
		bexService{Name: "api", EnvVars: []bexEnvVar{{Key: "QA_PROMPT", Sync: &sync}}})
	if err == nil || !strings.Contains(err.Error(), core.EnvValueSentence("QA_PROMPT")) {
		t.Fatalf("NUL prompt value = %v, want the refusal", err)
	}
}

func TestCheckReservedSpecEnvRefusesNUL(t *testing.T) {
	if err := checkReservedSpecEnv([]appv1alpha1.EnvVar{{Name: "QA", Value: "q\x00a"}}); err == nil ||
		!strings.Contains(err.Error(), core.EnvValueSentence("QA")) {
		t.Fatalf("create literal NUL = %v, want the refusal", err)
	}
	if err := checkReservedSpecEnv([]appv1alpha1.EnvVar{{Name: "QA", Value: "a\nb\tc\r\n\\u0000"}}); err != nil {
		t.Fatalf("representable create literal = %v", err)
	}
}
