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
)

// TestBlueprintRefusesAHostTwoServicesShare (w5/105): two declarations that
// claim one host, or a host and its www or apex sibling, are refused at the
// second declaration before anything is written. The apply used to create
// the first service and refuse the second, half-applying the Blueprint, while
// validation passed: its host checks compare a service with existing ones only.
func TestBlueprintRefusesAHostTwoServicesShare(t *testing.T) {
	service := func(name, hosts string) string {
		return "  - {type: web, name: " + name + `, runtime: image, image: {url: "nginx:1.27"}, ` + hosts + "}\n"
	}
	for _, tc := range []struct{ name, manifest, path, cites string }{
		{"the same host, however it is spelled",
			service("a", "domains: [shop.example.com]") + service("b", "domains: [www2.example.com, Shop.Example.com.]"),
			"#/services/1/domains/1", "#/services/0/domains/0"},
		{"an apex and its www",
			service("a", "domains: [example.com]") + service("b", "domains: [www.example.com]"),
			"#/services/1/domains/0", "#/services/0/domains/0"},
		{"a host its first claimant lists beside its sibling",
			service("a", "domains: [example.com, www.example.com]") + service("b", "domains: [www.example.com]"),
			"#/services/1/domains/0", "#/services/0/domains/1"},
		{"the singular domain",
			service("a", "domains: [shop.example.com]") + service("b", "domain: shop.example.com"),
			"#/services/1/domain", "#/services/0/domains/0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := "services:\n" + tc.manifest
			_, _, problems := CompileBlueprintIR(manifest)
			problem := findBlueprintProblem(problems, "BLUEPRINT_DUPLICATE_HOST")
			if problem == nil || problem.Path != tc.path || !strings.Contains(problem.Message, "already claimed at "+tc.cites+";") {
				t.Fatalf("problems = %+v, want BLUEPRINT_DUPLICATE_HOST at %s citing %s", problems, tc.path, tc.cites)
			}

			// newService verifies every domain's ownership, so without the
			// check the apply creates the first service before it refuses.
			svc, cl := newService(nil)
			_, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest})
			if err == nil || !strings.Contains(err.Error(), problem.Message) {
				t.Fatalf("apply = %v, want the validation's refusal %q", err, problem.Message)
			}
			assertNoApps(t, cl)
		})
	}

	// One service may list its own apex and www. A domain beside domains
	// claims nothing: the apply reads it only when domains is empty.
	for _, manifest := range []string{
		service("a", "domains: [example.com, www.example.com]"),
		service("a", "domains: [x.example.com], domain: y.example.com") + service("b", "domains: [y.example.com]"),
	} {
		if _, _, problems := CompileBlueprintIR("services:\n" + manifest); len(problems) != 0 {
			t.Fatalf("%s = %+v, want no problem", manifest, problems)
		}
	}
}
