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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The pull-request verb (ADR090) is the PR-side twin of the Git proxy's
// exact-branch receive-pack rule: the agent in the sandbox opens or updates its
// own DRAFT pull request, but only for the session's bound branch, into the
// repository's default branch, through the same Pod-bound gateway hop. The
// sandbox sends a title and a body and nothing else; owner, repository, base
// and head all come from the durable session record, and the GitHub token
// never leaves bex-api.

const (
	// PullRequestProxySegment is the fifth path segment that turns a Git proxy
	// URL (ProxyRepositoryURL) into the gateway PR verb:
	// POST /git/<ns>/<sid>/<repo>/<branch>/pull-request.
	PullRequestProxySegment = "pull-request"
	// InternalPullRequestPath lives only on bex-api's cluster-internal :8091
	// listener, beside InternalMintPath, under the same gateway-only HMAC.
	InternalPullRequestPath = "/v1/agent-session-pull-requests"

	// MaxPullRequestRequestBytes bounds the sandbox's JSON request at the gateway.
	MaxPullRequestRequestBytes = 64 << 10
	// MaxPullRequestTitleRunes bounds the single-line title.
	MaxPullRequestTitleRunes = 256
	// MaxPullRequestBodyBytes is GitHub's 65,536-byte body limit minus headroom
	// for the footer bex appends.
	MaxPullRequestBodyBytes = 60000

	// maxPullRequestInternalBody bounds the signed gateway→bex-api request: the
	// sandbox request (JSON-escaping can grow a body up to 6x for control
	// characters, but a valid body is capped at 60,000 bytes) plus the identity.
	maxPullRequestInternalBody = 512 << 10

	// AuditVerbPullRequest records every PR-verb decision, on both hops.
	AuditVerbPullRequest = "agentsessions.PullRequest"

	// PullRequestFooterMarker opens the server-appended footer. Everything from
	// the marker on is replaced on every update, so the footer appears exactly
	// once however many times the agent rewrites the body.
	PullRequestFooterMarker = "<!-- bex-agent-footer -->"

	// agentSessionDashboardRoute is the dashboard's session detail route
	// (dashboard/src/routes/agents_.$agentSessionId.tsx).
	agentSessionDashboardRoute = "agents"
)

// Pull-request error codes returned to the sandbox. They are stable: the
// in-sandbox tool reports them to the agent verbatim.
const (
	PullRequestBranchNotPushed = "branch_not_pushed"
	PullRequestNotAhead        = "not_ahead"
	PullRequestInvalidRequest  = "invalid_request"
	PullRequestTooLarge        = "too_large"
	PullRequestForbidden       = "forbidden"
	PullRequestUpstreamError   = "upstream_error"
	PullRequestRateLimited     = "rate_limited"
	PullRequestUnavailable     = "unavailable"
)

// noOtherRoute ends every hint: the agent must not go looking for a token, the
// GitHub API, or the gh CLI when the tool refuses.
const noOtherRoute = " Do not try another route (GitHub API, gh CLI, or a token): this tool is the only way this session can open its pull request."

// PullRequestError is a typed refusal of the PR verb. It carries no upstream
// text and no credential, so the gateway relays it to the sandbox as-is.
type PullRequestError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *PullRequestError) Error() string { return e.Code + ": " + e.Message }

// branchNotPushedHint names the exact push the agent's overlay prescribes.
func branchNotPushedHint(branch string) string {
	return "Commit your work and run `git push origin HEAD:" + branch + "` first, then call this tool again."
}

// NewPullRequestError builds a typed refusal with the code's standard status
// and hint.
func NewPullRequestError(code, message string) *PullRequestError {
	status, hint := http.StatusBadGateway, "Retry once later; if it keeps failing, tell the user the pull request could not be opened."
	switch code {
	case PullRequestBranchNotPushed:
		status, hint = http.StatusConflict, branchNotPushedHint("<session branch>")
	case PullRequestNotAhead:
		status, hint = http.StatusConflict, "The session branch has no commits beyond the base branch. Commit and push your changes, then call this tool again."
	case PullRequestInvalidRequest:
		status, hint = http.StatusUnprocessableEntity, "Send only a JSON object with a non-empty single-line \"title\" and a \"body\"; head and base are fixed by the platform."
	case PullRequestTooLarge:
		status, hint = http.StatusRequestEntityTooLarge, fmt.Sprintf("Keep the title to %d characters on one line and the body under %d bytes, then call this tool again.", MaxPullRequestTitleRunes, MaxPullRequestBodyBytes)
	case PullRequestForbidden:
		status, hint = http.StatusForbidden, "This sandbox may only open the pull request for its own session branch while the session is running."
	case PullRequestRateLimited:
		status, hint = http.StatusTooManyRequests, "Wait and call this tool again later; do not call it in a loop."
	case PullRequestUnavailable:
		status, hint = http.StatusServiceUnavailable, "The pull-request service is unavailable; tell the user the branch is pushed but the pull request could not be opened."
	}
	return &PullRequestError{Status: status, Code: code, Message: message, Hint: hint + noOtherRoute}
}

