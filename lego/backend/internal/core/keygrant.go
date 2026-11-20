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
	"fmt"
	"log"
	"slices"
	"strings"
)

// A bex API key carries a grant: a chosen subset of Render's per-area
// permissions, never more than its creator's current role (w9/m496,
// docs/ADR024-members.md § An API key carries a chosen set of permissions).
// Render: "API keys limit access to resources within a specific workspace, so
// you can grant each key only the permissions it needs"; "Your member role in
// the selected workspace determines which permissions you can grant an API
// key" (render.com/docs/api-keys, read 2026-11-18).
//
// The catalog below is Render's permission tables: each area lists
// permissions, each permission offers some of Render's levels, and each level
// grants a set of actions. An operation needs actions (the api package's
// keyPermissionOps table); a key may call it only when its grant covers every
// one of them AND its creator may do it (keyCeiling, I6). The grant can only
// refuse.

// KeyLevel is one of Render's permission levels.
type KeyLevel string

const (
	KeyLevelRead      KeyLevel = "read"
	KeyLevelWrite     KeyLevel = "write"
	KeyLevelReadWrite KeyLevel = "read_write"
	KeyLevelRun       KeyLevel = "run"
	KeyLevelExecute   KeyLevel = "execute"
)

// KeyAction is one row of Render's tables: the unit an operation needs.
type KeyAction string

// The actions. Each is one "Action" row of render.com/docs/api-keys, or a
// bex action placed in the Render area Render's docs put that surface in
// (KeyActionInfo.Bex).
const (
	KeyServicesView   KeyAction = "services.view"
	KeyServicesManage KeyAction = "services.manage"
	KeyServicesJobs   KeyAction = "services.jobs"
	KeyServicesEnv    KeyAction = "services.env.view"
	KeyServicesEnvSet KeyAction = "services.env.set"

	KeySandboxesView        KeyAction = "sandboxes.view"
	KeySandboxesHistory     KeyAction = "sandboxes.history"
	KeySandboxesManage      KeyAction = "sandboxes.manage"
	KeySandboxesGroups      KeyAction = "sandboxes.groups"
	KeySandboxesSnapshots   KeyAction = "sandboxes.snapshots"
	KeySandboxesExec        KeyAction = "sandboxes.exec"
	KeySandboxesFiles       KeyAction = "sandboxes.files"
	KeySandboxesCredentials KeyAction = "sandboxes.credentials"

	KeyDeploysView     KeyAction = "deploys.view"
	KeyDeploysRun      KeyAction = "deploys.run"
	KeyDeploysRollback KeyAction = "deploys.rollback"

	KeyMonitoringEvents  KeyAction = "monitoring.events"
	KeyMonitoringMetrics KeyAction = "monitoring.metrics"
	KeyMonitoringLogs    KeyAction = "monitoring.logs"
	KeyMonitoringStreams KeyAction = "monitoring.streams"

	KeyPostgresView              KeyAction = "postgres.view"
	KeyPostgresManage            KeyAction = "postgres.manage"
	KeyPostgresCredentials       KeyAction = "postgres.credentials.view"
	KeyPostgresCredentialsManage KeyAction = "postgres.credentials.manage"
	KeyPostgresExports           KeyAction = "postgres.exports.view"
	KeyPostgresExportsCreate     KeyAction = "postgres.exports.create"

	KeyKeyValueView       KeyAction = "keyvalue.view"
	KeyKeyValueManage     KeyAction = "keyvalue.manage"
	KeyKeyValueConnection KeyAction = "keyvalue.connection.view"

	KeyObjectsView   KeyAction = "objects.view"
	KeyObjectsManage KeyAction = "objects.manage"

	KeyBuildSourcesView   KeyAction = "buildsources.view"
	KeyBuildSourcesManage KeyAction = "buildsources.manage"
	KeyBuildSourcesBuild  KeyAction = "buildsources.build"
	KeyBuildSourcesEnv    KeyAction = "buildsources.env.view"
	KeyBuildSourcesEnvSet KeyAction = "buildsources.env.set"

	KeyWorkspaceMembers             KeyAction = "workspace.members.view"
	KeyWorkspaceNotifications       KeyAction = "workspace.notifications.view"
	KeyWorkspaceWebhooks            KeyAction = "workspace.webhooks.view"
	KeyWorkspaceStreams             KeyAction = "workspace.streams.view"
	KeyWorkspaceRegistry            KeyAction = "workspace.registry.view"
	KeyWorkspaceSettings            KeyAction = "workspace.settings.view"
	KeyWorkspaceMembersManage       KeyAction = "workspace.members.manage"
	KeyWorkspaceNotificationsManage KeyAction = "workspace.notifications.manage"
	KeyWorkspaceWebhooksManage      KeyAction = "workspace.webhooks.manage"
	KeyWorkspaceStreamsManage       KeyAction = "workspace.streams.manage"
	KeyWorkspaceRegistryManage      KeyAction = "workspace.registry.manage"
	KeyWorkspaceSettingsManage      KeyAction = "workspace.settings.manage"

	KeyProjectsView    KeyAction = "projects.view"
	KeyProjectsManage  KeyAction = "projects.manage"
	KeyProjectsMove    KeyAction = "projects.move"
	KeyProjectsProtect KeyAction = "projects.protect"

	KeyEnvGroupsView      KeyAction = "envgroups.view"
	KeyEnvGroupsManage    KeyAction = "envgroups.manage"
	KeyEnvGroupsValues    KeyAction = "envgroups.values.view"
	KeyEnvGroupsValuesSet KeyAction = "envgroups.values.set"

	KeyBlueprintsView   KeyAction = "blueprints.view"
	KeyBlueprintsManage KeyAction = "blueprints.manage"

	KeyAuditLogsView KeyAction = "auditlogs.view"

	KeyAgentsView       KeyAction = "agents.view"
	KeyAgentsManage     KeyAction = "agents.manage"
	KeyAgentsSecretsSet KeyAction = "agents.secrets.set"

	KeyGitView   KeyAction = "git.view"
	KeyGitManage KeyAction = "git.manage"
)

