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
	"context"
	"errors"
	"slices"
	"testing"
)

// rendersTables is render.com/docs/api-keys § Permissions as read on
// 2026-11-18, transcribed independently of the catalog: each area's
// permissions, and per level the action rows marked 🟢. Workflows is left out
// (off bex's roadmap, .pm/DO_NOT_DO.md). A change to Render's page, or to the
// catalog, has to be made here too, deliberately.
var rendersTables = []struct {
	area, permission string
	levels           map[KeyLevel][]string
}{
	{"Services", "Web and private services, static sites, background workers, and cron jobs", map[KeyLevel][]string{
		KeyLevelRead:      {"View services and their configuration"},
		KeyLevelReadWrite: {"View services and their configuration", "Create, update, and delete services", "Manage preview environments and one-off jobs"},
	}},
	{"Services", "Environment variables and secret files", map[KeyLevel][]string{
		KeyLevelRead:      {"View environment variables and secret files"},
		KeyLevelWrite:     {"Set environment variable and secret file values"},
		KeyLevelReadWrite: {"View environment variables and secret files", "Set environment variable and secret file values"},
	}},
	{"Sandboxes", "Sandbox lifecycle and snapshots", map[KeyLevel][]string{
		KeyLevelRead: {"View sandboxes, sandbox groups, and snapshots", "View execution history and credential settings"},
		KeyLevelReadWrite: {"View sandboxes, sandbox groups, and snapshots", "View execution history and credential settings",
			"Create and terminate sandboxes", "Create, update, and delete sandbox groups", "Create and delete snapshots"},
	}},
	{"Sandboxes", "Sandbox execution and file transfer", map[KeyLevel][]string{
		KeyLevelExecute: {"Run commands in sandboxes", "Upload and download files"},
	}},
	{"Sandboxes", "Manage sandbox credentials", map[KeyLevel][]string{
		KeyLevelWrite: {"Create, update, and delete credentials"},
	}},
	{"Deploys", "Deployment information", map[KeyLevel][]string{
		KeyLevelRead: {"View deploys, their status, and history"},
		KeyLevelRun:  {"View deploys, their status, and history"},
	}},
	{"Deploys", "Run deployments", map[KeyLevel][]string{
		KeyLevelRun: {"Trigger and cancel deploys", "Trigger rollbacks"},
	}},
	{"Monitoring", "Read resource logs, metrics, events, and export settings", map[KeyLevel][]string{
		KeyLevelRead: {"View service events", "View service metrics", "View service logs", "View log forwarding and metrics stream settings"},
	}},
	{"Postgres", "Postgres instances", map[KeyLevel][]string{
		KeyLevelRead:      {"View databases and their configuration"},
		KeyLevelReadWrite: {"View databases and their configuration", "Create, update, and delete databases"},
	}},
	{"Postgres", "Postgres credentials and connection details", map[KeyLevel][]string{
		KeyLevelRead:      {"View connection strings and credentials"},
		KeyLevelWrite:     {"Create and delete database credentials"},
		KeyLevelReadWrite: {"View connection strings and credentials", "Create and delete database credentials"},
	}},
	{"Postgres", "Postgres exports and backups", map[KeyLevel][]string{
		KeyLevelRead:      {"View and download exports and backups"},
		KeyLevelReadWrite: {"View and download exports and backups", "Create exports and backups"},
	}},
	{"Key Value", "Key Value instances", map[KeyLevel][]string{
		KeyLevelRead:      {"View instances and their configuration"},
		KeyLevelReadWrite: {"View instances and their configuration", "Create, update, and delete instances"},
	}},
	{"Key Value", "Read Key Value connection details", map[KeyLevel][]string{
		KeyLevelRead: {"View connection URLs and credentials"},
	}},
	{"Object storage", "Objects and blobs", map[KeyLevel][]string{
		KeyLevelRead:      {"View and download objects"},
		KeyLevelReadWrite: {"View and download objects", "Upload and delete objects"},
	}},
	{"Build sources", "Build sources and builds", map[KeyLevel][]string{
		KeyLevelRead:      {"View build sources, their configuration, and their builds"},
		KeyLevelReadWrite: {"View build sources, their configuration, and their builds", "Create, update, and delete build sources", "Trigger builds"},
	}},
	{"Build sources", "Build-source environment variables and secret files", map[KeyLevel][]string{
		KeyLevelRead:      {"View build-source environment variables and secret files"},
		KeyLevelWrite:     {"Set build-source environment variable and secret file values"},
		KeyLevelReadWrite: {"View build-source environment variables and secret files", "Set build-source environment variable and secret file values"},
	}},
	{"Workspaces", "Members, notifications, webhooks, and log and metric streams", map[KeyLevel][]string{
		KeyLevelRead: {"View workspace members", "View notification settings", "View webhooks", "View log and metric stream settings"},
		KeyLevelReadWrite: {"View workspace members", "View notification settings", "View webhooks", "View log and metric stream settings",
			"Update member roles and remove members", "Update notification settings", "Create, update, and delete webhooks", "Update log and metric stream settings"},
	}},
	{"Projects", "Projects and environments", map[KeyLevel][]string{
		KeyLevelRead: {"View projects and environments"},
		KeyLevelReadWrite: {"View projects and environments", "Create, update, and delete projects and environments",
			"Move resources into or out of an environment", "Configure protected environments and cross-environment traffic blocking"},
	}},
	{"Environment groups", "Environment groups", map[KeyLevel][]string{
		KeyLevelRead:      {"View environment groups"},
		KeyLevelReadWrite: {"View environment groups", "Create, update, and delete environment groups"},
	}},
	{"Environment groups", "Environment group variables and secret files", map[KeyLevel][]string{
		KeyLevelRead:      {"View environment group variable and secret file values"},
		KeyLevelWrite:     {"Set environment group variable and secret file values"},
		KeyLevelReadWrite: {"View environment group variable and secret file values", "Set environment group variable and secret file values"},
	}},
	{"Blueprints", "Blueprints", map[KeyLevel][]string{
		KeyLevelRead:      {"View Blueprint configuration"},
		KeyLevelReadWrite: {"View Blueprint configuration", "Update and disconnect Blueprints"},
	}},
	{"Audit logs", "Read workspace audit logs", map[KeyLevel][]string{
		KeyLevelRead: {"Read workspace audit logs"},
	}},
}

