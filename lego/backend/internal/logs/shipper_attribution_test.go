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

// shipper_attribution_test.go is the guard w6/m131 owed: it reads the ACTUAL
// Traefik ServiceName regex out of deploy/gitops/base/log-shipper.yaml and
// asserts it attributes a request line to the exact (namespace, app) pair
// bex-api's LogQL selector queries by — for a per-tenant `tea-<xid>` namespace
// (ADR043), not just the shared `default`. The original regex was anchored to
// the literal `default`, so every tenant access line missed the regex, had its
// `app` left empty, and was dropped as not_a_tenant_app — leaving type=request
// silently empty for every service in production. That is exactly the failure
// this test now fails on: the request pipeline's attribution regex is the one
// piece of the shipper that RECONSTRUCTS the namespace/app from the access line
// instead of reading them from pod metadata, so it is the one piece that can
// (and did) drift from the App CR's real placement.
//
// It reads the deployed config rather than a copy, so the two cannot diverge.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// shipperServiceNameRegex reads log-shipper.yaml, extracts the Traefik
// access-log ServiceName regex (the only `expression =` line that parses an
// `@kubernetes` service name), un-escapes the River string, and compiles it.
func shipperServiceNameRegex(t *testing.T) *regexp.Regexp {
	t.Helper()
	path := findRepoFile(t, filepath.Join("deploy", "gitops", "base", "log-shipper.yaml"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var exprLine string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "expression =") && strings.Contains(trimmed, "@kubernetes") {
			exprLine = trimmed
			break
		}
	}
	if exprLine == "" {
		t.Fatal("could not find the Traefik ServiceName `expression =` line in log-shipper.yaml")
	}
	// A stray Helm double-brace action inside this line would break the whole
	// ConfigMap render (see the HELM TPL ESCAPING note in the file); the regex
	// is plain and must carry none.
	if strings.Contains(exprLine, "{{") || strings.Contains(exprLine, "}}") {
		t.Fatalf("ServiceName regex line carries a raw Helm double-brace, which breaks chart render: %s", exprLine)
	}
	// Pull the double-quoted River string and undo River's `\\` escaping so RE2
	// sees the real pattern (`\\d` in the source is the regex `\d`).
	first := strings.IndexByte(exprLine, '"')
	last := strings.LastIndexByte(exprLine, '"')
	if first < 0 || last <= first {
		t.Fatalf("could not extract the quoted expression from: %s", exprLine)
	}
	pattern := strings.ReplaceAll(exprLine[first+1:last], `\\`, `\`)
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("shipper ServiceName regex does not compile as RE2 (%q): %v", pattern, err)
	}
	return re
}

// findRepoFile walks up from the test's working directory until it finds rel,
// so the test is independent of how deep the package sits under the repo root.
func findRepoFile(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate %s walking up from the test directory", rel)
		}
		dir = parent
	}
}

// attribute runs the shipper regex over a Traefik ServiceName and returns the
// namespace/app it would stamp — the mechanics of the pipeline's stage.regex.
func attribute(re *regexp.Regexp, serviceName string) (namespace, app string, matched bool) {
	m := re.FindStringSubmatch(serviceName)
	if m == nil {
		return "", "", false
	}
	for i, name := range re.SubexpNames() {
		switch name {
		case "namespace":
			namespace = m[i]
		case "app":
			app = m[i]
		}
	}
	return namespace, app, true
}

func TestShipperRegexAttributesTenantNamespaceServices(t *testing.T) {
	re := shipperServiceNameRegex(t)

	// A real tenant (ADR043): the App lives in namespace `tea-<xid>` and its CR
	// name — which is also the k8s Service name Traefik reports — is itself
	// tenant-prefixed (core.CRName == "<tenant>-<name>"). bex-api queries request
	// logs with namespace=app.Namespace and app=app.Name, so the regex must
	// recover exactly those two values from the doubly-prefixed ServiceName.
	const tenant = "tea-d98210cbbpdc73dcrkvg" // the fixture workspace from the m131 hunt
	cases := []struct {
		name        string
		crName      string // == app.Name, the Service segment
		port        string
		wantAppName string
	}{
		{"simple", tenant + "-web", "8080", tenant + "-web"},
		{"hyphenated app", tenant + "-hello-go", "3000", tenant + "-hello-go"},
		{"numeric-suffixed app", tenant + "-qa-20260828-logs", "8080", tenant + "-qa-20260828-logs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serviceName := tenant + "-" + tc.crName + "-" + tc.port + "@kubernetes"
			ns, app, ok := attribute(re, serviceName)
			if !ok {
				t.Fatalf("tenant ServiceName %q was NOT matched — its request lines would be dropped as not_a_tenant_app (the m131 bug)", serviceName)
			}
			if ns != tenant {
				t.Errorf("namespace = %q, want %q (must equal app.Namespace so the query selector matches)", ns, tenant)
			}
			if app != tc.wantAppName {
				t.Errorf("app = %q, want %q (must equal app.Name so the query selector matches)", app, tc.wantAppName)
			}
		})
	}
}

func TestShipperRegexStillAttributesDefaultNamespace(t *testing.T) {
	re := shipperServiceNameRegex(t)

	// The shared/storeless namespace path must not regress: a `default`-namespace
	// App keeps attributing exactly as before this milestone.
	ns, app, ok := attribute(re, "default-web-8080@kubernetes")
	if !ok {
		t.Fatal("default-namespace ServiceName was not matched")
	}
	if ns != "default" || app != "web" {
		t.Errorf("default attribution = (%q, %q), want (default, web)", ns, app)
	}
}

func TestShipperRegexRejectsNonTenantServices(t *testing.T) {
	re := shipperServiceNameRegex(t)

	// A non-tenant edge service (a platform/system Ingress, or Traefik's own
	// dashboard) has no App to attribute to and must NOT match — matching would
	// mint a bogus request stream. It is dropped as not_a_tenant_app instead.
	for _, svc := range []string{
		"monitoring-loki-3100@kubernetes",
		"kube-system-traefik-dashboard-9000@kubernetes",
		"bex-system-bex-api-8090@kubernetes",
		"teapot-web-8080@kubernetes", // starts with "tea" but is not a tea-<xid> namespace
	} {
		if _, _, ok := attribute(re, svc); ok {
			t.Errorf("non-tenant ServiceName %q matched the tenant attribution regex; it should be dropped", svc)
		}
	}
}
