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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/oidcissuer"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The Managed OIDC operator commands (docs/ADR116-managed-oidc.md D6,
// docs/runbooks/oidc-key-rotation.md), run inside the bex-api pod:
//
//	kubectl -n bex-system exec deploy/bex-api -- /api oidc-rotate-key [--emergency] [--apply]
//	kubectl -n bex-system exec deploy/bex-api -- /api oidc-test-token <tea-…> <srv-…>
//
// oidc-rotate-key adds a signing key: published now, signing an hour later
// (with --emergency: signing at once, every other key deleted). Without
// --apply it prints the key set and the plan and writes nothing.
//
// oidc-test-token prints a token for one service, for
// scripts/oidc-issuer-verify.sh to check through the public issuer. Its
// audience is fixed to oidcTestTokenAudience, which no provider accepts, so
// the command cannot be used to act as a tenant at AWS, GCP or a model
// provider.
const (
	oidcRotateKeyCommand  = "oidc-rotate-key"
	oidcTestTokenCommand  = "oidc-test-token"
	oidcTestTokenAudience = "bex-oidc-verify"
)

const (
	oidcRotateKeyUsage = "usage: " + oidcRotateKeyCommand + " [--emergency] [--apply]"
	oidcTestTokenUsage = "usage: " + oidcTestTokenCommand + " <tea-…> <srv-…>"
)

func runOIDCRotateKey(ctx context.Context, cfg *Config, args []string, out, errOut io.Writer) int {
	var emergency, apply bool
	for _, arg := range args {
		switch arg {
		case "--emergency":
			emergency = true
		case "--apply":
			apply = true
		default:
			fmt.Fprintln(errOut, oidcRotateKeyUsage)
			return 2
		}
	}
	if cfg.CPDBURI == "" {
		fmt.Fprintln(errOut, "BEX_CP_DB_URI is required: the OIDC signing key set lives in the control-plane database")
		return 2
	}
	pool := openControlPlaneDB(ctx, cfg.CPDBURI)
	defer pool.Close()
	report, err := oidcissuer.Rotate(ctx, store.NewPGStore(pool), time.Now(), emergency, apply)
	if report.Keys != nil {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	}
	if errors.Is(err, store.ErrOIDCRotationPending) {
		fmt.Fprintln(errOut, "refused: a key from an earlier rotation has not started signing yet; wait until its activatesAt, or use --emergency")
		return 1
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

func runOIDCTestToken(ctx context.Context, cfg *Config, args []string, out, errOut io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(errOut, oidcTestTokenUsage)
		return 2
	}
	sub := oidcissuer.Subject{Workspace: args[0], Service: args[1]}
	if err := sub.Validate(); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if cfg.OIDCIssuerURL == "" || cfg.CPDBURI == "" {
		fmt.Fprintln(errOut, "BEX_OIDC_ISSUER_URL and BEX_CP_DB_URI are required: the issuer's origin names the token's iss, and its keys live in the control-plane database")
		return 2
	}
	pool := openControlPlaneDB(ctx, cfg.CPDBURI)
	defer pool.Close()
	keys := &oidcissuer.KeySet{Store: store.NewPGStore(pool)}
	if err := keys.Load(ctx); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	issuer, err := oidcissuer.New(cfg.OIDCIssuerURL, keys)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	tok, err := issuer.Mint(sub, oidcTestTokenAudience)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	fmt.Fprintln(out, tok.Raw)
	return 0
}
