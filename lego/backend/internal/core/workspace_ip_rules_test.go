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
	"slices"
	"testing"
)

type fakeWorkspaceRules struct {
	rows  map[string][]IPAllowListEntry
	reads int
}

func (f *fakeWorkspaceRules) WorkspaceIPAllowList(_ context.Context, id string) ([]IPAllowListEntry, bool, error) {
	f.reads++
	if id == "tea-broken" {
		return nil, false, errors.New("db down")
	}
	entries, ok := f.rows[id]
	return entries, ok, nil
}

// w9/m388: no row is no layer; a saved list projects its CIDRs; a saved empty
// list projects the deny-all placeholder.
func TestWorkspaceIPLayer(t *testing.T) {
	rules := &fakeWorkspaceRules{rows: map[string][]IPAllowListEntry{
		"tea-office": {{CIDRBlock: "203.0.113.0/24", Description: "office"}},
		"tea-deny":   {},
	}}
	b := &Base{WorkspaceIPRules: rules}
	ctx := context.Background()
	for _, tc := range []struct {
		ws   string
		want []string
	}{
		{"tea-unset", nil},
		{"tea-office", []string{"203.0.113.0/24"}},
		{"tea-deny", []string{DenyAllCIDR}},
		{"", nil},
		{DefaultTenant, nil},
	} {
		got, err := b.WorkspaceIPLayer(ctx, tc.ws)
		if err != nil || !slices.Equal(got, tc.want) || (tc.want == nil) != (got == nil) {
			t.Errorf("WorkspaceIPLayer(%q) = %#v, %v; want %#v", tc.ws, got, err, tc.want)
		}
	}
	if _, err := b.WorkspaceIPLayer(ctx, "tea-broken"); err == nil {
		t.Error("a failed read must fail the create, not create it outside its rules")
	}
	if got, err := (&Base{}).WorkspaceIPLayer(ctx, "tea-office"); got != nil || err != nil {
		t.Errorf("store off = %v, %v; want no layer", got, err)
	}
	// A Blueprint stack reads its workspace's layer once: the memo answers
	// every create in that workspace, and only that workspace.
	memo, err := b.WithWorkspaceIPLayer(ctx, "tea-office")
	if err != nil {
		t.Fatal(err)
	}
	reads := rules.reads
	for range 3 {
		if got, err := b.WorkspaceIPLayer(memo, "tea-office"); err != nil || !slices.Equal(got, []string{"203.0.113.0/24"}) {
			t.Fatalf("memoized layer = %v, %v", got, err)
		}
	}
	if rules.reads != reads {
		t.Fatalf("the memo re-read the rules %d times", rules.reads-reads)
	}
	if got, _ := b.WorkspaceIPLayer(memo, "tea-deny"); !slices.Equal(got, []string{DenyAllCIDR}) || rules.reads != reads+1 {
		t.Fatalf("another workspace answered from the memo: %v", got)
	}
	if _, err := b.WithWorkspaceIPLayer(ctx, "tea-broken"); err == nil {
		t.Fatal("a failed read must fail the stack")
	}
}

// The plan gate: Scale and Enterprise may restrict; below Scale a new
// restriction is the coded refusal, while allow-all and the stored list pass.
func TestRequireWorkspaceInboundIPRules(t *testing.T) {
	plans := map[string]string{"tea-hobby": "hobby", "tea-pro": "pro", "tea-scale": "scale", "tea-ent": "enterprise"}
	b := &Base{Plans: WorkspacePlanFunc(func(_ context.Context, id string) (string, error) { return plans[id], nil })}
	ctx := context.Background()
	office := []IPAllowListEntry{{CIDRBlock: "203.0.113.0/24"}}
	stored := []IPAllowListEntry{{CIDRBlock: "198.51.100.0/24"}}
	for ws := range plans {
		allowed := ws == "tea-scale" || ws == "tea-ent"
		for _, next := range [][]IPAllowListEntry{office, {}} {
			err := b.RequireWorkspaceInboundIPRules(ctx, ws, next, stored)
			var coded *CodedError
			if allowed && err != nil {
				t.Errorf("%s restricting: %v", ws, err)
			}
			if !allowed && (!errors.As(err, &coded) || coded.Code != PlanFeatureRequiredCode || coded.Params["feature"] != "inboundIPRules" || coded.Params["field"] != "ipAllowList") {
				t.Errorf("%s restricting: want the coded plan refusal, got %v", ws, err)
			}
		}
		if err := b.RequireWorkspaceInboundIPRules(ctx, ws, DefaultEnvironmentAllowList(), stored); err != nil {
			t.Errorf("%s allow-all: %v", ws, err)
		}
		if err := b.RequireWorkspaceInboundIPRules(ctx, ws, stored, stored); err != nil {
			t.Errorf("%s re-sending the stored list: %v", ws, err)
		}
	}
}
