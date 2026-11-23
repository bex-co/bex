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

package agentsession

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The mark-ready and merge verbs (ADR090 D3 amendments w10/m538, w10/m577
// and w10/041) let the agent finish the job when its task asks, as Devin does:
// undraft a pull request and merge it, now or, with auto, once its required
// checks pass (GitHub auto-merge, or the merge queue a base branch requires).
// By default they act on the session's own recorded pull request, pinned to
// the head bex recorded for it; a number names another pull request of the
// session's repository, pinned to the head GitHub reports when the call is
// made, as Devin's credentials can merge any pull request there.
// They ride the comment verb's Pod-bound gateway hop. The sandbox names no
// owner or repository: both come from the durable session record, and the
// GitHub token never leaves bex-api. Branch protection and rulesets decide
// whether GitHub merges, and an approval is never one of these verbs, so a
// required review stays a person's.

const (
	// PullRequestReadyProxySegment and PullRequestMergeProxySegment are the
	// path tails that turn a Git proxy URL (ProxyRepositoryURL) into the
	// gateway verbs: POST /git/<ns>/<sid>/<repo>/<branch>/pull-request/ready
	// and …/pull-request/merge.
	PullRequestReadyProxySegment = PullRequestProxySegment + "/ready"
	PullRequestMergeProxySegment = PullRequestProxySegment + "/merge"
	// InternalPullRequestReadyPath and InternalPullRequestMergePath live only
	// on bex-api's cluster-internal :8091 listener, beside the comment verb.
	InternalPullRequestReadyPath = "/v1/agent-session-pull-request-ready"
	InternalPullRequestMergePath = "/v1/agent-session-pull-request-merges"

	// PullRequestReadyKind and PullRequestMergeKind tag each verb's envelope
	// and its MAC domain.
	PullRequestReadyKind = "pull-request-ready"
	PullRequestMergeKind = "pull-request-merge"

	// AuditVerbPullRequestReady and AuditVerbPullRequestMerge record every
	// decision, on both hops.
	AuditVerbPullRequestReady = "agentsessions.PullRequestReady"
	AuditVerbPullRequestMerge = "agentsessions.PullRequestMerge"

	// MaxMergeCommitTitleRunes bounds the single-line merge commit title.
	MaxMergeCommitTitleRunes = MaxPullRequestTitleRunes
	// MaxMergeCommitMessageBytes bounds the merge commit message.
	MaxMergeCommitMessageBytes = MaxPullRequestBodyBytes
)

// The codes only these verbs return, beside the shared PullRequestNoPullRequest,
// PullRequestClosed and the PR verb's admission codes.
const (
	// PullRequestDraft refuses a merge of a draft, which GitHub never merges.
	PullRequestDraft = "pull_request_draft"
	// PullRequestHeadChanged refuses a merge whose pull request head is not
	// the one bex recorded: a commit pushed since is never merged unchecked.
	PullRequestHeadChanged = "head_changed"
	// PullRequestMergeBlocked is GitHub refusing the merge: required checks
	// pending or failing, a required review, a conflict, a rule.
	PullRequestMergeBlocked = "merge_blocked"
	// PullRequestMergeMethodNotAllowed refuses a method the repository or
	// its base branch's rules do not allow.
	PullRequestMergeMethodNotAllowed = "merge_method_not_allowed"
	// PullRequestNotPermitted is GitHub refusing the App itself: a permission
	// the installation has not granted, or a rule that restricts who merges.
	PullRequestNotPermitted = "not_permitted"
	// PullRequestAutoMergeNotAllowed refuses auto: the repository's "Allow
	// auto-merge" setting is off, so GitHub cannot merge it later.
	PullRequestAutoMergeNotAllowed = "auto_merge_not_allowed"
	// PullRequestNotFound refuses a number that names no pull request of the
	// session's repository (w10/041): none was opened, or it is an issue's.
	PullRequestNotFound = "pull_request_not_found"
)

// mergeNoOtherRoute ends every hint of these verbs, as noOtherRoute does the
// PR verb's.
const mergeNoOtherRoute = " Do not try another route (gh pr ready, gh pr merge, the GitHub API or a token): mark_pull_request_ready and merge_pull_request are the only way this session can change a pull request's state."

// NewPullRequestMergeError builds a typed refusal of the mark-ready or merge
// verb with the code's standard status and hint. Both verbs share it, the PR
// verb's shape and its writer (WritePullRequestError).
func NewPullRequestMergeError(code, message string) *PullRequestError {
	status, hint := http.StatusBadGateway, "Retry once later; if it keeps failing, tell the user the pull request was left as it is."
	switch code {
	case PullRequestNoPullRequest:
		status, hint = http.StatusConflict, "This session has no pull request yet. Push your work and open it with open_or_update_draft_pr first, or pass the \"number\" of the pull request you were asked to act on."
	case PullRequestClosed:
		status, hint = http.StatusConflict, "The pull request is closed or already merged, so there is nothing to mark ready or merge. Tell the user."
	case PullRequestDraft:
		status, hint = http.StatusConflict, "GitHub does not merge a draft. Call mark_pull_request_ready first, then merge_pull_request."
	case PullRequestHeadChanged:
		status, hint = http.StatusConflict, "A merge is pinned to the head bex recorded for the session's pull request, or for another pull request to the head GitHub reported when you called, so a commit it has not seen is never merged unchecked. "+
			"For the session's pull request: push your work, call open_or_update_draft_pr to record the new head, then call merge_pull_request once more (with \"auto\": true to have GitHub merge it once its checks pass). "+
			"For another pull request: look at what was pushed to it, and call merge_pull_request once more only if you were still asked to merge it."
	case PullRequestMergeBlocked:
		status, hint = http.StatusConflict, "GitHub refused the merge for the reason in the message; the repository's branch protection decides, not you. "+
			"Required checks pending: call merge_pull_request with \"auto\": true so GitHub merges it once they pass, and end your turn; or wait until they finish (`gh pr checks <number> --watch`, with the pull request's number: the one you passed, or the `number` open_or_update_draft_pr returned) and call merge_pull_request once more. "+
			"Checks failed: fix the failure, push, call open_or_update_draft_pr and wait again. "+
			"A review or approval required: you cannot give it, so stop and tell the user the pull request is waiting for a reviewer. "+
			"A conflict, or a branch behind its base: merge the base branch into your branch, push, call open_or_update_draft_pr, and try once more. "+
			"Do not call merge_pull_request in a loop."
	case PullRequestMergeMethodNotAllowed:
		status, hint = http.StatusUnprocessableEntity, "Call merge_pull_request again with a method the message lists, or with no method to use the repository's first allowed one."
	case PullRequestNotPermitted:
		status, hint = http.StatusForbidden, "GitHub does not let the bex GitHub App do this in this repository (a permission the installation has not granted, or a rule on who may merge). Do not retry; tell the user a person has to do it."
	case PullRequestAutoMergeNotAllowed:
		status, hint = http.StatusConflict, "The repository does not allow auto-merge (its \"Allow auto-merge\" setting is off), so GitHub cannot merge the pull request later. "+
			"Wait until its required checks pass (`gh pr checks <number> --watch`, with the pull request's number) and call merge_pull_request without \"auto\", or tell the user a person has to turn auto-merge on or merge it."
	case PullRequestInvalidRequest:
		status, hint = http.StatusUnprocessableEntity, "mark_pull_request_ready takes only an optional \"number\". merge_pull_request takes only an optional \"number\", \"method\" (merge, squash or rebase), \"commitTitle\", \"commitMessage\" and \"auto\": true, or \"disableAuto\": true with at most a \"number\". "+
			"\"number\" is a pull request number of the session's repository, which is fixed by the platform; omit it for the session's own pull request."
	case PullRequestNotFound:
		status, hint = http.StatusNotFound, "Check the number with `gh pr list` (an issue's number is not a pull request's), or tell the user the pull request was not found."
	case PullRequestTooLarge:
		status, hint = http.StatusRequestEntityTooLarge, fmt.Sprintf("Keep the commit title to %d characters on one line and the commit message under %d bytes.", MaxMergeCommitTitleRunes, MaxMergeCommitMessageBytes)
	case PullRequestForbidden:
		status, hint = http.StatusForbidden, "This sandbox may only mark ready or merge pull requests of its own session's repository while the session is running."
	case PullRequestRateLimited:
		status, hint = http.StatusTooManyRequests, "Wait and call this tool again later; do not call it in a loop."
	case PullRequestUnavailable:
		status, hint = http.StatusServiceUnavailable, "The pull-request service is unavailable; tell the user the pull request was left as it is."
	}
	return &PullRequestError{Status: status, Code: code, Message: message, Hint: hint + mergeNoOtherRoute}
}

// PullRequestReadyContent is the sandbox's whole say in a mark-ready: which
// pull request of the session's repository, the session's own when Number is
// unset (w10/041).
type PullRequestReadyContent struct {
	Number int `json:"number,omitempty"`
}

// Validate enforces the content rules on both hops.
func (c PullRequestReadyContent) Validate() *PullRequestError {
	return validPullRequestNumber(c.Number)
}

// validPullRequestNumber refuses a number no pull request can have; zero is
// unset, the session's own pull request.
func validPullRequestNumber(number int) *PullRequestError {
	if number < 0 {
		return NewPullRequestMergeError(PullRequestInvalidRequest, "number must be a positive pull request number")
	}
	return nil
}

// PullRequestMergeContent is the sandbox's whole say in a merge: which pull
// request of the session's repository (Number, the session's own when unset,
// w10/041), an optional method and commit text, and whether GitHub merges it
// now or once its required checks pass (Auto, gh pr merge --auto), or stops
// merging it later (DisableAuto, gh pr merge --disable-auto). Nothing in it
// names a repository or a head.
type PullRequestMergeContent struct {
	Number        int    `json:"number,omitempty"`
	Method        string `json:"method,omitempty"`
	CommitTitle   string `json:"commitTitle,omitempty"`
	CommitMessage string `json:"commitMessage,omitempty"`
	Auto          bool   `json:"auto,omitempty"`
	DisableAuto   bool   `json:"disableAuto,omitempty"`
}

// Validate enforces the content rules on both hops.
func (c PullRequestMergeContent) Validate() *PullRequestError {
	if invalid := validPullRequestNumber(c.Number); invalid != nil {
		return invalid
	}
	if c.DisableAuto && (c.Auto || c.Method != "" || c.CommitTitle != "" || c.CommitMessage != "") {
		return NewPullRequestMergeError(PullRequestInvalidRequest, "disableAuto takes no other field but number")
	}
	if c.Method != "" && !slices.Contains(github.MergeMethods, c.Method) {
		return NewPullRequestMergeError(PullRequestInvalidRequest, "method must be merge, squash or rebase")
	}
	if !utf8.ValidString(c.CommitTitle) || !utf8.ValidString(c.CommitMessage) || strings.ContainsRune(c.CommitMessage, 0) {
		return NewPullRequestMergeError(PullRequestInvalidRequest, "commitTitle and commitMessage must be valid UTF-8 text")
	}
	if strings.IndexFunc(c.CommitTitle, unicode.IsControl) >= 0 {
		return NewPullRequestMergeError(PullRequestInvalidRequest, "commitTitle must be a single line")
	}
	if utf8.RuneCountInString(c.CommitTitle) > MaxMergeCommitTitleRunes {
		return NewPullRequestMergeError(PullRequestTooLarge, fmt.Sprintf("commitTitle exceeds %d characters", MaxMergeCommitTitleRunes))
	}
	if len(c.CommitMessage) > MaxMergeCommitMessageBytes {
		return NewPullRequestMergeError(PullRequestTooLarge, fmt.Sprintf("commitMessage exceeds %d bytes", MaxMergeCommitMessageBytes))
	}
	return nil
}

// PullRequestReadyRequest is the signed, kind-tagged gateway→bex-api
// mark-ready request: the Pod-verified session identity plus the pull request
// the sandbox named, if any.
type PullRequestReadyRequest struct {
	Kind string `json:"kind"`
	MintRequest
	PullRequestReadyContent
}

func (r PullRequestReadyRequest) verbKind() string { return r.Kind }

// PullRequestReadyResponse is what the sandbox receives. AlreadyReady marks
// a pull request that was not a draft, so nothing changed. OtherPullRequest
// marks one that is not the session's own (w10/041).
type PullRequestReadyResponse struct {
	URL              string `json:"url"`
	Number           int    `json:"number"`
	AlreadyReady     bool   `json:"alreadyReady,omitempty"`
	OtherPullRequest bool   `json:"otherPullRequest,omitempty"`
}

// PullRequestMergeRequest is the signed, kind-tagged gateway→bex-api merge
// request: the Pod-verified session identity plus the sandbox's merge.
type PullRequestMergeRequest struct {
	Kind string `json:"kind"`
	MintRequest
	PullRequestMergeContent
}

func (r PullRequestMergeRequest) verbKind() string { return r.Kind }

// PullRequestMergeResponse is what a merge did to the pull request, its URL
// and number, at HeadSHA (the head bex recorded for the session's own pull
// request; for another, OtherPullRequest, the head GitHub reported when the
// call was made, w10/041), with exactly one outcome:
//   - Merged: merged now, by SHA (the commit the merge created) and Method;
//     for the session's own pull request the driver's delivery check counts
//     from HeadSHA once GitHub deletes the merged branch;
//   - AutoMerge: GitHub auto-merge is armed with Method, so GitHub merges it
//     once its required checks pass;
//   - MergeQueue: it is in its base branch's merge queue, or joins it once
//     its required checks pass, and the queue merges it with its own method;
//   - AutoMergeDisabled: GitHub no longer merges it later (disableAuto).
//
// An armed or queued pull request is not merged: the PR-state poller or a
// pull_request delivery records prState merged once GitHub merges it.
type PullRequestMergeResponse struct {
	URL               string `json:"url"`
	Number            int    `json:"number"`
	Merged            bool   `json:"merged"`
	SHA               string `json:"sha,omitempty"`
	Method            string `json:"method,omitempty"`
	HeadSHA           string `json:"headSha"`
	AutoMerge         bool   `json:"autoMerge,omitempty"`
	MergeQueue        bool   `json:"mergeQueue,omitempty"`
	AutoMergeDisabled bool   `json:"autoMergeDisabled,omitempty"`
	OtherPullRequest  bool   `json:"otherPullRequest,omitempty"`
}

// wellFormed is an answer the gateway client and the driver accept: the pull
// request named, exactly one outcome, and a merge's commit.
func (r PullRequestMergeResponse) wellFormed() bool {
	outcomes := 0
	for _, outcome := range []bool{r.Merged, r.AutoMerge, r.MergeQueue, r.AutoMergeDisabled} {
		if outcome {
			outcomes++
		}
	}
	return r.URL != "" && r.Number > 0 && outcomes == 1 && (!r.Merged || r.SHA != "")
}

// MarkReady undrafts the session's own recorded pull request, or the pull
// request of its repository the request names (w10/041). A pull request that
// is already ready answers AlreadyReady and is left as it is. Every refusal
// is a *PullRequestError.
func (p *PullRequester) MarkReady(ctx context.Context, req PullRequestReadyRequest) (response PullRequestReadyResponse, err error) {
	auditTarget := ""
	defer func() { p.audit(ctx, AuditVerbPullRequestReady, req.MintRequest, auditTarget, err) }()
	if invalid := req.PullRequestReadyContent.Validate(); invalid != nil {
		return PullRequestReadyResponse{}, invalid
	}
	target, pr, refusal := p.actedOnPullRequest(ctx, req.MintRequest, req.Number)
	auditTarget = target.auditName()
	if refusal != nil {
		return PullRequestReadyResponse{}, refusal
	}
	response = PullRequestReadyResponse{URL: pr.HTMLURL, Number: pr.Number, OtherPullRequest: target.other()}
	if !pr.Draft {
		response.AlreadyReady = true
		return response, nil
	}
	if pr.NodeID == "" {
		return PullRequestReadyResponse{}, NewPullRequestMergeError(PullRequestUpstreamError, "GitHub did not return the pull request's node id")
	}
	if err := p.GitHub.MarkPullRequestReady(ctx, target.installationID, target.name, pr.NodeID); err != nil {
		return PullRequestReadyResponse{}, gitHubStateRefusal(err, PullRequestUpstreamError, "GitHub refused or failed to mark the pull request ready")
	}
	return response, nil
}

// Merge merges the session's own recorded pull request, pinned to the head
// bex recorded for it, or the pull request of its repository the request
// names, pinned to the head GitHub reports for it now (w10/041), with the
// method the agent named or else the repository's first allowed one (ADR090
// D3, w10/m577 for the rest):
//   - a base branch that requires a merge queue takes it into the queue, with
//     or without auto, as gh pr merge and GitHub's button do; one already
//     queued is left as it is;
//   - auto arms GitHub auto-merge so GitHub merges it once its required
//     checks pass, unless it is mergeable now (gh's rule), which merges it;
//     one already armed is left as it is;
//   - disableAuto takes it out of the queue or disables its auto-merge.
//
// A merge records prState merged at once on every session of the workspace
// that recorded the pull request, so REST, GraphQL and MCP read it without
// waiting for the PR-state poller; an armed, queued or disarmed answer
// records the pull request's auto-merge and merge queue there the same way
// (w10/047), and the merge itself is left to the poller and the webhook.
// Every refusal is a *PullRequestError.
func (p *PullRequester) Merge(ctx context.Context, req PullRequestMergeRequest) (response PullRequestMergeResponse, err error) {
	auditTarget := ""
	defer func() { p.audit(ctx, AuditVerbPullRequestMerge, req.MintRequest, auditTarget, err) }()
	if invalid := req.PullRequestMergeContent.Validate(); invalid != nil {
		return PullRequestMergeResponse{}, invalid
	}
	target, pr, refusal := p.actedOnPullRequest(ctx, req.MintRequest, req.Number)
	auditTarget = target.auditName()
	if refusal != nil {
		return PullRequestMergeResponse{}, refusal
	}
	session := target.session
	// The session's own pull request merges at the head bex recorded for it.
	// Another has no recorded head, so it merges at the head GitHub reported
	// just now, which GitHub refuses if a push moves it before the merge.
	other, pin := target.other(), session.HeadSHA
	if other {
		pin = pr.HeadSHA
	}
	response = PullRequestMergeResponse{URL: pr.HTMLURL, Number: pr.Number, HeadSHA: pin, OtherPullRequest: other}
	if req.DisableAuto {
		// Stopping a later merge is never pinned: any head may be left unmerged.
		return p.disableAutoMerge(ctx, target, pr, response)
	}
	if pr.Draft {
		return PullRequestMergeResponse{}, NewPullRequestMergeError(PullRequestDraft, fmt.Sprintf("pull request #%d is still a draft", pr.Number))
	}
	switch {
	case other && pin == "":
		return PullRequestMergeResponse{}, NewPullRequestMergeError(PullRequestUpstreamError, fmt.Sprintf("GitHub did not return pull request #%d's head", pr.Number))
	case !other && (pin == "" || !strings.EqualFold(pin, pr.HeadSHA)):
		return PullRequestMergeResponse{}, NewPullRequestMergeError(PullRequestHeadChanged, fmt.Sprintf(
			"pull request #%d's head is %s, but bex recorded %s for it", pr.Number, ShortSHA(pr.HeadSHA), cmp.Or(ShortSHA(pin), "nothing")))
	}
	opts := github.MergeOptions{
		SHA: pin, Method: req.Method, CommitTitle: strings.TrimSpace(req.CommitTitle), CommitMessage: req.CommitMessage, Base: pr.BaseRef,
	}
	state, stateErr := p.mergeState(ctx, target.pullRequestTarget, pr)
	switch {
	case stateErr != nil && req.Auto:
		return PullRequestMergeResponse{}, stateErr
	case stateErr != nil:
		// A plain merge does not need the state: GitHub's answer to it is
		// final, a queue's refusal included.
		log.Printf("agent-session pull request merge: merge state unread, merging (session=%s): %v", session.ID, stateErr)
	case state.InMergeQueue:
		p.recordMergeMarks(ctx, target, state)
		response.MergeQueue = true
		return response, nil
	case state.MergeQueueEnabled:
		if err := p.GitHub.EnqueuePullRequest(ctx, target.installationID, target.name, pr.NodeID, pin); err != nil {
			return PullRequestMergeResponse{}, mergeRefusal(err)
		}
		p.rereadMergeMarks(ctx, target, pr, github.MergeState{InMergeQueue: true})
		response.MergeQueue = true
		return response, nil
	case req.Auto && state.AutoMerge != nil:
		p.recordMergeMarks(ctx, target, state)
		response.AutoMerge, response.Method = true, state.AutoMerge.Method
		return response, nil
	case req.Auto && !state.MergeableNow():
		method, err := p.GitHub.EnablePullRequestAutoMerge(ctx, target.installationID, target.owner, target.name, pr.NodeID, opts)
		switch {
		case err == nil:
			now := p.now().UTC()
			p.rereadMergeMarks(ctx, target, pr, github.MergeState{AutoMerge: &store.PRAutoMerge{Method: method, EnabledAt: &now}})
			response.AutoMerge, response.Method = true, method
			return response, nil
		case !errors.Is(err, github.ErrMergeableNow):
			return PullRequestMergeResponse{}, mergeRefusal(err)
		}
		// GitHub would merge it now, so it is merged now, as gh pr merge --auto does.
	}
	result, err := p.GitHub.MergePullRequest(ctx, target.installationID, target.owner, target.name, pr.Number, opts)
	if err != nil {
		return PullRequestMergeResponse{}, mergeRefusal(err)
	}
	p.recordMerged(ctx, target)
	response.Merged, response.SHA, response.Method = true, result.SHA, result.Method
	return response, nil
}

// mergeState reads what tells merging now, auto-merge and the queue apart.
func (p *PullRequester) mergeState(ctx context.Context, target pullRequestTarget, pr github.PullRequestDetail) (github.MergeState, *PullRequestError) {
	if pr.NodeID == "" {
		return github.MergeState{}, NewPullRequestMergeError(PullRequestUpstreamError, "GitHub did not return the pull request's node id")
	}
	state, err := p.GitHub.PullRequestMergeState(ctx, target.installationID, target.name, pr.NodeID)
	if err != nil {
		return github.MergeState{}, gitHubStateRefusal(err, PullRequestUpstreamError, "GitHub could not read the pull request's merge state")
	}
	return state, nil
}

// disableAutoMerge carries out disableAuto: the pull request leaves the merge
// queue or its auto-merge is disabled; one with neither answers the same.
func (p *PullRequester) disableAutoMerge(ctx context.Context, target stateTarget, pr github.PullRequestDetail, response PullRequestMergeResponse) (PullRequestMergeResponse, error) {
	state, refusal := p.mergeState(ctx, target.pullRequestTarget, pr)
	if refusal != nil {
		return PullRequestMergeResponse{}, refusal
	}
	if state.InMergeQueue || state.AutoMerge != nil {
		if err := p.GitHub.DisablePullRequestAutoMerge(ctx, target.installationID, target.name, pr.NodeID, state.InMergeQueue); err != nil {
			return PullRequestMergeResponse{}, gitHubStateRefusal(err, PullRequestUpstreamError, "GitHub refused or failed to disable auto-merge")
		}
		p.rereadMergeMarks(ctx, target, pr, github.MergeState{})
	} else {
		p.recordMergeMarks(ctx, target, state)
	}
	response.AutoMergeDisabled = true
	return response, nil
}

// rereadMergeMarks records the pull request's auto-merge and merge queue as
// GitHub reports them after the verb changed them, or, when that read fails,
// as implied says (what the verb's answer means), which the PR-state poller
// corrects on its next read.
func (p *PullRequester) rereadMergeMarks(ctx context.Context, target stateTarget, pr github.PullRequestDetail, implied github.MergeState) {
	state, err := p.GitHub.PullRequestMergeState(ctx, target.installationID, target.name, pr.NodeID)
	if err != nil {
		log.Printf("agent-session pull request merge: merge state unread after the change, recording the answer (session=%s): %v", target.session.ID, err)
		state = implied
	}
	p.recordMergeMarks(ctx, target, state)
}

// recordMergeMarks records the auto-merge and merge queue of state on every
// session of the workspace that recorded the target's pull request (w10/047),
// as recordMerged records a merge. They stand on GitHub either way, and the
// PR-state poller would record them later, so a failed write is only logged.
func (p *PullRequester) recordMergeMarks(ctx context.Context, target stateTarget, state github.MergeState) {
	session := target.session
	if _, err := p.Sessions.RecordPullRequestMergeMarks(ctx, []string{session.WorkspaceID}, target.repository, target.number, state.AutoMerge, state.InMergeQueue); err != nil {
		log.Printf("agent-session pull request merge: record auto-merge of %s#%d failed (session=%s): %v", target.repository, target.number, session.ID, err)
	}
}

// stateTarget is pullRequestTarget plus the number of the pull request whose
// state both verbs change.
type stateTarget struct {
	pullRequestTarget
	number int
}

// other reports a pull request other than the session's own (w10/041).
func (t stateTarget) other() bool {
	return t.number != t.session.PRNumber
}

// auditName names the repository, and the pull request once it is chosen,
// for the verbs' audit.
func (t stateTarget) auditName() string {
	return PullRequestAuditName(t.repository, t.number)
}

// PullRequestAuditName is the audit TargetName of a verb acting on pull
// request number of repository ("octo/repo#42"), or on the repository alone
// before a number is known (w10/041).
func PullRequestAuditName(repository string, number int) string {
	if repository == "" || number <= 0 {
		return repository
	}
	return fmt.Sprintf("%s#%d", repository, number)
}

// actedOnPullRequest is the front half both verbs share: the verified
// identity, the live sandbox, and the pull request read back from GitHub and
// still open. With number unset (or the session's own) that is the session's
// recorded pull request, verified as its own (its number, its head branch,
// its repository); a takeover session's is the person's pull request it took
// over. Any other number names a pull request of the session's repository,
// verified as that repository's, from any head branch or fork (w10/041). The
// target names the repository and the number as soon as each is known, so
// the caller's audit names them even when a later step refuses.
func (p *PullRequester) actedOnPullRequest(ctx context.Context, req MintRequest, number int) (stateTarget, github.PullRequestDetail, *PullRequestError) {
	if p.GitHub == nil || p.Connections == nil || p.Sessions == nil {
		return stateTarget{}, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestUnavailable, "pull requests are not configured")
	}
	if req.Workspace == "" || req.SessionID == "" || req.PodName == "" || req.PodUID == "" {
		return stateTarget{}, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestForbidden, "incomplete verified identity")
	}
	resolved, refusal := p.sessionTarget(ctx, req, NewPullRequestMergeError)
	target := stateTarget{pullRequestTarget: resolved}
	if refusal != nil {
		return target, github.PullRequestDetail{}, refusal
	}
	session := resolved.session
	target.number = cmp.Or(number, session.PRNumber)
	if target.number <= 0 {
		return target, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestNoPullRequest, "this session has not opened its pull request yet")
	}
	pr, err := p.GitHub.GetPullRequest(ctx, target.installationID, target.owner, target.name, target.number)
	var apiErr *github.APIError
	switch {
	case err != nil && target.other() && errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound:
		return target, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestNotFound, fmt.Sprintf("%s has no pull request #%d", target.repository, target.number))
	case err != nil:
		return target, github.PullRequestDetail{}, gitHubStateRefusal(err, PullRequestUpstreamError, fmt.Sprintf("GitHub could not read pull request #%d", target.number))
	}
	if pr.Number != target.number || !strings.EqualFold(pr.BaseRepo, target.repository) {
		return target, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestUpstreamError, fmt.Sprintf("GitHub returned a different pull request than #%d of %s", target.number, target.repository))
	}
	if !target.other() {
		headRef := session.Branch
		if session.IsTakeover() {
			headRef = session.TakeoverHeadRef
		}
		if pr.HeadRef != headRef || !strings.EqualFold(pr.HeadRepo, target.repository) {
			return target, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestUpstreamError, "GitHub returned a different pull request than the session's own")
		}
	}
	if pr.Merged {
		p.recordMerged(ctx, target)
	}
	if pr.Merged || pr.State != "open" {
		state := store.PRStateClosed
		if pr.Merged {
			state = store.PRStateMerged
		}
		return target, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestClosed, fmt.Sprintf("pull request #%d is already %s", pr.Number, state))
	}
	return target, pr, nil
}

