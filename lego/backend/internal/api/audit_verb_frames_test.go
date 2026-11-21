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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// derivedVerbEntries are the core.Base entry points that name the audit verb
// after the stack frame that called them (core.callerVerb): the verb is the
// first frame above them that is not an unexported method. Their ...As twins
// take the verb and are never a problem.
var derivedVerbEntries = map[string]bool{
	"Authorize": true, "AuthorizeTarget": true, "AuthorizeOn": true, "AuthorizeOnOwner": true,
	"AuthorizeOnTarget": true, "AuthorizeWorkspace": true, "AuthorizeWorkspaceOrRecorded": true, "AuthorizeWorkspaceTarget": true,
	"AuthorizeApp": true, "AuthorizeDatabase": true, "AuthorizeKeyValue": true,
}

// verbFrameSite is one place in a function that authorizes, calls a function
// that does, or takes one as a value.
type verbFrameSite struct {
	pkg string
	fn  string // the enclosing declaration: "Service.helper" for a method
	// typ is the receiver type of the enclosing method; callee is the type the
	// selector names when it is the method's own receiver, "" when unknown.
	typ, callee string
	closure     bool // inside a function literal: the frame callerVerb stops at
	value       bool // a method taken as a value: its caller is not this function
	method      bool // the enclosing declaration is a method
	public      bool // ... and it is exported
	named       bool // the enclosing declaration names its operation (core.WithAuditVerb)
	pos         token.Position
}

func (s verbFrameSite) String() string {
	where := s.pkg + "." + s.fn
	if s.closure {
		where += ".funcN"
	}
	if s.value {
		where += " [method value]"
	}
	return fmt.Sprintf("%s (%s:%d)", where, filepath.ToSlash(s.pos.Filename), s.pos.Line)
}

// verbFramePackage is the call structure of one package that the stack walk
// depends on.
type verbFramePackage struct {
	name string
	// entries are the calls to a derived-verb entry point.
	entries []verbFrameSite
	// callers maps an unexported method's name to every place that calls it or
	// takes it as a value: a name-based approximation, narrowed by receiver type
	// where the selector is the enclosing method's own receiver.
	callers map[string][]verbFrameSite
}

func newVerbFramePackage(fset *token.FileSet, name string, files []*ast.File) *verbFramePackage {
	vp := &verbFramePackage{name: name, callers: map[string][]verbFrameSite{}}
	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body != nil {
					vp.scanFunc(fset, d)
				}
			case *ast.GenDecl:
				// A package-level variable's function literal runs with no verb method
				// beneath it.
				site := verbFrameSite{pkg: name, fn: "<package-level>", closure: true}
				vp.walk(fset, d, site, "")
			}
		}
	}
	return vp
}

func (vp *verbFramePackage) scanFunc(fset *token.FileSet, fn *ast.FuncDecl) {
	site := verbFrameSite{pkg: vp.name, fn: fn.Name.Name, method: fn.Recv != nil, public: fn.Name.IsExported()}
	recvName := ""
	if fn.Recv != nil {
		site.typ = receiverTypeName(fn.Recv.List[0].Type)
		site.fn = site.typ + "." + fn.Name.Name
		if names := fn.Recv.List[0].Names; len(names) == 1 {
			recvName = names[0].Name
		}
	}
	site.named = namesItsOperation(fn.Body)
	vp.walk(fset, fn.Body, site, recvName)
}

// namesItsOperation reports whether body calls core.WithAuditVerb: the function
// hands the verb down to everything it calls.
func namesItsOperation(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			switch f := call.Fun.(type) {
			case *ast.SelectorExpr:
				found = found || f.Sel.Name == "WithAuditVerb"
			case *ast.Ident:
				found = found || f.Name == "WithAuditVerb"
			}
		}
		return !found
	})
	return found
}

// walk records the sites under root; recvName is the enclosing method's receiver.
func (vp *verbFramePackage) walk(fset *token.FileSet, root ast.Node, site verbFrameSite, recvName string) {
	called := map[*ast.SelectorExpr]bool{}
	ast.Inspect(root, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				called[sel] = true
			}
		}
		return true
	})
	var visit func(n ast.Node, closure bool)
	visit = func(n ast.Node, closure bool) {
		ast.Inspect(n, func(c ast.Node) bool {
			switch x := c.(type) {
			case *ast.FuncLit:
				visit(x.Body, true)
				return false
			case *ast.SelectorExpr:
				at := site
				at.closure = closure || site.closure
				at.pos = fset.Position(x.Sel.Pos())
				if r := x.Sel.Name[0]; derivedVerbEntries[x.Sel.Name] && called[x] {
					vp.entries = append(vp.entries, at)
				} else if r >= 'a' && r <= 'z' {
					at.value = !called[x]
					if id, ok := x.X.(*ast.Ident); ok && recvName != "" && id.Name == recvName {
						at.callee = site.typ
					}
					vp.callers[x.Sel.Name] = append(vp.callers[x.Sel.Name], at)
				}
			}
			return true
		})
	}
	visit(root, false)
}

