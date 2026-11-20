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

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// CredentialStore is the Postgres metadata & version-history store for sandbox
// group credentials (w2/m532, ADR122). *store.PGStore implements it.
type CredentialStore interface {
	CredentialSecretRefStore
	ListSandboxGroupCredentials(ctx context.Context, workspaceID, groupID string) ([]store.SandboxCredential, error)
	GetSandboxGroupCredential(ctx context.Context, workspaceID, groupID, credID string) (store.SandboxCredential, error)
	ListSandboxGroupCredentialVersions(ctx context.Context, workspaceID, groupID, credID string) ([]store.SandboxCredentialVersion, error)
	CreateSandboxGroupCredential(ctx context.Context, workspaceID, groupID, credID string, prepare store.SandboxCredentialPrepareCreateFn) (store.SandboxCredential, string, error)
	UpdateSandboxGroupCredential(ctx context.Context, workspaceID, groupID, credID string, prepare store.SandboxCredentialPrepareUpdateFn) (store.SandboxCredential, []string, string, error)
	RestoreSandboxGroupCredential(ctx context.Context, workspaceID, groupID, credID string, targetVersionID int64, updatedBy string) (store.SandboxCredential, []string, error)
	DeleteSandboxGroupCredential(ctx context.Context, workspaceID, groupID, credID string, onBeforeCommit func(ctx context.Context, secretRefs []string) error) ([]string, bool, error)
}

// GroupCredentialInjection matches Render's injection settings schema.
type GroupCredentialInjection struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Template string `json:"template,omitempty"`
}

// GroupCredentialVersion matches Render's credential version metadata schema.
type GroupCredentialVersion struct {
	ID                    int64     `json:"id"`
	UpdatedAt             time.Time `json:"updatedAt"`
	UpdatedBy             string    `json:"updatedBy"`
	Fingerprint           string    `json:"fingerprint"`
	RestoredFromVersionID *int64    `json:"restoredFromVersionId,omitempty"`
}

// GroupCredential matches Render's sandbox group credential response schema.
// Secret values are never present on this type or any read response.
type GroupCredential struct {
	ID        string                   `json:"id"`
	Domain    string                   `json:"domain"`
	Version   GroupCredentialVersion   `json:"version"`
	Enabled   bool                     `json:"enabled"`
	Methods   []string                 `json:"methods"`
	Paths     []string                 `json:"paths"`
	Injection GroupCredentialInjection `json:"injection"`
}

// CreateGroupCredentialRequest is the body of POST /v1/sandbox-groups/{id}/credentials.
type CreateGroupCredentialRequest struct {
	ID        string                    `json:"id"`
	Domain    string                    `json:"domain"`
	Value     string                    `json:"value"`
	Enabled   *bool                     `json:"enabled,omitempty"`
	Methods   []string                  `json:"methods,omitempty"`
	Paths     []string                  `json:"paths,omitempty"`
	Injection *GroupCredentialInjection `json:"injection"`
}

// UpdateGroupCredentialRequest is the body of PATCH /v1/sandbox-groups/{id}/credentials/{credId}.
// Pointer fields distinguish omitted (keep current) from explicitly set (replace / clear).
type UpdateGroupCredentialRequest struct {
	Value     *string                   `json:"value,omitempty"`
	Domain    *string                   `json:"domain,omitempty"`
	Enabled   *bool                     `json:"enabled,omitempty"`
	Methods   *[]string                 `json:"methods,omitempty"`
	Paths     *[]string                 `json:"paths,omitempty"`
	Injection *GroupCredentialInjection `json:"injection,omitempty"`
}

// RestoreGroupCredentialRequest is the body of POST /v1/sandbox-groups/{id}/credentials/{credId}/restore.
type RestoreGroupCredentialRequest struct {
	VersionID int64 `json:"versionId"`
}

var errCredentialNoOp = errors.New("sandbox credential update produced no changes")