// recordMerged records the target's pull request as merged, now, and never
// read again: on the session itself when it is the session's own, and
// otherwise on every session of the workspace that recorded it, another
// session's included, as a pull_request delivery would (w10/041). The merge
// stands on GitHub either way, and the PR-state poller would record it later,
// so a failed write is only logged.
func (p *PullRequester) recordMerged(ctx context.Context, target stateTarget) {
	session, number, now := target.session, target.number, p.now()
	if !target.other() {
		if _, err := p.Sessions.RecordAgentSessionPRState(ctx, session.ID, store.PRStateMerged, now, nil); err != nil {
			log.Printf("agent-session pull request merge: record merged state failed (session=%s): %v", session.ID, err)
		}
		return
	}
	if _, err := p.Sessions.RecordPullRequestState(ctx, []string{session.WorkspaceID}, target.repository, number, store.PRStateMerged, now, nil); err != nil {
		log.Printf("agent-session pull request merge: record merged state of %s#%d failed (session=%s): %v", target.repository, number, session.ID, err)
	}
}

// methodRefusal matches GitHub's own words for a merge method the repository
// or the branch does not allow ("Merge commits are not allowed on this
// repository.", "Squash merges are not allowed …").
var methodRefusal = regexp.MustCompile(`(?i)(merge commits|squash merges|rebase merges|merge method)[^.]*not (allowed|permitted)`)

