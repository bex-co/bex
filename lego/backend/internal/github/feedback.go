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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The reads and writes the PR feedback loop needs (ADR047 D11): who may steer
// (the commenter's repository permission), what a review said (its body and
// inline comments), why CI failed (a suite's check runs), and the agent's
// reply on its own pull request. A `/bex` takeover also reads the pull
// request, acknowledges the comment with a reaction, and fast-forwards the
// pull request's branch to the session's.

// maxFeedbackItems bounds one review's comments and one suite's check runs.
const maxFeedbackItems = 100

// ReviewComment is one inline comment of a pull request review.
type ReviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
	URL  string `json:"html_url"`
}

// repoJSON mints a token for the one repository with only permissions (plus
// metadata:read), sends one request and decodes a wantStatus answer into out
// (nil skips the body). Any other status is an *APIError.
func (c *Client) repoJSON(ctx context.Context, installationID int64, repo string, permissions map[string]string, method, url string, payload any, wantStatus int, out any) error {
	token, err := c.repoToken(ctx, installationID, repo, permissions)
	if err != nil {
		return err
	}
	return c.tokenJSON(ctx, token, method, url, payload, wantStatus, out)
}

// repoToken mints an installation token for the one repository with only
// permissions plus metadata:read.
func (c *Client) repoToken(ctx context.Context, installationID int64, repo string, permissions map[string]string) (string, error) {
	permissions["metadata"] = "read"
	tok, err := c.mintInstallationToken(ctx, installationID, struct {
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}{Repositories: []string{repo}, Permissions: permissions})
	return tok.Token, err
}

// tokenJSON is repoJSON's request half, for a caller that already holds a token.
func (c *Client) tokenJSON(ctx context.Context, token, method, url string, payload any, wantStatus int, out any) error {
	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	setTokenAuth(req, token)
	req.Header.Set("Accept", acceptHeader)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != wantStatus {
		return newAPIError(resp, body)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("github: decode %s: %w", url, err)
	}
	return nil
}

func (c *Client) repoURL(owner, repo string) string {
	return c.baseURL + "/repos/" + neturl.PathEscape(owner) + "/" + neturl.PathEscape(repo)
}

// CollaboratorPermission returns login's permission on owner/repo as GitHub's
// legacy four levels: admin, write (which maintain maps to), read or none. A
// custom repository role maps onto them too, which role_name would not. A
// user who is not a collaborator is none.
func (c *Client) CollaboratorPermission(ctx context.Context, installationID int64, owner, repo, login string) (string, error) {
	var out struct {
		Permission string `json:"permission"`
	}
	err := c.repoJSON(ctx, installationID, repo, map[string]string{}, http.MethodGet,
		c.repoURL(owner, repo)+"/collaborators/"+neturl.PathEscape(login)+"/permission", nil, http.StatusOK, &out)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return "none", nil
	}
	return out.Permission, err
}

// ReviewComments lists one review's inline comments (at most maxFeedbackItems).
func (c *Client) ReviewComments(ctx context.Context, installationID int64, owner, repo string, number int, reviewID int64) ([]ReviewComment, error) {
	var out []ReviewComment
	err := c.repoJSON(ctx, installationID, repo, map[string]string{"pull_requests": "read"}, http.MethodGet,
		fmt.Sprintf("%s/pulls/%d/reviews/%d/comments?per_page=%d", c.repoURL(owner, repo), number, reviewID, maxFeedbackItems),
		nil, http.StatusOK, &out)
	return out, err
}

// SuiteCheckRuns lists a check suite's check runs (at most maxFeedbackItems).
func (c *Client) SuiteCheckRuns(ctx context.Context, installationID int64, owner, repo string, suiteID int64) ([]CheckRun, error) {
	var out struct {
		CheckRuns []CheckRun `json:"check_runs"`
	}
	err := c.repoJSON(ctx, installationID, repo, map[string]string{"checks": "read"}, http.MethodGet,
		c.repoURL(owner, repo)+"/check-suites/"+strconv.FormatInt(suiteID, 10)+"/check-runs?per_page="+strconv.Itoa(maxFeedbackItems),
		nil, http.StatusOK, &out)
	return out.CheckRuns, err
}

