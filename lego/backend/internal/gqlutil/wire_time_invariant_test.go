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

package gqlutil_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNoTimeReachesAStringField is the schema-wide half of w7/052.
//
// TestTimeField (gqlutil_test.go) pins what TimeField does. It cannot fail for
// a field that never calls TimeField — which is the entire failure mode: five
// fields on Project/Environment/Blueprint handed a raw time.Time to StrField
// for months, graphql-go coerced it with fmt.Sprintf("%v", value), and one
// instant read two ways depending on which adapter asked. The sweep that found
// those five would not have caught a sixth; w7/052 said so itself.
//
// So this walks every GraphQL field registration in the backend, resolves the
// declared Go type of the view member each resolver returns, and fails when a
// time.Time reaches a string-typed field. It is a format invariant over the
// whole surface, not a golden snapshot: it asserts one property and names the
// offending file:line when it breaks.
func TestNoTimeReachesAStringField(t *testing.T) {
	root := backendInternal(t)
	pkgs := parseInternalPackages(t, root)

	// Views are usually declared beside their resolver, but plenty are
	// qualified (`core.EnvVar`, `store.AccountWorkspaceDisposition`). Indexing
	// every package by its directory name resolves those exactly, rather than
	// guessing by bare struct name across the tree.
	byPkgName := map[string]structIndex{}
	for _, pkg := range pkgs {
		byPkgName[pkg.name()] = pkg.structFields()
	}

	var violations []string
	stats := coverage{}

	for _, pkg := range pkgs {
		structs := pkg.structFields()
		for _, file := range pkg.files {
			ast.Inspect(file.syntax, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				kind, resolver := classifyFieldCall(call)
				if kind == notAField {
					return true
				}
				stats.total++
				if kind != stringField {
					stats.nonString++
					return true
				}
				recv, field, ok := trivialReturn(resolver)
				if !ok {
					// A resolver whose body is not `return v.Field` cannot be
					// resolved by inspection. Counted, and asserted against a
					// declared budget below, so a new one is visible rather
					// than silently uncovered.
					stats.opaque++
					stats.opaqueAt = append(stats.opaqueAt, pkg.pos(file, call))
					return true
				}
				declared, found := resolveMember(structs, byPkgName, recv, field)
				if !found {
					stats.unresolved++
					stats.unresolvedAt = append(stats.unresolvedAt, pkg.pos(file, call))
					return true
				}
				stats.resolved++
				if declared == "time.Time" || declared == "*time.Time" {
					violations = append(violations, pkg.pos(file, call)+
						": "+recv+"."+field+" is "+declared+
						" behind a string GraphQL field — use gqlutil.TimeField")
				}
				return true
			})
		}
	}

	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("%s", v)
	}
	if len(violations) > 0 {
		t.Logf("graphql-go coerces a string field with fmt.Sprintf(\"%%v\", value), " +
			"so a time.Time renders as Go's time.String() layout and no strict " +
			"parser accepts it (w7/052). TimeField emits the same RFC3339 instant " +
			"REST and MCP do.")
	}

	// The sweep is only worth what it reaches. If these budgets drift upward,
	// coverage is eroding and the invariant is quietly becoming decorative —
	// which is the failure this test exists to prevent one level down.
	if stats.resolved < 480 {
		t.Errorf("resolved only %d string fields; the walk is not reaching the "+
			"schema it claims to cover (did the registration idiom change?)", stats.resolved)
	}
	assertBudget(t, "opaque resolvers", stats.opaque, maxOpaqueResolvers, stats.opaqueAt)
	assertBudget(t, "unresolvable view members", stats.unresolved, maxUnresolvedMembers, stats.unresolvedAt)

	t.Logf("checked %d gqlutil field registrations: %d string-typed (%d resolved, %d opaque, %d unresolved), %d non-string",
		stats.total, stats.total-stats.nonString, stats.resolved, stats.opaque, stats.unresolved, stats.nonString)
}

// Budgets, deliberately exact rather than "a few". A new opaque resolver is not
// a failure in itself — it is a field this test stopped covering, which is
// worth one line of acknowledgement in a diff.
const (
	maxOpaqueResolvers   = 45
	maxUnresolvedMembers = 4
)

func assertBudget(t *testing.T, what string, got, max int, where []string) {
	t.Helper()
	if got <= max {
		return
	}
	sort.Strings(where)
	t.Errorf("%d %s exceeds the budget of %d — this test no longer covers them. "+
		"Either make the resolver a plain `return v.Field`, or raise the budget "+
		"deliberately.\n  %s", got, what, max, strings.Join(where, "\n  "))
}

type coverage struct {
	total        int
	nonString    int
	resolved     int
	opaque       int
	unresolved   int
	opaqueAt     []string
	unresolvedAt []string
}

type fieldKind int

const (
	notAField fieldKind = iota
	stringField
	otherField
)

