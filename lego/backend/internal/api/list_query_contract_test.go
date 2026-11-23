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
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// These are deliberate omissions, documented in ADR018's list-query census.
// Every other parameter admitted by the pinned gate must be read in the
// operation's handler or a reachable REST helper. Behavioral tests complement
// this source guard: reading a query alone does not prove correct filtering.
var listQueryOmissions = map[string]map[string]string{
	"list-services":      {"includePreviews": "ADR018: bex has no preview services"},
	"list-build-sources": {"includePreviews": "ADR018: bex has no preview services"},
	"list-postgres":      {"includeReplicas": "ADR018: read replicas are not exposed as separate list records"},
}

type listHandlerRoot struct {
	pkg   string
	fn    string
	route string // when non-empty, inspect only the handler mounted for this exact pattern
}

// servedListHandlers maps every served Render list operation in
// unpagedListPages to the REST handler that serves it. Adding a new served
// list to the pinned OpenAPI fails TestListOperationsConsumeAdmittedQueries
// until it is wired here and every admitted query parameter is verified.
var servedListHandlers = map[string]listHandlerRoot{
	"list-services":                      {"apps", "registerServiceRoutes", "GET /v1/services"},
	"list-blueprints":                    {"apps", "registerBlueprintRoutes", "GET /v1/blueprints"},
	"list-blueprint-syncs":               {"apps", "registerBlueprintRoutes", "GET /v1/blueprints/{id}/syncs"},
	"list-build-sources":                 {"apps", "registerBuildSourceRoutes", "GET /v1/build-sources"},
	"list-notification-overrides":        {"apps", "registerNotificationOverrideRoutes", "GET /v1/notification-settings/overrides"},
	"list-custom-domains":                {"apps", "registerDomainRoutes", "GET /v1/services/{id}/custom-domains"},
	"list-routes":                        {"apps", "registerDomainRoutes", "GET /v1/services/{id}/routes"},
	"list-headers":                       {"apps", "registerDomainRoutes", "GET /v1/services/{id}/headers"},
	"list-disks":                         {"apps", "registerDiskRoutes", "GET /v1/disks"},
	"list-postgres":                      {"postgres", "RegisterREST", "GET /v1/postgres"},
	"list-key-value":                     {"keyvalue", "handleListKeyValues", ""},
	"list-redis":                         {"keyvalue", "handleListKeyValues", ""},
	"list-env-groups":                    {"envgroups", "RegisterREST", "GET /v1/env-groups"},
	"list-projects":                      {"projects", "RegisterREST", "GET /v1/projects"},
	"list-environments":                  {"environments", "RegisterREST", "GET /v1/environments"},
	"list-deploys":                       {"deploys", "RegisterREST", "GET /v1/services/{id}/deploys"},
	"list-builds-in-build-source":        {"deploys", "registerBuildSourceRoutes", "GET /v1/build-sources/{buildSourceId}/builds"},
	"list-job":                           {"jobs", "RegisterREST", "GET /v1/services/{id}/jobs"},
	"list-events":                        {"events", "RegisterREST", "GET /v1/services/{id}/events"},
	"list-owners":                        {"workspaces", "RegisterREST", "GET /v1/owners"},
	"list-owner-audit-logs":              {"audit", "serveAuditLogs", ""},
	"list-organization-audit-logs":       {"audit", "serveAuditLogs", ""},
	"list-registry-credentials":          {"registrycreds", "RegisterREST", "GET /v1/registrycredentials"},
	"get-env-vars-for-service":           {"secrets", "registerEnvRoutes", ""},
	"list-secret-files-for-service":      {"secrets", "registerEnvRoutes", ""},
	"get-env-vars-for-build-source":      {"secrets", "registerEnvRoutes", ""},
	"list-secret-files-for-build-source": {"secrets", "registerEnvRoutes", ""},
	"list-logs":                          {"logs", "logsQuery", ""},
	"list-logs-values":                   {"logs", "logsValues", ""},
	"list-webhooks":                      {"webhooks", "RegisterREST", "GET /v1/webhooks"},
	"list-webhook-events":                {"webhooks", "RegisterREST", "GET /v1/webhooks/{id}/events"},
	"list-sandboxes":                     {"sandbox", "RegisterREST", "GET /v1/sandboxes"},
	"list-sandbox-executions":            {"sandbox", "RegisterREST", "GET /v1/sandboxes/{id}/execs"},
	"list-sandbox-groups":                {"sandbox", "registerGroupREST", "GET /v1/sandbox-groups"},
	"list-sandbox-builds":                {"sandbox", "registerImageREST", "GET /v1/sandbox-groups/{sandboxGroupId}/builds"},
	"list-sandbox-snapshots":             {"sandbox", "registerSnapshotREST", "GET /v1/sandbox-groups/{sandboxGroupId}/snapshots"},
}

