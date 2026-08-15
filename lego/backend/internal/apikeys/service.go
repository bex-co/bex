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
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// APIKey is a machine credential: an OAuth2 client in the platform's Hydra
// (docs/ADR012-auth.md). Secret is populated exactly once — on create — and never
// returned by list. CreatedBy/LastUsedAt are hygiene metadata (w4/m13): who
// minted the key, and when a token for it was last introspected.
type APIKey struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Secret     string `json:"secret,omitempty"` // only on create; store it — it is not retrievable
	CreatedAt  string `json:"createdAt"`
	CreatedBy  string `json:"createdBy,omitempty"`  // minting caller's identity subject
	LastUsedAt string `json:"lastUsedAt,omitempty"` // last introspection (RFC 3339); empty => never used
}

// APIKeyStore is the Service's seam to the OAuth2 client registry — Hydra's admin
// API in production (NewHydraAPIKeys), a fake in tests. nil => the api-key verbs
// report core.ErrAPIKeysUnavailable.
type APIKeyStore interface {
	Create(ctx context.Context, name, createdBy string) (APIKey, error)
	List(ctx context.Context) ([]APIKey, error)
	Delete(ctx context.Context, id string) error
	// Touch stamps the key `id` as used at `at`. Best-effort, called off the
	// request path (throttled by the Service); a no-op for non-api-key clients.
	Touch(ctx context.Context, id string, at time.Time) error
	// KeyOwner resolves key `id` to the identity subject that minted it (the
	// bex.co/created-by stamp, w4/m13) — the key→identity binding a machine
	// caller's GET /v1/users resolves its owning human through (w4/m25).
	// ok=false for an unknown/non-API-key client or a key minted before the
	// stamp existed.
	KeyOwner(ctx context.Context, id string) (subject string, ok bool)
}

// KeyBinder ties a minted API key to its tenant: the mapping row that lets a
// machine caller resolve to workspace:tea-<id>, plus the OpenFGA membership that
// lets it authorize there (replacing the legacy workspace:default). nil Binding
// (the store off) => keys mint unbound, as before tenant onboarding (w1/m9).
type KeyBinder interface {
	BindKey(ctx context.Context, clientID, tenantID string) error
	UnbindKey(ctx context.Context, clientID string) error
	// TenantForKey resolves the workspace a bound key belongs to (ok=false for
	// an unbound/unknown key) — the read side of BindKey, used to scope
	// ListAPIKeys/RevokeAPIKey to one workspace's own keys (w6/m18).
	TenantForKey(ctx context.Context, clientID string) (tenantID string, ok bool)
}

// Service manages machine credentials over the injected APIKeyStore.
type Service struct {
	*core.Base
	APIKeys APIKeyStore
	// Binding, when set (the control-plane store is on), ties each minted key to
	// the caller's tenant. nil => legacy unbound mint (store off).
	Binding KeyBinder
	// touch throttle: earliest next last-used write per key id. Guards against a
	// chatty caller turning every introspection into a Hydra write (w4/m13).
	touchMu   sync.Mutex
	nextTouch map[string]time.Time
}

// CreateAPIKey mints a new machine credential. The returned Secret is shown once,
// and the caller's identity (IdentityFrom) is stamped as the key's created-by.
// With the store on, the key is bound to ownerID's tenant (Render's `ownerId`,
// w6/m18; "" => the caller's default workspace, membership-checked via
// core.WithWorkspace like every other explicit-target verb) — a mapping row +
// the FGA membership that lets it authorize against workspace:tea-<id>. A
// session-less machine caller with no tenant is refused rather than minting an
// orphaned, tuple-less key. A bind failure rolls the Hydra client back so no
// half-bound credential lingers.
func (s *Service) CreateAPIKey(ctx context.Context, ownerID, name string) (APIKey, error) {
	// codex round-7 F3: minting a durable credential is reserved for direct
	// human callers (AuthorizeMintClass) — a machine token must not
	// self-replicate, and a consented third-party client must not persist past
	// its consent.
	if err := s.AuthorizeMintClass(ctx); err != nil {
		return APIKey{}, err
	}
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManageKeys); err != nil {
		return APIKey{}, err
	}
	// round-5 finding 4: minting a bound API key issues a durable credential with
	// its own lifetime, so re-assert can_manage_keys against the source of truth
	// (uncached). A caller whose key/role was revoked within the last PositiveTTL
	// must not ride a stale cached positive to mint a replacement key.
	if err := s.AuthorizeFresh(ctx, core.RelCanManageKeys); err != nil {
		return APIKey{}, err
	}
	if s.APIKeys == nil {
		return APIKey{}, core.ErrAPIKeysUnavailable
	}
	if strings.TrimSpace(name) == "" {
		return APIKey{}, core.ErrBadRequest
	}
	createdBy := ""
	if id, ok := core.IdentityFrom(ctx); ok {
		createdBy = id.Subject
	}
	if s.Binding == nil {
		return s.APIKeys.Create(ctx, name, createdBy) // store off — unbound, as before
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		return APIKey{}, fmt.Errorf("%w: caller has no tenant to bind the key to", core.ErrBadRequest)
	}
	key, err := s.APIKeys.Create(ctx, name, createdBy)
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

