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

// Package github is bex-api's GitHub App integration (docs/ADR026-github-integration.md):
// a small client that signs the app JWT, mints short-lived installation tokens,
// and lists an installation's repositories — plus the workspace-connection
// service verbs over the control-plane store. The operator never imports this;
// bex-api mints tokens and writes them into a k8s Secret the build Job consumes.
package github

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// defaultBaseURL is GitHub's REST API root. Overridable in tests.
const defaultBaseURL = "https://api.github.com"

// acceptHeader is GitHub's recommended REST media type.
const acceptHeader = "application/vnd.github+json"

// Config is the GitHub App configuration read once at startup from
// BEX_GITHUB_APP_ID / BEX_GITHUB_APP_PRIVATE_KEY / BEX_GITHUB_APP_SLUG. Any
// field empty (or an unparseable id/key) => NewClient errors and the caller
// leaves the github service unconfigured (503).
type Config struct {
	AppID      string // numeric GitHub App id (the JWT `iss`)
	PrivateKey string // RSA private key, PEM (out-of-band secret)
	Slug       string // app slug, builds the install URL
}

// Client talks to the GitHub REST API as a GitHub App. It is safe for
// concurrent use.
type Client struct {
	appID      int64
	privateKey *rsa.PrivateKey
	slug       string
	baseURL    string
	httpClient *http.Client
	now        func() time.Time
}

// NewClient parses the config (numeric app id, PEM private key) and returns a
// ready client. It errors if any field is missing or malformed.
func NewClient(cfg Config) (*Client, error) {
	if cfg.AppID == "" || cfg.PrivateKey == "" || cfg.Slug == "" {
		return nil, fmt.Errorf("github: incomplete config (need app id, private key, slug)")
	}
	appID, err := strconv.ParseInt(strings.TrimSpace(cfg.AppID), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("github: invalid app id %q: %w", cfg.AppID, err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("github: invalid private key: %w", err)
	}
	return &Client{
		appID:      appID,
		privateKey: key,
		slug:       strings.TrimSpace(cfg.Slug),
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		now:        time.Now,
	}, nil
}

// Slug is the configured app slug (used to build install URLs).
func (c *Client) Slug() string { return c.slug }

// InstallURL is where a workspace admin installs the app (and grants repos).
func (c *Client) InstallURL() string {
	return "https://github.com/apps/" + c.slug + "/installations/new"
}

// InstallationToken is a short-lived (1h) installation access token.
type InstallationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Repo is the subset of a GitHub repository the repo picker and deploy path
// need. JSON tags are the bex-api camelCase shape (identical across surfaces),
// not GitHub's snake_case wire shape (decoded via ghRepo below).
type Repo struct {
	ID            int64  `json:"id"`
	FullName      string `json:"fullName"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"defaultBranch"`
	HTMLURL       string `json:"htmlUrl"`
	CloneURL      string `json:"cloneUrl"`
}

// APIError is a non-2xx GitHub response. Callers map it to a clean bex error
// (never a raw 500) so a GitHub outage surfaces as "GitHub said N", not a panic.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github: unexpected status %d: %s", e.Status, e.Body)
}

// appJWT signs a short-lived (≤10m) RS256 assertion identifying the app itself.
// iat is backdated 60s to tolerate clock skew; exp is 9m out (GitHub's ceiling
// is 10m).
func (c *Client) appJWT() (string, error) {
	now := c.now()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(c.appID, 10),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	})
	return tok.SignedString(c.privateKey)
}

// MintInstallationToken exchanges the app JWT for a 1h installation access
// token for the given installation.
func (c *Client) MintInstallationToken(ctx context.Context, installationID int64) (InstallationToken, error) {
	appTok, err := c.appJWT()
	if err != nil {
		return InstallationToken{}, err
	}
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", c.baseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(nil))
	if err != nil {
		return InstallationToken{}, err
	}
	req.Header.Set("Authorization", "Bearer "+appTok)
	req.Header.Set("Accept", acceptHeader)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return InstallationToken{}, fmt.Errorf("github: mint token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated {
		return InstallationToken{}, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return InstallationToken{}, fmt.Errorf("github: decode token response: %w", err)
	}
	return InstallationToken{Token: out.Token, ExpiresAt: out.ExpiresAt}, nil
}

// Installation identifies a GitHub App installation and the account it belongs
// to. Fetched with the app JWT, so a forged/unknown id fails authentication —
// which is how Connect validates a browser-supplied installation_id.
type Installation struct {
	ID           int64  `json:"id"`
	AccountLogin string `json:"accountLogin"`
}

// GetInstallation fetches one installation by id (GET /app/installations/{id}),
// authenticated as the app. A non-existent or inaccessible id returns an
// *APIError (typically 404) — the authenticity check for the connect callback.
func (c *Client) GetInstallation(ctx context.Context, installationID int64) (Installation, error) {
	appTok, err := c.appJWT()
	if err != nil {
		return Installation{}, err
	}
	url := fmt.Sprintf("%s/app/installations/%d", c.baseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Installation{}, err
	}
	req.Header.Set("Authorization", "Bearer "+appTok)
	req.Header.Set("Accept", acceptHeader)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Installation{}, fmt.Errorf("github: get installation: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Installation{}, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	var out struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Installation{}, fmt.Errorf("github: decode installation response: %w", err)
	}
	return Installation{ID: out.ID, AccountLogin: out.Account.Login}, nil
}

// RepoAccessible reports whether the given installation token can reach
// owner/repo (GET /repos/{owner}/{repo}). An installation token only reaches
// repos in the installation's grant, so 404 => not granted (ok=false, no error);
// 2xx => granted; any other status is an *APIError.
func (c *Client) RepoAccessible(ctx context.Context, token, owner, repo string) (bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", acceptHeader)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("github: check repo access: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true, nil
	default:
		return false, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
}

// ghRepo is GitHub's wire shape; mapped to the exported Repo.
type ghRepo struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
}

// ListRepos returns every repository the installation can access, following
// pagination (per_page=100, GitHub `Link` header rel="next"). It mints a fresh
// installation token first.
func (c *Client) ListRepos(ctx context.Context, installationID int64) ([]Repo, error) {
	tok, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	url := c.baseURL + "/installation/repositories?per_page=100"
	var repos []Repo
	for url != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "token "+tok.Token)
		req.Header.Set("Accept", acceptHeader)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("github: list repos: %w", err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		nextURL := nextLink(resp.Header.Get("Link"))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
		}
		var page struct {
			Repositories []ghRepo `json:"repositories"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("github: decode repos response: %w", err)
		}
		for _, r := range page.Repositories {
			repos = append(repos, Repo{
				ID:            r.ID,
				FullName:      r.FullName,
				Private:       r.Private,
				DefaultBranch: r.DefaultBranch,
				HTMLURL:       r.HTMLURL,
				CloneURL:      r.CloneURL,
			})
		}
		url = nextURL
	}
	return repos, nil
}

// nextLink extracts the rel="next" URL from a GitHub `Link` header, or "".
func nextLink(header string) string {
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		segs := strings.Split(strings.TrimSpace(part), ";")
		if len(segs) < 2 {
			continue
		}
		var isNext bool
		for _, s := range segs[1:] {
			if strings.TrimSpace(s) == `rel="next"` {
				isNext = true
				break
			}
		}
		if !isNext {
			continue
		}
		u := strings.TrimSpace(segs[0])
		u = strings.TrimPrefix(u, "<")
		u = strings.TrimSuffix(u, ">")
		return u
	}
	return ""
}
