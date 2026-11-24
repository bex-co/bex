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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/114: `*` wildcards and `/RE2/` regular expressions on the `instance`
// filter must evaluate against authorized candidate public/legacy/raw IDs
// across history, pre-deploy, datastores, project views (IS & IS_NOT), and
// live tails (including newly arriving pods and deploys).

func TestResolveInstanceSelectorsPatterns(t *testing.T) {
	resource := "srv-c185th5c2rvvnhbfiltg"
	a := ids.InstanceCandidate{Name: "web-alpha-1", UID: "uid-alpha-1"}
	b := ids.InstanceCandidate{Name: "web-beta-2", UID: "uid-beta-2"}
	c := ids.InstanceCandidate{Name: "worker-gamma-3", UID: "uid-gamma-3"}
	cands := []ids.InstanceCandidate{a, b, c}

	pubA := ids.ServiceInstanceID(resource, a.Name)
	pubB := ids.ServiceInstanceID(resource, b.Name)
	pubC := ids.ServiceInstanceID(resource, c.Name)
	legA := ids.LegacyServiceInstanceID(resource, a.UID)

	// Pick distinct prefixes for pubA and pubB so each prefix matches only one.
	preA := pubA[:len(resource)+6]
	preB := pubB[:len(resource)+6]
	if preA == preB {
		preA = pubA[:len(resource)+10]
		preB = pubB[:len(resource)+10]
	}

	for _, tc := range []struct {
		name      string
		selectors []string
		want      []string
	}{
		{"star matches all", []string{"*"}, []string{a.Name, b.Name, c.Name}},
		{"resource prefix wildcard", []string{resource + "-*"}, []string{a.Name, b.Name, c.Name}},
		{"single public prefix wildcard", []string{preA + "*"}, []string{a.Name}},
		{"public suffix wildcard", []string{"*" + pubB[len(pubB)-6:]}, []string{b.Name}},
		{"raw pod wildcard", []string{"web-*"}, []string{a.Name, b.Name}},
		{"legacy UID prefix wildcard", []string{legA[:len(resource)+8] + "*"}, []string{a.Name}},
		{"unanchored regex single", []string{"/" + pubA[len(resource)+1:len(resource)+8] + "/"}, []string{a.Name}},
		{"unanchored regex alternation multi", []string{"/(" + preA + "|" + preB + ")/"}, []string{a.Name, b.Name}},
		{"anchored regex start", []string{"/^" + preB + "/"}, []string{b.Name}},
		{"anchored wildcard rejects leading chars", []string{pubC[len(resource)+1:] + "*"}, nil},
		{"mixed exact and wildcard deduped", []string{pubA, "web-*", "does-not-exist", pubA}, []string{a.Name, b.Name}},
		{"unmatched wildcard", []string{"srv-other-*"}, nil},
		{"unmatched regex", []string{"/no-such-instance/"}, nil},
		{"malformed regex fails closed", []string{"/[unclosed/"}, nil},
		{"exact public id control", []string{pubC}, []string{c.Name}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveInstanceSelectors(tc.selectors, resource, cands)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("resolveInstanceSelectors(%v) = %v, want %v", tc.selectors, got, tc.want)
			}
		})
	}

	// Exact selector still enforces uniqueness when two candidates share a legacy UID.
	dupCands := []ids.InstanceCandidate{a, {Name: "web-dup", UID: a.UID}}
	if got := resolveInstanceSelectors([]string{legA}, resource, dupCands); len(got) != 0 {
		t.Fatalf("ambiguous exact legacy UID resolved to %v, want empty", got)
	}
	// Pattern selector intentionally matches all matching candidates.
	if got := resolveInstanceSelectors([]string{legA[:len(resource)+8] + "*"}, resource, dupCands); !slices.Equal(got, []string{a.Name, "web-dup"}) {
		t.Fatalf("wildcard legacy UID = %v, want both candidates", got)
	}
}

