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
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// PullRequestSyncer is bex-api's internal PR verb as the gateway sees it
// (*agentsession.PullRequestClient). Every error it returns is a
// *agentsession.PullRequestError safe to relay to the sandbox.
type PullRequestSyncer interface {
	Sync(context.Context, agentsession.PullRequestRequest) (agentsession.PullRequestResponse, error)
}

// servePullRequest is the PR-side twin of serveGit (ADR090): the same admit
// gate — limiter, source-IP Pod binding, label-digest authorization, and
// per-session/workspace budgets — then one signed call to bex-api. The sandbox
// supplies only {title, body}; the session, repository, and branch it may name
// are the ones its own Pod labels bind, and bex-api resolves the real targets
// from the durable record. No credential ever crosses back.
func (b *Broker) servePullRequest(w http.ResponseWriter, r *http.Request, namespace string, claimed agentsession.MintRequest) {
	fail := func(code, message string) {
		agentsession.WritePullRequestError(w, agentsession.NewPullRequestError(code, message))
	}
	if b.PullRequests == nil {
		fail(agentsession.PullRequestUnavailable, "pull requests are not available on this gateway")
		return
	}
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if r.Method != http.MethodPost || r.URL.RawQuery != "" || mediaErr != nil || mediaType != "application/json" {
		fail(agentsession.PullRequestInvalidRequest, "POST a JSON object to this URL")
		return
	}
	admitted, ok := b.admit(r, namespace, claimed, func(refusal admissionRefusal) {
		switch refusal {
		case refusedSource:
			fail(agentsession.PullRequestForbidden, "unidentified source")
		case refusedConcurrency:
			fail(agentsession.PullRequestRateLimited, "too many concurrent requests from this sandbox")
		case refusedPod:
			fail(agentsession.PullRequestForbidden, "this request does not come from the session's sandbox")
		case refusedTarget:
			fail(agentsession.PullRequestForbidden, "this sandbox may only open the pull request for its own session branch")
		case refusedSessionBudget:
			fail(agentsession.PullRequestRateLimited, "request budget exhausted for this session")
		case refusedWorkspaceBudget:
			fail(agentsession.PullRequestRateLimited, "request budget exhausted for this workspace")
		}
	})
	if !ok {
		return
	}
	defer admitted.release()
	r, pod, verified := admitted.r, admitted.pod, admitted.verified
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, agentsession.MaxPullRequestRequestBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(agentsession.PullRequestTooLarge, fmt.Sprintf("request body exceeds %d KiB", agentsession.MaxPullRequestRequestBytes>>10))
			return
		}
		fail(agentsession.PullRequestInvalidRequest, "request body could not be read")
		return
	}
	// Only title and body: an unknown field (head, base, draft, …) is refused
	// rather than silently ignored, so the agent learns it cannot choose them.
	var content agentsession.PullRequestContent
	if err := core.UnmarshalJSON(core.WithStrictJSONDecoding(r.Context()), raw, &content); err != nil {
		fail(agentsession.PullRequestInvalidRequest, "the body must be a JSON object with only \"title\" and \"body\"")
		return
	}
	if invalid := content.Validate(); invalid != nil {
		agentsession.WritePullRequestError(w, invalid)
		return
	}
	response, err := b.PullRequests.Sync(r.Context(), agentsession.PullRequestRequest{MintRequest: verified, PullRequestContent: content})
	outcome := core.AuditAllowed
	if err != nil {
		outcome = core.AuditDenied
	}
	core.RecordAuditEvent(r.Context(), b.Audit, core.AuditEvent{
		Caller: pod.Name, CallerMethod: "sandbox", Verb: agentsession.AuditVerbPullRequest,
		Resource: core.WorkspaceObject(verified.Workspace), Target: "agent-session:" + verified.SessionID,
		TargetName: verified.Repository, Outcome: outcome, At: b.now(),
	})
	if err != nil {
		var typed *agentsession.PullRequestError
		if !errors.As(err, &typed) {
			typed = agentsession.NewPullRequestError(agentsession.PullRequestUpstreamError, "the pull request could not be opened")
		}
		log.Printf("agent pull request: refused (session=%s repo=%s): %s", verified.SessionID, verified.Repository, typed.Code)
		agentsession.WritePullRequestError(w, typed)
		return
	}
	b.Metrics.Authentication("accepted")
	agentsession.WriteNoStoreJSON(w, http.StatusOK, response)
}