// headMovedRefusal matches GitHub's GraphQL words for an expectedHeadOid that
// is no longer the pull request's head, which REST answers with 409.
var headMovedRefusal = regexp.MustCompile(`(?i)head[^.]*(does not|doesn't|did not|didn't) match`)

// mergeRefusal maps a refused merge, arm or enqueue to its typed code,
// carrying GitHub's own message: 405 not mergeable, 409 the head moved, 422
// invalid (GraphQL's refusals among them), 403 not permitted.
func mergeRefusal(err error) *PullRequestError {
	if errors.Is(err, github.ErrAutoMergeNotAllowed) {
		return NewPullRequestMergeError(PullRequestAutoMergeNotAllowed, githubSaid(github.APIErrorMessage(err), "the repository does not allow auto-merge"))
	}
	var methodErr *github.MergeMethodError
	if errors.As(err, &methodErr) {
		if methodErr.Method == "" {
			return NewPullRequestMergeError(PullRequestMergeMethodNotAllowed, "the repository allows no merge method")
		}
		return NewPullRequestMergeError(PullRequestMergeMethodNotAllowed, fmt.Sprintf(
			"the repository does not allow %s; it allows %s", methodErr.Method, orNone(methodErr.Allowed)))
	}
	var apiErr *github.APIError
	if errors.As(err, &apiErr) && !apiErr.RateLimited {
		message := github.APIErrorMessage(err)
		switch apiErr.Status {
		case http.StatusMethodNotAllowed, http.StatusUnprocessableEntity:
			if methodRefusal.MatchString(message) {
				return NewPullRequestMergeError(PullRequestMergeMethodNotAllowed, githubSaid(message, "the merge method is not allowed"))
			}
			if headMovedRefusal.MatchString(message) {
				return NewPullRequestMergeError(PullRequestHeadChanged, githubSaid(message, "the pull request's head no longer matches the recorded head"))
			}
			return NewPullRequestMergeError(PullRequestMergeBlocked, githubSaid(message, "the pull request is not mergeable"))
		case http.StatusConflict:
			return NewPullRequestMergeError(PullRequestHeadChanged, githubSaid(message, "the pull request's head no longer matches the recorded head"))
		}
	}
	return gitHubStateRefusal(err, PullRequestUpstreamError, "GitHub refused or failed the merge")
}

