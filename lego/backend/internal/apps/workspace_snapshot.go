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
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
	serviceMap   map[string]*appv1alpha1.App
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

// services keys the workspace's Apps by the name a manifest declares them
// under: the one resolution a Blueprint's plan and its apply share, so the
// apply updates the object the plan names (w5/m133).
//
//   - The key is the manifest-facing name (core.AppPublicName), never the
//     object name or a displayed name: store-managed Apps carry a tenant
//     prefix (CRName), and keying by object name reported live services as
//     fresh creates in the plan an approver reviews (round-21 finding 7).
//   - Only the workspace's own Apps count, whatever else the caller can reach;
//     a request with no workspace counts only Apps no workspace owns.
//   - Of a name's copies, the one in its workspace's own namespace answers; a
//     service still only in the shared namespace (mid ADR043 D8) answers from
//     there. Two copies in the answering namespace are refused: list order
//     would otherwise pick which one the plan diffs against (w6/m125).
func (snap *workspaceSnapshot) services(ctx context.Context) (map[string]*appv1alpha1.App, error) {
	if snap.serviceMap != nil {
		return snap.serviceMap, nil
	}
	apps, err := snap.apps(ctx)
	if err != nil {
		return nil, err
	}
	var names []string
	copies := map[string][]*appv1alpha1.App{}
	for i := range apps.Items {
		app := &apps.Items[i]
		if app.Labels[core.LabelTenant] != snap.tenantID {
			continue
		}
		name := core.AppPublicName(app)
		if copies[name] == nil {
			names = append(names, name)
		}
		copies[name] = append(copies[name], app)
	}
	byName := make(map[string]*appv1alpha1.App, len(names))
	for _, name := range names {
		answering := copies[name][0].Namespace
		for _, app := range copies[name] {
			if core.AppInOwnWorkspaceNamespace(app) {
				answering = app.Namespace
				break
			}
		}
		for _, app := range copies[name] {
			if app.Namespace != answering {
				continue
			}
			if byName[name] != nil {
				return nil, fmt.Errorf("%w: service name %q is already used more than once in this workspace", core.ErrConflict, name)
			}
			byName[name] = app
		}
	}
	snap.serviceMap = byName
	return byName, nil
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
