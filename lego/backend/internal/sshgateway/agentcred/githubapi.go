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
	"strings"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// The session's read-only GitHub API route (w10/m513, ADR090 D4 amendment).
// The in-sandbox `gh` sends every call to a socket the driver serves, and the
// driver forwards it here, under the session's Pod-bound Git proxy URL:
// <repo URL>/github-api/<REST path> or /github-api/graphql. The route
//
//   - admits REST GET and HEAD on /repos/<the session's repository>[/…] only,
//     and POST /graphql only for queries whose top-level fields read that
//     repository, a node by id, the rate limit or the schema;
//   - refuses every write in GitHub's error shape, with a message naming the
//     bex MCP tools, which stay the session's one pull-request path;
//   - mints a read-only installation token narrowed to the repository for
//     each call and injects it on the upstream hop only. A node lookup by id
//     is scoped by that narrowing, which GitHub enforces;
//   - follows GitHub's redirects to its log and archive storage itself,
//     without the token, so `gh run view --log` works with no other egress.
const (
	defaultGitHubAPIOrigin = "https://api.github.com"
	// maxGitHubGraphQLBytes bounds one GraphQL request body: gh's largest
	// query is a few KiB.
	maxGitHubGraphQLBytes = 256 << 10
	// maxGitHubAPIResponseBytes bounds one streamed response, a run's log
	// archive included.
	maxGitHubAPIResponseBytes = 1 << 30
	// maxGitHubAPIRedirects bounds the storage redirects one read follows.
	maxGitHubAPIRedirects = 3
	githubAPIUserAgent    = "bex-agent-github-api-proxy/1"
)

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