// The two markers an operation table may name instead of actions. They are
// never granted and never in the catalog.
const (
	// KeyAnyGrant: any key may call the operation whatever its grant (it
	// names the caller or its workspace, or describes the platform).
	KeyAnyGrant KeyAction = "*"
	// KeyNoGrant: no permission in the catalog covers the operation, so only
	// a key whose grant is All may call it, and then only as far as its own
	// gates allow (an account's own settings, credential mints, member verbs
	// about the caller: I4, I5).
	KeyNoGrant KeyAction = "-"
)

// KeyActionInfo names an action: Render's own words, or bex's for an action
// Render's table does not list (Bex).
type KeyActionInfo struct {
	ID   KeyAction `json:"id"`
	Name string    `json:"name"`
	Bex  bool      `json:"bex"`
}

// KeyPermissionLevel is one level a permission offers: the actions it grants
// and the workspace relations its creator must hold to grant it.
type KeyPermissionLevel struct {
	Level     KeyLevel
	Actions   []KeyAction
	Relations []string
}

// KeyPermission is one "Permission" row group of Render's table.
type KeyPermission struct {
	ID     string
	Name   string
	Levels []KeyPermissionLevel
}

// KeyArea is one of Render's resource types (a table), or a bex area (Bex)
// for a product Render does not have.
type KeyArea struct {
	ID          string
	Name        string
	Bex         bool
	Permissions []KeyPermission
}

func lvl(level KeyLevel, relations []string, actions ...KeyAction) KeyPermissionLevel {
	return KeyPermissionLevel{Level: level, Actions: actions, Relations: relations}
}

func rels(r ...string) []string { return r }

