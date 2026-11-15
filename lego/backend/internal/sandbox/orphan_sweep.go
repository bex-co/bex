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
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// WorkspaceLookup says which workspaces still exist; *store.PGStore satisfies it.
type WorkspaceLookup interface {
	// ExistingWorkspaceIDs returns the given ids that name a workspace whose
	// row exists, whatever stage of deletion it has reached, or that is being
	// created.
	ExistingWorkspaceIDs(ctx context.Context, workspaceIDs []string) (map[string]bool, error)
}

const (
	// orphanSweepInterval is how often a replica may start a round: the sweep
	// closes a gap that opens only when a workspace is deleted, so a few registry
	// calls every few hours are enough.
	orphanSweepInterval = 3 * time.Hour
	// orphanSweepTick is how often Run asks whether a round is due.
	orphanSweepTick = time.Minute

	// A round reads at most orphanCatalogPages pages of orphanCatalogPage
	// repositories and deletes the images of at most orphanMaxRepositories
	// repositories, within orphanSweepBudget; the rest wait for the next round.
	orphanCatalogPage     = 500
	orphanCatalogPages    = 20
	orphanMaxRepositories = 100
	orphanSweepBudget     = 2 * time.Minute
)

// OrphanSweeper deletes the images a deleted workspace left in the registry
// (w22/017): the repositories under `<prefix>/<ws>-sandbox/` of a workspace with
// no row in the tenants table. The workspace purge deletes a workspace's
// suspended sandboxes' images before the tenant row goes, but whatever it could
// not reach (an image committed after the purge, a workspace deleted before the
// purge existed, a delete that ended in the cascade) has no row naming it, so the
// registry's catalog is the only record. The sweep reads the catalog, takes the
// repositories under a workspace's sandbox namespace, asks which of those
// workspaces exist, and deletes the controller's and importer's tags of the rest
// with the reaper's credential.
//
// It deletes only inside the snapshot prefix and only under a namespace that
// parses as a workspace's; it never deletes for a workspace that exists, however
// far its own delete has got, or that is being created; and a failed catalog or
// workspace read ends the round with nothing deleted, because a lookup that
// fails would make every workspace look gone. It is bounded per round and
// paces itself per replica; replicas may sweep at the same time, a tag the
// other deleted first being success.
//
// It belongs to the control plane that owns the registry: with several control
// planes on one registry each would see the others' workspaces as gone, so the
// composition root runs it for the default identity only (ADR043 D9).
type OrphanSweeper struct {
	Reaper     *RegistryReaper
	Workspaces WorkspaceLookup
	// Metrics, when set, counts the repositories reaped and the failures.
	Metrics *StorageMetrics
	// Interval is the pause between rounds on this replica (default
	// orphanSweepInterval).
	Interval time.Duration
	// Now is the clock (default time.Now).
	Now func() time.Time

	// The bounds, and the pace of Run's check, with the constants as defaults:
	// fields so the tests can shrink them.
	pageSize, maxPages, maxRepositories int
	tick                                time.Duration

	mu   sync.Mutex
	last time.Time // when the last round started
}

func (s *OrphanSweeper) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *OrphanSweeper) interval() time.Duration {
	return cmp.Or(s.Interval, orphanSweepInterval)
}

// Run checks for a due round every orphanSweepTick until ctx ends, so the first
// round follows a minute after start and then one starts per interval.
func (s *OrphanSweeper) Run(ctx context.Context) {
	if s == nil {
		return
	}
	ticker := time.NewTicker(cmp.Or(s.tick, orphanSweepTick))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Sweep(ctx)
		}
	}
}

// due starts a round unless one started less than an interval ago. A round that
// fails counts as started: a registry that is down is not asked again at once.
func (s *OrphanSweeper) due() bool {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.last.IsZero() && now.Sub(s.last) < s.interval() {
		return false
	}
	s.last = now
	return true
}

// Sweep runs one round if one is due.
func (s *OrphanSweeper) Sweep(ctx context.Context) {
	if s == nil || s.Reaper == nil || s.Workspaces == nil || !s.due() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, orphanSweepBudget)
	defer cancel()
	reaped, err := s.round(ctx)
	if err != nil {
		s.Metrics.orphanSweepFailed()
		log.Printf("sandbox orphan sweep: round skipped, nothing deleted: %v", err)
	}
	if reaped > 0 {
		log.Printf("sandbox orphan sweep: deleted the images of %d repositories of deleted workspaces", reaped)
	}
}

