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

package events

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryTargetedVerbIsNamedOrExcused is the guard that keeps this feed
// truthful as bex grows. A verb becomes service-attributable the moment it calls
// core.Base.AuthorizeTarget — from then on its audit rows carry a service target,
// and the ONLY thing deciding whether it reaches the feed is whether eventTypes
// names it. Without this test, a verb added next quarter would be silently
// missing from every service's activity feed, and the feed would quietly stop
// being the 1:1 record its DoD promises.
//
// So: parse the sibling feature packages, find every method that calls
// AuthorizeTarget, and require each to be either named in eventTypes or listed in
// excusedVerbs below WITH a reason. Adding a targeted verb then forces a
// deliberate choice — name it, or write down why it is not an event.
func TestEveryTargetedVerbIsNamedOrExcused(t *testing.T) {
	// Deliberately not events. Each of these is explained in service.go's
	// eventTypes doc comment; the reason lives here so the guard fails loudly if
	// someone deletes the entry rather than the rationale.
	excusedVerbs := map[string]string{
		"apps.Create":     "its first deploy already appears as deploy_started with trigger.firstBuild",
		"apps.Delete":     "the service and its feed are gone; the row stays in the workspace audit log",
		"deploys.Trigger": "the deploys row it opens IS the deploy_started event — mapping the verb too would double-count",
	}

	// Every sibling feature package, ENUMERATED rather than listed: a literal list
	// of today's four would leave the guard blind to the fifth — the day `postgres`
	// or `keyvalue` adopts AuthorizeTarget, its verbs would record targets, produce
	// no event, and nothing would fail. That is precisely the hole this test exists
	// to close, so its own coverage must be a property of the tree.
	found := map[string]bool{}
	for _, pkg := range featurePackages(t) {
		for verb := range targetedVerbsIn(t, filepath.Join("..", pkg), pkg) {
			found[verb] = true
		}
	}
	if len(found) == 0 {
		t.Fatal("parsed no AuthorizeTarget call sites — the guard is not actually guarding anything")
	}

	for verb := range found {
		_, named := eventTypes[verb]
		_, excused := excusedVerbs[verb]
		if !named && !excused {
			t.Errorf("%s records a service target but produces no event.\n"+
				"Every targeted write verb must either map to an event type in eventTypes "+
				"(so it shows up in the service's activity feed) or be listed in excusedVerbs with a reason.", verb)
		}
		if named && excused {
			t.Errorf("%s is both named in eventTypes and excused — pick one", verb)
		}
	}
	// The converse: a vocabulary entry for a verb that no longer targets anything
	// is dead weight that silently never fires.
	for verb := range eventTypes {
		if !found[verb] {
			t.Errorf("eventTypes names %s, but no verb by that name calls AuthorizeTarget — "+
				"the mapping is dead (verb renamed or reverted to plain Authorize?)", verb)
		}
	}
}

// featurePackages lists the sibling feature packages under internal/ — every
// package that could hold a service verb. The kernel (core), the composition root
// (api), the store, and the leaf helpers hold none, and events is this package.
func featurePackages(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("..")
	if err != nil {
		t.Fatalf("read internal/: %v", err)
	}
	notFeatures := map[string]bool{"core": true, "api": true, "store": true, "id": true, "authz": true, "gqlutil": true, "mailer": true, "events": true}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !notFeatures[e.Name()] {
			out = append(out, e.Name())
		}
	}
	return out
}

// targetedVerbsIn returns the "<pkg>.<Method>" names in dir whose body calls
// s.AuthorizeTarget — the same "<package>.<Method>" spelling core.callerVerb
// derives at runtime, which is what audit rows (and therefore eventTypes) key on.
func targetedVerbsIn(t *testing.T, dir, pkg string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	out := map[string]bool{}
	for _, p := range pkgs {
		for _, file := range p.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if ok && sel.Sel.Name == "AuthorizeTarget" {
						out[pkg+"."+fn.Name.Name] = true
					}
					return true
				})
			}
		}
	}
	return out
}