// gitHubStateRefusal maps the failures both verbs share: GitHub's rate limit,
// the App not permitted (a 403, or a token the installation cannot mint), and
// otherwise fallback with GitHub's message when it gave one.
func gitHubStateRefusal(err error, fallback, message string) *PullRequestError {
	var apiErr *github.APIError
	switch {
	case errors.Is(err, github.ErrPermissionNotGranted):
		return NewPullRequestMergeError(PullRequestNotPermitted, "the bex GitHub App's installation has not granted Contents and Pull requests write for this repository")
	case errors.As(err, &apiErr) && apiErr.RateLimited:
		return NewPullRequestMergeError(PullRequestRateLimited, "GitHub's rate limit for the bex GitHub App is exhausted")
	case errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden:
		return NewPullRequestMergeError(PullRequestNotPermitted, githubSaid(github.APIErrorMessage(err), "GitHub refused the bex GitHub App"))
	}
	return NewPullRequestMergeError(fallback, githubSaid(github.APIErrorMessage(err), message))
}

// githubSaid quotes GitHub's own message, or says fallback when it gave none.
func githubSaid(message, fallback string) string {
	if message == "" {
		return fallback
	}
	return "GitHub: " + message
}

func orNone(methods []string) string {
	if len(methods) == 0 {
		return "none"
	}
	return strings.Join(methods, ", ")
}

