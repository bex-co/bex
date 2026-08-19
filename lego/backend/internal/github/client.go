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
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// defaultBaseURL is GitHub's REST API root. Overridable in tests.
const defaultBaseURL = "https://api.github.com"

// defaultOAuthBaseURL is GitHub's user-authorization (OAuth) host — where the
// install-callback `code` is exchanged for a user token (F2). Distinct from the
// REST API root; overridable in tests.
const defaultOAuthBaseURL = "https://github.com"

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
	// ClientID / ClientSecret are the App's OAuth credentials, used ONLY to verify
	// that the user completing an install actually administers the installation
	// (F2, docs/ADR026-github-integration.md). Both empty leaves existing
	// connection reads/deploys available but makes every new binding fail closed;
	// exactly one set is invalid configuration. The App must enable "Request user
	// authorization (OAuth) during installation" so the callback carries a code.
	ClientID     string
	ClientSecret string
}

// Client talks to the GitHub REST API as a GitHub App. It is safe for
// concurrent use.
type Client struct {
	appID        int64
	privateKey   *rsa.PrivateKey
	slug         string
	baseURL      string
	oauthBaseURL string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	now          func() time.Time
}

// NewClient parses the config (numeric app id, PEM private key) and returns a
// ready client. It errors if any field is missing or malformed.
func NewClient(cfg Config) (*Client, error) {
	if (strings.TrimSpace(cfg.ClientID) == "") != (strings.TrimSpace(cfg.ClientSecret) == "") {
		return nil, errors.New("github: OAuth client id and secret must be configured together")
	}
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
		appID:        appID,
		privateKey:   key,
		slug:         strings.TrimSpace(cfg.Slug),
		baseURL:      defaultBaseURL,
		oauthBaseURL: defaultOAuthBaseURL,
		clientID:     strings.TrimSpace(cfg.ClientID),
		clientSecret: strings.TrimSpace(cfg.ClientSecret),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		now:          time.Now,
	}, nil
}

// InstallVerificationConfigured reports whether the OAuth credentials needed to
// verify installation administration are present (F2). The composition root
// wires the Service's Verifier only when this is true.
func (c *Client) InstallVerificationConfigured() bool {
	return c.clientID != "" && c.clientSecret != ""
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
	// AccountLogin and InstallationID are set by the service when it aggregates
	// repos across a workspace's several connections (ADR075 §4), so the picker
	// can group repos by GitHub account. The client itself leaves them zero.
	AccountLogin   string `json:"accountLogin,omitempty"`
	InstallationID int64  `json:"installationId,omitempty"`
}

// APIError is a non-2xx GitHub response. Callers map it to a clean bex error
// (never a raw 500) so a GitHub outage surfaces as "GitHub said N", not a panic.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	// Status only — the upstream response body is intentionally NOT interpolated
	// here. Error() flows into HTTP responses (core.WriteErr), and GitHub's
	// verbatim 4xx/5xx error text is upstream-error disclosure (w6/005). The body
	// stays on the struct for any internal/structured-log consumer that wants it.
	return fmt.Sprintf("github: unexpected status %d", e.Status)
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
// token for the given installation. The existing deploy/list/commit path needs
// all installation repositories but remains explicitly read-only even though
// the App itself now holds contents:write for agent sessions.
func (c *Client) MintInstallationToken(ctx context.Context, installationID int64) (InstallationToken, error) {
	return c.mintInstallationToken(ctx, installationID, struct {
		Permissions map[string]string `json:"permissions"`
	}{Permissions: map[string]string{"contents": "read", "metadata": "read"}})
}

// MintSessionInstallationToken mints the least-privilege Git credential used by
// one agent session. GitHub's request accepts repository NAMES (the installation
// itself fixes the owner), so callers must separately bind owner/account before
// reaching this method. Contents write includes clone/fetch read access and push;
// metadata stays read-only. The token expires after GitHub's fixed one-hour TTL.
func (c *Client) MintSessionInstallationToken(ctx context.Context, installationID int64, repository string) (InstallationToken, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" || strings.Contains(repository, "/") {
		return InstallationToken{}, fmt.Errorf("github: repository name is required")
	}
	return c.mintInstallationToken(ctx, installationID, struct {
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}{
		Repositories: []string{repository},
		Permissions: map[string]string{
			"contents": "write",
			"metadata": "read",
		},
	})
}

