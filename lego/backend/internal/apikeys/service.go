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

// Package apikeys is the machine-credential feature: OAuth2 clients
// (client_credentials grant) in the platform's Hydra. The Service gates + guards;
// the Hydra admin store is the injected seam. One implementation, three surfaces.
package apikeys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// APIKey is a machine credential: an OAuth2 client in the platform's Hydra
// (docs/auth.md). Secret is populated exactly once — on create — and never
// returned by list.
type APIKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Secret    string `json:"secret,omitempty"` // only on create; store it — it is not retrievable
	CreatedAt string `json:"createdAt"`
}

// APIKeyStore is the Service's seam to the OAuth2 client registry — Hydra's admin
// API in production (NewHydraAPIKeys), a fake in tests. nil => the api-key verbs
// report core.ErrAPIKeysUnavailable.
type APIKeyStore interface {
	Create(ctx context.Context, name string) (APIKey, error)
	List(ctx context.Context) ([]APIKey, error)
	Delete(ctx context.Context, id string) error
}

// KeyBinder ties a minted API key to its tenant: the mapping row that lets a
// machine caller resolve to workspace:tea-<id>, plus the OpenFGA membership that
// lets it authorize there (replacing the legacy workspace:default). nil Binding
// (the store off) => keys mint unbound, as before tenant onboarding (w1/m9).
type KeyBinder interface {
	BindKey(ctx context.Context, clientID, tenantID string) error
	UnbindKey(ctx context.Context, clientID string) error
}

// Service manages machine credentials over the injected APIKeyStore.
type Service struct {
	*core.Base
	APIKeys APIKeyStore
	// Binding, when set (the control-plane store is on), ties each minted key to
	// the caller's tenant. nil => legacy unbound mint (store off).
	Binding KeyBinder
}

// CreateAPIKey mints a new machine credential. The returned Secret is shown once.
// With the store on, the key is bound to the caller's tenant (a mapping row +
// the FGA membership that lets it authorize against workspace:tea-<id>); a
// session-less machine caller with no tenant is refused rather than minting an
// orphaned, tuple-less key. A bind failure rolls the Hydra client back so no
// half-bound credential lingers.
func (s *Service) CreateAPIKey(ctx context.Context, name string) (APIKey, error) {
	if err := s.Authorize(ctx, core.RelCanManageKeys); err != nil {
		return APIKey{}, err
	}
	if s.APIKeys == nil {
		return APIKey{}, core.ErrAPIKeysUnavailable
	}
	if strings.TrimSpace(name) == "" {
		return APIKey{}, core.ErrBadRequest
	}
	if s.Binding == nil {
		return s.APIKeys.Create(ctx, name) // store off — unbound, as before
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		return APIKey{}, fmt.Errorf("%w: caller has no tenant to bind the key to", core.ErrBadRequest)
	}
	key, err := s.APIKeys.Create(ctx, name)
	if err != nil {
		return APIKey{}, err
	}
	if err := s.Binding.BindKey(ctx, key.ID, tenantID); err != nil {
		// No orphaned credentials: the Hydra client is live but unbound — delete
		// it so a half-minted key can't authenticate (its tuple was never written).
		_ = s.APIKeys.Delete(ctx, key.ID)
		return APIKey{}, fmt.Errorf("bind api key to tenant: %w", err)
	}
	return key, nil
}

// ListAPIKeys returns every machine credential (secrets omitted).
func (s *Service) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	if err := s.Authorize(ctx, core.RelCanManageKeys); err != nil {
		return nil, err
	}
	if s.APIKeys == nil {
		return nil, core.ErrAPIKeysUnavailable
	}
	return s.APIKeys.List(ctx)
}

// RevokeAPIKey deletes the credential; tokens already minted with it stop
// introspecting active (subject to bex-api's ≤30s introspection cache). With the
// store on it also drops the tenant binding + FGA tuple (best-effort: the Hydra
// client is already gone, so a leftover mapping is harmless, just cleaned up).
func (s *Service) RevokeAPIKey(ctx context.Context, id string) error {
	if err := s.Authorize(ctx, core.RelCanManageKeys); err != nil {
		return err
	}
	if s.APIKeys == nil {
		return core.ErrAPIKeysUnavailable
	}
	if err := s.APIKeys.Delete(ctx, id); err != nil {
		return err
	}
	if s.Binding != nil {
		_ = s.Binding.UnbindKey(ctx, id)
	}
	return nil
}