// WritePullRequestError writes a typed refusal as {"error":{code,message,hint}}.
func WritePullRequestError(w http.ResponseWriter, err *PullRequestError) {
	WriteNoStoreJSON(w, err.Status, struct {
		Error *PullRequestError `json:"error"`
	}{err})
}

// WriteNoStoreJSON writes v as an uncacheable JSON response: every gateway and
// signed-verb answer describes one live sandbox and must never be cached.
func WriteNoStoreJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// PullRequestContent is the sandbox's whole say in the PR: its title and body.
type PullRequestContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Validate enforces the content caps on both hops.
func (c PullRequestContent) Validate() *PullRequestError {
	if !utf8.ValidString(c.Title) || !utf8.ValidString(c.Body) || strings.ContainsRune(c.Body, 0) {
		return NewPullRequestError(PullRequestInvalidRequest, "title and body must be valid UTF-8 text")
	}
	title := strings.TrimSpace(c.Title)
	if title == "" {
		return NewPullRequestError(PullRequestInvalidRequest, "title is required")
	}
	if strings.IndexFunc(title, unicode.IsControl) >= 0 {
		return NewPullRequestError(PullRequestInvalidRequest, "title must be a single line")
	}
	if utf8.RuneCountInString(title) > MaxPullRequestTitleRunes {
		return NewPullRequestError(PullRequestTooLarge, fmt.Sprintf("title exceeds %d characters", MaxPullRequestTitleRunes))
	}
	if len(c.Body) > MaxPullRequestBodyBytes {
		return NewPullRequestError(PullRequestTooLarge, fmt.Sprintf("body exceeds %d bytes", MaxPullRequestBodyBytes))
	}
	return nil
}

// PullRequestRequest is the signed gateway→bex-api request: the Pod-verified
// session identity (as for a Git credential mint) plus the sandbox's content.
type PullRequestRequest struct {
	MintRequest
	PullRequestContent
}

// PullRequestResponse is what the sandbox receives. It never carries a token.
type PullRequestResponse struct {
	URL     string `json:"url"`
	Number  int    `json:"number"`
	Created bool   `json:"created"`
	HeadSHA string `json:"headSha"`
}

// PullRequestGitHub opens or updates the draft PR (*github.Client).
type PullRequestGitHub interface {
	SyncDraftPullRequest(ctx context.Context, installationID int64, owner, repo, head, title, body string) (github.DraftPullRequestResult, error)
}

// PullRequestSessionStore reads the durable session and records the PR on it
// mid-turn (*store.PGStore).
type PullRequestSessionStore interface {
	GetAgentSession(context.Context, string) (store.AgentSession, error)
	RecordAgentSessionPullRequest(ctx context.Context, id, prURL string, prNumber int, headSHA string) error
}

// PullRequester is bex-api's internal-only PR verb. It trusts only a request
// authenticated by PullRequestHandler's HMAC, then re-runs the live-generation
// check and resolves every target from the durable session record.
type PullRequester struct {
	GitHub      PullRequestGitHub
	Connections ConnectionStore
	Sessions    PullRequestSessionStore
	Audit       core.AuditSink
	// DashboardURL (BEX_DASHBOARD_URL) is the origin the footer's session link
	// points at; empty or invalid prints the session id instead.
	DashboardURL string
	Now          func() time.Time
}

