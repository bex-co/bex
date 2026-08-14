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

package postgres

import (
	"context"
	"net/http"
	"slices"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

type renderOwner struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Type  string `json:"type"`
}

// renderPostgres is the REST-only Render enrichment wrapping PostgresView
// (which now carries region/dashboardUrl) with the owner object.
type renderPostgres struct {
	PostgresView
	Owner *renderOwner `json:"owner,omitempty"`
}

// postgresWithCursor is components.schemas.postgresWithCursor — the list-item
// envelope GET /v1/postgres returns as a bare JSON array (cursor is a SIBLING
// of the postgres object, not a wrapper member; the same shape
// serviceWithCursor/ownerWithCursor use), verified against the render-oss/cli
// generated client.
type postgresWithCursor struct {
	Postgres renderPostgres `json:"postgres"`
	Cursor   string         `json:"cursor"`
}

func (s *Service) renderPostgres(ctx context.Context, pgs []PostgresView) []renderPostgres {
	ownerIDs := make([]string, 0, len(pgs))
	for _, pg := range pgs {
		ownerIDs = append(ownerIDs, pg.OwnerID)
	}
	owners := resourcemeta.ResolveOwners(ctx, s.Owners, ownerIDs)
	out := make([]renderPostgres, 0, len(pgs))
	for _, pg := range pgs {
		rendered := renderPostgres{PostgresView: pg}
		if owner, ok := owners[pg.OwnerID]; ok && owner.Available() {
			rendered.Owner = &renderOwner{ID: owner.ID, Name: owner.Name, Email: owner.Email, Type: owner.Type}
		}
		out = append(out, rendered)
	}
	return out
}

func (s *Service) renderOnePostgres(ctx context.Context, pg PostgresView) renderPostgres {
	return s.renderPostgres(ctx, []PostgresView{pg})[0]
}

func (s *Service) toPostgresList(ctx context.Context, pgs []PostgresView) []postgresWithCursor {
	rendered := s.renderPostgres(ctx, pgs)
	out := make([]postgresWithCursor, 0, len(pgs))
	for i, p := range pgs {
		// cursor is opaque in Render; the Database name/id is a stable, valid cursor.
		out = append(out, postgresWithCursor{Postgres: rendered[i], Cursor: p.ID})
	}
	return out
}

// decodeOr400 decodes the JSON body into dst, answering the flat 400 this
// surface uses on a malformed body; reports whether decoding succeeded.
func decodeOr400(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := core.DecodeJSON(r, dst); err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, "bad request body")
		return false
	}
	return true
}

// respondPostgres writes the Render-enriched view with the given status, or
// the mapped error — the shared tail of every verb that answers a PostgresView.
func (s *Service) respondPostgres(w http.ResponseWriter, r *http.Request, status int, pg PostgresView, err error) {
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	core.WriteJSON(w, status, s.renderOnePostgres(r.Context(), pg))
}