// The catalog's Render areas are exactly Render's tables: the same areas,
// permissions, levels and actions, in Render's order, plus only the actions
// marked Bex (Render's docs place those surfaces in the area but its table
// does not list them).
func TestKeyPermissionCatalogMatchesRendersTables(t *testing.T) {
	names := map[KeyAction]KeyActionInfo{}
	for _, a := range KeyActions {
		if _, dup := names[a.ID]; dup {
			t.Errorf("action %s listed twice", a.ID)
		}
		names[a.ID] = a
	}
	var got []string
	for _, area := range KeyPermissionCatalog {
		for _, p := range area.Permissions {
			if area.Bex {
				continue
			}
			got = append(got, area.Name+" / "+p.Name)
		}
	}
	var want []string
	for _, row := range rendersTables {
		want = append(want, row.area+" / "+row.permission)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("catalog permissions\n got %q\nwant %q", got, want)
	}
	i := 0
	for _, area := range KeyPermissionCatalog {
		if area.Bex {
			continue
		}
		for _, p := range area.Permissions {
			row := rendersTables[i]
			i++
			if len(p.Levels) != len(row.levels) {
				t.Errorf("%s offers %d levels, Render %d", p.Name, len(p.Levels), len(row.levels))
			}
			for _, l := range p.Levels {
				var render []string
				for _, a := range l.Actions {
					info, ok := names[a]
					if !ok {
						t.Errorf("%s %s grants unnamed action %s", p.Name, l.Level, a)
					}
					if !info.Bex {
						render = append(render, info.Name)
					}
				}
				if !slices.Equal(render, row.levels[l.Level]) {
					t.Errorf("%s at %s grants %q, Render %q", p.Name, l.Level, render, row.levels[l.Level])
				}
			}
		}
	}
	// Every action is granted somewhere, and permission ids are unique.
	seen := map[string]bool{}
	granted := map[KeyAction]bool{}
	for _, area := range KeyPermissionCatalog {
		for _, p := range area.Permissions {
			if seen[p.ID] {
				t.Errorf("permission id %s used twice", p.ID)
			}
			seen[p.ID] = true
			for _, l := range p.Levels {
				if len(l.Relations) == 0 {
					t.Errorf("%s %s names no relation its creator must hold", p.ID, l.Level)
				}
				for _, a := range l.Actions {
					granted[a] = true
				}
			}
		}
	}
	for _, a := range KeyActions {
		if !granted[a.ID] {
			t.Errorf("action %s is in no level", a.ID)
		}
	}
}

