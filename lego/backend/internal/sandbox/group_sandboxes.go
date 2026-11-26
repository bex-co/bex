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
	"slices"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A sandbox group's Sandboxes page (w1/m658), as Render's dashboard reads it:
// `sandboxesForGroup(input)` pages the group's sandboxes, live and ended,
// filtered by status, a search and a creation range; the group's run summary
// counts the runs that ended in the range; and `runningSandboxCount` is what
// counts against the group's concurrency limit now.

// GroupSandboxQuery is Render's SandboxesForGroupFilterInput, plus the
// snapshot or image the sandboxes started from (Render's snapshot and image
// drawers list those sandboxes).
type GroupSandboxQuery struct {
	GroupID string
	// Statuses narrow the list; none lists every status, ended ones included.
	Statuses []string
	// Search matches a sandbox's id exactly or its name in part, ignoring case.
	Search                      string
	CreatedAfter, CreatedBefore *time.Time
	SourceSnapshotID            string
	// SourceBuilds are imported images' versions (sbb- ids); a sandbox that
	// started from any of them is listed.
	SourceBuilds []string
	Cursor       string
	Limit        int
}

// GroupSandboxPage is one page of a group's sandboxes, newest first.
type GroupSandboxPage struct {
	Sandboxes []Sandbox
	Cursor    string
	HasMore   bool
}

// Render's sort of the list, the only one bex serves.
const (
	groupSandboxSortField     = "CREATED_AT"
	groupSandboxSortDirection = "DESC"
)

// groupSandboxes are every sandbox of the group the caller may see, live and
// ended, newest first, read in the group's own workspace with no allow-list
// rules (a page reads its own). One GraphQL document reads it once: the list,
// its run summary and the running count poll together.
func (s *Service) groupSandboxes(ctx context.Context, groupID string) (context.Context, []Sandbox, error) {
	ctx, err := s.scopeGroup(ctx, groupID)
	if err != nil {
		return ctx, nil, err
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return ctx, nil, err
	}
	if err := validateGroupID(groupID); err != nil {
		return ctx, nil, err
	}
	key := "sandbox.groupSandboxes:" + s.WorkspaceOrDefault(ctx) + ":" + groupID
	boxes, err := core.Memoized(ctx, key, func() ([]Sandbox, error) {
		return s.listFilteredRules(ctx, renderListStatuses, "", groupID, false)
	})
	// A copy: the memo's slice is shared by every reader of the document.
	return ctx, slices.Clone(boxes), err
}

// matchesIDOrName is the pages' search: an id exactly or a name in part,
// ignoring case; search is already lower-cased and trimmed.
func matchesIDOrName(search, id string, name *string) bool {
	return search == "" || strings.ToLower(id) == search || (name != nil && strings.Contains(strings.ToLower(*name), search))
}

// pageAfter is one page of items after the cursor (core.Page), whether more
// follow, and the cursor of the next page.
func pageAfter[T any](items []T, cursor string, limit int, cursorOf func(T) string) (page []T, next string, more bool) {
	limit = core.PageLimitOrDefault(limit)
	page = core.Page(items, cursor, limit+1, cursorOf)
	more = len(page) > limit
	page = page[:min(limit, len(page))]
	if n := len(page); n > 0 {
		next = cursorOf(page[n-1])
	}
	return page, next, more
}

