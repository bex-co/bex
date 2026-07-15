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

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// CommitResolver resolves a repo ref (branch, tag, or SHA) to the exact
// commit it points at, via workspaceID's GitHub App connection —
// github.Service's DeployCommitSource satisfies it, the CloneTokenSource
// precedent (w9/001). nil on the Service ⇒ deploy rows this package opens
// (the create-path first deploy, the blueprint redeploy) carry no commit
// metadata.
type CommitResolver interface {
	// ResolveCommit returns the resolved commit, or ok=false (nil err) when
	// there is nothing to resolve (no connection, repo not in the grant,
	// unknown ref). Commit metadata is provenance, never load-bearing —
	// callers treat a non-nil err like ok=false instead of failing the deploy.
	ResolveCommit(ctx context.Context, workspaceID, repoURL, ref string) (store.CommitInfo, bool, error)
}

// DeployStartedNotifier is the request-time notification seam used by the
// HMAC-authenticated git-push redeploy path. The notifications service
// satisfies it structurally without apps importing that feature package.
type DeployStartedNotifier interface {
	NotifyDeployStarted(ctx context.Context, tenantID, appName, notificationsToSend string)
}

// resolveDeployCommit resolves repo@ref for a deploy row this package is
// about to open — best-effort by design (w9/001): a missing resolver, an
// image-backed app, or any resolution failure returns the zero CommitInfo
// and the deploy proceeds without commit metadata (omitted, not faked). ref
// defaults to "main", the operator's own Branch fallback (app_controller.go).
func (s *Service) resolveDeployCommit(ctx context.Context, workspaceID, repo, ref string) store.CommitInfo {
	if s.Commits == nil || repo == "" {
		return store.CommitInfo{}
	}
	if ref == "" {
		ref = "main"
	}
	commit, ok, err := s.Commits.ResolveCommit(ctx, workspaceID, repo, ref)
	if err != nil || !ok {
		return store.CommitInfo{}
	}
	return commit
}