func (c *Client) mintInstallationToken(ctx context.Context, installationID int64, payload any) (InstallationToken, error) {
	appTok, err := c.appJWT()
	if err != nil {
		return InstallationToken{}, err
	}
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", c.baseURL, installationID)
	var requestBody io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return InstallationToken{}, err
		}
		requestBody = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, requestBody)
	if err != nil {
		return InstallationToken{}, err
	}
	req.Header.Set("Authorization", "Bearer "+appTok)
	req.Header.Set("Accept", acceptHeader)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
	AccountType  string `json:"accountType"`
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
		ID         int64  `json:"id"`
		TargetType string `json:"target_type"`
		Account    struct {
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Installation{}, fmt.Errorf("github: decode installation response: %w", err)
	}
	return Installation{ID: out.ID, AccountLogin: out.Account.Login, AccountType: out.TargetType}, nil
}

// VerifyInstallationAdmin reports whether the GitHub user identified by the
// install-callback OAuth `code` actually administers installationID — the
// principal proof the connect callback needs so an App-JWT lookup is never
// mistaken for ownership (F2). It exchanges the code for a short-lived user
// token, resolves the installation's owning account, and requires either the
// exact personal-account owner or an active organization membership with role
// `admin`. Merely seeing an installation or one selected repository is not an
// administration proof.
func (c *Client) VerifyInstallationAdmin(ctx context.Context, code string, installationID int64) (bool, error) {
	if c.clientID == "" || c.clientSecret == "" {
		return false, fmt.Errorf("github: installation-admin verification not configured")
	}
	if strings.TrimSpace(code) == "" {
		return false, nil // no user-authorization code => cannot prove administration
	}
	token, err := c.exchangeUserCode(ctx, code)
	if err != nil {
		return false, err
	}
	if token == "" {
		return false, nil
	}
	installation, err := c.GetInstallation(ctx, installationID)
	if err != nil {
		return false, err
	}
	login, err := c.authenticatedUser(ctx, token)
	if err != nil {
		return false, err
	}
	switch installation.AccountType {
	case "User":
		return strings.EqualFold(login, installation.AccountLogin), nil
	case "Organization":
		return c.userIsOrganizationAdmin(ctx, token, installation.AccountLogin)
	default:
		return false, nil
	}
}

// exchangeUserCode swaps an install-callback OAuth code for a user access token
// (POST https://github.com/login/oauth/access_token). Returns "" (no error) when
// GitHub declines the exchange with an error payload rather than a token.
func (c *Client) exchangeUserCode(ctx context.Context, code string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
		"code":          code,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.oauthBaseURL+"/login/oauth/access_token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("github: exchange user code: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("github: decode user token: %w", err)
	}
	return out.AccessToken, nil // "" when out.Error is set (GitHub declined)
}

func (c *Client) authenticatedUser(ctx context.Context, userToken string) (string, error) {
	var out struct {
		Login string `json:"login"`
	}
	if err := c.getAsUser(ctx, userToken, "/user", &out); err != nil {
		return "", err
	}
	return out.Login, nil
}

func (c *Client) userIsOrganizationAdmin(ctx context.Context, userToken, organization string) (bool, error) {
	var membership struct {
		State string `json:"state"`
		Role  string `json:"role"`
	}
	err := c.getAsUser(ctx, userToken, "/user/memberships/orgs/"+neturl.PathEscape(organization), &membership)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return membership.State == "active" && membership.Role == "admin", nil
}

func (c *Client) getAsUser(ctx context.Context, userToken, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Accept", acceptHeader)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: verify user authority: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("github: decode user authority: %w", err)
	}
	return nil
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
	setTokenAuth(req, token)
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

// Commit is the resolved tip of a ref — the SHA plus its message and author
// timestamp (w9/001 + w2/m42). The subset of GitHub's commit object the
// deploy path stamps onto a deploy row as provenance.
type Commit struct {
	SHA      string     `json:"sha"`
	Message  string     `json:"message"`
	AuthorAt *time.Time `json:"authorAt,omitempty"`
}

