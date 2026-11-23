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

package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The writes behind an agent session's mark-ready and merge verbs (ADR090
// D3, w10/m538 and w10/m577): GitHub undrafts a pull request only through
// GraphQL; the merge is the synchronous REST merge, pinned to the head bex
// recorded, with a method the repository allows; and GitHub auto-merge and
// the merge queue take a pull request through GraphQL, pinned the same way,
// as gh pr merge --auto does. Branch protection and rulesets decide whether
// it may happen; GitHub's refusal comes back as an *APIError whose message
// APIErrorMessage reads.

// The merge methods, in the order GitHub's settings and merge button list
// them; a merge that names none takes the first one the repository allows.
const (
	MergeMethodMerge  = "merge"
	MergeMethodSquash = "squash"
	MergeMethodRebase = "rebase"
)

// MergeMethods is every method GitHub's merge endpoint accepts, in order.
var MergeMethods = []string{MergeMethodMerge, MergeMethodSquash, MergeMethodRebase}

// statePermissions is the token of a mark-ready call and of disabling
// auto-merge or leaving the merge queue. GitHub's reference names no
// permission for these mutations, and installations with only
// pull_requests:write have been refused markPullRequestReadyForReview
// ("Resource not accessible by integration") while contents:write beside it
// works, so the token holds both. contents:write is what the session's Git
// credential already holds for the same repository.
func statePermissions() map[string]string {
	return map[string]string{"contents": "write", "pull_requests": "write"}
}

// mergePermissions is the token of a merge, and of arming auto-merge or the
// merge queue: Contents write is what GitHub lists for PUT …/pulls/{n}/merge
// (and the only way it shows a repository's merge settings), and Workflows
// write lets the merge land a pull request that changes .github/workflows/,
// as the session's push token does (w10/014). GitHub makes an auto-merge as
// the actor who armed it, so the arming token needs the same.
func mergePermissions() map[string]string {
	return map[string]string{"contents": "write", "pull_requests": "write", "workflows": "write"}
}

// ErrPermissionNotGranted refuses a mark-ready or merge whose token the
// installation cannot mint: it has not granted Contents or Pull requests
// write (GitHub answers the mint 422), which only an owner of the account can
// change.
var ErrPermissionNotGranted = errors.New("github: the installation has not granted the permissions this call needs")

// notGranted turns a mint GitHub refused with 422 into ErrPermissionNotGranted.
func notGranted(err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
		return ErrPermissionNotGranted
	}
	return err
}

// MergeOptions is one merge of a pull request. SHA pins it to the head bex
// recorded: GitHub refuses with 409 when the head moved. Method "" takes the
// first method the repository and its base branch's rules allow. Base is the
// pull request's base branch, whose rules are read for that choice.
type MergeOptions struct {
	SHA           string
	Method        string
	CommitTitle   string
	CommitMessage string
	Base          string
}

// MergeResult is a merged pull request: the commit its merge created on the
// base branch and the method that made it.
type MergeResult struct {
	SHA    string
	Method string
}

// MergeMethodError refuses a method the repository does not allow, before
// GitHub is asked to merge. Allowed lists the ones it does, in order.
type MergeMethodError struct {
	Method  string
	Allowed []string
}

func (e *MergeMethodError) Error() string {
	if e.Method == "" {
		return "github: the repository allows no merge method"
	}
	return fmt.Sprintf("github: merge method %q is not allowed", e.Method)
}

// MarkPullRequestReady undrafts the pull request whose GraphQL node id is
// nodeID, in repository repo of the installation. REST cannot undraft:
// `PATCH …/pulls/{n}` takes no draft field, and `gh pr ready` sends this same
// mutation. A GraphQL error comes back as an *APIError (gitHubGraphQL).
func (c *Client) MarkPullRequestReady(ctx context.Context, installationID int64, repo, nodeID string) error {
	token, err := c.repoToken(ctx, installationID, repo, statePermissions())
	if err != nil {
		return notGranted(err)
	}
	var out struct {
		Mark *struct {
			PullRequest struct {
				IsDraft bool `json:"isDraft"`
			} `json:"pullRequest"`
		} `json:"markPullRequestReadyForReview"`
	}
	if err := c.gitHubGraphQL(ctx, token, `mutation($id: ID!) { markPullRequestReadyForReview(input: {pullRequestId: $id}) { pullRequest { isDraft } } }`,
		map[string]any{"id": nodeID}, &out); err != nil {
		return err
	}
	if out.Mark == nil || out.Mark.PullRequest.IsDraft {
		return errors.New("github: the pull request is still a draft after markPullRequestReadyForReview")
	}
	return nil
}

