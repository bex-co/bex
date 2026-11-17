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

package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func planIs(plan string) WorkspacePlanReader {
	return WorkspacePlanFunc(func(context.Context, string) (string, error) { return plan, nil })
}

// w8/m137: inbound IP rules on services, static sites and environments are a
// Scale/Enterprise feature; allow-all, clearing and an unchanged list never are.
func TestRequireInboundIPRules(t *testing.T) {
	office := []IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}}
	other := []IPAllowListEntry{{CIDRBlock: "198.51.100.0/24"}}
	for _, tc := range []struct {
		name          string
		plan          string
		next, current []IPAllowListEntry
		refused       bool
	}{
		{"hobby restricting", "hobby", office, nil, true},
		{"pro restricting", "pro", office, nil, true},
		{"scale restricting", "scale", office, nil, false},
		{"enterprise restricting", "enterprise", office, nil, false},
		{"hobby allow-all", "hobby", DefaultEnvironmentAllowList(), nil, false},
		{"hobby opens a legacy rule with allow-all", "hobby", UnrestrictedAllowList(), office, false},
		{"hobby unchanged legacy rule", "hobby", office, office, false},
		{"hobby edited legacy rule", "hobby", other, office, true},
		// An empty list denies every source, on an environment and on a
		// service alike (w8/m157): it is the strongest restriction.
		{"hobby empty list blocks", "hobby", []IPAllowListEntry{}, office, true},
		{"hobby deny-all", "hobby", []IPAllowListEntry{}, DefaultEnvironmentAllowList(), true},
		{"hobby already deny-all", "hobby", []IPAllowListEntry{}, []IPAllowListEntry{}, false},
		{"scale empty list blocks", "scale", []IPAllowListEntry{}, office, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &Base{Plans: planIs(tc.plan)}
			err := b.RequireInboundIPRules(context.Background(), "tea-a", tc.next, tc.current)
			if tc.refused {
				if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), InboundIPRulesMessage) {
					t.Fatalf("want the Scale-plan refusal, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected refusal: %v", err)
			}
		})
	}
}

func TestRequireInboundIPRulesDegradesOpen(t *testing.T) {
	office := []IPAllowListEntry{{CIDRBlock: "203.0.113.0/24"}}
	if err := (&Base{}).RequireInboundIPRules(context.Background(), "tea-a", office, nil); err != nil {
		t.Fatalf("no plan reader must leave the feature open: %v", err)
	}
	b := &Base{Plans: planIs("hobby")}
	for _, ws := range []string{"", DefaultTenant} {
		if err := b.RequireInboundIPRules(context.Background(), ws, office, nil); err != nil {
			t.Fatalf("workspace %q is unscoped: %v", ws, err)
		}
	}
	failing := &Base{Plans: WorkspacePlanFunc(func(context.Context, string) (string, error) { return "", errors.New("down") })}
	if err := failing.RequireInboundIPRules(context.Background(), "tea-a", office, nil); err == nil {
		t.Fatal("a plan read failure must not silently open the gate")
	}
}

