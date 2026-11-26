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
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The reads behind a sandbox group's Snapshots and Custom images pages
// (w1/m658), as Render's dashboard makes them: `SandboxSnapshots(input)` with
// a search and a time range, a snapshot by id alone for a link, and
// `SandboxBuildSources(input)`, one row per image name.

// SnapshotQuery narrows a group's snapshots for its page.
type SnapshotQuery struct {
	Name     string
	Statuses []string
	// Search matches a snapshot's id exactly or its name in part, ignoring case.
	Search                      string
	SourceSandboxID             string
	CreatedAfter, CreatedBefore *time.Time
	Cursor                      string
	Limit                       int
}

// SnapshotPage is one page of a group's snapshots, newest first.
type SnapshotPage struct {
	Snapshots []Snapshot
	Cursor    string
	HasMore   bool
}

// SearchSnapshots is one page of a group's snapshots, filtered as the
// Snapshots page and a sandbox's Snapshots tab ask. A group of another
// workspace is not found.
func (s *Service) SearchSnapshots(ctx context.Context, groupID string, q SnapshotQuery) (SnapshotPage, error) {
	ctx, group, err := s.snapshotGroup(ctx, groupID, core.RelCanView)
	if err != nil {
		return SnapshotPage{}, err
	}
	if err := validateSnapshotStatuses(q.Statuses); err != nil {
		return SnapshotPage{}, err
	}
	rows, err := s.Snapshots.Store.SandboxSnapshots(ctx, group.WorkspaceID, group.ID, q.Name, q.Statuses, s.Now())
	if err != nil {
		return SnapshotPage{}, store.MapError(err)
	}
	search := strings.ToLower(strings.TrimSpace(q.Search))
	var out []Snapshot
	for _, row := range rows {
		sn := snapshotView(row)
		if matchesIDOrName(search, sn.ID, sn.Name) &&
			(q.SourceSandboxID == "" || sn.SourceSandboxID == q.SourceSandboxID) &&
			(q.CreatedAfter == nil || !sn.RequestedAt.Before(*q.CreatedAfter)) &&
			(q.CreatedBefore == nil || sn.RequestedAt.Before(*q.CreatedBefore)) {
			out = append(out, sn)
		}
	}
	var page SnapshotPage
	page.Snapshots, page.Cursor, page.HasMore = pageAfter(out, q.Cursor, q.Limit, func(sn Snapshot) string { return sn.ID })
	return page, nil
}

// GetSnapshotByID is a snapshot of the caller's workspace by its id alone,
// whatever its group: what a link to a snapshot resolves, as Render's
// `resourceTypeBySandboxSnapshotId`. Another workspace's id is not found.
func (s *Service) GetSnapshotByID(ctx context.Context, snapshotID string) (Snapshot, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Snapshot{}, err
	}
	if err := s.requireSnapshots(); err != nil {
		return Snapshot{}, err
	}
	if err := validateSnapshotID(snapshotID); err != nil {
		return Snapshot{}, err
	}
	snap, err := s.Snapshots.Store.SandboxSnapshot(ctx, s.WorkspaceOrDefault(ctx), "", snapshotID, s.Now())
	if errors.Is(err, store.ErrNotFound) {
		return Snapshot{}, snapshotNotFound(snapshotID)
	}
	if err != nil {
		return Snapshot{}, store.MapError(err)
	}
	return snapshotView(snap), nil
}

// Render's SandboxSnapshotFailureCode values.
const (
	snapshotFailureDeadline = "DEADLINE_EXCEEDED"
	snapshotFailureInternal = "INTERNAL_ERROR"
)

// snapshotFailureCode is a failed snapshot's Render failure code: a capture
// that ran out of time, or any other failure bex records.
func snapshotFailureCode(sn Snapshot) *string {
	if sn.Status != store.SandboxSnapshotFailed {
		return nil
	}
	code := snapshotFailureInternal
	if sn.Error != nil && *sn.Error == snapshotTimedOutReason {
		code = snapshotFailureDeadline
	}
	return &code
}

// ImageSource is one row of the Custom images page: an image name with the
// source it was last imported from, its newest version and its newest
// available one.
type ImageSource struct {
	Name                 string
	Source               BuildSource
	LatestBuild          Build
	LatestSucceededBuild *Build
}

// ImageSourcePage is one page of a group's image names, newest import first.
type ImageSourcePage struct {
	Sources []ImageSource
	Cursor  string
	HasMore bool
}

// ListImageSources is one page of a group's images by name. Search matches a
// name in part, ignoring case. A group of another workspace is not found.
func (s *Service) ListImageSources(ctx context.Context, groupID, search, cursor string, limit int) (ImageSourcePage, error) {
	ctx, group, err := s.imageGroup(ctx, groupID, core.RelCanView)
	if err != nil {
		return ImageSourcePage{}, err
	}
	rows, err := s.Images.Store.SandboxBuilds(ctx, group.WorkspaceID, group.ID, "", nil)
	if err != nil {
		return ImageSourcePage{}, store.MapError(err)
	}
	builds := make([]Build, 0, len(rows))
	for _, b := range rows {
		builds = append(builds, buildView(b))
	}
	// The store lists builds newest first, so a name's first is its latest.
	search = strings.ToLower(strings.TrimSpace(search))
	var sources []ImageSource
	index := map[string]int{}
	for _, b := range builds {
		i, seen := index[b.Name]
		if !seen {
			if search != "" && !strings.Contains(strings.ToLower(b.Name), search) {
				continue
			}
			index[b.Name] = len(sources)
			sources = append(sources, ImageSource{Name: b.Name, Source: b.Source, LatestBuild: b})
			i = len(sources) - 1
		}
		if b.Status == store.SandboxBuildSucceeded && sources[i].LatestSucceededBuild == nil {
			latest := b
			sources[i].LatestSucceededBuild = &latest
		}
	}
	var out ImageSourcePage
	out.Sources, out.Cursor, out.HasMore = pageAfter(sources, cursor, limit, func(src ImageSource) string { return src.Name })
	return out, nil
}
