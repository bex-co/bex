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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// The pull-request comment verb (ADR047 D11) lets the agent answer the PR
// feedback that steered it: one comment on the session's OWN pull request,
// through the same Pod-bound gateway hop as the PR verb (ADR090). The sandbox
// sends only the comment body; owner, repository and PR number all come from
// the durable session record, so no call can name another pull request, and
// the GitHub token never leaves bex-api. The App posts the comment as a Bot,
// so the reply never steers the session back.

const (
	// PullRequestCommentProxySegment is the path tail that turns a Git proxy
	// URL (ProxyRepositoryURL) into the gateway comment verb:
	// POST /git/<ns>/<sid>/<repo>/<branch>/pull-request/comment.
	PullRequestCommentProxySegment = PullRequestProxySegment + "/comment"
	// InternalPullRequestCommentPath lives only on bex-api's cluster-internal
	// :8091 listener, beside InternalPullRequestPath.
	InternalPullRequestCommentPath = "/v1/agent-session-pull-request-comments"

	// PullRequestCommentKind tags the envelope, and its MAC domain, as this
	// verb's: a PR-verb signature (bare body) never verifies here, and this
	// envelope's "kind" is refused by every bare-body verb.
	PullRequestCommentKind = "pull-request-comment"

	// MaxPullRequestCommentBytes is the PR body's bound: GitHub's 65,536 limit
	// minus headroom for the footer bex appends.
	MaxPullRequestCommentBytes = MaxPullRequestBodyBytes

	// AuditVerbPullRequestComment records every comment decision, on both hops.
	AuditVerbPullRequestComment = "agentsessions.PullRequestComment"

	// PullRequestNoPullRequest refuses a comment before the session has opened
	// its pull request.
	PullRequestNoPullRequest = "no_pull_request"
)

var pullRequestCommentMACDomain = []byte(PullRequestCommentKind + "\n")

func pullRequestCommentMACInput(envelope []byte) []byte {
	return append(append([]byte{}, pullRequestCommentMACDomain...), envelope...)
}

// commentNoOtherRoute ends every comment hint, as noOtherRoute does the PR
// verb's.
const commentNoOtherRoute = " Do not try another route (GitHub API, gh CLI, or a token): this tool is the only way this session can comment on its pull request."

// NewPullRequestCommentError builds a typed refusal of the comment verb with
// the code's standard status and hint. It shares the PR verb's codes, shape and
// writer (WritePullRequestError).
func NewPullRequestCommentError(code, message string) *PullRequestError {
	status, hint := http.StatusBadGateway, "Retry once later; if it keeps failing, say in your final reply what you changed instead."
	switch code {
	case PullRequestNoPullRequest:
		status, hint = http.StatusConflict, "This session has no pull request yet. Push your work and open the draft pull request first, then call this tool again."
	case PullRequestInvalidRequest:
		status, hint = http.StatusUnprocessableEntity, "Send only a JSON object with a non-empty \"body\"; the repository and the pull request are fixed by the platform."
	case PullRequestTooLarge:
		status, hint = http.StatusRequestEntityTooLarge, fmt.Sprintf("Keep the comment under %d bytes, then call this tool again.", MaxPullRequestCommentBytes)
	case PullRequestForbidden:
		status, hint = http.StatusForbidden, "This sandbox may only comment on its own session's pull request while the session is running."
	case PullRequestRateLimited:
		status, hint = http.StatusTooManyRequests, "Wait and call this tool again later; do not call it in a loop."
	case PullRequestUnavailable:
		status, hint = http.StatusServiceUnavailable, "The comment service is unavailable; say in your final reply what you changed instead."
	}
	return &PullRequestError{Status: status, Code: code, Message: message, Hint: hint + commentNoOtherRoute}
}

// PullRequestCommentContent is the sandbox's whole say in a comment: its body.
type PullRequestCommentContent struct {
	Body string `json:"body"`
}

// Validate enforces the content caps on both hops.
func (c PullRequestCommentContent) Validate() *PullRequestError {
	if !utf8.ValidString(c.Body) || strings.ContainsRune(c.Body, 0) {
		return NewPullRequestCommentError(PullRequestInvalidRequest, "body must be valid UTF-8 text")
	}
	if strings.TrimSpace(c.Body) == "" {
		return NewPullRequestCommentError(PullRequestInvalidRequest, "body is required")
	}
	if len(c.Body) > MaxPullRequestCommentBytes {
		return NewPullRequestCommentError(PullRequestTooLarge, fmt.Sprintf("body exceeds %d bytes", MaxPullRequestCommentBytes))
	}
	return nil
}

// PullRequestCommentRequest is the signed, kind-tagged gateway→bex-api
// request: the Pod-verified session identity plus the sandbox's comment.
type PullRequestCommentRequest struct {
	Kind string `json:"kind"`
	MintRequest
	PullRequestCommentContent
}

// PullRequestCommentResponse is what the sandbox receives: the comment's URL
// and the pull request it was posted on. It never carries a token.
type PullRequestCommentResponse struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
}

// decodePullRequestCommentRequest reads exactly a comment envelope: its kind,
// no unknown field, one JSON value.
func decodePullRequestCommentRequest(raw []byte) (PullRequestCommentRequest, bool) {
	var req PullRequestCommentRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || decoder.More() {
		return PullRequestCommentRequest{}, false
	}
	return req, req.Kind == PullRequestCommentKind
}

