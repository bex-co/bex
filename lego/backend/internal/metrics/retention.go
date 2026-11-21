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

package metrics

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Per-plan metrics retention (w7/m427). Render keeps metrics for 7 days on
// Hobby, 14 on Pro and 30 on Scale and Enterprise (core.MetricsRetention), so
// no chart read returns a point older than the workspace plan's window. The
// rule lives here, under the two core verbs (Metrics, DatastoreMetrics) and
// filter discovery, so REST, GraphQL and MCP cannot drift.
//
// Billing reads are not charts and are not floored: MonthToDateBandwidth (the
// included-bandwidth figure) and /v1/usage read the whole billing period on
// every plan.

// planFloor is the earliest instant workspaceID's plan keeps metrics from at
// now: the plan's window back from now. Unlike a log floor it remembers no
// earlier plan: Render documents that an upgrade reveals no older logs and
// says nothing of the kind for metrics, so the current plan's window alone
// applies. Zero, with no error, where no plan applies (no store, the default
// tenant): reads then reach as far back as the metrics store does. An
// unreadable plan fails the read, as it does a log read.
func (s *Service) planFloor(ctx context.Context, workspaceID string, now time.Time) (time.Time, error) {
	plan, ok, err := s.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok {
		return time.Time{}, err
	}
	return now.Add(-core.MetricsRetention(plan)), nil
}

// withinRetention applies workspaceID's plan floor to a read of [start, end]
// stepped by step: start moves forward to the first point of the read's own
// grid (start + k×step) at or after the floor. kept is false when no point of
// the grid is left, a range wholly before the floor: the read then answers
// empty, never an error, as a log read does.
//
// The grid is kept, not restarted at the floor, so a clamped read returns
// exactly the points the unclamped one would at the same timestamps: Prometheus
// and Loki evaluate each step on its own, and a grid that moves with the
// caller's plan would shift every bucket (the w4/484 lesson).
//
// A read that starts inside the shortest plan window reaches past no
// workspace's floor and reads no plan at all.
func (s *Service) withinRetention(ctx context.Context, workspaceID string, start, end time.Time, step time.Duration) (clamped time.Time, kept bool, err error) {
	now := s.Now()
	if !start.Before(now.Add(-core.ShortestMetricsRetention())) {
		return start, true, nil
	}
	floor, err := s.planFloor(ctx, workspaceID, now)
	if err != nil || floor.IsZero() || !start.Before(floor) {
		return start, true, err
	}
	if step <= 0 { // defensive: both verbs normalize to a positive step first
		return floor, !floor.After(end), nil
	}
	behind := floor.Sub(start)
	steps := behind / step
	if behind%step != 0 {
		steps++
	}
	clamped = start.Add(steps * step)
	return clamped, !clamped.After(end), nil
}

// RetentionWindow is how far back one resource's metrics reach (w7/m427), as
// the dashboard bounds its range picker by it: the workspace plan, the window
// it keeps (core.MetricsRetention), and each longer window with the cheapest
// plan that has it (core.MetricsRetentionUpgrades; empty on the longest).
type RetentionWindow struct {
	Plan     string
	Window   time.Duration
	Upgrades []core.RetentionUpgrade
}

// MetricsRetention is resource's metrics window, the floor every chart read of
// it applies, for an App, managed Postgres or Key Value the caller may view.
// nil, with no error, where no plan applies: metrics then reach as far back as
// the metrics store keeps them.
func (s *Service) MetricsRetention(ctx context.Context, resource string) (*RetentionWindow, error) {
	var owner client.Object
	var err error
	switch kind, _ := ids.KindOf(resource); kind {
	case ids.Postgres:
		owner, err = s.AuthorizeDatabase(ctx, core.RelCanView, resource)
	case ids.KeyValue:
		owner, err = s.AuthorizeKeyValue(ctx, core.RelCanView, resource)
	default:
		owner, err = s.AuthorizeApp(ctx, core.RelCanView, resource)
	}
	if err != nil {
		return nil, err
	}
	// A datastore whose delete was accepted is absent, as a service is
	// (AuthorizeApp, w8/m35, w8/366).
	if err := core.NotFoundIfDeleting(owner); err != nil {
		return nil, err
	}
	plan, ok, err := s.WorkspacePlanOf(ctx, owner.GetLabels()[core.LabelTenant])
	if err != nil || !ok {
		return nil, err
	}
	return &RetentionWindow{
		Plan:     plan,
		Window:   core.MetricsRetention(plan),
		Upgrades: core.MetricsRetentionUpgrades(plan),
	}, nil
}
