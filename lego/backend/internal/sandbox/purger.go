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

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// PurgeKeyLookup resolves a workspace's existing OpenSandbox tenant key WITHOUT
// minting one — *store.PGStore's SandboxKeyLookup satisfies it structurally. The
// purger needs a lookup-only seam, never the mint-on-miss KeyProvider: minting a
// key for a workspace being torn down would write during teardown and, since the
// tenant-row cascade (migration 0056) is about to drop it, be pointless.
// found=false means no key was ever minted ⇒ no sandbox was ever created (the
// first create is what mints the key), so there is nothing to purge.
type PurgeKeyLookup interface {
	SandboxKeyLookup(ctx context.Context, workspaceID string) (string, bool, error)
}

// WorkspacePurger adapts the OpenSandbox client to the workspaces feature's
// WorkspacePurger seam (satisfied structurally — this package never imports
// workspaces), closing a w1/m61 gap: a deleted workspace's running sandboxes
// were never stopped through the API. It runs PRE-cascade in workspaces.Delete
// because it needs the workspace's tenant key, which the tenant-row cascade
// drops. The `<ws>-sandbox` namespace prune eventually reaps the pods at the
// Kubernetes layer too, but that path is best-effort; terminating through
// OpenSandbox is the always-on clean shutdown and also clears the OpenSandbox
// control plane's own records.
type WorkspacePurger struct {
	Client *Client
	Keys   PurgeKeyLookup
	// Images, when it has a reaper, deletes the workspace's imported images
	// from the registry (w2/m359) while their rows still name them.
	Images *Images
	// AgentBuilds lists the workspace's repository environment build images,
	// deleted with its imported ones (ADR111 D8); *store.PGStore.
	AgentBuilds AgentBuildImages
	// AgentRetained lists the workspace's stopped agent turns' retained
	// images, deleted with them (ADR047 D12); *store.PGStore.
	AgentRetained AgentRetainedImages
	// Reaper and Storage delete the images the workspace's /v1/sandboxes
	// sandboxes retained when they were suspended (w22/017): Storage lists their
	// registry refs while the `sandbox_storage_objects` rows that name them
	// still exist, and Reaper deletes their `snap-gen<N>` tags, the meter's
	// reaper. Either nil leaves them, as a deployment without the reaper
	// credential always has; the orphan sweep (OrphanSweeper) still finds them
	// once the workspace is gone. *store.PGStore is the Storage.
	Reaper  ImageReaper
	Storage RetainedSandboxImages
	// Snapshots, when it has a reaper, deletes the images of the workspace's
	// sandbox snapshots (w32/m285) while their rows still name them.
	Snapshots *Snapshots
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// RetainedSandboxImages lists the registry refs of the rootfs images a
// workspace's sandboxes retained, whether open, closed or queued for the
// reaper.
type RetainedSandboxImages interface {
	SandboxRootfsRefs(ctx context.Context, workspaceID string) ([]string, error)
}

// AgentBuildImages lists the repository environment builds whose images the
// registry may hold.
type AgentBuildImages interface {
	AgentRepoEnvironmentBuildImageIDs(ctx context.Context, workspaceID string) ([]string, error)
}

// AgentRetainedImages lists the retained images of stopped agent turns whose
// handles exist, which the registry may still hold.
type AgentRetainedImages interface {
	AgentRetainedImageIDs(ctx context.Context, workspaceID string) ([]string, error)
}

// PurgeWorkspace stops every sandbox in the deleted workspace's `<ws>-sandbox`
// namespace. The tenant key scopes the OpenSandbox list to exactly that
// namespace, so this is a workspace-wide teardown — no owner/admin filtering,
// because it is an internal system operation after the deleting caller was
// already authorized, not a user-facing list. Idempotent: no key ⇒ nothing was
// ever created ⇒ no-op; Client.Terminate already tolerates a missing sandbox
// (NotFound), so a retried delete (or one racing the namespace prune) completes
// cleanly. Never authorizes here — there is no Identity in ctx to check against.
//
// The suspended sandboxes' images go last, after the sandboxes are ended, so a
// pause cannot commit a new image behind the delete; whatever still lands after
// it is the orphan sweep's.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	if err := p.purgeImages(ctx, tenantID); err != nil {
		return err
	}
	if err := p.terminateSandboxes(ctx, tenantID); err != nil {
		return err
	}
	if err := p.purgeSandboxImages(ctx, tenantID); err != nil {
		return err
	}
	return p.purgeSnapshots(ctx, tenantID)
}

// purgeSnapshots deletes the images of the workspace's sandbox snapshots.
// Like the suspended sandboxes' images it runs after the sandboxes are ended,
// so no capture can settle a new image behind the delete.
func (p *WorkspacePurger) purgeSnapshots(ctx context.Context, tenantID string) error {
	if p.Snapshots == nil || p.Snapshots.Reaper == nil {
		return nil
	}
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	return p.Snapshots.purgeWorkspace(ctx, tenantID, now())
}

func (p *WorkspacePurger) terminateSandboxes(ctx context.Context, tenantID string) error {
	if p.Client == nil || p.Keys == nil {
		return nil
	}
	key, found, err := p.Keys.SandboxKeyLookup(ctx, tenantID)
	if err != nil {
		return err
	}
	if !found || key == "" {
		return nil
	}
	list, err := p.Client.List(ctx, key)
	if err != nil {
		return err
	}
	for _, sb := range list {
		if err := p.Client.Terminate(ctx, key, sb.ID); err != nil {
			return err
		}
	}
	return nil
}

// purgeImages deletes every image the workspace imported or built. It runs
// before the tenant's rows cascade, which would otherwise leave the registry
// holding images nothing names.
func (p *WorkspacePurger) purgeImages(ctx context.Context, tenantID string) error {
	if p.Images == nil || p.Images.Reaper == nil {
		return nil
	}
	builds, err := p.Images.Store.SandboxBuildIDs(ctx, tenantID, "")
	if err != nil {
		return err
	}
	if p.AgentBuilds != nil {
		agentBuilds, err := p.AgentBuilds.AgentRepoEnvironmentBuildImageIDs(ctx, tenantID)
		if err != nil {
			return err
		}
		builds = append(builds, agentBuilds...)
	}
	if p.AgentRetained != nil {
		retained, err := p.AgentRetained.AgentRetainedImageIDs(ctx, tenantID)
		if err != nil {
			return err
		}
		builds = append(builds, retained...)
	}
	return p.Images.deleteImported(ctx, tenantID, builds)
}

// purgeSandboxImages deletes the `snap-gen<N>` tags of every rootfs image the
// workspace's sandboxes retained, with the meter's reaper. A suspended sandbox's
// image is named only by its `sandbox_storage_objects` row, which cascades away
// with the tenant row, and no reap_pending row would ever ask for it after that.
// It tries every repository and returns every failure, so the delete is retried
// whole and one refused repository hides no other.
func (p *WorkspacePurger) purgeSandboxImages(ctx context.Context, tenantID string) error {
	if p.Reaper == nil || p.Storage == nil {
		return nil
	}
	refs, err := p.Storage.SandboxRootfsRefs(ctx, tenantID)
	if err != nil {
		return err
	}
	var errs []error
	for _, ref := range refs {
		if !registryRef.MatchString(ref) {
			continue // no repository can carry this name, so there is no image to delete
		}
		if err := p.Reaper.DeleteImages(ctx, tenantID, ref); err != nil {
			errs = append(errs, fmt.Errorf("delete images of sandbox %s: %w", ref, err))
		}
	}
	return errors.Join(errs...)
}
