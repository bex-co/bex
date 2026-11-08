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

package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// sqlAppsReadsThatMayIncludeBuildSources names every function (or package
// value) whose SQL reads the apps table without a build source filter, and
// why that is right (w9/m180, ADR097 Consequences). The Kubernetes-side guard
// (TestEveryAppEnumerationDeclaresBuildSources) cannot see these: a carrier
// is an apps row too, so a new query over apps must either filter carriers
// (serviceAppsOnly / buildSourcesOnly / TypeBuildSource / a build_source
// predicate), resolve rows by id, or be listed here.
var sqlAppsReadsThatMayIncludeBuildSources = map[string]string{
	"store/environments.go:ListEnvironmentServices":             "filters environment_id, which a carrier never sets (its project scope is scope_project_id)",
	"store/environments.go:ListWorkspaceEnvironmentServices":    "inner-joins environments on environment_id, which a carrier never sets",
	"store/grouping_tx.go:ReclaimEmptyBlueprintGroupings":       "counts members by environment_id / project_id, which a carrier never sets",
	"store/projects.go:ListServicesForProjects":                 "filters project_id, which a carrier never sets",
	"store/product_inventory.go:populateProductInventorySample": "inventory records every app under its own type, build_source included; a carrier has no domains",
	"store/store.go:CountWorkspaceDomainClaims":                 "a carrier claims no domain",
	"store/store.go:ListDesiredApps":                            "the reconciler projects every App, carriers included",
	"store/store.go:listAppsWhere":                              "callers pass serviceAppsOnly or buildSourcesOnly",
}

// appsTableRead matches SQL that reads the apps table (not apps_*).
var appsTableRead = regexp.MustCompile(`(?is)\b(from|join)\s+apps\b`)

// appsLookupByID matches a read that resolves named rows by id rather than
// enumerating: FROM apps [a] [LEFT JOIN x ON …] WHERE [a.]id = $n / = ANY($n).
var appsLookupByID = regexp.MustCompile(`(?is)\bfrom\s+apps(\s+a)?(\s+(left\s+)?join\s+\w+(\s+\w+)?\s+on\s+[^;]*?)?\s+where\s+(a\.)?id\s*=`)

func TestEverySQLAppsReadDeclaresBuildSources(t *testing.T) {
	root := filepath.Join("..")
	var unclassified []string
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		check := func(name string, decl ast.Node) {
			reads, filtered := sqlAppsReads(decl)
			if len(reads) == 0 {
				return
			}
			key := rel + ":" + name
			seen[key] = true
			if filtered || allLookupsByID(reads) {
				return
			}
			if _, ok := sqlAppsReadsThatMayIncludeBuildSources[key]; !ok {
				unclassified = append(unclassified, key)
			}
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				check(d.Name.Name, d)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Names) > 0 {
						check(vs.Names[0].Name, vs)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(unclassified)
	for _, key := range unclassified {
		t.Errorf("%s reads the apps table without filtering build sources (serviceAppsOnly / buildSourcesOnly) and is not in sqlAppsReadsThatMayIncludeBuildSources", key)
	}
	for key := range sqlAppsReadsThatMayIncludeBuildSources {
		if !seen[key] {
			t.Errorf("%s is listed as an apps-table read but no longer is one: drop it", key)
		}
	}
}

// sqlAppsReads returns the string literals in n that read the apps table, and
// whether n declares carrier handling: the shared filter constants, the type
// constant, or a literal build_source predicate.
func sqlAppsReads(n ast.Node) (reads []string, filtered bool) {
	ast.Inspect(n, func(x ast.Node) bool {
		switch v := x.(type) {
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				break
			}
			if appsTableRead.MatchString(v.Value) {
				reads = append(reads, v.Value)
			}
			if strings.Contains(v.Value, "build_source") {
				filtered = true
			}
		case *ast.Ident:
			switch v.Name {
			case "serviceAppsOnly", "buildSourcesOnly", "TypeBuildSource":
				filtered = true
			}
		}
		return true
	})
	return reads, filtered
}

// allLookupsByID reports whether every apps read resolves rows by id.
func allLookupsByID(reads []string) bool {
	for _, sql := range reads {
		for _, loc := range appsTableRead.FindAllStringIndex(sql, -1) {
			if !appsLookupByID.MatchString(sql[loc[0]:]) || !strings.HasPrefix(strings.ToLower(sql[loc[0]:]), "from") {
				return false
			}
		}
	}
	return true
}
