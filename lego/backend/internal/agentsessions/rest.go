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
	mux.HandleFunc("GET /v1/agent-sessions", func(w http.ResponseWriter, r *http.Request) {
		views, err := s.List(r.Context(), r.URL.Query().Get("ownerId"))
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