// PullRequestDetail is what a `/bex` takeover and a takeover
// session's PR verb read of one pull request: its state, head and base.
type PullRequestDetail struct {
	PullRequest
	Title string
	// Merged is set on a merged pull request, whose State is closed.
	Merged  bool
	HeadRef string
	HeadSHA string
	// HeadRepo is the head repository's owner/name, empty when the fork the
	// pull request came from was deleted. BaseRepo is the repository it
	// targets, whose default branch is DefaultBranch.
	HeadRepo      string
	BaseRepo      string
	DefaultBranch string
}

// GetPullRequest reads pull request number of owner/repo.
func (c *Client) GetPullRequest(ctx context.Context, installationID int64, owner, repo string, number int) (PullRequestDetail, error) {
	var out struct {
		ghPull
		Title  string `json:"title"`
		Merged bool   `json:"merged"`
		Head   struct {
			Ref  string  `json:"ref"`
			SHA  string  `json:"sha"`
			Repo *ghRepo `json:"repo"`
		} `json:"head"`
		Base struct {
			Repo ghRepo `json:"repo"`
		} `json:"base"`
	}
	err := c.repoJSON(ctx, installationID, repo, map[string]string{"pull_requests": "read"}, http.MethodGet,
		fmt.Sprintf("%s/pulls/%d", c.repoURL(owner, repo), number), nil, http.StatusOK, &out)
	if err != nil {
		return PullRequestDetail{}, err
	}
	detail := PullRequestDetail{
		PullRequest: out.export(), Title: out.Title, Merged: out.Merged,
		HeadRef: out.Head.Ref, HeadSHA: out.Head.SHA, BaseRepo: out.Base.Repo.FullName, DefaultBranch: out.Base.Repo.DefaultBranch,
	}
	if out.Head.Repo != nil {
		detail.HeadRepo = out.Head.Repo.FullName
	}
	return detail, nil
}

// Refusals of FastForwardBranch.
var (
	// ErrNotFastForward means from's head does not descend from branch's: the
	// branch moved on, so a merge into from has to come first.
	ErrNotFastForward = errors.New("github: the branch update is not a fast-forward")
	// ErrBranchUpdateRefused means GitHub refused a fast-forward itself: a
	// branch protection rule or ruleset forbids the App to update the branch.
	ErrBranchUpdateRefused = errors.New("github: a rule on the branch refused the update")
)

// FastForwardBranch moves branch of owner/repo to from's head commit and
// returns that commit. It compares the two first, so only a real
// fast-forward is attempted (a branch already there is left as it is), and
// it sends force:false, so GitHub refuses anything else too; history on
// branch is never rewritten. ErrHeadBranchNotFound means from does not exist
// on GitHub. One token, narrowed to the repository with contents:write,
// serves every call.
func (c *Client) FastForwardBranch(ctx context.Context, installationID int64, owner, repo, branch, from string) (string, error) {
	token, err := c.repoToken(ctx, installationID, repo, map[string]string{"contents": "write"})
	if err != nil {
		return "", err
	}
	sha, err := c.GetRepoCommitSHA(ctx, token, owner, repo, from)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return "", ErrHeadBranchNotFound
	}
	if err != nil {
		return "", err
	}
	relation, err := c.compare(ctx, token, owner, repo, branch, sha)
	if err != nil {
		return "", err
	}
	switch relation.Status {
	case "identical":
		return sha, nil
	case "ahead":
	default: // behind or diverged: branch has commits from lacks
		return "", ErrNotFastForward
	}
	segments := strings.Split(branch, "/")
	for i, segment := range segments {
		segments[i] = neturl.PathEscape(segment)
	}
	err = c.tokenJSON(ctx, token, http.MethodPatch, c.repoURL(owner, repo)+"/git/refs/heads/"+strings.Join(segments, "/"),
		map[string]any{"sha": sha, "force": false}, http.StatusOK, nil)
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
		// The compare said fast-forward, so a refusal now is a branch rule
		// (or a race with a push, which the next call sees as diverged).
		return "", ErrBranchUpdateRefused
	}
	return sha, err
}