func TestInstancePatternFiltersAcrossSurfaces(t *testing.T) {
	base := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	start, end := base, base.Add(time.Minute)
	startStr, endStr := start.Format(time.RFC3339), end.Format(time.RFC3339)

	svcRes := "srv-pattern000000000001"
	app := logTenantNSApp("web", logTestWS, svcRes)
	app.Spec.Repo = rolloutRepo
	pod1 := podFor(app.Name, "web-pod-1")
	pod1.Namespace = app.Namespace
	pod2 := podFor(app.Name, "web-pod-2")
	pod2.Namespace = app.Namespace
	pre1 := preDeployPodIn(app.Name, "predeploy-1", app.Namespace)
	pre2 := preDeployPodIn(app.Name, "predeploy-2", app.Namespace)

	pgRes := postgresID
	pg := sampleDatabase(pgRes)
	pgPod1 := databasePod(pgRes, pgRes+"-1")
	pgPod2 := databasePod(pgRes, pgRes+"-2")

	kvRes := keyValueID
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: kvRes, Namespace: "default"}}
	kvPod1 := keyValuePod(kvRes, kvRes+"-0")
	kvPod2 := keyValuePod(kvRes, kvRes+"-1")

	buildPodName := buildPod(4, "pat11")
	dep1, dep2 := "dep-pat0000000000000001", "dep-pat0000000000000002"

	loki := newFrontendLoki(t,
		fakeStream{
			labels: map[string]string{"namespace": app.Namespace, "app": app.Name, "container": "app", "type": LogTypeApp, "pod": pod1.Name, "level": "info"},
			stamps: []int64{base.Add(5 * time.Second).UnixNano()},
			text:   func(int64) string { return "app line from pod1" },
		},
		fakeStream{
			labels: map[string]string{"namespace": app.Namespace, "app": app.Name, "container": "app", "type": LogTypeApp, "pod": pod2.Name, "level": "info"},
			stamps: []int64{base.Add(10 * time.Second).UnixNano()},
			text:   func(int64) string { return "app line from pod2" },
		},
		fakeStream{
			labels: map[string]string{"namespace": app.Namespace, "app": app.Name, "type": LogTypeBuild, "pod": buildPodName},
			stamps: []int64{base.Add(12 * time.Second).UnixNano()},
			text:   func(int64) string { return "buildkit output" },
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "database": pgRes, "pod": pgPod1.Name, "level": "info"},
			stamps: []int64{base.Add(15 * time.Second).UnixNano()},
			text:   func(int64) string { return "pg primary line" },
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "database": pgRes, "pod": pgPod2.Name, "level": "info"},
			stamps: []int64{base.Add(16 * time.Second).UnixNano()},
			text:   func(int64) string { return "pg replica line" },
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "keyvalue": kvRes, "pod": kvPod1.Name, "level": "info"},
			stamps: []int64{base.Add(17 * time.Second).UnixNano()},
			text:   func(int64) string { return "kv primary line" },
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "keyvalue": kvRes, "pod": kvPod2.Name, "level": "info"},
			stamps: []int64{base.Add(18 * time.Second).UnixNano()},
			text:   func(int64) string { return "kv replica line" },
		},
	)

	svc := newService(map[string][]string{
		pre1.Name: {base.Add(20*time.Second).Format(time.RFC3339) + " predeploy one"},
		pre2.Name: {base.Add(21*time.Second).Format(time.RFC3339) + " predeploy two"},
	}, app, pod1, pod2, pre1, pre2, pg, pgPod1, pgPod2, kv, kvPod1, kvPod2)
	svc.History = NewLokiSource(loki.srv.URL, loki.srv.Client())
	svc.LabelValues = func(_ context.Context, _ string, label string, q LogQuery) ([]string, error) {
		if label != LabelInstance {
			return nil, nil
		}
		switch {
		case q.Database == pgRes:
			return []string{pgPod1.Name, pgPod2.Name}, nil
		case q.KeyValue == kvRes:
			return []string{kvPod1.Name, kvPod2.Name}, nil
		case slices.Contains(q.Types, LogTypeBuild):
			return []string{buildPodName}, nil
		default:
			return []string{pod1.Name, pod2.Name}, nil
		}
	}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{
			{ID: dep2, Generation: 5, Status: "live", Image: "nginx:1.27", CreatedAt: base.Add(30 * time.Second), StartedAt: base.Add(31 * time.Second), FinishedAt: base.Add(35 * time.Second)},
			{ID: dep1, Generation: 4, Status: "deactivated", Built: true, Commit: "abc1234", CreatedAt: base.Add(6 * time.Second), StartedAt: base.Add(8 * time.Second), FinishedAt: base.Add(14 * time.Second)},
		}, nil
	}

	pubPod1 := ids.ServiceInstanceID(svcRes, pod1.Name)
	pubPod2 := ids.ServiceInstanceID(svcRes, pod2.Name)
	pubDep1 := ids.ServiceInstanceID(svcRes, dep1)
	pubDep2 := ids.ServiceInstanceID(svcRes, dep2)
	pubPre1 := ids.ServiceInstanceID(svcRes, pre1.Name)
	pubPre2 := ids.ServiceInstanceID(svcRes, pre2.Name)
	pubPG1 := ids.ServiceInstanceID(pgRes, pgPod1.Name)
	pubKV1 := ids.ServiceInstanceID(kvRes, kvPod1.Name)

	prefix := func(res, pub string) string { return pub[:len(res)+8] }

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("gqlSchema: %v", err)
	}
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(mcpSrv)
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpSrv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	readSurface := func(t *testing.T, surface, resource, logType string, inst []string) []string {
		t.Helper()
		switch surface {
		case "REST":
			params := url.Values{
				"resource":  {resource},
				"startTime": {startStr},
				"endTime":   {endStr},
				"limit":     {"100"},
				"direction": {"forward"},
				"instance":  inst,
			}
			if logType != "" {
				params.Set("type", logType)
			}
			rec := serveREST(svc, http.MethodGet, "/v1/logs?"+params.Encode())
			page := decodeLogList(t, rec)
			out := make([]string, len(page.Logs))
			for i, l := range page.Logs {
				out[i] = l.Message
			}
			return out
		case "GraphQL":
			rawInst, _ := json.Marshal(inst)
			typeArg := ""
			if logType != "" {
				typeArg = fmt.Sprintf(", type:%q", logType)
			}
			data := runQuery(t, schema, fmt.Sprintf(`{ logs(resource:%q%s, instance:%s, startTime:%q, endTime:%q, direction:"forward", limit:100) { logs { message } } }`, resource, typeArg, rawInst, startStr, endStr))
			rows := data["logs"].(map[string]any)["logs"].([]any)
			out := make([]string, len(rows))
			for i, raw := range rows {
				out[i] = raw.(map[string]any)["message"].(string)
			}
			return out
		case "MCP":
			args := map[string]any{
				"resource":  []string{resource},
				"instance":  inst,
				"startTime": startStr,
				"endTime":   endStr,
				"direction": "forward",
				"limit":     100,
			}
			if logType != "" {
				args["type"] = []string{logType}
			}
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: args})
			if err != nil || res.IsError {
				t.Fatalf("MCP list_logs(%s, %v): err=%v res=%+v", resource, inst, err, res)
			}
			var page renderLogList
			rendertest.DecodeMCPText(t, res, &page)
			out := make([]string, len(page.Logs))
			for i, l := range page.Logs {
				out[i] = l.Message
			}
			return out
		default:
			t.Fatalf("unknown surface %q", surface)
			return nil
		}
	}

	for _, tc := range []struct {
		name     string
		resource string
		logType  string
		inst     []string
		wantAll  []string // every message must appear
		wantOnly []string // if non-nil, exact message list
		wantZero bool
	}{
		{
			name:     "service app star matches both pods",
			resource: svcRes, logType: LogTypeApp, inst: []string{"*"},
			wantOnly: []string{"app line from pod1", "app line from pod2"},
		},
		{
			name:     "service app prefix wildcard matches pod1 only",
			resource: svcRes, logType: LogTypeApp, inst: []string{prefix(svcRes, pubPod1) + "*"},
			wantOnly: []string{"app line from pod1"},
		},
		{
			name:     "service app regex matches pod2 only",
			resource: svcRes, logType: LogTypeApp, inst: []string{"/" + prefix(svcRes, pubPod2) + "/"},
			wantOnly: []string{"app line from pod2"},
		},
		{
			name:     "service app mixed exact and wildcard",
			resource: svcRes, logType: LogTypeApp, inst: []string{pubPod1, prefix(svcRes, pubPod2) + "*"},
			wantOnly: []string{"app line from pod1", "app line from pod2"},
		},
		{
			name:     "service app exact control",
			resource: svcRes, logType: LogTypeApp, inst: []string{pubPod1},
			wantOnly: []string{"app line from pod1"},
		},
		{
			name:     "service app unmatched wildcard returns empty",
			resource: svcRes, logType: LogTypeApp, inst: []string{"srv-nomatch*"},
			wantZero: true,
		},
		{
			name:     "service app unmatched regex returns empty",
			resource: svcRes, logType: LogTypeApp, inst: []string{"/does-not-match/"},
			wantZero: true,
		},
		{
			name:     "service build star matches buildkit pod and both deploys",
			resource: svcRes, logType: LogTypeBuild, inst: []string{"*"},
			wantAll: []string{"==> Build queued", "buildkit output", rendersLive},
		},
		{
			name:     "service build regex matches both deploy progress instances",
			resource: svcRes, logType: LogTypeBuild, inst: []string{"/(" + prefix(svcRes, pubDep1) + "|" + prefix(svcRes, pubDep2) + ")/"},
			wantAll: []string{"==> Build queued", rendersLive},
		},
		{
			name:     "predeploy star matches both pods",
			resource: svcRes, logType: LogTypePreDeploy, inst: []string{"*"},
			wantOnly: []string{"predeploy one", "predeploy two"},
		},
		{
			name:     "predeploy regex matches single pod",
			resource: svcRes, logType: LogTypePreDeploy, inst: []string{"/" + prefix(svcRes, pubPre2) + "/"},
			wantOnly: []string{"predeploy two"},
		},
		{
			name:     "predeploy wildcard matches single pod",
			resource: svcRes, logType: LogTypePreDeploy, inst: []string{prefix(svcRes, pubPre1) + "*"},
			wantOnly: []string{"predeploy one"},
		},
		{
			name:     "postgres star matches both pods",
			resource: pgRes, inst: []string{"*"},
			wantOnly: []string{"pg primary line", "pg replica line"},
		},
		{
			name:     "postgres prefix wildcard matches primary only",
			resource: pgRes, inst: []string{prefix(pgRes, pubPG1) + "*"},
			wantOnly: []string{"pg primary line"},
		},
		{
			name:     "keyvalue star matches both pods",
			resource: kvRes, inst: []string{"*"},
			wantOnly: []string{"kv primary line", "kv replica line"},
		},
		{
			name:     "keyvalue regex matches primary only",
			resource: kvRes, inst: []string{"/" + prefix(kvRes, pubKV1) + "/"},
			wantOnly: []string{"kv primary line"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, surface := range []string{"REST", "GraphQL", "MCP"} {
				t.Run(surface, func(t *testing.T) {
					got := readSurface(t, surface, tc.resource, tc.logType, tc.inst)
					if tc.wantZero {
						if len(got) != 0 {
							t.Fatalf("got %q, want 0 lines", got)
						}
						return
					}
					if tc.wantOnly != nil && !slices.Equal(got, tc.wantOnly) {
						t.Fatalf("got %q, want %q", got, tc.wantOnly)
					}
					for _, want := range tc.wantAll {
						if !slices.Contains(got, want) {
							t.Fatalf("missing %q in %q", want, got)
						}
					}
				})
			}
		})
	}

	// Malformed regex still returns 400 naming instance across surfaces.
	t.Run("malformed regex still rejected with 400", func(t *testing.T) {
		_, err := svc.QueryLogs(ctx, LogQuery{App: svcRes, Instance: []string{"/[unclosed/"}, Since: start, End: end})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "instance") {
			t.Fatalf("QueryLogs malformed regex err = %v, want 400 naming instance", err)
		}
		rec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+svcRes+"&instance="+url.QueryEscape("/[unclosed/")+"&startTime="+startStr+"&endTime="+endStr)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid instance filter") {
			t.Fatalf("REST malformed regex = %d %s, want 400 invalid instance filter", rec.Code, rec.Body.String())
		}
	})
}

