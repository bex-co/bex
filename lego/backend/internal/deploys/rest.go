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

package deploys

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the deploy-history REST fragment (w2/m5's list/get/trigger,
// w2/m10's cancel/rollback): Render's GET .../deploys (the {deploy, cursor}
// list envelope, honoring status/createdBefore/createdAfter/cursor/limit
// since w2/m31), GET .../deploys/{id}, POST .../deploys (trigger),
// POST .../deploys/{id}/cancel, and POST .../rollback. Served under both
// /v1/services and /v1/apps, same as every other apps-adjacent route.
// Behavior lives in the Service, so GraphQL and MCP stay identical.

// renderCommit is Render's nested deploy.commit object — the resolved commit
// a build-from-git deploy ran (w9/001). Render also carries createdAt; bex
// doesn't capture the author date, so the field is omitted, not faked.
type renderCommit struct {
	ID      string `json:"id"`
	Message string `json:"message,omitempty"`
}

// renderDeployImage is Render's nested deploy.image object
// (components.schemas.Deploy: "Image information used when creating the
// deploy. Not present for Git-backed deploys" — {ref, registryCredential,
// sha}), verified against the render-oss/cli generated client. bex only
// tracks the resolved image string, reported as Ref.
type renderDeployImage struct {
	Ref string `json:"ref,omitempty"`
}

// renderDeploy mirrors Render's deploy object for the fields bex can honor
// (id, status, commit, timestamps) plus bex-native extras (trigger,
// rollbackOf). Commit (w9/001) is Render's nested commit object, present only
// when the deploy's triggering ref was resolved through the workspace's
// GitHub connection — omitted, not faked, otherwise.
type renderDeploy struct {
	ID         string             `json:"id"`
	Status     string             `json:"status"`
	Trigger    string             `json:"trigger,omitempty"`    // bex extra: "create" | "api" | "deploy_hook" | "rollback"
	Image      *renderDeployImage `json:"image,omitempty"`      // Render's nested image object, not a bare string
	RollbackOf string             `json:"rollbackOf,omitempty"` // bex extra (w2/m10): the deploy this one restores, if any
	Commit     *renderCommit      `json:"commit,omitempty"`
	CreatedAt  string             `json:"createdAt,omitempty"`
	StartedAt  string             `json:"startedAt,omitempty"`
	FinishedAt string             `json:"finishedAt,omitempty"`
	// PreDeployStatus is the pre-deploy command's outcome (bex extra, w1/m33):
	// "running" | "succeeded" | "failed"; omitted when no pre-deploy step ran.
	// Distinguishes a migration failure from a health-check failure (both
	// status=update_failed). Logs: GET /v1/logs?service=<id>&type=predeploy.
	PreDeployStatus string `json:"preDeployStatus,omitempty"`
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func toRenderDeploy(d DeployView) renderDeploy {
	out := renderDeploy{
		ID:              d.ID,
		Status:          d.Status,
		Trigger:         d.Trigger,
		RollbackOf:      d.RollbackOf,
		CreatedAt:       formatTime(d.CreatedAt),
		StartedAt:       formatTimePtr(d.StartedAt),
		FinishedAt:      formatTimePtr(d.FinishedAt),
		PreDeployStatus: d.PreDeployStatus,
	}
	if d.Image != "" {
		out.Image = &renderDeployImage{Ref: d.Image}
	}
	if d.CommitID != "" {
		out.Commit = &renderCommit{ID: d.CommitID, Message: d.CommitMessage}
	}
	return out
}

// deployWithCursor is Render's deploy list-item envelope ({deploy, cursor}) —
// the same shape services/env-vars use. The deploy id is a stable, opaque
// cursor.
type deployWithCursor struct {
	Deploy renderDeploy `json:"deploy"`
	Cursor string       `json:"cursor"`
}

func toDeployList(deploys []DeployView) []deployWithCursor {
	out := make([]deployWithCursor, 0, len(deploys))
	for _, d := range deploys {
		out = append(out, deployWithCursor{Deploy: toRenderDeploy(d), Cursor: d.ID})
	}
	return out
}

// filterFromQuery translates Render's ListDeploysParams query params —
// status (repeatable), createdBefore/createdAfter (RFC3339), cursor, limit —
// into a ListFilter (w2/m31), over the one shared translator (FilterOf) the
// GraphQL and MCP fragments also use. Absent limit means the full history
// (the pre-m31 contract — a documented divergence from Render's default-20
// page, docs/ADR018-render-parity.md), which is why this parses limit itself
// instead of core.PageParams (whose absent-means-20 default and silent
// unparseable-means-default reading are both wrong here: a limit the caller
// spelled wrong must 400, not silently return the full history).
func filterFromQuery(q url.Values) (ListFilter, error) {
	limit := 0
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return ListFilter{}, fmt.Errorf("%w: limit must be a positive integer", core.ErrBadRequest)
		}
		limit = n
	}
	return FilterOf(q["status"], q.Get("createdBefore"), q.Get("createdAfter"), q.Get("cursor"), limit)
}

