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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// testKeyPEM generates an RSA key and returns its PKCS1 PEM plus the key.
func testKeyPEM(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return string(pemBytes), key
}

func TestNewClientRejectsBadConfig(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	cases := []struct {
		name string
		cfg  Config
	}{
		{"empty", Config{}},
		{"missing slug", Config{AppID: "1", PrivateKey: keyPEM}},
		{"non-numeric id", Config{AppID: "abc", PrivateKey: keyPEM, Slug: "x"}},
		{"bad key", Config{AppID: "1", PrivateKey: "not a pem", Slug: "x"}},
		{"OAuth client id only", Config{AppID: "1", PrivateKey: keyPEM, Slug: "x", ClientID: "client"}},
		{"OAuth client secret only", Config{AppID: "1", PrivateKey: keyPEM, Slug: "x", ClientSecret: "secret"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(tc.cfg); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestAppJWTClaims(t *testing.T) {
	keyPEM, key := testKeyPEM(t)
	c, err := NewClient(Config{AppID: "12345", PrivateKey: keyPEM, Slug: "bex"})
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return fixed }

	tokStr, err := c.appJWT()
	if err != nil {
		t.Fatal(err)
	}
	var claims jwt.RegisteredClaims
	parsed, err := jwt.ParseWithClaims(tokStr, &claims, func(tok *jwt.Token) (any, error) {
		if tok.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected alg %s", tok.Method.Alg())
		}
		return &key.PublicKey, nil
	}, jwt.WithTimeFunc(func() time.Time { return fixed }))
	if err != nil || !parsed.Valid {
		t.Fatalf("parse jwt: %v valid=%v", err, parsed.Valid)
	}
	if claims.Issuer != "12345" {
		t.Errorf("iss = %q, want 12345", claims.Issuer)
	}
	if got := claims.IssuedAt.Time; !got.Equal(fixed.Add(-60 * time.Second)) {
		t.Errorf("iat = %v, want %v", got, fixed.Add(-60*time.Second))
	}
	if got := claims.ExpiresAt.Time; !got.Equal(fixed.Add(9 * time.Minute)) {
		t.Errorf("exp = %v, want %v", got, fixed.Add(9*time.Minute))
	}
}

func TestMintInstallationToken(t *testing.T) {
	keyPEM, key := testKeyPEM(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/app/installations/99/access_tokens" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		// The Authorization header must be a valid app JWT signed by our key.
		auth := r.Header.Get("Authorization")
		const p = "Bearer "
		if len(auth) <= len(p) || auth[:len(p)] != p {
			t.Fatalf("missing Bearer auth: %q", auth)
		}
		_, err := jwt.Parse(auth[len(p):], func(*jwt.Token) (any, error) { return &key.PublicKey, nil })
		if err != nil {
			t.Errorf("app jwt invalid: %v", err)
		}
		var body struct {
			Repositories []string          `json:"repositories"`
			Permissions  map[string]string `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Repositories) != 0 || body.Permissions["contents"] != "read" || body.Permissions["metadata"] != "read" {
			t.Fatalf("ordinary token must remain read-only across the installation: %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"token":"ghs_abc","expires_at":"2026-07-11T13:00:00Z"}`)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
	c.baseURL = srv.URL

	tok, err := c.MintInstallationToken(context.Background(), 99)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token != "ghs_abc" {
		t.Errorf("token = %q", tok.Token)
	}
	want := time.Date(2026, 7, 11, 13, 0, 0, 0, time.UTC)
	if !tok.ExpiresAt.Equal(want) {
		t.Errorf("expiresAt = %v, want %v", tok.ExpiresAt, want)
	}
}

func TestMintSessionInstallationTokenNarrowsRepositoryAndPermissions(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/installations/42/access_tokens" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("request path=%q content-type=%q", r.URL.Path, r.Header.Get("Content-Type"))
		}
		var body struct {
			Repositories []string          `json:"repositories"`
			Permissions  map[string]string `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Repositories) != 1 || body.Repositories[0] != "repo" || body.Permissions["contents"] != "write" || body.Permissions["metadata"] != "read" || len(body.Permissions) != 2 {
			t.Fatalf("scoped token body = %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"token":"ghs_scoped","expires_at":"2026-08-01T13:00:00Z"}`)
	}))
	defer server.Close()

	client, _ := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
	client.baseURL = server.URL
	token, err := client.MintSessionInstallationToken(context.Background(), 42, "repo")
	if err != nil || token.Token != "ghs_scoped" {
		t.Fatalf("MintSessionInstallationToken token=%+v err=%v", token, err)
	}
	if _, err := client.MintSessionInstallationToken(context.Background(), 42, "owner/repo"); err == nil {
		t.Fatal("repository owner must not reach GitHub's name-only narrowing field")
	}
}

