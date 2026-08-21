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

package logs

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"time"
)

// Platform progress lines (w1/m48): Render's deploy feed is never silent — the
// platform itself narrates (`==> Cloning from…`, `==> Your service is live 🎉`)
// while the build pod schedules and pulls. bex synthesizes the equivalent from
// the deploy row's observed lifecycle (created/started/finished + terminal
// status), so a cold node's minutes-long pre-build window shows the deploy's
// own narration instead of nothing. The contract table lives in
// docs/render-artifacts/live-deploy-following.md § Platform progress-line
// contract. Lines carry type=build (they narrate the build/deploy), the deploy
// id as `instance`, and container "platform"; ids derive from the existing
// logID (instance+timestamp+message), so identical lines are deterministic
// across reads and dedupe against the live tail for free.

// progressContainer is the `container` label on synthesized lines — no pod
// container is called this (buildkit / sign / kpack step names), so the
// platform's own narration stays distinguishable from real build stdout.
const progressContainer = "platform"

// DeployProgress is one deploy row's lifecycle facts, the input to line
// synthesis — the logs domain's own shape so it stays store-free (the
// composition root adapts store.Deploy, the way PodLogSource keeps the
// clientset out).
type DeployProgress struct {
	ID         string
	Status     string
	Image      string // image-backed deploys; "" for repo builds
	Commit     string // resolved commit sha; "" when unresolved
	CreatedAt  time.Time
	StartedAt  time.Time // zero until the deploy starts
	FinishedAt time.Time // zero until terminal
}

// DeployProgressSource lists an App's deploy rows, newest-first, created
// before `end` (zero = no upper bound), bounded to a reasonable page by the
// implementation. nil => no synthesis (byte-identical prior behavior). The
// resource key is the public srv- id — the store's app_id — so legacy
// label-less Apps simply resolve to no rows.
type DeployProgressSource func(ctx context.Context, resource string, end time.Time) ([]DeployProgress, error)

// progressLines renders the platform lines a deploy row has earned so far.
// Only observed moments produce lines — no invented clone/checkout stages
// (those arrive as real BuildKit stdout once the pod runs). `deactivated`
// keeps its live line: finished_at records when this deploy went live; its
// later replacement is outside this deploy's own story.
func progressLines(d DeployProgress, repo, branch, serviceType string) []LogEntry {
	repoBacked := repo != ""
	var out []LogEntry
	add := func(t time.Time, msg string) {
		out = append(out, LogEntry{
			Timestamp: t.UTC().Format(time.RFC3339Nano),
			Message:   msg,
			Labels: map[string]string{
				LabelType:     LogTypeBuild,
				LabelInstance: d.ID,
				"container":   progressContainer,
			},
		})
	}
	if !d.CreatedAt.IsZero() {
		if repoBacked {
			add(d.CreatedAt, "==> Build queued")
		} else {
			add(d.CreatedAt, "==> Deploy queued")
		}
	}
	if !d.StartedAt.IsZero() {
		if repoBacked {
			ref := d.Commit
			if len(ref) > 7 {
				ref = ref[:7]
			}
			if ref == "" {
				ref = branch
			}
			if ref != "" {
				add(d.StartedAt, fmt.Sprintf("==> Building from %s@%s", repo, ref))
			} else {
				add(d.StartedAt, fmt.Sprintf("==> Building from %s", repo))
			}
		} else {
			add(d.StartedAt, fmt.Sprintf("==> Deploying image %s", d.Image))
		}
	}
	if !d.FinishedAt.IsZero() {
		if msg := terminalLine(d.Status, serviceType); msg != "" {
			add(d.FinishedAt, msg)
		}
	}
	return out
}

// terminalLine maps a terminal deploy status to its closing line; "" for the
// open states (no line yet) and unknown values (never invent an outcome). A
// cron_job runs its command to completion on a schedule — it is never a served
// "live" service — so a successful deploy narrates that it is scheduled, not
// live (background workers keep the "live" line: they run continuously).
func terminalLine(status, serviceType string) string {
	switch status {
	case "live", "deactivated":
		if serviceType == "cron_job" {
			return "==> Cron job deployed — runs on schedule 🎉"
		}
		return "==> Your service is live 🎉"
	case "build_failed":
		return "==> Build failed"
	case "pre_deploy_failed":
		return "==> Pre-deploy failed"
	case "update_failed":
		return "==> Deploy failed"
	case "canceled":
		return "==> Deploy canceled"
	}
	return ""
}

// synthesizeProgress merges platform progress lines into a windowed read's
// entries. Applies only when the query explicitly asks for build logs (the
// same condition under which the store selector includes build streams) and a
// source is wired. Additive by design: a source error degrades to the plain
// read (narration must never break a log query), and it runs only on the
// store-backed path — a missing Loki still reports buildStoreUnavailable, so
// platform lines never masquerade as a successful empty build history.
func (s *Service) synthesizeProgress(ctx context.Context, q LogQuery, resource, repo, branch, serviceType string, entries []LogEntry) []LogEntry {
	if s.DeployProgress == nil || !slices.Contains(q.Types, LogTypeBuild) {
		return entries
	}
	rows, err := s.DeployProgress(ctx, resource, q.End)
	if err != nil {
		return entries
	}
	merged := entries
	for _, d := range rows {
		// Skip rows that ended before the window opened; per-line keep()
		// applies the exact bounds below.
		if !q.Since.IsZero() && !d.FinishedAt.IsZero() && d.FinishedAt.Before(q.Since) {
			continue
		}
		if !q.keepPod(d.ID) {
			continue
		}
		for _, e := range progressLines(d, repo, branch, serviceType) {
			if q.keep(e) {
				merged = append(merged, e)
			}
		}
	}
	if len(merged) == len(entries) {
		return entries
	}
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].Timestamp < merged[j].Timestamp })
	return q.capToLimit(merged)
}

// progressFollower tracks which platform lines a live build tail has already
// emitted, so subscribe-time catch-up, wait-loop transitions, and the
// post-stream terminal check each emit a line exactly once.
type progressFollower struct {
	s           *Service
	q           LogQuery
	resource    string
	repo        string
	branch      string
	serviceType string
	emitted     map[string]bool
}

// newProgressFollower returns nil when no source is wired — every method is
// nil-safe, so the tail path stays a straight line.
func (s *Service) newProgressFollower(q LogQuery, resource, repo, branch, serviceType string) *progressFollower {
	if s.DeployProgress == nil {
		return nil
	}
	return &progressFollower{s: s, q: q, resource: resource, repo: repo, branch: branch, serviceType: serviceType, emitted: map[string]bool{}}
}

// emitReached sends every not-yet-emitted line the newest deploy row has
// earned. Source errors are swallowed (narration never tears down a tail);
// emit errors propagate (the client is gone).
func (f *progressFollower) emitReached(ctx context.Context, emit func(LogEntry) error) error {
	if f == nil {
		return nil
	}
	rows, err := f.s.DeployProgress(ctx, f.resource, time.Time{})
	if err != nil || len(rows) == 0 {
		return nil
	}
	d := rows[0] // newest — the deploy this tail is following
	if !f.q.keepPod(d.ID) {
		return nil
	}
	for _, e := range progressLines(d, f.repo, f.branch, f.serviceType) {
		// logID is the adapters' stable line identity — reusing it here keeps
		// the follower's dedupe in lockstep with what clients see.
		key := logID(e)
		if f.emitted[key] || !f.q.keep(e) {
			continue
		}
		f.emitted[key] = true
		setLogEntryResource(&e, f.resource)
		if err := emit(e); err != nil {
			return err
		}
	}
	return nil
}
