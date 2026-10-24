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
	"errors"
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