func TestVerifyInstallationAdminRequiresExplicitOrganizationAdmin(t *testing.T) {
	for _, tc := range []struct {
		name, state, role string
		want              bool
	}{
		{"active admin", "active", "admin", true},
		{"repository-only member", "active", "member", false},
		{"pending admin", "pending", "admin", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keyPEM, _ := testKeyPEM(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/login/oauth/access_token":
					fmt.Fprint(w, `{"access_token":"ghu_user"}`)
				case "/app/installations/42":
					fmt.Fprint(w, `{"id":42,"target_type":"Organization","account":{"login":"octo-org"}}`)
				case "/user":
					fmt.Fprint(w, `{"login":"octocat"}`)
				case "/user/memberships/orgs/octo-org":
					if r.Header.Get("Authorization") != "Bearer ghu_user" {
						t.Fatalf("membership request auth = %q", r.Header.Get("Authorization"))
					}
					fmt.Fprintf(w, `{"state":%q,"role":%q}`, tc.state, tc.role)
				default:
					t.Fatalf("unexpected GitHub request %s", r.URL.Path)
				}
			}))
			defer server.Close()

			client, err := NewClient(Config{
				AppID: "1", PrivateKey: keyPEM, Slug: "bex",
				ClientID: "client", ClientSecret: "secret",
			})
			if err != nil {
				t.Fatal(err)
			}
			client.baseURL = server.URL
			client.oauthBaseURL = server.URL
			got, err := client.VerifyInstallationAdmin(context.Background(), "code", 42)
			if err != nil || got != tc.want {
				t.Fatalf("VerifyInstallationAdmin = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestVerifyInstallationAdminRequiresPersonalInstallationOwner(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			fmt.Fprint(w, `{"access_token":"ghu_user"}`)
		case "/app/installations/42":
			fmt.Fprint(w, `{"id":42,"target_type":"User","account":{"login":"owner"}}`)
		case "/user":
			fmt.Fprint(w, `{"login":"collaborator"}`)
		default:
			t.Fatalf("unexpected GitHub request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex", ClientID: "client", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	client.baseURL, client.oauthBaseURL = server.URL, server.URL
	if ok, err := client.VerifyInstallationAdmin(context.Background(), "code", 42); err != nil || ok {
		t.Fatalf("collaborator accepted for personal installation: ok=%v err=%v", ok, err)
	}
}

func TestListReposPagination(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/7/access_tokens":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"token":"ghs_x","expires_at":"2026-07-11T13:00:00Z"}`)
		case "/installation/repositories":
			if r.Header.Get("Authorization") != "token ghs_x" {
				t.Errorf("installation call not authed with install token: %q", r.Header.Get("Authorization"))
			}
			if r.URL.Query().Get("page") == "2" {
				fmt.Fprint(w, `{"repositories":[{"id":2,"full_name":"o/two","private":true,"default_branch":"dev","html_url":"h2","clone_url":"c2"}]}`)
				return
			}
			// page 1 points at page 2 via Link.
			w.Header().Set("Link", fmt.Sprintf(`<%s/installation/repositories?page=2>; rel="next"`, srv.URL))
			fmt.Fprint(w, `{"repositories":[{"id":1,"full_name":"o/one","private":false,"default_branch":"main","html_url":"h1","clone_url":"c1"}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c, _ := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
	c.baseURL = srv.URL

	repos, err := c.ListRepos(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("got %d repos, want 2: %+v", len(repos), repos)
	}
	if repos[0].FullName != "o/one" || repos[0].Private {
		t.Errorf("repo0 = %+v", repos[0])
	}
	if repos[1].FullName != "o/two" || !repos[1].Private || repos[1].DefaultBranch != "dev" {
		t.Errorf("repo1 = %+v", repos[1])
	}
}

func TestListReposRejectsCyclicPagination(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/7/access_tokens":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"token":"ghs_x","expires_at":"2026-07-11T13:00:00Z"}`)
		case "/installation/repositories":
			w.Header().Set("Link", fmt.Sprintf(`<%s/installation/repositories?page=1>; rel="next"`, srv.URL))
			fmt.Fprint(w, `{"repositories":[]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c, _ := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
	c.baseURL = srv.URL
	if _, err := c.ListRepos(context.Background(), 7); !errors.Is(err, errInventoryBound) {
		t.Fatalf("cyclic pagination error = %v, want inventory bound", err)
	}
}

func TestListReposEnforcesInventoryBounds(t *testing.T) {
	cases := []struct {
		name string
		mode string
	}{
		{name: "page", mode: "page"},
		{name: "items", mode: "items"},
		{name: "bytes", mode: "bytes"},
		{name: "origin", mode: "origin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keyPEM, _ := testKeyPEM(t)
			calls := 0
			var srv *httptest.Server
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/app/installations/7/access_tokens":
					w.WriteHeader(http.StatusCreated)
					fmt.Fprint(w, `{"token":"ghs_x","expires_at":"2026-07-11T13:00:00Z"}`)
				case "/installation/repositories":
					calls++
					switch tc.mode {
					case "page":
						w.Header().Set("Link", fmt.Sprintf(`<%s/installation/repositories?page=%d>; rel="next"`, srv.URL, calls))
						fmt.Fprint(w, `{"repositories":[]}`)
					case "items":
						items := strings.TrimSuffix(strings.Repeat(`{"id":1},`, maxInventoryItems+1), ",")
						fmt.Fprintf(w, `{"repositories":[%s]}`, items)
					case "bytes":
						_, _ = w.Write(make([]byte, maxInventoryPageBytes+1))
					case "origin":
						w.Header().Set("Link", `<https://evil.example/installation/repositories?page=2>; rel="next"`)
						fmt.Fprint(w, `{"repositories":[]}`)
					}
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer srv.Close()

			c, _ := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
			c.baseURL = srv.URL
			if _, err := c.ListRepos(context.Background(), 7); !errors.Is(err, errInventoryBound) {
				t.Fatalf("%s bound error = %v, want inventory bound", tc.mode, err)
			}
			if tc.mode == "page" && calls != maxInventoryPages {
				t.Fatalf("page calls = %d, want exactly %d", calls, maxInventoryPages)
			}
		})
	}
}

