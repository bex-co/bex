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
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"

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
// itself, since a snapshot would miss the CRs the write created. The one reader
// that follows writes is a Blueprint create's resources view: the ownership
// stamp, the apply's last write, re-reads the workspace through the apply's
// snapshot (forget), and the view reads what it listed (w5/141).
//
// A resource whose deletion Kubernetes has accepted (a DeletionTimestamp, kept
// alive only by its finalizer's teardown) is not listed: every by-id read
// already answers 404 for it (core.NotFoundIfDeleting, w3/m81), and on Render
// a deleted resource the Blueprint defines is missing, so the next plan
// recreates it (render.com/docs/infrastructure-as-code). Counting it made a
// re-created Blueprint plan no_changes against a service the customer had
// just deleted (w2/m256 t014).
type workspaceSnapshot struct {
	s        *Service
	tenantID string
	// rawApps is the workspace's one App listing, carriers included;
	// appList is its services.
	rawApps      *appv1alpha1.AppList
	appList      *appv1alpha1.AppList
	serviceMap   map[string]*appv1alpha1.App
	databaseList *appv1alpha1.DatabaseList
	keyValueList *appv1alpha1.KeyValueList
	// buildSourceMap is the workspace's build source carriers by name.
	buildSourceMap map[string]*appv1alpha1.App
	// deletingServices are the manifest-facing names of the workspace's
	// services still being torn down: absent from every map above, but their
	// App object still holds the name a re-create would write.
	deletingServices map[string]bool
}

func (s *Service) newWorkspaceSnapshot(ctx context.Context) *workspaceSnapshot {
	tenantID, _ := s.Tenant(ctx)
	return &workspaceSnapshot{s: s, tenantID: tenantID}
}

// forget drops what the snapshot has listed, so its next reads see the
// workspace as it is now.
func (snap *workspaceSnapshot) forget() {
	snap.rawApps, snap.appList, snap.serviceMap, snap.databaseList, snap.keyValueList, snap.buildSourceMap = nil, nil, nil, nil, nil, nil
	snap.deletingServices = nil
}

// apps lists the workspace's Apps the way its datastores are listed: by tenant
// label in every namespace, since they may straddle the shared namespace and
// the workspace's own (w5/125). Its readers hold pointers into the list, so
// none may reorder it, and only the ownership stamp's label patches write to
// its objects.
func (snap *workspaceSnapshot) apps(ctx context.Context) (*appv1alpha1.AppList, error) {
	if snap.appList == nil {
		raw, err := snap.allApps(ctx)
		if err != nil {
			return nil, err
		}
		list := &appv1alpha1.AppList{}
		for i := range raw.Items {
			if !core.IsBuildSource(&raw.Items[i]) {
				list.Items = append(list.Items, raw.Items[i])
			}
		}
		snap.appList = list
	}
	return snap.appList, nil
}

// allApps is the workspace's Apps, build source carriers included, listed
// once for services and build sources alike.
func (snap *workspaceSnapshot) allApps(ctx context.Context) (*appv1alpha1.AppList, error) {
	if snap.rawApps == nil {
		listed := &appv1alpha1.AppList{}
		if err := snap.s.Client.List(ctx, listed, snap.s.DatastoreListOptions(snap.tenantID)...); err != nil {
			return nil, err
		}
		list := &appv1alpha1.AppList{}
		deleting := map[string]bool{}
		for i := range listed.Items {
			app := &listed.Items[i]
			if app.DeletionTimestamp.IsZero() {
				list.Items = append(list.Items, *app)
				continue
			}
			if app.Labels[core.LabelTenant] == snap.tenantID && !core.IsBuildSource(app) {
				deleting[core.AppPublicName(app)] = true
			}
		}
		snap.rawApps, snap.deletingServices = list, deleting
	}
	return snap.rawApps, nil
}

// serviceDeleting reports whether a service the workspace no longer has (it
// is not in services) is still being torn down under name, so a create of
// that name would collide with its App object until the teardown finishes.
func (snap *workspaceSnapshot) serviceDeleting(ctx context.Context, name string) (bool, error) {
	if _, err := snap.allApps(ctx); err != nil {
		return false, err
	}
	return snap.deletingServices[name], nil
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
	aliasLegacyResourceNames(byName)
	snap.serviceMap = byName
	return byName, nil
}

func (snap *workspaceSnapshot) databases(ctx context.Context) (*appv1alpha1.DatabaseList, error) {
	if snap.databaseList == nil {
		list, err := snap.s.listWorkspaceDatabases(ctx, snap.tenantID)
		if err != nil {
			return nil, err
		}
		list.Items = slices.DeleteFunc(list.Items, func(d appv1alpha1.Database) bool { return !d.DeletionTimestamp.IsZero() })
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
		list.Items = slices.DeleteFunc(list.Items, func(kv appv1alpha1.KeyValue) bool { return !kv.DeletionTimestamp.IsZero() })
		snap.keyValueList = list
	}
	return snap.keyValueList, nil
}

// aliasLegacyResourceNames lets a Blueprint keep finding a service created
// before w4/385 changed the name mapping: it is labelled with the old name
// ("resource" for a name with no Latin letters, accents dropped otherwise),
// while its display name now maps to a different one. Without the alias the
// plan would create a second service and orphan the first. An unlabelled
// service is its own typed name: one kept unchanged before w8/230 ("my--app")
// now maps elsewhere ("my-app"). A name the new mapping already owns is never
// displaced.
func aliasLegacyResourceNames(byName map[string]*appv1alpha1.App) {
	for name, app := range maps.Clone(byName) {
		label := cmp.Or(app.Spec.DisplayName, name)
		if legacyBlueprintResourceSlug(label) != name {
			continue
		}
		if current := blueprintResourceSlug(label); current != name && byName[current] == nil {
			byName[current] = app
		}
	}
}
