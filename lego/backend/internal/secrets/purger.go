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

package secrets

import (
	"context"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// WorkspacePurger adapts Service to the workspaces feature's WorkspacePurger
// seam (satisfied structurally — this package never imports workspaces, same
// discipline as the rest of the composition-root wiring) for the
// out-of-cascade teardown workspaces.Service.Delete runs after a workspace's
// tenant row is gone (w6/m4/t003): every App the tenant owned has its own
// per-service OpenBao env vars + secret files, which the row's FK cascade and
// the App-CR projector prune do NOT reach (that data lives in OpenBao, not
// Postgres or etcd), so it would otherwise linger forever.
//
// Env groups are deliberately not purged here: unlike per-service secrets,
// env groups carry no tenant attribution at all today (any workspace's App
// can link any group — a pre-existing gap this task didn't introduce and
// isn't scoped to fix), so there is no safe way to attribute one to the
// deleted workspace without risking deleting another tenant's still-linked
// group.
type WorkspacePurger struct {
	*Service
}

// PurgeWorkspace deletes the OpenBao env-var and secret-file paths for every
// App the given tenant owned, found via core.LabelTenant — the same label
// CreatePostgres/CreateKeyValue/the App-CR projector stamp. Best-effort and
// idempotent: an absent Store is a no-op (secrets disabled, nothing to purge),
// and deleting an already-absent path is a no-op (core.SecretKV.Delete's
// contract), so a retried workspace delete completes the teardown cleanly.
// Never call s.Authorize/AuthorizeOn here — this runs as an internal system
// operation after the deleting caller's own authorization already passed,
// with no Identity in ctx to check against.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	if p.Store == nil {
		return nil
	}
	var apps appv1alpha1.AppList
	if err := p.ListByTenant(ctx, &apps, tenantID); err != nil {
		return err
	}
	for i := range apps.Items {
		if err := p.purgeAppSecrets(ctx, &apps.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

// PurgeApp deletes the OpenBao env-var and secret-file paths for a single App —
// the per-app complement to PurgeWorkspace, called on individual service deletion
// so secrets don't linger after the App CR is gone. It resolves the public service
// name (core.LabelServiceName) and workspace (core.LabelTenant) from the App
// itself, NOT metadata.name — a store-managed App's metadata.name is the
// tenant-prefixed CRName, while the live values are keyed by the public name
// (w7/m70 F15: purging a.Name left the real paths behind). Idempotent: nil Store
// or absent paths are no-ops.
func (p *WorkspacePurger) PurgeApp(ctx context.Context, a *appv1alpha1.App) error {
	if p.Store == nil || a == nil {
		return nil
	}
	return p.purgeAppSecrets(ctx, a)
}

// purgeAppSecrets deletes one App's env-var and secret-file maps from both the
// tenant-scoped path (w7/m70) and the legacy single-tenant (baoTenant) path, so a
// delete cleans up regardless of whether the lazy migrator (readMap) has run yet.
// The public name resolves through storeServiceName; the tenant through
// storeTenant — matching the write path exactly.
func (p *WorkspacePurger) purgeAppSecrets(ctx context.Context, a *appv1alpha1.App) error {
	name := storeServiceName(a, a.Name)
	tenantCtx := withTenant(ctx, storeTenant(a))
	legacyCtx := withTenant(ctx, baoTenant)
	for _, path := range []string{envPath(name), filesPath(name)} {
		if err := p.Store.Delete(tenantCtx, path); err != nil {
			return err
		}
		if err := p.Store.Delete(legacyCtx, path); err != nil {
			return err
		}
	}
	return nil
}