func TestGitHubErrorPassthrough(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
	c.baseURL = srv.URL

	_, err := c.MintInstallationToken(context.Background(), 1)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Errorf("status = %d", apiErr.Status)
	}
}

func TestRepoAccessible(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	cases := []struct {
		status  int
		wantOK  bool
		wantErr bool
	}{
		{http.StatusOK, true, false},
		{http.StatusNotFound, false, false}, // not in the grant
		{http.StatusInternalServerError, false, true},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/octo/app" {
				t.Errorf("unexpected path %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "token ghs_x" {
				t.Errorf("not authed with install token: %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(tc.status)
		}))
		c, _ := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
		c.baseURL = srv.URL
		ok, err := c.RepoAccessible(context.Background(), "ghs_x", "octo", "app")
		if ok != tc.wantOK || (err != nil) != tc.wantErr {
			t.Errorf("status %d => ok=%v err=%v, want ok=%v err=%v", tc.status, ok, err, tc.wantOK, tc.wantErr)
		}
		srv.Close()
	}
}

func TestNextLink(t *testing.T) {
	cases := map[string]string{
		"":                                 "",
		`<https://a/x?page=2>; rel="next"`: "https://a/x?page=2",
		`<https://a/x?page=5>; rel="last"`: "",
		`<https://a/x?page=2>; rel="next", <https://a/x?page=9>; rel="last"`: "https://a/x?page=2",
	}
	for header, want := range cases {
		if got := nextLink(header); got != want {
			t.Errorf("nextLink(%q) = %q, want %q", header, got, want)
		}
	}
}