// RegisterREST adds the managed-Postgres endpoints, Render-shaped (/v1/postgres)
// using Render's canonical /v1/postgres noun.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	// verb maps a lifecycle action to a handler with a Render-accurate status code
	// (suspend/resume 202, restart 200 — same as services).
	verb := func(status int, fn func(context.Context, string) (PostgresView, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := core.WithConfirm(r.Context(), r.URL.Query().Get("confirm"))
			pg, err := fn(ctx, r.PathValue("id"))
			s.respondPostgres(w, r, status, pg, err)
		}
	}
	const base = "/v1/postgres"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		out, err := s.ListPostgres(r.Context(), q.Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		// name filters by exact name, OR'd across repeated ?name= values (Render's
		// documented "Filter by name" — the official CLI resolves a bare
		// name/id argument to a database id by calling this with ?name=, and
		// requires it to narrow to exactly one match).
		names := core.QueryList(q, "name")
		envIDs := core.QueryList(q, "environmentId")
		// suspended= filters by Render's string enum (w2/m53). Unknown value → 400.
		suspended, err := core.ParseEnum("suspended", q.Get("suspended"), core.RenderSuspended, core.RenderNotSuspended)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		// Time-window filters (w2/m53): RFC3339, named 400 on malformed, empty
		// timestamp passes (legacy DBs without stored timestamps are never excluded).
		created, err := core.QueryTimeWindow(q, "createdBefore", "createdAfter")
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		updated, err := core.QueryTimeWindow(q, "updatedBefore", "updatedAfter")
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		out = core.Filter(out, func(p PostgresView) bool {
			return (len(names) == 0 || slices.Contains(names, p.Name) || slices.Contains(names, p.ID)) &&
				(len(envIDs) == 0 || slices.Contains(envIDs, p.EnvironmentID)) &&
				(suspended == "" || p.Suspended == suspended) &&
				created.Contains(p.CreatedAt) && updated.Contains(p.UpdatedAt)
		})
		// Render's cursor-pagination envelope (components.schemas.postgresWithCursor),
		// verified against the render-oss/cli generated client: a bare array of
		// Postgres objects breaks the official CLI's list decode. Omission of both
		// paging params preserves bex's original complete-list behavior; once either
		// is present the stable id order makes a full walk gap/duplicate-free.
		after, limit := core.PageParams(q)
		page := core.StablePage(out, after, limit, q.Has("cursor") || q.Has("limit"), func(p PostgresView) string { return p.ID })
		core.WriteJSON(w, http.StatusOK, s.toPostgresList(r.Context(), page)) // [{postgres, cursor}, ...]
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		var req CreatePostgresRequest
		if !decodeOr400(w, r, &req) {
			return
		}
		req.DryRun = core.DryRunRequested(r, req.DryRun)
		pg, err := s.CreatePostgres(r.Context(), req)
		status := http.StatusCreated // Render: create => 201
		if req.DryRun {
			status = http.StatusOK // dry-run: 200 (nothing created, w2/m29)
		}
		s.respondPostgres(w, r, status, pg, err)
	})
	mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		pg, err := s.GetPostgres(r.Context(), r.PathValue("id"))
		s.respondPostgres(w, r, http.StatusOK, pg, err)
	})
	mux.HandleFunc("PATCH "+base+"/{id}", s.handleUpdatePostgres)
	mux.HandleFunc("DELETE "+base+"/{id}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		ctx := core.WithConfirm(r.Context(), r.URL.Query().Get("confirm"))
		return s.DeletePostgres(ctx, r.PathValue("id")) // Render: delete => 204
	}))
	mux.HandleFunc("GET "+base+"/{id}/connection-info", core.HandleByID(s.PostgresConnectionInfo))
	mux.HandleFunc("POST "+base+"/{id}/query", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SQL         string `json:"sql"`
			AllowWrites bool   `json:"allowWrites"`
		}
		if !decodeOr400(w, r, &req) {
			return
		}
		result, err := s.ExecuteQuery(r.Context(), r.PathValue("id"), req.SQL, req.AllowWrites)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, result)
	})

	// --- lifecycle (Render: POST /postgres/{id}/{suspend,resume,restart,failover}) ---
	mux.HandleFunc("POST "+base+"/{id}/suspend", verb(http.StatusAccepted, s.Suspend))
	mux.HandleFunc("POST "+base+"/{id}/resume", verb(http.StatusAccepted, s.Resume))
	mux.HandleFunc("POST "+base+"/{id}/restart", verb(http.StatusOK, s.Restart))
	// Render: failover => 202 Accepted, no response body.
	mux.HandleFunc("POST "+base+"/{id}/failover", core.HandleNoBody(http.StatusAccepted, func(r *http.Request) error {
		return s.Failover(r.Context(), r.PathValue("id"))
	}))

	// --- recovery / exports (Render: recovery-info, recover, export) ---
	mux.HandleFunc("POST "+base+"/{id}/recovery-info", core.HandleByID(s.RecoveryInfo))
	mux.HandleFunc("POST "+base+"/{id}/recover", func(w http.ResponseWriter, r *http.Request) {
		var req RecoverRequest
		if !decodeOr400(w, r, &req) {
			return
		}
		pg, err := s.Recover(r.Context(), r.PathValue("id"), req)
		s.respondPostgres(w, r, http.StatusCreated, pg, err) // a new instance => 201
	})
	mux.HandleFunc("GET "+base+"/{id}/export", core.HandleByID(s.ListExports))
	// Render: export => 202 with no response body.
	mux.HandleFunc("POST "+base+"/{id}/export", core.HandleNoBody(http.StatusAccepted, func(r *http.Request) error {
		_, err := s.CreateExport(r.Context(), r.PathValue("id"))
		return err
	}))

	// --- access: IP allowlist + users ---
	// The bex-native GET/PUT pair keeps its plain {"cidrs": [...]} response
	// shape (nothing Render-side depends on this endpoint); descriptions
	// travel through the Render-shaped create/PATCH/get/list. The PUT body's
	// array elements remain the route's documented string list. Structured
	// entries and descriptions travel through Render's create/PATCH/get/list.
	mux.HandleFunc("GET "+base+"/{id}/ip-allow-list", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		list, err := s.GetIPAllowList(r.Context(), r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return map[string][]string{"cidrs": core.AllowListCIDRs(list)}, nil
	}))
	mux.HandleFunc("PUT "+base+"/{id}/ip-allow-list", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CIDRs []string `json:"cidrs"`
		}
		if !decodeOr400(w, r, &req) {
			return
		}
		pg, err := s.SetIPAllowList(r.Context(), r.PathValue("id"), core.AllowListFromCIDRs(req.CIDRs))
		s.respondPostgres(w, r, http.StatusOK, pg, err)
	})
	mux.HandleFunc("GET "+base+"/{id}/users", core.HandleByID(s.ListUsers))
	mux.HandleFunc("POST "+base+"/{id}/users", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if !decodeOr400(w, r, &req) {
			return
		}
		res, err := s.CreateUser(r.Context(), r.PathValue("id"), req.Name)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, res)
	})
	mux.HandleFunc("DELETE "+base+"/{id}/users/{user}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteUser(r.Context(), r.PathValue("id"), r.PathValue("user"))
	}))

	// --- logs (w3/m28) ---
	mux.HandleFunc("GET "+base+"/{id}/logs", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		limit, _ := strconv.ParseInt(q.Get("limit"), 10, 64)
		since, end, err := core.ParseTimeWindow(q.Get("startTime"), q.Get("endTime"))
		if err != nil {
			return nil, err
		}
		return s.QueryDatabaseLogs(r.Context(), r.PathValue("id"), DatabaseLogQuery{
			Search:    q.Get("text"),
			Since:     since,
			End:       end,
			Limit:     limit,
			Direction: q.Get("direction"),
			Instance:  q["instance"],
		})
	}))

	// --- observability: processes / top-queries / sizes / table-scans / parameter-overrides ---
	mux.HandleFunc("GET "+base+"/{id}/processes", core.HandleByID(s.Processes))
	mux.HandleFunc("GET "+base+"/{id}/top-queries", core.HandleByID(s.TopQueries))
	mux.HandleFunc("GET "+base+"/{id}/sizes", core.HandleByID(s.Sizes))
	mux.HandleFunc("GET "+base+"/{id}/table-scans", core.HandleByID(s.TableScans))
	mux.HandleFunc("GET "+base+"/{id}/parameter-overrides", core.HandleByID(s.ParameterOverrides))
	mux.HandleFunc("PUT "+base+"/{id}/parameter-overrides", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Parameters map[string]string `json:"parameters"`
		}
		if !decodeOr400(w, r, &req) {
			return
		}
		pg, err := s.SetParameterOverrides(r.Context(), r.PathValue("id"), req.Parameters)
		s.respondPostgres(w, r, http.StatusOK, pg, err)
	})
}

