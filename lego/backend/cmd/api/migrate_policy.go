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

// migrateContainerPolicyCommand is the operator-run move of one existing
// service onto render-v1 (w6/m149 t008), run inside the bex-api pod:
//
//	kubectl -n bex-system exec deploy/bex-api -- /api migrate-container-policy srv-… [--apply]
//
// Without --apply it prints the plan and writes nothing.
const migrateContainerPolicyCommand = "migrate-container-policy"

// parseMigrateContainerPolicyArgs reads `<srv-id> [--apply]`.
func parseMigrateContainerPolicyArgs(args []string) (serviceID string, apply bool, err error) {
	for _, arg := range args {
		switch {
		case arg == "--apply":
			apply = true
		case serviceID == "" && len(arg) > 0 && arg[0] != '-':
			serviceID = arg
		default:
			return "", false, fmt.Errorf("usage: %s <srv-id> [--apply]", migrateContainerPolicyCommand)
		}
	}
	if serviceID == "" {
		return "", false, fmt.Errorf("usage: %s <srv-id> [--apply]", migrateContainerPolicyCommand)
	}
	return serviceID, apply, nil
}

// runMigrateContainerPolicy prints the migration as JSON and returns the exit
// code.
func runMigrateContainerPolicy(ctx context.Context, cpDBURI string, base *core.Base, args []string, out, errOut io.Writer) int {
	serviceID, apply, err := parseMigrateContainerPolicyArgs(args)
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
	result, err := svc.MigrateContainerPolicy(ctx, serviceID, apply)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if result.Applied {
		log.Printf("bex-api: migrated %s (%s) from container policy %q to %q", result.ServiceID, result.Workspace, result.From, result.To)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
	return 0
}
