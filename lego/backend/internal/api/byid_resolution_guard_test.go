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
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// byIDResolution is how one route family's by-id verbs find the workspace they
// act in (w4/m172). Render's by-id endpoints take no owner, so a resource must
// be found in whichever of the caller's workspaces owns it — never only in the
// caller's default, the defect three families shipped one at a time (Blueprints
// w4/m169, API keys w4/194, webhooks and five more in w4/m172).
//
// pkg/mechanism name the resolver the family's package must still use; an
// empty pkg is a family whose path id is not a workspace-owned resource (the
// workspace itself, or a caller-owned row) and so needs none.
type byIDResolution struct {
	pkg       string
	mechanism string
	why       string
	// within names the family's routing helpers, each of which must itself
	// call mechanism (w5/m115): a call elsewhere in the package — another
	// family's routing — does not count. Empty checks the whole package, for
	// mechanisms only these families use (AuthorizeApp and the like).
	within []string
}

var byIDResolutions = map[string]byIDResolution{
	"services":              {"apps", "AuthorizeApp", "cluster-wide CR lookup, authorized on the App's tenant label", nil},
	"cron-jobs":             {"apps", "AuthorizeApp", "a cron job is a service", nil},
	"disks":                 {"apps", "authorizeDisk", "disk row → its App → AuthorizeApp", nil},
	"blueprints":            {"apps", "ScopeByID", "BlueprintWorkspace routing read (w4/m169, w5/m115)", []string{"blueprintScope"}},
	"postgres":              {"postgres", "AuthorizeDatabase", "cluster-wide CR lookup, authorized on the Database's tenant label", nil},
	"key-value":             {"keyvalue", "AuthorizeKeyValue", "cluster-wide CR lookup, authorized on the KeyValue's tenant label", nil},
	"env-groups":            {"envgroups", "ScopeByID", "meta/locator workspace routing read, then fetchGroup", []string{"authorizeGroup"}},
	"environments":          {"environments", "ScopeByID", "GetEnvironment routing read, then requireEnvironment", []string{"scopeEnvironment"}},
	"projects":              {"projects", "authorizedProject", "GetProject, authorized on the project's tenant", nil},
	"webhooks":              {"webhooks", "ScopeByID", "WebhookEndpointWorkspace routing read", []string{"scopeEndpoint"}},
	"registrycredentials":   {"registrycreds", "ScopeByID", "GetRegistryCredentialByID routing read", []string{"scopeCredential"}},
	"sandboxes":             {"sandbox", "ScopeByVisibleID", "member-workspace probe of OpenSandbox", []string{"scopeSandbox"}},
	"agent-sessions":        {"agentsessions", "sessionObject", "OpenFGA check on the session object itself", nil},
	"git":                   {"github", "ScopeByVisibleID", "claim selection / installation routing reads (visible-only: GitHub ids are enumerable)", []string{"scopeSelection", "scopeInstallation"}},
	"events":                {"events", "ScopeByID", "ServiceEventWorkspaces routing read", []string{"scopeEvent"}},
	"notifications":         {"notifications", "ScopeByID", "PushNotificationWorkspaces routing read (caller's own rows)", []string{"scopeNotification"}},
	"api-keys":              {"apikeys", "ScopeByID", "TenantForKey binding (w4/194, w5/m115)", []string{"revokeScope"}},
	"notification-settings": {"apps", "AuthorizeApp", "per-service overrides are service-scoped", nil},

	"workspaces":                         {"", "", "the path id is the workspace", nil},
	"owners":                             {"", "", "the path id is the workspace", nil},
	"ssh-keys":                           {"", "", "user-scoped: a caller's own public keys", nil},
	"notification-device-subscriptions":  {"notifications", "ScopeByID", "the caller's own device, in the workspace it was registered in (w5/m115)", []string{"scopeDevice"}},
	"notification-webpush-subscriptions": {"", "", "the caller's own browser registration", nil},
}

