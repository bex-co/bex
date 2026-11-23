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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// The session's GitHub API route (w10/m513 reads, w10/m522 writes; ADR090 D4
// amendments). The in-sandbox `gh` sends every call to a socket the driver
// serves, and the driver forwards it here, under the session's Pod-bound Git
// proxy URL: <repo URL>/github-api/<REST path> or /github-api/graphql. The
// route
//
//   - admits REST GET and HEAD on /repos/<the session's repository>[/…], and
//     POST /graphql for queries whose top-level fields read that repository,
//     a node by id, the rate limit or the schema;
//   - admits the writes in githubAPIWrites (REST) and githubAPIMutations
//     (GraphQL) on that repository: comments and replies, reviews that do not
//     approve, labels, assignees, requested reviewers, issues, re-running or
//     cancelling Actions runs, and dispatching a workflow on the session
//     branch;
//   - refuses every other write in GitHub's error shape, with a message naming
//     the bex MCP tools, which stay the session's one pull-request path, and
//     the session branch, which stays its one code path;
//   - mints an installation token narrowed to the repository for each call,
//     read-only for a read and with pull requests, issues and Actions write
//     (never contents) for an admitted write, and injects it on the upstream
//     hop only. A node id is scoped by that narrowing, which GitHub enforces;
//   - follows GitHub's redirects to its log and archive storage itself,
//     without the token, so `gh run view --log` works with no other egress.
const (
	defaultGitHubAPIOrigin = "https://api.github.com"
	// maxGitHubAPIRequestBytes bounds one request body, a GraphQL document
	// or a write: gh's largest query is a few KiB, and GitHub caps a comment
	// at 65536 characters, which JSON escaping can grow to several bytes each.
	maxGitHubAPIRequestBytes = 1 << 20
	// maxGitHubAPIResponseBytes bounds one streamed response, a run's log
	// archive included.
	maxGitHubAPIResponseBytes = 1 << 30
	// maxGitHubAPIRedirects bounds the storage redirects one read follows.
	maxGitHubAPIRedirects = 3
	githubAPIUserAgent    = "bex-agent-github-api-proxy/1"
)

// githubCLIOffRefusal answers every call of a session whose binding chain has
// `gh` off (w10/m524, w10/m561, ADR090 D4): bex-api refused the mint, so no
// token exists for the call. It names the level that turned `gh` off and how
// that level turns it back on; the workspace's words are also what a level
// this gateway does not know reads as.
func githubCLIOffRefusal(level agentsession.GitHubCLILevel) string {
	var head string
	switch level {
	case agentsession.GitHubCLISession:
		head = "bex: gh is turned off for this agent session (its agentConfig.githubCli, or that of the session that started it). " +
			"A workspace admin turns it back on by setting that session's GitHub CLI to inherit or on in its settings in the dashboard or with `bex agent update <session> --github-cli inherit`. "
	case agentsession.GitHubCLIAutomation:
		head = "bex: gh is turned off for the sessions of the automation that started this session (its template's agentConfig.githubCli). " +
			"A workspace admin turns it back on by setting it to inherit or on in the automation's settings or with `bex agent automations update <automation> --github-cli inherit`, after `bex agent automations take-over <automation>` when someone else created it. "
	case agentsession.GitHubCLIAutomationsDefault:
		head = "bex: gh is turned off for the sessions this workspace's automations start (the agent environment's automations GitHub CLI setting, automationsGithubCliEnabled). " +
			"A workspace admin turns it back on in Settings → Agents or with `bex agent env set --automations-github-cli inherit` or `on`. "
	default:
		head = "bex: gh is turned off for this workspace's agent sessions (the agent environment's GitHub CLI setting, githubCliEnabled). " +
			"A workspace admin turns it back on in Settings → Agents or with `bex agent env set --github-cli on`. "
	}
	return head + "Pushes to your session branch and the bex MCP tools open_or_update_draft_pr, reply_on_pull_request, mark_pull_request_ready and merge_pull_request still work."
}

// githubAPIRequestHeaders are the sandbox's request headers forwarded
// upstream. Authorization is never among them: it carries gh's placeholder.
var githubAPIRequestHeaders = []string{
	"Accept", "Content-Type", "GraphQL-Features", "If-Modified-Since", "If-None-Match", "Time-Zone", "X-GitHub-Api-Version",
}