// Two shapes the picker relies on: a permission's last level is its broadest
// (it grants every action any other level does), and every relation a level
// names maps to the member role that holds it, so the required role a
// disabled level shows is never a silent "viewer".
func TestKeyPermissionCatalogShapes(t *testing.T) {
	for _, area := range KeyPermissionCatalog {
		for _, p := range area.Permissions {
			broadest := p.Levels[len(p.Levels)-1]
			for _, l := range p.Levels {
				for _, a := range l.Actions {
					if !slices.Contains(broadest.Actions, a) {
						t.Errorf("%s: %s grants %s, which its last level %s does not", p.ID, l.Level, a, broadest.Level)
					}
				}
				for _, rel := range l.Relations {
					if _, ok := relationRole[rel]; !ok {
						t.Errorf("%s %s names relation %s, which maps to no member role", p.ID, l.Level, rel)
					}
				}
			}
		}
	}
}

func TestKeyGrantAllows(t *testing.T) {
	all := AllKeyGrant()
	none := KeyGrantFromEntries([]string{})
	envWrite := KeyGrantFromEntries([]string{"services_env:write"})
	deploys := KeyGrantFromEntries([]string{"services:read", "deploys_run:run"})
	cases := []struct {
		name    string
		g       KeyGrant
		actions []KeyAction
		want    bool
	}{
		{"All allows anything", all, []KeyAction{KeyWorkspaceMembersManage, KeyNoGrant}, true},
		{"Clear all allows only the identity reads", none, []KeyAction{KeyAnyGrant}, true},
		{"Clear all refuses a read", none, []KeyAction{KeyServicesView}, false},
		{"no scoped grant reaches a no-key operation", deploys, []KeyAction{KeyNoGrant}, false},
		{"Write sets without reading", envWrite, []KeyAction{KeyServicesEnvSet}, true},
		{"Write does not read", envWrite, []KeyAction{KeyServicesEnv}, false},
		{"every action is needed", deploys, []KeyAction{KeyServicesView, KeyDeploysRun}, true},
		{"one missing action refuses", deploys, []KeyAction{KeyServicesView, KeyServicesManage}, false},
		{"Run on deploys_run does not view deploys", deploys, []KeyAction{KeyDeploysView}, false},
		{"an unknown action is never granted", deploys, []KeyAction{"services.everything"}, false},
	}
	for _, c := range cases {
		if got := c.g.Allows(c.actions...); got != c.want {
			t.Errorf("%s: Allows(%v) = %v, want %v", c.name, c.actions, got, c.want)
		}
	}
}

func TestValidateKeyGrant(t *testing.T) {
	g, err := ValidateKeyGrant(KeyGrant{Grants: []KeyGrantEntry{
		{Permission: "audit_logs", Level: KeyLevelRead},
		{Permission: "services", Level: KeyLevelReadWrite},
	}})
	if err != nil || g.Grants[0].Permission != "services" || g.Grants[1].Permission != "audit_logs" {
		t.Fatalf("valid grant = %+v, %v; want catalog order", g, err)
	}
	if g, err := ValidateKeyGrant(KeyGrant{All: true}); err != nil || !g.All {
		t.Fatalf("All = %+v, %v", g, err)
	}
	for _, bad := range []KeyGrant{
		{All: true, Grants: []KeyGrantEntry{{Permission: "services", Level: KeyLevelRead}}},
		{Grants: []KeyGrantEntry{{Permission: "workflows", Level: KeyLevelRead}}},
		{Grants: []KeyGrantEntry{{Permission: "services", Level: KeyLevelRun}}},
		{Grants: []KeyGrantEntry{{Permission: "services", Level: ""}}},
		{Grants: []KeyGrantEntry{{Permission: "deploys", Level: KeyLevelRead}, {Permission: "deploys", Level: KeyLevelRun}}},
	} {
		if _, err := ValidateKeyGrant(bad); !errors.Is(err, ErrBadRequest) {
			t.Errorf("%+v = %v, want 400", bad, err)
		}
	}
}

// A stored grant reads back as written; NULL is All, an empty array Clear
// all, and an entry the catalog no longer offers grants nothing (I7).
func TestKeyGrantStorageRoundTrip(t *testing.T) {
	g, _ := ValidateKeyGrant(KeyGrant{Grants: []KeyGrantEntry{{Permission: "postgres_credentials", Level: KeyLevelWrite}, {Permission: "projects", Level: KeyLevelRead}}})
	back := KeyGrantFromEntries(g.Entries())
	if !slices.Equal(back.Entries(), g.Entries()) || back.All {
		t.Fatalf("round trip %v -> %v", g.Entries(), back.Entries())
	}
	if AllKeyGrant().Entries() != nil {
		t.Fatal("All is not stored as NULL")
	}
	if g := KeyGrantFromEntries(nil); g.All {
		t.Fatal("a scoped grant read from no entries became All")
	}
	if cleared := KeyGrantFromEntries([]string{}); cleared.All || cleared.Entries() == nil {
		t.Fatal("an empty array is not Clear all")
	}
	if retired := KeyGrantFromEntries([]string{"workflows:read", "services:sudo"}); retired.All || len(retired.Grants) != 0 {
		t.Fatalf("retired entries = %+v, want nothing granted", retired)
	}
}