func TestListOperationsConsumeAdmittedQueries(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}

	// Cross-check servedListHandlers against every served paged list in the
	// pinned OpenAPI document (the same set TestEveryServedRenderListIsClassified
	// derives from contract.allowedQuery minus renderOperationOmissions and
	// unpagedListExemptions).
	for op := range unpagedListPages {
		if _, ok := servedListHandlers[op]; !ok {
			t.Errorf("served list %s is missing from servedListHandlers", op)
		}
	}
	for op := range servedListHandlers {
		if _, ok := unpagedListPages[op]; !ok {
			t.Errorf("servedListHandlers entry %s is not in unpagedListPages", op)
		}
	}

	// Collect per-operation OpenAPI parameter definitions (after compatibility
	// rewrites such as renderCommaListQueryCompatibility) so we can check both
	// consumption and explode=false array arity.
	opParams := map[*openapi3.Operation]map[string]*openapi3.Parameter{}
	for _, item := range contract.document.Paths.Map() {
		for _, operation := range item.Operations() {
			pm := map[string]*openapi3.Parameter{}
			for _, params := range []openapi3.Parameters{item.Parameters, operation.Parameters} {
				for _, ref := range params {
					if ref != nil && ref.Value != nil && ref.Value.In == openapi3.ParameterInQuery {
						pm[ref.Value.Name] = ref.Value
					}
				}
			}
			opParams[operation] = pm
		}
	}

	pkgCache := map[string]*parsedPkg{}
	seen := map[string]bool{}
	for op, params := range contract.allowedQuery {
		if op.OperationID == "" {
			continue
		}
		if _, hasLimit := params["limit"]; !hasLimit {
			continue
		}
		if _, omitted := renderOperationOmissions[op.OperationID]; omitted {
			continue
		}
		if _, exempt := unpagedListExemptions[op.OperationID]; exempt {
			continue
		}
		root, ok := servedListHandlers[op.OperationID]
		if !ok {
			t.Errorf("served list %s is not wired into servedListHandlers", op.OperationID)
			continue
		}
		seen[op.OperationID] = true
		t.Run(op.OperationID, func(t *testing.T) {
			got := listHandlerQueryReads(t, pkgCache, root)
			for param := range params {
				if listQueryOmissions[op.OperationID][param] != "" {
					continue
				}
				if !got.reads[param] {
					t.Errorf("%s admits %q but its REST handler never reads it; implement it or document a deliberate omission in ADR018", op.OperationID, param)
					continue
				}
				// Every explode=false array parameter must be parsed with a
				// comma-splitting/multi-value helper (core.QueryList or
				// core.QueryNameList), and explode=false `name` arrays must use
				// core.QueryNameList so percent-encoded literal commas survive
				// (w8/m55).
				if isExplodeFalseArray(opParams[op][param]) {
					if !got.commaListReads[param] {
						t.Errorf("%s declares %q as an explode=false array, but its REST handler does not parse it with core.QueryList or core.QueryNameList", op.OperationID, param)
					}
					if param == "name" && !got.nameListReads[param] {
						t.Errorf("%s declares %q as an explode=false name array, but its REST handler does not parse it with core.QueryNameList (needed for literal commas, w8/m55)", op.OperationID, param)
					}
				}
			}
			for param := range listQueryOmissions[op.OperationID] {
				if _, ok := params[param]; !ok {
					t.Errorf("stale omission %q", param)
				}
				if got.reads[param] {
					t.Errorf("%q is now consumed; remove stale omission", param)
				}
			}
		})
	}
	for op := range servedListHandlers {
		if !seen[op] {
			t.Errorf("operation disappeared from query contract: %s", op)
		}
	}
	for op := range listQueryOmissions {
		if !seen[op] {
			t.Errorf("listQueryOmissions references unguarded or removed operation %q", op)
		}
	}
}