// KeyActions names every action, in catalog order.
var KeyActions = []KeyActionInfo{
	{KeyServicesView, "View services and their configuration", false},
	{KeyServicesManage, "Create, update, and delete services", false},
	{KeyServicesJobs, "Manage preview environments and one-off jobs", false},
	{KeyServicesEnv, "View environment variables and secret files", false},
	{KeyServicesEnvSet, "Set environment variable and secret file values", false},

	{KeySandboxesView, "View sandboxes, sandbox groups, and snapshots", false},
	{KeySandboxesHistory, "View execution history and credential settings", false},
	{KeySandboxesManage, "Create and terminate sandboxes", false},
	{KeySandboxesGroups, "Create, update, and delete sandbox groups", false},
	{KeySandboxesSnapshots, "Create and delete snapshots", false},
	{KeySandboxesExec, "Run commands in sandboxes", false},
	{KeySandboxesFiles, "Upload and download files", false},
	{KeySandboxesCredentials, "Create, update, and delete credentials", false},

	{KeyDeploysView, "View deploys, their status, and history", false},
	{KeyDeploysRun, "Trigger and cancel deploys", false},
	{KeyDeploysRollback, "Trigger rollbacks", false},

	{KeyMonitoringEvents, "View service events", false},
	{KeyMonitoringMetrics, "View service metrics", false},
	{KeyMonitoringLogs, "View service logs", false},
	{KeyMonitoringStreams, "View log forwarding and metrics stream settings", false},

	{KeyPostgresView, "View databases and their configuration", false},
	{KeyPostgresManage, "Create, update, and delete databases", false},
	{KeyPostgresCredentials, "View connection strings and credentials", false},
	{KeyPostgresCredentialsManage, "Create and delete database credentials", false},
	{KeyPostgresExports, "View and download exports and backups", false},
	{KeyPostgresExportsCreate, "Create exports and backups", false},

	{KeyKeyValueView, "View instances and their configuration", false},
	{KeyKeyValueManage, "Create, update, and delete instances", false},
	{KeyKeyValueConnection, "View connection URLs and credentials", false},

	{KeyObjectsView, "View and download objects", false},
	{KeyObjectsManage, "Upload and delete objects", false},

	{KeyBuildSourcesView, "View build sources, their configuration, and their builds", false},
	{KeyBuildSourcesManage, "Create, update, and delete build sources", false},
	{KeyBuildSourcesBuild, "Trigger builds", false},
	{KeyBuildSourcesEnv, "View build-source environment variables and secret files", false},
	{KeyBuildSourcesEnvSet, "Set build-source environment variable and secret file values", false},

	{KeyWorkspaceMembers, "View workspace members", false},
	{KeyWorkspaceNotifications, "View notification settings", false},
	{KeyWorkspaceWebhooks, "View webhooks", false},
	{KeyWorkspaceStreams, "View log and metric stream settings", false},
	{KeyWorkspaceRegistry, "View registry credentials", true},
	{KeyWorkspaceSettings, "View workspace settings, plan, and usage", true},
	{KeyWorkspaceMembersManage, "Update member roles and remove members", false},
	{KeyWorkspaceNotificationsManage, "Update notification settings", false},
	{KeyWorkspaceWebhooksManage, "Create, update, and delete webhooks", false},
	{KeyWorkspaceStreamsManage, "Update log and metric stream settings", false},
	{KeyWorkspaceRegistryManage, "Create, update, and delete registry credentials", true},
	{KeyWorkspaceSettingsManage, "Update workspace settings, plan, and billing", true},

	{KeyProjectsView, "View projects and environments", false},
	{KeyProjectsManage, "Create, update, and delete projects and environments", false},
	{KeyProjectsMove, "Move resources into or out of an environment", false},
	{KeyProjectsProtect, "Configure protected environments and cross-environment traffic blocking", false},

	{KeyEnvGroupsView, "View environment groups", false},
	{KeyEnvGroupsManage, "Create, update, and delete environment groups", false},
	{KeyEnvGroupsValues, "View environment group variable and secret file values", false},
	{KeyEnvGroupsValuesSet, "Set environment group variable and secret file values", false},

	{KeyBlueprintsView, "View Blueprint configuration", false},
	{KeyBlueprintsManage, "Update and disconnect Blueprints", false},

	{KeyAuditLogsView, "Read workspace audit logs", false},

	{KeyAgentsView, "View agent sessions, automations, skills, and repository environments", true},
	{KeyAgentsManage, "Start, steer, and delete agent sessions; create, update, and delete automations, skills, and repository environments", true},
	{KeyAgentsSecretsSet, "Set and delete agent secret values", true},

	{KeyGitView, "View Git connections and repositories", true},
	{KeyGitManage, "Connect and disconnect Git providers", true},
}

