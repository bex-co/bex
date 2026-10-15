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

// w8/099: a sibling's fromService.envVarKey copy looked only at the source's
// own declared values, so a literal it inherits from a linked in-file group
// was refused as "no such plainly-defined variable". The copy now resolves
// against the env the source's process gets: linked groups in link order,
// then its own values.
func TestBlueprintEnvVarKeyCopiesALinkedGroupLiteral(t *testing.T) {
	const image = `"runtime":"image","image":{"url":"nginx:1"}`
	doc := func(groups, sourceEnv string) string {
		return `{"envVarGroups":[` + groups + `],"services":[
  {"name":"consumer","type":"web",` + image + `,"envVars":[{"key":"COPY","fromService":{"type":"web","name":"source","envVarKey":"VALUE"}}]},
  {"name":"source","type":"web",` + image + `,"envVars":[` + sourceEnv + `]}]}`
	}
	group := func(name, env string) string { return `{"name":"` + name + `","envVars":[` + env + `]}` }
	link := func(name string) string { return `{"fromGroup":"` + name + `"}` }
	for _, tc := range []struct {
		name, manifest, want, wantErr string
	}{
		{"linked literal", doc(group("g", `{"key":"VALUE","value":"source-182"}`), link("g")), "source-182", ""},
		{"linked zero", doc(group("g", `{"key":"VALUE","value":0}`), link("g")), "0", ""},
		{"linked empty", doc(group("g", `{"key":"VALUE","value":""}`), link("g")), "", ""},
		{"local overrides group", doc(group("g", `{"key":"VALUE","value":"group"}`), link("g")+`,{"key":"VALUE","value":"local"}`), "local", ""},
		{"later group wins", doc(group("a", `{"key":"VALUE","value":"first"}`)+","+group("b", `{"key":"VALUE","value":"second"}`), link("a")+","+link("b")), "second", ""},
		{"later generated group value masks", doc(group("a", `{"key":"VALUE","value":"first"}`)+","+group("b", `{"key":"VALUE","generateValue":true}`), link("a")+","+link("b")), "", "no such plainly-defined variable"},
		{"local prompt masks", doc(group("g", `{"key":"VALUE","value":"group"}`), link("g")+`,{"key":"VALUE","sync":false}`), "", "no such plainly-defined variable"},
		{"missing key", doc(group("g", `{"key":"OTHER","value":"x"}`), link("g")), "", "no such plainly-defined variable"},
		{"unlinked group", doc(group("g", `{"key":"VALUE","value":"x"}`), ""), "", "no such plainly-defined variable"},
		{"undeclared group linked", doc(group("g", `{"key":"VALUE","value":"x"}`), link("g")+","+link("elsewhere")), "", "no such plainly-defined variable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, ir, problems := CompileBlueprintIR(tc.manifest)
			if len(problems) > 0 {
				t.Fatalf("compile: %+v", problems)
			}
			st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parse = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			var got *string
			for _, s := range st.services {
				if s.req.Name != "consumer" {
					continue
				}
				for _, e := range s.req.Env {
					if e.Name == "COPY" {
						v := e.Value
						got = &v
					}
				}
			}
			if got == nil || *got != tc.want {
				t.Fatalf("COPY = %v, want %q", got, tc.want)
			}
		})
	}

	// The same through validation, nested under a project environment.
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	nested := `{"projects":[{"name":"p","environments":[{"name":"e","envVarGroups":[{"name":"g","envVars":[{"key":"VALUE","value":"source-182"}]}],"services":[
  {"name":"consumer","type":"web",` + image + `,"envVars":[{"key":"COPY","fromService":{"type":"web","name":"source","envVarKey":"VALUE"}}]},
  {"name":"source","type":"web",` + image + `,"envVars":[{"fromGroup":"g"}]}]}]}]}`
	if result, err := svc.ValidateBlueprint(context.Background(), "", nested, ""); err != nil || !result.Valid {
		t.Fatalf("nested linked literal = %+v, %v; want valid", result, err)
	}
}
