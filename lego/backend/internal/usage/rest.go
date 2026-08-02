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

package usage

import (
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
)

// rest.go is the usage REST fragment. GET /v1/usage returns the calling
// workspace's month-to-date usage in a bex-native JSON envelope. This is a
// deliberate bex extension — Render's public REST API has no usage/billing
// endpoints; see docs/ADR023-usage-metering.md and docs/ADR018-render-parity.md
// § bex ahead of Render.

// usageRow is one (kind, tier, total) entry in the REST/JSON response.
type usageRow struct {
	Kind  string `json:"kind"`
	Tier  string `json:"tier,omitempty"`
	Total int64  `json:"total"`
}

// usageServiceEntry is one service's contribution in the REST/MCP response.
type usageServiceEntry struct {
	ServiceID    string     `json:"serviceId"`
	ServiceName  string     `json:"serviceName,omitempty"`
	ResourceKind string     `json:"resourceKind,omitempty"`
	Rows         []usageRow `json:"rows"`
}

// usageResponse is the shared JSON envelope for GET /v1/usage and the MCP
// get_usage tool — both surfaces answer identical JSON so diffing is trivial.
type usageResponse struct {
	WorkspaceID   string                `json:"workspaceId"`
	Period        string                `json:"period"` // "YYYY-MM"
	Services      []usageServiceEntry   `json:"services"`
	EstimatedCost pricing.EstimatedCost `json:"estimatedCost"`
	// Billing is the real Stripe cost + invoices (m48/m50); omitted (null) when
	// estimate-only — no contract, comped/excluded, billing off, or degraded.
	Billing  *billing.Billing `json:"billing,omitempty"`
	Coverage usageCoverage    `json:"coverage"`
}

type usageCoverage struct {
	State           string   `json:"state"`
	Through         string   `json:"through,omitempty"`
	DegradedSources []string `json:"degradedSources"`
}

// RegisterREST mounts the usage REST endpoint on the shared mux. ownerId
// (Render's workspace-scoping query param, w6/m18) names the workspace to
// query; empty means the caller's default workspace.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/usage", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		now := resolvePeriodEnd(q.Get("period"), s.Now().UTC())
		summary, err := s.monthToDateAt(r.Context(), q.Get("ownerId"), now)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toUsageResponse(summary))
	})
}

// toUsageResponse maps a Summary onto the shared JSON envelope. Services is
// always a non-nil slice so an empty workspace serialises as [] not null.
func toUsageResponse(sum Summary) usageResponse {
	period := sum.Period
	svcs := make([]usageServiceEntry, 0, len(sum.Services))
	for _, svc := range sum.Services {
		rows := make([]usageRow, 0, len(svc.Rows))
		for _, r := range svc.Rows {
			rows = append(rows, usageRow{Kind: r.Kind, Tier: r.Tier, Total: r.Total})
		}
		svcs = append(svcs, usageServiceEntry{ServiceID: svc.ServiceID, ServiceName: svc.ServiceName, ResourceKind: svc.ResourceKind, Rows: rows})
	}
	coverage := usageCoverage{State: sum.Coverage.State, DegradedSources: append([]string(nil), sum.Coverage.DegradedSources...)}
	if coverage.State == "" {
		coverage.State = CoverageUnknown
	}
	if coverage.DegradedSources == nil {
		coverage.DegradedSources = []string{}
	}
	if !sum.Coverage.Through.IsZero() {
		coverage.Through = sum.Coverage.Through.UTC().Format(time.RFC3339)
	}
	return usageResponse{
		WorkspaceID:   sum.WorkspaceID,
		Period:        period,
		Services:      svcs,
		EstimatedCost: sum.EstimatedCost,
		Billing:       sum.Billing,
		Coverage:      coverage,
	}
}

// resolvePeriodEnd maps an optional "YYYY-MM" period string to the effective
// upper-bound for a usage query. For a past month it returns the last nanosecond
// of that month; for the current or future month (or empty string) it returns
// now unchanged.
func resolvePeriodEnd(period string, now time.Time) time.Time {
	if period != "" {
		if end, err := parsePeriod(period); err == nil && end.Before(now) {
			return end
		}
	}
	return now
}

// parsePeriod parses "YYYY-MM" and returns the last nanosecond of that month.
func parsePeriod(period string) (time.Time, error) {
	t, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, err
	}
	return t.AddDate(0, 1, 0).Add(-time.Nanosecond), nil
}
