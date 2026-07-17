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

// Package registrycreds is the registry-credentials feature (w2/m14): a
// workspace's stored credentials for a private external image registry
// (Docker Hub, GHCR, GitLab Container Registry, ECR, etc.), so an
// existing-image service can pull from a non-public, non-Zot source —
// closing bex's only-public-or-internal-Zot image-source limitation
// (docs/ADR018-render-parity.md, "Registry credentials").
//
// A credential's metadata (host, username, expiry, workspace) lives in the
// control-plane Postgres store (internal/store); the actual secret value
// lives in OpenBao (docs/ADR013-secrets.md — the same split the env-vars/
// secret-files feature uses for exactly this class of tenant credential).
// That split makes "the secret is never returned by a read/list query" a
// structural property, not an adapter's policy choice: the secret simply
// isn't a column any read query ever touches. The Service gates + guards;
// both stores are injected seams. One implementation, three surfaces
// (REST/GraphQL/MCP).
package registrycreds

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// CredentialStore is the Service's seam to the control-plane store — the
// metadata rows. *store.PGStore satisfies it; a fake backs the tests. nil =>
// the control-plane store is off (BEX_CP_DB_URI unset).
type CredentialStore interface {
	CreateRegistryCredential(ctx context.Context, workspaceID, name, host, username, createdBy string, expiresAt *time.Time) (store.RegistryCredential, error)
	ListRegistryCredentials(ctx context.Context, workspaceID string) ([]store.RegistryCredential, error)
	GetRegistryCredential(ctx context.Context, workspaceID, id string) (store.RegistryCredential, error)
	GetRegistryCredentialByID(ctx context.Context, id string) (store.RegistryCredential, error)
	GetRegistryCredentialsByIDs(ctx context.Context, ids []string) ([]store.RegistryCredential, error)
	GetRegistryCredentialByHost(ctx context.Context, workspaceID, host string) (store.RegistryCredential, error)
	UpdateRegistryCredential(ctx context.Context, workspaceID, id, name, username string, expiresAt *time.Time) (store.RegistryCredential, error)
	TouchRegistryCredential(ctx context.Context, workspaceID, id string) error
	DeleteRegistryCredential(ctx context.Context, workspaceID, id string) error
}

// Service manages a workspace's registry credentials over the injected
// control-plane store (metadata) and OpenBao-backed secret store (the actual
// password/token). Both seams must be present; either nil => 503.
type Service struct {
	*core.Base
	Store  CredentialStore
	Secret core.SecretKV
}

// CredentialView is the neutral shape every adapter renders. Secret is never
// populated by List/Get — a credential's secret value is write-only from the
// API's perspective past creation (t001's acceptance bar).
type CredentialView struct {
	ID   string
	Name string
	Host string
	// OwnerID is the workspace this credential belongs to — Render's
	// `ownerId` scoping field, mirroring AppView/KeyValueView's OwnerID.
	OwnerID   string
	Username  string
	ExpiresAt string // RFC3339, or "" when the credential never expires
	// Status is computed from ExpiresAt vs. the current time (w2/m14/t007):
	// "active" (no expiry, or not yet near it), "expiring_soon" (within the
	// next hour — ECR-style short-lived tokens are the motivating case), or
	// "expired". Never hidden — a stale credential must be visible, not
	// silently let pulls start failing.
	Status    string
	CreatedBy string
	CreatedAt string
	UpdatedAt string
}

// expiringSoonWindow is how far ahead of expiry a credential is flagged
// "expiring_soon" — ECR's 12h token rotation is the motivating case Render's
// own docs call out; a caller checking this field should have time to act.
const expiringSoonWindow = time.Hour

func (s *Service) configured() bool { return s.Store != nil && s.Secret != nil }

// workspaceID is the store key for the caller's credentials: the caller's
// tenant when resolvable, else the single-workspace default (mirrors
// github.Service.workspaceID).
func (s *Service) workspaceID(ctx context.Context) string {
	if tid, ok := s.Tenant(ctx); ok && tid != "" {
		return tid
	}
	return core.DefaultTenant
}

