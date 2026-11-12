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
)

// The reads and the one write the PR feedback loop needs (ADR047 D11): who
// may steer (the commenter's repository permission), what a review said
// (its inline comments), why CI failed (a suite's check runs), and the
// agent's reply on its own pull request.

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
	permissions["metadata"] = "read"
	tok, err := c.mintInstallationToken(ctx, installationID, struct {
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}{Repositories: []string{repo}, Permissions: permissions})
	if err != nil {
		return err
	}
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
	setTokenAuth(req, tok.Token)
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
