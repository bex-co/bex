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

package api

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// loginImpactStore is the store half of the enable confirmation's credential
// read (w7/m471): which keys are bound to which workspaces, and people's SSH
// keys. *store.PGStore satisfies it.
type loginImpactStore interface {
	ClientBindingsOfTenants(ctx context.Context, tenantIDs []string) (map[string]string, error)
	ListSSHKeysOfSubjects(ctx context.Context, subjects []string) ([]store.SSHKey, error)
}

// loginImpactKeys adapts the Hydra client list (each key's name and stamps)
// and the store's bindings onto workspaces.LoginImpactKeys.
type loginImpactKeys struct {
	keys  apikeys.APIKeyStore
	store loginImpactStore
}

// newLoginImpactKeys is nil when either source is missing, which answers the
// read as "could not check".
func newLoginImpactKeys(keys apikeys.APIKeyStore, ws workspaces.WorkspaceStore) workspaces.LoginImpactKeys {
	st, ok := ws.(loginImpactStore)
	if keys == nil || !ok {
		return nil
	}
	return loginImpactKeys{keys: keys, store: st}
}

// BoundAPIKeys lists the keys bound to workspaceIDs: the bindings first, so a
// workspace with no key costs no Hydra read.
func (l loginImpactKeys) BoundAPIKeys(ctx context.Context, workspaceIDs []string) ([]workspaces.BoundAPIKey, error) {
	bound, err := l.store.ClientBindingsOfTenants(ctx, workspaceIDs)
	if err != nil || len(bound) == 0 {
		return nil, err
	}
	keys, err := l.keys.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]workspaces.BoundAPIKey, 0, len(bound))
	for _, k := range keys {
		if ws, ok := bound[k.ID]; ok {
			out = append(out, workspaces.BoundAPIKey{ID: k.ID, Name: k.Name, WorkspaceID: ws, CreatedBy: k.CreatedBy, CreatedSignIn: k.CreatedSignIn})
		}
	}
	return out, nil
}

func (l loginImpactKeys) SSHKeysOf(ctx context.Context, subjects []string) ([]store.SSHKey, error) {
	if len(subjects) == 0 {
		return nil, nil
	}
	return l.store.ListSSHKeysOfSubjects(ctx, subjects)
}
