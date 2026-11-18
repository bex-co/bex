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

package oidcissuer

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/bex-co/bex/lego/backend/internal/id"
)

// DefaultEnvironment is the sub claim's environment segment for a service in
// no environment, as on Render.
const DefaultEnvironment = "default"

// Issuer signs and publishes for every workspace under one base URL.
type Issuer struct {
	baseURL string // scheme://host[:port], no trailing slash
	host    string // the base URL's host without its port: the routing key
	keys    *KeySet
}

// New returns the issuer for baseURL (BEX_OIDC_ISSUER_URL) over keys.
func New(baseURL string, keys *KeySet) (*Issuer, error) {
	base, host, err := ParseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &Issuer{baseURL: base, host: host, keys: keys}, nil
}

// ParseBaseURL validates an issuer base URL: an http or https origin with no
// path, query, fragment or user info. It returns the URL without a trailing
// slash and its host without the port.
func ParseBaseURL(raw string) (base, host string, err error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" ||
		u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", "", fmt.Errorf("oidc issuer base URL %q must be an http(s) origin with no path, query or user info, e.g. https://oidc.bex.co", raw)
	}
	return u.Scheme + "://" + u.Host, strings.ToLower(u.Hostname()), nil
}

// Host is the host the issuer answers on, without a port.
func (i *Issuer) Host() string { return i.host }

// URL is workspace's issuer: the iss claim and the discovery base.
func (i *Issuer) URL(workspaceID string) string { return i.baseURL + "/" + workspaceID }

// Subject names the service a token is for. Environment is a public evm- id
// (an env- storage id is respelled) or empty/"default" for a service in no
// environment.
type Subject struct {
	Workspace   string
	Environment string
	Service     string
}

// String is Render's sub claim:
// workspace:<tea>:environment:<evm-…|default>:service:<srv-…>.
func (s Subject) String() string {
	return "workspace:" + s.Workspace + ":environment:" + s.environment() + ":service:" + s.Service
}

func (s Subject) environment() string {
	if s.Environment == "" {
		return DefaultEnvironment
	}
	return id.EnvironmentPublicID(s.Environment)
}

// Validate refuses a subject any part of which is not a well-formed id of its
// kind, so a minted token always names a workspace whose issuer answers.
func (s Subject) Validate() error {
	if !id.Is(id.Workspace, s.Workspace) {
		return fmt.Errorf("oidc subject: workspace %q is not a workspace id", s.Workspace)
	}
	if env := s.environment(); env != DefaultEnvironment && !id.Is(id.Environment, env) {
		return fmt.Errorf("oidc subject: environment %q is not an environment id", s.Environment)
	}
	if !id.Is(id.Service, s.Service) {
		return fmt.Errorf("oidc subject: service %q is not a service id", s.Service)
	}
	return nil
}

// Token is a signed identity token.
type Token struct {
	Raw       string
	KeyID     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Mint signs a token for sub, audience aud, with the current key
// (docs/ADR116-managed-oidc.md D3): iss is the workspace's issuer, exp is
// iat + TokenLifetime, and the header names the key's kid.
func (i *Issuer) Mint(sub Subject, aud string) (Token, error) {
	if err := sub.Validate(); err != nil {
		return Token{}, err
	}
	if strings.TrimSpace(aud) == "" {
		return Token{}, errors.New("oidc token: the audience is empty")
	}
	now := i.keys.now()
	key, err := i.keys.signing(now)
	if err != nil {
		return Token{}, err
	}
	iat := now.Truncate(time.Second)
	exp := iat.Add(TokenLifetime)
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": i.URL(sub.Workspace),
		"sub": sub.String(),
		"aud": aud,
		"iat": iat.Unix(),
		"exp": exp.Unix(),
	})
	tok.Header["kid"] = key.ID
	raw, err := tok.SignedString(key.Private)
	if err != nil {
		return Token{}, err
	}
	return Token{Raw: raw, KeyID: key.ID, IssuedAt: iat, ExpiresAt: exp}, nil
}