type keyGrantsFunc func(context.Context, string) (KeyGrant, error)

func (f keyGrantsFunc) APIKeyGrant(ctx context.Context, clientID string) (KeyGrant, error) {
	return f(ctx, clientID)
}

// I7: a grant that cannot be read refuses (ErrAuthzUnavailable), never falls
// back to All; a caller that is not a key, and the store-off mode, are All.
func TestKeyGrantFailsClosed(t *testing.T) {
	key := WithIdentity(context.Background(), Identity{Subject: "key-1", ClientID: "key-1", Method: MethodOAuth2})
	person := WithIdentity(context.Background(), Identity{Subject: "person", Method: "session"})
	b := &Base{KeyGrants: keyGrantsFunc(func(context.Context, string) (KeyGrant, error) {
		return KeyGrant{}, errors.New("postgres down")
	})}
	if _, err := b.KeyGrant(key); !errors.Is(err, ErrAuthzUnavailable) {
		t.Fatalf("unreadable grant = %v, want ErrAuthzUnavailable", err)
	}
	if ok, err := b.KeyGrantAllows(key, KeyServicesView); ok || err == nil {
		t.Fatalf("unreadable grant allowed a projection: %v %v", ok, err)
	}
	if g, err := b.KeyGrant(person); err != nil || !g.All {
		t.Fatalf("a person's grant = %+v, %v; want All, unread", g, err)
	}
	if g, err := (&Base{}).KeyGrant(key); err != nil || !g.All {
		t.Fatalf("store-off key grant = %+v, %v; want All", g, err)
	}
	var asked string
	b.KeyGrants = keyGrantsFunc(func(_ context.Context, clientID string) (KeyGrant, error) {
		asked = clientID
		return KeyGrantFromEntries([]string{"services:read"}), nil
	})
	if ok, err := b.KeyGrantAllows(key, KeyServicesManage); ok || err != nil || asked != "key-1" {
		t.Fatalf("Services Read key allowed a manage projection: %v %v (asked %q)", ok, err, asked)
	}
}

// relationsOf is a Checker giving each subject a fixed set of relations on
// workspace:tea-a.
type relationsOf map[string][]string

func (r relationsOf) Check(_ context.Context, subject, relation, object string) (bool, error) {
	return object == WorkspaceObject("tea-a") && slices.Contains(r[subject], relation), nil
}

func TestRequireGrantableKeyGrant(t *testing.T) {
	developer := []string{RelCanView, RelCanViewLogs, RelCanOperate, RelCanCreate, RelCanViewSensitive, RelCanManageKeys}
	b := &Base{Authz: relationsOf{"user:dev": developer, "user:admin": append(slices.Clone(developer), RelCanManage)}}
	ctx := context.Background()
	webhooks := KeyGrant{Grants: []KeyGrantEntry{{Permission: "workspaces", Level: KeyLevelReadWrite}}}
	err := b.RequireGrantableKeyGrant(ctx, "dev", "tea-a", webhooks)
	var coded *CodedError
	if !errors.As(err, &coded) || coded.Code != KeyPermissionAboveRoleCode || coded.Params["requiredRole"] != "admin" {
		t.Fatalf("Developer granting Workspaces Read & write = %v", err)
	}
	if err := b.RequireGrantableKeyGrant(ctx, "admin", "tea-a", webhooks); err != nil {
		t.Fatalf("Admin granting Workspaces Read & write: %v", err)
	}
	// Projects Read & write needs a Developer; its protected-environment
	// action stays the creator ceiling's to refuse.
	projects := KeyGrant{Grants: []KeyGrantEntry{{Permission: "projects", Level: KeyLevelReadWrite}}}
	if err := b.RequireGrantableKeyGrant(ctx, "dev", "tea-a", projects); err != nil {
		t.Fatalf("Developer granting Projects Read & write: %v", err)
	}
	if err := b.RequireGrantableKeyGrant(ctx, "dev", "tea-b", KeyGrant{Grants: []KeyGrantEntry{{Permission: "services", Level: KeyLevelRead}}}); err == nil {
		t.Fatal("a grant in a workspace the creator is not in passed")
	}
	if err := b.RequireGrantableKeyGrant(ctx, "nobody", "tea-a", AllKeyGrant()); err != nil {
		t.Fatalf("All is whatever the role allows: %v", err)
	}
}
