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
// core.Base.AuthorizeTarget, or (w6/m17) core.Base.AuthorizeApp/
// AuthorizeDatabase/AuthorizeKeyValue — the single seam that replaced the old
// Authorize+GetApp pair and, like AuthorizeTarget, always records a target — from
// then on its audit rows carry a service target, and the ONLY thing deciding
// whether it reaches the feed is whether eventTypes names it. Without this test,
// a verb added next quarter would be silently missing from every service's
// activity feed, and the feed would quietly stop being the 1:1 record its DoD
// promises.
//
// So: parse the sibling feature packages, find every method that calls
// AuthorizeTarget/AuthorizeApp/AuthorizeDatabase/AuthorizeKeyValue, and require
// each to be either named in eventTypes or listed in excusedVerbs below WITH a
// reason. Adding a targeted verb then forces a deliberate choice — name it, or
// write down why it is not an event.
func TestEveryTargetedVerbIsNamedOrExcused(t *testing.T) {
	// Deliberately not events. Each of these is explained in service.go's
	// eventTypes doc comment; the reason lives here so the guard fails loudly if
	// someone deletes the entry rather than the rationale.
	excusedVerbs := map[string]string{
		"apps.Create":        "its first deploy already appears as deploy_started with trigger.firstBuild",
		"apps.Delete":        "the service and its feed are gone; the row stays in the workspace audit log",
		"apps.Deploy":        "stack apply opens a deploy row for each changed service; mapping the wrapper would double-count",
		"apps.DeployStack":   "stack apply opens a deploy row for each changed service; mapping the wrapper would double-count",
		"apps.SyncBlueprint": "Blueprint sync delegates to stack apply, whose changed-service deploy rows already produce the events",
		"deploys.Trigger":    "the deploys row it opens IS the deploy_started event — mapping the verb too would double-count",
		// w6/m20: new verbs from scratch (not a w6/m17 seam-collapse side
		// effect — see below), deliberately deferred the same way their
		// SetProjectID siblings were: postgres/keyvalue have no events feed
		// integration at all yet.
		"postgres.SetEnvironmentID": "mirrors postgres.SetProjectID; postgres has no events feed integration yet",
		"postgres.ExecuteQuery":     "managed Postgres has no events feed; confirmed writes remain visible in the workspace audit log without recording SQL text",
		"postgres.UpdatePostgres":   "REST's general PATCH handler (rename-rejection/disk/HA/ip-allow-list/version, supersedes dedicated setters for that surface); postgres has no events feed integration yet",
		"postgres.SetVersion":       "managed Postgres has no events feed integration yet; the workspace audit log still records the targeted write",
		"keyvalue.SetEnvironmentID": "mirrors keyvalue.SetProjectID; keyvalue has no events feed integration yet",
		// w6/m19: these environment verbs call AuthorizeApp per MEMBER App as a
		// fan-out side effect of an environment-level action (syncing
		// core.LabelNetworkIsolation for NetworkPolicy scoping, SetACL's
		// ipAllowList propagation) — the target recorded is incidental
		// plumbing, not the verb's own resource the way a single-App verb's
		// target is. The environment-level action itself has no events feed
		// of its own to join (environments is a grouping feature, like
		// projects, neither of which has one yet).
		"environments.Delete":      "fans AuthorizeApp out over member Apps to clear core.LabelNetworkIsolation; not itself a per-App verb",
		"environments.SetServices": "fans AuthorizeApp out over member Apps to sync core.LabelNetworkIsolation; not itself a per-App verb",
		"environments.SetACL":      "fans AuthorizeApp out over member Apps to sync core.LabelNetworkIsolation; not itself a per-App verb",
	}
	// w6/m17 moved every resource-scoped write verb off a separate Authorize
	// call onto the single AuthorizeApp/AuthorizeDatabase/AuthorizeKeyValue seam
	// (fixing the caller-default/resource-workspace intersection bug, w6/013) —
	// which, for verbs that previously called plain Authorize (no target) or
	// routed through a package with no events feed at all (postgres, keyvalue),
	// incidentally started recording a target too. That is a mechanical side
	// effect of the collapse, not a deliberate choice to add any of these to a
	// feed; naming real event types for them is a separate decision, left for
	// later milestones (postgres/keyvalue have no events feed integration yet
	// regardless).
	for _, verb := range []string{
		"apps.SetCronJob", "apps.SetHealthCheckPath",
		"deploys.Cancel", "deploys.Rollback",
		"secrets.SetSecretFile", "secrets.SeedSecretFiles", "secrets.DeleteSecretFile",
		"postgres.Suspend", "postgres.Resume", "postgres.Restart", "postgres.Failover",
		"postgres.SetPlan", "postgres.DeletePostgres", "postgres.SetIPAllowList",
		"postgres.CreateUser", "postgres.DeleteUser", "postgres.Recover",
		"postgres.CreateExport", "postgres.SetParameterOverrides", "postgres.SetProjectID",
		"keyvalue.Suspend", "keyvalue.Resume", "keyvalue.SetPlan",
		"keyvalue.DeleteKeyValue", "keyvalue.SetIPAllowList", "keyvalue.SetProjectID",
	} {
		excusedVerbs[verb] = "w6/m17 seam collapse newly targets it; not yet given an event type"
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

// targetingCalls are the core.Base entry points that record a target on their
// audit event — AuthorizeTarget, and (w6/m17) the AuthorizeApp/AuthorizeDatabase/
// AuthorizeKeyValue seam that replaced the old Authorize+GetApp pair and always
// targets the resource it fetched.
var targetingCalls = map[string]bool{
	"AuthorizeTarget":   true,
	"AuthorizeApp":      true,
	"AuthorizeDatabase": true,
	"AuthorizeKeyValue": true,
}

// targetedVerbsIn returns the "<pkg>.<Method>" names in dir whose body calls
// one of targetingCalls, directly or through a same-package helper method
// (apps.patch/writeThroughStore/setSuspended: w6/m17's Restart/Suspend/Resume/
// SetAutoDeploy authorize+fetch through a shared helper rather than inline) —
// the same "<package>.<Method>" spelling core.callerVerb derives at runtime,
// which is what audit rows (and therefore eventTypes) key on.
//
// AuthorizeTarget is, by convention, only ever called with a write relation, so
// any call counts. AuthorizeApp/AuthorizeDatabase/AuthorizeKeyValue (w6/m17) are
// NOT that convention — they are also the read-verb fetch (Get, ListRoutes, ...)
// — so a call only counts when its relation argument is PROVABLY a read
// relation (targetsReadOnly); anything else, including a helper's `relation`
// parameter this static sweep can't trace back to a literal, is conservatively
// treated as targeted — this guard exists to force a human decision, and
// silently skipping an unprovable call risks missing a real write verb.
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
		methods := map[string]*ast.FuncDecl{}
		for _, file := range p.Files {
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Body != nil {
					methods[fn.Name.Name] = fn
				}
			}
		}
		for name, fn := range methods {
			// Only a VERB starts the walk (exported, (ctx, ...) shape — the same
			// isVerbMethod contract server_test.go's reflection sweeps use):
			// RegisterREST/RegisterMCP/GraphQLMutation call verbs inline as request
			// handlers, so without this filter the walk would reach a targeting call
			// through them too and misreport the ADAPTER as a targeted "verb". An
			// unexported helper (patch, writeThroughStore, setSuspended) is never a
			// verb either — it is only ever a hop the walk passes through.
			if !isVerbFuncDecl(fn) {
				continue
			}
			if targetsResource(fn, methods, map[string]bool{}, map[string]string{}) {
				out[pkg+"."+name] = true
			}
		}
	}
	return out
}

