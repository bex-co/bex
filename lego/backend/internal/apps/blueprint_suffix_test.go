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
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

const suffixManifest = `
envVarGroups:
  - name: shared
    envVars:
      - {key: A, value: b}
databases:
  - name: main-db
services:
  - type: web
    name: api
    runtime: image
    image: {url: docker.io/library/nginx:1.27}
    envVars:
      - fromGroup: shared
      - key: DATABASE_URL
        fromDatabase: {name: main-db, property: connectionString}
      - key: CACHE_URL
        fromService: {type: keyvalue, name: cache, property: connectionString}
      - fromGroup: existing-group
      - key: LEGACY_URL
        fromDatabase: {name: legacy-db, property: connectionString}
  - type: keyvalue
    name: cache
    ipAllowList: []
ungrouped:
  services:
    - type: worker
      name: loose
      runtime: image
      image: {url: docker.io/library/nginx:1.27}
projects:
  - name: shop
    environments:
      - name: production
        services:
          - type: worker
            name: jobs
            runtime: image
            image: {url: docker.io/library/nginx:1.27}
            envVars:
              - key: API_HOST
                fromService: {type: web, name: api, property: host}
`

// w2/m261: create_new renames every managed resource — root, ungrouped and
// project environment lists, Key Value included — and every reference to one,
// and leaves projects, environments and undeclared targets alone.
func TestSuffixRenamesEveryResourceAndReference(t *testing.T) {
	source, ir, problems := compileBlueprintIRFor(suffixManifest, "p9vq")
	if len(problems) > 0 {
		t.Fatalf("compile: %+v", problems)
	}
	var names []string
	for _, r := range ir.Resources {
		names = append(names, r.Name)
		if r.Project != "" && (r.Project != "shop" || r.Environment != "production") {
			t.Errorf("%s grouped under %s/%s, want shop/production unsuffixed", r.Name, r.Project, r.Environment)
		}
	}
	slices.Sort(names)
	want := []string{"api-p9vq", "cache-p9vq", "jobs-p9vq", "loose-p9vq", "main-db-p9vq", "shared-p9vq"}
	if !slices.Equal(names, want) {
		t.Fatalf("resources = %v, want %v", names, want)
	}
	doc, _ := json.Marshal(source.Value)
	for _, ref := range []string{`"fromGroup":"shared-p9vq"`, `"name":"main-db-p9vq","property":"connectionString"`, `"name":"cache-p9vq"`, `"name":"api-p9vq","property":"host"`} {
		if !strings.Contains(string(doc), ref) {
			t.Errorf("references missing %s in %s", ref, doc)
		}
	}
	// A reference to a resource the file does not declare names an existing
	// workspace resource and keeps its name.
	for _, ref := range []string{`"fromGroup":"existing-group"`, `"name":"legacy-db"`} {
		if !strings.Contains(string(doc), ref) {
			t.Errorf("undeclared reference renamed: want %s in %s", ref, doc)
		}
	}
	// Source locations still point at the customer's YAML.
	if _, ok := source.Locations["#/services/0/name"]; !ok {
		t.Error("locations lost the authored service path")
	}
}

func TestSuffixEmptyIsTheAdoptCompile(t *testing.T) {
	_, plain, _ := CompileBlueprintIR(suffixManifest)
	_, none, _ := compileBlueprintIRFor(suffixManifest, "")
	if len(plain.Resources) != len(none.Resources) {
		t.Fatalf("resources %d vs %d", len(plain.Resources), len(none.Resources))
	}
	for i := range plain.Resources {
		if plain.Resources[i].Name != none.Resources[i].Name {
			t.Errorf("resource %d = %q, want the authored name %q", i, none.Resources[i].Name, plain.Resources[i].Name)
		}
	}
	// And the full stack parse resolves the suffixed references.
	if _, _, err := compileStack(DeployRequest{Manifest: suffixManifest, resourceSuffix: "p9vq"}); err != nil {
		t.Fatalf("compileStack with a suffix: %v", err)
	}
}