// gitHubGraphQL sends one document to GitHub's GraphQL API with token and
// decodes its data into out. A GraphQL error comes back as an *APIError with
// GitHub's message as its body: RATE_LIMITED as a rate-limited 403,
// FORBIDDEN as 403, NOT_FOUND as 404, any other as 422.
func (c *Client) gitHubGraphQL(ctx context.Context, token, query string, variables map[string]any, out any) error {
	var answer struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	payload := map[string]any{"query": query, "variables": variables}
	if err := c.tokenJSON(ctx, token, http.MethodPost, c.baseURL+"/graphql", payload, http.StatusOK, &answer); err != nil {
		return err
	}
	if len(answer.Errors) > 0 {
		apiErr := &APIError{Status: http.StatusUnprocessableEntity}
		switch answer.Errors[0].Type {
		case "RATE_LIMITED":
			apiErr.Status, apiErr.RateLimited = http.StatusForbidden, true
		case "FORBIDDEN":
			apiErr.Status = http.StatusForbidden
		case "NOT_FOUND":
			apiErr.Status = http.StatusNotFound
		}
		body, _ := json.Marshal(map[string]string{"message": answer.Errors[0].Message})
		apiErr.Body = string(body)
		return apiErr
	}
	if len(answer.Data) == 0 || string(answer.Data) == "null" {
		return errors.New("github: the GraphQL answer carries no data")
	}
	return json.Unmarshal(answer.Data, out)
}

// MergePullRequest merges pull request number of owner/repo with the
// synchronous REST merge (PUT …/pulls/{n}/merge), whose answer is final: 200
// merged, 405 not mergeable (checks, reviews, conflicts, rules), 409 the head
// is not opts.SHA, 422 invalid, 403 not permitted. A branch that requires a
// merge queue takes the pull request through EnqueuePullRequest instead. A
// method the repository does not allow is a *MergeMethodError before any
// merge is sent. The token is mergeToken's.
func (c *Client) MergePullRequest(ctx context.Context, installationID int64, owner, repo string, number int, opts MergeOptions) (MergeResult, error) {
	token, err := c.mergeToken(ctx, installationID, repo)
	if err != nil {
		return MergeResult{}, err
	}
	settings, err := c.mergeSettings(ctx, token, owner, repo, opts.Base)
	if err != nil {
		return MergeResult{}, err
	}
	method, err := settings.method(opts.Method)
	if err != nil {
		return MergeResult{}, err
	}
	payload := map[string]string{"sha": opts.SHA, "merge_method": method}
	if opts.CommitTitle != "" {
		payload["commit_title"] = opts.CommitTitle
	}
	if opts.CommitMessage != "" {
		payload["commit_message"] = opts.CommitMessage
	}
	var out struct {
		SHA    string `json:"sha"`
		Merged bool   `json:"merged"`
	}
	if err := c.tokenJSON(ctx, token, http.MethodPut, fmt.Sprintf("%s/pulls/%d/merge", c.repoURL(owner, repo), number), payload, http.StatusOK, &out); err != nil {
		return MergeResult{}, err
	}
	if !out.Merged {
		return MergeResult{}, errors.New("github: the merge answered 200 without merging the pull request")
	}
	return MergeResult{SHA: out.SHA, Method: method}, nil
}

// mergeToken mints a merge's token, narrowed to the one repository, with
// mergePermissions; an installation that has not granted Workflows refuses
// that set with 422, so the mint retries without it and only a pull request
// that changes a workflow file is then refused.
func (c *Client) mergeToken(ctx context.Context, installationID int64, repo string) (string, error) {
	token, err := c.repoToken(ctx, installationID, repo, mergePermissions())
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
		permissions := mergePermissions()
		delete(permissions, "workflows")
		token, err = c.repoToken(ctx, installationID, repo, permissions)
	}
	if err != nil {
		return "", notGranted(err)
	}
	return token, nil
}

// repositoryMergeSettings is what a merge reads of the repository before it
// is sent: the methods it allows, and whether it allows auto-merge.
type repositoryMergeSettings struct {
	allowed []string
	// autoMergeOff is set only when GitHub said allow_auto_merge is false.
	autoMergeOff bool
}

// method is the merge method a merge naming named uses: named when the
// repository allows it, the first one it allows when named is "", and a
// *MergeMethodError otherwise.
func (s repositoryMergeSettings) method(named string) (string, error) {
	switch {
	case named == "" && len(s.allowed) == 0:
		return "", &MergeMethodError{}
	case named == "":
		return s.allowed[0], nil
	case !slices.Contains(s.allowed, named):
		return "", &MergeMethodError{Method: named, Allowed: s.allowed}
	}
	return named, nil
}

