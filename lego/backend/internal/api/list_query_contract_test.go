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

package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These are deliberate omissions, documented in ADR018's list-query census.
// Every other parameter admitted by the pinned gate must be read in the
// operation's handler or a reachable REST helper. Behavioral tests complement
// this source guard: reading a query alone does not prove correct filtering.
var listQueryOmissions = map[string]map[string]string{
	"list-services":  {"includePreviews": "bex has no preview services"},
	"list-postgres":  {"region": "single-region datastore listing has no placement filter", "includeReplicas": "read replicas are not exposed as separate list records"},
	"list-key-value": {"region": "single-region datastore listing has no placement filter"},
}

func TestListOperationsConsumeAdmittedQueries(t *testing.T) {
	roots := map[string]struct{ pkg, fn string }{
		"list-services":       {"apps", "registerServiceRoutes"},
		"list-postgres":       {"postgres", "RegisterREST"},
		"list-key-value":      {"keyvalue", "handleListKeyValues"},
		"list-env-groups":     {"envgroups", "RegisterREST"},
		"list-projects":       {"projects", "RegisterREST"},
		"list-environments":   {"environments", "RegisterREST"},
		"list-blueprints":     {"apps", "registerBlueprintRoutes"},
		"list-deploys":        {"deploys", "RegisterREST"},
		"list-custom-domains": {"apps", "registerDomainRoutes"},
	}
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for op, params := range contract.allowedQuery {
		root, ok := roots[op.OperationID]
		if !ok {
			continue
		}
		seen[op.OperationID] = true
		t.Run(op.OperationID, func(t *testing.T) {
			consumed := listHandlerQueryReads(t, root.pkg, root.fn)
			for param := range params {
				if consumed[param] || listQueryOmissions[op.OperationID][param] != "" {
					continue
				}
				t.Errorf("%s admits %q but its REST handler never reads it; implement it or document a deliberate omission in ADR018", op.OperationID, param)
			}
			for param := range listQueryOmissions[op.OperationID] {
				if _, ok := params[param]; !ok {
					t.Errorf("stale omission %q", param)
				}
				if consumed[param] {
					t.Errorf("%q is now consumed; remove stale omission", param)
				}
			}
		})
	}
	for op := range roots {
		if !seen[op] {
			t.Errorf("operation disappeared from query contract: %s", op)
		}
	}
}

func listHandlerQueryReads(t *testing.T, pkg, root string) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", pkg, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	functions := map[string]*ast.FuncDecl{}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				functions[fn.Name.Name] = fn
			}
		}
	}
	reads, visited := map[string]bool{}, map[string]bool{}
	addLiteral := func(e ast.Expr) {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			v, err := strconv.Unquote(lit.Value)
			if err == nil {
				reads[v] = true
			}
		}
	}
	var visit func(string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		fn := functions[name]
		if fn == nil {
			return
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if index, ok := n.(*ast.IndexExpr); ok {
				if q, ok := index.X.(*ast.Ident); ok && q.Name == "q" {
					addLiteral(index.Index)
				}
			}
			// The environments handler explicitly refuses updated-time filters by
			// iterating a literal list and checking q.Has(key).
			if loop, ok := n.(*ast.RangeStmt); ok {
				checksQuery := false
				ast.Inspect(loop.Body, func(n ast.Node) bool {
					if c, ok := n.(*ast.CallExpr); ok {
						if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Has" {
							if q, ok := s.X.(*ast.Ident); ok && q.Name == "q" {
								checksQuery = true
							}
						}
					}
					return true
				})
				if checksQuery {
					if values, ok := loop.X.(*ast.CompositeLit); ok {
						for _, v := range values.Elts {
							addLiteral(v)
						}
					}
				}
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch callee := call.Fun.(type) {
			case *ast.Ident:
				visit(callee.Name)
			case *ast.SelectorExpr:
				owner, _ := callee.X.(*ast.Ident)
				if owner != nil && owner.Name == "core" {
					switch callee.Sel.Name {
					case "PageParams":
						reads["cursor"], reads["limit"] = true, true
					case "QueryLimit":
						reads["limit"] = true
					case "QueryList", "QueryTime", "QueryTimeWindow":
						for _, arg := range call.Args[1:] {
							addLiteral(arg)
						}
					}
				} else if callee.Sel.Name == "Get" || callee.Sel.Name == "Has" {
					if len(call.Args) == 1 {
						addLiteral(call.Args[0])
					}
				} else if owner != nil && owner.Name == "s" {
					visit(callee.Sel.Name)
				}
			}
			return true
		})
	}
	if functions[root] == nil {
		t.Fatalf("missing handler %s.%s", pkg, root)
	}
	visit(root)
	return reads
}
