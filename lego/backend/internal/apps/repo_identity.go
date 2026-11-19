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
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// repo_identity.go follows a GitHub repository by its numeric id rather than
// by the URL a service or Blueprint stored (w10/m490,
// docs/ADR026-github-integration.md § Repository identity). `tianpan-v4-web`
// missed a month of pushes after `puncsky/tianpan-v3` became `puncsky/tpkb`:
// each push named the new URL, the service stored the old one, and the
// webhook answered 200 having matched nothing. GitHub's id survives a rename
// and a transfer, so:
//
//   - the row stores the id (apps.github_repo_id, blueprints.github_repo_id)
//     from the repository check of a create or a source change, from the
//     first App-signed push that matched it by URL, or from the backfill
//     below; the projector carries a service's onto its App
//     (core.AnnotationGitHubRepoID);
//   - an App-signed push matches a row that has an id by the id alone
//     (gitRepository.tracks), within the installation's workspaces;
//   - a push matched by id whose URL is not the stored one moves the stored
//     URL to the repository's current one, so every read names it.

// repoIdentityStore is the apps row's half of the repository id, a
// compare-and-set on the row's current repo each. *store.PGStore satisfies it.
type repoIdentityStore interface {
	SetAppGitHubRepoID(ctx context.Context, id, repo string, repoID int64) (bool, error)
	MoveAppRepo(ctx context.Context, id string, repoID int64, from, to string) (bool, error)
}

// blueprintRepoIdentityStore is repoIdentityStore for Blueprints.
type blueprintRepoIdentityStore interface {
	SetBlueprintGitHubRepoID(ctx context.Context, id, repo string, repoID int64) (bool, error)
	MoveBlueprintRepo(ctx context.Context, id string, repoID int64, from, to string) (bool, error)
}

// recordGitHubRepoID records the repository id a source change's repository
// check read (next.githubRepoID) on the row SetAppSource just wrote, which
// forgot the old repository's. Best-effort: the change has landed, and a row
// without an id still matches by URL until a push or the backfill supplies it.
func (s *Service) recordGitHubRepoID(ctx context.Context, appID string, next sourceFields) {
	ids, ok := s.Store.(repoIdentityStore)
	if !ok || next.repo == "" || next.githubRepoID == 0 {
		return
	}
	if _, err := ids.SetAppGitHubRepoID(ctx, appID, next.repo, next.githubRepoID); err != nil {
		log.Printf("apps: record GitHub repository id for %s: %v", appID, err)
	}
}

// followRepository keeps the rows an App-signed push reached in step with the
// repository GitHub signed (w10/m490): a row that matched by URL learns the
// repository's id, and a row that matched by id but stores another URL moves
// to the repository's current one. Every write is a compare-and-set, so a
// redelivery or a concurrent push writes and records nothing. A failure is
// logged and never fails the delivery: the push matched either way.
func (h *GitWebhook) followRepository(ctx context.Context, repo gitRepository, apps []appv1alpha1.App, blueprints []store.Blueprint) {
	if repo.ID == 0 || h.Svc == nil {
		return
	}
	for i := range apps {
		if err := h.Svc.followAppRepository(ctx, &apps[i], repo); err != nil {
			log.Printf("webhook: follow GitHub repository %d for %s: %v", repo.ID, apps[i].Name, err)
		}
	}
	for i := range blueprints {
		if err := h.Svc.followBlueprintRepository(ctx, &blueprints[i], repo); err != nil {
			log.Printf("webhook: follow GitHub repository %d for %s: %v", repo.ID, blueprints[i].ID, err)
		}
	}
}

// movedRepoURL is the URL a moved row stores: the repository's web address,
// https://github.com/<owner>/<name>, the form Render's repo field takes. ""
// when the payload carries none bex accepts, or names the stored repository
// already.
func movedRepoURL(stored string, repo gitRepository) string {
	if repoURLsMatch(stored, repo.urls()...) || !store.ValidRepo(repo.HTMLURL) {
		return ""
	}
	return repo.HTMLURL
}

// followAppRepository is followRepository for one App the push matched. Only a
// store-managed App has a row to follow on.
func (s *Service) followAppRepository(ctx context.Context, a *appv1alpha1.App, repo gitRepository) error {
	appID := managedAppID(a)
	ids, ok := s.Store.(repoIdentityStore)
	if appID == "" || !ok {
		return nil
	}
	switch stored := core.GitHubRepoID(a); {
	case stored == 0:
		// The projector stamps the id onto the App from the row.
		_, err := ids.SetAppGitHubRepoID(ctx, appID, a.Spec.Repo, repo.ID)
		return err
	case stored == repo.ID:
		if to := movedRepoURL(a.Spec.Repo, repo); to != "" {
			return s.moveAppRepo(ctx, ids, a, appID, repo.ID, to)
		}
	}
	return nil
}

// moveAppRepo moves a's stored repository to to, in SetSourceAndRegistryCredential's
// order: the pending-source marker first, so the operator keeps serving the
// active release until a deploy asks for the source (the push's own, when it
// deploys), then the row, then the App. The repository is the same one, so
// its clone token and pinned commit stay.
func (s *Service) moveAppRepo(ctx context.Context, ids repoIdentityStore, a *appv1alpha1.App, appID string, repoID int64, to string) error {
	from := a.Spec.Repo
	if err := s.markSourcePending(ctx, a); err != nil {
		return err
	}
	moved, err := ids.MoveAppRepo(ctx, appID, repoID, from, to)
	if err != nil || !moved {
		return err
	}
	if _, err := s.patchUntracked(ctx, a, func(a *appv1alpha1.App) { a.Spec.Repo = to }); err != nil {
		return err
	}
	s.RecordServiceRepoMoved(ctx, a, from, to)
	log.Printf("webhook: %s follows GitHub repository %d from %s to %s", appID, repoID, from, to)
	return nil
}