// classifyFieldCall recognises the gqlutil field constructors and reports
// whether the GraphQL output type is a string (the coercion path that can
// swallow a time.Time), along with the resolver literal.
func classifyFieldCall(call *ast.CallExpr) (fieldKind, *ast.FuncLit) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return notAField, nil
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "gqlutil" {
		return notAField, nil
	}

	switch sel.Sel.Name {
	case "StrField", "ReqStrField", "OptionalStrField":
		if len(call.Args) != 1 {
			return notAField, nil
		}
		lit, _ := call.Args[0].(*ast.FuncLit)
		return stringField, lit
	case "TimeField":
		// Already correct by construction — the helper this test exists to
		// steer people toward.
		if len(call.Args) != 1 {
			return notAField, nil
		}
		lit, _ := call.Args[0].(*ast.FuncLit)
		return otherField, lit
	case "IntField", "BoolField", "FloatField", "StrsField":
		if len(call.Args) != 1 {
			return notAField, nil
		}
		lit, _ := call.Args[0].(*ast.FuncLit)
		return otherField, lit
	case "Typed", "Field":
		// Typed(out, resolver) — the escape hatch, and the one that most needs
		// checking, since router/ registers its timestamps through it.
		if sel.Sel.Name == "Typed" && len(call.Args) == 2 {
			lit, _ := call.Args[1].(*ast.FuncLit)
			if mentionsStringOutput(call.Args[0]) {
				return stringField, lit
			}
			return otherField, lit
		}
		return notAField, nil
	}
	return notAField, nil
}

// mentionsStringOutput reports whether a graphql.Output expression bottoms out
// in graphql.String, through any nesting of NewNonNull/NewList.
func mentionsStringOutput(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "graphql" && sel.Sel.Name == "String" {
			found = true
		}
		return true
	})
	return found
}

// trivialReturn matches `func(v T) any { return v.Field }` and reports the
// receiver type name and the member. Anything else is opaque to inspection.
func trivialReturn(lit *ast.FuncLit) (recv, field string, ok bool) {
	if lit == nil || lit.Type.Params == nil || len(lit.Type.Params.List) != 1 {
		return "", "", false
	}
	param := lit.Type.Params.List[0]
	if len(param.Names) != 1 {
		return "", "", false
	}
	paramName := param.Names[0].Name
	recv = strings.TrimPrefix(types.ExprString(param.Type), "*")

	if lit.Body == nil || len(lit.Body.List) != 1 {
		return "", "", false
	}
	ret, ok := lit.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return "", "", false
	}
	selector, ok := ret.Results[0].(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	base, ok := selector.X.(*ast.Ident)
	if !ok || base.Name != paramName {
		return "", "", false
	}
	return recv, selector.Sel.Name, true
}

// resolveMember looks a `v.Field` up in the resolver's own package, falling
// back to the package named by an explicit qualifier (`core.EnvVar` → the
// package whose directory is `core`).
func resolveMember(local structIndex, byPkgName map[string]structIndex, recv, field string) (string, bool) {
	qualifier, name, qualified := strings.Cut(recv, ".")
	if !qualified {
		return local.lookup(recv, field)
	}
	remote, ok := byPkgName[qualifier]
	if !ok {
		return "", false
	}
	return remote.lookup(name, field)
}

type structIndex map[string]map[string]string

func (s structIndex) lookup(structName, field string) (string, bool) {
	fields, ok := s[structName]
	if !ok {
		return "", false
	}
	declared, ok := fields[field]
	return declared, ok
}

type parsedFile struct {
	path   string
	syntax *ast.File
}

type parsedPackage struct {
	fset  *token.FileSet
	files []parsedFile
	root  string
}

// name is the package's directory name, which is how a qualified view type
// (`core.EnvVar`) names it at the call site.
func (p *parsedPackage) name() string {
	if len(p.files) == 0 {
		return ""
	}
	return filepath.Base(filepath.Dir(p.files[0].path))
}

func (p *parsedPackage) pos(f parsedFile, n ast.Node) string {
	position := p.fset.Position(n.Pos())
	rel, err := filepath.Rel(p.root, f.path)
	if err != nil {
		rel = f.path
	}
	return filepath.ToSlash(rel) + ":" + itoa(position.Line)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// structFields indexes every struct declared in the package, so a resolver's
// `v.Field` can be resolved to its declared Go type without a full type-check.
// Views and their resolvers always live in the same package, which is what
// makes this tractable.
func (p *parsedPackage) structFields() structIndex {
	index := structIndex{}
	for _, file := range p.files {
		ast.Inspect(file.syntax, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := spec.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				return true
			}
			fields, ok := index[spec.Name.Name]
			if !ok {
				fields = map[string]string{}
				index[spec.Name.Name] = fields
			}
			for _, f := range st.Fields.List {
				declared := types.ExprString(f.Type)
				for _, name := range f.Names {
					fields[name.Name] = declared
				}
			}
			return true
		})
	}
	return index
}

// parseInternalPackages parses every non-test Go file under internal/, grouped
// by directory (a package).
func parseInternalPackages(t *testing.T, root string) []*parsedPackage {
	t.Helper()
	byDir := map[string]*parsedPackage{}
	fset := token.NewFileSet()

	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		syntax, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		dir := filepath.Dir(path)
		pkg, ok := byDir[dir]
		if !ok {
			pkg = &parsedPackage{fset: fset, root: root}
			byDir[dir] = pkg
		}
		pkg.files = append(pkg.files, parsedFile{path: path, syntax: syntax})
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/: %v", err)
	}
	if len(byDir) == 0 {
		t.Fatal("parsed no packages under internal/ — the walk is broken, not the code")
	}

	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	pkgs := make([]*parsedPackage, 0, len(dirs))
	for _, dir := range dirs {
		pkgs = append(pkgs, byDir[dir])
	}
	return pkgs
}

// backendInternal finds lego/backend by walking up from the test's directory to
// the go.mod that declares it.
func backendInternal(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "internal")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate lego/backend from the test's working directory")
		}
		dir = parent
	}
}
