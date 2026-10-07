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

package apps

import (
	"context"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// datastoreSnapshot is one Blueprint request's list of each of its workspace's
// datastore kinds, shared by every read before the request writes a Database
// or Key Value: the action plan, the ownership check, reference resolution and
// the apply's display-name lookups (w5/124). bex-api's client is uncached, so
// each list is an API-server round trip returning every datastore in the
// workspace. A kind is listed on first use. A read after a datastore write
// lists for itself, since a snapshot would miss the CRs the write created.
type datastoreSnapshot struct {
	s            *Service
	tenantID     string
	databaseList *appv1alpha1.DatabaseList
	keyValueList *appv1alpha1.KeyValueList
}

func (s *Service) newDatastoreSnapshot(ctx context.Context) *datastoreSnapshot {
	tenantID, _ := s.Tenant(ctx)
	return &datastoreSnapshot{s: s, tenantID: tenantID}
}

func (snap *datastoreSnapshot) databases(ctx context.Context) (*appv1alpha1.DatabaseList, error) {
	if snap.databaseList == nil {
		list, err := snap.s.listWorkspaceDatabases(ctx, snap.tenantID)
		if err != nil {
			return nil, err
		}
		snap.databaseList = list
	}
	return snap.databaseList, nil
}

func (snap *datastoreSnapshot) keyValues(ctx context.Context) (*appv1alpha1.KeyValueList, error) {
	if snap.keyValueList == nil {
		list, err := snap.s.listWorkspaceKeyValues(ctx, snap.tenantID)
		if err != nil {
			return nil, err
		}
		snap.keyValueList = list
	}
	return snap.keyValueList, nil
}
