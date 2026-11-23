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
	"strings"
	"sync"
)

// PullRequest is the subset of a GitHub pull request the agent-session PR verb
// records (ADR047 D4, ADR090). JSON tags are bex-api's camelCase shape, not
// GitHub's snake_case wire shape.
type PullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"htmlUrl"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
}

// ghPull is GitHub's wire shape.
type ghPull struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
}

func (p ghPull) export() PullRequest {
	return PullRequest{Number: p.Number, HTMLURL: p.HTMLURL, State: p.State, Draft: p.Draft}
}

// Typed refusals of SyncPullRequest: the agent's PR request is refused, not
// failed, when there is nothing reviewable on its branch yet (ADR090), or when
// it asked for a draft the repository cannot have.
var (
	// ErrHeadBranchNotFound means the head branch does not exist on GitHub —
	// the agent has not pushed it.
	ErrHeadBranchNotFound = errors.New("github: head branch does not exist")
	// ErrHeadNotAhead means the head branch has no commits that base lacks.
	ErrHeadNotAhead = errors.New("github: head branch is not ahead of base")
	// ErrDraftNotSupported means GitHub refused to open a draft because the
	// repository's plan has none (a private repository on GitHub Free or Pro).
	ErrDraftNotSupported = errors.New("github: draft pull requests are not supported in this repository")
)

// PullRequestSyncResult is the outcome of SyncPullRequest. HeadSHA is the
// remote head the PR now points at.
type PullRequestSyncResult struct {
	PullRequest
	Created bool
	HeadSHA string
}

// SyncPullRequest opens a PR from head into the repository's default branch,
// ready for review unless draft asks for a draft (ADR090 D3, w10/040: Devin
// opens a regular pull request by default), or — when an open PR for head
// already exists — updates that PR's title and body (ADR090: one branch, one
// PR per session, so a steering turn updates the same PR). draft is read only
// when the PR is opened: it never changes draft state, base, or head of an
// existing PR. It refuses with ErrHeadBranchNotFound when head is not on
// GitHub, ErrHeadNotAhead when head has no commits base lacks, and
// ErrDraftNotSupported when GitHub refuses the draft. The installation token
// is minted per call, narrowed to the one repository with pull_requests:write
// + contents:read, and never returned to the caller.
func (c *Client) SyncPullRequest(ctx context.Context, installationID int64, owner, repo, head, title, body string, draft bool) (PullRequestSyncResult, error) {
	tok, err := c.mintInstallationToken(ctx, installationID, struct {
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}{
		Repositories: []string{repo},
		Permissions:  map[string]string{"pull_requests": "write", "contents": "read", "metadata": "read"},
	})
	if err != nil {
		return PullRequestSyncResult{}, err
	}
	// The repository, the head commit, and any open PR for head are independent
	// reads; fetch them together, then judge their errors in a fixed order.
	var (
		wg                         sync.WaitGroup
		info                       Repo
		found                      bool
		headSHA                    string
		existing                   PullRequest
		repoErr, headErr, existErr error
	)
	wg.Go(func() { info, found, repoErr = c.LookupRepo(ctx, tok.Token, owner, repo) })
	wg.Go(func() { headSHA, headErr = c.GetRepoCommitSHA(ctx, tok.Token, owner, repo, head) })
	wg.Go(func() { existing, existErr = c.findOpenPull(ctx, tok.Token, owner, repo, head) })
	wg.Wait()
	if repoErr != nil {
		return PullRequestSyncResult{}, repoErr
	}
	if !found || info.DefaultBranch == "" {
		return PullRequestSyncResult{}, &APIError{Status: http.StatusNotFound}
	}
	base := info.DefaultBranch
	if headErr != nil {
		var apiErr *APIError
		if errors.As(headErr, &apiErr) && apiErr.Status == http.StatusNotFound {
			return PullRequestSyncResult{}, ErrHeadBranchNotFound
		}
		return PullRequestSyncResult{}, headErr
	}
	comparison, err := c.compare(ctx, tok.Token, owner, repo, base, headSHA)
	if err != nil {
		return PullRequestSyncResult{}, err
	}
	if comparison.AheadBy <= 0 {
		return PullRequestSyncResult{}, ErrHeadNotAhead
	}
	if existErr != nil {
		return PullRequestSyncResult{}, existErr
	}
	result := PullRequestSyncResult{HeadSHA: headSHA}
	if existing.Number == 0 {
		created, createErr := c.createPull(ctx, tok.Token, owner, repo, head, base, title, body, draft)
		if createErr == nil {
			result.PullRequest, result.Created = created, true
			return result, nil
		}
		var apiErr *APIError
		if !errors.As(createErr, &apiErr) || apiErr.Status != http.StatusUnprocessableEntity {
			return PullRequestSyncResult{}, createErr
		}
		if draft && strings.Contains(strings.ToLower(APIErrorMessage(createErr)), "draft pull requests are not supported") {
			return PullRequestSyncResult{}, ErrDraftNotSupported
		}
		// A concurrent request for the same head won the create (GitHub answers
		// 422): fall through and update the PR it opened.
		if existing, err = c.findOpenPull(ctx, tok.Token, owner, repo, head); err != nil {
			return PullRequestSyncResult{}, err
		}
		if existing.Number == 0 {
			return PullRequestSyncResult{}, createErr
		}
	}
	updated, err := c.updatePull(ctx, tok.Token, owner, repo, existing.Number, title, body)
	if err != nil {
		return PullRequestSyncResult{}, err
	}
	result.PullRequest = updated
	return result, nil
}

