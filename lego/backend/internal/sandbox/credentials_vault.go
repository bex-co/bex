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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// CodeSandboxCredentialsUnavailable is returned (HTTP 503) when the control-plane
// store or OpenBao secret store is unconfigured, sealed, or temporarily unreachable.
const CodeSandboxCredentialsUnavailable = "SANDBOX_CREDENTIALS_UNAVAILABLE"

const sandboxCredentialSecretField = "value"

// CredentialSecretRefStore lists OpenBao secret references owned by a workspace
// or group so teardown can purge them before rows cascade away. *store.PGStore
// implements it.
type CredentialSecretRefStore interface {
	ListSandboxGroupCredentialSecretRefs(ctx context.Context, workspaceID, groupID string) ([]store.SandboxCredentialSecretRefEntry, error)
}

// CredentialVault stores sandbox group credential secret values write-only in
// OpenBao under workspace- and group-scoped KV v2 paths (ADR122 D3):
//
//	tenants/data/<workspaceID>/sandbox-groups/<sandboxGroupID>/credentials/<sha256(credentialID)>/<secretRef>
//
// Raw secret values are never stored in Postgres, never returned to API callers,
// and never written to errors or logs.
type CredentialVault struct {
	KV core.SecretKV
}

// NewCredentialVault returns a CredentialVault backed by kv, or nil if kv is nil.
func NewCredentialVault(kv core.SecretKV) *CredentialVault {
	if kv == nil {
		return nil
	}
	return &CredentialVault{KV: kv}
}

func (v *CredentialVault) enabled() bool { return v != nil && v.KV != nil }

func errCredentialsNotConfigured() error {
	return core.NewUnavailableError(CodeSandboxCredentialsUnavailable,
		"sandbox group credentials require the control-plane and secret stores", nil)
}

func errCredentialsVaultFailed(op string, err error) error {
	return core.HideCause(
		core.NewUnavailableError(CodeSandboxCredentialsUnavailable,
			"sandbox credential secret store is temporarily unavailable; try again", nil),
		fmt.Errorf("sandbox credential vault %s: %w", op, err),
	)
}

func credentialIDHash(credID string) string {
	sum := sha256.Sum256([]byte(credID))
	return hex.EncodeToString(sum[:])
}

func credentialGroupPrefix(groupID string) string {
	return "sandbox-groups/" + groupID + "/credentials"
}

func credentialPrefix(groupID, credID string) string {
	return credentialGroupPrefix(groupID) + "/" + credentialIDHash(credID)
}

func credentialSecretPath(groupID, credID, secretRef string) string {
	return credentialPrefix(groupID, credID) + "/" + secretRef
}

// Put writes one credential secret value to OpenBao under the workspace's
// tenant prefix. Fails closed with a sanitized 503 if OpenBao is unconfigured
// or returns an error.
func (v *CredentialVault) Put(ctx context.Context, workspaceID, groupID, credID, secretRef, value string) error {
	if !v.enabled() {
		return errCredentialsNotConfigured()
	}
	if workspaceID == "" || groupID == "" || credID == "" || secretRef == "" {
		return errCredentialsVaultFailed("put", errors.New("missing path component"))
	}
	tctx := secrets.WithTenant(ctx, workspaceID)
	if err := v.KV.Put(tctx, credentialSecretPath(groupID, credID, secretRef), map[string]string{
		sandboxCredentialSecretField: value,
	}); err != nil {
		return errCredentialsVaultFailed("put", err)
	}
	return nil
}

// Get reads one credential secret value for internal control-plane or broker
// use only (never surfaced to external callers). Returns ("", false, nil) when
// absent, or a sanitized 503 on OpenBao errors.
func (v *CredentialVault) Get(ctx context.Context, workspaceID, groupID, credID, secretRef string) (string, bool, error) {
	if !v.enabled() {
		return "", false, errCredentialsNotConfigured()
	}
	if workspaceID == "" || groupID == "" || credID == "" || secretRef == "" {
		return "", false, errCredentialsVaultFailed("get", errors.New("missing path component"))
	}
	tctx := secrets.WithTenant(ctx, workspaceID)
	data, err := v.KV.Get(tctx, credentialSecretPath(groupID, credID, secretRef))
	if err != nil {
		return "", false, errCredentialsVaultFailed("get", err)
	}
	val, ok := data[sandboxCredentialSecretField]
	return val, ok, nil
}

// Delete removes one secret reference idempotently (an already-absent path is a
// no-op).
func (v *CredentialVault) Delete(ctx context.Context, workspaceID, groupID, credID, secretRef string) error {
	if !v.enabled() {
		return nil
	}
	if workspaceID == "" || groupID == "" || credID == "" || secretRef == "" {
		return nil
	}
	tctx := secrets.WithTenant(ctx, workspaceID)
	if err := v.KV.Delete(tctx, credentialSecretPath(groupID, credID, secretRef)); err != nil {
		return errCredentialsVaultFailed("delete", err)
	}
	return nil
}