// reachesVerb returns the chains by which site reaches a frame that is not an
// exported method, walking up through unexported methods exactly as callerVerb
// does: each is a place where the recorded verb would be a closure's name, a plain
// function's, or a library's.
func (vp *verbFramePackage) reachesVerb(site verbFrameSite, seen map[string]bool) []string {
	switch {
	case site.named:
		return nil
	case site.closure || site.value:
		return []string{site.String()}
	case !site.method:
		return []string{site.String() + " [plain function]"}
	case site.public:
		return nil
	}
	if seen[site.pkg+"."+site.fn] {
		return nil
	}
	seen[site.pkg+"."+site.fn] = true
	name := site.fn[strings.Index(site.fn, ".")+1:]
	var out []string
	for _, caller := range vp.callers[name] {
		if caller.callee != "" && caller.callee != site.typ {
			continue // a call on another type's method of the same name
		}
		for _, chain := range vp.reachesVerb(caller, seen) {
			out = append(out, chain+" -> "+site.fn)
		}
	}
	return out
}

// unnamedVerbPaths lists every chain by which a derived-verb entry point is
// reached through a frame that names no verb.
func (vp *verbFramePackage) unnamedVerbPaths() []string {
	var out []string
	for _, e := range vp.entries {
		if vp.name == "core" {
			// The kernel's own helpers authorize through the unexported twins: a call
			// of a derived entry from an exported method of core would record that
			// method's name (core.AuthorizeLiveApp).
			out = append(out, e.String()+" [core authorizes under its own name]")
			continue
		}
		for _, chain := range vp.reachesVerb(e, map[string]bool{}) {
			out = append(out, chain+" [entry "+fmt.Sprintf("%s:%d", filepath.ToSlash(e.pos.Filename), e.pos.Line)+"]")
		}
	}
	return out
}

