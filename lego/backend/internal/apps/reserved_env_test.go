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

// w2/m95 t003: PORT is bex's, on every write path — including the two that
// reach a service without touching the env store, a create request's envVars
// and a Blueprint manifest.

const reservedManifest = `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - key: PORT
        value: "8080"
`

const reservedGroupManifest = `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1}
envVarGroups:
  - name: shared
    envVars:
      - key: PORT
        value: "8080"
`

func TestBlueprintValidationReportsAReservedEnvKey(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}

	for _, tc := range []struct{ name, manifest, where string }{
		{"a service's envVars", reservedManifest, "api"},
		{"an envVarGroups entry", reservedGroupManifest, "shared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.ValidateBlueprint(context.Background(), "", tc.manifest, "")
			if err != nil {
				t.Fatalf("ValidateBlueprint: %v", err)
			}
			if result.Valid || len(result.Errors) == 0 {
				t.Fatalf("validation = %+v, want the reserved key refused", result)
			}
			joined := strings.Join(messagesOf(result), " | ")
			if !strings.Contains(joined, core.ReservedEnvKeySentence("PORT")) {
				t.Fatalf("errors = %s, want the reserved sentence", joined)
			}
			if !strings.Contains(joined, tc.where) {
				t.Fatalf("errors = %s, want them to name %q so the author can find the entry", joined, tc.where)
			}
		})
	}
}

func messagesOf(v BlueprintValidation) []string {
	out := make([]string, 0, len(v.Errors))
	for _, e := range v.Errors {
		out = append(out, e.Error)
	}
	return out
}

// The control: the same manifest with an ordinary key validates.
func TestBlueprintValidationAcceptsANearMissOfTheReservedKey(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := strings.ReplaceAll(reservedManifest, "key: PORT", "key: APP_PORT")
	result, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !result.Valid {
		t.Fatalf("ValidateBlueprint(APP_PORT) = %+v, %v; want valid", result, err)
	}
}

// Create refuses a reserved key before the App CR exists, so the refusal does
// not depend on whether the env store (OpenBao) is wired.
func TestCheckReservedSpecEnv(t *testing.T) {
	if err := checkReservedSpecEnv([]appv1alpha1.EnvVar{
		{Name: "LOG_LEVEL", Value: "debug"},
		{Name: "PORT", Value: "8080"},
	}); err == nil || !strings.Contains(err.Error(), core.ReservedEnvKeySentence("PORT")) {
		t.Fatalf("literal PORT = %v, want the reserved refusal", err)
	}
	if err := checkReservedSpecEnv([]appv1alpha1.EnvVar{
		{Name: "PORT", ValueFrom: &appv1alpha1.EnvVarSource{}},
	}); err != nil {
		t.Fatalf("a ValueFrom entry = %v, want it left alone (the operator resolves it)", err)
	}
	if err := checkReservedSpecEnv([]appv1alpha1.EnvVar{
		{Name: "port", Value: "8080"},
		{Name: "APP_PORT", Value: "8080"},
	}); err != nil {
		t.Fatalf("near-miss names = %v, want them accepted", err)
	}
}