// KeyPermissionCatalog is Render's permission tables (render.com/docs/api-keys,
// re-read 2026-11-18), in Render's order, then bex's own areas. Render's
// Workflows area is absent: Workflows is off bex's roadmap
// (.pm/DO_NOT_DO.md), so it has nothing to grant. A level's Relations are the
// workspace relations its creator must hold to grant it: the relation bex's
// own verbs decide its actions with, the strongest where a level bundles
// several, except that Projects Read & write needs only a Developer's
// can_create (its protected-environment action stays refused to a Developer's
// key by the creator ceiling, I6).
var KeyPermissionCatalog = []KeyArea{
	{ID: "services", Name: "Services", Permissions: []KeyPermission{
		{ID: "services", Name: "Web and private services, static sites, background workers, and cron jobs", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyServicesView),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanCreate), KeyServicesView, KeyServicesManage, KeyServicesJobs),
		}},
		{ID: "services_env", Name: "Environment variables and secret files", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanViewSensitive), KeyServicesEnv),
			lvl(KeyLevelWrite, rels(RelCanCreate), KeyServicesEnvSet),
			lvl(KeyLevelReadWrite, rels(RelCanViewSensitive, RelCanCreate), KeyServicesEnv, KeyServicesEnvSet),
		}},
	}},
	{ID: "sandboxes", Name: "Sandboxes", Permissions: []KeyPermission{
		{ID: "sandboxes", Name: "Sandbox lifecycle and snapshots", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeySandboxesView, KeySandboxesHistory),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanCreate), KeySandboxesView, KeySandboxesHistory, KeySandboxesManage, KeySandboxesGroups, KeySandboxesSnapshots),
		}},
		{ID: "sandbox_execution", Name: "Sandbox execution and file transfer", Levels: []KeyPermissionLevel{
			lvl(KeyLevelExecute, rels(RelCanViewSensitive), KeySandboxesExec, KeySandboxesFiles),
		}},
		{ID: "sandbox_credentials", Name: "Manage sandbox credentials", Levels: []KeyPermissionLevel{
			lvl(KeyLevelWrite, rels(RelCanCreate), KeySandboxesCredentials),
		}},
	}},
	{ID: "deploys", Name: "Deploys", Permissions: []KeyPermission{
		{ID: "deploys", Name: "Deployment information", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyDeploysView),
			lvl(KeyLevelRun, rels(RelCanView), KeyDeploysView),
		}},
		{ID: "deploys_run", Name: "Run deployments", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRun, rels(RelCanOperate), KeyDeploysRun, KeyDeploysRollback),
		}},
	}},
	{ID: "monitoring", Name: "Monitoring", Permissions: []KeyPermission{
		{ID: "monitoring", Name: "Read resource logs, metrics, events, and export settings", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView, RelCanViewLogs), KeyMonitoringEvents, KeyMonitoringMetrics, KeyMonitoringLogs, KeyMonitoringStreams),
		}},
	}},
	{ID: "postgres", Name: "Postgres", Permissions: []KeyPermission{
		{ID: "postgres", Name: "Postgres instances", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyPostgresView),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanCreate), KeyPostgresView, KeyPostgresManage),
		}},
		{ID: "postgres_credentials", Name: "Postgres credentials and connection details", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanViewSensitive), KeyPostgresCredentials),
			lvl(KeyLevelWrite, rels(RelCanCreate), KeyPostgresCredentialsManage),
			lvl(KeyLevelReadWrite, rels(RelCanViewSensitive, RelCanCreate), KeyPostgresCredentials, KeyPostgresCredentialsManage),
		}},
		{ID: "postgres_exports", Name: "Postgres exports and backups", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanViewSensitive), KeyPostgresExports),
			lvl(KeyLevelReadWrite, rels(RelCanViewSensitive, RelCanCreate), KeyPostgresExports, KeyPostgresExportsCreate),
		}},
	}},
	{ID: "key_value", Name: "Key Value", Permissions: []KeyPermission{
		{ID: "key_value", Name: "Key Value instances", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyKeyValueView),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanCreate), KeyKeyValueView, KeyKeyValueManage),
		}},
		{ID: "key_value_connection", Name: "Read Key Value connection details", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanViewSensitive), KeyKeyValueConnection),
		}},
	}},
	{ID: "object_storage", Name: "Object storage", Permissions: []KeyPermission{
		{ID: "objects", Name: "Objects and blobs", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanViewSensitive), KeyObjectsView),
			lvl(KeyLevelReadWrite, rels(RelCanViewSensitive, RelCanCreate), KeyObjectsView, KeyObjectsManage),
		}},
	}},
	{ID: "build_sources", Name: "Build sources", Permissions: []KeyPermission{
		{ID: "build_sources", Name: "Build sources and builds", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyBuildSourcesView),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanCreate), KeyBuildSourcesView, KeyBuildSourcesManage, KeyBuildSourcesBuild),
		}},
		{ID: "build_source_env", Name: "Build-source environment variables and secret files", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanViewSensitive), KeyBuildSourcesEnv),
			lvl(KeyLevelWrite, rels(RelCanCreate), KeyBuildSourcesEnvSet),
			lvl(KeyLevelReadWrite, rels(RelCanViewSensitive, RelCanCreate), KeyBuildSourcesEnv, KeyBuildSourcesEnvSet),
		}},
	}},
	{ID: "workspaces", Name: "Workspaces", Permissions: []KeyPermission{
		{ID: "workspaces", Name: "Members, notifications, webhooks, and log and metric streams", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView),
				KeyWorkspaceMembers, KeyWorkspaceNotifications, KeyWorkspaceWebhooks, KeyWorkspaceStreams,
				KeyWorkspaceRegistry, KeyWorkspaceSettings),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanManage),
				KeyWorkspaceMembers, KeyWorkspaceNotifications, KeyWorkspaceWebhooks, KeyWorkspaceStreams,
				KeyWorkspaceRegistry, KeyWorkspaceSettings,
				KeyWorkspaceMembersManage, KeyWorkspaceNotificationsManage, KeyWorkspaceWebhooksManage, KeyWorkspaceStreamsManage,
				KeyWorkspaceRegistryManage, KeyWorkspaceSettingsManage),
		}},
	}},
	{ID: "projects", Name: "Projects", Permissions: []KeyPermission{
		{ID: "projects", Name: "Projects and environments", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyProjectsView),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanCreate), KeyProjectsView, KeyProjectsManage, KeyProjectsMove, KeyProjectsProtect),
		}},
	}},
	{ID: "env_groups", Name: "Environment groups", Permissions: []KeyPermission{
		{ID: "env_groups", Name: "Environment groups", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyEnvGroupsView),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanCreate), KeyEnvGroupsView, KeyEnvGroupsManage),
		}},
		{ID: "env_group_values", Name: "Environment group variables and secret files", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanViewSensitive), KeyEnvGroupsValues),
			lvl(KeyLevelWrite, rels(RelCanCreate), KeyEnvGroupsValuesSet),
			lvl(KeyLevelReadWrite, rels(RelCanViewSensitive, RelCanCreate), KeyEnvGroupsValues, KeyEnvGroupsValuesSet),
		}},
	}},
	{ID: "blueprints", Name: "Blueprints", Permissions: []KeyPermission{
		{ID: "blueprints", Name: "Blueprints", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyBlueprintsView),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanCreate), KeyBlueprintsView, KeyBlueprintsManage),
		}},
	}},
	{ID: "audit_logs", Name: "Audit logs", Permissions: []KeyPermission{
		{ID: "audit_logs", Name: "Read workspace audit logs", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanManage), KeyAuditLogsView),
		}},
	}},
	{ID: "cloud_agents", Name: "Cloud agents", Bex: true, Permissions: []KeyPermission{
		{ID: "agents", Name: "Agent sessions, automations, skills, and repository environments", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyAgentsView),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanCreate), KeyAgentsView, KeyAgentsManage),
		}},
		{ID: "agent_secrets", Name: "Agent secrets", Levels: []KeyPermissionLevel{
			lvl(KeyLevelWrite, rels(RelCanCreate), KeyAgentsSecretsSet),
		}},
	}},
	{ID: "git", Name: "Git", Bex: true, Permissions: []KeyPermission{
		{ID: "git", Name: "Git connections and repositories", Levels: []KeyPermissionLevel{
			lvl(KeyLevelRead, rels(RelCanView), KeyGitView),
			lvl(KeyLevelReadWrite, rels(RelCanView, RelCanManage), KeyGitView, KeyGitManage),
		}},
	}},
}

