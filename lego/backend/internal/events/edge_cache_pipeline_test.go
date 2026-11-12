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

package events

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w9/m345: the edge-cache verbs and a build refused for exhausted pipeline
// minutes are Render's edge_cache_enabled / _disabled / _purged and
// pipeline_minutes_exhausted, with Render's details on REST (and so MCP, which
// renders the same object).

func cacheRow(verb string, enabledTo *bool) store.ServiceEventRow {
	return store.ServiceEventRow{
		Key: "aud-1:", At: now, Source: store.EventSourceAudit, Verb: verb, Caller: "user-x",
		AuditChanges: store.AuditChanges{EdgeCacheEnabledTo: enabledTo},
	}
}

func restDetails(t *testing.T, e Event) string {
	t.Helper()
	raw, err := json.Marshal(toRenderEvent(e).Details)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestEdgeCacheEventsCarryRendersTrigger(t *testing.T) {
	const trigger = `{"trigger":{"manual":true,"user":{"id":"user-x"},"system":false}}`
	for _, tc := range []struct {
		name     string
		row      store.ServiceEventRow
		wantType string
	}{
		{"a save that turned caching on", cacheRow(core.AuditVerbSetCacheProfile, new(true)), TypeEdgeCacheEnabled},
		{"a save that turned caching off", cacheRow(core.AuditVerbSetCacheProfile, new(false)), TypeEdgeCacheDisabled},
		{"a purge", cacheRow(core.AuditVerbPurgeCache, nil), TypeEdgeCachePurged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev := view(tc.row, "web")
			if ev.Type != tc.wantType {
				t.Fatalf("type = %q, want %q", ev.Type, tc.wantType)
			}
			if got := restDetails(t, ev); got != trigger {
				t.Fatalf("REST details = %s, want Render's %s", got, trigger)
			}
		})
	}
	// A save that kept caching on (or off) is no event.
	if ev := view(cacheRow(core.AuditVerbSetCacheProfile, nil), "web"); ev.Type != "" {
		t.Fatalf("a save that left caching as it was is %q, want no event", ev.Type)
	}
}