func TestNegatedInstancePatternFiltersInProjectView(t *testing.T) {
	svc := negationProject(t)
	const start, end = fixtureDay, "2026-07-05T00:01:00Z"
	all := corpusKeys(func(negationLine) bool { return true })
	pubWeb1 := ids.ServiceInstanceID(projectWeb, "web-1")

	for _, tc := range []struct {
		name    string
		pattern string
		want    []string
	}{
		{
			name:    "star excludes every instance",
			pattern: "*",
			want:    nil,
		},
		{
			name:    "wildcard on one service excludes all its pods only",
			pattern: projectWeb + "-*",
			want:    corpusKeys(func(l negationLine) bool { return l.resource != projectWeb }),
		},
		{
			name:    "regex on one pod excludes only that pod",
			pattern: "/" + pubWeb1[:len(projectWeb)+8] + "/",
			want:    corpusKeys(func(l negationLine) bool { return l.pod != "web-1" }),
		},
		{
			name:    "unmatched pattern excludes nothing",
			pattern: "srv-does-not-exist-*",
			want:    all,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isGQL, isRaw := viewDocument("INSTANCE", "IS", tc.pattern)
			notGQL, notRaw := viewDocument("INSTANCE", "IS_NOT", tc.pattern)
			assertComplement(t, "GraphQL instance "+tc.pattern,
				projectHistory(t, svc, LogTypeApp, isGQL, start, end),
				projectHistory(t, svc, LogTypeApp, notGQL, start, end),
				all, tc.want)
			assertComplement(t, "tail instance "+tc.pattern,
				projectTail(t, svc, LogTypeApp, isRaw, 400*time.Millisecond),
				projectTail(t, svc, LogTypeApp, notRaw, 400*time.Millisecond),
				all, tc.want)
		})
	}
}

