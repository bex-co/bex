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

package keyvalue

import (
	"context"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// WorkspacePurger adapts Service to the workspaces feature's WorkspacePurger
// seam (satisfied structurally — this package never imports workspaces) for
// the out-of-cascade teardown workspaces.Service.Delete runs after a
// workspace's tenant row is gone (w6/m4/t004): a tenant's managed KeyValue CRs
// live outside the control-plane Postgres row's FK cascade entirely, so
// they'd otherwise survive a workspace delete indefinitely.
type WorkspacePurger struct {
	*Service
}

// PurgeWorkspace deletes every KeyValue CR labeled with the given tenant id
// (core.LabelTenant, stamped by CreateKeyValue — w6/m4/t002). Each delete
// cascades its StatefulSet/PVC/Secret via owner refs, same as DeleteKeyValue.
// Idempotent: no matching KeyValue is a no-op, not an error. Never call
// s.Authorize/AuthorizeOn here — this runs as an internal system operation
// after the deleting caller's own authorization already passed, with no
// Identity in ctx to check against.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	return p.PurgeByTenant(ctx, &appv1alpha1.KeyValueList{}, tenantID)
}
