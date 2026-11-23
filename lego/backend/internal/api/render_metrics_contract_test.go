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

package api

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w1/m155: every path under Render's /metrics, with each of its documented
// query parameters, walked through the composed server and the strict request
// validator. The matrix is generated from the pinned spec, so a new Render
// path or parameter fails here until it has a verdict. At filing time REST
// silently ignored aggregateBy and eight Render /metrics paths were bare 404s.

// renderMetricsNonGoals are Render /metrics paths bex deliberately does not
// serve (.pm/DO_NOT_DO.md: workflows).
var renderMetricsNonGoals = map[string]string{
	"/metrics/task-runs-queued":    "workflows are a non-goal (DO_NOT_DO.md)",
	"/metrics/task-runs-completed": "workflows are a non-goal (DO_NOT_DO.md)",
}

const (
	metricsParamServed = "served"
)

// renderMetricsParamVerdicts records, per Render /metrics path, what each
// documented query parameter does: served (it changes the read), refused (a
// coded 400/501), or ignored with the reason.
var renderMetricsParamVerdicts = map[string]map[string]string{
	"/metrics/active-connections": {"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed},
	"/metrics/bandwidth":          {"startTime": metricsParamServed, "endTime": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed},
	"/metrics/bandwidth-sources": {
		"startTime": "served: hourly points; an earlier start than 2025-03-09 is a 400", "endTime": metricsParamServed,
		"resource": metricsParamServed, "service": metricsParamServed,
	},
	"/metrics/cpu": {
		"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed,
		"instance": metricsParamServed, "aggregationMethod": "served: AVG; MAX/MIN refused with a 400 (metrics-server snapshots)",
	},
	"/metrics/cpu-limit": {"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed, "instance": metricsParamServed},
	"/metrics/cpu-target": {
		"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed,
		"instance": "refused: a target is App configuration with no instance axis",
	},
	"/metrics/disk-capacity": {"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed},
	"/metrics/disk-usage":    {"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed},
	"/metrics/filters/application": {
		"startTime": metricsParamServed, "endTime": metricsParamServed,
		"resolutionSeconds": "ignored: instance filter discovery reads pod label values over the window rather than stepped buckets",
		"resource":          metricsParamServed, "service": metricsParamServed,
	},
	"/metrics/filters/http": {
		"startTime": metricsParamServed, "endTime": metricsParamServed,
		"resolutionSeconds": "ignored: HTTP filter discovery reads label values and access-log streams over the window rather than stepped buckets",
		"resource":          metricsParamServed, "service": metricsParamServed,
		"host": metricsParamServed, "statusCode": metricsParamServed,
	},
	"/metrics/filters/path": {
		"startTime": metricsParamServed, "endTime": metricsParamServed,
		"resolutionSeconds": "ignored: path suggestions scan raw access-log lines rather than stepped buckets",
		"resource":          metricsParamServed, "service": metricsParamServed,
		"host": metricsParamServed, "statusCode": metricsParamServed, "path": metricsParamServed,
	},
	"/metrics/http-latency": {
		"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed,
		"host": metricsParamServed, "path": metricsParamServed, "quantile": metricsParamServed,
	},
	"/metrics/http-requests": {
		"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed,
		"host": metricsParamServed, "path": metricsParamServed, "aggregateBy": "served: statusCode; host refused with a 400 (no host axis)",
	},
	"/metrics/instance-count": {"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed},
	"/metrics/memory":         {"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed, "instance": metricsParamServed},
	"/metrics/memory-limit":   {"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed, "instance": metricsParamServed},
	"/metrics/memory-target": {
		"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed, "service": metricsParamServed,
		"instance": "refused: a target is App configuration with no instance axis",
	},
	"/metrics/replication-lag": {"startTime": metricsParamServed, "endTime": metricsParamServed, "resolutionSeconds": metricsParamServed, "resource": metricsParamServed},
}

var renderMetricsParamSamples = map[string]string{
	"startTime": "2026-09-14T19:00:00Z", "endTime": "2026-09-14T20:00:00Z", "resolutionSeconds": "60",
	"resource": "srv-c185th5c2rvvnhbfiltg", "service": "srv-c185th5c2rvvnhbfiltg",
	"instance": "srv-c185th5c2rvvnhbfiltg-abc12", "host": "web.onbex.co", "path": "/qa", "quantile": "0.95",
	"aggregateBy": "statusCode", "aggregationMethod": "AVG", "statusCode": "200",
}

// renderMetricsDatastorePaths take a datastore resource; the rest take a service.
var renderMetricsDatastorePaths = map[string]bool{
	"/metrics/active-connections": true, "/metrics/disk-capacity": true,
	"/metrics/disk-usage": true, "/metrics/replication-lag": true,
}

func TestEveryRenderMetricsPathAndParameterHasAVerdict(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	h, _ := serverWith(t, &core.Base{
		Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"client-1": "tea-cli"},
	}, Deps{APIKeys: newFakeKeyStore()})

	var paths []string
	for path := range contract.document.Paths.Map() {
		if strings.HasPrefix(path, "/metrics/") {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		t.Fatal("the pinned Render spec has no /metrics paths")
	}

	for _, path := range paths {
		if reason, ok := renderMetricsNonGoals[path]; ok {
			t.Logf("%s: recorded non-goal (%s)", path, reason)
			continue
		}
		item := contract.document.Paths.Find(path)
		verdicts, ok := renderMetricsParamVerdicts[path]
		if !ok {
			t.Errorf("%s: no verdict table — serve it, refuse it with a code, or record it as a non-goal", path)
			continue
		}
		specParams := map[string]bool{}
		for _, ref := range append(item.Parameters, item.Get.Parameters...) {
			if ref.Value != nil && ref.Value.In == "query" {
				specParams[ref.Value.Name] = true
			}
		}
		for name := range specParams {
			verdict, ok := verdicts[name]
			switch {
			case !ok:
				t.Errorf("%s: Render parameter %q has no verdict (served, refused, or ignored with a reason)", path, name)
			case strings.HasPrefix(verdict, "ignored") && !strings.Contains(verdict, ": "):
				t.Errorf("%s: %q is ignored without a reason", path, name)
			}
		}
		for name := range verdicts {
			if !specParams[name] {
				t.Errorf("%s: verdict for %q, which Render's spec does not document", path, name)
			}
		}

		// Through the composed server: the path is never a bare mux 404, and no
		// documented parameter is refused as unknown.
		resource := renderMetricsParamSamples["resource"]
		if renderMetricsDatastorePaths[path] {
			resource = "dpg-c185th5c2rvvnhbfiltg"
		}
		for name := range specParams {
			q := url.Values{"resource": {resource}}
			switch name {
			case "resource":
			case "service":
				q = url.Values{"service": {resource}}
			default:
				q.Set(name, renderMetricsParamSamples[name])
			}
			target := "/v1" + path + "?" + q.Encode()
			rec := do(t, h, http.MethodGet, target, testToken, "")
			body := rec.Body.String()
			if rec.Code == http.StatusNotFound && strings.HasPrefix(body, "404 page not found") {
				t.Errorf("GET %s = bare 404", target)
			}
			if strings.Contains(body, "unsupported query parameter") {
				t.Errorf("GET %s refused Render's documented %q as unsupported", target, name)
			}
			// The verdict must match what the server does. The sample resource does
			// not exist, so a served or ignored parameter reaches the verb's 404 —
			// never a 400 or 501 — while a refused one is a 4xx or 501.
			refusal := rec.Code == http.StatusBadRequest || rec.Code == http.StatusNotImplemented
			switch verdict := verdicts[name]; {
			case strings.HasPrefix(verdict, "refused"):
				if rec.Code < 400 || (rec.Code >= 500 && rec.Code != http.StatusNotImplemented) {
					t.Errorf("GET %s = %d, but %q is recorded as refused", target, rec.Code, name)
				}
			case refusal:
				t.Errorf("GET %s = %d %s, but %q is recorded as %q", target, rec.Code, body, name, verdict)
			}
		}
	}
}

// w8/351: Render types resolutionSeconds as a number of at least 30 on every
// /metrics path, and its pinned client sends a float32. Through the composed
// server, a decimal or exponent spelling clears the strict validator and the
// handler alike (the sample resource does not exist, so it reaches the verb's
// 404), and a refusal never carries a Go parser message.
func TestRenderMetricsResolutionSecondsIsANumber(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	h, _ := serverWith(t, &core.Base{
		Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"client-1": "tea-cli"},
	}, Deps{APIKeys: newFakeKeyStore()})

	checked := 0
	for path := range contract.document.Paths.Map() {
		item := contract.document.Paths.Find(path)
		if _, nonGoal := renderMetricsNonGoals[path]; nonGoal || !strings.HasPrefix(path, "/metrics/") || item.Get == nil {
			continue
		}
		if item.Get.Parameters.GetByInAndName("query", "resolutionSeconds") == nil && item.Parameters.GetByInAndName("query", "resolutionSeconds") == nil {
			continue
		}
		checked++
		resource := renderMetricsParamSamples["resource"]
		if renderMetricsDatastorePaths[path] {
			resource = "dpg-c185th5c2rvvnhbfiltg"
		}
		for _, value := range []string{"60.0", "1e2", "90.5", "abc", "29"} {
			target := "/v1" + path + "?" + url.Values{"resource": {resource}, "resolutionSeconds": {value}}.Encode()
			rec := do(t, h, http.MethodGet, target, testToken, "")
			body := rec.Body.String()
			if strings.Contains(body, "strconv") {
				t.Errorf("GET %s leaked a Go parser message: %s", target, body)
			}
			if refused := rec.Code == http.StatusBadRequest; refused != (value == "abc" || value == "29") {
				t.Errorf("GET %s = %d %s", target, rec.Code, body)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no Render /metrics path declares resolutionSeconds")
	}
}
