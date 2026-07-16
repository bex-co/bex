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

package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestRenderEventVerbsExist guards renderEvents' keys against silent drift:
// verbs are runtime-derived "<pkg>.<Method>" names (core.callerVerb), so a
// service-method rename would quietly demote a mapped Render event to bex
// passthrough with no failing test — the same hole the events feed closed
// with its own vocabulary guard (internal/events/vocabulary_test.go). Each
// mapped verb must name a real method on its feature package's *Service,
// except the synthetic maintenance verbs core emits by constant.
func TestRenderEventVerbsExist(t *testing.T) {
	synthetic := map[string]bool{
		core.AuditVerbMaintenanceModeEnabled:    true,
		core.AuditVerbMaintenanceModeURIUpdated: true,
	}
	methodsByPkg := map[string]map[string]bool{}
	for verb := range renderEvents {
		if synthetic[verb] {
			continue
		}
		pkg, method, ok := strings.Cut(verb, ".")
		if !ok {
			t.Errorf("renderEvents key %q is not a <pkg>.<Method> verb", verb)
			continue
		}
		if methodsByPkg[pkg] == nil {
			methodsByPkg[pkg] = serviceMethods(t, filepath.Join("..", pkg))
		}
		if !methodsByPkg[pkg][method] {
			t.Errorf("renderEvents maps %q, but %s has no *Service method %s — "+
				"the verb was renamed or removed and the Render event name is now dead", verb, pkg, method)
		}
	}
}

// serviceMethods parses a sibling feature package and returns the names of
// every method declared on its *Service receiver.
func serviceMethods(t *testing.T, dir string) map[string]bool {
	t.Helper()
	pkgs, err := parser.ParseDir(token.NewFileSet(), dir, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	out := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
					continue
				}
				if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
					if ident, ok := star.X.(*ast.Ident); ok && ident.Name == "Service" {
						out[fn.Name.Name] = true
					}
				}
			}
		}
	}
	return out
}