func (s *Service) requireCredentials() error {
	if s.Groups == nil || s.Credentials == nil || s.CredentialVault == nil || !s.CredentialVault.enabled() {
		return errCredentialsNotConfigured()
	}
	return nil
}

// credentialGroup scopes the request to the sandbox group's workspace, runs the
// verb's single authorization check, verifies credential storage is configured,
// and loads the group (answering 404 for absent or foreign groups).
func (s *Service) credentialGroup(ctx context.Context, groupID, rel string) (context.Context, store.SandboxGroup, error) {
	ctx, err := s.scopeGroup(ctx, groupID)
	if err != nil {
		return ctx, store.SandboxGroup{}, err
	}
	if err := s.Authorize(ctx, rel); err != nil {
		return ctx, store.SandboxGroup{}, err
	}
	if err := s.requireCredentials(); err != nil {
		return ctx, store.SandboxGroup{}, err
	}
	g, err := s.storedGroup(ctx, groupID)
	return ctx, g, err
}

func credentialNotFound(credID string) error {
	return core.NewNotFoundError(CodeSandboxCredentialNotFound, "sandbox group credential not found", map[string]any{"id": credID})
}

func credentialVersionNotFound(versionID int64) error {
	return core.NewNotFoundError(CodeSandboxCredentialVersionNotFound, "sandbox group credential version not found", map[string]any{"versionId": versionID})
}

func mapCredentialStoreErr(err error, credID string, versionID int64) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return credentialNotFound(credID)
	case errors.Is(err, store.ErrSandboxCredentialVersionNotFound):
		return credentialVersionNotFound(versionID)
	case errors.Is(err, store.ErrSandboxCredentialExists):
		return core.NewConflictError(CodeSandboxCredentialExists,
			fmt.Sprintf("credential %q already exists in this sandbox group", credID), map[string]any{"id": credID})
	case errors.Is(err, store.ErrSandboxCredentialLimit):
		return core.NewBadRequestError(CodeSandboxCredentialInvalid,
			fmt.Sprintf("sandbox group is at its limit of %d credentials", store.SandboxCredentialMaxPerGroup),
			map[string]any{"limit": store.SandboxCredentialMaxPerGroup})
	case errors.Is(err, store.ErrSandboxCredentialCurrentSizeLimit):
		return core.NewBadRequestError(CodeSandboxCredentialInvalid,
			fmt.Sprintf("combined serialized size of current credentials in this group exceeds %d bytes", store.SandboxCredentialMaxCurrentBytes),
			map[string]any{"maxBytes": store.SandboxCredentialMaxCurrentBytes})
	case errors.Is(err, store.ErrSandboxCredentialTotalSizeLimit):
		return core.NewBadRequestError(CodeSandboxCredentialInvalid,
			fmt.Sprintf("total credential storage including version history in this group exceeds %d bytes", store.SandboxCredentialMaxTotalBytes),
			map[string]any{"maxBytes": store.SandboxCredentialMaxTotalBytes})
	case errors.Is(err, store.ErrSandboxCredentialBusy):
		return core.NewTooManyRequestsError(CodeSandboxCredentialBusy,
			"another credential update is in progress for this sandbox group; retry shortly",
			map[string]any{"retryAfterSeconds": 1})
	default:
		return store.MapError(err)
	}
}

func credentialActorSubject(ctx context.Context) string {
	if id, ok := core.IdentityFrom(ctx); ok && id.Subject != "" {
		return id.Subject
	}
	return "unknown"
}

