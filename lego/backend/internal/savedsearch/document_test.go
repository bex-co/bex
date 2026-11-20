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

package savedsearch

import (
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const env = "evm-c185th5c2rvvnhbfiltg"

func TestParseAcceptsRenderShapedDocuments(t *testing.T) {
	for _, tc := range []struct {
		viewType, raw string
	}{
		{TypeApplicationLogs, `{"filters": [], "version": 1}`},
		{TypeRequestLogs, `{"filters": [], "version": 1}`},
		{TypeMetrics, `{"filters": [], "version": 1}`},
		{TypeApplicationLogs, `{"version": 1}`},
		{TypeApplicationLogs, `{"filters": [
			{"field": "ENVIRONMENT", "operator": "IS", "values": ["` + env + `"]},
			{"field": "RESOURCE_TYPE", "operator": "IS_NOT", "values": ["POSTGRES", "KEY_VALUE"]},
			{"field": "RESOURCE", "operator": "IS", "values": ["srv-c185th5c2rvvnhbfiltg", "dpg-c185th5c2rvvnhbfiltg", "red-c185th5c2rvvnhbfiltg"]},
			{"field": "LEVEL", "operator": "IS", "values": ["error", "warning"]},
			{"field": "INSTANCE", "operator": "IS", "values": ["srv-c185th5c2rvvnhbfiltg-abcde"]},
			{"field": "TEXT", "operator": "IS", "values": ["timeout"]},
			{"field": "TIME_RANGE", "operator": "IS", "values": ["4h"]}], "version": 1}`},
		{TypeRequestLogs, `{"filters": [
			{"field": "METHOD", "operator": "IS", "values": ["GET"]},
			{"field": "STATUS_CODE", "operator": "IS", "values": ["5*"]},
			{"field": "HOST", "operator": "IS", "values": ["example.com"]},
			{"field": "PATH", "operator": "IS", "values": ["/api/*"]}], "version": 1}`},
		// Render's Edit view offers `is not one of` on every line field but
		// Text Search (w3/m523).
		{TypeApplicationLogs, `{"filters": [
			{"field": "LEVEL", "operator": "IS_NOT", "values": ["info"]},
			{"field": "INSTANCE", "operator": "IS_NOT", "values": ["srv-c185th5c2rvvnhbfiltg-abcde"]}], "version": 1}`},
		{TypeRequestLogs, `{"filters": [
			{"field": "LEVEL", "operator": "IS_NOT", "values": ["info", "debug"]},
			{"field": "INSTANCE", "operator": "IS_NOT", "values": ["srv-c185th5c2rvvnhbfiltg-abcde"]},
			{"field": "METHOD", "operator": "IS_NOT", "values": ["GET", "HEAD"]},
			{"field": "STATUS_CODE", "operator": "IS_NOT", "values": ["2xx", "304"]},
			{"field": "HOST", "operator": "IS_NOT", "values": ["*.onrender.com"]},
			{"field": "PATH", "operator": "IS_NOT", "values": ["/healthz", "/^/static//"]}], "version": 1}`},
		// A legacy env- spelling of an environment is the same environment.
		{TypeApplicationLogs, `{"filters": [{"field": "ENVIRONMENT", "operator": "IS", "values": ["env-c185th5c2rvvnhbfiltg"]}], "version": 1}`},
	} {
		doc, err := Parse([]byte(tc.raw), tc.viewType)
		if err != nil {
			t.Errorf("%s %s: %v", tc.viewType, tc.raw, err)
			continue
		}
		if doc.Filters == nil {
			t.Errorf("%s: filters decoded as null, want []", tc.raw)
		}
	}
}

// Every refusal is a 400 that names what it objects to.
func TestParseRefusesByName(t *testing.T) {
	for _, tc := range []struct {
		viewType, raw, want string
	}{
		{"WORKFLOW_RUNS", `{"filters": [], "version": 1}`, `unknown saved search type "WORKFLOW_RUNS"`},
		{TypeApplicationLogs, `{"filters": [], "version": 2}`, "version 2 is not supported"},
		{TypeApplicationLogs, `{"version": 1, "extra": true}`, `unknown field "extra"`},
		{TypeApplicationLogs, `{"version": 1} {}`, "trailing data"},
		{TypeApplicationLogs, `[]`, "not a filter document"},
		{TypeApplicationLogs, `{"filters": [{"field": "METHOD", "operator": "IS", "values": ["GET"]}], "version": 1}`, `filter field "METHOD" is not a field of a APPLICATION_LOGS view`},
		{TypeRequestLogs, `{"filters": [{"field": "TEXT", "operator": "IS", "values": ["x"]}], "version": 1}`, `filter field "TEXT" is not a field of a REQUEST_LOGS view`},
		{TypeMetrics, `{"filters": [{"field": "LEVEL", "operator": "IS", "values": ["error"]}], "version": 1}`, `filter field "LEVEL" is not a field of a METRICS view`},
		{TypeApplicationLogs, `{"filters": [{"field": "LEVEL", "operator": "IS", "values": ["error"]}, {"field": "LEVEL", "operator": "IS", "values": ["info"]}], "version": 1}`, `"LEVEL" appears more than once`},
		{TypeApplicationLogs, `{"filters": [{"field": "LEVEL", "operator": "CONTAINS", "values": ["error"]}], "version": 1}`, `unknown operator "CONTAINS"`},
		{TypeApplicationLogs, `{"filters": [{"field": "ENVIRONMENT", "operator": "IS_NOT", "values": ["` + env + `"]}], "version": 1}`, `"ENVIRONMENT" does not support operator IS_NOT`},
		{TypeRequestLogs, `{"filters": [{"field": "ENVIRONMENT", "operator": "IS_NOT", "values": ["` + env + `"]}], "version": 1}`, `"ENVIRONMENT" does not support operator IS_NOT`},
		{TypeApplicationLogs, `{"filters": [{"field": "TEXT", "operator": "IS_NOT", "values": ["timeout"]}], "version": 1}`, `"TEXT" does not support operator IS_NOT`},
		{TypeApplicationLogs, `{"filters": [{"field": "TIME_RANGE", "operator": "IS_NOT", "values": ["4h"]}], "version": 1}`, `"TIME_RANGE" does not support operator IS_NOT`},
		{TypeMetrics, `{"filters": [{"field": "TIME_RANGE", "operator": "IS_NOT", "values": ["4h"]}], "version": 1}`, `"TIME_RANGE" does not support operator IS_NOT`},
		{TypeApplicationLogs, `{"filters": [{"field": "LEVEL", "operator": "IS", "values": []}], "version": 1}`, `"LEVEL" needs at least one value`},
		{TypeApplicationLogs, `{"filters": [{"field": "ENVIRONMENT", "operator": "IS", "values": ["` + env + `", "` + env + `"]}], "version": 1}`, `"ENVIRONMENT" takes exactly one value`},
		{TypeApplicationLogs, `{"filters": [{"field": "ENVIRONMENT", "operator": "IS", "values": ["prj-c185th5c2rvvnhbfiltg"]}], "version": 1}`, `"ENVIRONMENT" has an invalid value`},
		{TypeApplicationLogs, `{"filters": [{"field": "RESOURCE", "operator": "IS", "values": ["evg-c185th5c2rvvnhbfiltg"]}], "version": 1}`, `"RESOURCE" has an invalid value`},
		{TypeApplicationLogs, `{"filters": [{"field": "RESOURCE_TYPE", "operator": "IS", "values": ["STATIC_SITE"]}], "version": 1}`, `"RESOURCE_TYPE" has an invalid value "STATIC_SITE"`},
		{TypeApplicationLogs, `{"filters": [{"field": "TIME_RANGE", "operator": "IS", "values": ["90d"]}], "version": 1}`, `"TIME_RANGE" has an invalid value "90d"`},
		{TypeApplicationLogs, `{"filters": [{"field": "TEXT", "operator": "IS", "values": [""]}], "version": 1}`, `"TEXT" has an empty, invalid or over-long value`},
		{TypeApplicationLogs, `{"filters": [{"field": "TEXT", "operator": "IS", "values": ["` + strings.Repeat("x", maxValueBytes+1) + `"]}], "version": 1}`, `"TEXT" has an empty, invalid or over-long value`},
		{TypeApplicationLogs, `{"filters": [{"field": "TEXT", "operator": "IS", "values": [` + strings.Repeat(`"x",`, maxValues) + `"x"]}], "version": 1}`, `"TEXT" takes at most 50 values`},
	} {
		_, err := Parse([]byte(tc.raw), tc.viewType)
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s %s: %v, want a bad request naming %q", tc.viewType, tc.raw, err, tc.want)
		}
	}
}

