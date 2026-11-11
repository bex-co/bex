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
	"strings"
	"testing"
)

// w8/252: a wrong `type` on a route or a fromService reference was answered
// with the six SERVICE types ("value must be one of 'web', 'worker', …"),
// because the service-field test matched any `…/services/<n>/…/type`. Each
// nested field now keeps the enum the schema states for it, and a service's
// own type, at the root or inside a project environment, still gets the
// service list.
func TestNestedTypeFieldsReportTheirOwnEnum(t *testing.T) {
	const serviceTypes = "value must be one of 'web', 'worker', 'pserv', 'cron', 'keyvalue', 'redis'"
	route := func(indent string) string {
		lines := []string{
			"services:",
			"  - type: web",
			"    name: st",
			"    runtime: static",
			"    staticPublishPath: .",
			"    routes:",
			"      - type: bogus",
			"        source: /a",
			"        destination: /b",
		}
		return indent + strings.Join(lines, "\n"+indent) + "\n"
	}
	const fromService = `services:
  - type: web
    name: api
    runtime: image
    image: {url: docker.io/library/nginx:1.27}
    envVars:
      - key: REF
        fromService:
          type: bogus
          name: api
          property: host
`
	inProject := "projects:\n  - name: shop\n    environments:\n      - name: production\n"
	for name, tc := range map[string]struct {
		manifest, path string
		want           []string // every one must appear in the message
		service        bool     // the service-type list is the right answer
	}{
		"route type at the root":    {route(""), "#/services/0/routes/0/type", []string{"'redirect'", "'rewrite'"}, false},
		"route type in a project":   {inProject + route("        "), "#/projects/0/environments/0/services/0/routes/0/type", []string{"'redirect'", "'rewrite'"}, false},
		"fromService type":          {fromService, "#/services/0/envVars/0/fromService/type", []string{"'pserv'"}, false},
		"service type at the root":  {"services:\n  - type: webb\n    name: x\n", "#/services/0/type", []string{serviceTypes}, true},
		"service type in a project": {inProject + "        services:\n          - type: webb\n            name: x\n", "#/projects/0/environments/0/services/0/type", []string{serviceTypes}, true},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, problems := CompileBlueprintIR(tc.manifest)
			var message string
			for _, p := range problems {
				if p.Path == tc.path {
					message = p.Message
				}
			}
			if message == "" {
				t.Fatalf("no problem at %s; problems = %+v", tc.path, problems)
			}
			for _, want := range tc.want {
				if !strings.Contains(message, want) {
					t.Errorf("message at %s = %q, want it to contain %s", tc.path, message, want)
				}
			}
			if !tc.service && strings.HasSuffix(message, serviceTypes) {
				t.Errorf("message at %s lists the service types: %q", tc.path, message)
			}
		})
	}
}

// The helper itself: only services/<n>/<field>, wherever the services list is.
func TestServiceFieldLocationIsTheServicesOwnField(t *testing.T) {
	for _, tc := range []struct {
		location []string
		want     bool
	}{
		{[]string{"services", "0", "type"}, true},
		{[]string{"projects", "0", "environments", "1", "services", "2", "type"}, true},
		{[]string{"services", "0", "routes", "0", "type"}, false},
		{[]string{"services", "0", "envVars", "3", "fromService", "type"}, false},
		{[]string{"services", "x", "type"}, false},
		{[]string{"databases", "0", "type"}, false},
		{[]string{"type"}, false},
	} {
		if got := blueprintServiceFieldLocation(tc.location, "type"); got != tc.want {
			t.Errorf("blueprintServiceFieldLocation(%v, type) = %v, want %v", tc.location, got, tc.want)
		}
	}
	// The free-cron exemption shares the helper (w8/241) and keeps working
	// for a cron inside a project environment.
	if !blueprintServicePlanLocation([]string{"projects", "0", "environments", "0", "services", "0", "plan"}) {
		t.Error("a service's plan inside a project environment is no longer recognized")
	}
}
