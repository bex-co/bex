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
)

// A paid-only refusal on a free plan names "instance type", but the field to
// change is the plan or the paid feature declared, never type (w4/416).
func TestBlueprintPaidRefusalsPointAtTheFieldToChange(t *testing.T) {
	for _, tc := range []struct {
		name, manifest, path string
		line                 int
	}{
		{"second free Key Value", "services:\n  - type: keyvalue\n    name: kv1\n    plan: free\n    ipAllowList: []\n  - type: keyvalue\n    name: kv2\n    plan: free\n    ipAllowList: []\n", "services[1].plan", 8},
		{"second free Postgres", "databases:\n  - name: db1\n    plan: free\n  - name: db2\n    plan: free\n", "databases[1].plan", 5},
		{"free autoscaling", "services:\n  - type: web\n    name: auto\n    runtime: image\n    image:\n      url: docker.io/traefik/whoami:v1.10.3\n    plan: free\n    scaling:\n      minInstances: 1\n      maxInstances: 3\n      targetCPUPercent: 60\n", "services[0].scaling", 9},
		{"free disk", "services:\n  - type: web\n    name: withdisk\n    runtime: image\n    image:\n      url: docker.io/traefik/whoami:v1.10.3\n    plan: free\n    disk:\n      name: data\n      mountPath: /data\n      sizeGB: 1\n", "services[0].disk", 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
			validation, err := svc.ValidateBlueprint(context.Background(), "", tc.manifest, "")
			if err != nil || validation.Valid || len(validation.Errors) != 1 {
				t.Fatalf("validation = %+v, %v", validation, err)
			}
			problem := validation.Errors[0]
			if problem.Path == nil || *problem.Path != tc.path || problem.Line == nil || *problem.Line != tc.line {
				t.Fatalf("want %s at line %d, got %v at %v: %s", tc.path, tc.line, ptrValue(problem.Path), ptrValue(problem.Line), problem.Error)
			}
		})
	}
}

func TestBlueprintErrorFieldNeverAttributesInstanceTypeToType(t *testing.T) {
	for message, want := range map[string]string{
		"autoscaling requires a paid instance type": ".scaling",
		"disks require a paid instance type":        ".disk",
		"this workspace already has a free Key Value store (kv1); delete it or choose a paid instance type": ".plan",
		// Controls: the earlier keyword fixes are unchanged (w8/055).
		"hostname example.com is already in use": ".domains",
		"unsupported type \"cron\"":              ".type",
	} {
		if got := blueprintErrorField(message); got != want {
			t.Errorf("blueprintErrorField(%q) = %q, want %q", message, got, want)
		}
	}
}

// An envVarGroups name follows the label rule, located at its name (w4/419).
func TestBlueprintEnvGroupNameFollowsTheLabelRule(t *testing.T) {
	manifest := "envVarGroups:\n  - name: \"line1\\nline2\"\n    envVars:\n      - key: A\n        value: \"1\"\n"
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	validation, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || validation.Valid || len(validation.Errors) != 1 {
		t.Fatalf("validation = %+v, %v", validation, err)
	}
	problem := validation.Errors[0]
	if problem.Path == nil || *problem.Path != "envVarGroups[0].name" || !strings.Contains(problem.Error, "control characters") {
		t.Fatalf("want envVarGroups[0].name control-character refusal, got %v: %s", ptrValue(problem.Path), problem.Error)
	}
}

// A Blueprint env var no process could start with is refused at validate
// (w4/422).
func TestBlueprintRefusesAnEnvEntryNoProcessCanStartWith(t *testing.T) {
	manifest := "services:\n  - name: web\n    type: web\n    runtime: image\n    image: {url: nginx:alpine}\n    envVars:\n      - key: BIG\n        value: " + strings.Repeat("v", core.MaxEnvEntryBytes) + "\n"
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	validation, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || validation.Valid || len(validation.Errors) != 1 || !strings.Contains(validation.Errors[0].Error, "too large") {
		t.Fatalf("validation = %+v, %v; want the size refusal", validation, err)
	}
}
