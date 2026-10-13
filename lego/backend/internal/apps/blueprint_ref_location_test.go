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

// w8/100: a parse-time reference refusal did not name the referencing env
// entry, so validation guessed a field from words in the message and
// reported a phantom services[0].connectionPool (or the caller's image, plan
// or type) at the service header. It now locates the authored envVars entry.
func TestBlueprintReferenceErrorsLocateTheEnvEntry(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	poolDB := "databases:\n  - name: db\n    plan: free\n"
	caller := func(indent, envs string) string {
		return indent + "services:\n" +
			indent + "  - name: caller\n" +
			indent + "    type: web\n" +
			indent + "    runtime: image\n" +
			indent + "    image:\n" +
			indent + "      url: nginx:1\n" +
			indent + "    envVars:\n" + envs
	}
	env := func(indent, key, ref string) string {
		return indent + "      - key: " + key + "\n" + indent + "        " + ref + "\n"
	}
	literal := func(indent string) string { return env(indent, "PLAIN", `value: "ok"`) }
	pool := func(indent string) string {
		return env(indent, "DB_URL", "fromDatabase: {name: db, property: connectionPoolString}")
	}
	sibling := func(indent, key string) string {
		return env(indent, "COPY_VALUE", "fromService: {name: target, type: web, envVarKey: "+key+"}")
	}
	target := func(indent string) string {
		return indent + "  - name: target\n" + indent + "    type: web\n" + indent + "    runtime: image\n" + indent + "    image:\n" + indent + "      url: nginx:1\n"
	}
	nest := func(body string) string {
		return "projects:\n  - name: p\n    environments:\n      - name: e\n" + body
	}
	const pad = "        "
	cases := []struct {
		name, manifest, wantPath, wantKey string
	}{
		{"root pool first", poolDB + caller("", pool("")), "services[0].envVars[0]", "DB_URL"},
		{"root pool second", poolDB + caller("", literal("")+pool("")), "services[0].envVars[1]", "DB_URL"},
		{"nested pool second", nest(pad+"databases:\n"+pad+"  - name: db\n"+pad+"    plan: free\n"+caller(pad, literal(pad)+pool(pad))), "projects[0].environments[0].services[0].envVars[1]", "DB_URL"},
		{"unknown target", caller("", literal("")+env("", "COPY_VALUE", "fromService: {name: ghost, type: web, envVarKey: X}")), "services[0].envVars[1]", "COPY_VALUE"},
	}
	for _, key := range []string{"IMAGE", "PLAN", "TYPE", "CONNECTIONPOOL"} {
		cases = append(cases,
			struct{ name, manifest, wantPath, wantKey string }{"root missing " + key, caller("", literal("")+sibling("", key)) + target(""), "services[0].envVars[1]", "COPY_VALUE"},
			struct{ name, manifest, wantPath, wantKey string }{"nested missing " + key, nest(caller(pad, literal(pad)+sibling(pad, key)) + target(pad)), "projects[0].environments[0].services[0].envVars[1]", "COPY_VALUE"})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.ValidateBlueprint(context.Background(), "", tc.manifest, "")
			if err != nil {
				t.Fatalf("ValidateBlueprint: %v", err)
			}
			if result.Valid || len(result.Errors) != 1 {
				t.Fatalf("validation = %+v, want one refusal", result)
			}
			e := result.Errors[0]
			if e.Path == nil || *e.Path != tc.wantPath {
				t.Fatalf("path = %v, want %s (%s)", e.Path, tc.wantPath, e.Error)
			}
			// The line is the env entry's own "- key: KEY" line, not the service header.
			lines := strings.Split(tc.manifest, "\n")
			if e.Line == nil || *e.Line < 1 || *e.Line > len(lines) || !strings.Contains(lines[*e.Line-1], "key: "+tc.wantKey) {
				t.Fatalf("line = %v, want the %s entry (%s)", e.Line, tc.wantKey, e.Error)
			}
		})
	}
}

// w8/100 (sweep208): ordinary service and group env refusals also name their
// entry, so a key such as IMAGE, PLAN, TYPE or CONNECTIONPOOL no longer
// selects an unrelated or nonexistent field by keyword.
func TestBlueprintEnvValueRefusalsLocateTheEntry(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	const image = `"runtime":"image","image":{"url":"nginx:1"}`
	plain := `{"key":"PLAIN","value":"ok"}`
	conflict := func(key string) string { return `{"key":"` + key + `","value":"","generateValue":true}` }
	service := func(envs string) string {
		return `{"name":"api","type":"web",` + image + `,"envVars":[` + envs + `]}`
	}
	group := func(envs string) string { return `{"name":"shared","envVars":[` + envs + `]}` }
	root := func(services, groups string) string {
		doc := `{"services":[` + services + `]`
		if groups != "" {
			doc += `,"envVarGroups":[` + groups + `]`
		}
		return doc + `}`
	}
	nested := func(services, groups string) string {
		env := `{"name":"e","services":[` + services + `]`
		if groups != "" {
			env += `,"envVarGroups":[` + groups + `]`
		}
		return `{"projects":[{"name":"p","environments":[` + env + `}]}]}`
	}
	const nest = "projects[0].environments[0]."
	type tc struct{ name, manifest, wantPath string }
	var cases []tc
	for _, key := range []string{"IMAGE", "PLAN", "TYPE", "CONNECTIONPOOL"} {
		cases = append(cases,
			tc{"service first " + key, root(service(conflict(key)), ""), "services[0].envVars[0]"},
			tc{"service second " + key, root(service(plain+","+conflict(key)), ""), "services[0].envVars[1]"},
			tc{"nested service second " + key, nested(service(plain+","+conflict(key)), ""), nest + "services[0].envVars[1]"},
			tc{"group first " + key, root(service(plain), group(conflict(key))), "envVarGroups[0].envVars[0]"},
			tc{"group second " + key, root(service(plain), group(plain+","+conflict(key))), "envVarGroups[0].envVars[1]"},
			tc{"nested group second " + key, nested(service(plain), group(plain+","+conflict(key))), nest + "envVarGroups[0].envVars[1]"},
		)
	}
	// A group reference is refused by the pinned schema before parsing.
	cases = append(cases,
		tc{"group sync:false", root(service(plain), group(plain+`,{"key":"TYPE","value":"x","sync":false}`)), "envVarGroups[0].envVars[1]"},
	)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, err := svc.ValidateBlueprint(context.Background(), "", c.manifest, "")
			if err != nil {
				t.Fatalf("ValidateBlueprint: %v", err)
			}
			if result.Valid || len(result.Errors) != 1 || result.Errors[0].Path == nil || *result.Errors[0].Path != c.wantPath {
				t.Fatalf("validation = %+v, want one refusal at %s", result, c.wantPath)
			}
		})
	}
	// Controls keep their decisions: an empty literal alone and generation alone are valid.
	for _, ok := range []string{root(service(`{"key":"PLAN","value":""}`), group(`{"key":"TYPE","generateValue":true}`))} {
		if result, err := svc.ValidateBlueprint(context.Background(), "", ok, ""); err != nil || !result.Valid {
			t.Fatalf("control = %+v, %v; want valid", result, err)
		}
	}
}