// ListGroupSandboxes is one page of a group's sandboxes. A group of another
// workspace is not found.
func (s *Service) ListGroupSandboxes(ctx context.Context, q GroupSandboxQuery) (GroupSandboxPage, error) {
	want, err := parseStatusFilter(q.Statuses)
	if err != nil {
		return GroupSandboxPage{}, err
	}
	ctx, boxes, err := s.groupSandboxes(ctx, q.GroupID)
	if err != nil {
		return GroupSandboxPage{}, err
	}
	search := strings.ToLower(strings.TrimSpace(q.Search))
	boxes = core.Filter(boxes, func(b Sandbox) bool {
		return (len(want) == 0 || want[b.Status]) &&
			matchesIDOrName(search, b.ID, b.Name) &&
			createdWithin(b, q.CreatedAfter, q.CreatedBefore) &&
			(q.SourceSnapshotID == "" || b.Image == q.SourceSnapshotID) &&
			(len(q.SourceBuilds) == 0 || slices.Contains(q.SourceBuilds, b.Image))
	})
	var out GroupSandboxPage
	out.Sandboxes, out.Cursor, out.HasMore = pageAfter(boxes, q.Cursor, q.Limit, func(b Sandbox) string { return b.ID })
	ws := s.WorkspaceOrDefault(ctx)
	for i := range out.Sandboxes {
		if out.Sandboxes[i], err = s.withRules(ctx, ws, out.Sandboxes[i]); err != nil {
			return GroupSandboxPage{}, err
		}
	}
	out.Sandboxes, err = s.withDetails(ctx, ws, out.Sandboxes)
	return out, err
}

// createdWithin reports whether b was created in [after, before).
func createdWithin(b Sandbox, after, before *time.Time) bool {
	if b.CreatedAt == nil {
		return after == nil && before == nil
	}
	return (after == nil || !b.CreatedAt.Before(*after)) && (before == nil || b.CreatedAt.Before(*before))
}

// GroupRunningCount is how many of the group's sandboxes hold a concurrency
// slot now (holdsSlotStatus): Render's "Creating, running, resuming, or
// suspending now".
func (s *Service) GroupRunningCount(ctx context.Context, groupID string) (int, error) {
	_, boxes, err := s.groupSandboxes(ctx, groupID)
	if err != nil {
		return 0, err
	}
	return len(core.Filter(boxes, func(b Sandbox) bool { return holdsSlotStatus(b.Status) })), nil
}

// RunBucket is one interval of a group's run chart: the runs that ended in it.
type RunBucket struct {
	Time      time.Time
	Succeeded int
	Errored   int
}

// GroupRunSummary is Render's sandbox run summary over a time range: the runs
// that ended in it, how many ended in an error, and the ended runs by
// interval for the chart.
type GroupRunSummary struct {
	Total     int
	Errored   int
	Succeeded int
	Buckets   []RunBucket
}

// runBuckets is how many intervals a run chart splits its range into.
const runBuckets = 30

// GroupRuns summarizes the group's runs that ended in [after, before), before
// defaulting to now and after to the oldest end. A terminated sandbox ended
// when it was terminated; an errored one is counted where it was created,
// since bex does not record when it errored.
func (s *Service) GroupRuns(ctx context.Context, groupID string, after, before *time.Time) (GroupRunSummary, error) {
	_, boxes, err := s.groupSandboxes(ctx, groupID)
	if err != nil {
		return GroupRunSummary{}, err
	}
	// With no end the range runs to now, a run that ended this instant in it.
	end := s.Now().UTC().Add(time.Nanosecond)
	if before != nil {
		end = before.UTC()
	}
	type run struct {
		at      time.Time
		errored bool
	}
	var runs []run
	for _, b := range boxes {
		r := run{errored: b.Status == StatusErrored}
		switch {
		case r.errored && b.CreatedAt != nil:
			r.at = *b.CreatedAt
		case !r.errored && b.TerminatedAt != nil:
			r.at = *b.TerminatedAt
		default:
			continue
		}
		if (after != nil && r.at.Before(*after)) || !r.at.Before(end) {
			continue
		}
		runs = append(runs, r)
	}
	start := end.Add(-time.Hour)
	if after != nil {
		start = after.UTC()
	} else if len(runs) > 0 {
		start = slices.MinFunc(runs, func(a, b run) int { return a.at.Compare(b.at) }).at.UTC()
	}
	step := max(time.Minute, end.Sub(start)/runBuckets).Round(time.Minute)
	summary := GroupRunSummary{}
	for t := start.Truncate(step); t.Before(end); t = t.Add(step) {
		summary.Buckets = append(summary.Buckets, RunBucket{Time: t})
	}
	for _, r := range runs {
		i := min(len(summary.Buckets)-1, max(0, int(r.at.Sub(start.Truncate(step))/step)))
		summary.Total++
		if r.errored {
			summary.Errored++
			summary.Buckets[i].Errored++
		} else {
			summary.Succeeded++
			summary.Buckets[i].Succeeded++
		}
	}
	return summary, nil
}

