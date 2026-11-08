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
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const maxAutomationBodyBytes = 1 << 20

// registerAutomationREST mounts the ADR101 D8 agent automations. {id} is an
// aut- id (its workspace comes from the row) or a name in ownerId's workspace.
func (s *Service) registerAutomationREST(mux *http.ServeMux) {
	owner := func(r *http.Request) string { return r.URL.Query().Get("ownerId") }
	input := func(r *http.Request) (AgentAutomationInput, error) {
		var in AgentAutomationInput
		data, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, maxAutomationBodyBytes))
		if err != nil {
			return in, automationInvalid("body", "the request body could not be read or is over 1 MiB")
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&in); err != nil {
			// A template that defines no such field already carries the named 400.
			if coded := new(core.CodedError); errors.As(err, &coded) {
				return in, err
			}
			return in, automationInvalid("body", "the request body is not a valid automation: "+err.Error())
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return in, automationInvalid("body", "the request body must contain exactly one JSON value")
		}
		return in, nil
	}

	mux.HandleFunc("GET /v1/agent-automations", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		mine := false
		if raw := q.Get("mine"); raw != "" {
			var err error
			if mine, err = strconv.ParseBool(raw); err != nil {
				return nil, automationInvalid("mine", "mine must be true or false")
			}
		}
		limit, err := core.QueryLimit(q)
		if err != nil {
			return nil, err
		}
		return s.AgentAutomations(r.Context(), owner(r), mine, q.Get("cursor"), limit)
	}))
	mux.HandleFunc("POST /v1/agent-automations", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		in, err := input(r)
		if err != nil {
			return nil, err
		}
		return s.CreateAgentAutomation(r.Context(), owner(r), in)
	}))
	mux.HandleFunc("GET /v1/agent-automations/{id}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.AgentAutomation(r.Context(), owner(r), r.PathValue("id"))
	}))
	mux.HandleFunc("PATCH /v1/agent-automations/{id}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		in, err := input(r)
		if err != nil {
			return nil, err
		}
		return s.UpdateAgentAutomation(r.Context(), owner(r), r.PathValue("id"), in)
	}))
	mux.HandleFunc("DELETE /v1/agent-automations/{id}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteAgentAutomation(r.Context(), owner(r), r.PathValue("id"))
	}))
	for verb, call := range map[string]func(*http.Request) (any, error){
		"pause": func(r *http.Request) (any, error) {
			return s.PauseAgentAutomation(r.Context(), owner(r), r.PathValue("id"))
		},
		"resume": func(r *http.Request) (any, error) {
			return s.ResumeAgentAutomation(r.Context(), owner(r), r.PathValue("id"))
		},
		"take-over": func(r *http.Request) (any, error) {
			return s.TakeOverAgentAutomation(r.Context(), owner(r), r.PathValue("id"))
		},
	} {
		mux.HandleFunc("POST /v1/agent-automations/{id}/"+verb, core.HandleJSON(http.StatusOK, call))
	}
	mux.HandleFunc("POST /v1/agent-automations/{id}/run", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		return s.RunAgentAutomationNow(r.Context(), owner(r), r.PathValue("id"))
	}))
	mux.HandleFunc("GET /v1/agent-automations/{id}/runs", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		limit, err := core.QueryLimit(q)
		if err != nil {
			return nil, err
		}
		return s.AgentAutomationRuns(r.Context(), owner(r), r.PathValue("id"), q.Get("cursor"), limit)
	}))
}