// The trigger's user is resolved like every other event user: email first, by
// subject, then the usr- id.
func TestEdgeCacheTriggerUserIsResolved(t *testing.T) {
	st := &fakeStore{rows: []store.ServiceEventRow{cacheRow(core.AuditVerbPurgeCache, nil)}}
	svc := newService(st, sampleApp("web", "srv-1", "tea-a"))
	svc.Identities = emailLookup{"user-x": "x@example.com"}
	events, err := svc.List(context.Background(), "web", Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if got := events[0].Details.EdgeCacheTrigger; got == nil || got.User == nil || got.User.Email != "x@example.com" {
		t.Fatalf("trigger = %+v, want the caller with their email", got)
	}
}

type emailLookup map[string]string

func (l emailLookup) LookupEmails(_ context.Context, subjects []string) (map[string]string, error) {
	out := map[string]string{}
	for _, s := range subjects {
		if e, ok := l[s]; ok {
			out[s] = e
		}
	}
	return out, nil
}

// Get by id of a profile save that left caching as it was answers 404, as
// List never returns it.
func TestGetOfAProfileSaveThatChangedNothingIsNotFound(t *testing.T) {
	st := &fakeStore{lookup: store.ServiceEventLookup{Event: cacheRow(core.AuditVerbSetCacheProfile, nil), ServiceID: "web"}}
	_, err := newService(st).Get(context.Background(), ids.Derive(ids.Event, "aud-1:"))
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != EventNotFoundCode {
		t.Fatalf("Get = %v, want %s", err, EventNotFoundCode)
	}
}

func TestPipelineMinutesExhaustedNamesTheRefusedBuildAndItsTrigger(t *testing.T) {
	row := store.ServiceEventRow{
		Key: "fact:deploy:dep-9:pipeline_minutes_exhausted", At: now, Source: store.EventSourceFact,
		FactType: TypePipelineMinutesExhausted, DeployID: "dep-9", Trigger: store.TriggerManual, Caller: "user-x",
	}
	ev := view(row, "web")
	if ev.Type != TypePipelineMinutesExhausted {
		t.Fatalf("type = %q", ev.Type)
	}
	var got struct {
		BuildID string `json:"buildId"`
		Trigger struct {
			FirstBuild, EnvUpdated, Manual, DeployedByRender, ClearCache, Rollback *bool
			User                                                                   *struct{ ID string }
		} `json:"trigger"`
	}
	if err := json.Unmarshal([]byte(restDetails(t, ev)), &got); err != nil {
		t.Fatal(err)
	}
	tr := got.Trigger
	if got.BuildID != "dep-9" {
		t.Errorf("buildId = %q, want the refused deploy dep-9", got.BuildID)
	}
	for name, flag := range map[string]*bool{"firstBuild": tr.FirstBuild, "envUpdated": tr.EnvUpdated, "deployedByRender": tr.DeployedByRender, "clearCache": tr.ClearCache, "rollback": tr.Rollback} {
		if flag == nil || *flag {
			t.Errorf("trigger.%s = %v, want a present false (Render requires it)", name, flag)
		}
	}
	if tr.Manual == nil || !*tr.Manual || tr.User == nil || tr.User.ID != "user-x" {
		t.Errorf("trigger = manual %v user %+v, want the manual deploy's user-x", tr.Manual, tr.User)
	}
}

func TestEdgeCacheAndPipelineTypeFiltersArePushedDown(t *testing.T) {
	for _, tc := range []struct {
		name      string
		types     []string
		wantVerbs []string
		wantFacts []string
		wantCache store.EdgeCacheFilter
	}{
		{"enabled", []string{TypeEdgeCacheEnabled}, []string{core.AuditVerbSetCacheProfile}, nil, store.EdgeCacheFilterEnabled},
		{"disabled", []string{TypeEdgeCacheDisabled}, []string{core.AuditVerbSetCacheProfile}, nil, store.EdgeCacheFilterDisabled},
		{"both share the verb once", []string{TypeEdgeCacheEnabled, TypeEdgeCacheDisabled}, []string{core.AuditVerbSetCacheProfile}, nil, store.EdgeCacheFilterEnabled | store.EdgeCacheFilterDisabled},
		{"purged", []string{TypeEdgeCachePurged}, []string{core.AuditVerbPurgeCache}, nil, store.EdgeCacheFilterNone},
		{"pipeline minutes", []string{TypePipelineMinutesExhausted}, nil, []string{TypePipelineMinutesExhausted}, store.EdgeCacheFilterNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &fakeStore{}
			svc := newService(st, sampleApp("web", "srv-1", "tea-a"))
			if _, err := svc.List(context.Background(), "web", Filter{Types: tc.types}); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(st.got.Verbs, tc.wantVerbs) || !slices.Equal(st.got.FactTypes, tc.wantFacts) || st.got.EdgeCache != tc.wantCache {
				t.Errorf("pushed down verbs=%v facts=%v edgeCache=%v, want %v %v %v",
					st.got.Verbs, st.got.FactTypes, st.got.EdgeCache, tc.wantVerbs, tc.wantFacts, tc.wantCache)
			}
		})
	}
	st := &fakeStore{}
	if _, err := newService(st, sampleApp("web", "srv-1", "tea-a")).List(context.Background(), "web", Filter{}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(st.got.Verbs, core.AuditVerbSetCacheProfile) || !slices.Contains(st.got.Verbs, core.AuditVerbPurgeCache) ||
		!slices.Contains(st.got.FactTypes, TypePipelineMinutesExhausted) || st.got.EdgeCache != store.EdgeCacheFilterNone {
		t.Errorf("the unfiltered feed must ask for both cache verbs and the pipeline fact: %+v", st.got)
	}
}