// RegisterREST adds the Render-shaped deploy-history endpoints. Store
// unconfigured => the Service returns core.ErrDeploysUnavailable => 503 on
// these routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		// Deploy-hook management is authenticated like every other service setting.
		// The URL itself is the credential, so prevent intermediary/browser caches
		// from retaining either a read or a newly rotated value.
		mux.HandleFunc("GET "+base+"/{id}/deploy-hook", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			hook, err := s.GetDeployHook(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, hook)
		})
		mux.HandleFunc("POST "+base+"/{id}/deploy-hook/regenerate", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			hook, err := s.RegenerateDeployHook(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, hook)
		})
		mux.HandleFunc("GET "+base+"/{id}/deploys", func(w http.ResponseWriter, r *http.Request) {
			filter, err := filterFromQuery(r.URL.Query())
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			deploys, err := s.List(r.Context(), r.PathValue("id"), filter)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toDeployList(deploys))
		})
		mux.HandleFunc("GET "+base+"/{id}/deploys/{deployId}", func(w http.ResponseWriter, r *http.Request) {
			d, err := s.Get(r.Context(), r.PathValue("id"), r.PathValue("deployId"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toRenderDeploy(d))
		})
		// Trigger (Render's POST .../deploys): decode the optional body fields
		// bex can honestly honor (commitId, deployMode). clearCache is rejected
		// when true — bex builds are always cache-free (ephemeral BuildKit Jobs,
		// no --cache-to/--cache-from), so accepting clearCache silently would be
		// a lie; callers must omit it or set it to false.
		mux.HandleFunc("POST "+base+"/{id}/deploys", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				CommitID   string `json:"commitId"`
				ClearCache bool   `json:"clearCache"`
				DeployMode string `json:"deployMode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
				return
			}
			if body.ClearCache {
				core.WriteErr(w, fmt.Errorf("%w: clearCache: bex builds are always cache-free "+
					"(ephemeral BuildKit Jobs with no persistent layer cache); "+
					"this field must be false or omitted", core.ErrBadRequest))
				return
			}
			d, err := s.Trigger(r.Context(), r.PathValue("id"), TriggerParams{
				CommitID:   body.CommitID,
				DeployMode: body.DeployMode,
			})
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, toRenderDeploy(d))
		})
		// Cancel (Render's POST .../deploys/{deployId}/cancel, w2/m10): past the
		// cancelable window this is a 409, never a silent no-op.
		mux.HandleFunc("POST "+base+"/{id}/deploys/{deployId}/cancel", func(w http.ResponseWriter, r *http.Request) {
			d, err := s.Cancel(r.Context(), r.PathValue("id"), r.PathValue("deployId"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toRenderDeploy(d))
		})
		// Rollback (Render's POST .../rollback {deployId}, w2/m10): a fresh
		// deploy restoring deployId's exact image, never a history rewrite.
		mux.HandleFunc("POST "+base+"/{id}/rollback", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				DeployID string `json:"deployId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
				return
			}
			d, err := s.Rollback(r.Context(), r.PathValue("id"), body.DeployID)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, toRenderDeploy(d))
		})
	}
}