func isExplodeFalseArray(p *openapi3.Parameter) bool {
	if p == nil || p.Schema == nil || p.Schema.Value == nil {
		return false
	}
	if p.Schema.Value.Type == nil || !p.Schema.Value.Type.Is(openapi3.TypeArray) {
		return false
	}
	return p.Explode != nil && !*p.Explode
}

type queryReadSummary struct {
	reads          map[string]bool
	commaListReads map[string]bool
	nameListReads  map[string]bool
}

type parsedPkg struct {
	functions map[string]*ast.FuncDecl
	consts    map[string]string
	// sliceStringFields records package-level slice-of-struct literals
	// (such as logs.unsupportedLogFilters) mapped to the string literals in
	// their first field, so `for _, f := range slice { v.Has(f.name) }` is
	// recognized statically.
	sliceStringFields map[string][]string
}

func loadParsedPkg(t *testing.T, cache map[string]*parsedPkg, pkg string) *parsedPkg {
	t.Helper()
	if cached, ok := cache[pkg]; ok {
		return cached
	}
	files, err := filepath.Glob(filepath.Join("..", pkg, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := &parsedPkg{
		functions:         map[string]*ast.FuncDecl{},
		consts:            map[string]string{},
		sliceStringFields: map[string][]string{},
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				out.functions[decl.Name.Name] = decl
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						if decl.Tok == token.CONST {
							if s, ok := evalStringExpr(vs.Values[i], out.consts); ok {
								out.consts[name.Name] = s
							}
						}
						if comp, ok := vs.Values[i].(*ast.CompositeLit); ok {
							var literals []string
							for _, elt := range comp.Elts {
								if inner, ok := elt.(*ast.CompositeLit); ok && len(inner.Elts) > 0 {
									if lit, ok := inner.Elts[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
										if v, err := strconv.Unquote(lit.Value); err == nil {
											literals = append(literals, v)
										}
									}
								}
							}
							if len(literals) > 0 {
								out.sliceStringFields[name.Name] = literals
							}
						}
					}
				}
			}
		}
	}
	cache[pkg] = out
	return out
}

func evalStringExpr(e ast.Expr, consts map[string]string) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s, err := strconv.Unquote(v.Value)
			return s, err == nil
		}
	case *ast.Ident:
		s, ok := consts[v.Name]
		return s, ok
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			l, okL := evalStringExpr(v.X, consts)
			r, okR := evalStringExpr(v.Y, consts)
			if okL && okR {
				return l + r, true
			}
		}
	}
	return "", false
}

