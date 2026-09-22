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
package router

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const BetaWorkspace = "tea-d98210cbbpdc73dcrkvg"

// ErrUnavailable is an upstream OUTAGE: the router answered unusably. It stays
// an unwrapped sentinel, so it reaches the caller as a 5xx, which is what it is.
var ErrUnavailable = errors.New("router is unavailable")

// ErrNotEnabled is the router not being turned on for this workspace — a
// capability the caller does not have, not a failure (w4/116).
//
// The three write verbs never consulted Available, so a caller outside the beta
// workspace reached the upstream call and got ErrUnavailable's generic
// "internal error" where a named refusal belongs. The wording deliberately says
// nothing about WHICH workspace has the router: a refusal that named the beta
// workspace, or that differed between "not you" and "no such feature", would be
// an existence oracle for it.
var ErrNotEnabled = fmt.Errorf("%w: the router is not enabled for this workspace", core.ErrForbidden)

type MemberStore interface {
	GetTenantMember(context.Context, string, string) (store.TenantMember, error)
}

// Service bridges the published Router GraphQL contract. The browser never
// receives the assertion or the private upstream address.
type Service struct {
	*core.Base
	URL     string
	Secret  string
	Members MemberStore
}

type Window struct {
	Kind           string  `json:"kind"`
	UtilizationBps float64 `json:"utilizationBps"`
	ResetsAt       string  `json:"resetsAt"`
}
type Quota struct {
	ObservedAt string   `json:"observedAt"`
	Windows    []Window `json:"windows"`
}
type Options struct {
	AllowOrigin      string `json:"allowOrigin"`
	AllowMethods     string `json:"allowMethods"`
	AllowHeaders     string `json:"allowHeaders"`
	AllowCredentials bool   `json:"allowCredentials"`
}
type Key struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	AccessKey string   `json:"accessKey"`
	CreatedAt string   `json:"createdAt"`
	Options   *Options `json:"options"`
}
type Overview struct {
	Quota *Quota `json:"quota"`
	Keys  []Key  `json:"keys"`
}

func (s *Service) Available(ctx context.Context) (bool, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return false, err
	}
	tenant, ok := s.Tenant(ctx)
	return ok && tenant == BetaWorkspace && s.URL != "" && len(s.Secret) >= 32 && s.Members != nil && s.Authz != nil, nil
}

// requireEnabled is the gate every router verb past the availability query
// shares: routerAvailable exists precisely to say whether this workspace has
// the router, and the verbs used to answer a different question by reaching
// upstream anyway (w4/116). An Available error is the authorization error it
// already returns; false is the named refusal.
func (s *Service) requireEnabled(ctx context.Context) error {
	available, err := s.Available(ctx)
	if err != nil {
		return err
	}
	if !available {
		return ErrNotEnabled
	}
	return nil
}

func (s *Service) Overview(ctx context.Context) (*Overview, error) {
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return nil, err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}

	workspace, keys, err := s.keys(ctx)
	if err != nil {
		return nil, err
	}
	var data struct {
		Quota *Quota `json:"workspaceQuotaStatus"`
	}
	// Quota collection outages must not disable existing key management.
	if err := s.call(ctx, `query RouterQuota($workspaceId: String!) { workspaceQuotaStatus(workspaceId: $workspaceId) { observedAt windows { kind utilizationBps resetsAt } } }`, map[string]any{"workspaceId": workspace}, &data); err != nil {
		data.Quota = nil
	}
	return &Overview{Quota: data.Quota, Keys: keys}, nil
}

func (s *Service) keys(ctx context.Context) (string, []Key, error) {
	workspace, err := s.workspace(ctx)
	if err != nil {
		return "", nil, err
	}
	var data struct {
		Keys *struct {
			Edges []struct {
				Node Key `json:"node"`
			} `json:"edges"`
		} `json:"accessKeys"`
	}
	err = s.call(ctx, `query RouterKeys($workspaceId: String!) { accessKeys(workspaceId: $workspaceId) { edges { node { id name accessKey createdAt options { allowOrigin allowMethods allowHeaders allowCredentials } } } } }`, map[string]any{"workspaceId": workspace}, &data)
	if err != nil {
		return "", nil, err
	}
	if data.Keys == nil {
		return "", nil, ErrUnavailable
	}
	keys := make([]Key, 0, len(data.Keys.Edges))
	for _, edge := range data.Keys.Edges {
		keys = append(keys, edge.Node)
	}
	return workspace, keys, nil
}

func (s *Service) Create(ctx context.Context, name string) (bool, error) {
	if err := s.Authorize(ctx, core.RelCanManageKeys); err != nil {
		return false, err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return false, err
	}
	if len(name) == 0 || len(name) > 200 {
		return false, core.ErrForbidden
	}
	workspace, err := s.workspace(ctx)
	if err != nil {
		return false, err
	}
	var data struct {
		Value bool `json:"createAccessKey"`
	}
	err = s.call(ctx, `mutation RouterCreate($workspaceId: String!, $name: String!) { createAccessKey(workspaceId: $workspaceId, name: $name) }`, map[string]any{"workspaceId": workspace, "name": name}, &data)
	return data.Value, err
}

