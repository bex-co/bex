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

// The mark-ready and merge verbs (ADR090 D3 amendment w10/m538) let the agent
// finish the job when its task asks, as Devin does: undraft the session's OWN
// recorded pull request, and merge it, pinned to the head bex recorded for it.
// They ride the comment verb's Pod-bound gateway hop. The sandbox names no
// pull request, owner or repository: all of them come from the durable
// session record, and the GitHub token never leaves bex-api. Branch
// protection and rulesets decide whether GitHub merges, and an approval is
// never one of these verbs, so a required review stays a person's.

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
)

// mergeNoOtherRoute ends every hint of these verbs, as noOtherRoute does the
// PR verb's.
const mergeNoOtherRoute = " Do not try another route (gh pr ready, gh pr merge, the GitHub API or a token): mark_pull_request_ready and merge_pull_request are the only way this session can change its pull request's state."

// NewPullRequestMergeError builds a typed refusal of the mark-ready or merge
// verb with the code's standard status and hint. Both verbs share it, the PR
// verb's shape and its writer (WritePullRequestError).
func NewPullRequestMergeError(code, message string) *PullRequestError {
	status, hint := http.StatusBadGateway, "Retry once later; if it keeps failing, tell the user the pull request was left as it is."
	switch code {
	case PullRequestNoPullRequest:
		status, hint = http.StatusConflict, "This session has no pull request yet. Push your work and open it with open_or_update_draft_pr first."
	case PullRequestClosed:
		status, hint = http.StatusConflict, "The pull request is closed or already merged, so there is nothing to mark ready or merge. Tell the user."
	case PullRequestDraft:
		status, hint = http.StatusConflict, "GitHub does not merge a draft. Call mark_pull_request_ready first, then merge_pull_request."
	case PullRequestHeadChanged:
		status, hint = http.StatusConflict, "A merge is pinned to the head bex recorded for the pull request, so a commit it has not recorded is never merged unchecked. "+
			"Push your work, call open_or_update_draft_pr to record the new head, wait for its checks, then call merge_pull_request once more."
	case PullRequestMergeBlocked:
		status, hint = http.StatusConflict, "GitHub refused the merge for the reason in the message; the repository's branch protection decides, not you. "+
			"Required checks pending: wait until they finish (`gh pr checks --watch`), then call merge_pull_request once more. "+
			"Checks failed: fix the failure, push, call open_or_update_draft_pr and wait again. "+
			"A review or approval required: you cannot give it, so stop and tell the user the pull request is waiting for a reviewer. "+
			"A conflict, or a branch behind its base: merge the base branch into your branch, push, call open_or_update_draft_pr, and try once more. "+
			"Do not call merge_pull_request in a loop."
	case PullRequestMergeMethodNotAllowed:
		status, hint = http.StatusUnprocessableEntity, "Call merge_pull_request again with a method the message lists, or with no method to use the repository's first allowed one."
	case PullRequestNotPermitted:
		status, hint = http.StatusForbidden, "GitHub does not let the bex GitHub App do this in this repository (a permission the installation has not granted, or a rule on who may merge). Do not retry; tell the user a person has to do it."
	case PullRequestInvalidRequest:
		status, hint = http.StatusUnprocessableEntity, "mark_pull_request_ready takes no input. merge_pull_request takes only an optional \"method\" (merge, squash or rebase), \"commitTitle\" and \"commitMessage\"; the repository and the pull request are fixed by the platform."
	case PullRequestTooLarge:
		status, hint = http.StatusRequestEntityTooLarge, fmt.Sprintf("Keep the commit title to %d characters on one line and the commit message under %d bytes.", MaxMergeCommitTitleRunes, MaxMergeCommitMessageBytes)
	case PullRequestForbidden:
		status, hint = http.StatusForbidden, "This sandbox may only mark ready or merge its own session's pull request while the session is running."
	case PullRequestRateLimited:
		status, hint = http.StatusTooManyRequests, "Wait and call this tool again later; do not call it in a loop."
	case PullRequestUnavailable:
		status, hint = http.StatusServiceUnavailable, "The pull-request service is unavailable; tell the user the pull request was left as it is."
	}
	return &PullRequestError{Status: status, Code: code, Message: message, Hint: hint + mergeNoOtherRoute}
}

