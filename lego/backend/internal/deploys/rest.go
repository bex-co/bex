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
// list envelope, honoring status, created/updated/finished time bounds,
// cursor, and limit), GET .../deploys/{id}, POST .../deploys (trigger),
// POST .../deploys/{id}/cancel, and POST .../rollback. Served under both
// /v1/services, the canonical Render service route.
// Behavior lives in the Service, so GraphQL and MCP stay identical.

// renderCommit is Render's nested deploy.commit object — the resolved commit
// a build-from-git deploy ran (w9/001 + w2/m42). createdAt is the git author
// timestamp captured at deploy-open time; nil when unavailable (omitted, not
// faked).
type renderCommit struct {
	ID        string  `json:"id"`
	Message   string  `json:"message,omitempty"`
	CreatedAt *string `json:"createdAt,omitempty"`
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
	ServiceID  string             `json:"serviceId,omitempty"`
	Status     string             `json:"status"`
	Trigger    string             `json:"trigger,omitempty"`    // bex extra: "create" | "api" | "deploy_hook" | "rollback"
	Image      *renderDeployImage `json:"image,omitempty"`      // Render's nested image object, not a bare string
	RollbackOf string             `json:"rollbackOf,omitempty"` // bex extra (w2/m10): the deploy this one restores, if any
	Commit     *renderCommit      `json:"commit,omitempty"`
	CreatedAt  string             `json:"createdAt,omitempty"`
	UpdatedAt  string             `json:"updatedAt,omitempty"`
	StartedAt  string             `json:"startedAt,omitempty"`
	FinishedAt string             `json:"finishedAt,omitempty"`
	// PreDeployStatus is the pre-deploy command's outcome (bex extra, w1/m33):
	// "running" | "succeeded" | "failed"; omitted when no pre-deploy step ran.
	// Distinguishes a migration failure from a health-check failure (both
	// status=update_failed). Logs: GET /v1/logs?service=<id>&type=predeploy.
	PreDeployStatus string `json:"preDeployStatus,omitempty"`
	// FailureReason is the actionable cause of a failed deploy (bex extra,
	// w9/011): the operator's diagnosis (crash loop with the $PORT hint,
	// image-pull failure, an unresolvable Secret/ConfigMap reference naming the
	// missing object — w7/m79 — or a build error) or a health-gate-timeout line.
	// Omitted unless the deploy failed.
	FailureReason string `json:"failureReason,omitempty"`
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// Preserve sub-second transition order. The store guarantees updated_at is
	// monotonic even when multiple facts land in one wall-clock second; dropping
	// fractional precision here would make those real advances look identical.
	return t.UTC().Format(time.RFC3339Nano)
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
		ServiceID:       d.ServiceID,
		Status:          d.Status,
		Trigger:         d.Trigger,
		RollbackOf:      d.RollbackOf,
		CreatedAt:       formatTime(d.CreatedAt),
		UpdatedAt:       formatTime(d.UpdatedAt),
		StartedAt:       formatTimePtr(d.StartedAt),
		FinishedAt:      formatTimePtr(d.FinishedAt),
		PreDeployStatus: d.PreDeployStatus,
		FailureReason:   d.FailureReason,
	}
	if d.Image != "" {
		out.Image = &renderDeployImage{Ref: d.Image}
	}
	if d.CommitID != "" {
		rc := &renderCommit{ID: d.CommitID, Message: d.CommitMessage}
		if d.CommitAuthorAt != nil {
			s := formatTime(*d.CommitAuthorAt)
			rc.CreatedAt = &s
		}
		out.Commit = rc
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

// filterFromQuery translates Render's ListDeploysParams query params — status
// (repeatable), created/updated/finished before/after bounds (RFC3339), cursor,
// and limit —
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
	return FilterOf(
		q["status"],
		q.Get("createdBefore"), q.Get("createdAfter"),
		q.Get("updatedBefore"), q.Get("updatedAfter"),
		q.Get("finishedBefore"), q.Get("finishedAfter"),
		q.Get("cursor"), limit,
	)
}

// RegisterREST adds the Render-shaped deploy-history endpoints. Store
// unconfigured => the Service returns core.ErrDeploysUnavailable => 503 on
// these routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	const base = "/v1/services"
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
	// bex can honestly honor (commitId, deployMode). clearCache is Render's
	// string enum "clear" | "do_not_clear" (cli/pkg/client/types_gen.go's
	// CreateDeployJSONBodyClearCache) — NOT a bool; the official CLI always
	// sends it explicitly (defaulting to "do_not_clear" absent --clear-cache),
	// so a bool-typed field here 400s every deploys-create call the CLI
	// makes. bex builds are always cache-free (ephemeral BuildKit Jobs, no
	// --cache-to/--cache-from) — "clear" and "do_not_clear" are both
	// already-true no-ops, so any recognized value (or an omitted one) is
	// accepted rather than rejected; only a value outside the enum 400s.
	mux.HandleFunc("POST "+base+"/{id}/deploys", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			CommitID   string `json:"commitId"`
			ClearCache string `json:"clearCache"`
			DeployMode string `json:"deployMode"`
			ImageURL   string `json:"imageUrl"`
		}
		if err := core.DecodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
			core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
			return
		}
		if body.ClearCache != "" && body.ClearCache != "clear" && body.ClearCache != "do_not_clear" {
			core.WriteErr(w, fmt.Errorf("%w: unknown clearCache %q (valid: clear, do_not_clear)",
				core.ErrBadRequest, body.ClearCache))
			return
		}
		d, err := s.Trigger(r.Context(), r.PathValue("id"), TriggerParams{
			CommitID:   body.CommitID,
			DeployMode: body.DeployMode,
			ImageURL:   body.ImageURL,
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
		if err := core.DecodeJSON(r, &body); err != nil {
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