// withDetails fills what Render's drawer reads about each sandbox and bex
// keeps beside it: who started it and through which surface, when it first
// ran, and the name of the image it started from.
func (s *Service) withDetails(ctx context.Context, ws string, boxes []Sandbox) ([]Sandbox, error) {
	if len(boxes) == 0 {
		return boxes, nil
	}
	if s.Lifecycle != nil {
		sandboxIDs := make([]string, len(boxes))
		for i, b := range boxes {
			sandboxIDs[i] = b.ID
		}
		events, err := s.Lifecycle.SandboxStartEvents(ctx, ws, sandboxIDs)
		if err != nil {
			return nil, err
		}
		for _, e := range events {
			i := slices.IndexFunc(boxes, func(b Sandbox) bool { return b.ID == e.SandboxID })
			if i < 0 {
				continue
			}
			switch e.LifecycleType {
			case LifecycleCreated:
				boxes[i].details.trigger, boxes[i].details.createdBy = e.Trigger, e.UserEmail
			case LifecycleReady:
				if boxes[i].details.startedAt == nil {
					at := e.At.UTC()
					boxes[i].details.startedAt = &at
				}
			}
		}
	}
	names, err := s.imageNames(ctx, ws, boxes)
	if err != nil {
		return nil, err
	}
	for i := range boxes {
		b := &boxes[i]
		if b.details.startedAt == nil && b.Status != StatusCreating {
			b.details.startedAt = b.CreatedAt
		}
		// A deleted image keeps its id on the sandbox and has no name.
		b.details.sourceBuildName = names[b.sourceBuildID()]
	}
	return boxes, nil
}

// imageNames are the names of the imported images the sandboxes started from,
// by build id: one read of each group's builds.
func (s *Service) imageNames(ctx context.Context, ws string, boxes []Sandbox) (map[string]string, error) {
	names := map[string]string{}
	if s.Images == nil {
		return names, nil
	}
	read := map[string]bool{}
	for _, b := range boxes {
		if b.sourceBuildID() == "" || b.SandboxGroupID == "" || read[b.SandboxGroupID] {
			continue
		}
		read[b.SandboxGroupID] = true
		builds, err := s.Images.Store.SandboxBuilds(ctx, ws, b.SandboxGroupID, "", nil)
		if err != nil {
			return nil, store.MapError(err)
		}
		for _, build := range builds {
			names[build.ID] = build.Name
		}
	}
	return names, nil
}

// sandboxDetails are the parts of a sandbox only the dashboard's drawer reads
// (w1/m658), on no REST or MCP shape.
type sandboxDetails struct {
	startedAt       *time.Time
	timeoutAt       *time.Time
	trigger         string
	createdBy       string
	sourceBuildName string
}

// sourceSnapshotID is the snapshot the sandbox was restored from, if any.
func (b Sandbox) sourceSnapshotID() string {
	if ids.Is(ids.SandboxSnapshot, b.Image) {
		return b.Image
	}
	return ""
}

// sourceBuildID is the imported image the sandbox started from, if any.
func (b Sandbox) sourceBuildID() string {
	if ids.Is(ids.SandboxBuild, b.Image) {
		return b.Image
	}
	return ""
}

// GetDetailed is Get with what the dashboard's drawer reads (withDetails).
func (s *Service) GetDetailed(ctx context.Context, id string) (Sandbox, error) {
	ctx, err := s.scopeSandbox(ctx, "", id)
	if err != nil {
		return Sandbox{}, err
	}
	box, err := s.Get(ctx, id)
	if err != nil {
		return Sandbox{}, err
	}
	out, err := s.withDetails(ctx, s.WorkspaceOrDefault(ctx), []Sandbox{box})
	if err != nil {
		return Sandbox{}, err
	}
	return out[0], nil
}
