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
	"errors"
	"io"
	"log"
	"mime"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// spawnRoute is one child-session operation as the sandbox addresses it.
type spawnRoute int

const (
	routeSpawn spawnRoute = iota
	routeRead
	routeMessage
	routeCancel
	routeWait
)

// spawnRouteFor maps a method and the path's last segment to an operation;
// ok is false for any other combination.
func spawnRouteFor(method, rest string) (spawnRoute, bool) {
	switch {
	case method == http.MethodPost && rest == "":
		return routeSpawn, true
	case method == http.MethodPost && rest == agentsession.SpawnWaitSegment:
		return routeWait, true
	case rest == "" || rest == agentsession.SpawnWaitSegment:
		return 0, false
	case method == http.MethodGet:
		return routeRead, true
	case method == http.MethodPost:
		return routeMessage, true
	case method == http.MethodDelete:
		return routeCancel, true
	}
	return 0, false
}

// SpawnHandler serves the sandbox's child-session routes (ADR102 D9).
func (b *Broker) SpawnHandler() http.Handler { return http.HandlerFunc(b.serveSpawn) }

// serveSpawn is the spawn twin of serveResult: the same admit gate with the
// session-only Pod rule, then one signed call to bex-api naming the verified
// session and Pod. The verified session is the parent. POST starts a child
// from the sandbox's declared fields; GET reads one descendant by id; POST or
// DELETE on a descendant messages or cancels it; POST …/wait waits for
// descendants (w9/m346). The sandbox holds no credential for any of them.
func (b *Broker) serveSpawn(w http.ResponseWriter, r *http.Request) {
	fail := func(code, message string) {
		agentsession.WriteSpawnError(w, agentsession.NewSpawnError(code, message))
	}
	if !b.Enabled() || b.Spawns == nil {
		fail(agentsession.SpawnUnavailable, "child sessions are not available on this gateway")
		return
	}
	if r.URL.RawQuery != "" {
		fail(agentsession.SpawnInvalidRequest, "this route takes no query string")
		return
	}
	namespace, sessionID, childID, err := agentsession.ParseSpawnPath(r.URL.Path)
	route, routed := spawnRouteFor(r.Method, childID)
	if err != nil || !routed {
		fail(agentsession.SpawnInvalidRequest, "POST a JSON object to start a child; GET, POST or DELETE a child's id to read, message or cancel it; POST …/wait to wait")
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
	var verb string
	var response any
	switch route {
	case routeSpawn:
		verb = agentsession.AuditVerbSpawn
		raw, refusal := b.readSpawnBody(w, r)
		if refusal == nil {
			var content agentsession.SpawnContent
			if content, refusal = agentsession.DecodeSpawnContent(raw); refusal == nil {
				envelope.Spawn = &content
				response, err = b.Spawns.Spawn(r.Context(), envelope)
			}
		}
		if refusal != nil {
			agentsession.WriteSpawnError(w, refusal)
			return
		}
	case routeRead:
		verb, envelope.ChildID = agentsession.AuditVerbReadChild, childID
		response, err = b.Spawns.ReadChild(r.Context(), envelope)
	case routeCancel:
		verb, envelope.ChildID = agentsession.AuditVerbCancelChild, childID
		response, err = b.Spawns.CancelChild(r.Context(), envelope)
	case routeMessage:
		verb, envelope.ChildID = agentsession.AuditVerbMessageChild, childID
		var message agentsession.SpawnMessage
		if refusal := b.readCoordinatorBody(w, r, &message); refusal != nil {
			agentsession.WriteSpawnError(w, refusal)
			return
		}
		envelope.Message = &message
		response, err = b.Spawns.MessageChild(r.Context(), envelope)
	case routeWait:
		verb = agentsession.AuditVerbWaitChildren
		var wait agentsession.SpawnWait
		if refusal := b.readCoordinatorBody(w, r, &wait); refusal != nil {
			agentsession.WriteSpawnError(w, refusal)
			return
		}
		envelope.Wait = &wait
		// A wait idles for up to MaxSpawnWaitSeconds: past the Pod check it
		// must not hold the sandbox's concurrency slot, which git shares.
		admitted.freeSlot()
		response, err = b.Spawns.WaitChildren(r.Context(), envelope)
	}
	outcome := core.AuditAllowed
	if err != nil {
		outcome = core.AuditDenied
	}
	core.RecordAuditEvent(r.Context(), b.Audit, core.AuditEvent{
		Caller: pod.Name, CallerMethod: "sandbox", Verb: verb,
		Resource: core.WorkspaceObject(verified.Workspace), Target: "agent-session:" + verified.SessionID,
		TargetName: envelope.ChildID, Outcome: outcome, At: b.now(),
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

// readCoordinatorBody reads and strictly decodes a message or wait body.
func (b *Broker) readCoordinatorBody(w http.ResponseWriter, r *http.Request, out interface {
	Validate() *agentsession.SpawnError
}) *agentsession.SpawnError {
	raw, refusal := b.readSpawnBody(w, r)
	if refusal != nil {
		return refusal
	}
	return agentsession.DecodeSpawnBody(raw, out)
}

// readSpawnBody reads a bounded JSON request body.
func (b *Broker) readSpawnBody(w http.ResponseWriter, r *http.Request) ([]byte, *agentsession.SpawnError) {
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/json" {
		return nil, agentsession.NewSpawnError(agentsession.SpawnInvalidRequest, "POST a JSON object to this URL")
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, agentsession.MaxSpawnRequestBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, agentsession.NewSpawnError(agentsession.SpawnTooLarge, "the request body is too large")
		}
		return nil, agentsession.NewSpawnError(agentsession.SpawnInvalidRequest, "request body could not be read")
	}
	return raw, nil
}
