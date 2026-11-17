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
// time or every native runtime service at once (w32/m462):
//
//	kubectl -n bex-system exec deploy/bex-api -- /api migrate-container-policy srv-… [--apply]
//	kubectl -n bex-system exec deploy/bex-api -- /api migrate-container-policy --all-native [--apply]
//
// Without --apply it prints the plan and writes nothing.
const migrateContainerPolicyCommand = "migrate-container-policy"

// migrateContainerPolicyArgs is the command's parsed arguments: one service
// id, or allNative.
type migrateContainerPolicyArgs struct {
	serviceID string
	allNative bool
	apply     bool
}

// parseMigrateContainerPolicyArgs reads `<srv-id> [--apply]` or
// `--all-native [--apply]`.
func parseMigrateContainerPolicyArgs(args []string) (migrateContainerPolicyArgs, error) {
	var parsed migrateContainerPolicyArgs
	usage := fmt.Errorf("usage: %s (<srv-id> | --all-native) [--apply]", migrateContainerPolicyCommand)
	for _, arg := range args {
		switch {
		case arg == "--apply":
			parsed.apply = true
		case arg == "--all-native" && parsed.serviceID == "":
			parsed.allNative = true
		case parsed.serviceID == "" && !parsed.allNative && len(arg) > 0 && arg[0] != '-':
			parsed.serviceID = arg
		default:
			return migrateContainerPolicyArgs{}, usage
		}
	}
	if parsed.serviceID == "" && !parsed.allNative {
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
			log.Printf("bex-api: migrated %s (%s) from container policy %q to %q", m.ServiceID, m.Workspace, m.From, m.To)
		}
	}
	enc := json.NewEncoder(out)
	if parsed.allNative {
		summary, err := svc.MigrateNativeContainerPolicies(ctx, parsed.apply, func(line apps.ContainerPolicyMigrationLine) {
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