// scanVerbFrames parses the non-test files of every package under internal/.
func scanVerbFrames(t *testing.T) []*verbFramePackage {
	t.Helper()
	fset := token.NewFileSet()
	var out []*verbFramePackage
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		pkgs, err := parser.ParseDir(fset, path, func(fi fs.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			return err
		}
		for _, p := range pkgs {
			files := make([]*ast.File, 0, len(p.Files))
			for _, f := range p.Files {
				files = append(files, f)
			}
			out = append(out, newVerbFramePackage(fset, p.Name, files))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestNoAuthorizationReachesAVerbThroughAClosureOrAMethodValue is w21/093's sweep
// over the whole backend. The route, tool and field sweeps in audit_verbs_test.go
// drive what a request can reach through a fixture with no feature dependencies
// wired, so a body that fails validation, a feature that answers 503 and every
// background path stop before they authorize. The audit log still held rows named
// "agentsessions.RunOnce.func1" (the automation scheduler's goroutine) and
// "keyvalue.ClearProjectID.func1" (a retry callback) from exactly those.
//
// This reads the source instead. core.callerVerb names a verb after the first
// frame above an authorize entry point that is not an unexported method, so
// every chain from such an entry up through unexported methods has to end at an
// exported method. A closure (a goroutine, a retry or transaction callback, a
// route handler), a plain function, and an unexported method taken as a value (a
// handler mounted on the mux, a worker's step handed to core.Poll, whose caller is
// the standard library or core) end somewhere else, and the verb recorded is that
// frame's name. A path names its operation instead through an ...As entry point,
// or by calling core.WithAuditVerb before it authorizes. Matching is by method
// name, narrowed by receiver type where the call is on the method's own receiver.
func TestNoAuthorizationReachesAVerbThroughAClosureOrAMethodValue(t *testing.T) {
	var problems []string
	entries, packages := 0, 0
	for _, vp := range scanVerbFrames(t) {
		packages++
		entries += len(vp.entries)
		problems = append(problems, vp.unnamedVerbPaths()...)
	}
	// Guard the guard: a scan that found no authorize call has stopped reading
	// how verbs are authorized.
	if entries < 200 || packages < 40 {
		t.Fatalf("the scan found %d derived-verb authorizations in %d packages — it has gone vacuous", entries, packages)
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Errorf("%s\n\tauthorizes under a verb derived from a closure, a plain function or a method value: "+
			"name it with an AuthorizeAs-family entry point, or call core.WithAuditVerb before it", p)
	}
}

// TestVerbFrameScanFlagsTheShapesThatRecordedClosureVerbs is the self-check of the
// scan above: each case is the shape of a path that recorded a closure's name, or
// of one that does not and must not be reported.
func TestVerbFrameScanFlagsTheShapesThatRecordedClosureVerbs(t *testing.T) {
	for _, tc := range []struct {
		name string
		pkg  string
		src  string
		want int // problems
	}{
		{"a goroutine closure reaching a helper chain (the automation scheduler)", "agentsessions", `
type Service struct{}
type Scheduler struct{ Service *Service }
func (w *Scheduler) RunOnce() { go func() { w.Service.fireRun() }() }
func (s *Service) fireRun() { s.decideRun() }
func (s *Service) decideRun() { s.AuthorizeWorkspace(nil, "", "") }
`, 1},
		{"a retry callback (the placement clears)", "keyvalue", `
type Service struct{}
func (s *Service) ClearProjectID() { retry(func() { s.fetchKeyValue() }) }
func (s *Service) fetchKeyValue() { s.AuthorizeKeyValue(nil, "", "") }
`, 1},
		{"a resolver closure (GraphQL envVars)", "secrets", `
type Service struct{}
func (s *Service) graphQLQuery() { _ = func() { s.listEnvVarsPage() } }
func (s *Service) listEnvVarsPage() { s.listEnvVars() }
func (s *Service) listEnvVars() { s.readAuthorizedEnv() }
func (s *Service) readAuthorizedEnv() { s.scope() }
func (s *Service) scope() { s.AuthorizeApp(nil, "", "") }
`, 1},
		{"a worker's step handed to core.Poll as a method value", "apps", `
type Worker struct{ Svc *Service }
type Service struct{}
func (w *Worker) Run() { core.Poll(nil, "w", 0, w.tick) }
func (w *Worker) tick() { w.process() }
func (w *Worker) process() { w.Svc.AuthorizeOn(nil, "", "") }
`, 1},
		{"a handler mounted as a method value", "logs", `
type Service struct{}
func (s *Service) RegisterREST(mux any) { mux.HandleFunc("GET /x", s.handle) }
func (s *Service) handle() { s.AuthorizeApp(nil, "", "") }
`, 1},
		{"a plain function", "api", `
func serve(s *Service) { s.Authorize(nil, "") }
`, 1},
		{"a package-level function literal", "apps", `
var rows = []row{{apply: func(s *Service) { s.AuthorizeApp(nil, "", "") }}}
`, 1},
		{"a kernel entry point delegating to a sibling", "core", `
type Base struct{}
func (b *Base) AuthorizeLiveApp() { b.AuthorizeApp(nil, "", "") }
`, 1},

		{"an exported verb through unexported helpers", "apps", `
type Service struct{}
func (s *Service) Suspend() { s.setSuspended() }
func (s *Service) setSuspended() { s.writeThroughStore() }
func (s *Service) writeThroughStore() { s.AuthorizeApp(nil, "", "") }
`, 0},
		{"a closure that authorizes under a name it was given", "agentsessions", `
type Service struct{}
func (w *Scheduler) RunOnce() { go func() { w.Service.fireRun() }() }
func (s *Service) fireRun() { s.AuthorizeWorkspaceAs(nil, "agentsessions.FireAgentAutomation", "", "") }
`, 0},
		{"a function that names its operation for the authorizations below it", "apps", `
type Worker struct{ Svc *Service }
func (w *Worker) Run() { core.Poll(nil, "w", 0, w.tick) }
func (w *Worker) tick() { ctx := core.WithAuditVerb(nil, "apps.AutoSyncBlueprint"); w.Svc.runSync(ctx) }
func (s *Service) runSync(ctx any) { s.AuthorizeOn(ctx, "", "") }
`, 0},
		{"a function value of another type with the helper's name", "secrets", `
type Service struct{}
type routes struct{ scope func() }
func (p routes) scoped() { p.scope() }
func (s *Service) Get() { s.scope() }
func (s *Service) scope() { s.AuthorizeApp(nil, "", "") }
`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", "package "+tc.pkg+"\n"+tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := newVerbFramePackage(fset, tc.pkg, []*ast.File{file}).unnamedVerbPaths()
			if len(got) != tc.want {
				t.Fatalf("%d problems %v, want %d", len(got), got, tc.want)
			}
		})
	}
}
