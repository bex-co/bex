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
	"cmp"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// servePullRequest is the PR-side twin of serveGit (ADR090): the same admit
// gate — limiter, source-IP Pod binding, label-digest authorization, and
// per-session/workspace budgets — then one signed call to bex-api. The sandbox
// supplies only {title, body, draft}; the session, repository, and branch it
// may name are the ones its own Pod labels bind, and bex-api resolves the real
// targets from the durable record. No credential ever crosses back.
func (b *Broker) servePullRequest(w http.ResponseWriter, r *http.Request, namespace string, claimed agentsession.MintRequest) {
	if b.PullRequests == nil {
		agentsession.WritePullRequestError(w, agentsession.NewPullRequestError(agentsession.PullRequestUnavailable, "pull requests are not available on this gateway"))
		return
	}
	admitted, raw, ok := b.admitPullRequestVerb(w, r, namespace, claimed, agentsession.NewPullRequestError,
		"this sandbox may only open the pull request for its own session branch")
	if !ok {
		return
	}
	defer admitted.release()
	// Only title, body and draft: an unknown field (head, base, state, …) is
	// refused rather than silently ignored, so the agent learns it cannot
	// choose them.
	var content agentsession.PullRequestContent
	if err := core.UnmarshalJSON(core.WithStrictJSONDecoding(admitted.r.Context()), raw, &content); err != nil {
		agentsession.WritePullRequestError(w, agentsession.NewPullRequestError(agentsession.PullRequestInvalidRequest, "the body must be a JSON object with only \"title\", \"body\" and an optional boolean \"draft\""))
		return
	}
	if invalid := content.Validate(); invalid != nil {
		agentsession.WritePullRequestError(w, invalid)
		return
	}
	response, err := b.PullRequests.Sync(admitted.r.Context(), agentsession.PullRequestRequest{MintRequest: admitted.verified, PullRequestContent: content})
	b.answerPullRequestVerb(w, admitted, agentsession.AuditVerbPullRequest, admitted.verified.Repository, agentsession.NewPullRequestError, response, err)
}

// servePullRequestComment is the comment twin of servePullRequest (ADR047
// D11): the same gate, then one signed call to bex-api. The sandbox supplies
// only {body}; bex-api posts it on the session's recorded pull request, so a
// request naming any other number, repository or field is refused here.
func (b *Broker) servePullRequestComment(w http.ResponseWriter, r *http.Request, namespace string, claimed agentsession.MintRequest) {
	if b.PullRequestComments == nil {
		agentsession.WritePullRequestError(w, agentsession.NewPullRequestCommentError(agentsession.PullRequestUnavailable, "pull request comments are not available on this gateway"))
		return
	}
	admitted, raw, ok := b.admitPullRequestVerb(w, r, namespace, claimed, agentsession.NewPullRequestCommentError,
		"this sandbox may only comment on its own session's pull request")
	if !ok {
		return
	}
	defer admitted.release()
	var content agentsession.PullRequestCommentContent
	if err := core.UnmarshalJSON(core.WithStrictJSONDecoding(admitted.r.Context()), raw, &content); err != nil {
		agentsession.WritePullRequestError(w, agentsession.NewPullRequestCommentError(agentsession.PullRequestInvalidRequest, "the body must be a JSON object with only \"body\""))
		return
	}
	if invalid := content.Validate(); invalid != nil {
		agentsession.WritePullRequestError(w, invalid)
		return
	}
	response, err := b.PullRequestComments.Comment(admitted.r.Context(), agentsession.PullRequestCommentRequest{MintRequest: admitted.verified, PullRequestCommentContent: content})
	b.answerPullRequestVerb(w, admitted, agentsession.AuditVerbPullRequestComment, admitted.verified.Repository, agentsession.NewPullRequestCommentError, response, err)
}