func (s *Service) credentialVersionViews(ctx context.Context, versions []store.SandboxCredentialVersion) ([]GroupCredentialVersion, error) {
	subjects := make([]string, 0, len(versions))
	for _, v := range versions {
		if v.UpdatedBy != "" {
			subjects = append(subjects, v.UpdatedBy)
		}
	}
	slices.Sort(subjects)
	users, err := core.PublicUserIDs(ctx, s.UserIDs, slices.Compact(subjects))
	if err != nil {
		return nil, err
	}
	out := make([]GroupCredentialVersion, len(versions))
	for i, v := range versions {
		updatedBy := v.UpdatedBy
		if pub, ok := users[v.UpdatedBy]; ok && pub != "" {
			updatedBy = pub
		}
		out[i] = GroupCredentialVersion{
			ID:                    v.ID,
			UpdatedAt:             v.UpdatedAt.UTC(),
			UpdatedBy:             updatedBy,
			Fingerprint:           v.Fingerprint,
			RestoredFromVersionID: v.RestoredFromVersionID,
		}
	}
	return out, nil
}

func (s *Service) credentialViews(ctx context.Context, rows []store.SandboxCredential) ([]GroupCredential, error) {
	versions := make([]store.SandboxCredentialVersion, len(rows))
	for i, r := range rows {
		versions[i] = r.CurrentVersion
	}
	vViews, err := s.credentialVersionViews(ctx, versions)
	if err != nil {
		return nil, err
	}
	out := make([]GroupCredential, len(rows))
	for i, r := range rows {
		methods := r.Config.Methods
		if methods == nil {
			methods = []string{}
		}
		paths := r.Config.Paths
		if paths == nil {
			paths = []string{}
		}
		out[i] = GroupCredential{
			ID:      r.ID,
			Domain:  r.Config.Domain,
			Version: vViews[i],
			Enabled: r.Config.Enabled,
			Methods: methods,
			Paths:   paths,
			Injection: GroupCredentialInjection{
				Type:     r.Config.Injection.Type,
				Name:     r.Config.Injection.Name,
				Template: r.Config.Injection.Template,
			},
		}
	}
	return out, nil
}

func (s *Service) credentialView(ctx context.Context, row store.SandboxCredential) (GroupCredential, error) {
	views, err := s.credentialViews(ctx, []store.SandboxCredential{row})
	if err != nil {
		return GroupCredential{}, err
	}
	return views[0], nil
}

func credentialConfigsEqual(a, b store.SandboxCredentialConfig) bool {
	ca := store.CanonicalSandboxCredentialConfig(a)
	cb := store.CanonicalSandboxCredentialConfig(b)
	return ca.Domain == cb.Domain &&
		ca.Enabled == cb.Enabled &&
		slices.Equal(ca.Methods, cb.Methods) &&
		slices.Equal(ca.Paths, cb.Paths) &&
		ca.Injection == cb.Injection
}

// ListGroupCredentials lists all credentials of a sandbox group, sorted
// lexicographically by ID, without secret values. Requires Viewer (`can_view`).
func (s *Service) ListGroupCredentials(ctx context.Context, groupID string) ([]GroupCredential, error) {
	ctx, group, err := s.credentialGroup(ctx, groupID, core.RelCanView)
	if err != nil {
		return nil, err
	}
	rows, err := s.Credentials.ListSandboxGroupCredentials(ctx, group.WorkspaceID, group.ID)
	if err != nil {
		return nil, store.MapError(err)
	}
	return s.credentialViews(ctx, rows)
}

// GetGroupCredential returns one credential's metadata and settings (no secret
// value). Requires Viewer (`can_view`).
func (s *Service) GetGroupCredential(ctx context.Context, groupID, credID string) (GroupCredential, error) {
	ctx, group, err := s.credentialGroup(ctx, groupID, core.RelCanView)
	if err != nil {
		return GroupCredential{}, err
	}
	if err := ValidateCredentialID(credID); err != nil {
		return GroupCredential{}, err
	}
	row, err := s.Credentials.GetSandboxGroupCredential(ctx, group.WorkspaceID, group.ID, credID)
	if err != nil {
		return GroupCredential{}, mapCredentialStoreErr(err, credID, 0)
	}
	return s.credentialView(ctx, row)
}

