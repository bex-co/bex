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

package jobs

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the one-off jobs REST fragment: Render's
//
//	GET  /v1/services/{id}/jobs                  → list jobs (array of {job, cursor})
//	POST /v1/services/{id}/jobs                  → create job (job 201)
//	GET  /v1/services/{id}/jobs/{jobId}          → get job (job 200)
//	POST /v1/services/{id}/jobs/{jobId}/cancel   → cancel job (job 200)
//
// Behavior lives in the Service so GraphQL and MCP stay identical.

// renderJobStatus maps Render's job status values — the same set the CLI's
// types_gen.go declares (pending/running/succeeded/failed/canceled).
type renderJob struct {
	ID           string     `json:"id"`
	ServiceID    string     `json:"serviceId"`
	StartCommand string     `json:"startCommand"`
	PlanID       string     `json:"planId"`
	Status       *string    `json:"status,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
}

// jobWithCursor wraps a job for the list response — Render's {job, cursor}
// envelope (matching how services/deploys/postgres already wrap their lists).
type jobWithCursor struct {
	Job    renderJob `json:"job"`
	Cursor string    `json:"cursor"`
}

func toRenderJob(v JobView) renderJob {
	rj := renderJob{
		ID:           v.ID,
		ServiceID:    v.ServiceID,
		StartCommand: v.StartCommand,
		PlanID:       v.PlanID,
		CreatedAt:    v.CreatedAt,
		StartedAt:    v.StartedAt,
		FinishedAt:   v.FinishedAt,
	}
	if v.Status != "" {
		rj.Status = &v.Status
	}
	return rj
}

func toJobList(views []JobView) []jobWithCursor {
	out := make([]jobWithCursor, 0, len(views))
	for _, v := range views {
		out = append(out, jobWithCursor{Job: toRenderJob(v), Cursor: v.ID})
	}
	return out
}

// filterFromQuery translates Render's ListJobParams query params into a
// ListFilter. Absent limit means the full history (the bex default — Render
// defaults to 20 but bex's explicit design choice is unbounded without a
// stated limit, same as deploys).
func filterFromQuery(q http.Header, vals map[string][]string) (ListFilter, error) {
	_ = q
	limit := 0
	if v := vals["limit"]; len(v) > 0 {
		n, err := strconv.Atoi(v[0])
		if err != nil || n < 1 {
			return ListFilter{}, core.ErrBadRequest
		}
		limit = n
	}
	str := func(key string) string {
		if v := vals[key]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	return FilterFromStrings(
		vals["status"],
		str("createdBefore"), str("createdAfter"),
		str("startedBefore"), str("startedAfter"),
		str("finishedBefore"), str("finishedAfter"),
		str("cursor"), limit,
	)
}

// RegisterREST adds the Render-shaped one-off jobs endpoints to the mux.
// Store unconfigured ⇒ the Service returns ErrJobsUnavailable ⇒ 503.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		mux.HandleFunc("GET "+base+"/{id}/jobs", func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			vals := map[string][]string(q)
			filter, err := filterFromQuery(r.Header, vals)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			jobs, err := s.List(r.Context(), r.PathValue("id"), filter)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toJobList(jobs))
		})

		mux.HandleFunc("POST "+base+"/{id}/jobs", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				StartCommand string `json:"startCommand"`
				PlanID       string `json:"planId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				core.WriteErr(w, core.ErrBadRequest)
				return
			}
			if body.StartCommand == "" {
				core.WriteErr(w, core.ErrBadRequest)
				return
			}
			j, err := s.Create(r.Context(), r.PathValue("id"), body.StartCommand, body.PlanID)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, toRenderJob(j))
		})

		mux.HandleFunc("GET "+base+"/{id}/jobs/{jobId}", func(w http.ResponseWriter, r *http.Request) {
			j, err := s.Get(r.Context(), r.PathValue("id"), r.PathValue("jobId"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toRenderJob(j))
		})

		mux.HandleFunc("POST "+base+"/{id}/jobs/{jobId}/cancel", func(w http.ResponseWriter, r *http.Request) {
			j, err := s.Cancel(r.Context(), r.PathValue("id"), r.PathValue("jobId"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toRenderJob(j))
		})
	}
}
