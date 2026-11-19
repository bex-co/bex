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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// migrateContainerPolicyCommand is the operator-run move of existing services
// onto render-v1 (w6/m149 t008), run inside the bex-api pod, one service at a
// time, every native runtime service at once (w32/m462), or every service but
// a static site at once (w32/040):
//
//	kubectl -n bex-system exec deploy/bex-api -- /api migrate-container-policy srv-… [--apply]
//	kubectl -n bex-system exec deploy/bex-api -- /api migrate-container-policy --all-native [--apply]
//	kubectl -n bex-system exec deploy/bex-api -- /api migrate-container-policy --all [--apply]
//
// Without --apply it prints the plan and writes nothing. A service that runs
// nothing (no release has served) reads "rollout": false: its policy is saved
// for its next deploy instead of rolling it (w32/044).
const migrateContainerPolicyCommand = "migrate-container-policy"

// migrateContainerPolicyBulkFlags maps each bulk flag to the services it
// moves.
var migrateContainerPolicyBulkFlags = map[string]apps.ContainerPolicyMigrationScope{
	"--all-native": apps.MigrationScopeNative,
	"--all":        apps.MigrationScopeAll,
}

// migrateContainerPolicyArgs is the command's parsed arguments: one service
// id, or a bulk scope.
type migrateContainerPolicyArgs struct {
	serviceID string
	scope     apps.ContainerPolicyMigrationScope
	apply     bool
}

// parseMigrateContainerPolicyArgs reads `<srv-id> [--apply]`,
// `--all-native [--apply]` or `--all [--apply]`.
func parseMigrateContainerPolicyArgs(args []string) (migrateContainerPolicyArgs, error) {
	var parsed migrateContainerPolicyArgs
	usage := fmt.Errorf("usage: %s (<srv-id> | --all-native | --all) [--apply]", migrateContainerPolicyCommand)
	for _, arg := range args {
		scope, bulk := migrateContainerPolicyBulkFlags[arg]
		switch {
		case arg == "--apply":
			parsed.apply = true
		case bulk && parsed.serviceID == "" && parsed.scope == 0:
			parsed.scope = scope
		case parsed.serviceID == "" && parsed.scope == 0 && len(arg) > 0 && arg[0] != '-':
			parsed.serviceID = arg
		default:
			return migrateContainerPolicyArgs{}, usage
		}
	}
	if parsed.serviceID == "" && parsed.scope == 0 {
		return migrateContainerPolicyArgs{}, usage
	}
	return parsed, nil
}

// runMigrateContainerPolicy prints the migration as JSON and returns the exit
// code. A bulk run prints one line per service, then {"summary": …}; it exits
// 0 even when it refused services, which their lines and the summary name.
func runMigrateContainerPolicy(ctx context.Context, cpDBURI string, base *core.Base, args []string, out, errOut io.Writer) int {
	parsed, err := parseMigrateContainerPolicyArgs(args)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if cpDBURI == "" {
		fmt.Fprintln(errOut, "BEX_CP_DB_URI is required: the persisted policy lives in the control-plane database")
		return 2
	}
	pool := openControlPlaneDB(ctx, cpDBURI)
	defer pool.Close()
	svc := &apps.Service{Base: base, Store: store.NewPGStore(pool)}
	logApplied := func(m apps.ContainerPolicyMigration) {
		if m.Applied {
			how := "rolled once"
			if !m.Rollout {
				how = "saved for its next deploy"
			}
			log.Printf("bex-api: migrated %s (%s) from container policy %q to %q, %s", m.ServiceID, m.Workspace, m.From, m.To, how)
		}
	}
	enc := json.NewEncoder(out)
	if parsed.scope != 0 {
		summary, err := svc.MigrateContainerPolicies(ctx, parsed.scope, parsed.apply, func(line apps.ContainerPolicyMigrationLine) {
			logApplied(line.ContainerPolicyMigration)
			_ = enc.Encode(line)
		})
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		_ = enc.Encode(map[string]apps.ContainerPolicyMigrationSummary{"summary": summary})
		return 0
	}
	result, err := svc.MigrateContainerPolicy(ctx, parsed.serviceID, parsed.apply)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	logApplied(result)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
	return 0
}