func TestParseValueTakesADecodedObject(t *testing.T) {
	doc, err := ParseValue(map[string]any{"filters": []any{map[string]any{"field": "LEVEL", "operator": "IS", "values": []any{"error"}}}, "version": int64(1)}, TypeApplicationLogs)
	if err != nil {
		t.Fatal(err)
	}
	if values, negated := doc.Values(FieldLevel); len(values) != 1 || values[0] != "error" || negated {
		t.Fatalf("LEVEL = %v (negated %v)", values, negated)
	}
	if values, _ := doc.Values(FieldText); values != nil {
		t.Fatalf("an absent field reads %v", values)
	}
}

var projectResources = []Resource{
	{ID: "srv-web", Name: "web", Type: "web_service", EnvironmentID: env},
	{ID: "srv-legacy", Name: "legacy", Type: ""},
	{ID: "srv-private", Name: "internal", Type: "private_service", EnvironmentID: env},
	{ID: "srv-worker", Name: "worker", Type: "background_worker"},
	{ID: "srv-cron", Name: "nightly", Type: "cron_job", EnvironmentID: env},
	{ID: "srv-docs", Name: "docs", Type: "static_site", EnvironmentID: env},
	{ID: "dpg-db", Name: "db", Type: KindPostgres, EnvironmentID: env},
	{ID: "red-kv", Name: "cache", Type: KindKeyValue},
}

