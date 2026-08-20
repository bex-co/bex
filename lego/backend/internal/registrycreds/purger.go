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

package registrycreds

import (
	"context"
	"fmt"
)

// WorkspacePurger removes registry-credential values from OpenBao before the
// workspace's Postgres row is deleted. The database cascade owns metadata-row
// removal; this purger deliberately touches only the out-of-cascade secrets.
type WorkspacePurger struct {
	*Service
}

// PurgeWorkspace is an internal system operation and therefore performs no
// caller authorization. It is idempotent because Secret.Delete treats an absent
// path as success. A failure surfaces so workspace deletion retains its tenant
// row and can safely be retried.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	if p == nil || p.Service == nil || p.Store == nil || p.Secret == nil {
		return nil
	}
	credentials, err := p.Store.ListRegistryCredentials(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, credential := range credentials {
		if err := p.Secret.Delete(ctx, secretPath(tenantID, credential.ID)); err != nil {
			return fmt.Errorf("delete registry credential %q secret: %w", credential.ID, err)
		}
	}
	return nil
}