// isVerbFuncDecl reports whether fn has a verb's shape: exported, first
// parameter context.Context — see targetedVerbsIn.
func isVerbFuncDecl(fn *ast.FuncDecl) bool {
	if !fn.Name.IsExported() || fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return false
	}
	sel, ok := fn.Type.Params.List[0].Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == "context" && sel.Sel.Name == "Context"
}

// targetsResource reports whether fn's body reaches a targeting call, either
// directly or by calling another same-package method (followed transitively;
// visited guards recursion). See targetedVerbsIn.
//
// binding maps a PARAMETER NAME in fn's own signature to the relation literal
// (e.g. "RelCanView") the CALLER passed for it — one-hop constant propagation,
// needed because the shared fetch helpers (postgres.fetchDatabase/
// loadAppSecret/runInsight, apps.patch/writeThroughStore) all forward the
// verb's own relation through a `relation`-named parameter rather than
// repeating the literal at each hop. Without this, every verb reached through
// one of these helpers would look equally "unprovable" and default to
// targeted — which is correct for a genuine write, but wrongly flags every
// READ verb that happens to route through the same shared helper (e.g.
// postgres.GetPostgres → fetchDatabase → AuthorizeDatabase(relation, ...)).
func targetsResource(fn *ast.FuncDecl, methods map[string]*ast.FuncDecl, visited map[string]bool, binding map[string]string) bool {
	if visited[fn.Name.Name] {
		return false
	}
	visited[fn.Name.Name] = true
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if found {
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
		switch {
		case targetingCalls[sel.Sel.Name]:
			if sel.Sel.Name == "AuthorizeTarget" || !targetsReadOnly(call, binding) {
				found = true
			}
		case methods[sel.Sel.Name] != nil:
			helper := methods[sel.Sel.Name]
			if targetsResource(helper, methods, visited, relationBinding(helper, call, binding)) {
				found = true
			}
		}
		return true
	})
	return found
}

// relationBinding computes the binding a recursive targetsResource call into
// helper should see: helper's own `relation` parameter (if it has one) mapped
// to whatever literal — direct, or resolved one level further back through
// the CALLER's own binding — call passed for it. See targetsResource.
func relationBinding(helper *ast.FuncDecl, call *ast.CallExpr, binding map[string]string) map[string]string {
	next := map[string]string{}
	if helper.Type.Params == nil {
		return next
	}
	var names []string
	for _, field := range helper.Type.Params.List {
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
	}
	for i, name := range names {
		if name != "relation" || i >= len(call.Args) {
			continue
		}
		switch a := call.Args[i].(type) {
		case *ast.SelectorExpr:
			next["relation"] = a.Sel.Name
		case *ast.Ident:
			if lit, ok := binding[a.Name]; ok {
				next["relation"] = lit
			}
		}
	}
	return next
}

// targetsReadOnly reports whether call's relation argument (the second
// argument to AuthorizeApp/AuthorizeDatabase/AuthorizeKeyValue) is PROVABLY a
// read relation — a literal core.RelCanView/RelCanViewLogs/RelCanViewSensitive
// selector, or a `relation` parameter binding (see relationBinding) that
// resolves to one. Anything else (a write relation, or a variable this sweep
// can't resolve) is not provably read-only — see targetedVerbsIn.
func targetsReadOnly(call *ast.CallExpr, binding map[string]string) bool {
	if len(call.Args) < 2 {
		return false
	}
	name := ""
	switch a := call.Args[1].(type) {
	case *ast.SelectorExpr:
		name = a.Sel.Name
	case *ast.Ident:
		name = binding[a.Name]
	}
	switch name {
	case "RelCanView", "RelCanViewLogs", "RelCanViewSensitive":
		return true
	default:
		return false
	}
}
