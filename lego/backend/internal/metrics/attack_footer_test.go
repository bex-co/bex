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
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type attackWindowsFake struct {
	windows                []store.EgressAttackWindow
	err                    error
	workspaceID, serviceID string
}

func (f *attackWindowsFake) EgressAttackWindowsOverlapping(_ context.Context, workspaceID, serviceID string, _, _ time.Time) ([]store.EgressAttackWindow, error) {
	f.workspaceID, f.serviceID = workspaceID, serviceID
	return f.windows, f.err
}

// w4/m709 (ADR023 § Bandwidth served to a DDoS attack is not billed): the
// Outbound Bandwidth footer reports the bandwidth that bills, so the hours a
// live attack window covers come off it, read with the same source over the
// window's whole hours.
func TestMonthToDateBandwidthLeavesAttackHoursOut(t *testing.T) {
	const mib = 1 << 20
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelAppID: "srv-web", core.LabelTenant: "tea-a"}
	app.Spec.Host = "web.onbex.co"
	svc := newService(nil, nil, app, ingressFor("web", "web", "web.onbex.co"))
	now := fixedClock()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	attackFrom := monthStart.Add(26 * time.Hour)
	var spans [][2]time.Time
	svc.MonthToDateBandwidthSource = func(_ context.Context, _ string, _ []string, _ bool, since, at time.Time) (BandwidthBytes, []string, error) {
		if since.Equal(monthStart) && at.Equal(now) {
			return BandwidthBytes{HTTP: 10 * mib, NAT: 4 * mib, WebSocket: 2 * mib}, nil, nil
		}
		spans = append(spans, [2]time.Time{since, at})
		return BandwidthBytes{HTTP: 6 * mib, NAT: 1 * mib}, nil, nil
	}
	windows := &attackWindowsFake{windows: []store.EgressAttackWindow{{
		WorkspaceID: "tea-a", StartsAt: attackFrom.Add(10 * time.Minute), EndsAt: attackFrom.Add(2*time.Hour + 5*time.Minute),
	}}}
	svc.AttackWindows = windows

	got, err := svc.MonthToDateBandwidth(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	if windows.workspaceID != "tea-a" || windows.serviceID != "srv-web" {
		t.Errorf("windows read for %s/%s, want tea-a/srv-web", windows.workspaceID, windows.serviceID)
	}
	if len(spans) != 1 || !spans[0][0].Equal(attackFrom) || !spans[0][1].Equal(attackFrom.Add(3*time.Hour)) {
		t.Fatalf("attack spans read = %v, want the three whole hours the window overlaps", spans)
	}
	if got.HTTPEgressBandwidthMB != 4 || got.NATEgressBandwidthMB != 3 || got.WebsocketEgressBandwidthMB != 2 || got.EgressBandwidthMB != 9 {
		t.Errorf("footer = %+v, want 4 + 3 + 2 MB: the attack hours' 6 + 1 MB left out", got)
	}

	// Two overlapping windows take their hours off once.
	windows.windows = append(windows.windows, store.EgressAttackWindow{
		WorkspaceID: "tea-a", StartsAt: attackFrom.Add(time.Hour), EndsAt: attackFrom.Add(2 * time.Hour),
	})
	spans = nil
	if got, err := svc.MonthToDateBandwidth(context.Background(), "web"); err != nil || got.EgressBandwidthMB != 9 || len(spans) != 1 {
		t.Errorf("footer with overlapping windows = %+v, %v (spans %v), want 9 MB from one span", got, err, spans)
	}

	// No window: the footer is what was served.
	windows.windows = nil
	spans = nil
	if got, err := svc.MonthToDateBandwidth(context.Background(), "web"); err != nil || got.EgressBandwidthMB != 16 || len(spans) != 0 {
		t.Errorf("footer without a window = %+v, %v (spans %v), want 16 MB", got, err, spans)
	}
	// A failed window read fails the read rather than show a figure the bill
	// does not carry.
	windows.err = errors.New("db down")
	if _, err := svc.MonthToDateBandwidth(context.Background(), "web"); err == nil {
		t.Error("a failed window read was not reported")
	}
}
