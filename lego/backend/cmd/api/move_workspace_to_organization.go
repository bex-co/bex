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
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// moveWorkspaceToOrganizationCommand is Render support's transfer of an
// existing workspace into an organization (w7/m464, docs/ADR115-
// organizations.md D3: "Existing workspaces can be transferred into an org by
// contacting Render support"), run inside the bex-api pod on a customer's
// request:
//
//	kubectl -n bex-system exec deploy/bex-api -- /api move-workspace-to-organization <tea-id> <org-id> [--apply]
//
// It attaches a standalone workspace to the org, sets its plan to the org's,
// makes each of its people not yet in the org a Member, and writes the org
// tuples. Without --apply it prints what it would do and writes nothing. A
// workspace already in an org (a primary included) is refused.
const moveWorkspaceToOrganizationCommand = "move-workspace-to-organization"

const moveWorkspaceToOrganizationUsage = "usage: " + moveWorkspaceToOrganizationCommand + " <workspace-id> <organization-id> [--apply]"

type moveWorkspaceRequest struct {
	workspaceID, organizationID string
	apply                       bool
}

func parseMoveWorkspaceArgs(args []string) (moveWorkspaceRequest, error) {
	var req moveWorkspaceRequest
	var positional []string
	for _, arg := range args {
		switch {
		case arg == "--apply":
			req.apply = true
		case arg != "" && !strings.HasPrefix(arg, "-"):
			positional = append(positional, arg)
		default:
			return moveWorkspaceRequest{}, errors.New(moveWorkspaceToOrganizationUsage)
		}
	}
	if len(positional) != 2 || !strings.HasPrefix(positional[0], "tea-") || !strings.HasPrefix(positional[1], "org-") {
		return moveWorkspaceRequest{}, errors.New(moveWorkspaceToOrganizationUsage)
	}
	req.workspaceID, req.organizationID = positional[0], positional[1]
	return req, nil
}

// runMoveWorkspaceToOrganization prints the transfer plan or its outcome as
// JSON and returns the exit code.
func runMoveWorkspaceToOrganization(ctx context.Context, cfg *Config, args []string, out, errOut io.Writer) int {
	req, err := parseMoveWorkspaceArgs(args)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if cfg.CPDBURI == "" {
		fmt.Fprintln(errOut, "BEX_CP_DB_URI is required: organizations live in the control-plane database")
		return 2
	}
	pool := openControlPlaneDB(ctx, cfg.CPDBURI)
	defer pool.Close()
	st := store.NewPGStore(pool)
	svc := &workspaces.Service{Base: &core.Base{Audit: st}, Store: st}
	if cfg.OpenFGAURL != "" {
		if w, ok := authz.NewOpenFGAChecker(cfg.OpenFGAURL, cfg.OpenFGAToken).(core.OrganizationTupleWriter); ok {
			svc.OrgTuples = w
		}
	}
	plan, runErr := workspaces.OrganizationTransfer{Service: svc}.Move(ctx, req.workspaceID, req.organizationID, req.apply)
	if runErr != nil && plan.WorkspaceID == "" {
		fmt.Fprintln(errOut, runErr)
		return 1
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(plan)
	if runErr != nil {
		fmt.Fprintln(errOut, runErr)
		return 1
	}
	return 0
}
