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
// under agent-sessions/, those holding a model key, flags newly set, and
// entries whose key or flag could not be read or written.
type ModelKeyBackfill struct {
	Workspaces, Keys, Marked, Failed int
}

// BackfillModelKeyFlags sets model_key_set_at for every workspace whose model
// key exists in OpenBao but has no flag: keys put there by hand before the
// write-only modelApiKey existed (ADR091 D7), which route decisions would
// otherwise ignore. It never clears a flag, and a workspace that no longer
// exists is skipped. bex-api runs it once at startup, in the background;
// only counts are reported, never a workspace's key.
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