// keyPermissions indexes the catalog by permission id; keyPermissionOrder
// gives each its position in the catalog; keyActionInfo indexes KeyActions.
var keyPermissions, keyPermissionOrder = func() (map[string]KeyPermission, map[string]int) {
	byID, order := map[string]KeyPermission{}, map[string]int{}
	for _, area := range KeyPermissionCatalog {
		for _, p := range area.Permissions {
			order[p.ID] = len(byID)
			byID[p.ID] = p
		}
	}
	return byID, order
}()

var keyActionInfo = func() map[KeyAction]KeyActionInfo {
	out := make(map[KeyAction]KeyActionInfo, len(KeyActions))
	for _, a := range KeyActions {
		out[a.ID] = a
	}
	return out
}()

// KnownKeyAction reports whether a is an action of the catalog.
func KnownKeyAction(a KeyAction) bool {
	_, ok := keyActionInfo[a]
	return ok
}

// KeyActionNamed is an action of the catalog with its names.
func KeyActionNamed(a KeyAction) KeyActionInfo { return keyActionInfo[a] }

// LookupKeyPermissionLevel returns the level of permission the catalog offers.
func LookupKeyPermissionLevel(permission string, level KeyLevel) (KeyPermissionLevel, bool) {
	p, ok := keyPermissions[permission]
	if !ok {
		return KeyPermissionLevel{}, false
	}
	for _, l := range p.Levels {
		if l.Level == level {
			return l, true
		}
	}
	return KeyPermissionLevel{}, false
}

