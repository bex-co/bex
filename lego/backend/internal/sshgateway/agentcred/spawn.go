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

package agentcred

import (
	"context"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// SpawnForwarder is bex-api's internal spawn verb as the gateway sees it
// (*agentsession.SpawnClient). Every error it returns is a
// *agentsession.SpawnError safe to relay to the sandbox.
type SpawnForwarder interface {
	Spawn(context.Context, agentsession.SpawnEnvelope) (agentsession.SpawnResponse, error)
	ReadChild(context.Context, agentsession.SpawnEnvelope) (agentsession.ChildView, error)
}

// SpawnHandler serves the sandbox's child-session routes (ADR102 D9).
func (b *Broker) SpawnHandler() http.Handler { return http.HandlerFunc(b.serveSpawn) }

// serveSpawn is the spawn twin of serveResult: the same admit gate with the
// session-only Pod rule, then one signed call to bex-api naming the verified
// session and Pod. The verified session is the parent. POST starts a child
// from the sandbox's declared fields; GET reads one descendant by id. The
// sandbox holds no credential for either.
func (b *Broker) serveSpawn(w http.ResponseWriter, r *http.Request) {
	fail := func(code, message string) {
		agentsession.WriteSpawnError(w, agentsession.NewSpawnError(code, message))
	}
	if !b.Enabled() || b.Spawns == nil {
		fail(agentsession.SpawnUnavailable, "child sessions are not available on this gateway")
		return
	}
	if r.URL.RawQuery != "" || (r.Method != http.MethodPost && r.Method != http.MethodGet) {
		fail(agentsession.SpawnInvalidRequest, "POST a JSON object to start a child, or GET a child's id to read it")
		return
	}
	namespace, sessionID, childID, err := agentsession.ParseSpawnPath(r.URL.Path)
	if err != nil || (r.Method == http.MethodPost) != (childID == "") {
		fail(agentsession.SpawnInvalidRequest, "this is not a child-session URL")
		return
	}
	admitted, ok := b.admit(r, namespace, authorizeSession(sessionID), func(refusal admissionRefusal) {
		switch refusal {
		case refusedSource:
			fail(agentsession.SpawnForbidden, "unidentified source")
		case refusedConcurrency:
			fail(agentsession.SpawnRateLimited, "too many concurrent requests from this sandbox")
		case refusedPod:
			fail(agentsession.SpawnForbidden, "this request does not come from the session's sandbox")
		case refusedTarget:
			fail(agentsession.SpawnForbidden, "this sandbox may only manage children of its own session")
		case refusedSessionBudget:
			fail(agentsession.SpawnRateLimited, "request budget exhausted for this session")
		case refusedWorkspaceBudget:
			fail(agentsession.SpawnRateLimited, "request budget exhausted for this workspace")
		}
	})
	if !ok {
		return
	}
	defer admitted.release()
	r, pod, verified := admitted.r, admitted.pod, admitted.verified
	envelope := agentsession.SpawnEnvelope{
		Workspace: verified.Workspace, SessionID: verified.SessionID,
		PodName: verified.PodName, PodUID: verified.PodUID,
	}
	verb := agentsession.AuditVerbReadChild
	var response any
	if r.Method == http.MethodPost {
		verb = agentsession.AuditVerbSpawn
		content, refusal := b.readSpawnContent(w, r)
		if refusal != nil {
			agentsession.WriteSpawnError(w, refusal)
			return
		}
		envelope.Spawn = &content
		response, err = b.Spawns.Spawn(r.Context(), envelope)
	} else {
		envelope.ChildID = childID
		response, err = b.Spawns.ReadChild(r.Context(), envelope)
	}
	outcome := core.AuditAllowed
	if err != nil {
		outcome = core.AuditDenied
	}
	core.RecordAuditEvent(r.Context(), b.Audit, core.AuditEvent{
		Caller: pod.Name, CallerMethod: "sandbox", Verb: verb,
		Resource: core.WorkspaceObject(verified.Workspace), Target: "agent-session:" + verified.SessionID,
		Outcome: outcome, At: b.now(),
	})
	if err != nil {
		var typed *agentsession.SpawnError
		if !errors.As(err, &typed) {
			typed = agentsession.NewSpawnError(agentsession.SpawnUnavailable, "the request could not be completed")
		}
		log.Printf("agent spawn: refused (session=%s op=%s): %s", verified.SessionID, verb, typed.Code)
		agentsession.WriteSpawnError(w, typed)
		return
	}
	b.Metrics.Authentication("accepted")
	agentsession.WriteNoStoreJSON(w, http.StatusOK, response)
}

// readSpawnContent reads and strictly decodes a spawn request body.
func (b *Broker) readSpawnContent(w http.ResponseWriter, r *http.Request) (agentsession.SpawnContent, *agentsession.SpawnError) {
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/json" {
		return agentsession.SpawnContent{}, agentsession.NewSpawnError(agentsession.SpawnInvalidRequest, "POST a JSON object to this URL")
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, agentsession.MaxSpawnRequestBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return agentsession.SpawnContent{}, agentsession.NewSpawnError(agentsession.SpawnTooLarge, "the request body is too large")
		}
		return agentsession.SpawnContent{}, agentsession.NewSpawnError(agentsession.SpawnInvalidRequest, "request body could not be read")
	}
	return agentsession.DecodeSpawnContent(raw)
}
