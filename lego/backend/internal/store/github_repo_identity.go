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

package store

import (
	"context"
)

// github_repo_identity.go keeps GitHub's numeric repository id beside the repo
// URL a service or Blueprint stores (w10/m490, docs/ADR026-github-integration.md
// § Repository identity). The id survives a rename or a transfer, so a signed
// push matches by it, and a push that names a new URL for a known id moves the
// stored URL to it. Every write here is a compare-and-set on the row's current
// repo (and, for a move, its id): a concurrent source change or a redelivered
// push then writes nothing, and a caller records a fact only for a write that
// landed.

// RepoIdentityGap kinds: which table a gap row lives in.
const (
	RepoIdentityService   = "service"
	RepoIdentityBlueprint = "blueprint"
)

// RepoIdentityGap is a service or Blueprint whose GitHub repository has no id
// recorded yet: the backfill resolves Repo through the workspace's GitHub App
// installation (apps.RepoIdentityBackfill).
type RepoIdentityGap struct {
	Kind     string
	ID       string
	TenantID string
	Repo     string
}

// ListRepoIdentityGaps returns every service and live Blueprint that builds
// from a github.com repository and has no GitHub repository id. The host test
// is a coarse prefilter; the caller parses each URL itself. A build source is
// left out: no push reaches one yet (apps.repoCandidates).
func (s *PGStore) ListRepoIdentityGaps(ctx context.Context) ([]RepoIdentityGap, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT '`+RepoIdentityService+`', a.id, a.tenant_id, a.repo FROM apps a
		 WHERE a.github_repo_id = 0 AND a.repo ILIKE '%github.com%' AND `+serviceAppsOnly+`
		UNION ALL
		SELECT '`+RepoIdentityBlueprint+`', id, tenant_id, repo FROM blueprints
		 WHERE github_repo_id = 0 AND status <> 'disconnected' AND repo ILIKE '%github.com%'
		ORDER BY 1, 2`)
	if err != nil {
		return nil, classify("app", err)
	}
	defer rows.Close()
	var out []RepoIdentityGap
	for rows.Next() {
		var g RepoIdentityGap
		if err := rows.Scan(&g.Kind, &g.ID, &g.TenantID, &g.Repo); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// SetAppGitHubRepoID records repoID as the GitHub id of the repository service
// id builds from, provided the row still names repo. It reports whether the
// row changed. The projector carries the id onto the App's
// core.AnnotationGitHubRepoID.
func (s *PGStore) SetAppGitHubRepoID(ctx context.Context, id, repo string, repoID int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE apps SET github_repo_id = $3
		 WHERE id = $1 AND repo = $2 AND github_repo_id <> $3`, id, repo, repoID)
	if err != nil {
		return false, classify("app", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SetBlueprintGitHubRepoID is SetAppGitHubRepoID for a live Blueprint.
func (s *PGStore) SetBlueprintGitHubRepoID(ctx context.Context, id, repo string, repoID int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE blueprints SET github_repo_id = $3
		 WHERE id = $1 AND repo = $2 AND github_repo_id <> $3 AND status <> 'disconnected'`, id, repo, repoID)
	if err != nil {
		return false, classify("blueprint", err)
	}
	return tag.RowsAffected() > 0, nil
}

// MoveAppRepo replaces service id's repo from with to, provided the row still
// names from and records repoID (never 0): GitHub renamed or transferred the
// repository. It reports whether the row changed, so of a push and its
// redelivery only one records the move.
func (s *PGStore) MoveAppRepo(ctx context.Context, id string, repoID int64, from, to string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE apps SET repo = $4, updated_at = now()
		 WHERE id = $1 AND github_repo_id = $2 AND $2 <> 0 AND repo = $3`, id, repoID, from, to)
	if err != nil {
		return false, classify("app", err)
	}
	return tag.RowsAffected() > 0, nil
}

// MoveBlueprintRepo is MoveAppRepo for a live Blueprint.
func (s *PGStore) MoveBlueprintRepo(ctx context.Context, id string, repoID int64, from, to string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE blueprints SET repo = $4, updated_at = now()
		 WHERE id = $1 AND github_repo_id = $2 AND $2 <> 0 AND repo = $3 AND status <> 'disconnected'`, id, repoID, from, to)
	if err != nil {
		return false, classify("blueprint", err)
	}
	return tag.RowsAffected() > 0, nil
}