// touchThrottle is the minimum interval between last-used writes for one key.
const touchThrottle = time.Minute

// TouchAPIKey records that key `id` was just used — the auth gate calls it after
// a successful API-key introspection. It is deliberately NOT a Core verb: no
// Authorize (the platform is recording its own telemetry, not a caller action),
// no ctx/error return (fire-and-forget). It throttles to one store write per key
// per touchThrottle and runs the write on a background goroutine, so a request is
// never blocked on — nor charged for — a Hydra write. A non-api-key client id is
// filtered out by the store (isAPIKey) before any write.
func (s *Service) TouchAPIKey(id string) {
	if s.APIKeys == nil || id == "" {
		return
	}
	now := s.Now()
	s.touchMu.Lock()
	if next, ok := s.nextTouch[id]; ok && now.Before(next) {
		s.touchMu.Unlock()
		return
	}
	if s.nextTouch == nil {
		s.nextTouch = map[string]time.Time{}
	}
	// Drop entries whose window has elapsed so the map stays bounded by keys used
	// within the last touchThrottle, not by every client_id ever introspected
	// (dynamic client registration can mint unbounded distinct client_ids). This
	// runs only on the dispatch path (≤ once per key per window), so it's cheap.
	for k, next := range s.nextTouch {
		if !now.Before(next) {
			delete(s.nextTouch, k)
		}
	}
	s.nextTouch[id] = now.Add(touchThrottle)
	s.touchMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.APIKeys.Touch(ctx, id, now)
	}()
}

// ListAPIKeys returns ownerID's machine credentials (secrets omitted); ""
// means the caller's default workspace (w6/m18). With the store off (Binding
// nil), keys mint unbound and the list stays unfiltered, byte-identical to
// before — the legacy single-tenant mode. With the store on, a caller who
// resolves to no tenant sees an empty list rather than every workspace's keys
// (the same pattern apps.List uses).
func (s *Service) ListAPIKeys(ctx context.Context, ownerID string) ([]APIKey, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManageKeys); err != nil {
		return nil, err
	}
	if s.APIKeys == nil {
		return nil, core.ErrAPIKeysUnavailable
	}
	keys, err := s.APIKeys.List(ctx)
	if err != nil {
		return nil, err
	}
	tenantID, scoped := s.boundTenant(ctx)
	if !scoped {
		return keys, nil
	}
	if tenantID == "" {
		return []APIKey{}, nil
	}
	out := make([]APIKey, 0, len(keys))
	for _, k := range keys {
		if owner, ok := s.Binding.TenantForKey(ctx, k.ID); ok && owner == tenantID {
			out = append(out, k)
		}
	}
	return out, nil
}

// boundTenant resolves the workspace ListAPIKeys/RevokeAPIKey scope their key
// binding to. scoped=false means the store is off (Binding nil) — keys mint
// unbound, so there is no one workspace to scope to (List stays unfiltered,
// Revoke skips the ownership check); scoped=true with an empty tenantID means
// the caller has no resolvable tenant — scoped to nothing.
func (s *Service) boundTenant(ctx context.Context) (tenantID string, scoped bool) {
	if s.Binding == nil {
		return "", false
	}
	tenantID, _ = s.Tenant(ctx)
	return tenantID, true
}

