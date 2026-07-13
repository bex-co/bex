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
	"fmt"
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the deploy-history REST fragment (w2/m5's list/get/trigger,
// w2/m10's cancel/rollback): Render's GET .../deploys (the {deploy, cursor}
// list envelope), GET .../deploys/{id}, POST .../deploys (trigger),
// POST .../deploys/{id}/cancel, and POST .../rollback. Served under both
// /v1/services and /v1/apps, same as every other apps-adjacent route.
// Behavior lives in the Service, so GraphQL and MCP stay identical.

// renderDeploy mirrors Render's deploy object for the fields bex can honor
// (id, status, timestamps) plus bex-native extras (trigger, image,
// rollbackOf). Render nests commit/image under richer objects bex doesn't
// model yet — omitted, not faked, until w1/m5 tracks build-from-git commits.
type renderDeploy struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Trigger    string `json:"trigger,omitempty"`    // bex extra: "create" | "api" | "rollback"
	Image      string `json:"image,omitempty"`      // bex extra
	RollbackOf string `json:"rollbackOf,omitempty"` // bex extra (w2/m10): the deploy this one restores, if any
	CreatedAt  string `json:"createdAt,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
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
	return renderDeploy{
		ID:         d.ID,
		Status:     d.Status,
		Trigger:    d.Trigger,
		Image:      d.Image,
		RollbackOf: d.RollbackOf,
		CreatedAt:  formatTime(d.CreatedAt),
		StartedAt:  formatTimePtr(d.StartedAt),
		FinishedAt: formatTimePtr(d.FinishedAt),
	}
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

// RegisterREST adds the Render-shaped deploy-history endpoints. Store
// unconfigured => the Service returns core.ErrDeploysUnavailable => 503 on
// these routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		mux.HandleFunc("GET "+base+"/{id}/deploys", func(w http.ResponseWriter, r *http.Request) {
			deploys, err := s.List(r.Context(), r.PathValue("id"))
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
		// Trigger (Render's POST .../deploys): the body may carry clearCache —
		// accepted and ignored (bex has no build cache to clear), the safe-
		// superset rule, so it's never even decoded.
		mux.HandleFunc("POST "+base+"/{id}/deploys", func(w http.ResponseWriter, r *http.Request) {
			d, err := s.Trigger(r.Context(), r.PathValue("id"))
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
