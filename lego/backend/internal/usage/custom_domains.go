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
	"context"
	"log"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// custom_domains.go is the custom domain meter (w7/m429, docs/ADR030 §16): a
// workspace's plan includes some custom domains (core.IncludedCustomDomains)
// and each verified domain above them is metered by the second, as Render
// charges for them. A domain is never refused for the count.

// CustomDomains is a workspace's verified custom domains now against the
// count its plan includes (w7/m429): the Billing page's "N of M included".
// Unlimited is true on Enterprise, whose domains are all included.
type CustomDomains struct {
	Used      int  `json:"used"`
	Included  int  `json:"included"`
	Unlimited bool `json:"unlimited"`
}

// includedCustomDomainsFor is how many custom domains workspaceID's plan
// includes, and whether any domain of it is metered. As for every plan
// allowance (pricing.IncludedForWorkspace), no plan applies to the default
// tenant, so it is not metered.
func includedCustomDomainsFor(workspaceID, plan string) (included int, metered bool) {
	if workspaceID == core.DefaultTenant {
		return 0, false
	}
	n, unlimited := core.IncludedCustomDomains(plan)
	return n, !unlimited
}

// domainSecondsAbove is how many domain-seconds of [from, to) the domains
// verified at verifiedAt spent above an included count of included. At any
// moment the domains above the count are the most recently verified ones, so
// the k-th domain to be verified, k > included, is charged from its
// verification (or from, if earlier) to `to`.
func domainSecondsAbove(verifiedAt []time.Time, from, to time.Time, included int) int64 {
	starts := slices.SortedFunc(slices.Values(verifiedAt), func(a, b time.Time) int { return a.Compare(b) })
	var seconds int64
	for k, verified := range starts {
		if k < included || !verified.Before(to) {
			continue
		}
		start := verified
		if start.Before(from) {
			start = from
		}
		seconds += int64(to.Sub(start) / time.Second)
	}
	return seconds
}

// catchUpCustomDomainsThrough meters every closed hour window from the custom
// domain meter's cursor through `through`. Like the disk meter it reads
// control-plane rows, so it runs without Prometheus.
//
// Unlike disks, the domains table holds only the present: a deleted domain's
// row is gone and a plan keeps no history. So a window is metered once, as soon
// as it closes, and its own cursor records that (usage_meter_cursors) because
// a workspace within its included count writes no row the newest-row cursor
// could find. A window missed while bex-api was down is metered on restart
// from the domains present then (bounded by catchupLimit): a domain deleted
// in between goes unbilled for those hours, never billed twice. A meter with
// no cursor yet starts at the newest closed hour, so the charge never reaches
// back before the meter first ran.
func (s *Service) catchUpCustomDomainsThrough(ctx context.Context, through time.Time) {
	if s.Store == nil {
		return
	}
	through = through.UTC().Truncate(time.Hour)
	window := through
	cursor, ok, err := s.Store.UsageMeterCursor(ctx, store.UsageKindCustomDomainSeconds)
	if err != nil {
		log.Printf("usage: custom domain meter cursor: %v", err)
		return
	}
	if ok {
		window = cursor.UTC().Truncate(time.Hour).Add(time.Hour)
	}
	if floor := through.Add(-catchupLimit); window.Before(floor) {
		window = floor
	}
	for ; !window.After(through); window = window.Add(time.Hour) {
		if err := s.meterCustomDomainWindow(ctx, window); err != nil {
			// Stop at the first failure so the gap is retried oldest-first.
			log.Printf("usage: custom domain meter window %s: %v", window.Format(time.RFC3339), err)
			return
		}
		if err := s.Store.AdvanceUsageMeterCursor(ctx, store.UsageKindCustomDomainSeconds, window); err != nil {
			log.Printf("usage: custom domain meter cursor %s: %v", window.Format(time.RFC3339), err)
			return
		}
	}
}

// meterCustomDomainWindow writes one row per workspace whose verified custom
// domains were above its plan's included count during [window, window+1h).
func (s *Service) meterCustomDomainWindow(ctx context.Context, window time.Time) error {
	end := window.Add(time.Hour)
	workspaces, err := s.Store.CountedCustomDomains(ctx, end, "")
	if err != nil {
		return err
	}
	for _, ws := range workspaces {
		included, metered := includedCustomDomainsFor(ws.WorkspaceID, ws.Plan)
		if !metered {
			continue
		}
		seconds := domainSecondsAbove(ws.VerifiedAt, window, end, included)
		if seconds <= 0 {
			continue
		}
		if err := s.Store.UpsertUsageHourly(ctx, store.HourlyRow{
			WorkspaceID:  ws.WorkspaceID,
			ResourceKind: store.ResourceKindCustomDomains,
			ServiceID:    ws.WorkspaceID,
			Kind:         store.UsageKindCustomDomainSeconds,
			WindowStart:  window,
			Quantity:     seconds,
		}); err != nil {
			return err
		}
	}
	return nil
}

// customDomains is workspaceID's verified custom domains now against its
// plan's included count; nil where no plan applies.
func (s *Service) customDomains(ctx context.Context, workspaceID string, now time.Time) (*CustomDomains, error) {
	plan, ok, err := s.Base.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok {
		return nil, err
	}
	workspaces, err := s.Store.CountedCustomDomains(ctx, now, workspaceID)
	if err != nil {
		return nil, err
	}
	out := &CustomDomains{}
	out.Included, out.Unlimited = core.IncludedCustomDomains(plan)
	// CountedCustomDomains read workspaceID's rows only.
	for _, ws := range workspaces {
		out.Used += len(ws.VerifiedAt)
	}
	return out, nil
}
