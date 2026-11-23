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
	s.handle(mux, "POST /v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OwnerID  string `json:"ownerId"`
			Plan     Plan   `json:"plan"`
			Template string `json:"template"`
			Region   string `json:"region"`
			// A pointer so an omitted timeout (86400 s) differs from an
			// explicit 0 (no timeout); see resolveTimeout.
			TimeoutSeconds *int           `json:"timeoutSeconds"`
			NetworkPolicy  *NetworkPolicy `json:"networkPolicy"`
			// Render's create-time env (`--env-var`/`--env-file`).
			Env map[string]string `json:"env"`
			// `--snapshot-id`, and `--from` with an id, send snapshotId; `--from
			// <name>` sends snapshotName: a snapshot of the sandbox's group to
			// start from (w32/m285).
			SnapshotID   string `json:"snapshotId"`
			SnapshotName string `json:"snapshotName"`
			// Render CLI v2.31+ `--idle-timeout`. A pointer so an omitted value
			// (900 s) differs from an explicit 0 (disabled); see
			// resolveIdleTimeout.
			IdleTimeoutSeconds *int `json:"idleTimeoutSeconds"`
			// v2.32 `--group`/`--sandbox-group` sends sandboxGroupId (w22/m311).
			SandboxGroupID string `json:"sandboxGroupId"`
			// v2.34 `--image` sends buildId (an `sbb-` id) or buildName, an
			// image imported into the sandbox's group (w2/m358).
			BuildID   string `json:"buildId"`
			BuildName string `json:"buildName"`
			// Render CLI v2.36 / SDK v1.8 `--name`.
			Name string `json:"name"`
		}
		// A missing/empty body is fine — template+plan fall back to defaults —
		// but malformed or unknown nested policy fields must never be ignored.
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		// The Render CLI sends ownerId in the BODY; the query param is the REST
		// convention. Prefer the query, fall back to the body (empty ⇒ default ws).
		owner := r.URL.Query().Get("ownerId")
		if owner == "" {
			owner = body.OwnerID
		}
		sb, err := s.Create(r.Context(), CreateRequest{
			OwnerID:            owner,
			Template:           body.Template,
			Plan:               body.Plan,
			Region:             body.Region,
			TimeoutSeconds:     body.TimeoutSeconds,
			IdleTimeoutSeconds: body.IdleTimeoutSeconds,
			NetworkPolicy:      body.NetworkPolicy,
			Name:               body.Name,
			Env:                body.Env,
			SandboxGroupID:     body.SandboxGroupID,
			BuildID:            body.BuildID,
			BuildName:          body.BuildName,
			SnapshotID:         body.SnapshotID,
			SnapshotName:       body.SnapshotName,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, sb)
	})
	s.handle(mux, "GET /v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		// A list never ignores a filter it was given: the group and status
		// filters apply (an unknown status is refused by name).
		ctx := core.WithWorkspace(r.Context(), query.Get("ownerId"))
		out, err := s.listFiltered(ctx, core.QueryList(query, "status"), query.Get("name"), query.Get("sandboxGroupId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		// Render's list is a cursor-paginated `[{sandbox, cursor}]` array, and
		// its schema requires each cursor (the Render SDK refuses an item
		// without one). The cursor is the sandbox id over listFiltered's stable
		// newest-first order. An unpaged call gets Render's first page of 20
		// (w10/m377); GraphQL and MCP keep the whole list.
		after, limit := core.PageParams(query)
		out = core.Page(out, after, limit, func(sb Sandbox) string { return sb.ID })
		envelopes := make([]SandboxEnvelope, 0, len(out))
		for _, sb := range out {
			envelopes = append(envelopes, SandboxEnvelope{Sandbox: sb, Cursor: sb.ID})
		}
		core.WriteJSON(w, http.StatusOK, envelopes)
	})
	s.handle(mux, "GET /v1/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
		sb, err := s.Get(ctx, r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, sb)
	})
	// Render's execution history (w32/m298): newest first, `cursor`/`limit`.
	s.handle(mux, "GET /v1/sandboxes/{id}/execs", func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
		after, limit := core.PageParams(r.URL.Query())
		out, err := s.ListExecutions(ctx, r.PathValue("id"), after, limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	s.handle(mux, "GET /v1/sandboxes/{id}/execs/{execId}", func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
		out, err := s.GetExecution(ctx, r.PathValue("id"), r.PathValue("execId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	// Render CLI `exec` → run one command, stream stdout/stderr + exitCode as SSE.
	// The CLI POSTs {"command":"…"} and reads the response as an event stream
	// (docs/render-artifacts/ea-sandbox.md §exec). ownerId is query-or-body.
	s.handle(mux, "POST /v1/sandboxes/{id}/exec", func(w http.ResponseWriter, r *http.Request) {
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
		r = allowLongResume(w, r)
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
	s.handle(mux, "POST /v1/sandboxes/{id}/runs/{operation}/token", func(w http.ResponseWriter, r *http.Request) {
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
		// Minting starts the resume of a sandbox suspended for inactivity and
		// answers at once, as Render does; the redeem waits for the resume.
		r = r.WithContext(withoutResumeWait(r.Context()))
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
	s.handle(mux, "POST /v1/sandboxes/{id}/files/{operation}/token", func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(withoutResumeWait(r.Context()))
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
	// Render CLI `stop` → terminate (204).
	s.handle(mux, "POST /v1/sandboxes/{id}/terminate", func(w http.ResponseWriter, r *http.Request) {
		s.lifecycleREST(w, r, http.StatusNoContent, s.Terminate)
	})
	// Render `suspend-sandbox` (round 9, w4/m596 t003): 202 with no body once
	// the sandbox starts suspending or when it is already suspending/suspended.
	// Replaces the earlier bex-only /pause (204) extension (ADR104).
	s.handle(mux, "POST /v1/sandboxes/{id}/suspend", func(w http.ResponseWriter, r *http.Request) {
		s.lifecycleREST(w, r, http.StatusAccepted, s.Suspend)
	})
	// bex lifecycle extension (Render's resuming status).
	s.handle(mux, "POST /v1/sandboxes/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		s.lifecycleREST(w, r, http.StatusNoContent, s.Resume)
	})
	s.registerGroupREST(mux)
	s.registerImageREST(mux)
	s.registerSnapshotREST(mux)
	s.registerCredentialREST(mux)
}

func (s *Service) lifecycleREST(w http.ResponseWriter, r *http.Request, status int, op func(ctx context.Context, id string) error) {
	ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
	if err := op(ctx, r.PathValue("id")); err != nil {
		core.WriteErr(w, err)
		return
	}
	w.WriteHeader(status)
}