// w8/m138: the plan × feature table, the Hobby environment cap and the webhook
// destination limits, pinned to Render's features-by-plan page.
func TestPlanFeatureTable(t *testing.T) {
	for _, tc := range []struct {
		feature PlanFeature
		plan    string
		want    bool
	}{
		{FeatureNetworkIsolation, "hobby", false}, {FeatureNetworkIsolation, "pro", true},
		{FeatureWebhooks, "hobby", false}, {FeatureWebhooks, "pro", true}, {FeatureWebhooks, "scale", true},
		{FeatureRequestLogs, "hobby", false}, {FeatureRequestLogs, "pro", true},
		{FeatureAuditLogs, "hobby", false}, {FeatureAuditLogs, "pro", true}, {FeatureAuditLogs, "enterprise", true},
		{FeatureLatencyMetrics, "hobby", false}, {FeatureLatencyMetrics, "pro", true}, {FeatureLatencyMetrics, "scale", true},
		{FeatureInboundIPRules, "hobby", false}, {FeatureInboundIPRules, "pro", false},
		{FeatureInboundIPRules, "scale", true}, {FeatureInboundIPRules, "enterprise", true},
		{FeatureAutoscaling, "hobby", false}, {FeatureAutoscaling, "pro", true}, {FeatureAutoscaling, "enterprise", true},
		// w7/070: render.com/pricing's "Enforce 2FA & Google login" row.
		{FeatureTwoFactorEnforcement, "hobby", false}, {FeatureTwoFactorEnforcement, "pro", true},
		{FeatureTwoFactorEnforcement, "scale", true}, {FeatureTwoFactorEnforcement, "enterprise", true},
		{FeatureWebhooks, "future-plan", true},
	} {
		if got := PlanAllows(tc.plan, tc.feature); got != tc.want {
			t.Errorf("PlanAllows(%q, %q) = %v, want %v", tc.plan, tc.feature, got, tc.want)
		}
	}
	if got := PlanFeatureMessage("hobby", FeatureNetworkIsolation); got != "network-isolated environments require the Pro plan or higher; this workspace is on Hobby" {
		t.Errorf("message = %q", got)
	}
	if got := PlanFeatureMessage("hobby", FeatureAutoscaling); got != "autoscaling requires the Pro plan or higher; this workspace is on Hobby" {
		t.Errorf("message = %q", got)
	}
	if got := PlanFeatureMessage("hobby", FeatureTwoFactorEnforcement); got != "2FA enforcement requires the Pro plan or higher; this workspace is on Hobby" {
		t.Errorf("message = %q", got)
	}
	// w7/m429: render.com/pricing's Instant rollbacks and Custom domains rows.
	for plan, want := range map[string]int{"hobby": 5, "pro": 15, "scale": 30, "enterprise": 30} {
		if got, ok := KeptBuilds(plan); !ok || got != want {
			t.Errorf("KeptBuilds(%q) = %d, %v, want %d", plan, got, ok, want)
		}
	}
	if _, ok := KeptBuilds("future-plan"); ok {
		t.Error("an unknown plan must leave the registry's default retention")
	}
	for plan, want := range map[string]int{"hobby": 2, "pro": 15, "scale": 25} {
		if got, unlimited := IncludedCustomDomains(plan); unlimited || got != want {
			t.Errorf("IncludedCustomDomains(%q) = %d, %v, want %d", plan, got, unlimited, want)
		}
	}
	for _, plan := range []string{"enterprise", "future-plan"} {
		if _, unlimited := IncludedCustomDomains(plan); !unlimited {
			t.Errorf("IncludedCustomDomains(%q) must be unlimited", plan)
		}
	}
	// w7/069: render.com/pricing's Default concurrent sandboxes row. A plan the
	// table does not know, or none, gets Hobby's, never a larger cap.
	for plan, want := range map[string]int{"hobby": 100, "pro": 1000, "scale": 1000, "enterprise": 1000, "": 100, "future-plan": 100} {
		if got := ConcurrentSandboxes(plan); got != want {
			t.Errorf("ConcurrentSandboxes(%q) = %d, want %d", plan, got, want)
		}
	}
	for plan, want := range map[string]int{"hobby": 2, "pro": 0, "scale": 0} {
		if got := MaxEnvironmentsPerProject(plan); got != want {
			t.Errorf("MaxEnvironmentsPerProject(%q) = %d, want %d", plan, got, want)
		}
	}
	for plan, want := range map[string]int{"hobby": 0, "pro": 1, "scale": 100, "enterprise": 100} {
		if got := WebhookDestinationLimit(plan); got != want {
			t.Errorf("WebhookDestinationLimit(%q) = %d, want %d", plan, got, want)
		}
	}
}

func TestRequireNetworkIsolationAndEnvironmentCapacity(t *testing.T) {
	ctx := context.Background()
	hobby := &Base{Plans: planIs("hobby")}
	if err := hobby.RequireNetworkIsolation(ctx, "tea-a", true, false); !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "Pro plan") {
		t.Fatalf("isolation on: want the Pro-plan refusal, got %v", err)
	}
	for _, c := range [][2]bool{{false, false}, {false, true}, {true, true}} {
		if err := hobby.RequireNetworkIsolation(ctx, "tea-a", c[0], c[1]); err != nil {
			t.Errorf("next=%v current=%v must pass: %v", c[0], c[1], err)
		}
	}
	for existing, refused := range map[int]bool{0: false, 1: false, 2: true, 3: true} {
		err := hobby.RequireEnvironmentCapacity(ctx, "tea-a", existing)
		if refused != (err != nil) {
			t.Errorf("hobby with %d environments: err = %v, want refused=%v", existing, err, refused)
		}
	}
	if err := (&Base{Plans: planIs("pro")}).RequireEnvironmentCapacity(ctx, "tea-a", 50); err != nil {
		t.Errorf("pro is unlimited: %v", err)
	}
}

