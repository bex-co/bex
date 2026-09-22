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

package sandbox

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST wires the Render-compatible `/v1/sandboxes*` surface so
// `render ea sandbox create/list/stop/exec` work unmodified (ADR042 D2,
// docs/render-artifacts/ea-sandbox.md, cli-compatibility-checklist rows
// 238-241). Exec is two-step for the pinned CLI — the run connect-token mint
// below plus the redeem route in connect.go, mounted outside the auth gate —
// while the single-step `POST /exec` stays for pre-v2.24 clients and MCP.
// ownerId is Render's workspace binding (query param).
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OwnerID        string         `json:"ownerId"`
			Plan           Plan           `json:"plan"`
			Template       string         `json:"template"`
			Region         string         `json:"region"`
			TimeoutSeconds int            `json:"timeoutSeconds"`
			NetworkPolicy  *NetworkPolicy `json:"networkPolicy"`
			// Env and SnapshotID are DECLARED so they can be REFUSED BY NAME.
			// The pinned client sends `env` for `--env-var`/`--env-file` and
			// `snapshotId` for `--snapshot-id`; with the fields absent, strict
			// decoding answered `unknown field "env"`, which names an internal
			// wire key rather than the flag the user typed and reads like a
			// malformed request rather than an unsupported feature (w9/067).
			// Declaring them costs nothing at runtime — neither is ever read
			// into CreateRequest — and lets the refusal below say what bex does
			// not support, the way SANDBOX_NETWORK_POLICY_UNSUPPORTED already
			// does for a policy bex will not claim to enforce.
			Env        map[string]string `json:"env"`
			SnapshotID string            `json:"snapshotId"`
		}
		// A missing/empty body is fine — template+plan fall back to defaults —
		// but malformed or unknown nested policy fields must never be ignored.
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := refuseUnsupportedCreateOptions(len(body.Env) > 0, body.SnapshotID != ""); err != nil {
			core.WriteErr(w, err)
			return
		}
		// The Render CLI sends ownerId in the BODY; the query param is the REST
		// convention. Prefer the query, fall back to the body (empty ⇒ default ws).
		owner := r.URL.Query().Get("ownerId")
		if owner == "" {
			owner = body.OwnerID
		}
		sb, err := s.Create(r.Context(), CreateRequest{
			OwnerID:        owner,
			Template:       body.Template,
			Plan:           body.Plan,
			Region:         body.Region,
			TimeoutSeconds: body.TimeoutSeconds,
			NetworkPolicy:  body.NetworkPolicy,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, sb)
	})
	mux.HandleFunc("GET /v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
		out, err := s.List(ctx)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		// The Render CLI reads a cursor-paginated `[{sandbox, cursor}]` array, not
		// a bare list — wrap each item so `render ea sandbox list` renders it.
		envelopes := make([]SandboxEnvelope, 0, len(out))
		for _, sb := range out {
			envelopes = append(envelopes, SandboxEnvelope{Sandbox: sb})
		}
		core.WriteJSON(w, http.StatusOK, envelopes)
	})
	mux.HandleFunc("GET /v1/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
		sb, err := s.Get(ctx, r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, sb)
	})
	// Render CLI `exec` → run one command, stream stdout/stderr + exitCode as SSE.
	// The CLI POSTs {"command":"…"} and reads the response as an event stream
	// (docs/render-artifacts/ea-sandbox.md §exec). ownerId is query-or-body.
	mux.HandleFunc("POST /v1/sandboxes/{id}/exec", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OwnerID string `json:"ownerId"`
			Command string `json:"command"`
		}
		_ = core.DecodeJSON(r, &body)
		owner := r.URL.Query().Get("ownerId")
		if owner == "" {
			owner = body.OwnerID
		}
		flush := func() {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		err := s.StreamExec(r.Context(), ExecRequest{
			OwnerID: owner, SandboxID: r.PathValue("id"), Command: body.Command,
		}, w, flush)
		if err != nil {
			core.WriteErr(w, err)
		}
	})
	// Pinned Render CLI `exec`, step one (w7/m147): mint a run connect token
	// bound to this sandbox and command. The client then sends the command to
	// the returned `uri` with the token as its Bearer (ConnectStreamHandler).
	// ownerId is query-or-body, as for /exec.
	mux.HandleFunc("POST /v1/sandboxes/{id}/runs/{operation}/token", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OwnerID string `json:"ownerId"`
			Command string `json:"command"`
		}
		if err := core.DecodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		owner := r.URL.Query().Get("ownerId")
		if owner == "" {
			owner = body.OwnerID
		}
		out, err := s.ConnectRun(r.Context(), ConnectRequest{
			OwnerID: owner, SandboxID: r.PathValue("id"), Operation: r.PathValue("operation"), Command: body.Command,
		}, s.connectBaseURL(r))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, out)
	})
	// The pinned file-copy client sends no JSON body: both ownerId and path
	// are query parameters, and operation is part of the mint route.
	mux.HandleFunc("POST /v1/sandboxes/{id}/files/{operation}/token", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.ConnectFile(r.Context(), FileConnectRequest{
			OwnerID: r.URL.Query().Get("ownerId"), SandboxID: r.PathValue("id"),
			Operation: r.PathValue("operation"), Path: r.URL.Query().Get("path"),
		}, s.connectBaseURL(r))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, out)
	})
	// Render CLI `stop` → terminate.
	mux.HandleFunc("POST /v1/sandboxes/{id}/terminate", func(w http.ResponseWriter, r *http.Request) {
		s.lifecycleREST(w, r, s.Terminate)
	})
	// bex lifecycle extensions (Render's suspended/resuming statuses).
	mux.HandleFunc("POST /v1/sandboxes/{id}/pause", func(w http.ResponseWriter, r *http.Request) {
		s.lifecycleREST(w, r, s.Suspend)
	})
	mux.HandleFunc("POST /v1/sandboxes/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		s.lifecycleREST(w, r, s.Resume)
	})
}

func (s *Service) lifecycleREST(w http.ResponseWriter, r *http.Request, op func(ctx context.Context, id string) error) {
	ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
	if err := op(ctx, r.PathValue("id")); err != nil {
		core.WriteErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
