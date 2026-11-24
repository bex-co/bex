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
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/115: LogLabelValues (REST /v1/logs/values, GraphQL logLabelValues, MCP
// list_log_label_values) must translate public instance IDs (and w32/114
// wildcard/regex selectors) to internal pod/deploy names before querying Loki,
// progressInstanceValues, and the datastore pod-list fallback.

func TestLogLabelValuesTranslatesPublicInstanceFilterAcrossSurfaces(t *testing.T) {
	base := time.Date(2026, 11, 24, 11, 0, 0, 0, time.UTC)
	start, end := base, base.Add(time.Minute)
	startStr, endStr := start.Format(time.RFC3339), end.Format(time.RFC3339)

	svcRes := "srv-lvinst000000000001"
	app := logTenantNSApp("web", logTestWS, svcRes)
	app.Spec.Repo = rolloutRepo
	pod1 := podFor(app.Name, "web-pod-1")
	pod1.Namespace = app.Namespace
	pod2 := podFor(app.Name, "web-pod-2")
	pod2.Namespace = app.Namespace
	histPodName := "web-pod-hibernated" // Loki-only historical pod, absent from K8s

	pgRes := postgresID
	pg := sampleDatabase(pgRes)
	pgPod1 := databasePod(pgRes, pgRes+"-1")
	pgPod2 := databasePod(pgRes, pgRes+"-2")

	kvRes := keyValueID
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: kvRes, Namespace: "default"}}
	kvPod1 := keyValuePod(kvRes, kvRes+"-0")
	kvPod2 := keyValuePod(kvRes, kvRes+"-1")

	buildPodName := buildPod(4, "lv111")
	dep1, dep2 := "dep-lv0000000000000001", "dep-lv0000000000000002"

	loki := newFrontendLoki(t,
		fakeStream{
			labels: map[string]string{"namespace": app.Namespace, "app": app.Name, "container": "app", "type": LogTypeApp, "pod": pod1.Name, "level": "unknown"},
			stamps: []int64{base.Add(5 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": app.Namespace, "app": app.Name, "container": "app", "type": LogTypeApp, "pod": pod2.Name, "level": "error"},
			stamps: []int64{base.Add(10 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": app.Namespace, "app": app.Name, "container": "app", "type": LogTypeApp, "pod": histPodName, "level": "warning"},
			stamps: []int64{base.Add(12 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": app.Namespace, "app": app.Name, "type": LogTypeRequest, "level": "info", "method": "GET", "status": "200"},
			stamps: []int64{base.Add(14 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": app.Namespace, "app": app.Name, "container": "build", "type": LogTypeBuild, "pod": buildPodName},
			stamps: []int64{base.Add(15 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "database": pgRes, "pod": pgPod1.Name, "level": "info"},
			stamps: []int64{base.Add(16 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "database": pgRes, "pod": pgPod2.Name, "level": "error"},
			stamps: []int64{base.Add(17 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "keyvalue": kvRes, "pod": kvPod1.Name, "level": "info"},
			stamps: []int64{base.Add(18 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "keyvalue": kvRes, "pod": kvPod2.Name, "level": "warning"},
			stamps: []int64{base.Add(19 * time.Second).UnixNano()},
		},
	)

	svc := newService(nil, app, pod1, pod2, pg, pgPod1, pgPod2, kv, kvPod1, kvPod2)
	svc.History = NewLokiSource(loki.srv.URL, loki.srv.Client())
	svc.LabelValues = NewLokiLabelValuesSource(loki.srv.URL, loki.srv.Client())
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{
			{ID: dep2, Generation: 5, Status: "live", Image: "nginx:1.27", CreatedAt: base.Add(30 * time.Second), StartedAt: base.Add(31 * time.Second), FinishedAt: base.Add(35 * time.Second)},
			{ID: dep1, Generation: 4, Status: "deactivated", Built: true, Commit: "abc1234", CreatedAt: base.Add(6 * time.Second), StartedAt: base.Add(8 * time.Second), FinishedAt: base.Add(14 * time.Second)},
		}, nil
	}

	pubPod1 := ids.ServiceInstanceID(svcRes, pod1.Name)
	pubPod2 := ids.ServiceInstanceID(svcRes, pod2.Name)
	pubHist := ids.ServiceInstanceID(svcRes, histPodName)
	pubBuildPod := ids.ServiceInstanceID(svcRes, buildPodName)
	pubDep1 := ids.ServiceInstanceID(svcRes, dep1)
	pubDep2 := ids.ServiceInstanceID(svcRes, dep2)
	pubPG1 := ids.ServiceInstanceID(pgRes, pgPod1.Name)
	pubPG2 := ids.ServiceInstanceID(pgRes, pgPod2.Name)
	pubKV1 := ids.ServiceInstanceID(kvRes, kvPod1.Name)
	pubKV2 := ids.ServiceInstanceID(kvRes, kvPod2.Name)

	prefix := func(res, pub string) string { return pub[:len(res)+8] }
	sorted := func(vs ...string) []string {
		out := slices.Clone(vs)
		slices.Sort(out)
		return out
	}

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

	readSurface := func(t *testing.T, surface, resource, label, logType string, inst []string) []string {
		t.Helper()
		switch surface {
		case "REST":
			params := url.Values{
				"resource":  {resource},
				"label":     {label},
				"startTime": {startStr},
				"endTime":   {endStr},
			}
			if logType != "" {
				params.Set("type", logType)
			}
			for _, v := range inst {
				params.Add("instance", v)
			}
			rec := serveREST(svc, http.MethodGet, "/v1/logs/values?"+params.Encode())
			if rec.Code != http.StatusOK {
				t.Fatalf("REST /v1/logs/values (%s %s %v) = %d %s", resource, label, inst, rec.Code, rec.Body.String())
			}
			var got []string
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode REST values: %v", err)
			}
			return got
		case "GraphQL":
			typeArg := ""
			if logType != "" {
				typeArg = fmt.Sprintf(", type:%q", logType)
			}
			instArg := ""
			if len(inst) > 0 {
				raw, _ := json.Marshal(inst)
				instArg = fmt.Sprintf(", instance:%s", raw)
			}
			data := runQuery(t, schema, fmt.Sprintf(`{ logLabelValues(resource:%q, label:%q%s%s, startTime:%q, endTime:%q) }`, resource, label, typeArg, instArg, startStr, endStr))
			raw, _ := data["logLabelValues"].([]any)
			out := make([]string, len(raw))
			for i, v := range raw {
				out[i] = v.(string)
			}
			return out
		case "MCP":
			args := map[string]any{
				"resource":  []string{resource},
				"label":     label,
				"startTime": startStr,
				"endTime":   endStr,
			}
			if logType != "" {
				args["type"] = []string{logType}
			}
			if len(inst) > 0 {
				args["instance"] = inst
			}
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_log_label_values", Arguments: args})
			if err != nil || res.IsError {
				t.Fatalf("MCP list_log_label_values(%s %s %v): err=%v res=%+v", resource, label, inst, err, res)
			}
			var out []string
			rendertest.DecodeMCPText(t, res, &out)
			return out
		default:
			t.Fatalf("unknown surface %q", surface)
			return nil
		}
	}

	for _, tc := range []struct {
		name     string
		resource string
		label    string
		logType  string
		inst     []string
		want     []string
	}{
		{
			name:     "service unfiltered level control",
			resource: svcRes, label: LabelLevel,
			want: []string{"error", "info", "unknown", "warning"},
		},
		{
			name:     "service unfiltered instance control",
			resource: svcRes, label: LabelInstance,
			want: sorted(pubPod1, pubPod2, pubHist),
		},
		{
			name:     "service exact public instance filters level",
			resource: svcRes, label: LabelLevel, inst: []string{pubPod1},
			want: []string{"unknown"},
		},
		{
			name:     "service exact public instance filters type",
			resource: svcRes, label: LabelType, inst: []string{pubPod1},
			want: []string{LogTypeApp},
		},
		{
			name:     "service exact public instance filters instance",
			resource: svcRes, label: LabelInstance, inst: []string{pubPod1},
			want: []string{pubPod1},
		},
		{
			name:     "service historical-only pod resolves from Loki",
			resource: svcRes, label: LabelLevel, inst: []string{pubHist},
			want: []string{"warning"},
		},
		{
			name:     "service prefix wildcard filters instance",
			resource: svcRes, label: LabelInstance, inst: []string{prefix(svcRes, pubPod2) + "*"},
			want: []string{pubPod2},
		},
		{
			name:     "service regex filters level",
			resource: svcRes, label: LabelLevel, inst: []string{"/" + prefix(svcRes, pubPod2) + "/"},
			want: []string{"error"},
		},
		{
			name:     "service star wildcard returns all pod levels",
			resource: svcRes, label: LabelLevel, inst: []string{"*"},
			want: []string{"error", "unknown", "warning"},
		},
		{
			name:     "service unknown public instance returns empty level",
			resource: svcRes, label: LabelLevel, inst: []string{svcRes + "-nonexistent"},
			want: []string{},
		},
		{
			name:     "service unknown public instance returns empty instance",
			resource: svcRes, label: LabelInstance, inst: []string{svcRes + "-nonexistent"},
			want: []string{},
		},
		{
			name:     "service build exact deploy progress instance filters instance",
			resource: svcRes, label: LabelInstance, logType: LogTypeBuild, inst: []string{pubDep1},
			want: []string{pubDep1},
		},
		{
			name:     "service build exact deploy progress instance filters type",
			resource: svcRes, label: LabelType, logType: LogTypeBuild, inst: []string{pubDep1},
			want: []string{LogTypeBuild},
		},
		{
			name:     "service build exact buildkit pod filters instance",
			resource: svcRes, label: LabelInstance, logType: LogTypeBuild, inst: []string{pubBuildPod},
			want: []string{pubBuildPod},
		},
		{
			name:     "service build star wildcard returns buildkit pod and deploys",
			resource: svcRes, label: LabelInstance, logType: LogTypeBuild, inst: []string{"*"},
			want: sorted(pubBuildPod, pubDep1, pubDep2),
		},
		{
			name:     "service build unknown instance returns empty instance and does not leak deploys",
			resource: svcRes, label: LabelInstance, logType: LogTypeBuild, inst: []string{svcRes + "-nonexistent"},
			want: []string{},
		},
		{
			name:     "service build unknown instance returns empty type",
			resource: svcRes, label: LabelType, logType: LogTypeBuild, inst: []string{svcRes + "-nonexistent"},
			want: []string{},
		},
		{
			name:     "postgres exact public instance filters level",
			resource: pgRes, label: LabelLevel, inst: []string{pubPG1},
			want: []string{"info"},
		},
		{
			name:     "postgres exact public instance filters instance",
			resource: pgRes, label: LabelInstance, inst: []string{pubPG1},
			want: []string{pubPG1},
		},
		{
			name:     "postgres regex filters level",
			resource: pgRes, label: LabelLevel, inst: []string{"/" + prefix(pgRes, pubPG2) + "/"},
			want: []string{"error"},
		},
		{
			name:     "postgres star wildcard returns both instances",
			resource: pgRes, label: LabelInstance, inst: []string{"*"},
			want: sorted(pubPG1, pubPG2),
		},
		{
			name:     "postgres unknown instance returns empty",
			resource: pgRes, label: LabelLevel, inst: []string{pgRes + "-nonexistent"},
			want: []string{},
		},
		{
			name:     "keyvalue exact public instance filters level",
			resource: kvRes, label: LabelLevel, inst: []string{pubKV1},
			want: []string{"info"},
		},
		{
			name:     "keyvalue exact public instance filters instance",
			resource: kvRes, label: LabelInstance, inst: []string{pubKV1},
			want: []string{pubKV1},
		},
		{
			name:     "keyvalue wildcard filters level",
			resource: kvRes, label: LabelLevel, inst: []string{prefix(kvRes, pubKV2) + "*"},
			want: []string{"warning"},
		},
		{
			name:     "keyvalue unknown instance returns empty",
			resource: kvRes, label: LabelInstance, inst: []string{kvRes + "-nonexistent"},
			want: []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, surface := range []string{"REST", "GraphQL", "MCP"} {
				t.Run(surface, func(t *testing.T) {
					got := readSurface(t, surface, tc.resource, tc.label, tc.logType, tc.inst)
					if !slices.Equal(got, tc.want) {
						t.Fatalf("%s(%s %s %v) = %v, want %v", surface, tc.resource, tc.label, tc.inst, got, tc.want)
					}
				})
			}
		})
	}

	// Control: malformed regex on instance still returns 400 naming instance.
	t.Run("malformed regex still rejected with 400", func(t *testing.T) {
		_, err := svc.LogLabelValues(ctx, LabelLevel, LogQuery{App: svcRes, Instance: []string{"/[unclosed/"}, Since: start, End: end})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "instance") {
			t.Fatalf("LogLabelValues malformed regex err = %v, want 400 naming instance", err)
		}
		rec := serveREST(svc, http.MethodGet, "/v1/logs/values?resource="+svcRes+"&label=level&instance="+url.QueryEscape("/[unclosed/")+"&startTime="+startStr+"&endTime="+endStr)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid instance filter") {
			t.Fatalf("REST malformed regex = %d %s, want 400 invalid instance filter", rec.Code, rec.Body.String())
		}
	})
}

func TestDatastoreLogLabelValuesFallbackFiltersInstance(t *testing.T) {
	pgRes := postgresID
	pg := sampleDatabase(pgRes)
	pgPod1 := databasePod(pgRes, pgRes+"-1")
	pgPod2 := databasePod(pgRes, pgRes+"-2")

	kvRes := keyValueID
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: kvRes, Namespace: "default"}}
	kvPod1 := keyValuePod(kvRes, kvRes+"-0")
	kvPod2 := keyValuePod(kvRes, kvRes+"-1")

	// No LabelValues wired: exercises datastoreLogLabelValues's pod-list fallback.
	svc := newService(nil, pg, pgPod1, pgPod2, kv, kvPod1, kvPod2)
	svc.LabelValues = nil

	ctx := context.Background()
	for _, ds := range []struct {
		name string
		res  string
		pod1 string
		pod2 string
	}{
		{"postgres", pgRes, pgPod1.Name, pgPod2.Name},
		{"keyvalue", kvRes, kvPod1.Name, kvPod2.Name},
	} {
		t.Run(ds.name, func(t *testing.T) {
			pub1 := ids.ServiceInstanceID(ds.res, ds.pod1)
			pub2 := ids.ServiceInstanceID(ds.res, ds.pod2)
			both := []string{pub1, pub2}
			slices.Sort(both)
			pre1 := pub1[:len(ds.res)+8]

			for _, tc := range []struct {
				name string
				q    LogQuery
				want []string
			}{
				{"unfiltered returns all pods", LogQuery{App: ds.res}, both},
				{"exact public ID selects matching pod", LogQuery{App: ds.res, Instance: []string{pub1}}, []string{pub1}},
				{"wildcard prefix selects matching pod", LogQuery{App: ds.res, Instance: []string{pre1 + "*"}}, []string{pub1}},
				{"regex selects matching pod", LogQuery{App: ds.res, Instance: []string{"/" + pre1 + "/"}}, []string{pub1}},
				{"star selects all pods", LogQuery{App: ds.res, Instance: []string{"*"}}, both},
				{"unknown public ID returns empty", LogQuery{App: ds.res, Instance: []string{ds.res + "-nonexistent"}}, []string{}},
				{"negated exact excludes matching pod", LogQuery{App: ds.res, Instance: []string{pub1}, negated: []string{LabelInstance}}, []string{pub2}},
				{"negated unknown excludes nothing", LogQuery{App: ds.res, Instance: []string{ds.res + "-nonexistent"}, negated: []string{LabelInstance}}, both},
			} {
				t.Run(tc.name, func(t *testing.T) {
					got, err := svc.LogLabelValues(ctx, LabelInstance, tc.q)
					if err != nil {
						t.Fatalf("LogLabelValues(%+v): %v", tc.q, err)
					}
					if !slices.Equal(got, tc.want) {
						t.Fatalf("LogLabelValues(%+v) = %v, want %v", tc.q, got, tc.want)
					}
				})
			}
		})
	}
}

func TestProjectLogLabelValuesWithInstanceAndNegatedInstance(t *testing.T) {
	base := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	svc, _ := projectFixture()
	pgPod := postgresID + "-1"

	loki := newFrontendLoki(t,
		fakeStream{
			labels: map[string]string{"namespace": "default", "app": projectWeb, "container": "app", "type": LogTypeApp, "pod": "web-0", "level": "info"},
			stamps: []int64{base.Add(5 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "app": projectWeb, "container": "app", "type": LogTypeApp, "pod": "web-1", "level": "warning"},
			stamps: []int64{base.Add(10 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "app": projectCron, "container": "app", "type": LogTypeApp, "pod": "nightly-1", "level": "error"},
			stamps: []int64{base.Add(15 * time.Second).UnixNano()},
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "database": postgresID, "pod": pgPod, "level": "debug"},
			stamps: []int64{base.Add(20 * time.Second).UnixNano()},
		},
	)
	svc.LabelValues = NewLokiLabelValuesSource(loki.srv.URL, loki.srv.Client())

	pubWeb0 := ids.ServiceInstanceID(projectWeb, "web-0")
	pubWeb1 := ids.ServiceInstanceID(projectWeb, "web-1")
	pubCron := ids.ServiceInstanceID(projectCron, "nightly-1")
	pubPG := ids.ServiceInstanceID(postgresID, pgPod)

	filterDoc := func(op string, vals ...string) string {
		raw, _ := json.Marshal(vals)
		return fmt.Sprintf(`filterDocument:{filters: [{field: "INSTANCE", operator: %q, values: %s}], version: 1}`, op, raw)
	}
	sorted := func(vs ...string) []string {
		out := slices.Clone(vs)
		slices.Sort(out)
		return out
	}

	// Ensure project resources include all three (web, cron, postgres).
	if _, ok := svc.Projects.(*fakeProject); !ok {
		t.Fatal("expected *fakeProject")
	}
	_ = savedsearch.FieldInstance

	for _, tc := range []struct {
		name  string
		label string
		doc   string
		want  []string
	}{
		{
			name:  "IS exact service instance selects only that pod's level",
			label: LabelLevel, doc: filterDoc("IS", pubWeb1),
			want: []string{"warning"},
		},
		{
			name:  "IS exact postgres instance selects only postgres level",
			label: LabelLevel, doc: filterDoc("IS", pubPG),
			want: []string{"debug"},
		},
		{
			name:  "IS wildcard on service selects all its pods",
			label: LabelInstance, doc: filterDoc("IS", projectWeb+"-*"),
			want: sorted(pubWeb0, pubWeb1),
		},
		{
			name:  "IS_NOT exact service instance excludes only that pod across project",
			label: LabelLevel, doc: filterDoc("IS_NOT", pubWeb1),
			want: []string{"debug", "error", "info"},
		},
		{
			name:  "IS_NOT exact service instance excludes only that pod from instance values",
			label: LabelInstance, doc: filterDoc("IS_NOT", pubWeb1),
			want: sorted(pubWeb0, pubCron, pubPG),
		},
		{
			name:  "IS_NOT wildcard on service excludes both its pods only",
			label: LabelInstance, doc: filterDoc("IS_NOT", projectWeb+"-*"),
			want: sorted(pubCron, pubPG),
		},
		{
			name:  "IS_NOT unknown instance excludes nothing",
			label: LabelLevel, doc: filterDoc("IS_NOT", projectWeb+"-nonexistent"),
			want: []string{"debug", "error", "info", "warning"},
		},
		{
			name:  "IS unknown instance returns empty",
			label: LabelLevel, doc: filterDoc("IS", projectWeb+"-nonexistent"),
			want: []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, errs := runProjectValues(t, svc, context.Background(), fmt.Sprintf(`project:"prj-observed", label:%q, %s, endTime:"2026-07-05T00:01:00Z"`, tc.label, tc.doc))
			if errs != nil {
				t.Fatalf("runProjectValues: %v", errs)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
