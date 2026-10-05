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
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
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
}

var byIDResolutions = map[string]byIDResolution{
	"services":              {"apps", "AuthorizeApp", "cluster-wide CR lookup, authorized on the App's tenant label"},
	"cron-jobs":             {"apps", "AuthorizeApp", "a cron job is a service"},
	"disks":                 {"apps", "authorizeDisk", "disk row → its App → AuthorizeApp"},
	"blueprints":            {"apps", "blueprintScope", "BlueprintWorkspace routing read (w4/m169)"},
	"postgres":              {"postgres", "AuthorizeDatabase", "cluster-wide CR lookup, authorized on the Database's tenant label"},
	"key-value":             {"keyvalue", "AuthorizeKeyValue", "cluster-wide CR lookup, authorized on the KeyValue's tenant label"},
	"env-groups":            {"envgroups", "ScopeByID", "meta/locator workspace routing read, then fetchGroup"},
	"environments":          {"environments", "ScopeByID", "GetEnvironment routing read, then requireEnvironment"},
	"projects":              {"projects", "authorizedProject", "GetProject, authorized on the project's tenant"},
	"webhooks":              {"webhooks", "ScopeByID", "WebhookEndpointWorkspace routing read"},
	"registrycredentials":   {"registrycreds", "ScopeByID", "GetRegistryCredentialByID routing read"},
	"sandboxes":             {"sandbox", "ScopeByID", "member-workspace probe of OpenSandbox"},
	"agent-sessions":        {"agentsessions", "sessionObject", "OpenFGA check on the session object itself"},
	"git":                   {"github", "ScopeByID", "claim selection / installation routing reads"},
	"events":                {"events", "ScopeByID", "ServiceEventWorkspaces routing read"},
	"notifications":         {"notifications", "ScopeByID", "PushNotificationWorkspaces routing read (caller's own rows)"},
	"api-keys":              {"apikeys", "revokeScope", "TenantForKey binding (w4/194)"},
	"notification-settings": {"apps", "AuthorizeApp", "per-service overrides are service-scoped"},

	"workspaces":                         {"", "", "the path id is the workspace"},
	"owners":                             {"", "", "the path id is the workspace"},
	"ssh-keys":                           {"", "", "user-scoped: a caller's own public keys"},
	"notification-device-subscriptions":  {"", "", "the caller's own device registration"},
	"notification-webpush-subscriptions": {"", "", "the caller's own browser registration"},
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
		if !packageMentions(t, filepath.Join("..", r.pkg), r.mechanism) {
			t.Errorf("family %q is classified as resolving via %s in internal/%s, which no longer references it (%s)",
				family, r.mechanism, r.pkg, r.why)
		}
	}
}

// packageMentions reports whether any non-test Go file in dir references ident.
func packageMentions(t *testing.T, dir, ident string) bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), ident) {
			return true
		}
	}
	return false
}