// TestEveryByIDRouteResolvesItsOwnWorkspace fails when a REST route that takes
// a path id belongs to a family nobody has classified — the moment a new by-id
// family must decide how it finds its owning workspace — or when a classified
// family's package stops using the resolver it was classified with.
func TestEveryByIDRouteResolvesItsOwnWorkspace(t *testing.T) {
	pathParam := regexp.MustCompile(`\{[A-Za-z]+\}`)
	families := map[string]bool{}
	var unclassified []string
	for _, pattern := range serveMuxPatterns(matrixServer(t).restHandler(revokeRegistrar{})) {
		_, path, _ := strings.Cut(pattern, " ")
		if !pathParam.MatchString(path) {
			continue
		}
		segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(segments) < 2 || segments[0] != "v1" {
			unclassified = append(unclassified, pattern)
			continue
		}
		family := segments[1]
		families[family] = true
		if _, ok := byIDResolutions[family]; !ok {
			unclassified = append(unclassified, pattern)
		}
	}
	if len(families) < 15 {
		t.Fatalf("found %d by-id families; the route enumerator looks broken", len(families))
	}
	slices.Sort(unclassified)
	if len(unclassified) > 0 {
		t.Fatalf("by-id routes in an unclassified family — decide how each finds the resource's OWN workspace "+
			"(core.Base.ScopeByID, or a label/object authorization) and add the family to byIDResolutions:\n  %s",
			strings.Join(unclassified, "\n  "))
	}
	for family, r := range byIDResolutions {
		if !families[family] {
			t.Errorf("byIDResolutions[%q] classifies a family with no by-id route; remove it", family)
		}
		if r.pkg == "" {
			continue
		}
		within := r.within
		if len(within) == 0 {
			within = []string{""}
		}
		for _, fn := range within {
			if !packageCalls(t, filepath.Join("..", r.pkg), fn, r.mechanism) {
				where := "internal/" + r.pkg
				if fn != "" {
					where += "." + fn
				}
				t.Errorf("family %q is classified as resolving via %s in %s, which no longer calls it (%s)",
					family, r.mechanism, where, r.why)
			}
		}
	}
}

// packageCalls reports whether a non-test Go file in dir calls a function or
// method named name — inside the function or method named within, or anywhere
// when within is empty. Parsed, not grepped (w5/m115): a comment or a string
// that merely mentions the resolver satisfies nothing.
func packageCalls(t *testing.T, dir, within, name string) bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		var scopes []ast.Node
		if within == "" {
			scopes = []ast.Node{file}
		} else {
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == within && fn.Body != nil {
					scopes = append(scopes, fn.Body)
				}
			}
		}
		for _, scope := range scopes {
			if callsName(scope, name) {
				return true
			}
		}
	}
	return false
}

// callsName reports whether n contains a call to a function or method named name.
func callsName(n ast.Node, name string) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				found = found || fn.Sel.Name == name
			case *ast.Ident:
				found = found || fn.Name == name
			}
		}
		return !found
	})
	return found
}