// KeyGrantEntry is one permission of a grant at one level.
type KeyGrantEntry struct {
	Permission string   `json:"permission"`
	Level      KeyLevel `json:"level"`
}

// KeyGrant is what a key may do: All (every permission its creator's role
// allows, a key minted before scoped keys or with no permissions named), or
// exactly Grants. An empty Grants with All false is Render's "Clear all": the
// key may only identify itself and its workspace.
type KeyGrant struct {
	All    bool            `json:"all"`
	Grants []KeyGrantEntry `json:"grants"`
}

// AllKeyGrant is the grant of every key with no recorded permissions.
func AllKeyGrant() KeyGrant { return KeyGrant{All: true, Grants: []KeyGrantEntry{}} }

// Allows reports whether the grant covers every action. KeyAnyGrant is
// covered by every grant, KeyNoGrant only by All, an unknown action by none.
func (g KeyGrant) Allows(actions ...KeyAction) bool {
	if g.All {
		return true
	}
	for _, a := range actions {
		if a == KeyAnyGrant {
			continue
		}
		if !g.has(a) {
			return false
		}
	}
	return true
}

func (g KeyGrant) has(a KeyAction) bool {
	for _, e := range g.Grants {
		if l, ok := LookupKeyPermissionLevel(e.Permission, e.Level); ok && slices.Contains(l.Actions, a) {
			return true
		}
	}
	return false
}

// Entries is the grant's storage form, "permission:level" sorted; nil for All.
func (g KeyGrant) Entries() []string {
	if g.All {
		return nil
	}
	out := make([]string, 0, len(g.Grants))
	for _, e := range g.Grants {
		out = append(out, e.Permission+":"+string(e.Level))
	}
	slices.Sort(out)
	return out
}

// KeyGrantFromEntries reads a stored, scoped grant, in catalog order (a
// stored All is NULL, which its reader maps to AllKeyGrant). A stored entry
// the catalog no longer offers grants nothing (fail closed) and is logged.
func KeyGrantFromEntries(entries []string) KeyGrant {
	g := KeyGrant{Grants: make([]KeyGrantEntry, 0, len(entries))}
	for _, s := range entries {
		permission, level, _ := strings.Cut(s, ":")
		if _, ok := LookupKeyPermissionLevel(permission, KeyLevel(level)); !ok {
			log.Printf("core: API key grant entry %q is not in the catalog; it grants nothing", s)
			continue
		}
		g.Grants = append(g.Grants, KeyGrantEntry{Permission: permission, Level: KeyLevel(level)})
	}
	return g.ordered()
}

// ValidateKeyGrant checks a requested grant against the catalog: a named
// permission must exist, offer the level, and appear once; All names none.
// The result is the grant in catalog order.
func ValidateKeyGrant(g KeyGrant) (KeyGrant, error) {
	if g.All {
		if len(g.Grants) > 0 {
			return KeyGrant{}, fmt.Errorf("%w: permissions: all and grants are exclusive", ErrBadRequest)
		}
		return AllKeyGrant(), nil
	}
	seen := map[string]bool{}
	for _, e := range g.Grants {
		p, ok := keyPermissions[e.Permission]
		if !ok {
			return KeyGrant{}, fmt.Errorf("%w: permissions: unknown permission %q", ErrBadRequest, e.Permission)
		}
		if _, ok := LookupKeyPermissionLevel(e.Permission, e.Level); !ok {
			offered := make([]string, 0, len(p.Levels))
			for _, l := range p.Levels {
				offered = append(offered, string(l.Level))
			}
			return KeyGrant{}, fmt.Errorf("%w: permissions: %q offers %s, not %q", ErrBadRequest, e.Permission, strings.Join(offered, ", "), e.Level)
		}
		if seen[e.Permission] {
			return KeyGrant{}, fmt.Errorf("%w: permissions: %q is named twice", ErrBadRequest, e.Permission)
		}
		seen[e.Permission] = true
	}
	return KeyGrant{Grants: append([]KeyGrantEntry{}, g.Grants...)}.ordered(), nil
}

