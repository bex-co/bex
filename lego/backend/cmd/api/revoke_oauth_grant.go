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
	"log"

	"github.com/bex-co/bex/lego/backend/internal/oauthgrant"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// revokeOAuthGrantCommand is the operator's cut-off for one human's OAuth grant
// (w21/090, docs/runbooks/oauth-grant-cutoff.md), run inside the bex-api pod:
//
//	kubectl -n bex-system exec deploy/bex-api -- /api revoke-oauth-grant <subject> <client-id> [--apply]
//	kubectl -n bex-system exec deploy/bex-api -- /api revoke-oauth-grant <subject> --all-clients [--apply]
//
// It revokes the (subject, client) consent session at Hydra, which kills every
// access and refresh token of that grant, then advances the shared
// oauth_revocations marker for the same pair so every bex-api replica refuses
// the token at its next request instead of when its introspection cache entry
// expires, and records the action in the ops workspace's audit trail. Without
// --apply it prints the subject's grants and the plan and writes nothing.
const revokeOAuthGrantCommand = "revoke-oauth-grant"

const revokeOAuthGrantUsage = "usage: " + revokeOAuthGrantCommand + " <subject> (<client-id> | --all-clients) [--apply]"

// parseRevokeOAuthGrantArgs reads `<subject> [<client-id>] [--all-clients]
// [--apply]` in any order.
func parseRevokeOAuthGrantArgs(args []string) (oauthgrant.Request, error) {
	var req oauthgrant.Request
	var positional []string
	for _, arg := range args {
		switch {
		case arg == "--apply":
			req.Apply = true
		case arg == "--all-clients":
			req.AllClients = true
		case len(arg) > 0 && arg[0] != '-':
			positional = append(positional, arg)
		default:
			return oauthgrant.Request{}, errors.New(revokeOAuthGrantUsage)
		}
	}
	if len(positional) == 0 || len(positional) > 2 || (len(positional) == 2) == req.AllClients {
		return oauthgrant.Request{}, errors.New(revokeOAuthGrantUsage)
	}
	req.Subject = positional[0]
	if len(positional) == 2 {
		req.ClientID = positional[1]
	}
	return req, nil
}

// runRevokeOAuthGrant prints the plan or the outcome as JSON and returns the
// exit code.
func runRevokeOAuthGrant(ctx context.Context, cfg *Config, args []string, out, errOut io.Writer) int {
	req, err := parseRevokeOAuthGrantArgs(args)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if cfg.HydraAdminURL == "" || cfg.CPDBURI == "" {
		fmt.Fprintln(errOut, "BEX_HYDRA_ADMIN_URL and BEX_CP_DB_URI are required: the grant lives in Hydra and the shared marker in the control-plane database")
		return 2
	}
	pool := openControlPlaneDB(ctx, cfg.CPDBURI)
	defer pool.Close()
	st := store.NewPGStore(pool)
	revoker := &oauthgrant.Revoker{
		HydraAdminURL:  cfg.HydraAdminURL,
		Markers:        st,
		Audit:          st,
		AuditWorkspace: cfg.OpsWorkspace,
	}
	res, runErr := revoker.Run(ctx, req)
	for _, rev := range res.Revoked {
		if rev.Marker {
			// The pod's log is the durable record beside the audit row.
			log.Printf("bex-api: operator revoked the OAuth grant of subject %s to client %s (hydra: %s, audited: %t)",
				res.Subject, rev.ClientID, rev.Hydra, rev.Audited)
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
	if runErr != nil {
		fmt.Fprintln(errOut, runErr)
		return 1
	}
	return 0
}
