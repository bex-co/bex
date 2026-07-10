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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
	if err := p.Client.List(ctx, &apps,
		client.InNamespace(p.Namespace), client.MatchingLabels{core.LabelTenant: tenantID}); err != nil {
		return err
	}
	for i := range apps.Items {
		name := apps.Items[i].Name
		if err := p.Store.Delete(ctx, envPath(name)); err != nil {
			return err
		}
		if err := p.Store.Delete(ctx, filesPath(name)); err != nil {
			return err
		}
	}
	return nil
}