func (s *Service) Update(ctx context.Context, keyID, name string, options Options) (bool, error) {
	if err := s.Authorize(ctx, core.RelCanManageKeys); err != nil {
		return false, err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return false, err
	}
	if len(name) == 0 || len(name) > 200 {
		return false, core.ErrForbidden
	}
	// Verify the key belongs to the asserted workspace before sending a write.
	_, keys, err := s.keys(ctx)
	if err != nil {
		return false, err
	}
	found := false
	for _, key := range keys {
		if key.ID == keyID {
			found = true
			break
		}
	}
	if !found {
		return false, core.ErrForbidden
	}
	var data struct {
		Value bool `json:"updateAccessKey"`
	}
	err = s.call(ctx, `mutation RouterUpdate($accessKeyId: String!, $name: String!, $allowOrigin: String, $allowMethods: String, $allowHeaders: String, $allowCredentials: Boolean) { updateAccessKey(accessKeyId: $accessKeyId, name: $name, allowOrigin: $allowOrigin, allowMethods: $allowMethods, allowHeaders: $allowHeaders, allowCredentials: $allowCredentials) }`, map[string]any{"accessKeyId": keyID, "name": name, "allowOrigin": options.AllowOrigin, "allowMethods": options.AllowMethods, "allowHeaders": options.AllowHeaders, "allowCredentials": options.AllowCredentials}, &data)
	return data.Value, err
}

func (s *Service) Delete(ctx context.Context, keyID string) (bool, error) {
	if err := s.Authorize(ctx, core.RelCanManageKeys); err != nil {
		return false, err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return false, err
	}
	_, keys, err := s.keys(ctx)
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		if key.ID != keyID {
			continue
		}
		var data struct {
			Value bool `json:"deleteAccessKey"`
		}
		err = s.call(ctx, `mutation RouterDelete($accessKey: String!) { deleteAccessKey(accessKey: $accessKey) }`, map[string]any{"accessKey": key.AccessKey}, &data)
		return data.Value, err
	}
	return false, core.ErrForbidden
}

// myWorkspaces is assertion-scoped by the upstream contract: exactly the
// mapped workspace, never the user's legacy default workspace.
func (s *Service) workspace(ctx context.Context) (string, error) {
	var data struct {
		Workspaces []struct {
			ID string `json:"id"`
		} `json:"myWorkspaces"`
	}
	if err := s.call(ctx, `query RouterWorkspace { myWorkspaces { id } }`, nil, &data); err != nil {
		return "", err
	}
	if len(data.Workspaces) != 1 || data.Workspaces[0].ID == "" {
		return "", ErrUnavailable
	}
	return data.Workspaces[0].ID, nil
}

func (s *Service) call(ctx context.Context, query string, variables map[string]any, out any) error {
	if s.URL == "" || len(s.Secret) < 32 || s.Members == nil || s.Authz == nil {
		return ErrUnavailable
	}
	identity, ok := core.IdentityFrom(ctx)
	if !ok || identity.Method != "session" || !identity.EmailVerified {
		return core.ErrForbidden
	}
	tenant, ok := s.Tenant(ctx)
	if !ok || tenant != BetaWorkspace {
		return core.ErrForbidden
	}
	if err := s.AuthorizeFresh(ctx, core.RelCanManageKeys); err != nil {
		return err
	}
	member, err := s.Members.GetTenantMember(ctx, tenant, identity.Subject)
	if err != nil {
		return core.ErrForbidden
	}
	if member.Role != "admin" && member.Role != "developer" {
		return core.ErrForbidden
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return ErrUnavailable
	}
	now := time.Now().Unix()
	claims, err := json.Marshal(map[string]any{"iss": "bexbe", "aud": "backend-v2", "sub": identity.Subject, "workspace": "workspace:" + tenant, "email": identity.Email, "email_verified": true, "workspace_role": member.Role, "iat": now, "exp": now + 60, "jti": base64.RawURLEncoding.EncodeToString(nonce)})
	if err != nil {
		return ErrUnavailable
	}
	unsigned := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(s.Secret))
	_, _ = mac.Write([]byte(unsigned))
	assertion := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Bex-Identity-Assertion", assertion)
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(raw) > 2<<20 {
		return ErrUnavailable
	}
	var result struct {
		Data   json.RawMessage   `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Errors) > 0 || len(result.Data) == 0 || string(result.Data) == "null" {
		return ErrUnavailable
	}
	if json.Unmarshal(result.Data, out) != nil {
		return ErrUnavailable
	}
	return nil
}