// Sync opens or updates the session's draft PR. Every refusal is a
// *PullRequestError.
func (p *PullRequester) Sync(ctx context.Context, req PullRequestRequest) (response PullRequestResponse, err error) {
	auditRepository := ""
	defer func() {
		outcome := core.AuditAllowed
		if err != nil {
			outcome = core.AuditDenied
			log.Printf("agent-session pull request refused (session=%s repo=%s): %v", req.SessionID, auditRepository, err)
		}
		core.RecordAuditEvent(ctx, p.Audit, core.AuditEvent{
			Caller: req.PodName, CallerMethod: "sandbox", Verb: AuditVerbPullRequest,
			Resource: core.WorkspaceObject(req.Workspace), Target: "agent-session:" + req.SessionID,
			TargetName: auditRepository, Outcome: outcome, At: p.now(),
		})
	}()
	if p.GitHub == nil || p.Connections == nil || p.Sessions == nil {
		return PullRequestResponse{}, NewPullRequestError(PullRequestUnavailable, "pull requests are not configured")
	}
	if req.Workspace == "" || req.SessionID == "" || req.PodName == "" || req.PodUID == "" {
		return PullRequestResponse{}, NewPullRequestError(PullRequestForbidden, "incomplete verified identity")
	}
	if invalid := req.PullRequestContent.Validate(); invalid != nil {
		return PullRequestResponse{}, invalid
	}
	session, err := p.Sessions.GetAgentSession(ctx, req.SessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return PullRequestResponse{}, NewPullRequestError(PullRequestForbidden, "session not found")
		}
		return PullRequestResponse{}, NewPullRequestError(PullRequestUnavailable, "session lookup failed")
	}
	// The same live-generation + phase gate as the Git and model mints: a
	// retained, stale, or cross sandbox cannot open a PR after its turn ended.
	if !currentSandboxCaller(session, req.Workspace, req.PodName, req.PodUID) {
		return PullRequestResponse{}, NewPullRequestError(PullRequestForbidden, "this sandbox is not the session's live sandbox")
	}
	// The durable record is the authority; the Pod-label digests the gateway
	// checked must name the very same repository and branch.
	repository, normErr := NormalizeRepository(session.Repo)
	requested, reqErr := NormalizeRepository(req.Repository)
	if normErr != nil || reqErr != nil || repository != requested || session.Branch != req.Branch || ValidateBranch(session.Branch) != nil {
		return PullRequestResponse{}, NewPullRequestError(PullRequestForbidden, "the pull request must be for the session's own repository and branch")
	}
	auditRepository = repository
	owner, name, _ := strings.Cut(repository, "/")
	connection, err := p.Connections.GetGitConnectionByOwner(ctx, session.WorkspaceID, owner)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return PullRequestResponse{}, NewPullRequestError(PullRequestForbidden, "the workspace has no GitHub App installation for this repository's account")
		}
		return PullRequestResponse{}, NewPullRequestError(PullRequestUnavailable, "GitHub connection lookup failed")
	}
	body := PullRequestBodyWithFooter(req.Body, sessionProfile(session), p.sessionURL(session.ID), session.ID)
	result, err := p.GitHub.SyncDraftPullRequest(ctx, connection.InstallationID, owner, name, session.Branch, strings.TrimSpace(req.Title), body)
	switch {
	case errors.Is(err, github.ErrHeadBranchNotFound):
		refusal := NewPullRequestError(PullRequestBranchNotPushed, "the session branch "+session.Branch+" does not exist on GitHub yet")
		refusal.Hint = branchNotPushedHint(session.Branch) + noOtherRoute
		return PullRequestResponse{}, refusal
	case errors.Is(err, github.ErrHeadNotAhead):
		return PullRequestResponse{}, NewPullRequestError(PullRequestNotAhead, "the session branch "+session.Branch+" has no commits ahead of the base branch")
	case err != nil:
		return PullRequestResponse{}, NewPullRequestError(PullRequestUpstreamError, "GitHub refused or failed the pull request request")
	}
	// Persist immediately so REST/GraphQL/MCP show the PR mid-turn. The PR
	// exists on GitHub either way; a failed write is only logged.
	if recErr := p.Sessions.RecordAgentSessionPullRequest(ctx, session.ID, result.HTMLURL, result.Number, result.HeadSHA); recErr != nil {
		log.Printf("agent-session pull request: record on session failed (session=%s pr=%s): %v", session.ID, result.HTMLURL, recErr)
	}
	return PullRequestResponse{URL: result.HTMLURL, Number: result.Number, Created: result.Created, HeadSHA: result.HeadSHA}, nil
}

func (p *PullRequester) sessionURL(sessionID string) string {
	return resourcemeta.Config{DashboardBaseURL: p.DashboardURL}.DashboardURL(agentSessionDashboardRoute, sessionID)
}

func (p *PullRequester) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

// sessionProfile names the session's agent profile for the footer.
func sessionProfile(session store.AgentSession) string {
	var config struct {
		Agent string `json:"agent"`
	}
	_ = json.Unmarshal(session.AgentConfig, &config)
	if profile, ok := LookupAgentProfile(config.Agent); ok {
		return profile.ID
	}
	return "agent"
}