// ordered sorts the grant's entries into catalog order.
func (g KeyGrant) ordered() KeyGrant {
	if g.All {
		return g
	}
	slices.SortFunc(g.Grants, func(a, b KeyGrantEntry) int {
		return keyPermissionOrder[a.Permission] - keyPermissionOrder[b.Permission]
	})
	return g
}

// KeyGrantResolver reads a bex API key's recorded grant. A key with no
// recorded grant (minted before scoped keys, or with All) answers All. An
// error means the store could not answer, which refuses the request (I7).
type KeyGrantResolver interface {
	APIKeyGrant(ctx context.Context, clientID string) (KeyGrant, error)
}

// KeyGrant is the caller's grant: All for anything that is not an API key and
// for a deployment with no KeyGrants (the store-off mode, where no key can be
// scoped). It is read uncached, so a narrowed grant applies on the key's next
// request on every replica (I8). An unreadable grant is ErrAuthzUnavailable,
// never All (I7).
func (b *Base) KeyGrant(ctx context.Context) (KeyGrant, error) {
	id, ok := IdentityFrom(ctx)
	if !ok || !id.IsAPIKey() || b == nil || b.KeyGrants == nil {
		return AllKeyGrant(), nil
	}
	g, err := b.KeyGrants.APIKeyGrant(ctx, id.KeyClientID())
	if err != nil {
		log.Printf("core: API key grant lookup unavailable: %v", err)
		return KeyGrant{}, ErrAuthzUnavailable
	}
	return g, nil
}

// RequireKeyGrant refuses (403, Render's shape) a caller whose grant does not
// cover every action, and fails closed (ErrAuthzUnavailable) when the grant
// cannot be read: the form a live stream's watchdog re-asks.
func (b *Base) RequireKeyGrant(ctx context.Context, actions ...KeyAction) error {
	granted, err := b.KeyGrantAllows(ctx, actions...)
	if err != nil {
		return err
	}
	if !granted {
		return NewKeyPermissionDeniedError(actions)
	}
	return nil
}

// KeyGrantAllows reports whether the caller's grant covers every action: the
// form a field-level projection uses (does a read reveal a group's values)
// beside its relation check, which already carries the creator ceiling. It
// is true for every caller that is not a scoped API key.
func (b *Base) KeyGrantAllows(ctx context.Context, actions ...KeyAction) (bool, error) {
	g, err := b.KeyGrant(ctx)
	if err != nil {
		return false, err
	}
	return g.Allows(actions...), nil
}

// KeyStreamAction is what a gateway ticket minted now carries for the stream
// it opens (w9/125): action when the caller is a bex API key, else "". The
// SSH gateway cannot read a grant, so for a ticket that names an action it
// asks bex-api at redemption and on every revalidation tick whether the key's
// grant still covers it, and ends the stream when it no longer does. A
// person's ticket names none and is held to their relations alone.
func KeyStreamAction(ctx context.Context, action KeyAction) string {
	if id, ok := IdentityFrom(ctx); ok && id.IsAPIKey() {
		return string(action)
	}
	return ""
}

// SubjectHoldsFresh reports, uncached and audit-silent, whether subject (a
// person) holds relation on object: the role cap a key's grant is validated
// against at mint and on every edit, at the freshness of a write. nil Authz
// (authorization not enforced) holds everything.
func (b *Base) SubjectHoldsFresh(ctx context.Context, subject, relation, object string) (bool, error) {
	if b.Authz == nil {
		return true, nil
	}
	if subject == "" || !validFGAObject(object) {
		return false, nil
	}
	allowed, err := b.uncachedCheck()(ctx, "user:"+subject, relation, object)
	if err != nil {
		log.Printf("core: authorization unavailable: %v", err)
		return false, ErrAuthzUnavailable
	}
	return allowed, nil
}

// uncachedCheck is the checker's authoritative decision: CheckFresh when it
// offers one, else its Check.
func (b *Base) uncachedCheck() checkFunc {
	if fresh, ok := b.Authz.(FreshChecker); ok {
		return fresh.CheckFresh
	}
	return b.Authz.Check
}

// KeyGrantCap answers which levels one person may grant a key in one
// workspace: they must hold every relation a level names (Render: "Your
// member role in the selected workspace determines which permissions you can
// grant an API key"), each relation asked once.
type KeyGrantCap struct {
	b                  *Base
	subject, workspace string
	held               map[string]bool
}