// createPull opens head→base, as a draft when draft is set.
func (c *Client) createPull(ctx context.Context, token, owner, repo, head, base, title, body string, draft bool) (PullRequest, error) {
	return c.doPull(ctx, token, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/pulls", c.baseURL, owner, repo), struct {
		Title string `json:"title"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Body  string `json:"body"`
		Draft bool   `json:"draft"`
	}{Title: title, Head: head, Base: base, Body: body, Draft: draft}, http.StatusCreated)
}

// updatePull replaces an existing PR's title and body. It sends only those two
// fields, so it can never undraft, retarget, or close the PR.
func (c *Client) updatePull(ctx context.Context, token, owner, repo string, number int, title, body string) (PullRequest, error) {
	return c.doPull(ctx, token, http.MethodPatch, fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.baseURL, owner, repo, number), struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}{Title: title, Body: body}, http.StatusOK)
}

// doPull sends one JSON pull-request write and decodes the PR GitHub returns,
// refusing any status other than wantStatus as an *APIError.
func (c *Client) doPull(ctx context.Context, token, method, url string, payload any, wantStatus int) (PullRequest, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return PullRequest{}, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(encoded))
	if err != nil {
		return PullRequest{}, err
	}
	setTokenAuth(req, token)
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PullRequest{}, fmt.Errorf("github: %s pull request: %w", method, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != wantStatus {
		return PullRequest{}, &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	var out ghPull
	if err := json.Unmarshal(raw, &out); err != nil {
		return PullRequest{}, fmt.Errorf("github: decode pull request response: %w", err)
	}
	return out.export(), nil
}

// comparison is how a head commit relates to a base: how many commits it has
// that base lacks, and GitHub's status of it ("ahead", "behind", "identical"
// or "diverged").
type comparison struct {
	AheadBy int    `json:"ahead_by"`
	Status  string `json:"status"`
}

// compare reads GET /repos/{owner}/{repo}/compare/{base}...{head}. The head is
// addressed by its commit SHA so a branch name containing "/" never has to be
// escaped into the compare path. per_page=1 keeps the commit list to one
// entry: ahead_by and status cover the whole comparison regardless of the
// page size.
func (c *Client) compare(ctx context.Context, token, owner, repo, base, headSHA string) (comparison, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s?per_page=1", c.baseURL, owner, repo, neturl.PathEscape(base), neturl.PathEscape(headSHA))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return comparison{}, err
	}
	setTokenAuth(req, token)
	req.Header.Set("Accept", acceptHeader)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return comparison{}, fmt.Errorf("github: compare: %w", err)
	}
	defer resp.Body.Close()
	// The first compare page still carries the changed-files list; only the
	// summary is read, so the decode is bounded rather than fully buffered.
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return comparison{}, &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	var out comparison
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&out); err != nil {
		return comparison{}, fmt.Errorf("github: decode compare response: %w", err)
	}
	return out, nil
}

// findOpenPull returns the open PR for owner:head, if any (Number 0 when none).
func (c *Client) findOpenPull(ctx context.Context, token, owner, repo, head string) (PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&head=%s",
		c.baseURL, owner, repo, neturl.QueryEscape(owner+":"+head))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return PullRequest{}, err
	}
	setTokenAuth(req, token)
	req.Header.Set("Accept", acceptHeader)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PullRequest{}, fmt.Errorf("github: list pull requests: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return PullRequest{}, &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	var out []ghPull
	if err := json.Unmarshal(raw, &out); err != nil {
		return PullRequest{}, fmt.Errorf("github: decode pull request list: %w", err)
	}
	if len(out) == 0 {
		return PullRequest{}, nil
	}
	return out[0].export(), nil
}