// mergeSettings reads the repository's merge settings (GitHub shows them
// only to a token with Contents write): the methods they allow less what the
// base branch's active rules forbid, a pull_request rule's
// allowed_merge_methods, and required_linear_history, which refuses a merge
// commit; and allow_auto_merge. Classic branch protection's linear-history
// setting needs Administration read to see, so GitHub's own refusal reports
// it.
func (c *Client) mergeSettings(ctx context.Context, token, owner, repo, base string) (repositoryMergeSettings, error) {
	var (
		settings struct {
			Merge     *bool `json:"allow_merge_commit"`
			Squash    *bool `json:"allow_squash_merge"`
			Rebase    *bool `json:"allow_rebase_merge"`
			AutoMerge *bool `json:"allow_auto_merge"`
		}
		rules []struct {
			Type       string `json:"type"`
			Parameters struct {
				AllowedMergeMethods []string `json:"allowed_merge_methods"`
			} `json:"parameters"`
		}
		settingsErr, rulesErr error
		wg                    sync.WaitGroup
	)
	wg.Go(func() {
		settingsErr = c.tokenJSON(ctx, token, http.MethodGet, c.repoURL(owner, repo), nil, http.StatusOK, &settings)
	})
	if base != "" {
		wg.Go(func() {
			rulesErr = c.tokenJSON(ctx, token, http.MethodGet, c.repoURL(owner, repo)+"/rules/branches/"+neturl.PathEscape(base), nil, http.StatusOK, &rules)
		})
	}
	wg.Wait()
	if settingsErr != nil {
		return repositoryMergeSettings{}, settingsErr
	}
	// A field GitHub left out is not a "no": the merge then reports its answer.
	setting := map[string]*bool{MergeMethodMerge: settings.Merge, MergeMethodSquash: settings.Squash, MergeMethodRebase: settings.Rebase}
	out := repositoryMergeSettings{
		allowed: slices.DeleteFunc(slices.Clone(MergeMethods), func(method string) bool {
			return setting[method] != nil && !*setting[method]
		}),
		autoMergeOff: settings.AutoMerge != nil && !*settings.AutoMerge,
	}
	// Rules that cannot be read leave the choice to the settings, and the
	// merge then reports what a rule refused.
	if rulesErr != nil {
		return out, nil
	}
	for _, rule := range rules {
		switch {
		case rule.Type == "required_linear_history":
			out.allowed = slices.DeleteFunc(out.allowed, func(method string) bool { return method == MergeMethodMerge })
		case rule.Type == "pull_request" && len(rule.Parameters.AllowedMergeMethods) > 0:
			out.allowed = slices.DeleteFunc(out.allowed, func(method string) bool {
				return !slices.Contains(rule.Parameters.AllowedMergeMethods, method)
			})
		}
	}
	return out, nil
}

// MergeState is what tells merging now, arming GitHub auto-merge and the
// merge queue apart (w10/m577): the fields gh pr merge reads before it
// chooses.
type MergeState struct {
	// MergeQueueEnabled is set when the base branch requires a merge queue
	// (GraphQL isMergeQueueEnabled, rulesets and classic protection alike).
	MergeQueueEnabled bool
	// InMergeQueue is set when the pull request is in the queue already.
	InMergeQueue bool
	// Status is GitHub's mergeStateStatus: CLEAN, UNSTABLE, HAS_HOOKS,
	// BLOCKED, BEHIND, DIRTY, DRAFT or UNKNOWN.
	Status string
	// AutoMerge is the auto-merge armed on the pull request, nil when none
	// is: its method in lower case, and who armed it when (w10/047).
	AutoMerge *store.PRAutoMerge
}

// MergeableNow is gh's isImmediatelyMergeable: GitHub would merge the pull
// request now, and refuses to arm auto-merge on it.
func (s MergeState) MergeableNow() bool {
	switch s.Status {
	case "CLEAN", "HAS_HOOKS", "UNSTABLE":
		return true
	}
	return false
}