// orphanRepository is a repository under a workspace's sandbox namespace.
type orphanRepository struct{ repo, workspaceID string }

func (s *OrphanSweeper) round(ctx context.Context) (int, error) {
	catalog, err := s.readCatalog(ctx)
	if err != nil {
		return 0, fmt.Errorf("read the catalog: %w", err)
	}
	candidates := s.Reaper.sandboxRepositories(catalog)
	if len(candidates) == 0 {
		return 0, nil
	}
	var workspaceIDs []string
	for _, c := range candidates {
		workspaceIDs = append(workspaceIDs, c.workspaceID)
	}
	slices.Sort(workspaceIDs)
	exist, err := s.Workspaces.ExistingWorkspaceIDs(ctx, slices.Compact(workspaceIDs))
	if err != nil {
		return 0, fmt.Errorf("look up the workspaces: %w", err)
	}
	reaped, consecutive := 0, 0
	for _, c := range candidates {
		if exist[c.workspaceID] {
			continue
		}
		deleted, err := s.Reaper.deleteTags(ctx, c.repo, ownsOrphanTag)
		if err != nil {
			s.Metrics.orphanSweepFailed()
			log.Printf("sandbox orphan sweep: delete the images of %s (workspace %s is gone): %v", c.repo, c.workspaceID, err)
			if consecutive++; consecutive >= storageRegistryFailures {
				return reaped, nil
			}
			continue
		}
		consecutive = 0
		if deleted == 0 {
			continue // emptied already: the catalog may still list it
		}
		reaped++
		s.Metrics.orphanRepositoryReaped()
		log.Printf("sandbox orphan sweep: deleted %d image tags of %s (workspace %s no longer exists)", deleted, c.repo, c.workspaceID)
		if reaped >= cmp.Or(s.maxRepositories, orphanMaxRepositories) {
			return reaped, nil
		}
	}
	return reaped, nil
}

// readCatalog reads the registry's repositories page by page, all of them before
// any is deleted so that no page boundary is a repository this sweep has just
// emptied.
func (s *OrphanSweeper) readCatalog(ctx context.Context) ([]string, error) {
	pageSize := cmp.Or(s.pageSize, orphanCatalogPage)
	var all []string
	last := ""
	for range cmp.Or(s.maxPages, orphanCatalogPages) {
		page, err := s.Reaper.catalog(ctx, last, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			return all, nil
		}
		last = page[len(page)-1]
	}
	log.Printf("sandbox orphan sweep: the catalog holds more than %d repositories; those past them are not swept", len(all))
	return all, nil
}

// ownsOrphanTag is the tags the sweep deletes: the controller's snapshot tags
// and the importer's image tag, the two things bex writes under a namespace.
func ownsOrphanTag(tag string) bool { return snapshotTag.MatchString(tag) || tag == imageTag }

// sandboxRepositories picks, in catalog order and without repeats, the
// repositories of the form `<prefix>/<ws>-sandbox/<name>` whose `<ws>` is a
// workspace id; anything else is not this sweep's to delete.
func (r *RegistryConfig) sandboxRepositories(catalog []string) []orphanRepository {
	prefix := r.Prefix
	if prefix == "" {
		prefix = DefaultSnapshotRepoPrefix
	}
	var out []orphanRepository
	seen := map[string]bool{}
	for _, repo := range catalog {
		rest, ok := strings.CutPrefix(repo, prefix+"/")
		if !ok || seen[repo] {
			continue
		}
		namespace, name, ok := strings.Cut(rest, "/")
		if !ok || name == "" {
			continue
		}
		workspaceID, ok := store.WorkspaceOfSandboxNamespace(namespace)
		if !ok {
			continue
		}
		seen[repo] = true
		out = append(out, orphanRepository{repo: repo, workspaceID: workspaceID})
	}
	return out
}

// catalog lists up to n repositories that follow last in the registry's order
// (last "" starts at the first): the distribution API's `_catalog`, which Zot
// limits to the repositories the credential may read.
func (r *RegistryConfig) catalog(ctx context.Context, last string, n int) ([]string, error) {
	query := url.Values{"n": {strconv.Itoa(n)}}
	if last != "" {
		query.Set("last", last)
	}
	body, _, err := r.get(ctx, "/v2/_catalog?"+query.Encode(), "")
	if err != nil {
		return nil, err
	}
	var page struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("registry catalog: bad response")
	}
	return page.Repositories, nil
}