// w8/m138 t013: the environment cap and the feature gates are coded on every
// surface — REST's code/params beside the unchanged message (also through a
// caller's "invalid request body at …" wrap), GraphQL extensions and the MCP
// code prefix — like the webhook gate's WEBHOOK_PLAN_LIMIT.
func TestPlanRefusalsCarryCodes(t *testing.T) {
	ctx := context.Background()
	hobby := &Base{Plans: planIs("hobby")}
	isolation := hobby.RequireNetworkIsolation(ctx, "tea-a", true, false)
	capacity := hobby.RequireEnvironmentCapacity(ctx, "tea-a", 2)
	feature := hobby.RequirePlanFeature(ctx, "tea-a", FeatureWebhooks)
	for _, tc := range []struct {
		err     error
		code    string
		message string
		params  map[string]any
	}{
		{isolation, PlanFeatureRequiredCode, "bad request: networkIsolationEnabled: network-isolated environments require the Pro plan or higher; this workspace is on Hobby",
			map[string]any{"plan": "hobby", "feature": "networkIsolation", "requiredPlan": "pro", "field": "networkIsolationEnabled"}},
		{fmt.Errorf("invalid request body at /environments: %w", capacity), PlanLimitCode,
			"invalid request body at /environments: bad request: environments: a Hobby workspace is limited to 2 environments per project; upgrade the plan to add more",
			map[string]any{"plan": "hobby", "limit": float64(2), "field": "environments"}},
		{feature, PlanFeatureRequiredCode, "bad request: webhooks require the Pro plan or higher; this workspace is on Hobby",
			map[string]any{"plan": "hobby", "feature": "webhooks", "requiredPlan": "pro"}},
	} {
		rec := httptest.NewRecorder()
		WriteErr(rec, tc.err)
		var body struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Params  map[string]any `json:"params"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusBadRequest || body.Code != tc.code || body.Message != tc.message || !reflect.DeepEqual(body.Params, tc.params) {
			t.Errorf("REST %d %+v, want 400 %s %q %v", rec.Code, body, tc.code, tc.message, tc.params)
		}
		var coded *CodedError
		if !errors.As(tc.err, &coded) || coded.Extensions()["code"] != tc.code || coded.Extensions()["plan"] != "hobby" {
			t.Errorf("GraphQL extensions of %v lack %s", tc.err, tc.code)
		}
		if got := MCPError(tc.err).Error(); got != tc.code+": "+tc.message {
			t.Errorf("MCP = %q", got)
		}
	}
}

// w9/m389: each plan's log window is Render's (render.com/docs/logging), an
// unknown plan gets the shortest, and the upgrade hint names the cheapest plan
// that keeps logs longer.
func TestLogRetentionFollowsThePlan(t *testing.T) {
	const day = 24 * time.Hour
	if ShortestLogRetention() != 7*day {
		t.Fatalf("ShortestLogRetention = %s, want Hobby's 7 days", ShortestLogRetention())
	}
	for _, tc := range []struct {
		plan, next     string
		window, nextIn time.Duration
	}{
		{"hobby", "pro", 7 * day, 14 * day},
		{"pro", "scale", 14 * day, 30 * day},
		{"scale", "", 30 * day, 0},
		{"enterprise", "", 30 * day, 0},
		{"platinum", "pro", 7 * day, 14 * day},
		{"", "pro", 7 * day, 14 * day},
	} {
		if got := LogRetention(tc.plan); got != tc.window {
			t.Errorf("LogRetention(%q) = %s, want %s", tc.plan, got, tc.window)
		}
		next, window, ok := LogRetentionUpgrade(tc.plan)
		if next != tc.next || window != tc.nextIn || ok != (tc.next != "") {
			t.Errorf("LogRetentionUpgrade(%q) = %q, %s, %v; want %q, %s", tc.plan, next, window, ok, tc.next, tc.nextIn)
		}
	}
}

// w7/m427: each plan's metrics window is Render's (render.com/docs/service-metrics
// § Metrics retention period), an unknown plan gets the shortest, and the
// upgrade hint names each longer window with the cheapest plan that has it.
func TestMetricsRetentionFollowsThePlan(t *testing.T) {
	const day = 24 * time.Hour
	if ShortestMetricsRetention() != 7*day {
		t.Fatalf("ShortestMetricsRetention = %s, want Hobby's 7 days", ShortestMetricsRetention())
	}
	pro, scale := RetentionUpgrade{"pro", 14 * day}, RetentionUpgrade{"scale", 30 * day}
	for _, tc := range []struct {
		plan     string
		window   time.Duration
		upgrades []RetentionUpgrade
	}{
		{"hobby", 7 * day, []RetentionUpgrade{pro, scale}},
		{"pro", 14 * day, []RetentionUpgrade{scale}},
		// Enterprise keeps what Scale does, so it is never named as an upgrade.
		{"scale", 30 * day, nil},
		{"enterprise", 30 * day, nil},
		{"platinum", 7 * day, []RetentionUpgrade{pro, scale}},
		{"", 7 * day, []RetentionUpgrade{pro, scale}},
	} {
		if got := MetricsRetention(tc.plan); got != tc.window {
			t.Errorf("MetricsRetention(%q) = %s, want %s", tc.plan, got, tc.window)
		}
		if got := MetricsRetentionUpgrades(tc.plan); !slices.Equal(got, tc.upgrades) {
			t.Errorf("MetricsRetentionUpgrades(%q) = %v, want %v", tc.plan, got, tc.upgrades)
		}
	}
}

// w9/m389 t004: an upgrade keeps the old plan's floor until the new window
// overtakes it; a downgrade narrows at once.
func TestLogFloorAcrossPlanChanges(t *testing.T) {
	const day = 24 * time.Hour
	upgradeAt := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	// Hobby -> Pro at T: nothing older than T-7d until T+7d, then Pro's 14 days.
	kept := LogsVisibleFromAfterPlanChange("hobby", "pro", time.Time{}, upgradeAt)
	if !kept.Equal(upgradeAt.Add(-7 * day)) {
		t.Fatalf("hobby->pro keeps %s, want T-7d", kept)
	}
	for _, tc := range []struct {
		after time.Duration
		want  time.Time
	}{
		{0, upgradeAt.Add(-7 * day)},
		{3 * day, upgradeAt.Add(-7 * day)},
		{7 * day, upgradeAt.Add(-7 * day)},
		{10 * day, upgradeAt.Add(10*day - 14*day)},
	} {
		if got := LogFloor("pro", kept, upgradeAt.Add(tc.after)); !got.Equal(tc.want) {
			t.Errorf("pro floor at T+%s = %s, want %s", tc.after, got, tc.want)
		}
	}

	// Pro -> Scale three days after the Hobby upgrade keeps the same floor:
	// lines that aged out under Hobby stay hidden under Scale too.
	at := upgradeAt.Add(3 * day)
	if got := LogsVisibleFromAfterPlanChange("pro", "scale", kept, at); !got.Equal(kept) {
		t.Errorf("pro->scale keeps %s, want the hobby floor %s", got, kept)
	}
	// A plain Pro -> Scale keeps Pro's floor at that moment.
	if got := LogsVisibleFromAfterPlanChange("pro", "scale", time.Time{}, at); !got.Equal(at.Add(-14 * day)) {
		t.Errorf("pro->scale keeps %s, want T-14d", got)
	}

	// A downgrade narrows at once and keeps nothing the shorter window
	// already covers.
	if got := LogsVisibleFromAfterPlanChange("scale", "hobby", time.Time{}, at); !got.IsZero() {
		t.Errorf("scale->hobby keeps %s, want nothing", got)
	}
	if got := LogFloor("hobby", time.Time{}, at); !got.Equal(at.Add(-7 * day)) {
		t.Errorf("hobby floor after the downgrade = %s, want now-7d", got)
	}
	// Scale -> Pro while a Hobby upgrade's floor still bites keeps it.
	if got := LogsVisibleFromAfterPlanChange("scale", "pro", kept, at); !got.Equal(kept) {
		t.Errorf("scale->pro keeps %s, want the still-binding %s", got, kept)
	}
	// Equal windows (Scale <-> Enterprise) keep only a binding floor.
	if got := LogsVisibleFromAfterPlanChange("scale", "enterprise", time.Time{}, at); !got.IsZero() {
		t.Errorf("scale->enterprise keeps %s, want nothing", got)
	}
	// A kept floor that has lapsed is dropped at the next change.
	if got := LogsVisibleFromAfterPlanChange("pro", "scale", kept, upgradeAt.Add(20*day)); !got.Equal(upgradeAt.Add(20*day - 14*day)) {
		t.Errorf("pro->scale after the hobby floor lapsed keeps %s, want Pro's own floor", got)
	}
	// An unknown old plan is the strictest: an upgrade from it keeps 7 days.
	if got := LogsVisibleFromAfterPlanChange("platinum", "scale", time.Time{}, at); !got.Equal(at.Add(-7 * day)) {
		t.Errorf("unknown->scale keeps %s, want T-7d", got)
	}
}

// w7/m453: each plan's point-in-time recovery window is Render's
// (render.com/docs/postgresql-backups: 3 days on Hobby, 7 on Pro or higher),
// an unknown plan gets the shortest, the floor is the window back from now or
// a kept floor while that is later, and the hint names Pro from Hobby only.
func TestPITRRetentionFollowsThePlan(t *testing.T) {
	const day = 24 * time.Hour
	if LongestPITRRetention() != 7*day {
		t.Fatalf("LongestPITRRetention = %s, want Pro's 7 days", LongestPITRRetention())
	}
	for _, tc := range []struct {
		plan, next     string
		window, nextIn time.Duration
	}{
		{"hobby", "pro", 3 * day, 7 * day},
		{"pro", "", 7 * day, 0},
		{"scale", "", 7 * day, 0},
		{"enterprise", "", 7 * day, 0},
		{"platinum", "pro", 3 * day, 7 * day},
		{"", "pro", 3 * day, 7 * day},
	} {
		if got := PITRRetention(tc.plan); got != tc.window {
			t.Errorf("PITRRetention(%q) = %s, want %s", tc.plan, got, tc.window)
		}
		next, window, ok := PITRRetentionUpgrade(tc.plan)
		if next != tc.next || window != tc.nextIn || ok != (tc.next != "") {
			t.Errorf("PITRRetentionUpgrade(%q) = %q, %s, %v; want %q, %s", tc.plan, next, window, ok, tc.next, tc.nextIn)
		}
	}

	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	if got := PITRFloor("hobby", time.Time{}, now); !got.Equal(now.Add(-3 * day)) {
		t.Errorf("hobby floor = %s, want now-3d", got)
	}
	if got := PITRFloor("pro", time.Time{}, now); !got.Equal(now.Add(-7 * day)) {
		t.Errorf("pro floor = %s, want now-7d", got)
	}
	kept := now.Add(-5 * day)
	if got := PITRFloor("pro", kept, now); !got.Equal(kept) {
		t.Errorf("pro floor with a later kept floor = %s, want the kept %s", got, kept)
	}
	if got := PITRFloor("pro", now.Add(-9*day), now); !got.Equal(now.Add(-7 * day)) {
		t.Errorf("pro floor with a lapsed kept floor = %s, want now-7d", got)
	}
}

// w7/m453: upgrading from Hobby does not backfill the recovery window. A
// Hobby -> Pro change at T keeps T-3d, which lapses once Pro's own floor
// passes it four days later; a downgrade keeps nothing.
func TestPITRFloorAcrossPlanChanges(t *testing.T) {
	const day = 24 * time.Hour
	upgradeAt := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

	kept := PITRVisibleFromAfterPlanChange("hobby", "pro", time.Time{}, upgradeAt)
	if !kept.Equal(upgradeAt.Add(-3 * day)) {
		t.Fatalf("hobby->pro keeps %s, want T-3d", kept)
	}
	for _, tc := range []struct {
		after time.Duration
		want  time.Time
	}{
		{0, upgradeAt.Add(-3 * day)},
		{2 * day, upgradeAt.Add(-3 * day)},
		{4 * day, upgradeAt.Add(-3 * day)},
		{4*day + time.Hour, upgradeAt.Add(4*day + time.Hour - 7*day)},
		{10 * day, upgradeAt.Add(10*day - 7*day)},
	} {
		if got := PITRFloor("pro", kept, upgradeAt.Add(tc.after)); !got.Equal(tc.want) {
			t.Errorf("pro floor at T+%s = %s, want %s", tc.after, got, tc.want)
		}
	}

	at := upgradeAt.Add(2 * day)
	// Pro -> Scale while the Hobby floor still bites keeps it: Scale's window
	// is Pro's, so it never reaches further back.
	if got := PITRVisibleFromAfterPlanChange("pro", "scale", kept, at); !got.Equal(kept) {
		t.Errorf("pro->scale keeps %s, want the hobby floor %s", got, kept)
	}
	// Pro -> Hobby narrows at once to Hobby's three days.
	if got := PITRVisibleFromAfterPlanChange("pro", "hobby", time.Time{}, at); !got.IsZero() {
		t.Errorf("pro->hobby keeps %s, want nothing", got)
	}
	if got := PITRVisibleFromAfterPlanChange("pro", "hobby", kept, at); !got.IsZero() {
		t.Errorf("pro->hobby over a kept floor keeps %s, want nothing: Hobby's own floor is later", got)
	}
	// Equal windows keep no floor of their own.
	if got := PITRVisibleFromAfterPlanChange("pro", "enterprise", time.Time{}, at); !got.IsZero() {
		t.Errorf("pro->enterprise keeps %s, want nothing", got)
	}
	// The PITR floor is its own: a Hobby -> Pro change keeps T-3d for
	// recovery and T-7d for logs.
	if logs := LogsVisibleFromAfterPlanChange("hobby", "pro", time.Time{}, upgradeAt); logs.Equal(kept) {
		t.Errorf("the log floor %s equals the PITR floor; they must follow their own tables", logs)
	}
}

// pitrPlanIs is the tenants store's PITR shape, counting its reads.
type pitrPlanIs struct {
	plan        string
	visibleFrom time.Time
	reads       *int
}

func (p pitrPlanIs) WorkspacePlan(context.Context, string) (string, error) {
	*p.reads++
	return p.plan, nil
}

func (p pitrPlanIs) WorkspacePITRPlan(context.Context, string) (string, time.Time, error) {
	*p.reads++
	return p.plan, p.visibleFrom, nil
}

// WorkspacePITRPlanOf reads the plan and the kept recovery floor in one read
// where the reader offers both, the plan alone from a plain reader, nothing
// where no plan applies, and a read failure is an error.
func TestWorkspacePITRPlanOf(t *testing.T) {
	ctx := context.Background()
	if plan, from, ok, err := (&Base{}).WorkspacePITRPlanOf(ctx, "tea-a"); err != nil || ok || plan != "" || !from.IsZero() {
		t.Fatalf("no plan reader = %q, %s, %v, %v; want no plan", plan, from, ok, err)
	}
	if plan, from, ok, err := (&Base{Plans: planIs("hobby")}).WorkspacePITRPlanOf(ctx, "tea-a"); err != nil || !ok || plan != "hobby" || !from.IsZero() {
		t.Fatalf("plain reader = %q, %s, %v, %v; want hobby with no kept floor", plan, from, ok, err)
	}
	kept := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	reads := 0
	b := &Base{Plans: pitrPlanIs{"pro", kept, &reads}}
	if plan, from, ok, err := b.WorkspacePITRPlanOf(ctx, "tea-a"); err != nil || !ok || plan != "pro" || !from.Equal(kept) || reads != 1 {
		t.Fatalf("store reader = %q, %s, %v, %v after %d reads; want pro and the kept floor in one read", plan, from, ok, err, reads)
	}
	// A reader keeping only the PITR floor answers the log read with the plan
	// alone: the two floors never stand in for each other.
	if plan, from, ok, err := b.WorkspaceLogPlanOf(ctx, "tea-a"); err != nil || !ok || plan != "pro" || !from.IsZero() {
		t.Fatalf("log read through a PITR reader = %q, %s, %v, %v; want pro with no log floor", plan, from, ok, err)
	}
	if _, _, ok, err := b.WorkspacePITRPlanOf(ctx, DefaultTenant); err != nil || ok {
		t.Fatalf("default tenant = %v, %v; want no plan", ok, err)
	}
	b.Plans = WorkspacePlanFunc(func(context.Context, string) (string, error) { return "", errors.New("store down") })
	if _, _, _, err := b.WorkspacePITRPlanOf(ctx, "tea-a"); err == nil {
		t.Fatal("a failed plan read must fail the read")
	}
}

// logPlanIs is the tenants store's shape: the plan and the kept floor in one
// read, counting its reads.
type logPlanIs struct {
	plan        string
	visibleFrom time.Time
	reads       *int
}

func (l logPlanIs) WorkspacePlan(context.Context, string) (string, error) {
	*l.reads++
	return l.plan, nil
}

func (l logPlanIs) WorkspaceLogPlan(context.Context, string) (string, time.Time, error) {
	*l.reads++
	return l.plan, l.visibleFrom, nil
}

// WorkspaceLogPlanOf reads the plan and the kept floor in one read where the
// reader offers both, the plan alone from a plain reader, nothing where no
// plan applies (the store-off degrade), and a read failure is an error.
func TestWorkspaceLogPlanOf(t *testing.T) {
	ctx := context.Background()
	if plan, from, ok, err := (&Base{}).WorkspaceLogPlanOf(ctx, "tea-a"); err != nil || ok || plan != "" || !from.IsZero() {
		t.Fatalf("no plan reader = %q, %s, %v, %v; want no plan", plan, from, ok, err)
	}
	if plan, from, ok, err := (&Base{Plans: planIs("pro")}).WorkspaceLogPlanOf(ctx, "tea-a"); err != nil || !ok || plan != "pro" || !from.IsZero() {
		t.Fatalf("plain reader = %q, %s, %v, %v; want pro with no kept floor", plan, from, ok, err)
	}
	kept := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	reads := 0
	b := &Base{Plans: logPlanIs{"pro", kept, &reads}}
	if plan, from, ok, err := b.WorkspaceLogPlanOf(ctx, "tea-a"); err != nil || !ok || plan != "pro" || !from.Equal(kept) || reads != 1 {
		t.Fatalf("store reader = %q, %s, %v, %v after %d reads; want pro and the kept floor in one read", plan, from, ok, err, reads)
	}
	if _, _, ok, err := b.WorkspaceLogPlanOf(ctx, DefaultTenant); err != nil || ok || reads != 1 {
		t.Fatalf("default tenant = %v, %v after %d reads; want no plan and no read", ok, err, reads)
	}
	b.Plans = WorkspacePlanFunc(func(context.Context, string) (string, error) { return "", errors.New("store down") })
	if _, _, _, err := b.WorkspaceLogPlanOf(ctx, "tea-a"); err == nil {
		t.Fatal("a failed plan read must fail the read")
	}
}

// w9/098: Render's audit history starts at the upgrade to Pro. A change into
// a plan with audit logs from one without starts it at that moment; every
// other change keeps what was there, so a downgrade then an upgrade moves it
// forward and a workspace that was always on Pro has none.
func TestAuditVisibleFromAfterPlanChange(t *testing.T) {
	first := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		old, next string
		kept      time.Time
		want      time.Time
	}{
		{"hobby", "pro", time.Time{}, at},
		{"hobby", "scale", time.Time{}, at},
		{"hobby", "enterprise", first, at},
		{"pro", "scale", time.Time{}, time.Time{}},
		{"pro", "scale", first, first},
		{"scale", "hobby", first, first},
		{"pro", "hobby", time.Time{}, time.Time{}},
		{"hobby", "hobby", first, first},
	} {
		if got := AuditVisibleFromAfterPlanChange(tc.old, tc.next, tc.kept, at); !got.Equal(tc.want) {
			t.Errorf("%s->%s keeping %s = %s, want %s", tc.old, tc.next, tc.kept, got, tc.want)
		}
	}
}