// ShortSHA is a commit as GitHub's and bex's messages abbreviate it.
func ShortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// PullRequestReadyHandler authenticates the gateway→bex-api mark-ready
// request. It is mounted only on :8091 and never under the public router.
type PullRequestReadyHandler struct {
	Secret       []byte
	PullRequests *PullRequester
	Nonce        NonceClaimer
	Now          func() time.Time
}

func (h *PullRequestReadyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var run func(context.Context, PullRequestReadyRequest) (PullRequestReadyResponse, error)
	if h.PullRequests != nil {
		run = h.PullRequests.MarkReady
	}
	serveKindedVerb(w, r, h.Secret, h.Nonce, h.Now, PullRequestReadyKind, NewPullRequestMergeError, run)
}

// PullRequestMergeHandler authenticates the gateway→bex-api merge request.
// It is mounted only on :8091 and never under the public router.
type PullRequestMergeHandler struct {
	Secret       []byte
	PullRequests *PullRequester
	Nonce        NonceClaimer
	Now          func() time.Time
}

func (h *PullRequestMergeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var run func(context.Context, PullRequestMergeRequest) (PullRequestMergeResponse, error)
	if h.PullRequests != nil {
		run = h.PullRequests.Merge
	}
	serveKindedVerb(w, r, h.Secret, h.Nonce, h.Now, PullRequestMergeKind, NewPullRequestMergeError, run)
}

