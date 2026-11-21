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
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// The writes behind an agent session's mark-ready and merge verbs (ADR090
// D3, w10/m538): GitHub undrafts a pull request only through GraphQL, and the
// merge is the synchronous REST merge, pinned to the head bex recorded, with a
// method the repository allows. Branch protection and rulesets decide whether
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

// readyPermissions is the token of a mark-ready call. GitHub's reference
// names no permission for markPullRequestReadyForReview, and installations
// with only pull_requests:write have been refused it ("Resource not
// accessible by integration") while contents:write beside it works, so the
// token holds both. contents:write is what the session's Git credential
// already holds for the same repository.
func readyPermissions() map[string]string {
	return map[string]string{"contents": "write", "pull_requests": "write"}
}

// mergePermissions is the token of a merge: Contents write is what GitHub
// lists for PUT …/pulls/{n}/merge (and the only way it shows a repository's
// merge settings), and Workflows write lets the merge land a pull request that
// changes .github/workflows/, as the session's push token does (w10/014).
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
// mutation. A GraphQL error comes back as an *APIError: FORBIDDEN as 403,
// NOT_FOUND as 404, any other as 422, with GitHub's body.
func (c *Client) MarkPullRequestReady(ctx context.Context, installationID int64, repo, nodeID string) error {
	token, err := c.repoToken(ctx, installationID, repo, readyPermissions())
	if err != nil {
		return notGranted(err)
	}
	var out struct {
		Data struct {
			Mark *struct {
				PullRequest struct {
					IsDraft bool `json:"isDraft"`
				} `json:"pullRequest"`
			} `json:"markPullRequestReadyForReview"`
		} `json:"data"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	payload := map[string]any{
		"query":     `mutation($id: ID!) { markPullRequestReadyForReview(input: {pullRequestId: $id}) { pullRequest { isDraft } } }`,
		"variables": map[string]string{"id": nodeID},
	}
	if err := c.tokenJSON(ctx, token, http.MethodPost, c.baseURL+"/graphql", payload, http.StatusOK, &out); err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		status := http.StatusUnprocessableEntity
		switch out.Errors[0].Type {
		case "FORBIDDEN":
			status = http.StatusForbidden
		case "NOT_FOUND":
			status = http.StatusNotFound
		}
		body, _ := json.Marshal(map[string]string{"message": out.Errors[0].Message})
		return &APIError{Status: status, Body: string(body)}
	}
	if out.Data.Mark == nil || out.Data.Mark.PullRequest.IsDraft {
		return errors.New("github: the pull request is still a draft after markPullRequestReadyForReview")
	}
	return nil
}

// MergePullRequest merges pull request number of owner/repo with the
// synchronous REST merge (PUT …/pulls/{n}/merge), whose answer is final: 200
// merged, 405 not mergeable (checks, reviews, conflicts, rules), 409 the head
// is not opts.SHA, 422 invalid, 403 not permitted. The asynchronous
// merge-async endpoint, which GitHub now recommends, answers 202 and must be
// polled, and is what a merge queue needs; a repository that requires a merge
// queue therefore gets GitHub's own refusal here. A method the repository
// does not allow is a *MergeMethodError before any merge is sent. The token
// is minted for this call, narrowed to the one repository, with Contents,
// Pull requests and Workflows write; an installation that has not granted
// Workflows refuses that set with 422, so the mint retries without it and
// only a pull request that changes a workflow file is then refused.
func (c *Client) MergePullRequest(ctx context.Context, installationID int64, owner, repo string, number int, opts MergeOptions) (MergeResult, error) {
	token, err := c.repoToken(ctx, installationID, repo, mergePermissions())
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
		permissions := mergePermissions()
		delete(permissions, "workflows")
		token, err = c.repoToken(ctx, installationID, repo, permissions)
	}
	if err != nil {
		return MergeResult{}, notGranted(err)
	}
	allowed, err := c.allowedMergeMethods(ctx, token, owner, repo, opts.Base)
	if err != nil {
		return MergeResult{}, err
	}
	method := opts.Method
	switch {
	case method == "" && len(allowed) == 0:
		return MergeResult{}, &MergeMethodError{}
	case method == "":
		method = allowed[0]
	case !slices.Contains(allowed, method):
		return MergeResult{}, &MergeMethodError{Method: method, Allowed: allowed}
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

// allowedMergeMethods is what the repository's merge settings allow (GitHub
// shows them only to a token with Contents write) less what the base branch's
// active rules forbid: a pull_request rule's allowed_merge_methods, and
// required_linear_history, which refuses a merge commit. Classic branch
// protection's linear-history setting needs Administration read to see, so
// GitHub's own refusal reports it.
func (c *Client) allowedMergeMethods(ctx context.Context, token, owner, repo, base string) ([]string, error) {
	var (
		settings struct {
			Merge  *bool `json:"allow_merge_commit"`
			Squash *bool `json:"allow_squash_merge"`
			Rebase *bool `json:"allow_rebase_merge"`
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
		return nil, settingsErr
	}
	// A field GitHub left out is not a "no": the merge then reports its answer.
	setting := map[string]*bool{MergeMethodMerge: settings.Merge, MergeMethodSquash: settings.Squash, MergeMethodRebase: settings.Rebase}
	allowed := slices.DeleteFunc(slices.Clone(MergeMethods), func(method string) bool {
		return setting[method] != nil && !*setting[method]
	})
	// Rules that cannot be read leave the choice to the settings, and the
	// merge then reports what a rule refused.
	if rulesErr != nil {
		return allowed, nil
	}
	for _, rule := range rules {
		switch {
		case rule.Type == "required_linear_history":
			allowed = slices.DeleteFunc(allowed, func(method string) bool { return method == MergeMethodMerge })
		case rule.Type == "pull_request" && len(rule.Parameters.AllowedMergeMethods) > 0:
			allowed = slices.DeleteFunc(allowed, func(method string) bool {
				return !slices.Contains(rule.Parameters.AllowedMergeMethods, method)
			})
		}
	}
	return allowed, nil
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