func scopeIDs(rs []Resource) string {
	var out []string
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return strings.Join(out, ",")
}

func TestScopeSelectsWhatAViewReads(t *testing.T) {
	filter := func(field, op string, values ...string) Document {
		return Document{Filters: []Filter{{Field: field, Operator: op, Values: values}}, Version: Version}
	}
	for _, tc := range []struct {
		name     string
		viewType string
		doc      Document
		want     string
	}{
		{"application logs read every service, cron job and datastore, never a static site", TypeApplicationLogs, Default(),
			"srv-web,srv-legacy,srv-private,srv-worker,srv-cron,dpg-db,red-kv"},
		{"request logs read web services only", TypeRequestLogs, Default(), "srv-web,srv-legacy"},
		{"an environment narrows to its resources", TypeApplicationLogs, filter(FieldEnvironment, OpIs, env),
			"srv-web,srv-private,srv-cron,dpg-db"},
		{"the legacy env- spelling names the same environment", TypeApplicationLogs, filter(FieldEnvironment, OpIs, "env-c185th5c2rvvnhbfiltg"),
			"srv-web,srv-private,srv-cron,dpg-db"},
		{"resource IS", TypeApplicationLogs, filter(FieldResource, OpIs, "srv-cron", "red-kv"), "srv-cron,red-kv"},
		{"resource IS_NOT", TypeApplicationLogs, filter(FieldResource, OpIsNot, "srv-cron", "red-kv"),
			"srv-web,srv-legacy,srv-private,srv-worker,dpg-db"},
		{"resource type IS", TypeApplicationLogs, filter(FieldResourceType, OpIs, ResourceTypePostgres, ResourceTypeKeyValue), "dpg-db,red-kv"},
		{"resource type IS_NOT", TypeApplicationLogs, filter(FieldResourceType, OpIsNot, ResourceTypeService), "srv-cron,dpg-db,red-kv"},
		{"filters combine", TypeApplicationLogs, Document{Version: Version, Filters: []Filter{
			{Field: FieldEnvironment, Operator: OpIs, Values: []string{env}},
			{Field: FieldResourceType, Operator: OpIs, Values: []string{ResourceTypeService, ResourceTypeCronJob}},
		}}, "srv-web,srv-private,srv-cron"},
		{"request logs keep their own rule under a filter", TypeRequestLogs, filter(FieldResourceType, OpIs, ResourceTypeService), "srv-web,srv-legacy"},
		{"service metrics read every resource, a static site as a Service", TypeMetrics, Default(),
			"srv-web,srv-legacy,srv-private,srv-worker,srv-cron,srv-docs,dpg-db,red-kv"},
		{"a metrics view's Services include its static sites", TypeMetrics, filter(FieldResourceType, OpIs, ResourceTypeService),
			"srv-web,srv-legacy,srv-private,srv-worker,srv-docs"},
		{"a metrics view narrows to an environment", TypeMetrics, filter(FieldEnvironment, OpIs, env),
			"srv-web,srv-private,srv-cron,srv-docs,dpg-db"},
	} {
		if got := scopeIDs(tc.doc.Scope(tc.viewType, projectResources)); got != tc.want {
			t.Errorf("%s: scope = %s, want %s", tc.name, got, tc.want)
		}
	}
	if got := Default().Scope(TypeApplicationLogs, nil); got == nil || len(got) != 0 {
		t.Errorf("an empty project's scope = %#v, want an empty list", got)
	}
}