func TestLiveTailInstancePatternsFollowNewPodsAndDeploys(t *testing.T) {
	t.Run("followPods relist attaches new matching pod and skips non-matching pod", func(t *testing.T) {
		resource := "srv-tailpat000000000001"
		app := logTenantNSApp("web", logTestWS, resource)
		opened := time.Now().UTC().Truncate(time.Second)
		stamp := func(d time.Duration) string { return opened.Add(d).Format(time.RFC3339Nano) }

		podInitial := podFor(app.Name, "web-init")
		podInitial.Namespace = app.Namespace
		podMatch := podFor(app.Name, "web-late-match")
		podMatch.Namespace = app.Namespace
		podSkip := podFor(app.Name, "web-late-skip")
		podSkip.Namespace = app.Namespace

		pubMatch := ids.ServiceInstanceID(resource, podMatch.Name)
		matchPrefix := pubMatch[:len(resource)+8]

		for _, tc := range []struct {
			name     string
			selector string
			wantInit bool
		}{
			{"wildcard prefix matching late pod only", matchPrefix + "*", false},
			{"regex matching late pod only", "/" + matchPrefix + "/", false},
			{"star matching initial and late pods", "*", true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				lines := map[string][]string{
					podInitial.Name: {stamp(1*time.Second) + " line from initial"},
					podMatch.Name:   {stamp(2*time.Second) + " line from late-match"},
					podSkip.Name:    {stamp(3*time.Second) + " line from late-skip"},
				}
				svc := newService(lines, app, podInitial)
				svc.TailRelistInterval = 15 * time.Millisecond

				var mu sync.Mutex
				var got []LogEntry
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				errCh := make(chan error, 1)
				go func() {
					errCh <- svc.FollowLogs(ctx, LogQuery{
						App:      resource,
						Types:    []string{LogTypeApp},
						Instance: []string{tc.selector},
						Since:    opened,
					}, func(e LogEntry) error {
						mu.Lock()
						got = append(got, e)
						mu.Unlock()
						return nil
					})
				}()

				// Let the initial attach settle, then create two new pods after subscribe start.
				time.Sleep(45 * time.Millisecond)
				if err := svc.Client.Create(context.Background(), podMatch.DeepCopy()); err != nil {
					t.Fatalf("create podMatch: %v", err)
				}
				if err := svc.Client.Create(context.Background(), podSkip.DeepCopy()); err != nil {
					t.Fatalf("create podSkip: %v", err)
				}

				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					mu.Lock()
					hasMatch := slices.ContainsFunc(got, func(e LogEntry) bool { return e.Message == "line from late-match" })
					mu.Unlock()
					if hasMatch {
						// Wait two more relist ticks to confirm non-matching pod is still skipped.
						time.Sleep(50 * time.Millisecond)
						break
					}
					time.Sleep(15 * time.Millisecond)
				}
				cancel()
				if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
					t.Fatalf("FollowLogs: %v", err)
				}

				mu.Lock()
				msgs := make([]string, len(got))
				for i, e := range got {
					msgs[i] = e.Message
				}
				mu.Unlock()

				if !slices.Contains(msgs, "line from late-match") {
					t.Fatalf("selector %q missed late matching pod; got %q", tc.selector, msgs)
				}
				if tc.wantInit {
					if !slices.Contains(msgs, "line from initial") || !slices.Contains(msgs, "line from late-skip") {
						t.Fatalf("selector %q expected all pods; got %q", tc.selector, msgs)
					}
				} else {
					if slices.Contains(msgs, "line from initial") || slices.Contains(msgs, "line from late-skip") {
						t.Fatalf("selector %q leaked non-matching pod; got %q", tc.selector, msgs)
					}
				}
			})
		}
	})

	t.Run("typeless WebSocket tail with star follows newly created deploy after subscribe", func(t *testing.T) {
		app := sampleApp("web")
		app.Spec.Repo = rolloutRepo
		opened := time.Now().UTC().Truncate(time.Second)
		soon := func(d time.Duration) string { return opened.Add(d).Format(time.RFC3339Nano) }

		svc := newService(map[string][]string{
			"web-1": {soon(1*time.Second) + " app stdout"},
		}, app, podFor("web", "web-1"))
		svc.BuildNamespace = "builds"
		svc.BuildPodWaitInterval = 15 * time.Millisecond
		svc.TailRelistInterval = 15 * time.Millisecond

		var mu sync.Mutex
		var rows []DeployProgress
		svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
			mu.Lock()
			defer mu.Unlock()
			return slices.Clone(rows), nil
		}
		srv := revalidationTestServer(svc)
		defer srv.Close()

		query := "resource=web&instance=*&startTime=" + url.QueryEscape(opened.Format(time.RFC3339))
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?"+query, nil)
		if err != nil {
			t.Fatalf("WS dial: %v", err)
		}
		defer conn.Close()

		// Create a deploy strictly AFTER subscribe opened.
		go func() {
			time.Sleep(40 * time.Millisecond)
			mu.Lock()
			rows = []DeployProgress{{
				ID:         "dep-late-wildcard",
				Generation: 3,
				Status:     "live",
				Image:      "docker.io/library/nginx:1.27-alpine",
				CreatedAt:  opened.Add(2 * time.Second),
				StartedAt:  opened.Add(3 * time.Second),
				FinishedAt: opened.Add(4 * time.Second),
				LiveURLs:   []string{"https://web.onbex.co"},
			}}
			mu.Unlock()
		}()

		var sawApp, sawLive bool
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for !(sawApp && sawLive) {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("WS read (sawApp=%v sawLive=%v): %v", sawApp, sawLive, err)
			}
			var line renderLog
			if err := json.Unmarshal(payload, &line); err != nil {
				t.Fatalf("frame %s: %v", payload, err)
			}
			if line.Message == "app stdout" {
				sawApp = true
			}
			if line.Message == rendersLive {
				sawLive = true
			}
		}
	})
}

func TestMCPListLogsSchemaDocumentsInstancePatterns(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(mcpSrv)
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpSrv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "list_logs" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		props := schema["properties"].(map[string]any)
		inst := props["instance"].(map[string]any)
		desc, _ := inst["description"].(string)
		if !strings.Contains(desc, "* wildcards") || !strings.Contains(desc, "/regex/") {
			t.Fatalf("list_logs instance description = %q, want mention of * wildcards and /regex/", desc)
		}
		return
	}
	t.Fatal("list_logs tool not found")
}