// DeleteCredentialPrefix purges all OpenBao entries for one credential
// idempotently, combining Postgres-known refs with a KV metadata list sweep so
// no orphaned version secret survives.
func (v *CredentialVault) DeleteCredentialPrefix(ctx context.Context, workspaceID, groupID, credID string, knownRefs []string) error {
	if !v.enabled() || workspaceID == "" || groupID == "" || credID == "" {
		return nil
	}
	tctx := secrets.WithTenant(ctx, workspaceID)
	prefix := credentialPrefix(groupID, credID)
	seen := make(map[string]struct{}, len(knownRefs))
	var errs []error
	for _, ref := range knownRefs {
		if ref == "" {
			continue
		}
		seen[ref] = struct{}{}
		if err := v.KV.Delete(tctx, prefix+"/"+ref); err != nil {
			errs = append(errs, err)
		}
	}
	keys, err := v.KV.List(tctx, prefix)
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, k := range keys {
			k = strings.TrimSuffix(k, "/")
			if k == "" {
				continue
			}
			if _, done := seen[k]; done {
				continue
			}
			if err := v.KV.Delete(tctx, prefix+"/"+k); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errCredentialsVaultFailed("delete-credential-prefix", errors.Join(errs...))
	}
	return nil
}

// DeleteGroupPrefix purges every OpenBao credential secret belonging to a
// sandbox group, combining Postgres-known refs with a KV prefix walk.
func (v *CredentialVault) DeleteGroupPrefix(ctx context.Context, workspaceID, groupID string, knownRefs []store.SandboxCredentialSecretRefEntry) error {
	if !v.enabled() || workspaceID == "" || groupID == "" {
		return nil
	}
	tctx := secrets.WithTenant(ctx, workspaceID)
	seen := make(map[string]struct{}, len(knownRefs))
	var errs []error
	for _, entry := range knownRefs {
		if entry.SandboxGroupID != groupID || entry.CredentialID == "" || entry.SecretRef == "" {
			continue
		}
		p := credentialSecretPath(groupID, entry.CredentialID, entry.SecretRef)
		seen[p] = struct{}{}
		if err := v.KV.Delete(tctx, p); err != nil {
			errs = append(errs, err)
		}
	}
	groupRoot := credentialGroupPrefix(groupID)
	credHashes, err := v.KV.List(tctx, groupRoot)
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, h := range credHashes {
			h = strings.TrimSuffix(h, "/")
			if h == "" {
				continue
			}
			credRoot := groupRoot + "/" + h
			refs, listErr := v.KV.List(tctx, credRoot)
			if listErr != nil {
				errs = append(errs, listErr)
				continue
			}
			for _, ref := range refs {
				ref = strings.TrimSuffix(ref, "/")
				if ref == "" {
					continue
				}
				p := credRoot + "/" + ref
				if _, done := seen[p]; done {
					continue
				}
				if err := v.KV.Delete(tctx, p); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	if len(errs) > 0 {
		return errCredentialsVaultFailed("delete-group-prefix", errors.Join(errs...))
	}
	return nil
}

// DeleteWorkspacePrefix purges every OpenBao sandbox-group credential secret
// belonging to a workspace (pre-cascade during workspace deletion).
func (v *CredentialVault) DeleteWorkspacePrefix(ctx context.Context, workspaceID string, knownRefs []store.SandboxCredentialSecretRefEntry) error {
	if !v.enabled() || workspaceID == "" {
		return nil
	}
	tctx := secrets.WithTenant(ctx, workspaceID)
	byGroup := make(map[string][]store.SandboxCredentialSecretRefEntry)
	for _, entry := range knownRefs {
		if entry.SandboxGroupID != "" {
			byGroup[entry.SandboxGroupID] = append(byGroup[entry.SandboxGroupID], entry)
		}
	}
	groupIDs := make(map[string]struct{}, len(byGroup))
	for gid := range byGroup {
		groupIDs[gid] = struct{}{}
	}
	var errs []error
	listedGroups, err := v.KV.List(tctx, "sandbox-groups")
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, gid := range listedGroups {
			gid = strings.TrimSuffix(gid, "/")
			if gid != "" {
				groupIDs[gid] = struct{}{}
			}
		}
	}
	for gid := range groupIDs {
		if err := v.DeleteGroupPrefix(ctx, workspaceID, gid, byGroup[gid]); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errCredentialsVaultFailed("delete-workspace-prefix", errors.Join(errs...))
	}
	return nil
}