// serveGitHubAPI forwards one read of the session's repository (see above).
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
	if status, refusal := githubAPIRequestRefusal(r.Method, apiPath, repository); refusal != "" {
		b.refuseGitHubAPI(w, claimed, status, refusal)
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
	// The body is read only once admitted, under the admission's slot and
	// timeout, as the PR verbs read theirs.
	var graphql []byte
	if apiPath == "/graphql" {
		graphql, err = io.ReadAll(http.MaxBytesReader(w, r.Body, maxGitHubGraphQLBytes))
		if tooLarge := new(http.MaxBytesError); errors.As(err, &tooLarge) {
			writeGitHubAPIError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("bex: a GraphQL request is limited to %d KiB", maxGitHubGraphQLBytes>>10))
			return
		} else if err != nil {
			writeGitHubAPIError(w, http.StatusBadRequest, "bex: the GraphQL request could not be read")
			return
		}
		if status, refusal := graphQLRefusal(graphql, repository); refusal != "" {
			b.refuseGitHubAPI(w, claimed, status, refusal)
			return
		}
	}
	credential, err := b.GitHubRead.Mint(r.Context(), verified)
	if err != nil {
		if errors.Is(err, agentsession.ErrForbidden) {
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
	var body io.Reader
	if graphql != nil {
		body = bytes.NewReader(graphql)
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, target, body)
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
			return nil, errors.New("GitHub redirected a GraphQL request")
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

// githubAPIRequestRefusal is the method and path policy: REST reads of the
// session's repository and GraphQL POSTs. It answers the status and message
// of a refusal, or "" for an admitted request.
func githubAPIRequestRefusal(method, apiPath, repository string) (int, string) {
	lower := strings.ToLower(apiPath)
	for _, bad := range []string{"%2f", "%5c", "%2e", "%00", "\\", "//"} {
		if strings.Contains(lower, bad) {
			return http.StatusBadRequest, "bex: invalid request path"
		}
	}
	segments := strings.Split(strings.TrimPrefix(apiPath, "/"), "/")
	for _, segment := range segments {
		if segment == "." || segment == ".." {
			return http.StatusBadRequest, "bex: invalid request path"
		}
	}
	if apiPath == "/graphql" {
		if method != http.MethodPost {
			return http.StatusMethodNotAllowed, readOnlyRefusal(repository)
		}
		return 0, ""
	}
	if method != http.MethodGet && method != http.MethodHead {
		return http.StatusForbidden, readOnlyRefusal(repository)
	}
	if len(segments) < 3 || segments[0] != "repos" || !strings.EqualFold(segments[1]+"/"+segments[2], repository) {
		return http.StatusForbidden, otherRepositoryRefusal(repository)
	}
	return 0, ""
}

// readOnlyRefusal is the answer to every write: the bex tools stay the
// session's one way to open, update or answer its pull request.
func readOnlyRefusal(repository string) string {
	return "bex: gh in this session only reads " + repository + " (CI runs and logs, checks, pull requests and contents). " +
		"Open or update the pull request with the open_or_update_draft_pr tool of the bex MCP server, and reply on it with reply_on_pull_request."
}

func otherRepositoryRefusal(repository string) string {
	return "bex: gh in this session reads only " + repository + "; other repositories and GitHub API paths are not reachable from here."
}

// graphQLRefusal admits a GraphQL request only when every operation is a
// query whose top-level fields read the session's repository (its owner and
// name arguments, literal or variable, equal to it), a node by id, the rate
// limit or the schema. A mutation is a write; anything else names another
// part of GitHub. Fragments are admitted where a field uses them; a fragment
// spread at the top level could hide any field, so it is refused.
func graphQLRefusal(raw []byte, repository string) (int, string) {
	var request struct {
		Query         string         `json:"query"`
		Variables     map[string]any `json:"variables"`
		OperationName string         `json:"operationName"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || decoder.More() || request.Query == "" {
		return http.StatusBadRequest, "bex: the GraphQL request must be one JSON object with a query"
	}
	document, err := parser.Parse(parser.ParseParams{Source: request.Query})
	if err != nil {
		return http.StatusBadRequest, "bex: the GraphQL query could not be parsed"
	}
	owner, name, _ := strings.Cut(repository, "/")
	for _, definition := range document.Definitions {
		switch definition := definition.(type) {
		case *ast.FragmentDefinition:
		case *ast.OperationDefinition:
			if definition.Operation != ast.OperationTypeQuery {
				return http.StatusForbidden, readOnlyRefusal(repository)
			}
			defaults := map[string]ast.Value{}
			for _, variable := range definition.VariableDefinitions {
				if variable.DefaultValue != nil {
					defaults[variable.Variable.Name.Value] = variable.DefaultValue
				}
			}
			for _, selection := range definition.SelectionSet.Selections {
				field, ok := selection.(*ast.Field)
				if !ok || !graphQLFieldAllowed(field, owner, name, request.Variables, defaults) {
					return http.StatusForbidden, otherRepositoryRefusal(repository)
				}
			}
		default:
			return http.StatusBadRequest, "bex: the GraphQL request may hold only queries and fragments"
		}
	}
	return 0, ""
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

// graphQLString resolves a string argument: a literal, or a variable from the
// request's variables or its definition's default.
func graphQLString(value ast.Value, variables map[string]any, defaults map[string]ast.Value) (string, bool) {
	switch value := value.(type) {
	case *ast.StringValue:
		return value.Value, true
	case *ast.Variable:
		if supplied, ok := variables[value.Name.Value]; ok {
			text, isString := supplied.(string)
			return text, isString
		}
		if fallback, ok := defaults[value.Name.Value].(*ast.StringValue); ok {
			return fallback.Value, true
		}
	}
	return "", false
}

// refuseGitHubAPI answers a refused method, path or query in GitHub's error
// shape, which gh prints as "HTTP <status>: <message>".
func (b *Broker) refuseGitHubAPI(w http.ResponseWriter, claimed agentsession.MintRequest, status int, message string) {
	b.Metrics.Authentication("rejected_target")
	log.Printf("agent github api: refused (session=%s repo=%s status=%d)", claimed.SessionID, claimed.Repository, status)
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