// servePullRequestReady is the mark-ready twin of servePullRequestComment
// (w10/m538): the same gate, then one signed call to bex-api. The sandbox
// sends an empty JSON object, or only the number of another pull request of
// the session's repository (w10/041); bex-api undrafts the session's recorded
// pull request or that one, so a request naming a repository or any other
// field is refused here.
func (b *Broker) servePullRequestReady(w http.ResponseWriter, r *http.Request, namespace string, claimed agentsession.MintRequest) {
	if b.PullRequestReady == nil {
		agentsession.WritePullRequestError(w, agentsession.NewPullRequestMergeError(agentsession.PullRequestUnavailable, "marking pull requests ready is not available on this gateway"))
		return
	}
	admitted, raw, ok := b.admitPullRequestVerb(w, r, namespace, claimed, agentsession.NewPullRequestMergeError,
		"this sandbox may only mark pull requests of its own session's repository ready")
	if !ok {
		return
	}
	defer admitted.release()
	var content agentsession.PullRequestReadyContent
	if err := core.UnmarshalJSON(core.WithStrictJSONDecoding(admitted.r.Context()), raw, &content); err != nil {
		agentsession.WritePullRequestError(w, agentsession.NewPullRequestMergeError(agentsession.PullRequestInvalidRequest, "the body must be a JSON object with at most \"number\" (a pull request number)"))
		return
	}
	if invalid := content.Validate(); invalid != nil {
		agentsession.WritePullRequestError(w, invalid)
		return
	}
	response, err := b.PullRequestReady.MarkReady(admitted.r.Context(), agentsession.PullRequestReadyRequest{MintRequest: admitted.verified, PullRequestReadyContent: content})
	b.answerPullRequestVerb(w, admitted, agentsession.AuditVerbPullRequestReady, stateAuditName(admitted, response.Number, content.Number), agentsession.NewPullRequestMergeError, response, err)
}

// servePullRequestMerge is the merge twin (w10/m538). The sandbox supplies
// at most {number, method, commitTitle, commitMessage, auto}, or
// {number, disableAuto} (w10/m577, w10/041); bex-api merges the session's
// recorded pull request at its recorded head, or the numbered one at the head
// GitHub reports for it, arms GitHub auto-merge on it or queues it, or
// disarms it.
func (b *Broker) servePullRequestMerge(w http.ResponseWriter, r *http.Request, namespace string, claimed agentsession.MintRequest) {
	if b.PullRequestMerges == nil {
		agentsession.WritePullRequestError(w, agentsession.NewPullRequestMergeError(agentsession.PullRequestUnavailable, "merging pull requests is not available on this gateway"))
		return
	}
	admitted, raw, ok := b.admitPullRequestVerb(w, r, namespace, claimed, agentsession.NewPullRequestMergeError,
		"this sandbox may only merge pull requests of its own session's repository")
	if !ok {
		return
	}
	defer admitted.release()
	var content agentsession.PullRequestMergeContent
	if err := core.UnmarshalJSON(core.WithStrictJSONDecoding(admitted.r.Context()), raw, &content); err != nil {
		agentsession.WritePullRequestError(w, agentsession.NewPullRequestMergeError(agentsession.PullRequestInvalidRequest, "the body must be a JSON object with only \"number\" (a pull request number), \"method\", \"commitTitle\", \"commitMessage\" and \"auto\" (a boolean), or \"disableAuto\" (a boolean) with at most \"number\""))
		return
	}
	if invalid := content.Validate(); invalid != nil {
		agentsession.WritePullRequestError(w, invalid)
		return
	}
	response, err := b.PullRequestMerges.Merge(admitted.r.Context(), agentsession.PullRequestMergeRequest{MintRequest: admitted.verified, PullRequestMergeContent: content})
	b.answerPullRequestVerb(w, admitted, agentsession.AuditVerbPullRequestMerge, stateAuditName(admitted, response.Number, content.Number), agentsession.NewPullRequestMergeError, response, err)
}