// extractRouteHandler locates the handler expression registered for pattern
// inside fn (via mux.HandleFunc(pattern, h) or s.handle(mux, pattern, h)),
// resolving a local variable binding (e.g. `list := ...`) when h is an ident.
func extractRouteHandler(fn *ast.FuncDecl, pattern string, pkgConsts map[string]string) ast.Node {
	consts := maps.Clone(pkgConsts)
	locals := map[string]ast.Expr{}
	for _, stmt := range fn.Body.List {
		switch st := stmt.(type) {
		case *ast.DeclStmt:
			if gd, ok := st.Decl.(*ast.GenDecl); ok && gd.Tok == token.CONST {
				for _, spec := range gd.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range vs.Names {
							if i < len(vs.Values) {
								if s, ok := evalStringExpr(vs.Values[i], consts); ok {
									consts[name.Name] = s
								}
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			if len(st.Lhs) == 1 && len(st.Rhs) == 1 {
				if id, ok := st.Lhs[0].(*ast.Ident); ok {
					locals[id.Name] = st.Rhs[0]
					if s, ok := evalStringExpr(st.Rhs[0], consts); ok {
						consts[id.Name] = s
					}
				}
			}
		}
	}
	var matched ast.Node
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if matched != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		var patExpr, handlerExpr ast.Expr
		switch {
		case sel.Sel.Name == "HandleFunc" && len(call.Args) == 2:
			patExpr, handlerExpr = call.Args[0], call.Args[1]
		case sel.Sel.Name == "handle" && len(call.Args) == 3:
			patExpr, handlerExpr = call.Args[1], call.Args[2]
		default:
			return true
		}
		if got, ok := evalStringExpr(patExpr, consts); ok && got == pattern {
			if id, ok := handlerExpr.(*ast.Ident); ok {
				if bound, exists := locals[id.Name]; exists {
					matched = bound
					return false
				}
			}
			matched = handlerExpr
			return false
		}
		return true
	})
	return matched
}

func listHandlerQueryReads(t *testing.T, cache map[string]*parsedPkg, root listHandlerRoot) queryReadSummary {
	t.Helper()
	pkg := loadParsedPkg(t, cache, root.pkg)
	rootFn := pkg.functions[root.fn]
	if rootFn == nil {
		t.Fatalf("missing handler %s.%s", root.pkg, root.fn)
	}

	summary := queryReadSummary{
		reads:          map[string]bool{},
		commaListReads: map[string]bool{},
		nameListReads:  map[string]bool{},
	}
	visited := map[string]bool{}
	addLiteral := func(target map[string]bool, e ast.Expr) {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				target[v] = true
			}
		}
	}

	var inspectNode func(node ast.Node, queryVars, getterVars map[string]bool)
	var visitFunc func(name string)

	visitFunc = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		fn := pkg.functions[name]
		if fn == nil {
			return
		}
		qVars, gVars := collectQueryVars(fn)
		inspectNode(fn.Body, qVars, gVars)
	}

	inspectNode = func(node ast.Node, queryVars, getterVars map[string]bool) {
		if node == nil {
			return
		}
		// Also collect any url.Values bindings introduced inside this node
		// (e.g. `q := r.URL.Query()` in an inline route handler).
		localQVars := maps.Clone(queryVars)
		if localQVars == nil {
			localQVars = map[string]bool{}
		}
		ast.Inspect(node, func(n ast.Node) bool {
			if assign, ok := n.(*ast.AssignStmt); ok && len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
				if id, ok := assign.Lhs[0].(*ast.Ident); ok && isQueryCall(assign.Rhs[0]) {
					localQVars[id.Name] = true
				}
			}
			return true
		})

		ast.Inspect(node, func(n ast.Node) bool {
			if index, ok := n.(*ast.IndexExpr); ok {
				if q, ok := index.X.(*ast.Ident); ok && localQVars[q.Name] {
					addLiteral(summary.reads, index.Index)
				}
			}
			// Range loops that check query presence or read via getter:
			//  - `for _, key := range []string{"updatedBefore", ...} { q.Has(key) }`
			//  - `for _, f := range unsupportedLogFilters { v.Has(f.name) }`
			//  - `for field, dst := range map[string]*time.Time{"createdBefore": ...} { get(field) }`
			if loop, ok := n.(*ast.RangeStmt); ok {
				checksQuery := false
				ast.Inspect(loop.Body, func(inner ast.Node) bool {
					c, ok := inner.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch fun := c.Fun.(type) {
					case *ast.SelectorExpr:
						if fun.Sel.Name == "Has" || fun.Sel.Name == "Get" {
							if q, ok := fun.X.(*ast.Ident); ok && localQVars[q.Name] {
								checksQuery = true
							}
						}
					case *ast.Ident:
						if getterVars[fun.Name] {
							checksQuery = true
						}
					}
					return true
				})
				if checksQuery {
					switch src := loop.X.(type) {
					case *ast.CompositeLit:
						for _, elt := range src.Elts {
							if kv, ok := elt.(*ast.KeyValueExpr); ok {
								addLiteral(summary.reads, kv.Key)
							} else {
								addLiteral(summary.reads, elt)
							}
						}
					case *ast.Ident:
						for _, lit := range pkg.sliceStringFields[src.Name] {
							summary.reads[lit] = true
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
				if getterVars[callee.Name] && len(call.Args) == 1 {
					addLiteral(summary.reads, call.Args[0])
				} else {
					visitFunc(callee.Name)
				}
			case *ast.SelectorExpr:
				owner, _ := callee.X.(*ast.Ident)
				if owner != nil && owner.Name == "core" {
					switch callee.Sel.Name {
					case "PageParams":
						summary.reads["cursor"], summary.reads["limit"] = true, true
					case "QueryLimit":
						summary.reads["limit"] = true
					case "QueryList":
						for _, arg := range call.Args[1:] {
							addLiteral(summary.reads, arg)
							addLiteral(summary.commaListReads, arg)
						}
					case "QueryNameList":
						for _, arg := range call.Args[1:] {
							addLiteral(summary.reads, arg)
							addLiteral(summary.commaListReads, arg)
							addLiteral(summary.nameListReads, arg)
						}
					case "QueryTime", "QueryTimeWindow", "QueryLimit64":
						for _, arg := range call.Args[1:] {
							addLiteral(summary.reads, arg)
						}
					}
				} else if callee.Sel.Name == "Get" || callee.Sel.Name == "Has" {
					if len(call.Args) == 1 {
						addLiteral(summary.reads, call.Args[0])
					}
				} else if owner != nil && owner.Name == "s" {
					visitFunc(callee.Sel.Name)
				}
			}
			return true
		})
	}

	if root.route != "" {
		handlerNode := extractRouteHandler(rootFn, root.route, pkg.consts)
		if handlerNode == nil {
			t.Fatalf("could not locate route %q in %s.%s", root.route, root.pkg, root.fn)
		}
		qVars, gVars := collectQueryVars(rootFn)
		inspectNode(handlerNode, qVars, gVars)
		return summary
	}
	visitFunc(root.fn)
	return summary
}

func collectQueryVars(fn *ast.FuncDecl) (queryVars, getterVars map[string]bool) {
	queryVars, getterVars = map[string]bool{}, map[string]bool{}
	if fn == nil || fn.Type == nil || fn.Type.Params == nil {
		return queryVars, getterVars
	}
	for _, field := range fn.Type.Params.List {
		switch t := field.Type.(type) {
		case *ast.SelectorExpr:
			if x, ok := t.X.(*ast.Ident); ok && x.Name == "url" && t.Sel.Name == "Values" {
				for _, name := range field.Names {
					queryVars[name.Name] = true
				}
			}
		case *ast.FuncType:
			// Match `get func(string) string` (e.g. parseBuildSourceFilter).
			if isStringGetterFunc(t) {
				for _, name := range field.Names {
					getterVars[name.Name] = true
				}
			}
		}
	}
	return queryVars, getterVars
}

func isStringGetterFunc(ft *ast.FuncType) bool {
	if ft.Params == nil || len(ft.Params.List) != 1 || ft.Results == nil || len(ft.Results.List) != 1 {
		return false
	}
	pType, okP := ft.Params.List[0].Type.(*ast.Ident)
	rType, okR := ft.Results.List[0].Type.(*ast.Ident)
	return okP && okR && pType.Name == "string" && rType.Name == "string"
}

func isQueryCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Query" && len(call.Args) == 0
}