// secretPath is a credential's location in OpenBao, workspace-scoped within
// bex's own logical-path space (docs/ADR013-secrets.md §4 layout, minus the
// mount/tenant prefix the store prepends) — defense in depth alongside the
// Postgres row's own workspace_id scoping.
func secretPath(workspaceID, id string) string {
	return "registry-credentials/" + workspaceID + "/" + id
}

func toView(c store.RegistryCredential, now time.Time) CredentialView {
	v := CredentialView{
		ID:        c.ID,
		Name:      c.Name,
		Host:      c.Host,
		OwnerID:   c.WorkspaceID,
		Username:  c.Username,
		CreatedBy: c.CreatedBy,
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.UTC().Format(time.RFC3339),
		Status:    "active",
	}
	if c.ExpiresAt != nil {
		v.ExpiresAt = c.ExpiresAt.UTC().Format(time.RFC3339)
		switch {
		case !now.Before(*c.ExpiresAt):
			v.Status = "expired"
		case c.ExpiresAt.Sub(now) <= expiringSoonWindow:
			v.Status = "expiring_soon"
		}
	}
	return v
}

// List returns the workspace's stored credentials (secrets omitted; Render's
// GET /registrycredentials shape). ownerID optionally names a specific
// workspace to list (Render's `ownerId` filter) — empty means the caller's
// own resolved default workspace, mirroring apps.Service.List/
// keyvalue.Service.CreateKeyValue's core.WithWorkspace override. Member read.
func (s *Service) List(ctx context.Context, ownerID string) ([]CredentialView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrRegistryCredentialsUnavailable
	}
	rows, err := s.Store.ListRegistryCredentials(ctx, s.workspaceID(ctx))
	if err != nil {
		return nil, err
	}
	now := s.Now()
	out := make([]CredentialView, 0, len(rows))
	for _, c := range rows {
		out = append(out, toView(c, now))
	}
	return out, nil
}

// Get returns one credential (secret omitted). Member read.
func (s *Service) Get(ctx context.Context, id string) (CredentialView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return CredentialView{}, err
	}
	if s.Store == nil {
		return CredentialView{}, core.ErrRegistryCredentialsUnavailable
	}
	c, err := s.Store.GetRegistryCredential(ctx, s.workspaceID(ctx), id)
	if err != nil {
		return CredentialView{}, mapStoreErr(err)
	}
	return toView(c, s.Now()), nil
}

// CreateRequest is Create's input. Host/Username/Secret are required; Name is
// a human display label (Render's registryCredential.name) and defaults to
// Host when blank. OwnerID is the workspace to create in — Render's
// `ownerId` (mirrors keyvalue.CreateKeyValueRequest.OwnerID); empty means the
// caller's own resolved default workspace.
type CreateRequest struct {
	OwnerID   string
	Name      string
	Host      string
	Username  string
	Secret    string
	ExpiresAt *time.Time
}

// Create stores a new credential: the metadata row in Postgres, the secret in
// OpenBao. Admin-only (RelCanManage) — a registry credential is workspace
// infrastructure, the same bar github.Service's Connect holds. Host/Username/
// Secret are required; a blank one is rejected before either store is
// touched, so a bad request never partially writes.
func (s *Service) Create(ctx context.Context, req CreateRequest) (CredentialView, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return CredentialView{}, err
	}
	if !s.configured() {
		return CredentialView{}, core.ErrRegistryCredentialsUnavailable
	}
	host := strings.TrimSpace(req.Host)
	username := strings.TrimSpace(req.Username)
	if host == "" || username == "" || req.Secret == "" {
		return CredentialView{}, fmt.Errorf("%w: host, username, and secret are required", core.ErrBadRequest)
	}
	createdBy := ""
	if id, ok := core.IdentityFrom(ctx); ok {
		createdBy = id.Subject
	}
	workspaceID := s.workspaceID(ctx)
	c, err := s.Store.CreateRegistryCredential(ctx, workspaceID, strings.TrimSpace(req.Name), host, username, createdBy, req.ExpiresAt)
	if err != nil {
		return CredentialView{}, err
	}
	if err := s.Secret.Put(ctx, secretPath(workspaceID, c.ID), map[string]string{"password": req.Secret}); err != nil {
		// No orphaned metadata: the row exists but its secret failed to write —
		// delete it rather than leave a credential that can never materialize a
		// pull secret (mirrors apikeys.CreateAPIKey's bind-failure rollback).
		_ = s.Store.DeleteRegistryCredential(ctx, workspaceID, c.ID)
		return CredentialView{}, fmt.Errorf("store registry credential secret: %w", err)
	}
	return toView(c, s.Now()), nil
}