// admitPullRequestVerb is the front half both PR-side verbs share: a JSON POST,
// the admit gate with its refusals in the verb's vocabulary (refuse), and the
// bounded request body. On refusal it has answered and reports false; on
// success the caller owns admission.release.
func (b *Broker) admitPullRequestVerb(w http.ResponseWriter, r *http.Request, namespace string, claimed agentsession.MintRequest, refuse func(code, message string) *agentsession.PullRequestError, targetRefusal string) (admission, []byte, bool) {
	fail := func(code, message string) {
		agentsession.WritePullRequestError(w, refuse(code, message))
	}
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if r.Method != http.MethodPost || r.URL.RawQuery != "" || mediaErr != nil || mediaType != "application/json" {
		fail(agentsession.PullRequestInvalidRequest, "POST a JSON object to this URL")
		return admission{}, nil, false
	}
	admitted, ok := b.admit(r, namespace, authorizeBranch(claimed), b.gitBudget(), func(refusal admissionRefusal) {
		switch refusal {
		case refusedSource:
			fail(agentsession.PullRequestForbidden, "unidentified source")
		case refusedConcurrency:
			fail(agentsession.PullRequestRateLimited, "too many concurrent requests from this sandbox")
		case refusedPod:
			fail(agentsession.PullRequestForbidden, "this request does not come from the session's sandbox")
		case refusedTarget:
			fail(agentsession.PullRequestForbidden, targetRefusal)
		case refusedSessionBudget:
			fail(agentsession.PullRequestRateLimited, "request budget exhausted for this session")
		case refusedWorkspaceBudget:
			fail(agentsession.PullRequestRateLimited, "request budget exhausted for this workspace")
		}
	})
	if !ok {
		return admission{}, nil, false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, admitted.r.Body, agentsession.MaxPullRequestRequestBytes))
	if err != nil {
		admitted.release()
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(agentsession.PullRequestTooLarge, fmt.Sprintf("request body exceeds %d KiB", agentsession.MaxPullRequestRequestBytes>>10))
			return admission{}, nil, false
		}
		fail(agentsession.PullRequestInvalidRequest, "request body could not be read")
		return admission{}, nil, false
	}
	return admitted, raw, true
}

// stateAuditName names the pull request a mark-ready or merge acted on, as
// bex-api's hop does: the one bex-api answered for, else the one the sandbox
// named, else the repository alone (w10/041).
func stateAuditName(admitted admission, answered, named int) string {
	return agentsession.PullRequestAuditName(admitted.verified.Repository, cmp.Or(answered, named))
}

// answerPullRequestVerb is the back half: audit bex-api's decision on the
// gateway hop under targetName, and relay it, a refusal only in the verb's
// own vocabulary.
func (b *Broker) answerPullRequestVerb(w http.ResponseWriter, admitted admission, auditVerb, targetName string, refuse func(code, message string) *agentsession.PullRequestError, response any, err error) {
	verified := admitted.verified
	outcome := core.AuditAllowed
	if err != nil {
		outcome = core.AuditDenied
	}
	core.RecordAuditEvent(admitted.r.Context(), b.Audit, core.AuditEvent{
		Caller: admitted.pod.Name, CallerMethod: "sandbox", Verb: auditVerb,
		Resource: core.WorkspaceObject(verified.Workspace), Target: "agent-session:" + verified.SessionID,
		TargetName: targetName, Outcome: outcome, At: b.now(),
	})
	if err != nil {
		var typed *agentsession.PullRequestError
		if !errors.As(err, &typed) {
			typed = refuse(agentsession.PullRequestUpstreamError, "the request could not be completed")
		}
		log.Printf("agent %s: refused (session=%s repo=%s): %s", auditVerb, verified.SessionID, verified.Repository, typed.Code)
		agentsession.WritePullRequestError(w, typed)
		return
	}
	b.Metrics.Authentication("accepted")
	agentsession.WriteNoStoreJSON(w, http.StatusOK, response)
}
