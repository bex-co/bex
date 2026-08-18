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

package envgroups

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// NameClaimDuplicate is non-secret operator evidence for one legacy ambiguous
// name. The audit never reads env/file paths and never chooses a winner.
type NameClaimDuplicate struct {
	WorkspaceID string   `json:"workspaceId"`
	Name        string   `json:"name"`
	IDs         []string `json:"ids"`
}

type NameClaimAuditReport struct {
	Scanned    int                  `json:"scanned"`
	Missing    int                  `json:"missing"`
	Created    int                  `json:"created"`
	Existing   int                  `json:"existing"`
	Conflicts  int                  `json:"conflicts"`
	Duplicates []NameClaimDuplicate `json:"duplicates"`
}

type claimCandidate struct {
	id, workspace, name string
}

// AuditNameClaims is the opt-in startup-safe backfill path. dryRun reports the
// exact work without writing. Apply mode creates only unambiguous missing
// claims by CAS and is idempotent; duplicate or conflicting claims are reported
// for manual repair and are never merged, renamed, or deleted.
func AuditNameClaims(ctx context.Context, store core.SecretKV, dryRun bool) (NameClaimAuditReport, error) {
	var report NameClaimAuditReport
	if store == nil {
		return report, core.ErrSecretsUnavailable
	}
	versioned, ok := store.(core.VersionedSecretKV)
	if !ok && !dryRun {
		return report, core.ErrSecretsUnavailable
	}
	ids, err := store.List(ctx, "env-groups")
	if err != nil {
		return report, err
	}
	sort.Strings(ids)
	byName := make(map[string][]claimCandidate)
	for _, gid := range ids {
		raw, readErr := store.Get(ctx, metaPath(gid))
		if readErr != nil {
			return report, readErr
		}
		if len(raw) == 0 {
			continue // metadata-last create currently in flight
		}
		name := strings.TrimSpace(raw["name"])
		workspace := raw["workspace"]
		report.Scanned++
		byName[workspace+"\x00"+name] = append(byName[workspace+"\x00"+name], claimCandidate{
			id: gid, workspace: workspace, name: name,
		})
	}
	keys := make([]string, 0, len(byName))
	for key := range byName {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		candidates := byName[key]
		if len(candidates) > 1 {
			duplicate := NameClaimDuplicate{WorkspaceID: candidates[0].workspace, Name: candidates[0].name}
			for _, candidate := range candidates {
				duplicate.IDs = append(duplicate.IDs, candidate.id)
			}
			report.Duplicates = append(report.Duplicates, duplicate)
			continue
		}
		candidate := candidates[0]
		path := envGroupNameClaimPath(candidate.workspace, candidate.name)
		if dryRun {
			claim, readErr := store.Get(ctx, path)
			if readErr != nil {
				return report, readErr
			}
			switch claim["id"] {
			case candidate.id:
				report.Existing++
			case "":
				report.Missing++
			default:
				report.Conflicts++
			}
			continue
		}
		snapshot, readErr := versioned.GetVersioned(ctx, path)
		if readErr != nil {
			return report, readErr
		}
		switch snapshot.Data["id"] {
		case candidate.id:
			report.Existing++
			continue
		case "":
			report.Missing++
		default:
			report.Conflicts++
			continue
		}
		_, putErr := versioned.PutCAS(ctx, path, map[string]string{
			"id": candidate.id, "name": candidate.name, "workspace": candidate.workspace,
		}, snapshot.Version)
		if putErr != nil {
			if errors.Is(putErr, core.ErrConflict) {
				report.Conflicts++
				continue
			}
			return report, putErr
		}
		report.Created++
	}
	return report, nil
}