// hydraAPIKeys implements APIKeyStore over Hydra's admin API.
type hydraAPIKeys struct {
	adminURL string
	client   *http.Client
}

// NewHydraAPIKeys returns the production APIKeyStore talking to Hydra's
// cluster-internal admin API.
func NewHydraAPIKeys(adminURL string) APIKeyStore {
	return &hydraAPIKeys{
		adminURL: strings.TrimSuffix(adminURL, "/"),
		client:   &http.Client{Timeout: 10 * time.Second, Transport: core.OryTransport},
	}
}

// hydraClient is the subset of Hydra's OAuth2 client object bex reads/writes.
type hydraClient struct {
	ClientID     string         `json:"client_id,omitempty"`
	ClientName   string         `json:"client_name,omitempty"`
	ClientSecret string         `json:"client_secret,omitempty"`
	GrantTypes   []string       `json:"grant_types,omitempty"`
	AuthMethod   string         `json:"token_endpoint_auth_method,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at,omitzero"`
}

// apiKeyMarker stamps the Hydra clients bex minted. "API key" means exactly this
// subset: List hides and Delete refuses everything else, so platform clients
// (bex-bootstrap, future OIDC apps) can't be listed away or revoked here.
const apiKeyMarker = "bex.co/api-key"

func isAPIKey(c hydraClient) bool {
	v, ok := c.Metadata[apiKeyMarker].(bool)
	return ok && v
}

func apiKeyFromHydra(c hydraClient) APIKey {
	created := ""
	if !c.CreatedAt.IsZero() {
		created = c.CreatedAt.UTC().Format(time.RFC3339)
	}
	return APIKey{ID: c.ClientID, Name: c.ClientName, Secret: c.ClientSecret, CreatedAt: created}
}

func (h *hydraAPIKeys) Create(ctx context.Context, name string) (APIKey, error) {
	body, _ := json.Marshal(hydraClient{
		ClientName: name,
		GrantTypes: []string{"client_credentials"},
		AuthMethod: "client_secret_post",
		Metadata:   map[string]any{apiKeyMarker: true},
	})
	var out hydraClient
	if err := h.do(ctx, http.MethodPost, "/admin/clients", body, http.StatusCreated, &out); err != nil {
		return APIKey{}, err
	}
	return apiKeyFromHydra(out), nil
}

func (h *hydraAPIKeys) List(ctx context.Context) ([]APIKey, error) {
	var clients []hydraClient
	if err := h.do(ctx, http.MethodGet, "/admin/clients?page_size=500", nil, http.StatusOK, &clients); err != nil {
		return nil, err
	}
	keys := make([]APIKey, 0, len(clients))
	for _, c := range clients {
		if !isAPIKey(c) {
			continue // platform clients (bootstrap, OIDC apps) are not API keys
		}
		c.ClientSecret = "" // never surface secrets from list (Hydra omits them anyway)
		keys = append(keys, apiKeyFromHydra(c))
	}
	return keys, nil
}

func (h *hydraAPIKeys) Delete(ctx context.Context, id string) error {
	// Only bex-minted keys are revocable here — deleting a platform client (e.g.
	// bex-bootstrap) through this endpoint would brick its owner.
	var c hydraClient
	if err := h.do(ctx, http.MethodGet, "/admin/clients/"+id, nil, http.StatusOK, &c); err != nil {
		return err
	}
	if !isAPIKey(c) {
		return core.ErrNotFound
	}
	return h.do(ctx, http.MethodDelete, "/admin/clients/"+id, nil, http.StatusNoContent, nil)
}

// do runs one Hydra admin call (shared core.DoJSON mechanics), mapping 404 to
// core.ErrNotFound.
func (h *hydraAPIKeys) do(ctx context.Context, method, path string, body []byte, want int, out any) error {
	err := core.DoJSON(ctx, h.client, method, h.adminURL+path, "", body, want, out)
	var se *core.HTTPStatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return core.ErrNotFound
	}
	return err
}