// KeyGrantCap is subject's cap in workspaceID.
func (b *Base) KeyGrantCap(subject, workspaceID string) *KeyGrantCap {
	return &KeyGrantCap{b: b, subject: subject, workspace: workspaceID, held: map[string]bool{}}
}

// Grantable reports whether the cap allows level.
func (c *KeyGrantCap) Grantable(ctx context.Context, level KeyPermissionLevel) (bool, error) {
	for _, rel := range level.Relations {
		ok, seen := c.held[rel]
		if !seen {
			var err error
			if ok, err = c.b.SubjectHoldsFresh(ctx, c.subject, rel, WorkspaceObject(c.workspace)); err != nil {
				return false, err
			}
			c.held[rel] = ok
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// RequireGrantableKeyGrant refuses a grant naming a level subject's current
// role in workspaceID does not allow, naming the permission (403). All is
// always grantable: it is whatever the role allows, now and later.
func (b *Base) RequireGrantableKeyGrant(ctx context.Context, subject, workspaceID string, g KeyGrant) error {
	if g.All {
		return nil
	}
	caps := b.KeyGrantCap(subject, workspaceID)
	for _, e := range g.Grants {
		level, ok := LookupKeyPermissionLevel(e.Permission, e.Level)
		if !ok {
			return fmt.Errorf("%w: permissions: unknown permission %q", ErrBadRequest, e.Permission)
		}
		grantable, err := caps.Grantable(ctx, level)
		if err != nil {
			return err
		}
		if !grantable {
			return NewForbiddenError(KeyPermissionAboveRoleCode,
				fmt.Sprintf("the key creator's member role does not allow %s at %s; it needs %s", keyPermissions[e.Permission].Name, KeyLevelName(e.Level), KeyLevelRequiredRole(level)),
				map[string]any{"permission": e.Permission, "level": string(e.Level), "requiredRole": KeyLevelRequiredRole(level)})
		}
	}
	return nil
}

// KeyPermissionAboveRoleCode is the code of a grant its creator's role cannot give.
const KeyPermissionAboveRoleCode = "api_key_permission_above_role"

// KeyLevelName is Render's name of a level.
func KeyLevelName(l KeyLevel) string {
	switch l {
	case KeyLevelRead:
		return "Read"
	case KeyLevelWrite:
		return "Write"
	case KeyLevelReadWrite:
		return "Read & write"
	case KeyLevelRun:
		return "Run"
	case KeyLevelExecute:
		return "Execute"
	}
	return string(l)
}

// relationRole is the lowest member role holding each relation a catalog
// level names (deploy/gitops/authz/model.fga).
var relationRole = map[string]int{
	RelCanView: 0, RelCanViewLogs: 1, RelCanOperate: 1, RelCanCreate: 2, RelCanViewSensitive: 2, RelCanManage: 3,
}

var roleNames = []string{"viewer", "contributor", "developer", "admin"}

// KeyLevelRequiredRole is the lowest member role that may grant level.
func KeyLevelRequiredRole(level KeyPermissionLevel) string {
	rank := 0
	for _, rel := range level.Relations {
		rank = max(rank, relationRole[rel])
	}
	return roleNames[rank]
}

// NewKeyPermissionDeniedError is the refusal of an operation outside a key's
// grant: Render's 403 shape, naming the permissions the operation needs and
// nothing about its target.
func NewKeyPermissionDeniedError(needed []KeyAction) *CodedError {
	names := make([]string, 0, len(needed))
	for _, a := range needed {
		names = append(names, KeyActionName(a))
	}
	msg := "this API key is not granted the permission this request needs"
	if len(names) > 0 {
		msg += ": " + strings.Join(names, "; ")
	}
	return NewForbiddenError(KeyPermissionDeniedCode, msg, map[string]any{"required": actionStrings(needed)})
}

// KeyPermissionDeniedCode is the machine-readable code of a grant refusal.
const KeyPermissionDeniedCode = "api_key_permission_denied"

// KeyActionName is an action's catalog name, or the action itself.
func KeyActionName(a KeyAction) string {
	switch a {
	case KeyNoGrant:
		return "a permission no scoped API key can hold (create a key with All)"
	case KeyAnyGrant:
		return "none"
	}
	if info, ok := keyActionInfo[a]; ok {
		return info.Name
	}
	return string(a)
}

func actionStrings(actions []KeyAction) []string {
	out := make([]string, 0, len(actions))
	for _, a := range actions {
		out = append(out, string(a))
	}
	return out
}