// followBlueprintRepository is followRepository for one Blueprint the push
// reached.
func (s *Service) followBlueprintRepository(ctx context.Context, b *store.Blueprint, repo gitRepository) error {
	ids, ok := s.Blueprints.(blueprintRepoIdentityStore)
	if !ok {
		return nil
	}
	switch {
	case b.GitHubRepoID == 0:
		_, err := ids.SetBlueprintGitHubRepoID(ctx, b.ID, b.Repo, repo.ID)
		return err
	case b.GitHubRepoID == repo.ID:
		to := movedRepoURL(b.Repo, repo)
		if to == "" {
			return nil
		}
		moved, err := ids.MoveBlueprintRepo(ctx, b.ID, repo.ID, b.Repo, to)
		if err != nil || !moved {
			return err
		}
		s.RecordBlueprintRepoMoved(ctx, b.TenantID, b.ID, b.Repo, to)
		log.Printf("webhook: %s follows GitHub repository %d from %s to %s", b.ID, repo.ID, b.Repo, to)
		b.Repo = to
	}
	return nil
}

// repoAccessLost reports, for Get, whether a service bex's GitHub App
// connected (it carries the repository's GitHub id) builds from a repository
// the workspace's installation can no longer read, by method, the same live
// grant check PushDeliveryMethod reports: the repository moved to an account
// the App is not installed on, or its grant was revoked. GitHub then sends bex
// none of its pushes (w10/m490).
func repoAccessLost(a *appv1alpha1.App, method string) bool {
	return core.GitHubRepoID(a) != 0 && (method == PushDeliveryManualWebhook || method == PushDeliveryUnavailable)
}

// repoIdentityGapStore lists the rows without a repository id and records the
// ids the backfill resolves. *store.PGStore satisfies it.
type repoIdentityGapStore interface {
	ListRepoIdentityGaps(ctx context.Context) ([]store.RepoIdentityGap, error)
	SetAppGitHubRepoID(ctx context.Context, id, repo string, repoID int64) (bool, error)
	SetBlueprintGitHubRepoID(ctx context.Context, id, repo string, repoID int64) (bool, error)
}

// defaultRepoIdentityInterval is how often the backfill looks for rows
// without a repository id: rarely, as each lookup costs GitHub two calls, and
// a create, a source change and the first signed push fill the id on their own.
const defaultRepoIdentityInterval = 30 * time.Minute

// RepoIdentityBackfill resolves GitHub's repository id for every service and
// Blueprint on a github.com repository that has none (w10/m490): rows from
// before ids were stored, a Blueprint's services, a workspace that connected
// GitHub after the service was created. It reads each stored URL through the
// workspace's installation, as a deploy's clone token does, and GitHub's API
// follows a renamed repository's old name, so a row that is already stale
// gets the id that lets its next push match. It records the id only; moving
// the stored URL is the push's job, and the backfill logs what it saw.
type RepoIdentityBackfill struct {
	Svc *Service
	// Interval between passes; non-positive means defaultRepoIdentityInterval.
	Interval time.Duration
}

// Run drives the backfill until ctx is canceled. A no-op without the control
// plane store or the GitHub integration.
func (b *RepoIdentityBackfill) Run(ctx context.Context) {
	if b == nil || b.Svc == nil || b.Svc.GitHub == nil {
		return
	}
	st, ok := b.Svc.Store.(repoIdentityGapStore)
	if !ok {
		return
	}
	interval := b.Interval
	if interval <= 0 {
		interval = defaultRepoIdentityInterval
	}
	core.Poll(ctx, "github repository ids", interval, func(ctx context.Context) error {
		return b.pass(ctx, st)
	})
}

// pass resolves every gap once. A row the installation cannot read, or a
// failed lookup, waits for the next pass.
func (b *RepoIdentityBackfill) pass(ctx context.Context, st repoIdentityGapStore) error {
	gaps, err := st.ListRepoIdentityGaps(ctx)
	if err != nil {
		return err
	}
	for _, g := range gaps {
		if ctx.Err() != nil {
			return nil
		}
		repo, granted, err := b.Svc.GitHub.GrantedRepo(ctx, g.TenantID, g.Repo)
		if err != nil {
			log.Printf("github repository ids: read %s for %s %s: %v", g.Repo, g.Kind, g.ID, err)
			continue
		}
		if !granted || repo.ID == 0 {
			continue
		}
		record := st.SetAppGitHubRepoID
		if g.Kind == store.RepoIdentityBlueprint {
			record = st.SetBlueprintGitHubRepoID
		}
		if _, err := record(ctx, g.ID, g.Repo, repo.ID); err != nil {
			log.Printf("github repository ids: record %d for %s %s: %v", repo.ID, g.Kind, g.ID, err)
			continue
		}
		if !repoURLsMatch(g.Repo, repo.HTMLURL, repo.CloneURL) {
			log.Printf("github repository ids: %s %s stores %s, which GitHub now names %s (id %d); its next push moves it",
				g.Kind, g.ID, g.Repo, repo.FullName, repo.ID)
		}
	}
	return nil
}