// CreateGroupCredential creates a credential in a sandbox group. Requires Admin
// (`can_manage`). Secret values are written to OpenBao before Postgres commits;
// a failed commit cleans up the newly written OpenBao entry.
func (s *Service) CreateGroupCredential(ctx context.Context, groupID string, req CreateGroupCredentialRequest) (GroupCredential, error) {
	ctx, group, err := s.credentialGroup(ctx, groupID, core.RelCanManage)
	if err != nil {
		return GroupCredential{}, err
	}
	if err := ValidateCredentialID(req.ID); err != nil {
		return GroupCredential{}, err
	}
	domain, err := NormalizeCredentialDomain(req.Domain)
	if err != nil {
		return GroupCredential{}, err
	}
	if err := ValidateCredentialValue(req.Value); err != nil {
		return GroupCredential{}, err
	}
	methods, err := NormalizeCredentialMethods(req.Methods)
	if err != nil {
		return GroupCredential{}, err
	}
	paths, err := NormalizeCredentialPaths(req.Paths)
	if err != nil {
		return GroupCredential{}, err
	}
	inj, err := ValidateCredentialInjection(req.Injection, req.Value)
	if err != nil {
		return GroupCredential{}, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	cfg := store.SandboxCredentialConfig{
		Domain:    domain,
		Enabled:   enabled,
		Methods:   methods,
		Paths:     paths,
		Injection: inj,
	}
	actor := credentialActorSubject(ctx)

	created, wroteRef, err := s.Credentials.CreateSandboxGroupCredential(ctx, group.WorkspaceID, group.ID, req.ID,
		func(_ context.Context, salt []byte) (store.SandboxCredentialPreparedWrite, error) {
			fp := store.SandboxCredentialFingerprint(salt, cfg, req.Value)
			secretRef := store.SandboxCredentialSecretRef(salt, req.Value)
			return store.SandboxCredentialPreparedWrite{
				Config:      cfg,
				UpdatedBy:   actor,
				Fingerprint: fp,
				SecretRef:   secretRef,
				SecretBytes: len(req.Value),
				OnBeforeCommit: func(txCtx context.Context, ref string, _ bool) error {
					return s.CredentialVault.Put(txCtx, group.WorkspaceID, group.ID, req.ID, ref, req.Value)
				},
			}, nil
		})
	if err != nil {
		if wroteRef != "" {
			if cleanErr := s.CredentialVault.Delete(context.WithoutCancel(ctx), group.WorkspaceID, group.ID, req.ID, wroteRef); cleanErr != nil {
				log.Printf("sandbox credentials: rollback cleanup failed for group=%s: %v", group.ID, cleanErr)
			}
		}
		if core.IsPublicError(err) {
			return GroupCredential{}, err
		}
		return GroupCredential{}, mapCredentialStoreErr(err, req.ID, 0)
	}
	return s.credentialView(ctx, created)
}

// UpdateGroupCredential updates a credential with split RBAC (ADR122 D4):
// Developers (`can_create`) may update `injection`, `methods`, `paths`, and
// `enabled`; changing `domain` or `value` requires an Admin (`can_manage`).
func (s *Service) UpdateGroupCredential(ctx context.Context, groupID, credID string, req UpdateGroupCredentialRequest) (GroupCredential, error) {
	rel := core.RelCanCreate
	if req.Domain != nil || req.Value != nil {
		rel = core.RelCanManage
	}
	ctx, group, err := s.credentialGroup(ctx, groupID, rel)
	if err != nil {
		return GroupCredential{}, err
	}
	if err := ValidateCredentialID(credID); err != nil {
		return GroupCredential{}, err
	}

	var newDomain string
	if req.Domain != nil {
		newDomain, err = NormalizeCredentialDomain(*req.Domain)
		if err != nil {
			return GroupCredential{}, err
		}
	}
	if req.Value != nil {
		if err := ValidateCredentialValue(*req.Value); err != nil {
			return GroupCredential{}, err
		}
	}
	var newMethods []string
	if req.Methods != nil {
		newMethods, err = NormalizeCredentialMethods(*req.Methods)
		if err != nil {
			return GroupCredential{}, err
		}
	}
	var newPaths []string
	if req.Paths != nil {
		newPaths, err = NormalizeCredentialPaths(*req.Paths)
		if err != nil {
			return GroupCredential{}, err
		}
	}
	actor := credentialActorSubject(ctx)
	var unchangedCurrent store.SandboxCredential

	updated, evictedRefs, wroteNewRef, err := s.Credentials.UpdateSandboxGroupCredential(ctx, group.WorkspaceID, group.ID, credID,
		func(txCtx context.Context, salt []byte, cur store.SandboxCredential) (store.SandboxCredentialPreparedWrite, error) {
			merged := cur.Config
			if req.Domain != nil {
				merged.Domain = newDomain
			}
			if req.Enabled != nil {
				merged.Enabled = *req.Enabled
			}
			if req.Methods != nil {
				merged.Methods = newMethods
			}
			if req.Paths != nil {
				merged.Paths = newPaths
			}
			var injInput *GroupCredentialInjection
			if req.Injection != nil {
				// Render PATCH semantics: providing injection replaces the
				// entire object; template defaults to ${SECRET} when omitted.
				injInput = req.Injection
			} else {
				injInput = &GroupCredentialInjection{
					Type:     cur.Config.Injection.Type,
					Name:     cur.Config.Injection.Name,
					Template: cur.Config.Injection.Template,
				}
			}

			var secretVal string
			if req.Value != nil {
				secretVal = *req.Value
			} else {
				// No-op check before hitting OpenBao when nothing changed (still
				// validate injection below if the caller supplied a new one).
				if req.Injection == nil && credentialConfigsEqual(merged, cur.Config) {
					unchangedCurrent = cur
					return store.SandboxCredentialPreparedWrite{}, errCredentialNoOp
				}
				val, found, getErr := s.CredentialVault.Get(txCtx, group.WorkspaceID, group.ID, credID, cur.CurrentSecretRef)
				if getErr != nil {
					return store.SandboxCredentialPreparedWrite{}, getErr
				}
				if !found {
					return store.SandboxCredentialPreparedWrite{}, errCredentialsVaultFailed("get-missing", errors.New("stored credential secret missing"))
				}
				secretVal = val
			}

			validatedInj, valErr := ValidateCredentialInjection(injInput, secretVal)
			if valErr != nil {
				return store.SandboxCredentialPreparedWrite{}, valErr
			}
			merged.Injection = validatedInj

			if req.Value == nil && credentialConfigsEqual(merged, cur.Config) {
				unchangedCurrent = cur
				return store.SandboxCredentialPreparedWrite{}, errCredentialNoOp
			}

			fp := store.SandboxCredentialFingerprint(salt, merged, secretVal)
			secretRef := store.SandboxCredentialSecretRef(salt, secretVal)
			var beforeCommit store.SandboxCredentialCommitSecretFn
			if req.Value != nil {
				beforeCommit = func(cbCtx context.Context, ref string, alreadyRetained bool) error {
					if alreadyRetained {
						return nil
					}
					return s.CredentialVault.Put(cbCtx, group.WorkspaceID, group.ID, credID, ref, secretVal)
				}
			}
			return store.SandboxCredentialPreparedWrite{
				Config:         merged,
				UpdatedBy:      actor,
				Fingerprint:    fp,
				SecretRef:      secretRef,
				SecretBytes:    len(secretVal),
				OnBeforeCommit: beforeCommit,
			}, nil
		})
	if errors.Is(err, errCredentialNoOp) {
		return s.credentialView(ctx, unchangedCurrent)
	}
	if err != nil {
		if wroteNewRef != "" {
			if cleanErr := s.CredentialVault.Delete(context.WithoutCancel(ctx), group.WorkspaceID, group.ID, credID, wroteNewRef); cleanErr != nil {
				log.Printf("sandbox credentials: rollback cleanup failed for group=%s: %v", group.ID, cleanErr)
			}
		}
		if core.IsPublicError(err) {
			return GroupCredential{}, err
		}
		return GroupCredential{}, mapCredentialStoreErr(err, credID, 0)
	}
	s.purgeEvictedSecretRefs(ctx, group.WorkspaceID, group.ID, credID, evictedRefs)
	return s.credentialView(ctx, updated)
}

// DeleteGroupCredential deletes one credential and purges its OpenBao secrets
// idempotently. Requires Admin (`can_manage`). Repeated deletes of an absent
// credential in an existing group succeed without error.
func (s *Service) DeleteGroupCredential(ctx context.Context, groupID, credID string) error {
	ctx, group, err := s.credentialGroup(ctx, groupID, core.RelCanManage)
	if err != nil {
		return err
	}
	if err := ValidateCredentialID(credID); err != nil {
		return err
	}
	_, _, err = s.Credentials.DeleteSandboxGroupCredential(ctx, group.WorkspaceID, group.ID, credID,
		func(txCtx context.Context, refs []string) error {
			return s.CredentialVault.DeleteCredentialPrefix(txCtx, group.WorkspaceID, group.ID, credID, refs)
		})
	if err != nil {
		if core.IsPublicError(err) {
			return err
		}
		return mapCredentialStoreErr(err, credID, 0)
	}
	return nil
}

// ListGroupCredentialVersions returns retained versions of one credential,
// newest version ID first. Requires Viewer (`can_view`).
func (s *Service) ListGroupCredentialVersions(ctx context.Context, groupID, credID string) ([]GroupCredentialVersion, error) {
	ctx, group, err := s.credentialGroup(ctx, groupID, core.RelCanView)
	if err != nil {
		return nil, err
	}
	if err := ValidateCredentialID(credID); err != nil {
		return nil, err
	}
	rows, err := s.Credentials.ListSandboxGroupCredentialVersions(ctx, group.WorkspaceID, group.ID, credID)
	if err != nil {
		return nil, mapCredentialStoreErr(err, credID, 0)
	}
	return s.credentialVersionViews(ctx, rows)
}

// RestoreGroupCredential restores a retained credential version as a new
// current version with a higher monotonic ID and `restoredFromVersionId` set.
// Requires Admin (`can_manage`).
func (s *Service) RestoreGroupCredential(ctx context.Context, groupID, credID string, versionID int64) (GroupCredential, error) {
	ctx, group, err := s.credentialGroup(ctx, groupID, core.RelCanManage)
	if err != nil {
		return GroupCredential{}, err
	}
	if err := ValidateCredentialID(credID); err != nil {
		return GroupCredential{}, err
	}
	if versionID < 1 {
		return GroupCredential{}, credBadRequest("versionId", "versionId must be at least 1")
	}
	restored, evictedRefs, err := s.Credentials.RestoreSandboxGroupCredential(ctx, group.WorkspaceID, group.ID, credID, versionID, credentialActorSubject(ctx))
	if err != nil {
		return GroupCredential{}, mapCredentialStoreErr(err, credID, versionID)
	}
	s.purgeEvictedSecretRefs(ctx, group.WorkspaceID, group.ID, credID, evictedRefs)
	return s.credentialView(ctx, restored)
}

func (s *Service) purgeEvictedSecretRefs(ctx context.Context, workspaceID, groupID, credID string, evictedRefs []string) {
	if len(evictedRefs) == 0 || s.CredentialVault == nil {
		return
	}
	cleanCtx := context.WithoutCancel(ctx)
	for _, ref := range evictedRefs {
		if err := s.CredentialVault.Delete(cleanCtx, workspaceID, groupID, credID, ref); err != nil {
			log.Printf("sandbox credentials: post-commit evicted secret purge failed for group=%s: %v", groupID, err)
		}
	}
}
