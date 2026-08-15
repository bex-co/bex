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
	"net/http"
	"net/url"
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
// ListFilter. Absent limit stays 0 and the store bounds it at
// core.MaxPageLimit (codex-security round-6 #7 — Render defaults to 20; bex
// returns the larger newest-first cap and pages with the cursor, same as
// deploys).
func filterFromQuery(q url.Values) (ListFilter, error) {
	limit := 0
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return ListFilter{}, core.ErrBadRequest
		}
		limit = n
	}
	return FilterFromStrings(
		q["status"],
		q.Get("createdBefore"), q.Get("createdAfter"),
		q.Get("startedBefore"), q.Get("startedAfter"),
		q.Get("finishedBefore"), q.Get("finishedAfter"),
		q.Get("cursor"), limit,
	)
}

// RegisterREST adds the Render-shaped one-off jobs endpoints to the mux.
// Store unconfigured ⇒ the Service returns ErrJobsUnavailable ⇒ 503.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	const base = "/v1/services"
	mux.HandleFunc("GET "+base+"/{id}/jobs", func(w http.ResponseWriter, r *http.Request) {
		filter, err := filterFromQuery(r.URL.Query())
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
		if err := core.DecodeJSON(r, &body); err != nil {
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