// BranchHead returns branch's head commit on owner/repo, read with a
// contents:read token. ErrHeadBranchNotFound means the branch does not exist.
func (c *Client) BranchHead(ctx context.Context, installationID int64, owner, repo, branch string) (string, error) {
	var out struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	err := c.repoJSON(ctx, installationID, repo, map[string]string{"contents": "read"}, http.MethodGet,
		c.repoURL(owner, repo)+"/branches/"+neturl.PathEscape(branch), nil, http.StatusOK, &out)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return "", ErrHeadBranchNotFound
	}
	return out.Commit.SHA, err
}

// BranchProtected reports whether GitHub marks branch of owner/repo
// protected.
func (c *Client) BranchProtected(ctx context.Context, installationID int64, owner, repo, branch string) (bool, error) {
	var out struct {
		Protected bool `json:"protected"`
	}
	err := c.repoJSON(ctx, installationID, repo, map[string]string{"contents": "read"}, http.MethodGet,
		c.repoURL(owner, repo)+"/branches/"+neturl.PathEscape(branch), nil, http.StatusOK, &out)
	return out.Protected, err
}

// Review is one submitted pull request review. State is lowercase, as
// webhook payloads spell it (commented, changes_requested, approved).
type Review struct {
	Body  string
	State string
	URL   string
}

// GetReview reads review reviewID of pull request number.
func (c *Client) GetReview(ctx context.Context, installationID int64, owner, repo string, number int, reviewID int64) (Review, error) {
	var out struct {
		Body  string `json:"body"`
		State string `json:"state"`
		URL   string `json:"html_url"`
	}
	err := c.repoJSON(ctx, installationID, repo, map[string]string{"pull_requests": "read"}, http.MethodGet,
		fmt.Sprintf("%s/pulls/%d/reviews/%d", c.repoURL(owner, repo), number, reviewID), nil, http.StatusOK, &out)
	return Review{Body: out.Body, State: strings.ToLower(out.State), URL: out.URL}, err
}

// ReactionEyes is the reaction that acknowledges a `/bex` comment.
const ReactionEyes = "eyes"

// bexCommand is the word that must open a comment for it to be a `/bex`
// command, in any case.
const bexCommand = "/bex"

// BexCommand reads a `/bex` comment: the text after the command word, which
// must open the comment (in any case) and stand alone. ok is false for any
// other comment. The webhook's prefilter and the PR intake share it.
func BexCommand(body string) (task string, ok bool) {
	body = strings.TrimLeftFunc(body, unicode.IsSpace)
	if len(body) < len(bexCommand) || !strings.EqualFold(body[:len(bexCommand)], bexCommand) {
		return "", false
	}
	rest := body[len(bexCommand):]
	if first, _ := utf8.DecodeRuneInString(rest); rest != "" && !unicode.IsSpace(first) {
		return "", false // "/bexify" is not the command
	}
	return strings.TrimSpace(rest), true
}

// AddCommentReaction reacts with content to a pull request comment: an issue
// comment, or an inline review comment when inline is set. GitHub answers 200
// instead of 201 when the App already reacted, which is not an error.
func (c *Client) AddCommentReaction(ctx context.Context, installationID int64, owner, repo string, commentID int64, inline bool, content string) error {
	kind := "issues"
	if inline {
		kind = "pulls"
	}
	err := c.repoJSON(ctx, installationID, repo, map[string]string{"pull_requests": "write"}, http.MethodPost,
		fmt.Sprintf("%s/%s/comments/%d/reactions", c.repoURL(owner, repo), kind, commentID),
		map[string]string{"content": content}, http.StatusCreated, nil)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusOK {
		return nil
	}
	return err
}

// CreateIssueComment posts body on issue or pull request number and returns
// the comment's URL. GitHub accepts pull_requests:write for a pull request.
func (c *Client) CreateIssueComment(ctx context.Context, installationID int64, owner, repo string, number int, body string) (string, error) {
	var out struct {
		URL string `json:"html_url"`
	}
	err := c.repoJSON(ctx, installationID, repo, map[string]string{"pull_requests": "write"}, http.MethodPost,
		fmt.Sprintf("%s/issues/%d/comments", c.repoURL(owner, repo), number), map[string]string{"body": body}, http.StatusCreated, &out)
	return out.URL, err
}
