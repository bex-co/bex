package agentsessions

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

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
		// ADR094 D4: the header is the REST spelling of the body's idempotencyKey.
		if key := r.Header.Get("Idempotency-Key"); key != "" {
			if req.IdempotencyKey != "" && req.IdempotencyKey != key {
				core.WriteErrStatus(w, http.StatusBadRequest, "Idempotency-Key header and idempotencyKey body field disagree")
				return
			}
			req.IdempotencyKey = key
		}
		view, err := s.Create(r.Context(), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		status := http.StatusCreated
		if view.Created != nil && !*view.Created {
			status = http.StatusOK // an idempotent replay: the session already existed
		}
		core.WriteJSON(w, status, view)
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
		mine := false
		if raw := q.Get("mine"); raw != "" {
			if mine, err = strconv.ParseBool(raw); err != nil {
				core.WriteErr(w, core.NewBadRequestError("AGENT_SESSION_FILTER_INVALID",
					"mine must be true or false", map[string]any{"mine": raw}))
				return
			}
		}
		views, err := s.List(r.Context(), ListRequest{
			OwnerID:         q.Get("ownerId"),
			Archived:        q.Get("archived"),
			Phases:          core.QueryList(q, "phase"),
			States:          core.QueryList(q, "state"),
			Tag:             q.Get("tag"),
			Repo:            q.Get("repo"),
			AutomationID:    q.Get("automationId"),
			ParentSessionID: q.Get("parentSessionId"),
			Mine:            mine,
			CreatedBy:       core.QueryList(q, "createdBy"),
			PRStates:        core.QueryList(q, "prState"),
			CreatedBefore:   window.Before,
			CreatedAfter:    window.After,
			Cursor:          q.Get("cursor"),
			Limit:           limit,
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
	// The workspace agent environment (w10/m17): one per workspace, so it is
	// addressed by ownerId rather than an id path segment.
	mux.HandleFunc("GET /v1/agent-environment", func(w http.ResponseWriter, r *http.Request) {
		env, err := s.Environment(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, env)
	})
	mux.HandleFunc("PATCH /v1/agent-environment", func(w http.ResponseWriter, r *http.Request) {
		var input EnvironmentInput
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		raw, err := io.ReadAll(io.LimitReader(r.Body, maxEnvironmentBody+1))
		if err != nil || len(raw) > maxEnvironmentBody {
			core.WriteErrStatus(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		if err := core.DecodeJSON(r, &input); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		input.markSpendCapPresence(raw)
		env, err := s.UpdateEnvironment(r.Context(), r.URL.Query().Get("ownerId"), input)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, env)
	})
	mux.HandleFunc("GET /v1/agent-sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		view, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, view)
	})
	// The tree the session belongs to (ADR102 D8), whichever node the id names.
	mux.HandleFunc("GET /v1/agent-sessions/{id}/tree", core.HandleByID(s.Tree))
	mux.HandleFunc("GET /v1/agent-sessions/{id}/model-cost", core.HandleByID(s.ModelCost))
	mux.HandleFunc("POST /v1/agent-sessions/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		view, err := s.Cancel(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, view)
	})
	mux.HandleFunc("PUT /v1/agent-sessions/{id}/tags", func(w http.ResponseWriter, r *http.Request) {
		var req replaceTagsRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Tags == nil {
			core.WriteErr(w, errTagsRequired())
			return
		}
		view, err := s.ReplaceTags(r.Context(), r.PathValue("id"), *req.Tags)
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
	mux.HandleFunc("GET /v1/agent-sessions/{id}/tool-outputs/{toolCallId}", func(w http.ResponseWriter, r *http.Request) {
		turn, err := strconv.Atoi(r.URL.Query().Get("turn"))
		if err != nil {
			core.WriteErr(w, core.NewBadRequestError("AGENT_SESSION_TURN_INVALID", "turn query parameter must be a non-negative integer", nil))
			return
		}
		out, err := s.ToolOutput(r.Context(), r.PathValue("id"), turn, r.PathValue("toolCallId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
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
	s.registerAttachmentREST(mux)
	s.registerSecretREST(mux)
	s.registerSkillREST(mux)
	s.registerAutomationREST(mux)
}

// registerSkillREST mounts the ADR099 D3 workspace skills. {ref} is a skl-
// id (its workspace comes from the row) or a name in ownerId's workspace.
func (s *Service) registerSkillREST(mux *http.ServeMux) {
	decode := func(w http.ResponseWriter, r *http.Request) (AgentSkillInput, bool) {
		var in AgentSkillInput
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &in); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return in, false
		}
		return in, true
	}
	mux.HandleFunc("GET /v1/agent-skills", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.AgentSkills(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /v1/agent-skills", func(w http.ResponseWriter, r *http.Request) {
		in, ok := decode(w, r)
		if !ok {
			return
		}
		out, err := s.CreateAgentSkill(r.Context(), r.URL.Query().Get("ownerId"), in)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, out)
	})
	mux.HandleFunc("GET /v1/agent-skills/{ref}", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.AgentSkill(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("ref"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("GET /v1/agent-skills/{ref}/versions", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		limit, err := core.QueryLimit(q)
		if err != nil {
			return nil, err
		}
		return s.AgentSkillVersions(r.Context(), q.Get("ownerId"), r.PathValue("ref"), q.Get("cursor"), limit)
	}))
	mux.HandleFunc("GET /v1/agent-skills/{ref}/versions/{version}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		version, err := strconv.Atoi(r.PathValue("version"))
		if err != nil || version < 1 {
			return nil, skillInvalid("version", "version is a positive whole number")
		}
		return s.AgentSkillVersion(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("ref"), version)
	}))
	mux.HandleFunc("PATCH /v1/agent-skills/{ref}", func(w http.ResponseWriter, r *http.Request) {
		in, ok := decode(w, r)
		if !ok {
			return
		}
		out, err := s.UpdateAgentSkill(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("ref"), in)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("DELETE /v1/agent-skills/{ref}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteAgentSkill(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("ref"))
	}))
}

// registerSecretREST mounts the ADR098 D4 workspace agent secrets: reads
// return names and scopes (D9), never values.
func (s *Service) registerSecretREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/agent-secrets", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.AgentSecrets(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("PUT /v1/agent-secrets/{name}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Value string  `json:"value"`
			Scope *string `json:"scope"`
			Repo  string  `json:"repo"`
		}
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &body); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := s.SetAgentSecret(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("name"), AgentSecretInput{Value: body.Value, Scope: body.Scope, Repo: body.Repo})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("DELETE /v1/agent-secrets/{name}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteAgentSecret(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("name"))
	}))
}

