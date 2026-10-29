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
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/m138: HTTP request logs are a Pro-and-higher feature. On Hobby an explicit
// type=request is refused with the plan error; an all-types query keeps
// answering with the app lines.
func TestRequestLogsFollowThePlan(t *testing.T) {
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelTenant: "tea-a"}
	lines := map[string][]string{webInst: {"2026-07-05T00:00:01Z hello"}}

	for plan, refused := range map[string]bool{"hobby": true, "pro": false} {
		t.Run(plan, func(t *testing.T) {
			svc := newService(lines, app, podFor("web", webInst))
			svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return plan, nil })

			_, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeRequest}})
			if refused {
				if !errors.Is(err, core.ErrBadRequest) || err.Error() != "bad request: type: HTTP request logs require the Pro plan or higher; this workspace is on Hobby" {
					t.Fatalf("hobby: want the Pro-plan refusal, got %v", err)
				}
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode ||
					coded.Params["plan"] != "hobby" || coded.Params["feature"] != "requestLogs" || coded.Params["requiredPlan"] != "pro" || coded.Params["field"] != "type" {
					t.Fatalf("hobby: want PLAN_FEATURE_REQUIRED params, got %#v", coded)
				}
			} else if errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("pro: request logs must not be plan-refused: %v", err)
			}

			entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web"})
			if err != nil || len(entries) == 0 {
				t.Fatalf("%s: app lines must keep answering: %v (%d lines)", plan, err, len(entries))
			}
		})
	}
}

// w8/m138 t012: on Hobby an untyped (or type=all) store read must not select
// request streams, and label discovery must not offer request-only values.
// Before the fix lokiTypeMatcher returned the app+request union for an empty
// Types before consulting noRequest. Pro keeps the union byte-for-byte.
func TestHobbyAllTypesReadExcludesRequestStreams(t *testing.T) {
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelTenant: "tea-a"}
	allTypes, err := NormalizeTypes([]string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		matcher    string
		types      []string
		statusCode []string
		method     []string
	}{
		"hobby": {matcher: `container="app"`, types: []string{"app", "build"}, statusCode: []string{}, method: []string{}},
		"pro":   {matcher: `container=~"app|"`, types: []string{"app", "build", "request"}, statusCode: []string{"200", "503"}, method: []string{"GET"}},
	}
	for plan, want := range cases {
		t.Run(plan, func(t *testing.T) {
			svc := newService(nil, app, podFor("web", webInst))
			svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return plan, nil })
			var selector string
			svc.History = func(_ context.Context, ns string, q LogQuery) ([]LogEntry, error) {
				selector = lokiSelectorFor(ns, q)
				return nil, nil
			}
			svc.LabelValues = func(_ context.Context, _ string, label string, _ LogQuery) ([]string, error) {
				return map[string][]string{
					LabelType:       {"request", "app", "build"},
					LabelStatusCode: {"503", "200"},
					LabelMethod:     {"GET"},
				}[label], nil
			}
			ctx := context.Background()
			for _, q := range []LogQuery{{App: "web"}, {App: "web", Types: allTypes}} {
				selector = ""
				if _, err := svc.QueryLogs(ctx, q); err != nil {
					t.Fatalf("query: %v", err)
				}
				if !strings.Contains(selector, want.matcher) || (plan == "hobby" && strings.Contains(selector, "container=~")) {
					t.Fatalf("selector %s: want %s", selector, want.matcher)
				}
			}
			selector = ""
			if _, err := svc.Logs(ctx, "web", 10); err != nil || !strings.Contains(selector, want.matcher) {
				t.Fatalf("tail read selector %s (err %v): want %s", selector, err, want.matcher)
			}
			for label, wantValues := range map[string][]string{LabelType: want.types, LabelStatusCode: want.statusCode, LabelMethod: want.method} {
				got, err := svc.LogLabelValues(ctx, label, LogQuery{App: "web"})
				if err != nil || !slices.Equal(got, wantValues) {
					t.Fatalf("%s values = %v (err %v), want %v", label, got, err, wantValues)
				}
			}
		})
	}
}

// The matcher itself: an empty Types with noRequest is app-only.
func TestLokiTypeMatcherHonorsNoRequest(t *testing.T) {
	if got := lokiTypeMatcher(LogQuery{noRequest: true}); got != `container="app"` {
		t.Fatalf("hobby all-types matcher = %s", got)
	}
	if got := lokiTypeMatcher(LogQuery{}); got != `container=~"app|"` {
		t.Fatalf("pro all-types matcher = %s", got)
	}
	if got := lokiTypeMatcher(LogQuery{noRequest: true, Types: []string{LogTypeApp}}); got != `container="app"` {
		t.Fatalf("hobby app matcher = %s", got)
	}
}