// PullRequestMergeContent is the sandbox's whole say in a merge: an optional
// method and commit text. Nothing in it names a pull request.
type PullRequestMergeContent struct {
	Method        string `json:"method,omitempty"`
	CommitTitle   string `json:"commitTitle,omitempty"`
	CommitMessage string `json:"commitMessage,omitempty"`
}

// Validate enforces the content rules on both hops.
func (c PullRequestMergeContent) Validate() *PullRequestError {
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
// mark-ready request: the Pod-verified session identity, and nothing else.
type PullRequestReadyRequest struct {
	Kind string `json:"kind"`
	MintRequest
}

func (r PullRequestReadyRequest) verbKind() string { return r.Kind }

// PullRequestReadyResponse is what the sandbox receives. AlreadyReady marks
// a pull request that was not a draft, so nothing changed.
type PullRequestReadyResponse struct {
	URL          string `json:"url"`
	Number       int    `json:"number"`
	AlreadyReady bool   `json:"alreadyReady,omitempty"`
}

// PullRequestMergeRequest is the signed, kind-tagged gateway→bex-api merge
// request: the Pod-verified session identity plus the sandbox's merge.
type PullRequestMergeRequest struct {
	Kind string `json:"kind"`
	MintRequest
	PullRequestMergeContent
}

func (r PullRequestMergeRequest) verbKind() string { return r.Kind }

// PullRequestMergeResponse is a merged pull request: its URL and number, the
// commit the merge created, the method that made it, and the head it merged
// (the one bex recorded), which the driver's delivery check counts from once
// GitHub deletes the merged branch.
type PullRequestMergeResponse struct {
	URL     string `json:"url"`
	Number  int    `json:"number"`
	Merged  bool   `json:"merged"`
	SHA     string `json:"sha"`
	Method  string `json:"method"`
	HeadSHA string `json:"headSha"`
}

// MarkReady undrafts the session's own recorded pull request. A pull request
// that is already ready answers AlreadyReady and is left as it is. Every
// refusal is a *PullRequestError.
func (p *PullRequester) MarkReady(ctx context.Context, req PullRequestReadyRequest) (response PullRequestReadyResponse, err error) {
	auditRepository := ""
	defer func() { p.audit(ctx, AuditVerbPullRequestReady, req.MintRequest, auditRepository, err) }()
	target, pr, refusal := p.recordedPullRequest(ctx, req.MintRequest)
	auditRepository = target.repository
	if refusal != nil {
		return PullRequestReadyResponse{}, refusal
	}
	response = PullRequestReadyResponse{URL: pr.HTMLURL, Number: pr.Number}
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
// bex recorded for it, with the method the agent named or else the
// repository's first allowed one. A merge records prState merged on the
// session at once, so REST, GraphQL and MCP read it without waiting for the
// PR-state poller. Every refusal is a *PullRequestError.
func (p *PullRequester) Merge(ctx context.Context, req PullRequestMergeRequest) (response PullRequestMergeResponse, err error) {
	auditRepository := ""
	defer func() { p.audit(ctx, AuditVerbPullRequestMerge, req.MintRequest, auditRepository, err) }()
	if invalid := req.PullRequestMergeContent.Validate(); invalid != nil {
		return PullRequestMergeResponse{}, invalid
	}
	target, pr, refusal := p.recordedPullRequest(ctx, req.MintRequest)
	auditRepository = target.repository
	if refusal != nil {
		return PullRequestMergeResponse{}, refusal
	}
	if pr.Draft {
		return PullRequestMergeResponse{}, NewPullRequestMergeError(PullRequestDraft, fmt.Sprintf("pull request #%d is still a draft", pr.Number))
	}
	session := target.session
	if session.HeadSHA == "" || !strings.EqualFold(session.HeadSHA, pr.HeadSHA) {
		return PullRequestMergeResponse{}, NewPullRequestMergeError(PullRequestHeadChanged, fmt.Sprintf(
			"pull request #%d's head is %s, but bex recorded %s for it", pr.Number, ShortSHA(pr.HeadSHA), cmp.Or(ShortSHA(session.HeadSHA), "nothing")))
	}
	result, err := p.GitHub.MergePullRequest(ctx, target.installationID, target.owner, target.name, pr.Number, github.MergeOptions{
		SHA: session.HeadSHA, Method: req.Method, CommitTitle: strings.TrimSpace(req.CommitTitle), CommitMessage: req.CommitMessage, Base: pr.BaseRef,
	})
	if err != nil {
		return PullRequestMergeResponse{}, mergeRefusal(err)
	}
	p.recordMerged(ctx, session.ID)
	return PullRequestMergeResponse{URL: pr.HTMLURL, Number: pr.Number, Merged: true, SHA: result.SHA, Method: result.Method, HeadSHA: session.HeadSHA}, nil
}

// recordedPullRequest is the front half both verbs share: the verified
// identity, the live sandbox, and the session's recorded pull request read
// back from GitHub as the session's own (its number, its head branch, its
// repository) and still open. target is set once the repository is verified,
// so the caller's audit names it even when a later step refuses.
func (p *PullRequester) recordedPullRequest(ctx context.Context, req MintRequest) (pullRequestTarget, github.PullRequestDetail, *PullRequestError) {
	if p.GitHub == nil || p.Connections == nil || p.Sessions == nil {
		return pullRequestTarget{}, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestUnavailable, "pull requests are not configured")
	}
	if req.Workspace == "" || req.SessionID == "" || req.PodName == "" || req.PodUID == "" {
		return pullRequestTarget{}, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestForbidden, "incomplete verified identity")
	}
	target, refusal := p.sessionTarget(ctx, req, NewPullRequestMergeError)
	if refusal != nil {
		return target, github.PullRequestDetail{}, refusal
	}
	session := target.session
	// The one pull request these verbs can reach is the one the session
	// recorded; a takeover session's is the person's pull request it took over.
	if session.PRNumber <= 0 {
		return target, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestNoPullRequest, "this session has not opened its pull request yet")
	}
	pr, err := p.GitHub.GetPullRequest(ctx, target.installationID, target.owner, target.name, session.PRNumber)
	if err != nil {
		return target, github.PullRequestDetail{}, gitHubStateRefusal(err, PullRequestUpstreamError, "GitHub could not read the session's pull request")
	}
	headRef := session.Branch
	if session.IsTakeover() {
		headRef = session.TakeoverHeadRef
	}
	if pr.Number != session.PRNumber || pr.HeadRef != headRef || !strings.EqualFold(pr.BaseRepo, target.repository) || !strings.EqualFold(pr.HeadRepo, target.repository) {
		return target, github.PullRequestDetail{}, NewPullRequestMergeError(PullRequestUpstreamError, "GitHub returned a different pull request than the session's own")
	}
	if pr.Merged {
		p.recordMerged(ctx, session.ID)
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

// recordMerged records the session's pull request as merged, now, and never
// read again. The merge stands on GitHub either way, and the PR-state poller
// would record it later, so a failed write is only logged.
func (p *PullRequester) recordMerged(ctx context.Context, sessionID string) {
	if _, err := p.Sessions.RecordAgentSessionPRState(ctx, sessionID, store.PRStateMerged, p.now(), nil); err != nil {
		log.Printf("agent-session pull request merge: record merged state failed (session=%s): %v", sessionID, err)
	}
}

// methodRefusal matches GitHub's own words for a merge method the repository
// or the branch does not allow ("Merge commits are not allowed on this
// repository.", "Squash merges are not allowed …").
var methodRefusal = regexp.MustCompile(`(?i)(merge commits|squash merges|rebase merges|merge method)[^.]*not (allowed|permitted)`)

// mergeRefusal maps a refused merge to its typed code, carrying GitHub's own
// message: 405 not mergeable, 409 the head moved, 422 invalid, 403 not
// permitted.
func mergeRefusal(err error) *PullRequestError {
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
		func() bool { return out.Merged && out.URL != "" && out.Number > 0 && out.SHA != "" }); refusal != nil {
		return PullRequestMergeResponse{}, refusal
	}
	return out, nil
}
