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

// w1/126: Render scopes Blueprint names by kind. A database, a Key Value and a
// service may share a name (Render's own mattermost example names its
// database after its web service); two resources of one kind may not.
const sharedNameManifest = `
databases:
  - name: shared
    plan: free
services:
  - type: keyvalue
    name: shared
    plan: free
    ipAllowList: []
  - type: web
    name: shared
    runtime: image
    image: {url: registry.example.com/shared:1}
  - type: web
    name: app
    runtime: image
    image: {url: registry.example.com/app:1}
    envVars:
      - key: DATABASE_URL
        fromDatabase: {name: shared, property: connectionString}
      - key: CACHE_URL
        fromService: {type: keyvalue, name: shared, property: connectionString}
      - key: SHARED_HOST
        fromService: {type: web, name: shared, property: host}
`

func TestBlueprintNamesMayRepeatAcrossKinds(t *testing.T) {
	st, err := parseStack(DeployRequest{Manifest: sharedNameManifest})
	if err != nil {
		t.Fatalf("a database, a Key Value and a service sharing a name were refused: %v", err)
	}
	if len(st.databases) != 1 || len(st.keyValues) != 1 || len(st.services) != 2 {
		t.Fatalf("parsed %d databases, %d Key Values, %d services; want 1, 1, 2", len(st.databases), len(st.keyValues), len(st.services))
	}
	var app parsedService
	for _, svc := range st.services {
		if svc.req.Name == "app" {
			app = svc
		}
	}
	// Each reference resolves against its own kind.
	if len(app.databaseRefs) != 1 || app.databaseRefs[0].FromDatabase.Name != "shared" {
		t.Errorf("databaseRefs = %+v, want the one fromDatabase shared", app.databaseRefs)
	}
	if len(app.kvRefs) != 1 || app.kvRefs[0].FromService.Name != "shared" {
		t.Errorf("kvRefs = %+v, want the one Key Value reference to shared", app.kvRefs)
	}
	if len(app.hostRefs) != 1 || app.hostRefs[0].target != "shared" || app.hostRefs[0].port <= 0 {
		t.Errorf("hostRefs = %+v, want the web service shared with its port", app.hostRefs)
	}
}

// The compiler's per-kind check is what refuses these; this pins the rule,
// including two services of different types, as Render refuses them.
func TestBlueprintNamesStayUniqueWithinAKind(t *testing.T) {
	for _, tc := range []struct{ name, manifest, want string }{
		{"two services of different types", `
services:
  - {type: web, name: dup, runtime: image, image: {url: a}}
  - {type: worker, name: dup, runtime: image, image: {url: b}}
`, `duplicate name "dup" for service`},
		{"two databases", `
databases:
  - {name: dup, plan: free}
  - {name: dup, plan: free}
`, `duplicate name "dup" for postgres`},
		{"two Key Values", `
services:
  - {type: keyvalue, name: dup, plan: free, ipAllowList: []}
  - {type: keyvalue, name: dup, plan: free, ipAllowList: []}
`, `duplicate name "dup" for key_value`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseStack(DeployRequest{Manifest: tc.manifest})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

// A service reference never resolves to a same-named database or Key Value.
func TestFromServiceIgnoresOtherKindsOfTheSameName(t *testing.T) {
	_, err := parseStack(DeployRequest{Manifest: `
databases:
  - {name: store, plan: free}
services:
  - type: web
    name: app
    runtime: image
    image: {url: a}
    envVars:
      - key: STORE_PORT
        fromService: {type: web, name: store, property: port}
`})
	if err == nil || !strings.Contains(err.Error(), `fromService references unknown service "store"`) {
		t.Fatalf("err = %v, want the unknown-service refusal", err)
	}
}

// Validation locates a service-reference problem at the fromService entry,
// not at an earlier fromDatabase entry naming a same-named database.
func TestBlueprintReferencePointerSkipsSameNamedDatabases(t *testing.T) {
	manifest := `
databases:
  - {name: billing, plan: free}
services:
  - type: web
    name: app
    runtime: image
    image: {url: a}
    envVars:
      - key: DATABASE_URL
        fromDatabase: {name: billing, property: connectionString}
      - key: BILLING_HOST
        fromService: {type: web, name: billing, property: host}
`
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("compile: %+v", problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// No service billing is declared or exists, so validation reports one.
	unknown := unknownServiceReferences(st, nil)
	if len(unknown) != 1 {
		t.Fatalf("unknown service references = %q, want one for billing", unknown)
	}
	var app string
	for _, resource := range ir.Resources {
		if resource.Kind == BlueprintResourceService && resource.Name == "app" {
			app = resource.SourcePath
		}
	}
	if got, want := blueprintReferencePointer(ir, unknown[0]), app+"/envVars/1"; got != want {
		t.Fatalf("pointer for %q = %q, want %q (the fromService entry, not fromDatabase)", unknown[0], got, want)
	}
}
