package agentsessions

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/agent-sessions", func(w http.ResponseWriter, r *http.Request) {
		var req CreateRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		if owner := r.URL.Query().Get("ownerId"); owner != "" {
			req.OwnerID = owner
		}
		view, err := s.Create(r.Context(), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, view)
	})
	// The list is filtered + keyset-paginated (ADR065 D3): the default answer is
	// one page of the unarchived working set; `cursor` is the prior page's last
	// item id, and a shorter/empty page signals the end (Render's idiom). The
	// response stays a bare View array — pagination adds params, not an envelope.
	mux.HandleFunc("GET /v1/agent-sessions", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		window, err := core.QueryTimeWindow(q, "createdBefore", "createdAfter")
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		limit, err := core.QueryLimit(q)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		views, err := s.List(r.Context(), ListRequest{
			OwnerID:       q.Get("ownerId"),
			Archived:      q.Get("archived"),
			Phases:        core.QueryList(q, "phase"),
			Repo:          q.Get("repo"),
			CreatedBefore: window.Before,
			CreatedAfter:  window.After,
			Cursor:        q.Get("cursor"),
			Limit:         limit,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, views)
	})
	// Literal "capabilities" segment is more specific than "{id}" in net/http's
	// mux, so it is matched before the session-by-id route (session ids are
	// ags-<xid>, never the literal "capabilities").
	mux.HandleFunc("GET /v1/agent-sessions/capabilities", func(w http.ResponseWriter, r *http.Request) {
		caps, err := s.Capabilities(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, caps)
	})
	mux.HandleFunc("GET /v1/agent-sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		view, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, view)
	})
	mux.HandleFunc("POST /v1/agent-sessions/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		view, err := s.Cancel(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, view)
	})
	mux.HandleFunc("POST /v1/agent-sessions/{id}/archive", core.HandleByID(s.Archive))
	mux.HandleFunc("POST /v1/agent-sessions/{id}/unarchive", core.HandleByID(s.Unarchive))
	mux.HandleFunc("DELETE /v1/agent-sessions/{id}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.Delete(r.Context(), r.PathValue("id"))
	}))
	mux.HandleFunc("GET /v1/agent-sessions/{id}/transcript", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// Absent means -1 (the whole transcript); an explicit 0 is a real cursor
		// position, so presence is checked before the shared int64 parse + 400.
		afterSeq := int64(-1)
		if q.Get("afterSeq") != "" {
			n, err := core.QueryLimit64(q, "afterSeq")
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			afterSeq = n
		}
		limit, err := core.QueryLimit(q)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		page, err := s.Transcript(r.Context(), r.PathValue("id"), afterSeq, limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, page)
	})
	mux.HandleFunc("POST /v1/agent-sessions/{id}/pin", func(w http.ResponseWriter, r *http.Request) {
		view, err := s.Pin(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, view)
	})
	mux.HandleFunc("POST /v1/agent-sessions/{id}/unpin", func(w http.ResponseWriter, r *http.Request) {
		view, err := s.Unpin(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, view)
	})
	mux.HandleFunc("POST /v1/agent-sessions/{id}/attach-ticket", func(w http.ResponseWriter, r *http.Request) {
		// Determine action from query parameter or default to "read"
		action := r.URL.Query().Get("action")
		if action == "" {
			action = agentsessionticket.ActionRead // Default to read for safety
		}
		view, err := s.AttachTicket(r.Context(), r.PathValue("id"), action)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, view)
	})
	mux.HandleFunc("POST /v1/agent-sessions/{id}/steer", func(w http.ResponseWriter, r *http.Request) {
		var req SteerRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		req.SessionID = r.PathValue("id")
		view, err := s.Steer(r.Context(), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, view)
	})
}
