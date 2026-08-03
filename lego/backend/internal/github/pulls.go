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
)

// PullRequest is the subset of a GitHub pull request the agent-session delivery
// path records (ADR047 D4). JSON tags are bex-api's camelCase shape, not
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

// OpenDraftPullRequest opens a draft PR from head→base on owner/repo using an
// installation token narrowed to that one repository with pull_requests:write.
// One branch, one PR per session (Copilot model): when a PR already exists for
// the head branch (GitHub answers 422), the existing open PR is returned so a
// steering turn updates the same PR instead of failing. The token is minted per
// call, is repository-scoped, and is never returned to the caller.
func (c *Client) OpenDraftPullRequest(ctx context.Context, installationID int64, owner, repo, head, base, title, body string) (PullRequest, error) {
	tok, err := c.mintInstallationToken(ctx, installationID, struct {
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}{
		Repositories: []string{repo},
		Permissions:  map[string]string{"pull_requests": "write", "contents": "read", "metadata": "read"},
	})
	if err != nil {
		return PullRequest{}, err
	}
	pr, err := c.createPull(ctx, tok.Token, owner, repo, head, base, title, body)
	if err == nil {
		return pr, nil
	}
	// A pre-existing PR for this head branch is the steering-turn steady state,
	// not an error: return the open PR so branch+URL stay stable across turns.
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
		if existing, findErr := c.findOpenPull(ctx, tok.Token, owner, repo, head); findErr == nil && existing.Number != 0 {
			return existing, nil
		}
	}
	return PullRequest{}, err
}

func (c *Client) createPull(ctx context.Context, token, owner, repo, head, base, title, body string) (PullRequest, error) {
	payload, err := json.Marshal(struct {
		Title string `json:"title"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Body  string `json:"body"`
		Draft bool   `json:"draft"`
	}{Title: title, Head: head, Base: base, Body: body, Draft: true})
	if err != nil {
		return PullRequest{}, err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/pulls", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return PullRequest{}, err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PullRequest{}, fmt.Errorf("github: open pull request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated {
		return PullRequest{}, &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	var out ghPull
	if err := json.Unmarshal(raw, &out); err != nil {
		return PullRequest{}, fmt.Errorf("github: decode pull request response: %w", err)
	}
	return out.export(), nil
}

// findOpenPull returns the open PR for owner:head, if any. Used only to make
// OpenDraftPullRequest idempotent when GitHub reports the PR already exists.
func (c *Client) findOpenPull(ctx context.Context, token, owner, repo, head string) (PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&head=%s",
		c.baseURL, owner, repo, neturl.QueryEscape(owner+":"+head))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return PullRequest{}, err
	}
	req.Header.Set("Authorization", "token "+token)
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
