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

// workspaceSnapshot is one Blueprint request's list of each of its workspace's
// resource kinds (Apps, Databases and Key Values), shared by the reads of a kind
// before the request writes that kind: the action plan, the detachment check,
// the ownership check, reference resolution and the apply's display-name
// lookups (w5/124, w5/132). bex-api's client is uncached, so each list is an
// API-server round trip returning every resource of that kind in the
// workspace. A kind is listed on first use. A read after a write lists for
// itself, since a snapshot would miss the CRs the write created.
type workspaceSnapshot struct {
	s            *Service
	tenantID     string
	appList      *appv1alpha1.AppList
	databaseList *appv1alpha1.DatabaseList
	keyValueList *appv1alpha1.KeyValueList
}

func (s *Service) newWorkspaceSnapshot(ctx context.Context) *workspaceSnapshot {
	tenantID, _ := s.Tenant(ctx)
	return &workspaceSnapshot{s: s, tenantID: tenantID}
}

// apps lists the workspace's Apps the way its datastores are listed: by tenant
// label in every namespace, since they may straddle the shared namespace and
// the workspace's own (w5/125). Its readers hold pointers into the list, so
// none may modify or reorder it.
func (snap *workspaceSnapshot) apps(ctx context.Context) (*appv1alpha1.AppList, error) {
	if snap.appList == nil {
		list := &appv1alpha1.AppList{}
		if err := snap.s.Client.List(ctx, list, snap.s.DatastoreListOptions(snap.tenantID)...); err != nil {
			return nil, err
		}
		snap.appList = list
	}
	return snap.appList, nil
}

func (snap *workspaceSnapshot) databases(ctx context.Context) (*appv1alpha1.DatabaseList, error) {
	if snap.databaseList == nil {
		list, err := snap.s.listWorkspaceDatabases(ctx, snap.tenantID)
		if err != nil {
			return nil, err
		}
		snap.databaseList = list
	}
	return snap.databaseList, nil
}

func (snap *workspaceSnapshot) keyValues(ctx context.Context) (*appv1alpha1.KeyValueList, error) {
	if snap.keyValueList == nil {
		list, err := snap.s.listWorkspaceKeyValues(ctx, snap.tenantID)
		if err != nil {
			return nil, err
		}
		snap.keyValueList = list
	}
	return snap.keyValueList, nil
}
