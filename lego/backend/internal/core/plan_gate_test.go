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
	"strings"
	"testing"
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
		name           string
		plan           string
		next, current  []IPAllowListEntry
		emptyDeniesAll bool
		refused        bool
	}{
		{"hobby restricting", "hobby", office, nil, false, true},
		{"pro restricting", "pro", office, nil, false, true},
		{"scale restricting", "scale", office, nil, false, false},
		{"enterprise restricting", "enterprise", office, nil, false, false},
		{"hobby empty service list clears", "hobby", nil, office, false, false},
		{"hobby allow-all", "hobby", DefaultEnvironmentAllowList(), nil, true, false},
		{"hobby unchanged legacy rule", "hobby", office, office, false, false},
		{"hobby edited legacy rule", "hobby", other, office, false, true},
		{"hobby environment deny-all", "hobby", []IPAllowListEntry{}, DefaultEnvironmentAllowList(), true, true},
		{"hobby environment already deny-all", "hobby", []IPAllowListEntry{}, []IPAllowListEntry{}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &Base{Plans: planIs(tc.plan)}
			err := b.RequireInboundIPRules(context.Background(), "tea-a", tc.next, tc.current, tc.emptyDeniesAll)
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
	if err := (&Base{}).RequireInboundIPRules(context.Background(), "tea-a", office, nil, false); err != nil {
		t.Fatalf("no plan reader must leave the feature open: %v", err)
	}
	b := &Base{Plans: planIs("hobby")}
	for _, ws := range []string{"", DefaultTenant} {
		if err := b.RequireInboundIPRules(context.Background(), ws, office, nil, false); err != nil {
			t.Fatalf("workspace %q is unscoped: %v", ws, err)
		}
	}
	failing := &Base{Plans: WorkspacePlanFunc(func(context.Context, string) (string, error) { return "", errors.New("down") })}
	if err := failing.RequireInboundIPRules(context.Background(), "tea-a", office, nil, false); err == nil {
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
		{FeatureWebhooks, "future-plan", true},
	} {
		if got := PlanAllows(tc.plan, tc.feature); got != tc.want {
			t.Errorf("PlanAllows(%q, %q) = %v, want %v", tc.plan, tc.feature, got, tc.want)
		}
	}
	if got := PlanFeatureMessage("hobby", FeatureNetworkIsolation); got != "network-isolated environments require the Pro plan or higher; this workspace is on Hobby" {
		t.Errorf("message = %q", got)
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
