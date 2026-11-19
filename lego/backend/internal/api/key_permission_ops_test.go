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
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// keyPermissionDrift is the guard's comparison: live operations with no entry
// in table, and entries naming no live operation.
func keyPermissionDrift(live []string, table map[string][]core.KeyAction) (missing, stale []string) {
	liveSet := map[string]bool{}
	for _, op := range live {
		liveSet[op] = true
		if _, ok := table[op]; !ok {
			missing = append(missing, op)
		}
	}
	for op := range table {
		if !liveSet[op] {
			stale = append(stale, op)
		}
	}
	slices.Sort(stale)
	return missing, stale
}

// Every live REST, GraphQL and MCP operation names what a scoped key needs to
// call it (w9/m496 t004, I7), and every entry names a live operation.
func TestKeyPermissionsCoverLiveOperations(t *testing.T) {
	missing, stale := keyPermissionDrift(liveScopeOperations(t), keyPermissionOps)
	if len(missing) > 0 || len(stale) > 0 {
		t.Fatalf("keyPermissionOps drifted from the live operations\nmissing (%d): %s\nstale (%d): %s\n"+
			"For a NEW operation, add a deliberate entry: the catalog action(s) a scoped API key must be granted "+
			"to call it, core.KeyAnyGrant, or core.KeyNoGrant (see the table's header and ADR024 § An API key "+
			"carries a chosen set of permissions). A missing entry refuses every scoped key at runtime.",
			len(missing), strings.Join(missing, ", "), len(stale), strings.Join(stale, ", "))
	}
}

// The guard has teeth: an operation added without an entry is reported, and
// so is an entry whose operation was removed.
func TestKeyPermissionDriftReportsAnUnmappedOperation(t *testing.T) {
	live := append(slices.Clone(liveScopeOperations(t)), "REST POST /v1/services/{id}/brand-new")
	missing, stale := keyPermissionDrift(live, keyPermissionOps)
	if !slices.Equal(missing, []string{"REST POST /v1/services/{id}/brand-new"}) || len(stale) != 0 {
		t.Fatalf("missing = %v, stale = %v; want only the new operation missing", missing, stale)
	}
	table := map[string][]core.KeyAction{"REST GET /v1/gone": {core.KeyServicesView}}
	for op, need := range keyPermissionOps {
		table[op] = need
	}
	if _, stale := keyPermissionDrift(live[:len(live)-1], table); !slices.Equal(stale, []string{"REST GET /v1/gone"}) {
		t.Fatalf("stale = %v; want the removed operation", stale)
	}
}

// Every entry names catalog actions, or exactly one marker.
func TestKeyPermissionEntriesAreWellFormed(t *testing.T) {
	for op, need := range keyPermissionOps {
		if len(need) == 0 {
			t.Errorf("%s: no actions", op)
			continue
		}
		for _, a := range need {
			marker := a == core.KeyAnyGrant || a == core.KeyNoGrant
			if marker && len(need) != 1 {
				t.Errorf("%s: marker %q mixed with actions %v", op, a, need)
			}
			if !marker && !core.KnownKeyAction(a) {
				t.Errorf("%s: unknown action %q", op, a)
			}
		}
	}
}

// readLevelActions are the actions some permission grants at its Read level:
// what Render's "Read only" preset can give a key.
func readLevelActions() map[core.KeyAction]bool {
	out := map[core.KeyAction]bool{}
	for _, area := range core.KeyPermissionCatalog {
		for _, p := range area.Permissions {
			for _, l := range p.Levels {
				if l.Level == core.KeyLevelRead {
					for _, a := range l.Actions {
						out[a] = true
					}
				}
			}
		}
	}
	return out
}

// writesThatOnlyRead are the write- or mint-class operations a Read-level
// action is enough for, each because it changes nothing a key could not
// already read: validating or previewing a render.yaml, running SQL through
// the connection the credentials read reveals, a recovery window's lookup, a
// wait.
var writesThatOnlyRead = map[string]bool{
	"REST POST /v1/blueprints/validate":         true,
	"REST POST /v1/blueprints/preview":          true,
	"REST POST /v1/blueprints/generate":         true,
	"MCP validate_bex_yml":                      true,
	"MCP preview_blueprint":                     true,
	"MCP generate_blueprint":                    true,
	"REST POST /v1/postgres/{id}/query":         true,
	"GQL Mutation.executeDatabaseQuery":         true,
	"MCP query_render_postgres":                 true,
	"REST POST /v1/postgres/{id}/recovery-info": true,
}

// A Read only key can never write: every write- or mint-class operation needs
// an action no Read level grants (or no scoped key may call it), except the
// few that change nothing (writesThatOnlyRead).
func TestKeyPermissionWritesNeedAWriteAction(t *testing.T) {
	reads := readLevelActions()
	for op, class := range classifiedOps {
		if class != core.OpClassWrite && class != core.OpClassMint {
			continue
		}
		need := keyPermissionOps[op]
		if slices.Contains(need, core.KeyNoGrant) {
			continue
		}
		writes := slices.ContainsFunc(need, func(a core.KeyAction) bool { return a != core.KeyAnyGrant && !reads[a] })
		switch {
		case writesThatOnlyRead[op] && writes:
			t.Errorf("%s is listed as reading only but needs %v", op, need)
		case !writesThatOnlyRead[op] && !writes:
			if slices.Contains(need, core.KeyAnyGrant) && anyKeyWrites[op] {
				continue
			}
			t.Errorf("%s is a %s operation but a Read only key holds everything it needs (%v)", op, class, need)
		}
	}
}

// anyKeyWrites are the write-class operations any key may call: they touch
// only the caller's own credential or the CLI's telemetry.
var anyKeyWrites = map[string]bool{
	"REST POST /v1/oauth/revoke":         true,
	"REST POST /v1/cli-telemetry-events": true,
}

// A secret is never revealed on a resource's plain read: every sensitive read
// needs an action no plain resource view is.
func TestKeyPermissionSensitiveReadsNeedASecretView(t *testing.T) {
	secretViews := map[core.KeyAction]bool{
		core.KeyServicesEnv: true, core.KeyEnvGroupsValues: true, core.KeyBuildSourcesEnv: true,
		core.KeyPostgresCredentials: true, core.KeyPostgresExports: true, core.KeyKeyValueConnection: true,
		core.KeyObjectsView: true,
	}
	for op, class := range classifiedOps {
		if class != core.OpClassSensitive {
			continue
		}
		need := keyPermissionOps[op]
		if slices.ContainsFunc(need, func(a core.KeyAction) bool { return secretViews[a] }) {
			continue
		}
		// A sensitive write (a shell, a group link) needs its own write action.
		if slices.ContainsFunc(need, func(a core.KeyAction) bool { return !readLevelActions()[a] }) {
			continue
		}
		t.Errorf("%s reveals a secret but needs only %v", op, need)
	}
}