// UpdateRequest is Update's input. Name/Username/ExpiresAt follow the
// SetCronJob convention (nil pointer = leave unchanged); Secret nil means
// "don't rotate" — a non-nil, non-empty Secret writes a new OpenBao value.
// ExpiresAt uses the same nil/empty/set three-state as SetCronJob's Schedule/
// Command: nil = unchanged, a pointer to the zero Time = clear the expiry, a
// pointer to a real Time = set it (adapters parse the wire "" vs RFC3339
// distinction into this before calling Update).
type UpdateRequest struct {
	Name         *string
	Username     *string
	Secret       *string
	ExpiresAtSet bool
	ExpiresAt    *time.Time
}

// Update changes a credential's name/username/expiry and/or rotates its
// secret. Admin-only, matching Create.
func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (CredentialView, error) {
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return CredentialView{}, err
	}
	if !s.configured() {
		return CredentialView{}, core.ErrRegistryCredentialsUnavailable
	}
	workspaceID := s.workspaceID(ctx)
	existing, err := s.Store.GetRegistryCredential(ctx, workspaceID, id)
	if err != nil {
		return CredentialView{}, mapStoreErr(err)
	}
	name := existing.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return CredentialView{}, fmt.Errorf("%w: name cannot be empty", core.ErrBadRequest)
		}
	}
	username := existing.Username
	if req.Username != nil {
		username = strings.TrimSpace(*req.Username)
		if username == "" {
			return CredentialView{}, fmt.Errorf("%w: username cannot be empty", core.ErrBadRequest)
		}
	}
	expiresAt := existing.ExpiresAt
	if req.ExpiresAtSet {
		expiresAt = req.ExpiresAt
	}
	updated, err := s.Store.UpdateRegistryCredential(ctx, workspaceID, id, name, username, expiresAt)
	if err != nil {
		return CredentialView{}, mapStoreErr(err)
	}
	if req.Secret != nil {
		if *req.Secret == "" {
			return CredentialView{}, fmt.Errorf("%w: secret cannot be set to empty", core.ErrBadRequest)
		}
		if err := s.Secret.Put(ctx, secretPath(workspaceID, id), map[string]string{"password": *req.Secret}); err != nil {
			return CredentialView{}, fmt.Errorf("rotate registry credential secret: %w", err)
		}
	}
	return toView(updated, s.Now()), nil
}

// Delete removes a credential: the Postgres row and its OpenBao secret.
// Best-effort on the secret (mirrors RevokeAPIKey/Disconnect) — a Postgres row
// is the source of truth for "does this credential exist"; a leftover OpenBao
// value under a dead id is harmless (unreachable, since nothing can look it up
// by id without the deleted row). Admin-only, matching Create/Update.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return err
	}
	if !s.configured() {
		return core.ErrRegistryCredentialsUnavailable
	}
	workspaceID := s.workspaceID(ctx)
	if err := s.Store.DeleteRegistryCredential(ctx, workspaceID, id); err != nil {
		return mapStoreErr(err)
	}
	_ = s.Secret.Delete(ctx, secretPath(workspaceID, id))
	return nil
}

// mapStoreErr maps the store's ErrNotFound (a bare sentinel, wrapped with a
// "registry credential: " prefix by store.classify) onto core.ErrNotFound so
// the REST/GraphQL/MCP adapters' shared error mapping applies. Any other
// error passes through unchanged.
func mapStoreErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return core.ErrNotFound
	}
	return err
}
