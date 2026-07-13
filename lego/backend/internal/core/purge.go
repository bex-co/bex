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

package core

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// purge.go is the one implementation of "find a tenant's CRs, tear them down"
// that every workspaces.WorkspacePurger is built from (w6/m15/t002): the apps,
// postgres, keyvalue, and secrets purgers each used to carry a verbatim copy of
// the same List-by-LabelTenant loop.

// ListByTenant fills list with the objects of its kind in the Base's namespace
// that carry tenantID in LabelTenant — the label CreatePostgres/CreateKeyValue/
// the App-CR projector all stamp, and the only attribution a deleted workspace's
// leftovers still have.
func (b *Base) ListByTenant(ctx context.Context, list client.ObjectList, tenantID string) error {
	return b.Client.List(ctx, list,
		client.InNamespace(b.Namespace), client.MatchingLabels{LabelTenant: tenantID})
}

// PurgeByTenant deletes every object of list's kind labeled with tenantID (see
// ListByTenant). Deliberately a List + per-object Delete rather than a single
// client.DeleteAllOf: DeleteAllOf needs the `deletecollection` RBAC verb, which
// bex-api's role does not (and need not) hold. list is only scratch space for
// the fetch — callers pass an empty one.
//
// Idempotent, as WorkspacePurger requires: nothing matching is a no-op, and an
// object another teardown path (the row cascade, the projector prune) already
// removed is ignored (NotFound), so a retried workspace delete completes.
func (b *Base) PurgeByTenant(ctx context.Context, list client.ObjectList, tenantID string) error {
	if err := b.ListByTenant(ctx, list, tenantID); err != nil {
		return err
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}
	for _, item := range items {
		obj, ok := item.(client.Object)
		if !ok {
			return fmt.Errorf("purge tenant %s: %T is not a client.Object", tenantID, item)
		}
		if err := b.Client.Delete(ctx, obj); err != nil && client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}