// Comment posts the agent's comment on the session's own pull request, with a
// footer naming the bex agent session. Every refusal is a *PullRequestError.
func (p *PullRequester) Comment(ctx context.Context, req PullRequestCommentRequest) (response PullRequestCommentResponse, err error) {
	auditRepository := ""
	defer func() {
		outcome := core.AuditAllowed
		if err != nil {
			outcome = core.AuditDenied
			log.Printf("agent-session pull request comment refused (session=%s repo=%s): %v", req.SessionID, auditRepository, err)
		}
		core.RecordAuditEvent(ctx, p.Audit, core.AuditEvent{
			Caller: req.PodName, CallerMethod: "sandbox", Verb: AuditVerbPullRequestComment,
			Resource: core.WorkspaceObject(req.Workspace), Target: "agent-session:" + req.SessionID,
			TargetName: auditRepository, Outcome: outcome, At: p.now(),
		})
	}()
	if p.GitHub == nil || p.Connections == nil || p.Sessions == nil {
		return PullRequestCommentResponse{}, NewPullRequestCommentError(PullRequestUnavailable, "pull request comments are not configured")
	}
	if req.Workspace == "" || req.SessionID == "" || req.PodName == "" || req.PodUID == "" {
		return PullRequestCommentResponse{}, NewPullRequestCommentError(PullRequestForbidden, "incomplete verified identity")
	}
	if invalid := req.PullRequestCommentContent.Validate(); invalid != nil {
		return PullRequestCommentResponse{}, invalid
	}
	target, refusal := p.sessionTarget(ctx, req.MintRequest, NewPullRequestCommentError)
	auditRepository = target.repository
	if refusal != nil {
		return PullRequestCommentResponse{}, refusal
	}
	session := target.session
	// The one PR this verb can reach is the one the session recorded.
	if session.PRNumber <= 0 {
		return PullRequestCommentResponse{}, NewPullRequestCommentError(PullRequestNoPullRequest, "this session has not opened its pull request yet")
	}
	body := withAgentFooter(req.Body, commentFooterAttribution, sessionProfile(session), sessionDashboardURL(p.DashboardURL, session.ID), session.ID)
	url, err := p.GitHub.CreateIssueComment(ctx, target.installationID, target.owner, target.name, session.PRNumber, body)
	if err != nil {
		return PullRequestCommentResponse{}, NewPullRequestCommentError(PullRequestUpstreamError, "GitHub refused or failed the comment")
	}
	if url == "" {
		// Posted all the same: answer with the PR so the agent does not retry
		// into a duplicate comment.
		url = session.PRURL
	}
	return PullRequestCommentResponse{URL: url, Number: session.PRNumber}, nil
}

// PullRequestCommentHandler authenticates the gateway→bex-api comment request.
// It is mounted only on :8091 and never under the public bex-api router.
type PullRequestCommentHandler struct {
	Secret       []byte
	PullRequests *PullRequester
	// Nonce is not optional: a comment is not idempotent, so a replay inside
	// the clock skew would post it twice.
	Nonce NonceClaimer
	Now   func() time.Time
}

func (h *PullRequestCommentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(h.Secret) == 0 || h.PullRequests == nil || h.Nonce == nil {
		WritePullRequestError(w, NewPullRequestCommentError(PullRequestUnavailable, "pull request comments are not configured"))
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxPullRequestInternalBody+1))
	if err != nil || len(raw) == 0 || len(raw) > maxPullRequestInternalBody {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := Verify(h.Secret, pullRequestCommentMACInput(raw), r.Header.Get(TimestampHeader), r.Header.Get(SignatureHeader), now, mintClockSkew); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	req, ok := decodePullRequestCommentRequest(raw)
	if !ok {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !claimNonce(r.Context(), h.Nonce, req.Nonce, now) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	response, err := h.PullRequests.Comment(r.Context(), req)
	if err != nil {
		var typed *PullRequestError
		if !errors.As(err, &typed) {
			typed = NewPullRequestCommentError(PullRequestUpstreamError, "the comment could not be posted")
		}
		WritePullRequestError(w, typed)
		return
	}
	WriteNoStoreJSON(w, http.StatusOK, response)
}

// PullRequestCommentClient is the gateway's HMAC-authenticated caller for the
// internal comment verb.
type PullRequestCommentClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

// Comment forwards a Pod-verified comment. Every failure is a
// *PullRequestError safe to relay to the sandbox.
func (c *PullRequestCommentClient) Comment(ctx context.Context, req PullRequestCommentRequest) (PullRequestCommentResponse, error) {
	req.Kind = PullRequestCommentKind
	req.Nonce = newNonce()
	raw, err := json.Marshal(req)
	if err != nil {
		return PullRequestCommentResponse{}, NewPullRequestCommentError(PullRequestUnavailable, "the comment request could not be built")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(raw))
	if err != nil {
		return PullRequestCommentResponse{}, NewPullRequestCommentError(PullRequestUnavailable, "the comment request could not be built")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	signHeaders(httpReq.Header, c.Secret, pullRequestCommentMACInput(raw), now)
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return PullRequestCommentResponse{}, NewPullRequestCommentError(PullRequestUnavailable, "the pull-request service is unreachable")
	}
	var out PullRequestCommentResponse
	if refusal := readPullRequestReply(resp, NewPullRequestCommentError, &out, func() bool { return out.URL != "" && out.Number > 0 }); refusal != nil {
		return PullRequestCommentResponse{}, refusal
	}
	return out, nil
}