// GetCommit resolves ref (a branch name, tag, or SHA) to the exact commit it
// points at (GET /repos/{owner}/{repo}/commits/{ref}). An unknown ref or an
// out-of-grant repo returns an *APIError (404/422) — callers treat those as
// "unresolvable", not a failure.
func (c *Client) GetCommit(ctx context.Context, token, owner, repo, ref string) (Commit, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", c.baseURL, owner, repo, neturl.PathEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Commit{}, err
	}
	setTokenAuth(req, token)
	req.Header.Set("Accept", acceptHeader)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Commit{}, fmt.Errorf("github: get commit: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Commit{}, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	var out struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Date *time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Commit{}, fmt.Errorf("github: decode commit response: %w", err)
	}
	return Commit{SHA: out.SHA, Message: out.Commit.Message, AuthorAt: out.Commit.Author.Date}, nil
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

// ghBranch is one entry of GitHub's list-branches response.
type ghBranch struct {
	Name string `json:"name"`
}

// ListBranches returns every branch name of owner/repo the installation can
// access, following pagination (per_page=100, `Link` rel="next"). It mints a
// fresh installation token first. (w5/m54 — feeds the dashboard's searchable
// Branch combobox.)
func (c *Client) ListBranches(ctx context.Context, installationID int64, owner, repo string) ([]string, error) {
	tok, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	url := c.baseURL + "/repos/" + owner + "/" + repo + "/branches?per_page=100"
	var branches []string
	for url != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "token "+tok.Token)
		req.Header.Set("Accept", acceptHeader)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("github: list branches: %w", err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		nextURL := nextLink(resp.Header.Get("Link"))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
		}
		var page []ghBranch
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("github: decode branches response: %w", err)
		}
		for _, b := range page {
			branches = append(branches, b.Name)
		}
		url = nextURL
	}
	return branches, nil
}

// FileContents is the decoded content of a repository file (w2/m62 — blueprint
// manifest fetch). CommitSHA is the branch HEAD at the time of the fetch.
type FileContents struct {
	Contents  string
	CommitSHA string
}

// GetFileContents fetches the raw contents of path at ref in owner/repo using
// the supplied installation access token (GitHub GET /repos/{owner}/{repo}/contents/{path}).
// Returns ErrNotFound when the file does not exist; any other non-2xx is an *APIError.
// The response body is bounded to 1 MiB (same as render.yaml in practice).
// setTokenAuth sets the token Authorization header, omitting it entirely for
// an empty token: the blueprint fetcher's documented anonymous public-repo
// path passes "" — an empty "Authorization: token " header makes GitHub
// return 401 where sending no header at all succeeds.
func setTokenAuth(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
}

func (c *Client) GetFileContents(ctx context.Context, token, owner, repo, path, ref string) (FileContents, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", c.baseURL, owner, repo, neturl.PathEscape(path), neturl.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return FileContents{}, err
	}
	setTokenAuth(req, token)
	// Ask for the raw file directly — avoids base64 decoding.
	req.Header.Set("Accept", "application/vnd.github.raw+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return FileContents{}, fmt.Errorf("github: get file: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return FileContents{}, fmt.Errorf("github: file %q not found at ref %q: %w", path, ref, &APIError{Status: resp.StatusCode})
	}
	if resp.StatusCode != http.StatusOK {
		return FileContents{}, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	return FileContents{Contents: string(body), CommitSHA: resp.Header.Get("X-GitHub-File-Sha")}, nil
}

// GetRepoCommitSHA resolves the HEAD commit SHA for a branch using the branch
// info endpoint. Used by the blueprint fetcher to stamp the commitID on each
// sync run when we already hold the file contents.
func (c *Client) GetRepoCommitSHA(ctx context.Context, token, owner, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/branches/%s", c.baseURL, owner, repo, neturl.PathEscape(branch))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	setTokenAuth(req, token)
	req.Header.Set("Accept", acceptHeader)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("github: get branch: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	var out struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("github: decode branch response: %w", err)
	}
	return out.Commit.SHA, nil
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