// PullRequestReadyClient is the gateway's HMAC-authenticated caller for the
// internal mark-ready verb.
type PullRequestReadyClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

// MarkReady forwards a Pod-verified mark-ready request. Every failure is a
// *PullRequestError safe to relay to the sandbox.
func (c *PullRequestReadyClient) MarkReady(ctx context.Context, req PullRequestReadyRequest) (PullRequestReadyResponse, error) {
	req.Kind, req.Nonce = PullRequestReadyKind, newNonce()
	var out PullRequestReadyResponse
	if refusal := postKindedVerb(ctx, c.URL, c.Secret, c.Now, c.HTTP, PullRequestReadyKind, req, NewPullRequestMergeError, &out,
		func() bool { return out.URL != "" && out.Number > 0 }); refusal != nil {
		return PullRequestReadyResponse{}, refusal
	}
	return out, nil
}

// PullRequestMergeClient is the gateway's HMAC-authenticated caller for the
// internal merge verb.
type PullRequestMergeClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

// Merge forwards a Pod-verified merge. Every failure is a *PullRequestError
// safe to relay to the sandbox.
func (c *PullRequestMergeClient) Merge(ctx context.Context, req PullRequestMergeRequest) (PullRequestMergeResponse, error) {
	req.Kind, req.Nonce = PullRequestMergeKind, newNonce()
	var out PullRequestMergeResponse
	if refusal := postKindedVerb(ctx, c.URL, c.Secret, c.Now, c.HTTP, PullRequestMergeKind, req, NewPullRequestMergeError, &out,
		func() bool { return out.wellFormed() }); refusal != nil {
		return PullRequestMergeResponse{}, refusal
	}
	return out, nil
}