// handleUpdatePostgres is PATCH /v1/postgres/{id} —
// pulled out of RegisterREST to keep that registration function's own
// complexity down. Render's PostgresPATCHInput (verified against the
// render-oss/cli generated client, cli/pkg/client/types_gen.go): every field
// is a pointer — omitted means "leave unchanged", NOT "clear" or "zero
// value". A prior version of this handler decoded Plan as a plain string and
// called SetPlan unconditionally, so any update that didn't touch plan (a
// rename, a disk resize, an HA toggle, an ip-allow-list replace) 400'd with
// "plan must be one of ..." — found running `render postgres update <name>
// --name <new>` (and --disk-size-gb / --high-availability / --ip-allow-list)
// against a live bex-api: every one of those failed end to end.
func (s *Service) handleUpdatePostgres(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                   *string                  `json:"name,omitempty"`
		DatadogAPIKey          *string                  `json:"datadogAPIKey,omitempty"`
		DatadogSite            *string                  `json:"datadogSite,omitempty"`
		Plan                   *string                  `json:"plan,omitempty"`
		Version                *string                  `json:"version,omitempty"`
		DiskSizeGB             *int32                   `json:"diskSizeGB,omitempty"`
		EnableDiskAutoscaling  *bool                    `json:"enableDiskAutoscaling,omitempty"`
		EnableHighAvailability *bool                    `json:"enableHighAvailability,omitempty"`
		IPAllowList            *[]core.IPAllowListEntry `json:"ipAllowList,omitempty"`
		ParameterOverrides     *map[string]string       `json:"parameterOverrides,omitempty"`
		DryRun                 bool                     `json:"dryRun,omitempty"`
	}
	if !decodeOr400(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	patch := PostgresPatch{
		Name: req.Name, DatadogAPIKey: req.DatadogAPIKey, DatadogSite: req.DatadogSite,
		Plan: req.Plan, Version: req.Version, DiskSizeGB: req.DiskSizeGB,
		EnableDiskAutoscaling: req.EnableDiskAutoscaling, EnableHighAvailability: req.EnableHighAvailability,
		IPAllowList:        req.IPAllowList,
		ParameterOverrides: req.ParameterOverrides,
	}
	if req.DryRun || r.URL.Query().Get("dryRun") == "true" {
		pg, err := s.PreviewUpdatePostgres(r.Context(), id, patch)
		s.respondPostgres(w, r, http.StatusOK, pg, err)
		return
	}
	pg, err := s.UpdatePostgres(r.Context(), id, patch)
	s.respondPostgres(w, r, http.StatusOK, pg, err)
}
