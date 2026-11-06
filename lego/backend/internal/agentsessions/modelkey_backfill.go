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

package agentsessions

import (
	"context"
	"errors"
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ModelKeyFlagMarker records that a workspace has its own model key without
// ever clearing one; *store.PGStore satisfies it.
type ModelKeyFlagMarker interface {
	MarkAgentModelKeySet(ctx context.Context, workspaceID string) (bool, error)
}

// ModelKeyBackfill counts one BackfillModelKeyFlags pass: workspace entries
// under agent-sessions/, those holding a model key, flags newly set, keys
// left unflagged because the platform route covers their workspace, and
// entries whose key or flag could not be read or written.
type ModelKeyBackfill struct {
	Workspaces, Keys, Marked, PlatformRouted, Failed int
}

// BackfillModelKeyFlags sets model_key_set_at for every workspace whose model
// key exists in OpenBao but has no flag: keys put there by hand before the
// write-only modelApiKey existed (ADR091 D7), which route decisions would
// otherwise ignore. It never clears a flag, and a workspace that no longer
// exists is skipped. bex-api runs it once at startup, in the background;
// only counts are reported, never a workspace's key.
//
// A workspace the platform route covers is skipped too: the route never read
// its hand-put key, so that key was never in effect and is no admin's choice.
// Flagging it would move the workspace off the platform key onto a stale
// vendor key — every claude session then fails its first model call as
// vendor-auth-rejected (the 2026-11-06 workspace bex incident). An admin
// opts such a workspace into its own key through modelApiKey.
func BackfillModelKeyFlags(ctx context.Context, keys core.SecretKV, flags ModelKeyFlagMarker) (ModelKeyBackfill, error) {
	var out ModelKeyBackfill
	workspaces, err := keys.List(ctx, "agent-sessions")
	if err != nil {
		return out, fmt.Errorf("list agent-session secrets: %w", err)
	}
	out.Workspaces = len(workspaces)
	for _, workspaceID := range workspaces {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		data, err := keys.Get(ctx, agentsession.ModelKeySecretPath(workspaceID))
		if err != nil {
			out.Failed++
			continue
		}
		if data[agentsession.ModelKeyField] == "" {
			continue
		}
		out.Keys++
		if agentsession.PlatformModelKeyReady(workspaceID, false) {
			out.PlatformRouted++
			continue
		}
		marked, err := flags.MarkAgentModelKeySet(ctx, workspaceID)
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			out.Failed++
		case marked:
			out.Marked++
		}
	}
	return out, nil
}
