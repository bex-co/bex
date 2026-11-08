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

// ResultForwarder is bex-api's internal result verb as the gateway sees it
// (*agentsession.ResultClient). Every error it returns is a
// *agentsession.ResultError safe to relay to the sandbox.
type ResultForwarder interface {
	Submit(context.Context, agentsession.ResultEnvelope) (agentsession.ResultResponse, error)
}

// ResultHandler serves the sandbox's structured-result route (ADR094 D5).
func (b *Broker) ResultHandler() http.Handler { return http.HandlerFunc(b.serveResult) }

// serveResult is the result-side twin of serveShareFile: the same admit gate
// with the session-only Pod rule, so repo-less sessions submit too, then one
// signed call to bex-api naming the verified session and Pod. The sandbox
// supplies only {result, final}; bex-api holds the schema it is checked against.
func (b *Broker) serveResult(w http.ResponseWriter, r *http.Request) {
	fail := func(code, message string) {
		agentsession.WriteResultError(w, agentsession.NewResultError(code, message))
	}
	if !b.Enabled() || b.Results == nil {
		fail(agentsession.ResultUnavailable, "structured results are not available on this gateway")
		return
	}
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if r.Method != http.MethodPost || r.URL.RawQuery != "" || mediaErr != nil || mediaType != "application/json" {
		fail(agentsession.ResultInvalidRequest, "POST a JSON object to this URL")
		return
	}
	namespace, sessionID, err := agentsession.ParseResultPath(r.URL.Path)
	if err != nil {
		fail(agentsession.ResultInvalidRequest, "this is not a result URL")
		return
	}
	admitted, ok := b.admit(r, namespace, authorizeSession(sessionID), func(refusal admissionRefusal) {
		switch refusal {
		case refusedSource:
			fail(agentsession.ResultForbidden, "unidentified source")
		case refusedConcurrency:
			fail(agentsession.ResultRateLimited, "too many concurrent requests from this sandbox")
		case refusedPod:
			fail(agentsession.ResultForbidden, "this request does not come from the session's sandbox")
		case refusedTarget:
			fail(agentsession.ResultForbidden, "this sandbox may only submit a result for its own session")
		case refusedSessionBudget:
			fail(agentsession.ResultRateLimited, "request budget exhausted for this session")
		case refusedWorkspaceBudget:
			fail(agentsession.ResultRateLimited, "request budget exhausted for this workspace")
		}
	})
	if !ok {
		return
	}
	defer admitted.release()
	r, pod, verified := admitted.r, admitted.pod, admitted.verified
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, agentsession.MaxResultRequestBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(agentsession.ResultTooLarge, "the request body is too large")
			return
		}
		fail(agentsession.ResultInvalidRequest, "request body could not be read")
		return
	}
	content, invalid := agentsession.DecodeResultContent(raw)
	if invalid != nil {
		agentsession.WriteResultError(w, invalid)
		return
	}
	response, err := b.Results.Submit(r.Context(), agentsession.ResultEnvelope{
		Workspace: verified.Workspace, SessionID: verified.SessionID,
		PodName: verified.PodName, PodUID: verified.PodUID,
		Result: content.Result, Final: content.Final,
	})
	outcome := core.AuditAllowed
	if err != nil {
		outcome = core.AuditDenied
	}
	core.RecordAuditEvent(r.Context(), b.Audit, core.AuditEvent{
		Caller: pod.Name, CallerMethod: "sandbox", Verb: agentsession.AuditVerbResult,
		Resource: core.WorkspaceObject(verified.Workspace), Target: "agent-session:" + verified.SessionID,
		Outcome: outcome, At: b.now(),
	})
	if err != nil {
		var typed *agentsession.ResultError
		if !errors.As(err, &typed) {
			typed = agentsession.NewResultError(agentsession.ResultUnavailable, "the result could not be submitted")
		}
		log.Printf("agent result: refused (session=%s): %s", verified.SessionID, typed.Code)
		agentsession.WriteResultError(w, typed)
		return
	}
	b.Metrics.Authentication("accepted")
	agentsession.WriteNoStoreJSON(w, http.StatusOK, response)
}