// RevokeAPIKey deletes the credential; tokens already minted with it stop
// introspecting active (subject to bex-api's ≤30s introspection cache). ownerID
// ("" => the caller's default workspace, w6/m18) must be the key's OWN bound
// workspace — a caller who can manage keys in their own workspace may not
// revoke another workspace's (the same cross-workspace gate w6/m14 gave
// Apps/Databases/KeyValues, applied here since a key has no AuthorizeApp-style
// CRD to fetch through). With the store on it also drops the tenant binding +
// FGA tuple (best-effort: the Hydra client is already gone, so a leftover
// mapping is harmless, just cleaned up).
func (s *Service) RevokeAPIKey(ctx context.Context, ownerID, id string) error {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManageKeys); err != nil {
		return err
	}
	if s.APIKeys == nil {
		return core.ErrAPIKeysUnavailable
	}
	if tenantID, scoped := s.boundTenant(ctx); scoped {
		if tenantID == "" {
			return core.ErrForbidden
		}
		owner, boundOK := s.Binding.TenantForKey(ctx, id)
		if !boundOK {
			// An unbound/unknown key belongs to no workspace (w1/m53): the old
			// fall-through let a can_manage_keys caller in ANY workspace delete an
			// orphaned key. It isn't in the caller's workspace, so report not-found.
			return core.ErrNotFound
		}
		if owner != tenantID {
			return core.ErrForbidden
		}
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

// Hydra client `metadata` keys bex owns. apiKeyMarker stamps the Hydra clients
// bex minted: "API key" means exactly this subset — List hides and Delete/Touch
// refuse everything else, so platform clients (bex-bootstrap, OIDC apps) can't be
// listed away, revoked, or stamped here. createdByKey/lastUsedKey are the w4/m13
// hygiene fields.
const (
	apiKeyMarker = "bex.co/api-key"
	createdByKey = "bex.co/created-by"
	lastUsedKey  = "bex.co/last-used"
)

func isAPIKey(c hydraClient) bool {
	v, ok := c.Metadata[apiKeyMarker].(bool)
	return ok && v
}

func apiKeyFromHydra(c hydraClient) APIKey {
	k := APIKey{ID: c.ClientID, Name: c.ClientName, Secret: c.ClientSecret}
	if !c.CreatedAt.IsZero() {
		k.CreatedAt = c.CreatedAt.UTC().Format(time.RFC3339)
	}
	if v, ok := c.Metadata[createdByKey].(string); ok {
		k.CreatedBy = v
	}
	if v, ok := c.Metadata[lastUsedKey].(string); ok {
		k.LastUsedAt = v
	}
	return k
}

func (h *hydraAPIKeys) Create(ctx context.Context, name, createdBy string) (APIKey, error) {
	meta := map[string]any{apiKeyMarker: true}
	if createdBy != "" {
		meta[createdByKey] = createdBy
	}
	body, _ := json.Marshal(hydraClient{
		ClientName: name,
		GrantTypes: []string{"client_credentials"},
		AuthMethod: "client_secret_post",
		Metadata:   meta,
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

// getAPIKey fetches one Hydra client and gates on the bex API-key marker — the
// shared preamble of every verb that reads or acts on an existing key. A
// non-API-key client (platform/OIDC) is core.ErrNotFound, indistinguishable
// from an absent one, so no verb can list away, revoke, stamp, or resolve a
// client bex didn't mint.
func (h *hydraAPIKeys) getAPIKey(ctx context.Context, id string) (hydraClient, error) {
	var c hydraClient
	if err := h.do(ctx, http.MethodGet, "/admin/clients/"+id, nil, http.StatusOK, &c); err != nil {
		return hydraClient{}, err
	}
	if !isAPIKey(c) {
		return hydraClient{}, core.ErrNotFound
	}
	return c, nil
}

func (h *hydraAPIKeys) Delete(ctx context.Context, id string) error {
	// Only bex-minted keys are revocable here — deleting a platform client (e.g.
	// bex-bootstrap) through this endpoint would brick its owner.
	if _, err := h.getAPIKey(ctx, id); err != nil {
		return err
	}
	return h.do(ctx, http.MethodDelete, "/admin/clients/"+id, nil, http.StatusNoContent, nil)
}

// Touch stamps the key's last-used metadata. It first reads the client to confirm
// it is a bex API key (an unknown/revoked client or a platform/OIDC client is a
// silent no-op — the auth gate calls this for every introspected token, not only
// bex keys). The write is a JSON Patch (RFC 6902) `add`, which replaces the value
// when the pointer already exists; the key holds a slash, escaped per JSON Pointer
// (RFC 6901: "/" => "~1"). Best-effort by contract — the caller throttles + runs
// it off the request path, so a failed stamp only means a slightly stale last-used.
func (h *hydraAPIKeys) Touch(ctx context.Context, id string, at time.Time) error {
	if _, err := h.getAPIKey(ctx, id); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil // unknown/revoked or platform/OIDC client — nothing to stamp
		}
		return err
	}
	patch, _ := json.Marshal([]map[string]any{{
		"op":    "add",
		"path":  "/metadata/" + jsonPointerEscape(lastUsedKey),
		"value": at.UTC().Format(time.RFC3339),
	}})
	return h.do(ctx, http.MethodPatch, "/admin/clients/"+id, patch, http.StatusOK, nil)
}

// KeyOwner reads back the client's created-by stamp. Any failure (unknown
// client, not a bex API key, no stamp, Hydra unreachable) is ok=false — the
// honest-omit contract workspaces.KeyOwnerReader documents; the caller then
// degrades to its earliest-admin-email fallback rather than failing the request.
func (h *hydraAPIKeys) KeyOwner(ctx context.Context, id string) (string, bool) {
	c, err := h.getAPIKey(ctx, id)
	if err != nil {
		return "", false
	}
	k := apiKeyFromHydra(c)
	return k.CreatedBy, k.CreatedBy != ""
}

// jsonPointerEscape escapes a metadata key for use as a JSON Pointer path segment
// (RFC 6901): "~" => "~0" first, then "/" => "~1".
func jsonPointerEscape(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	return strings.ReplaceAll(s, "/", "~1")
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
