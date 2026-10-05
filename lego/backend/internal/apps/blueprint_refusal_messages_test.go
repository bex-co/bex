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
	"regexp"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
)

// w8/054: postgresMajorVersion "10"–"12" is valid Render that bex cannot
// provision; the Blueprint refusal says so in the create API's own words
// and list, and a cron's preDeployCommand names what to do instead.
func TestBlueprintRefusalsExplainThemselves(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	for _, version := range []string{"10", "11", "12"} {
		v, err := svc.ValidateBlueprint(ctxAs("dana"), "", "databases:\n  - name: qa-db\n    plan: free\n    postgresMajorVersion: \""+version+"\"\n", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(v.Errors) != 1 {
			t.Fatalf("errors = %+v", v.Errors)
		}
		e := v.Errors[0]
		want := "postgresMajorVersion: " + postgres.UnsupportedVersionMessage(version)
		if e.Error != want || e.Path == nil || *e.Path != "databases[0].postgresMajorVersion" || e.Line == nil {
			t.Errorf("v%s entry = %q at %v, want %q", version, e.Error, ptrValue(e.Path), want)
		}
	}
	if v, _ := svc.ValidateBlueprint(ctxAs("dana"), "", "databases:\n  - name: qa-db\n    plan: free\n    postgresMajorVersion: \"13\"\n", ""); len(v.Errors) != 0 {
		t.Errorf("13 refused: %+v", v.Errors)
	}
	v, _ := svc.ValidateBlueprint(ctxAs("dana"), "", "services:\n  - type: cron\n    name: qa-cron\n    runtime: image\n    image:\n      url: busybox:1.37\n    schedule: \"* * * * *\"\n    startCommand: echo hi\n    preDeployCommand: echo hi\n", "")
	found := false
	for _, e := range v.Errors {
		found = found || strings.Contains(e.Error, "cron jobs do not run a pre-deploy phase")
	}
	if !found {
		t.Errorf("cron preDeployCommand refusal = %+v", v.Errors)
	}
}

// Every unsupported registry entry's tenant message stays free of the
// maintainer references reviewed reasons carry.
func TestBlueprintRefusalMessagesCarryNoInternalReferences(t *testing.T) {
	registry, err := renderBlueprintRegistryOnce()
	if err != nil {
		t.Fatal(err)
	}
	internal := regexp.MustCompile(`DO_NOT_DO|ADR\d|\bw\d+/|\bm\d+\b|codex|blueprint-spec|non-goal|false-schema|reachable only`)
	check := func(msg string) {
		if internal.MatchString(msg) {
			t.Errorf("tenant message leaks an internal reference: %q", msg)
		}
	}
	for pointer, c := range registry.Fields {
		if c.State == BlueprintCapabilityUnsupported {
			check(blueprintUnsupportedCapabilityMessage(registry, "field", pointer))
		}
	}
	for pointer, values := range registry.EnumValues {
		for value, c := range values {
			if c.State == BlueprintCapabilityUnsupported {
				check(blueprintUnsupportedEnumMessage(registry, "field", pointer, value))
			}
		}
	}
}
