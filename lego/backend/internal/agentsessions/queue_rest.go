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

package agentsessions

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type editQueuedMessageRequest struct {
	Prompt string `json:"prompt"`
}

type moveQueuedMessageRequest struct {
	Position int `json:"position"`
}

// registerQueueREST mounts the ADR047 D12 message queue and Stop turn.
func (s *Service) registerQueueREST(mux *http.ServeMux) {
	// Stop turn: ends only the running turn; 202 because the turn settles
	// within a Completer tick, after the request reaches the driver.
	mux.HandleFunc("POST /v1/agent-sessions/{id}/interrupt", core.HandleJSON(http.StatusAccepted, func(r *http.Request) (any, error) {
		return s.Interrupt(r.Context(), r.PathValue("id"))
	}))
	mux.HandleFunc("GET /v1/agent-sessions/{id}/queued-messages", core.HandleByID(s.QueuedMessages))
	mux.HandleFunc("PATCH /v1/agent-sessions/{id}/queued-messages/{messageId}", func(w http.ResponseWriter, r *http.Request) {
		var req editQueuedMessageRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := s.EditQueuedMessage(r.Context(), r.PathValue("id"), r.PathValue("messageId"), req.Prompt)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("DELETE /v1/agent-sessions/{id}/queued-messages/{messageId}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteQueuedMessage(r.Context(), r.PathValue("id"), r.PathValue("messageId"))
	}))
	mux.HandleFunc("POST /v1/agent-sessions/{id}/queued-messages/{messageId}/move", func(w http.ResponseWriter, r *http.Request) {
		var req moveQueuedMessageRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := s.MoveQueuedMessage(r.Context(), r.PathValue("id"), r.PathValue("messageId"), req.Position)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
}
