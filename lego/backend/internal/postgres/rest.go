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
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

type renderOwner struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Type  string `json:"type"`
}

// renderPostgres is the REST-only Render enrichment. PostgresView remains the
// neutral GraphQL/MCP contract and retains bex's ownerId extension.
type renderPostgres struct {
	PostgresView
	Owner        *renderOwner `json:"owner,omitempty"`
	Region       string       `json:"region,omitempty"`
	DashboardURL string       `json:"dashboardUrl,omitempty"`
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
		rendered := renderPostgres{
			PostgresView: pg,
			Region:       s.Metadata.PlatformRegion(),
			DashboardURL: s.Metadata.DashboardURL("databases", pg.ID),
		}
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

// RegisterREST adds the managed-Postgres endpoints, Render-shaped (/v1/postgres)
// plus a bex-native /v1/databases alias.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	// verb maps a lifecycle action to a handler with a Render-accurate status code
	// (suspend/resume 202, restart 200 — same as services).
	verb := func(status int, fn func(context.Context, string) (PostgresView, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			pg, err := fn(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, status, s.renderOnePostgres(r.Context(), pg))
		}
	}
	for _, base := range []string{"/v1/postgres", "/v1/databases"} {
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
			if names := q["name"]; len(names) > 0 {
				filtered := make([]PostgresView, 0, len(out))
				for _, p := range out {
					if slices.Contains(names, p.Name) {
						filtered = append(filtered, p)
					}
				}
				out = filtered
			}
			// Render's cursor-pagination envelope (components.schemas.postgresWithCursor),
			// verified against the render-oss/cli generated client: a bare array of
			// Postgres objects breaks the official CLI's list decode.
			after, limit := core.PageParams(q)
			page := core.Page(out, after, limit, func(p PostgresView) string { return p.ID })
			core.WriteJSON(w, http.StatusOK, s.toPostgresList(r.Context(), page)) // [{postgres, cursor}, ...]
		})
		mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
			var req CreatePostgresRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			if !req.DryRun && r.URL.Query().Get("dryRun") == "true" {
				req.DryRun = true
			}
			pg, err := s.CreatePostgres(r.Context(), req)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			if req.DryRun {
				core.WriteJSON(w, http.StatusOK, s.renderOnePostgres(r.Context(), pg)) // dry-run: 200 (nothing created, w2/m29)
				return
			}
			core.WriteJSON(w, http.StatusCreated, s.renderOnePostgres(r.Context(), pg)) // Render: create => 201
		})
		mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			pg, err := s.GetPostgres(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, s.renderOnePostgres(r.Context(), pg))
		})
		mux.HandleFunc("PATCH "+base+"/{id}", s.handleUpdatePostgres)
		mux.HandleFunc("DELETE "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			if err := s.DeletePostgres(r.Context(), r.PathValue("id")); err != nil {
				core.WriteErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent) // Render: delete => 204
		})
		mux.HandleFunc("GET "+base+"/{id}/connection-info", func(w http.ResponseWriter, r *http.Request) {
			info, err := s.PostgresConnectionInfo(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, info)
		})
		mux.HandleFunc("POST "+base+"/{id}/query", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				SQL         string `json:"sql"`
				AllowWrites bool   `json:"allowWrites"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
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
		mux.HandleFunc("POST "+base+"/{id}/failover", func(w http.ResponseWriter, r *http.Request) {
			// Render: 202 Accepted, no response body.
			if err := s.Failover(r.Context(), r.PathValue("id")); err != nil {
				core.WriteErr(w, err)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		})

		// --- recovery / exports (Render: recovery-info, recover, export) ---
		recoveryInfo := func(w http.ResponseWriter, r *http.Request) {
			info, err := s.RecoveryInfo(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, info)
		}
		mux.HandleFunc("GET "+base+"/{id}/recovery-info", recoveryInfo)
		mux.HandleFunc("POST "+base+"/{id}/recovery-info", recoveryInfo) // Render uses POST; bex accepts both
		mux.HandleFunc("POST "+base+"/{id}/recover", func(w http.ResponseWriter, r *http.Request) {
			var req RecoverRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			pg, err := s.Recover(r.Context(), r.PathValue("id"), req)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, s.renderOnePostgres(r.Context(), pg)) // a new instance => 201
		})
		listExports := func(w http.ResponseWriter, r *http.Request) {
			out, err := s.ListExports(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		}
		createExport := func(w http.ResponseWriter, r *http.Request) {
			if _, err := s.CreateExport(r.Context(), r.PathValue("id")); err != nil {
				core.WriteErr(w, err)
				return
			}
			// Render returns 202 with no response body.
			w.WriteHeader(http.StatusAccepted)
		}
		mux.HandleFunc("GET "+base+"/{id}/export", listExports)
		mux.HandleFunc("POST "+base+"/{id}/export", createExport)
		// Keep the old plural spelling as a compatibility alias while generated
		// clients move to Render's singular /export path.
		mux.HandleFunc("GET "+base+"/{id}/exports", listExports)
		mux.HandleFunc("POST "+base+"/{id}/exports", func(w http.ResponseWriter, r *http.Request) {
			exp, err := s.CreateExport(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, exp)
		})

		// --- access: IP allowlist + users ---
		mux.HandleFunc("GET "+base+"/{id}/ip-allow-list", func(w http.ResponseWriter, r *http.Request) {
			list, err := s.GetIPAllowList(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, map[string][]string{"cidrs": list})
		})
		mux.HandleFunc("PUT "+base+"/{id}/ip-allow-list", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				CIDRs []string `json:"cidrs"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			pg, err := s.SetIPAllowList(r.Context(), r.PathValue("id"), req.CIDRs)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, s.renderOnePostgres(r.Context(), pg))
		})
		mux.HandleFunc("GET "+base+"/{id}/users", func(w http.ResponseWriter, r *http.Request) {
			users, err := s.ListUsers(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, users)
		})
		mux.HandleFunc("POST "+base+"/{id}/users", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			res, err := s.CreateUser(r.Context(), r.PathValue("id"), req.Name)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, res)
		})
		mux.HandleFunc("DELETE "+base+"/{id}/users/{user}", func(w http.ResponseWriter, r *http.Request) {
			if err := s.DeleteUser(r.Context(), r.PathValue("id"), r.PathValue("user")); err != nil {
				core.WriteErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// --- observability: processes / top-queries / sizes / table-scans / parameter-overrides ---
		mux.HandleFunc("GET "+base+"/{id}/processes", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.Processes(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("GET "+base+"/{id}/top-queries", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.TopQueries(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("GET "+base+"/{id}/sizes", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.Sizes(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("GET "+base+"/{id}/table-scans", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.TableScans(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("GET "+base+"/{id}/parameter-overrides", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.ParameterOverrides(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("PUT "+base+"/{id}/parameter-overrides", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Parameters map[string]string `json:"parameters"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			pg, err := s.SetParameterOverrides(r.Context(), r.PathValue("id"), req.Parameters)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, s.renderOnePostgres(r.Context(), pg))
		})
	}
}

// handleUpdatePostgres is PATCH /v1/postgres/{id} (+ /v1/databases alias) —
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
		Name                   *string             `json:"name,omitempty"`
		Plan                   *string             `json:"plan,omitempty"`
		Version                *string             `json:"version,omitempty"`
		DiskSizeGB             *int32              `json:"diskSizeGB,omitempty"`
		EnableHighAvailability *bool               `json:"enableHighAvailability,omitempty"`
		IPAllowList            *[]IPAllowListEntry `json:"ipAllowList,omitempty"`
		DryRun                 bool                `json:"dryRun,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
		return
	}
	id := r.PathValue("id")
	// bex uses the Database name as its id (docs/ADR020-identifiers.md
	// §Known deviations) — unlike Render's opaque dpg-... ids, renaming isn't
	// a field update, it's a k8s object rename. Reject cleanly instead of
	// silently ignoring the request or misreporting it as a plan error.
	if req.Name != nil && *req.Name != id {
		core.WriteErr(w, fmt.Errorf("%w: renaming a Postgres database isn't supported — bex uses the database name as its id (docs/ADR020-identifiers.md)", core.ErrBadRequest))
		return
	}
	patch := PostgresPatch{Plan: req.Plan, Version: req.Version, DiskSizeGB: req.DiskSizeGB, EnableHighAvailability: req.EnableHighAvailability}
	if req.IPAllowList != nil {
		cidrs := ipAllowListFromWire(*req.IPAllowList)
		patch.IPAllowList = &cidrs
	}
	dryRun := req.DryRun || r.URL.Query().Get("dryRun") == "true"
	if dryRun {
		pg, err := s.PreviewUpdatePostgres(r.Context(), id, patch)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, s.renderOnePostgres(r.Context(), pg))
		return
	}
	pg, err := s.UpdatePostgres(r.Context(), id, patch)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, s.renderOnePostgres(r.Context(), pg))
}