// githubAPIResponseHeaders are the upstream headers relayed to the sandbox:
// the representation, pagination, caching and rate-limit headers gh reads.
// Nothing describing the token (scopes, expiry) is relayed.
var githubAPIResponseHeaders = []string{
	"Cache-Control", "Content-Disposition", "Content-Type", "ETag", "Last-Modified", "Link", "Retry-After",
	"X-GitHub-Media-Type", "X-GitHub-Request-Id", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset",
	"X-RateLimit-Resource", "X-RateLimit-Used",
}

// paginationRepository is a Link header's repository-id form, which the route
// does not serve; it is rewritten to the owner/name form gh asked with.
var paginationRepository = regexp.MustCompile(`(https?://[^/>]+)/repositories/[0-9]+([/?>])`)

// serveGitHubAPI forwards one admitted read or write of the session's
// repository (see above).
func (b *Broker) serveGitHubAPI(w http.ResponseWriter, r *http.Request, namespace string, claimed agentsession.MintRequest) {
	if b.GitHubRead == nil {
		writeGitHubAPIError(w, http.StatusServiceUnavailable, "bex: gh is not available in this session")
		return
	}
	repository, err := agentsession.NormalizeRepository(claimed.Repository)
	if err != nil {
		writeGitHubAPIError(w, http.StatusBadRequest, "bex: invalid request")
		return
	}
	apiPath, ok := githubAPIPath(r.URL.EscapedPath())
	if !ok {
		writeGitHubAPIError(w, http.StatusBadRequest, "bex: invalid request")
		return
	}
	access, status, refusal := githubAPIRequestRefusal(r.Method, apiPath, r.URL.RawQuery, repository)
	if refusal != "" {
		b.refuseGitHubAPI(w, claimed, "rejected_target", status, refusal)
		return
	}
	admitted, ok := b.admit(r, namespace, authorizeBranch(claimed), routeBudget{b.githubAPIBudgets(), "github_api"}, func(refusal admissionRefusal) {
		switch refusal {
		case refusedConcurrency:
			writeGitHubAPIError(w, http.StatusTooManyRequests, "bex: too many concurrent requests from this sandbox; retry shortly")
		case refusedSessionBudget:
			writeGitHubAPIError(w, http.StatusTooManyRequests, fmt.Sprintf("bex: this session has used its %d GitHub API requests", b.MaxGitHubAPIRequestsPerSession))
		case refusedWorkspaceBudget:
			writeGitHubAPIError(w, http.StatusTooManyRequests, "bex: this workspace has used its GitHub API requests for now")
		default:
			writeGitHubAPIError(w, http.StatusForbidden, "bex: forbidden")
		}
	})
	if !ok {
		return
	}
	defer admitted.release()
	r, verified := admitted.r, admitted.verified
	// A body is read only once admitted, under the admission's slot and
	// timeout, as the PR verbs read theirs, and checked as GitHub will read it.
	write := access.write != nil
	var body []byte
	if access.graphql || write {
		body, err = io.ReadAll(http.MaxBytesReader(w, r.Body, maxGitHubAPIRequestBytes))
		if tooLarge := new(http.MaxBytesError); errors.As(err, &tooLarge) {
			writeGitHubAPIError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("bex: a GitHub API request body is limited to %d KiB", maxGitHubAPIRequestBytes>>10))
			return
		} else if err != nil {
			writeGitHubAPIError(w, http.StatusBadRequest, "bex: the request body could not be read")
			return
		}
		if access.graphql {
			write, status, refusal = graphQLRefusal(body, repository)
		} else {
			status, refusal = restWriteBodyRefusal(*access.write, body, repository, verified.Branch)
		}
		if refusal != "" {
			b.refuseGitHubAPI(w, claimed, "rejected_target", status, refusal)
			return
		}
	}
	mint := b.GitHubRead
	if write {
		if mint = b.GitHubWrite; mint == nil {
			writeGitHubAPIError(w, http.StatusServiceUnavailable, "bex: gh cannot write from this session right now")
			return
		}
	}
	credential, err := mint.Mint(r.Context(), verified)
	if err != nil {
		switch {
		// ErrGitHubCLIOff wraps ErrForbidden, so it is tested first.
		case errors.Is(err, agentsession.ErrGitHubCLIOff):
			// bex-api's denied-mint log line names the level too.
			level, _ := agentsession.GitHubCLIOffLevel(err)
			b.refuseGitHubAPI(w, verified, "github_cli_off", http.StatusForbidden, githubCLIOffRefusal(level))
			return
		case errors.Is(err, agentsession.ErrForbidden):
			b.Metrics.Authentication("rejected_target")
			writeGitHubAPIError(w, http.StatusForbidden, "bex: forbidden")
			return
		}
		b.failUpstream("mint", agentsession.GitHubAPIProxySegment, verified, err)
		writeGitHubAPIError(w, http.StatusBadGateway, "bex: the GitHub API is unavailable right now; retry shortly")
		return
	}
	origin := b.GitHubAPIOrigin
	if origin == "" {
		origin = defaultGitHubAPIOrigin
	}
	target := origin + apiPath
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	var upstreamBody io.Reader
	if body != nil {
		upstreamBody = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, target, upstreamBody)
	if err != nil {
		writeGitHubAPIError(w, http.StatusBadRequest, "bex: invalid request")
		return
	}
	for _, name := range githubAPIRequestHeaders {
		if value := r.Header.Get(name); value != "" {
			request.Header.Set(name, value)
		}
	}
	request.Header.Set("Authorization", "Bearer "+credential.Token)
	request.Header.Set("User-Agent", githubAPIUserAgent)
	response, err := b.followStorageRedirects(request)
	if err != nil {
		b.failUpstream("network", agentsession.GitHubAPIProxySegment, verified, err)
		writeGitHubAPIError(w, http.StatusBadGateway, "bex: "+err.Error())
		return
	}
	defer response.Body.Close()
	for _, name := range githubAPIResponseHeaders {
		if value := response.Header.Get(name); value != "" {
			if name == "Link" {
				value = paginationRepository.ReplaceAllString(value, "${1}/repos/"+repository+"${2}")
			}
			w.Header().Set(name, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	b.relayCapped(w, r, response.Body, maxGitHubAPIResponseBytes, agentsession.GitHubAPIProxySegment, verified, io.Copy)
	b.Metrics.Authentication("accepted")
}

// followStorageRedirects sends the credential-bearing request without
// following redirects (the shared client never does), then follows GitHub's
// redirects to its log and archive storage with fresh, credential-free GETs.
// A redirect anywhere else is an error, never a request.
func (b *Broker) followStorageRedirects(request *http.Request) (*http.Response, error) {
	client := b.httpClient()
	response, err := client.Do(request)
	for hops := 0; err == nil && isRedirect(response.StatusCode); hops++ {
		location, locErr := response.Location()
		_ = response.Body.Close()
		switch {
		case locErr != nil:
			return nil, errors.New("GitHub answered with a redirect that has no location")
		case hops == maxGitHubAPIRedirects:
			return nil, errors.New("GitHub redirected this request too many times")
		case request.Method != http.MethodGet && request.Method != http.MethodHead:
			return nil, errors.New("GitHub redirected a request that is not a read")
		case !b.storageRedirect(location):
			return nil, fmt.Errorf("GitHub redirected this request to %s, which this route does not follow", location.Hostname())
		}
		next, reqErr := http.NewRequestWithContext(request.Context(), request.Method, location.String(), nil)
		if reqErr != nil {
			return nil, errors.New("GitHub answered with an invalid redirect")
		}
		next.Header.Set("User-Agent", githubAPIUserAgent)
		response, err = client.Do(next)
	}
	if err != nil {
		return nil, errors.New("the GitHub API is unreachable right now; retry shortly")
	}
	return response, nil
}

func isRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

// storageRedirect admits the hosts GitHub sends log and archive downloads to:
// its own (github.com and its subdomains, *.githubusercontent.com) and the
// Azure blob storage Actions logs live in, over HTTPS only. The request that
// follows carries no token, whichever of them it is.
func (b *Broker) storageRedirect(location *url.URL) bool {
	if b.redirectAllowed != nil {
		return b.redirectAllowed(location)
	}
	host := strings.ToLower(location.Hostname())
	return location.Scheme == "https" && location.User == nil && (host == "github.com" || strings.HasSuffix(host, ".github.com") ||
		strings.HasSuffix(host, ".githubusercontent.com") || strings.HasSuffix(host, ".blob.core.windows.net"))
}

// githubAPIPath returns the GitHub API path a route URL carries: everything
// after the github-api segment, in its escaped form, so the upstream path is
// exactly the path the parser checked.
func githubAPIPath(escaped string) (string, bool) {
	parts := strings.SplitN(strings.TrimPrefix(escaped, agentsession.GitProxyPath), "/", 5)
	if len(parts) != 5 {
		return "", false
	}
	apiPath, ok := strings.CutPrefix(parts[4], agentsession.GitHubAPIProxySegment)
	return apiPath, ok && strings.HasPrefix(apiPath, "/")
}

// githubAPIAccess is what an admitted request is: a GraphQL document, whose
// operations decide whether it reads or writes; an admitted REST write; or a
// REST read.
type githubAPIAccess struct {
	graphql bool
	write   *githubAPIWrite
}

// githubAPIRequestRefusal is the method and path policy: REST reads of the
// session's repository, the REST writes of githubAPIWrites on it, and GraphQL
// POSTs. It answers what an admitted request is, or the status and message of
// a refusal.
func githubAPIRequestRefusal(method, apiPath, rawQuery, repository string) (githubAPIAccess, int, string) {
	lower := strings.ToLower(apiPath)
	for _, bad := range []string{"%2f", "%5c", "%2e", "%00", "\\", "//"} {
		if strings.Contains(lower, bad) {
			return githubAPIAccess{}, http.StatusBadRequest, "bex: invalid request path"
		}
	}
	segments := strings.Split(strings.TrimPrefix(apiPath, "/"), "/")
	for _, segment := range segments {
		if segment == "." || segment == ".." {
			return githubAPIAccess{}, http.StatusBadRequest, "bex: invalid request path"
		}
	}
	if apiPath == "/graphql" {
		if method != http.MethodPost {
			return githubAPIAccess{}, http.StatusMethodNotAllowed, writeRefusal(repository)
		}
		return githubAPIAccess{graphql: true}, 0, ""
	}
	ownRepository := len(segments) >= 3 && segments[0] == "repos" && strings.EqualFold(segments[1]+"/"+segments[2], repository)
	if method == http.MethodGet || method == http.MethodHead {
		if !ownRepository {
			return githubAPIAccess{}, http.StatusForbidden, otherRepositoryRefusal(repository)
		}
		return githubAPIAccess{}, 0, ""
	}
	var write *githubAPIWrite
	if ownRepository {
		write = matchGitHubAPIWrite(method, segments[3:])
	}
	if write == nil && ownRepository && method == http.MethodPut && len(segments) == 6 && segments[3] == "pulls" && segments[5] == "merge" {
		return githubAPIAccess{}, http.StatusForbidden, pullRequestStateRefusal(repository)
	}
	if write == nil {
		return githubAPIAccess{}, http.StatusForbidden, writeRefusal(repository)
	}
	// GitHub may read a parameter from the query string as well as the body,
	// and the route checks the body.
	if rawQuery != "" {
		return githubAPIAccess{}, http.StatusBadRequest, "bex: a write carries its parameters in its JSON body, not the query string"
	}
	return githubAPIAccess{write: write}, 0, ""
}

// githubAPIWrite is one REST write gh may make (w10/m522): its method and
// path below /repos/<the session's repository>/, where # is a number and * any
// one segment.
type githubAPIWrite struct {
	route string
	// review marks a review submission, whose event may not approve.
	review bool
	// dispatch marks a workflow dispatch, whose ref must be the session
	// branch (w10/033).
	dispatch bool
}

// githubAPIWrites are the REST writes admitted on the session's repository:
// issue and pull-request comments, review comments and their replies,
// reviews, reactions, labels, assignees and requested reviewers, new issues,
// re-running or cancelling a run, and dispatching a workflow, by id or file
// name, on the session branch. `PATCH issues/#` is not among them:
// GitHub treats a pull request as an issue there, so it could retitle,
// rewrite or close one. gh's own `issue edit` and `issue close` send GraphQL,
// where GitHub resolves the id as an issue.
var githubAPIWrites = []githubAPIWrite{
	{route: "POST issues"},
	{route: "POST issues/#/comments"},
	{route: "PATCH issues/comments/#"}, {route: "DELETE issues/comments/#"},
	{route: "POST issues/#/labels"}, {route: "PUT issues/#/labels"}, {route: "DELETE issues/#/labels"}, {route: "DELETE issues/#/labels/*"},
	{route: "POST issues/#/assignees"}, {route: "DELETE issues/#/assignees"},
	{route: "POST issues/#/reactions"}, {route: "DELETE issues/#/reactions/#"},
	{route: "POST issues/comments/#/reactions"}, {route: "DELETE issues/comments/#/reactions/#"},
	{route: "POST pulls/#/comments"}, {route: "POST pulls/#/comments/#/replies"},
	{route: "PATCH pulls/comments/#"}, {route: "DELETE pulls/comments/#"},
	{route: "POST pulls/comments/#/reactions"}, {route: "DELETE pulls/comments/#/reactions/#"},
	{route: "POST pulls/#/reviews", review: true}, {route: "POST pulls/#/reviews/#/events", review: true},
	{route: "PUT pulls/#/reviews/#"}, {route: "DELETE pulls/#/reviews/#"},
	{route: "POST pulls/#/requested_reviewers"}, {route: "DELETE pulls/#/requested_reviewers"},
	{route: "POST actions/runs/#/rerun"}, {route: "POST actions/runs/#/rerun-failed-jobs"},
	{route: "POST actions/runs/#/cancel"}, {route: "POST actions/runs/#/force-cancel"},
	{route: "POST actions/jobs/#/rerun"},
	{route: "POST actions/workflows/*/dispatches", dispatch: true},
}

// matchGitHubAPIWrite is the admitted write a method and the path segments
// below the repository name, or nil.
func matchGitHubAPIWrite(method string, rest []string) *githubAPIWrite {
	for i := range githubAPIWrites {
		ruleMethod, pattern, _ := strings.Cut(githubAPIWrites[i].route, " ")
		if ruleMethod == method && slices.EqualFunc(strings.Split(pattern, "/"), rest, segmentMatches) {
			return &githubAPIWrites[i]
		}
	}
	return nil
}

func segmentMatches(pattern, segment string) bool {
	switch pattern {
	case "#":
		return segment != "" && strings.Trim(segment, "0123456789") == ""
	case "*":
		return segment != ""
	}
	return pattern == segment
}

// restWriteBodyRefusal checks an admitted REST write's body: empty or one JSON
// value whose objects repeat no key, for a review, an event that does not
// approve, and for a dispatch, the session branch as its ref.
func restWriteBodyRefusal(write githubAPIWrite, body []byte, repository, branch string) (int, string) {
	if write.dispatch {
		if !dispatchesOnTheBranch(body, branch) {
			return http.StatusForbidden, dispatchRefusal(repository, branch)
		}
		return 0, ""
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return 0, ""
	}
	if !uniqueKeyJSON(body) {
		return http.StatusBadRequest, "bex: a write's body must be one JSON value that repeats no key"
	}
	if write.review {
		var review map[string]any
		if json.Unmarshal(body, &review) != nil || !notApproval(review) {
			return http.StatusForbidden, writeRefusal(repository)
		}
	}
	return 0, ""
}

// dispatchesOnTheBranch admits a workflow dispatch body (w10/033): one JSON
// object, repeating no key, whose ref is the session branch, by name or as
// refs/heads/<branch>, beside at most an inputs object and gh's
// return_run_details flag. GitHub then runs the workflow file and the code the
// session pushed to that branch, as a push or pull_request run on it already
// does, and an environment's deployment-branch rules hold for it as for them.
// Any other ref, a tag or the default branch, could start a run of code the
// session did not write, with the inputs it chose.
func dispatchesOnTheBranch(body []byte, branch string) bool {
	var fields map[string]json.RawMessage
	if !uniqueKeyJSON(body) || json.Unmarshal(body, &fields) != nil {
		return false
	}
	var ref string
	if json.Unmarshal(fields["ref"], &ref) != nil || (ref != branch && ref != "refs/heads/"+branch) {
		return false
	}
	for name, raw := range fields {
		switch name {
		case "ref":
		case "inputs":
			var inputs map[string]json.RawMessage
			if json.Unmarshal(raw, &inputs) != nil || inputs == nil {
				return false
			}
		case "return_run_details":
			var flag bool
			if json.Unmarshal(raw, &flag) != nil {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// notApproval admits a review input whose event is absent, null or anything
// but APPROVE. An approval is refused: GitHub counts one by an App that has
// write access toward a branch's required reviews.
func notApproval(input map[string]any) bool {
	event, present := input["event"]
	if !present || event == nil {
		return true
	}
	text, ok := event.(string)
	return ok && !strings.EqualFold(strings.TrimSpace(text), "APPROVE")
}

// uniqueKeyJSON reports whether raw is exactly one JSON value whose objects
// never name a key twice. Two parsers can resolve a repeated key differently,
// and the route must read a body as GitHub will.
func uniqueKeyJSON(raw []byte) bool {
	type object struct {
		keys    map[string]bool
		wantKey bool
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var open []*object // nil for an array
	values := 0
	valueDone := func() {
		if len(open) == 0 {
			values++
		} else if top := open[len(open)-1]; top != nil {
			top.wantKey = true
		}
	}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return values == 1 && len(open) == 0
		} else if err != nil {
			return false
		}
		if n := len(open); n > 0 && open[n-1] != nil && open[n-1].wantKey && token != json.Delim('}') {
			key, _ := token.(string)
			if open[n-1].keys[key] {
				return false
			}
			open[n-1].keys[key], open[n-1].wantKey = true, false
			continue
		}
		switch token {
		case json.Delim('{'):
			open = append(open, &object{keys: map[string]bool{}, wantKey: true})
		case json.Delim('['):
			open = append(open, nil)
		case json.Delim('}'), json.Delim(']'):
			open = open[:len(open)-1]
			valueDone()
		default:
			valueDone()
		}
	}
}

// writeRefusal answers every write the route does not admit: what gh can
// write here, and the bex tools and the session branch for the rest.
func writeRefusal(repository string) string {
	return "bex: gh in this session reads " + repository + " and writes to it only to comment and reply, review without approving, " +
		"label, assign, request reviewers, open, edit and close issues, re-run or cancel Actions runs, and dispatch a workflow on your session branch. " +
		"Open or update the pull request with the open_or_update_draft_pr tool of the bex MCP server, reply on it with reply_on_pull_request, " +
		"mark it ready with mark_pull_request_ready and merge it with merge_pull_request when your task or a person asks, " +
		"and change code only by pushing your session branch."
}

// pullRequestStateRefusal answers gh pr ready and gh pr merge (w10/m538): a
// pull request changes state only through the bex tools, which act on the
// session's own pull request at the head bex recorded, or on the one of its
// repository whose number they are given (w10/041), then what else gh can
// write here. It names the merge tool's auto and disableAuto, which stand in
// for gh pr merge --auto, the merge queue and --disable-auto (w10/m577).
func pullRequestStateRefusal(repository string) string {
	return "bex: gh pr ready and gh pr merge do not work in this session; call the mark_pull_request_ready and merge_pull_request tools of the bex MCP server instead, " +
		"only when your task or a person asks, with \"number\" for a pull request other than this session's own. merge_pull_request with \"auto\": true arms auto-merge or joins the merge queue (for gh pr merge --auto), and with \"disableAuto\": true undoes it (for --disable-auto). " +
		strings.TrimPrefix(writeRefusal(repository), "bex: ")
}

// pullRequestStateMutations are the GraphQL mutations of gh pr ready, gh pr
// merge (with --auto and the merge queue too) and their undo, all refused
// with pullRequestStateRefusal.
var pullRequestStateMutations = map[string]bool{
	"markPullRequestReadyForReview": true, "convertPullRequestToDraft": true, "mergePullRequest": true,
	"enablePullRequestAutoMerge": true, "disablePullRequestAutoMerge": true, "enqueuePullRequest": true, "dequeuePullRequest": true,
}

// dispatchRefusal answers a workflow dispatch whose body is not one on the
// session branch: the command that is, then what else gh can write here.
func dispatchRefusal(repository, branch string) string {
	return "bex: gh in this session dispatches a workflow only on your session branch, as `gh workflow run <workflow> --ref " + branch + "`. " +
		strings.TrimPrefix(writeRefusal(repository), "bex: ")
}

func otherRepositoryRefusal(repository string) string {
	return "bex: gh in this session reads only " + repository + "; other repositories and GitHub API paths are not reachable from here."
}

// graphQLRefusal admits a GraphQL request only when every operation is a
// query whose top-level fields read the session's repository (its owner and
// name arguments, literal or variable, equal to it), a node by id, the rate
// limit or the schema, or a mutation whose every top-level field is an
// admitted write (githubAPIMutations). Any other mutation, a subscription, or
// a query field naming another part of GitHub refuses the whole document.
// Fragments are admitted where a field uses them; a fragment spread at the
// top level could hide any field, so it is refused. It answers whether the
// request writes, or the status and message of its refusal.
func graphQLRefusal(raw []byte, repository string) (bool, int, string) {
	var request struct {
		Query         string         `json:"query"`
		Variables     map[string]any `json:"variables"`
		OperationName string         `json:"operationName"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || decoder.More() || request.Query == "" || !uniqueKeyJSON(raw) {
		return false, http.StatusBadRequest, "bex: the GraphQL request must be one JSON object with a query, repeating no key"
	}
	document, err := parser.Parse(parser.ParseParams{Source: request.Query})
	if err != nil {
		return false, http.StatusBadRequest, "bex: the GraphQL query could not be parsed"
	}
	owner, name, _ := strings.Cut(repository, "/")
	write := false
	for _, definition := range document.Definitions {
		switch definition := definition.(type) {
		case *ast.FragmentDefinition:
		case *ast.OperationDefinition:
			defaults := map[string]ast.Value{}
			for _, variable := range definition.VariableDefinitions {
				if variable.DefaultValue != nil {
					defaults[variable.Variable.Name.Value] = variable.DefaultValue
				}
			}
			switch definition.Operation {
			case ast.OperationTypeQuery:
				for _, selection := range definition.SelectionSet.Selections {
					field, ok := selection.(*ast.Field)
					if !ok || !graphQLFieldAllowed(field, owner, name, request.Variables, defaults) {
						return false, http.StatusForbidden, otherRepositoryRefusal(repository)
					}
				}
			case ast.OperationTypeMutation:
				write = true
				for _, selection := range definition.SelectionSet.Selections {
					field, ok := selection.(*ast.Field)
					if ok && pullRequestStateMutations[field.Name.Value] {
						return false, http.StatusForbidden, pullRequestStateRefusal(repository)
					}
					if !ok || !graphQLMutationAllowed(field, request.Variables, defaults) {
						return false, http.StatusForbidden, writeRefusal(repository)
					}
				}
			default:
				return false, http.StatusForbidden, writeRefusal(repository)
			}
		default:
			return false, http.StatusBadRequest, "bex: the GraphQL request may hold only operations and fragments"
		}
	}
	return write, 0, ""
}

// githubAPIMutations are the GraphQL writes admitted (w10/m522), each with
// the check its input must pass, if any: what gh's pr comment, pr review, pr
// edit (labels, assignees, reviewers), issue create, comment, edit, close and
// reopen send, and the review-thread and reaction writes beside them. A node
// id is scoped by the write token's one-repository narrowing, which GitHub
// enforces, and GitHub resolves each id as its own type, so an issue mutation
// cannot reach a pull request.
var githubAPIMutations = map[string]func(map[string]any) bool{
	"__typename": nil,
	"addComment": nil, "updateIssueComment": nil, "deleteIssueComment": nil,
	"addPullRequestReview": notApproval, "submitPullRequestReview": notApproval,
	"updatePullRequestReview": nil, "deletePullRequestReview": nil,
	"addPullRequestReviewComment": nil, "addPullRequestReviewThread": nil, "addPullRequestReviewThreadReply": nil,
	"updatePullRequestReviewComment": nil, "deletePullRequestReviewComment": nil,
	"addReaction": nil, "removeReaction": nil,
	"addLabelsToLabelable": nil, "removeLabelsFromLabelable": nil,
	"requestReviews": nil, "requestReviewsByLogin": nil,
	"addAssigneesToAssignable": nil, "removeAssigneesFromAssignable": nil, "replaceActorsForAssignable": nil,
	"createIssue": nil, "updateIssue": nil, "closeIssue": nil, "reopenIssue": nil,
	"updatePullRequest": keepsThePullRequestRecord,
}

// keepsThePullRequestRecord admits an updatePullRequest that leaves the title,
// body, base and state alone, which the D3 verb owns: gh's `pr edit
// --add-reviewer` sends one naming only the pull request before it requests
// the reviews.
func keepsThePullRequestRecord(input map[string]any) bool {
	for name := range input {
		switch name {
		case "pullRequestId", "clientMutationId", "labelIds", "assigneeIds", "milestoneId":
		default:
			return false
		}
	}
	return true
}

// graphQLMutationAllowed admits one top-level mutation field of
// githubAPIMutations whose input, literal or from variables, passes its check.
func graphQLMutationAllowed(field *ast.Field, variables map[string]any, defaults map[string]ast.Value) bool {
	check, admitted := githubAPIMutations[field.Name.Value]
	if !admitted || check == nil {
		return admitted
	}
	if len(field.Arguments) != 1 || field.Arguments[0].Name.Value != "input" {
		return false
	}
	value, _ := graphQLValue(field.Arguments[0].Value, variables, defaults)
	input, ok := value.(map[string]any)
	return ok && check(input)
}

// graphQLValue resolves an argument value as GitHub reads it: literals, input
// objects (refusing a repeated field), lists, and variables from the request
// or their definition's default; an unsupplied variable is null.
func graphQLValue(value ast.Value, variables map[string]any, defaults map[string]ast.Value) (any, bool) {
	switch value := value.(type) {
	case *ast.Variable:
		if supplied, ok := variables[value.Name.Value]; ok {
			return supplied, true
		}
		if fallback, ok := defaults[value.Name.Value]; ok {
			return graphQLValue(fallback, nil, nil)
		}
		return nil, true
	case *ast.ObjectValue:
		out := map[string]any{}
		for _, field := range value.Fields {
			resolved, ok := graphQLValue(field.Value, variables, defaults)
			if _, repeated := out[field.Name.Value]; repeated || !ok {
				return nil, false
			}
			out[field.Name.Value] = resolved
		}
		return out, true
	case *ast.ListValue:
		out := make([]any, 0, len(value.Values))
		for _, item := range value.Values {
			resolved, ok := graphQLValue(item, variables, defaults)
			if !ok {
				return nil, false
			}
			out = append(out, resolved)
		}
		return out, true
	case *ast.StringValue:
		return value.Value, true
	case *ast.EnumValue:
		return value.Value, true
	case *ast.BooleanValue:
		return value.Value, true
	case *ast.IntValue:
		return value.Value, true
	case *ast.FloatValue:
		return value.Value, true
	}
	return nil, false
}

func graphQLFieldAllowed(field *ast.Field, owner, name string, variables map[string]any, defaults map[string]ast.Value) bool {
	switch field.Name.Value {
	case "node", "nodes", "rateLimit", "__typename", "__type", "__schema":
		return true
	case "repository":
	default:
		return false
	}
	ownerMatched, nameMatched := false, false
	for _, argument := range field.Arguments {
		var want string
		switch argument.Name.Value {
		case "owner":
			want, ownerMatched = owner, true
		case "name":
			want, nameMatched = name, true
		default:
			continue
		}
		value, ok := graphQLString(argument.Value, variables, defaults)
		if !ok || !strings.EqualFold(value, want) {
			return false
		}
	}
	return ownerMatched && nameMatched
}

// graphQLString resolves a string argument: a string literal, or a variable
// whose value from the request or its definition's default is a string.
func graphQLString(value ast.Value, variables map[string]any, defaults map[string]ast.Value) (string, bool) {
	switch value.(type) {
	case *ast.StringValue, *ast.Variable:
		resolved, _ := graphQLValue(value, variables, defaults)
		text, ok := resolved.(string)
		return text, ok
	}
	return "", false
}

// refuseGitHubAPI answers a refused call in GitHub's error shape, which gh
// prints as "HTTP <status>: <message>", counted under result.
func (b *Broker) refuseGitHubAPI(w http.ResponseWriter, claimed agentsession.MintRequest, result string, status int, message string) {
	b.Metrics.Authentication(result)
	log.Printf("agent github api: refused (session=%s repo=%s status=%d result=%s)", claimed.SessionID, claimed.Repository, status, result)
	writeGitHubAPIError(w, status, message)
}

func writeGitHubAPIError(w http.ResponseWriter, status int, message string) {
	agentsession.WriteNoStoreJSON(w, status, struct {
		Message string `json:"message"`
	}{message})
}

func (b *Broker) githubAPIBudgets() *sshgateway.ExchangeBudget {
	b.githubAPIBudgetOnce.Do(func() {
		b.githubAPIBudget = sshgateway.NewExchangeBudget(b.MaxGitHubAPIRequestsPerSession, b.MaxGitHubAPIRequestsPerWorkspace)
	})
	return b.githubAPIBudget
}
