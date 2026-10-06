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
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// access_log_fields_test.go guards the Traefik access-log field allowlist
// (w5/076). A tenant's request log is the raw JSON line (ADR010), so the
// allowlist in deploy/gitops/base/values/traefik.values.yaml decides what a
// tenant reads: a field a Traefik upgrade adds stays dropped until reviewed,
// and a field something reads must stay kept, or the shipper stops
// attributing lines and bex-api's path, host and latency reads go empty.

const traefikValuesPath = "../../../../deploy/gitops/base/values/traefik.values.yaml"

// tenantAccessLogFields is the reviewed allowlist: every field tenants saw
// before the allowlist (Traefik v3.7's core fields, its Kubernetes Ingress ones
// and entryPointName, less the platform internals below). Adding a field to the
// values file means adding it here, which is the review.
var tenantAccessLogFields = []string{
	"ClientAddr", "ClientHost", "ClientPort", "ClientUsername",
	"DownstreamContentSize", "DownstreamStatus", "Duration", "GzipRatio",
	"KubernetesIngressName", "KubernetesIngressNamespace",
	"KubernetesServiceName", "KubernetesServicePort",
	"OriginContentSize", "OriginDuration", "OriginStatus", "Overhead",
	"RequestAddr", "RequestContentSize", "RequestHost", "RequestMethod",
	"RequestPath", "RequestPort", "RequestProtocol", "RequestScheme",
	"RetryAttempts", "ServiceName", "StartLocal", "StartUTC",
	"TLSCipher", "TLSClientSubject", "TLSVersion", "entryPointName",
}

// platformAccessLogFields must never reach a tenant (w8/047): the process-wide
// edge request counter, pod IPs, and the edge's internal routing name.
var platformAccessLogFields = []string{"RequestCount", "RouterName", "ServiceAddr", "ServiceURL"}

func TestAccessLogFieldsAreAReviewedAllowlist(t *testing.T) {
	fields := traefikAccessLogFields(t)
	if fields.DefaultMode != "drop" {
		t.Fatalf("accessLog.fields.defaultMode = %q; a field Traefik adds would reach tenants unreviewed", fields.DefaultMode)
	}
	var kept []string
	for name, mode := range fields.Names {
		if mode == "keep" {
			kept = append(kept, name)
		}
	}
	slices.Sort(kept)
	want := slices.Sorted(slices.Values(tenantAccessLogFields))
	if !slices.Equal(kept, want) {
		t.Errorf("kept access-log fields\n got %v\nwant %v (update tenantAccessLogFields only after reviewing what a tenant would read)", kept, want)
	}
	for _, name := range platformAccessLogFields {
		if fields.Names[name] != "drop" {
			t.Errorf("platform-internal field %s is %q, want an explicit drop", name, fields.Names[name])
		}
	}
}

// TestAccessLogFieldsKeepEveryConsumedField scans the access log's readers:
// the shipper's request pipeline (its JSON expressions and its access-line
// filter) and bex-api's LogQL json stages for request logs and request metrics.
func TestAccessLogFieldsKeepEveryConsumedField(t *testing.T) {
	names := traefikAccessLogFields(t).Names
	consumers := map[string]*regexp.Regexp{
		// stage.json expressions (`host = "RequestHost"`) and the line filter
		// that keeps only access lines (`!= \"ServiceName\"`).
		"../../../../deploy/gitops/base/log-shipper.yaml": regexp.MustCompile(`=\s*\\?"([A-Z][A-Za-z]+)\\?"`),
		// LogQL json extractions (`request_path="RequestPath"`).
		"loki.go":                  regexp.MustCompile(`[a-z_]+="([A-Z][A-Za-z]+)"`),
		"../metrics/lokisource.go": regexp.MustCompile(`[a-z_]+="([A-Z][A-Za-z]+)"`),
	}
	consumed := map[string]string{}
	for path, re := range consumers {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(src)
		if strings.HasSuffix(path, "log-shipper.yaml") {
			text = requestPipeline(t, text)
		}
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			consumed[m[1]] = path
		}
	}
	// The readers known today; fewer means a scan path or pattern went stale.
	for _, field := range []string{"ServiceName", "KubernetesIngressName", "RequestMethod", "DownstreamStatus", "RequestHost", "RequestPath", "Duration"} {
		if _, ok := consumed[field]; !ok {
			t.Errorf("the scan found no reader of %s; its scan path or pattern is stale", field)
		}
	}
	for _, field := range slices.Sorted(maps.Keys(consumed)) {
		if names[field] != "keep" {
			t.Errorf("%s reads %s, which the access log does not keep", consumed[field], field)
		}
	}
}

type accessLogFields struct {
	DefaultMode string            `yaml:"defaultMode"`
	Names       map[string]string `yaml:"names"`
}

func traefikAccessLogFields(t *testing.T) accessLogFields {
	t.Helper()
	src, err := os.ReadFile(traefikValuesPath)
	if err != nil {
		t.Fatalf("read %s: %v", traefikValuesPath, err)
	}
	var values struct {
		AccessLog struct {
			Fields accessLogFields `yaml:"fields"`
		} `yaml:"accessLog"`
	}
	if err := yaml.Unmarshal(src, &values); err != nil {
		t.Fatalf("parse %s: %v", traefikValuesPath, err)
	}
	return values.AccessLog.Fields
}

// requestPipeline returns the shipper's `loki.process "request_logs"` block,
// the only one that parses Traefik access lines.
func requestPipeline(t *testing.T, shipper string) string {
	t.Helper()
	start := strings.Index(shipper, `loki.process "request_logs"`)
	if start < 0 {
		t.Fatal(`log-shipper.yaml has no loki.process "request_logs" block`)
	}
	block := shipper[start:]
	end := strings.Index(block, "forward_to")
	if end < 0 {
		t.Fatal(`loki.process "request_logs" has no forward_to`)
	}
	return block[:end]
}