// PullRequestMergeState reads the merge state of the pull request whose
// GraphQL node id is nodeID, in repository repo of the installation, with a
// Pull requests read token, as the PR read is.
func (c *Client) PullRequestMergeState(ctx context.Context, installationID int64, repo, nodeID string) (MergeState, error) {
	token, err := c.repoToken(ctx, installationID, repo, map[string]string{"pull_requests": "read"})
	if err != nil {
		return MergeState{}, err
	}
	var out struct {
		Node *struct {
			IsMergeQueueEnabled bool   `json:"isMergeQueueEnabled"`
			IsInMergeQueue      bool   `json:"isInMergeQueue"`
			MergeStateStatus    string `json:"mergeStateStatus"`
			AutoMergeRequest    *struct {
				MergeMethod string     `json:"mergeMethod"`
				EnabledAt   *time.Time `json:"enabledAt"`
				EnabledBy   *struct {
					Login string `json:"login"`
				} `json:"enabledBy"`
			} `json:"autoMergeRequest"`
		} `json:"node"`
	}
	if err := c.gitHubGraphQL(ctx, token, `query($id: ID!) { node(id: $id) { ... on PullRequest { isMergeQueueEnabled isInMergeQueue mergeStateStatus autoMergeRequest { mergeMethod enabledAt enabledBy { login } } } } }`,
		map[string]any{"id": nodeID}, &out); err != nil {
		return MergeState{}, err
	}
	if out.Node == nil {
		return MergeState{}, errors.New("github: the merge state read found no pull request")
	}
	state := MergeState{MergeQueueEnabled: out.Node.IsMergeQueueEnabled, InMergeQueue: out.Node.IsInMergeQueue, Status: out.Node.MergeStateStatus}
	if request := out.Node.AutoMergeRequest; request != nil {
		state.AutoMerge = &store.PRAutoMerge{Method: strings.ToLower(request.MergeMethod), EnabledAt: request.EnabledAt}
		if request.EnabledBy != nil {
			state.AutoMerge.EnabledBy = request.EnabledBy.Login
		}
	}
	return state, nil
}

// The refusals of arming auto-merge, in GitHub's words or bex's pre-check.
var (
	// ErrAutoMergeNotAllowed is a repository whose "Allow auto-merge"
	// setting is off: GitHub answers "Auto merge is not allowed for this
	// repository", and bex reads allow_auto_merge false before asking.
	ErrAutoMergeNotAllowed = errors.New("github: the repository does not allow auto-merge")
	// ErrMergeableNow is a pull request GitHub would merge now, which it
	// refuses to arm ("Pull request is in clean status"); mergeStateStatus
	// lags behind checks that just settled, so the read alone misses it.
	ErrMergeableNow = errors.New("github: the pull request is mergeable now, so auto-merge cannot be armed")
)

var (
	autoMergeNotAllowedRefusal = regexp.MustCompile(`(?i)auto[ -]?merge is not allowed`)
	mergeableNowRefusal        = regexp.MustCompile(`(?i)is in (clean|unstable|has[_ ]hooks) status`)
)

// autoMergeRefusal names GitHub's refusals of enablePullRequestAutoMerge that
// a caller acts on, and keeps every other as it came.
func autoMergeRefusal(err error) error {
	switch message := APIErrorMessage(err); {
	case autoMergeNotAllowedRefusal.MatchString(message):
		return fmt.Errorf("%w: %w", ErrAutoMergeNotAllowed, err)
	case mergeableNowRefusal.MatchString(message):
		return fmt.Errorf("%w: %w", ErrMergeableNow, err)
	}
	return err
}

// enableAutoMergeMutation is gh pr merge --auto's mutation, with the head
// pin gh leaves out.
const enableAutoMergeMutation = `mutation($input: EnablePullRequestAutoMergeInput!) { enablePullRequestAutoMerge(input: $input) { clientMutationId } }`

// EnablePullRequestAutoMerge arms GitHub auto-merge on the pull request
// whose node id is nodeID, in owner/repo, on a base branch without a merge
// queue: GitHub merges it once its required reviews and checks pass. It is
// pinned to opts.SHA (expectedHeadOid) and uses the method rule of
// MergePullRequest, with the commit text when given. A repository whose
// allow_auto_merge is false is ErrAutoMergeNotAllowed before any mutation;
// GitHub's own refusals are ErrAutoMergeNotAllowed, ErrMergeableNow or an
// *APIError. It answers the method armed.
func (c *Client) EnablePullRequestAutoMerge(ctx context.Context, installationID int64, owner, repo, nodeID string, opts MergeOptions) (string, error) {
	token, err := c.mergeToken(ctx, installationID, repo)
	if err != nil {
		return "", err
	}
	settings, err := c.mergeSettings(ctx, token, owner, repo, opts.Base)
	if err != nil {
		return "", err
	}
	if settings.autoMergeOff {
		return "", ErrAutoMergeNotAllowed
	}
	method, err := settings.method(opts.Method)
	if err != nil {
		return "", err
	}
	input := map[string]any{"pullRequestId": nodeID, "expectedHeadOid": opts.SHA, "mergeMethod": strings.ToUpper(method)}
	if opts.CommitTitle != "" {
		input["commitHeadline"] = opts.CommitTitle
	}
	if opts.CommitMessage != "" {
		input["commitBody"] = opts.CommitMessage
	}
	var out json.RawMessage
	if err := c.gitHubGraphQL(ctx, token, enableAutoMergeMutation, map[string]any{"input": input}, &out); err != nil {
		return "", autoMergeRefusal(err)
	}
	return method, nil
}