// footerAttribution opens the footer's attribution line.
const footerAttribution = "🤖 Opened by a bex agent session"

// PullRequestBodyWithFooter drops every footer already in body (the agent may
// have copied the PR's current description, even edited below it) and appends
// exactly one fresh footer. sessionURL empty falls back to the session id.
func PullRequestBodyWithFooter(body, profile, sessionURL, sessionID string) string {
	for {
		i := strings.Index(body, PullRequestFooterMarker)
		if i < 0 {
			break
		}
		body = strings.TrimRight(body[:i], " \t\r\n") + "\n\n" + dropFooterLines(body[i+len(PullRequestFooterMarker):])
	}
	body = strings.TrimRight(body, " \t\r\n")
	link := "session `" + sessionID + "`"
	if sessionURL != "" {
		link = "[view session](" + sessionURL + ")"
	}
	footer := PullRequestFooterMarker + "\n---\n" + footerAttribution + " (" + profile + ") · " + link
	if strings.TrimSpace(body) == "" {
		return footer
	}
	return body + "\n\n" + footer
}

// dropFooterLines removes the footer's own lines — its separator and
// attribution, in that order, blank lines between — from the text that
// followed a footer marker, keeping whatever the agent wrote after them.
func dropFooterLines(rest string) string {
	lines := strings.Split(rest, "\n")
	expect := []func(string) bool{
		func(line string) bool { return line == "---" },
		func(line string) bool { return strings.HasPrefix(line, footerAttribution) },
	}
	i := 0
	for ; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if len(expect) == 0 || !expect[0](line) {
			break
		}
		expect = expect[1:]
	}
	return strings.Join(lines[i:], "\n")
}

// PullRequestHandler authenticates the gateway→bex-api PR request. It is
// mounted only on :8091 and never under the public bex-api router.
type PullRequestHandler struct {
	Secret       []byte
	PullRequests *PullRequester
	Nonce        NonceClaimer
	Now          func() time.Time
}

func (h *PullRequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(h.Secret) == 0 || h.PullRequests == nil {
		WritePullRequestError(w, NewPullRequestError(PullRequestUnavailable, "pull requests are not configured"))
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	req, ok := readSignedRequest(w, r, h.Secret, now, h.Nonce, maxPullRequestInternalBody, func(req PullRequestRequest) string { return req.Nonce })
	if !ok {
		return
	}
	response, err := h.PullRequests.Sync(r.Context(), req)
	if err != nil {
		var typed *PullRequestError
		if !errors.As(err, &typed) {
			typed = NewPullRequestError(PullRequestUpstreamError, "the pull request could not be opened")
		}
		WritePullRequestError(w, typed)
		return
	}
	WriteNoStoreJSON(w, http.StatusOK, response)
}

// PullRequestClient is the gateway's HMAC-authenticated caller for the
// internal PR verb.
type PullRequestClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

// Sync forwards a Pod-verified PR request. Every failure is a
// *PullRequestError safe to relay to the sandbox.
func (c *PullRequestClient) Sync(ctx context.Context, req PullRequestRequest) (PullRequestResponse, error) {
	req.Nonce = newNonce()
	resp, err := signedPost(ctx, c.URL, c.Secret, c.now(), c.HTTP, req)
	if err != nil {
		return PullRequestResponse{}, NewPullRequestError(PullRequestUnavailable, "the pull-request service is unreachable")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusOK {
		var out PullRequestResponse
		if err := json.Unmarshal(raw, &out); err != nil || out.URL == "" || out.Number <= 0 {
			return PullRequestResponse{}, NewPullRequestError(PullRequestUpstreamError, "the pull-request service returned an invalid response")
		}
		return out, nil
	}
	var envelope struct {
		Error *PullRequestError `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) == nil && envelope.Error != nil && envelope.Error.Code != "" {
		// Re-derive status and hint from the code so the gateway only ever
		// relays its own vocabulary, whatever bytes came back.
		return PullRequestResponse{}, NewPullRequestError(envelope.Error.Code, envelope.Error.Message)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return PullRequestResponse{}, NewPullRequestError(PullRequestForbidden, "the pull request request was not authorized")
	}
	return PullRequestResponse{}, NewPullRequestError(PullRequestUpstreamError, fmt.Sprintf("the pull-request service returned status %d", resp.StatusCode))
}

func (c *PullRequestClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