// AttachmentUploadPath is the streaming upload route; the REST mount exempts
// it from the buffered JSON body limit because the handler bounds its own
// stream at maxAttachmentBytes (ADR095 D2).
const AttachmentUploadPath = "/v1/agent-attachments"

func (s *Service) registerAttachmentREST(mux *http.ServeMux) {
	mux.HandleFunc("POST "+AttachmentUploadPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		view, err := s.UploadAttachment(r.Context(), UploadAttachmentRequest{
			OwnerID: q.Get("ownerId"),
			Name:    q.Get("name"),
			Size:    r.ContentLength,
			Body:    http.MaxBytesReader(w, r.Body, maxAttachmentBytes+1),
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, view)
	})
	mux.HandleFunc("GET "+AttachmentUploadPath+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		view, err := s.GetAttachment(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, view)
	})
	// Bytes are served only from the storage origin (ADR095 D7): a redirect,
	// never a proxy, so an uploaded HTML or SVG file cannot run on a bex origin.
	mux.HandleFunc("GET "+AttachmentUploadPath+"/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		signed, err := s.AttachmentURL(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, signed, http.StatusFound)
	})
	mux.HandleFunc("DELETE "+AttachmentUploadPath+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteAttachment(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /v1/agent-sessions/{id}/attachments", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, err := core.QueryLimit(q)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		var turn *int
		if raw := q.Get("turn"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				core.WriteErrStatus(w, http.StatusBadRequest, "turn must be a positive integer")
				return
			}
			turn = &n
		}
		list, err := s.ListAttachments(r.Context(), r.PathValue("id"), AttachmentListQuery{
			Direction: q.Get("direction"), Turn: turn, Cursor: q.Get("cursor"), Limit: limit,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, list)
	})
}

// maxEnvironmentBody bounds a PATCH /v1/agent-environment body (32 hosts and
// a model id fit in a few KiB).
const maxEnvironmentBody = 64 << 10