// EnqueuePullRequest takes the pull request whose node id is nodeID into its
// base branch's merge queue, pinned to headSHA, as gh pr merge does on a
// queue branch and GitHub's "Merge when ready" button: enablePullRequestAutoMerge
// adds a ready pull request to the queue and one whose checks are pending
// once they pass. The queue sets the method and the commit text, so none is
// sent. Where GitHub refuses that because the repository does not allow
// auto-merge or the pull request is ready now, it is enqueued directly
// (enqueuePullRequest), pinned the same way. A refusal is an *APIError.
func (c *Client) EnqueuePullRequest(ctx context.Context, installationID int64, repo, nodeID, headSHA string) error {
	token, err := c.mergeToken(ctx, installationID, repo)
	if err != nil {
		return err
	}
	input := map[string]any{"pullRequestId": nodeID, "expectedHeadOid": headSHA}
	var out json.RawMessage
	err = autoMergeRefusal(c.gitHubGraphQL(ctx, token, enableAutoMergeMutation, map[string]any{"input": input}, &out))
	if errors.Is(err, ErrAutoMergeNotAllowed) || errors.Is(err, ErrMergeableNow) {
		err = c.gitHubGraphQL(ctx, token, `mutation($input: EnqueuePullRequestInput!) { enqueuePullRequest(input: $input) { clientMutationId } }`,
			map[string]any{"input": input}, &out)
	}
	return err
}

// DisablePullRequestAutoMerge stops GitHub merging the pull request whose
// node id is nodeID later: it takes it out of the merge queue when queued
// (dequeuePullRequest), and otherwise disables its auto-merge
// (disablePullRequestAutoMerge), as Devin Review's merge button and
// gh pr merge --disable-auto do. A refusal is an *APIError.
func (c *Client) DisablePullRequestAutoMerge(ctx context.Context, installationID int64, repo, nodeID string, queued bool) error {
	token, err := c.repoToken(ctx, installationID, repo, statePermissions())
	if err != nil {
		return notGranted(err)
	}
	mutation, variables := `mutation($id: ID!) { disablePullRequestAutoMerge(input: {pullRequestId: $id}) { clientMutationId } }`, map[string]any{"id": nodeID}
	if queued {
		mutation = `mutation($id: ID!) { dequeuePullRequest(input: {id: $id}) { clientMutationId } }`
	}
	var out json.RawMessage
	return c.gitHubGraphQL(ctx, token, mutation, variables, &out)
}

// maxAPIErrorMessageRunes bounds the GitHub message APIErrorMessage returns.
const maxAPIErrorMessageRunes = 1000

// APIErrorMessage is GitHub's own explanation of a refusal: the "message" of
// its error body, followed by each entry of its "errors" list, on one line and
// bounded. It is "" for an error that is not an *APIError or carries no
// message. Callers relay it to the agent whose own pull request GitHub
// refused (w10/m538), which is why Error() alone never carries it (w6/005).
func APIErrorMessage(err error) string {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return ""
	}
	var body struct {
		Message string            `json:"message"`
		Errors  []json.RawMessage `json:"errors"`
	}
	if json.Unmarshal([]byte(apiErr.Body), &body) != nil {
		return ""
	}
	parts := []string{body.Message}
	for _, raw := range body.Errors {
		var entry struct {
			Message string `json:"message"`
		}
		var text string
		switch {
		case json.Unmarshal(raw, &entry) == nil && entry.Message != "":
			parts = append(parts, entry.Message)
		case json.Unmarshal(raw, &text) == nil && text != "":
			parts = append(parts, text)
		}
	}
	message := strings.ToValidUTF8(strings.Join(strings.Fields(strings.Join(parts, " ")), " "), "")
	if utf8.RuneCountInString(message) > maxAPIErrorMessageRunes {
		message = string([]rune(message)[:maxAPIErrorMessageRunes]) + "…"
	}
	return message
}
