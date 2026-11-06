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
	"log"
	"mime"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// DecisionForwarder is bex-api's internal decision verb as the gateway sees it
// (*agentsession.DecisionClient). Every error it returns is a
// *agentsession.ResultError safe to relay to the sandbox.
type DecisionForwarder interface {
	Forward(context.Context, agentsession.DecisionEnvelope) (agentsession.DecisionResponse, error)
}

// DecisionHandler serves the sandbox's decision route (ADR100 D5).
func (b *Broker) DecisionHandler() http.Handler { return http.HandlerFunc(b.serveDecision) }

// serveDecision is the decision-side twin of serveResult: the same admit gate
// with the session-only Pod rule, then one signed call to bex-api naming the
// verified session and Pod. An open is charged to the request budgets. An
// await (the path names the decision) is a repeated long poll on its own
// limiter and never spends the request budget: one bex-api held until its
// hold elapsed (still pending) is free, but one answered at once (an answer,
// an unknown or foreign decision, a refusal) is charged to the session's
// quick-await budget, so a sandbox cannot spin on bogus awaits.
func (b *Broker) serveDecision(w http.ResponseWriter, r *http.Request) {
	fail := func(code, message string) {
		agentsession.WriteResultError(w, agentsession.NewDecisionError(code, message))
	}
	if !b.Enabled() || b.Decisions == nil {
		fail(agentsession.ResultUnavailable, "decisions are not available on this gateway")
		return
	}
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if r.Method != http.MethodPost || r.URL.RawQuery != "" || mediaErr != nil || mediaType != "application/json" {
		fail(agentsession.ResultInvalidRequest, "POST a JSON object to this URL")
		return
	}
	namespace, sessionID, decisionID, err := agentsession.ParseDecisionPath(r.URL.Path)
	if err != nil {
		fail(agentsession.ResultInvalidRequest, "this is not a decision URL")
		return
	}
	awaiting := decisionID != ""
	opts := admitOptions{limiter: b.limiter(), charge: true}
	if awaiting {
		opts = admitOptions{limiter: b.awaitLimiter()}
	}
	admitted, ok := b.admitWith(r, namespace, authorizeSession(sessionID), refusalWriter(fail, "this sandbox may only ask for its own session"), opts)
	if !ok {
		return
	}
	defer admitted.release()
	r, pod, verified := admitted.r, admitted.pod, admitted.verified
	if awaiting && b.quickAwaits().sessionExhausted(namespace, verified.SessionID) {
		b.Metrics.LimitRejected("decision_await_budget")
		fail(agentsession.ResultRateLimited, "decision await budget exhausted for this session")
		return
	}
	raw, ok := readJSONBody(w, r, agentsession.MaxDecisionRequestBytes, fail)
	if !ok {
		return
	}
	envelope := agentsession.DecisionEnvelope{
		Workspace: verified.Workspace, SessionID: verified.SessionID,
		PodName: verified.PodName, PodUID: verified.PodUID,
	}
	if awaiting {
		envelope.Op, envelope.DecisionID = agentsession.DecisionOpAwait, decisionID
	} else {
		content, invalid := agentsession.DecodeDecisionContent(raw)
		if invalid != nil {
			agentsession.WriteResultError(w, invalid)
			return
		}
		envelope.Op, envelope.Content = agentsession.DecisionOpOpen, content
	}
	response, err := b.Decisions.Forward(r.Context(), envelope)
	if awaiting && (err != nil || response.State != store.AgentDecisionPending) {
		b.quickAwaits().chargeSession(namespace, verified.SessionID)
	}
	if !awaiting {
		outcome := core.AuditAllowed
		if err != nil {
			outcome = core.AuditDenied
		}
		core.RecordAuditEvent(r.Context(), b.Audit, core.AuditEvent{
			Caller: pod.Name, CallerMethod: "sandbox", Verb: agentsession.AuditVerbDecision,
			Resource: core.WorkspaceObject(verified.Workspace), Target: "agent-session:" + verified.SessionID,
			Outcome: outcome, At: b.now(),
		})
	}
	if err != nil {
		var typed *agentsession.ResultError
		if !errors.As(err, &typed) {
			typed = agentsession.NewDecisionError(agentsession.ResultUnavailable, "the decision could not be forwarded")
		}
		log.Printf("agent decision: refused (session=%s await=%t): %s", verified.SessionID, awaiting, typed.Code)
		agentsession.WriteResultError(w, typed)
		return
	}
	agentsession.WriteNoStoreJSON(w, http.StatusOK, response)
}
