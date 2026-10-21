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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render lets a service set PORT, from its own envVars or a group's
// (w5/m149, replacing w2/m95's refusal): a Blueprint carrying one validates,
// and a create carrying one is accepted.
func TestBlueprintValidationAcceptsPORTAsOnRender(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	for name, manifest := range map[string]string{
		"a service's envVars": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - key: PORT
        value: "8080"
`,
		"an envVarGroups entry": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1}
envVarGroups:
  - name: shared
    envVars:
      - key: PORT
        value: "8080"
`,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
			if err != nil || !result.Valid {
				t.Fatalf("ValidateBlueprint = %+v, %v; want valid", result, err)
			}
		})
	}
}

func TestCheckSpecEnvAcceptsPORT(t *testing.T) {
	if err := checkSpecEnv([]appv1alpha1.EnvVar{{Name: "PORT", Value: "8080"}}); err != nil {
		t.Fatalf("literal PORT = %v, want it accepted", err)
	}
	if err := checkSpecEnv([]appv1alpha1.EnvVar{{Name: "BAD KEY", Value: "x"}}); err == nil {
		t.Fatal("a malformed name must still be refused")
	}
}

func messagesOf(v BlueprintValidation) []string {
	out := make([]string, 0, len(v.Errors))
	for _, e := range v.Errors {
		out = append(out, e.Error)
	}
	return out
}