// TestPackageCallsIgnoresMentions proves the guard above cannot be satisfied
// by documentation: only a real call counts.
func TestPackageCallsIgnoresMentions(t *testing.T) {
	dir := t.TempDir()
	write := func(src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("package x\n\n// scope routes through ScopeByID.\nfunc scope() string { return \"ScopeByID\" }\n")
	if packageCalls(t, dir, "", "ScopeByID") {
		t.Fatal("a comment and a string satisfied the guard")
	}
	write("package x\n\nfunc scope(s interface{ ScopeByID() }) { s.ScopeByID() }\nfunc other() {}\n")
	if !packageCalls(t, dir, "scope", "ScopeByID") {
		t.Fatal("a real call did not satisfy the guard")
	}
	if packageCalls(t, dir, "other", "ScopeByID") {
		t.Fatal("another function's call satisfied the guard for this one")
	}
}

// idKindRouting classifies every id kind for by-id routing (w5/m115): the
// route family whose verbs resolve it, or why no by-id verb takes it. A new
// kind fails TestEveryIDKindIsClassifiedForByIDRouting until someone decides
// how a verb finds its owning workspace.
var idKindRouting = map[string]struct{ family, why string }{
	ids.Workspace.Prefix():                {"workspaces", "the id is the workspace"},
	ids.Service.Prefix():                  {"services", ""},
	ids.Postgres.Prefix():                 {"postgres", ""},
	ids.KeyValue.Prefix():                 {"key-value", ""},
	ids.Domain.Prefix():                   {"services", "addressed under its service"},
	ids.EnvGroup.Prefix():                 {"env-groups", ""},
	ids.Deploy.Prefix():                   {"services", "addressed under its service"},
	ids.Invite.Prefix():                   {"workspaces", "addressed under its workspace"},
	ids.Export.Prefix():                   {"postgres", "addressed under its database"},
	ids.Audit.Prefix():                    {"", "listed per owner, never addressed by id"},
	ids.Owner.Prefix():                    {"", "a user, not a workspace-owned resource"},
	ids.Event.Prefix():                    {"events", ""},
	ids.CronRun.Prefix():                  {"cron-jobs", "addressed under its cron job"},
	ids.Notification.Prefix():             {"", "a member's preferences, addressed by owner or service"},
	ids.Project.Prefix():                  {"projects", ""},
	ids.RegistryCredential.Prefix():       {"registrycredentials", ""},
	ids.Blueprint.Prefix():                {"blueprints", ""},
	ids.Environment.Prefix():              {"environments", ""},
	ids.Webhook.Prefix():                  {"webhooks", ""},
	ids.WebhookDelivery.Prefix():          {"webhooks", "addressed under its endpoint"},
	ids.WebhookReplayLease.Prefix():       {"", "internal git-webhook replay state, no API"},
	ids.Job.Prefix():                      {"services", "addressed under its service"},
	ids.SSHKey.Prefix():                   {"ssh-keys", "the caller's own keys"},
	ids.SSHSession.Prefix():               {"", "an audit record, never addressed by id"},
	ids.BlueprintSync.Prefix():            {"blueprints", "addressed under its Blueprint"},
	ids.BlueprintAutoSyncIntent.Prefix():  {"", "internal sync state, no API"},
	ids.AgentSession.Prefix():             {"agent-sessions", ""},
	ids.Disk.Prefix():                     {"disks", ""},
	ids.WorkspaceCreationAttempt.Prefix(): {"", "internal billing state, no API"},
	ids.CLITelemetryEvent.Prefix():        {"", "ingest-only telemetry, no API"},
	ids.SandboxExecution.Prefix():         {"sandboxes", "a token handshake under its sandbox"},
	ids.GitClaimSelection.Prefix():        {"git", ""},
	ids.Sandbox.Prefix():                  {"sandboxes", ""},
}

// TestEveryIDKindIsClassifiedForByIDRouting closes the gap the route walk
// leaves: a new workspace-owned id kind must name the family that resolves it
// (whose package must call its resolver) or say why it needs none.
func TestEveryIDKindIsClassifiedForByIDRouting(t *testing.T) {
	known := map[string]bool{}
	for _, k := range ids.Kinds() {
		known[k.Prefix()] = true
		c, ok := idKindRouting[k.Prefix()]
		switch {
		case !ok:
			t.Errorf("id kind %s- (%s) is unclassified: name the by-id family that resolves its workspace, or why none does", k.Prefix(), k.Desc())
		case c.family == "" && c.why == "":
			t.Errorf("id kind %s- has neither a family nor a reason", k.Prefix())
		case c.family != "":
			if _, ok := byIDResolutions[c.family]; !ok {
				t.Errorf("id kind %s- names family %q, which byIDResolutions does not classify", k.Prefix(), c.family)
			}
		}
	}
	for prefix := range idKindRouting {
		if !known[prefix] {
			t.Errorf("idKindRouting classifies %s-, which is no id kind; remove it", prefix)
		}
	}
}
