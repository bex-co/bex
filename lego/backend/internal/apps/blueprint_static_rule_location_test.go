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

// w8/101: a static rule refusal names routes[i].field or headers[i].field,
// but the locator ignored it, so the error pointed at the service (or, for a
// header name, at the service's own name). It now points at the authored
// rule field and its line.
func TestBlueprintStaticRuleErrorsLocateTheRule(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	site := func(indent, routes, headers string) string {
		return indent + "services:\n" +
			indent + "  - name: site\n" +
			indent + "    type: web\n" +
			indent + "    runtime: static\n" +
			indent + "    repo: https://github.com/acme/site\n" +
			indent + "    staticPublishPath: dist\n" +
			indent + "    routes:\n" + routes +
			indent + "    headers:\n" + headers
	}
	route := func(indent, source, destination string) string {
		return indent + "      - type: redirect\n" + indent + "        source: " + source + "\n" + indent + "        destination: " + destination + "\n"
	}
	header := func(indent, path, name string) string {
		return indent + "      - path: " + path + "\n" + indent + "        name: " + name + "\n" + indent + "        value: v\n"
	}
	nest := func(body string) string { return "projects:\n  - name: p\n    environments:\n      - name: e\n" + body }
	const pad = "        "
	type tc struct{ name, manifest, wantPath, wantLine string }
	var cases []tc
	for _, placement := range []struct {
		prefix, indent string
		wrap           func(string) string
	}{{"", "", func(s string) string { return s }}, {"projects[0].environments[0].", pad, nest}} {
		in, w := placement.indent, placement.wrap
		okRoute, okHeader := route(in, "/old", "/new"), header(in, "/*", "X-Frame-Options")
		cases = append(cases,
			tc{"second route source", w(site(in, okRoute+route(in, "no-slash", "/new"), okHeader)), placement.prefix + "services[0].routes[1].source", "source: no-slash"},
			tc{"second route destination", w(site(in, okRoute+route(in, "/a", "no-slash"), okHeader)), placement.prefix + "services[0].routes[1].destination", "destination: no-slash"},
			tc{"second header path", w(site(in, okRoute, okHeader+header(in, "no-slash", "X-Ok"))), placement.prefix + "services[0].headers[1].path", "path: no-slash"},
			tc{"second header name", w(site(in, okRoute, okHeader+header(in, "/*", `"bad name"`))), placement.prefix + "services[0].headers[1].name", `name: "bad name"`},
			tc{"first route source", w(site(in, route(in, "no-slash", "/new")+okRoute, okHeader)), placement.prefix + "services[0].routes[0].source", "source: no-slash"},
		)
	}
	for _, c := range cases {
		t.Run(c.name+" "+c.wantPath, func(t *testing.T) {
			result, err := svc.ValidateBlueprint(context.Background(), "", c.manifest, "")
			if err != nil {
				t.Fatalf("ValidateBlueprint: %v", err)
			}
			if result.Valid || len(result.Errors) != 1 {
				t.Fatalf("validation = %+v, want one refusal", result)
			}
			e := result.Errors[0]
			if e.Path == nil || *e.Path != c.wantPath {
				t.Fatalf("path = %v, want %s (%s)", e.Path, c.wantPath, e.Error)
			}
			lines := strings.Split(c.manifest, "\n")
			if e.Line == nil || *e.Line < 1 || *e.Line > len(lines) || !strings.Contains(lines[*e.Line-1], c.wantLine) {
				t.Fatalf("line = %v, want the %q line (%s)", e.Line, c.wantLine, e.Error)
			}
		})
	}

	// Valid controls: combined rules, a splat redirect and an empty header value.
	valid := site("", route("", "/old", "/new")+route("", "/blog/*", "/posts/:splat"), header("", "/*", "X-Frame-Options")+"      - path: /x\n        name: X-Empty\n        value: \"\"\n")
	if result, err := svc.ValidateBlueprint(context.Background(), "", valid, ""); err != nil || !result.Valid {
		t.Fatalf("valid rules = %+v, %v; want valid", result, err)
	}
}
