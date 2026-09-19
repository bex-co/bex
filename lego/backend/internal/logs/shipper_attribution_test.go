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
//
// The same read-the-real-config pattern also guards the request pipeline's
// bounded RequestHost allowlist (w4/m88, extended to the platform edge hosts
// by w5/053 per ADR088 §6): the should_drop / namespace / platform_service
// stage.template blocks are extracted out of log-shipper.yaml, un-escaped
// (Helm tpl + River quoting), and EXECUTED as the Go templates Alloy's
// pipeline engine runs — asserting each allowlisted host is retained under
// its fixed namespace/service pair and that an unlisted host still drops.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
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

// shipperPipelineTemplate reads log-shipper.yaml, finds the stage.template
// block whose `source` is name, and recovers the real template text the way it
// reaches Alloy: Helm's tpl pass renders the self-evaluating escape actions to
// literal double-braces (the HELM TPL ESCAPING note in the file), and River
// unquotes `\"` inside its double-quoted string. Parsing the result with Go's
// text/template mirrors Alloy's pipeline-stage engine — these templates use
// only the builtin eq/or, no Sprig functions.
func shipperPipelineTemplate(t *testing.T, name string) *template.Template {
	t.Helper()
	path := findRepoFile(t, filepath.Join("deploy", "gitops", "base", "log-shipper.yaml"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sourceLine := regexp.MustCompile(`^source\s*=\s*"` + regexp.QuoteMeta(name) + `"$`)
	lines := strings.Split(string(data), "\n")
	var tmplLine string
	// A source name is not unique across stage kinds — "app" is both this
	// pipeline's stage.template source and a stage.drop source in the build
	// pipeline — so keep scanning until a block with a template line is found
	// rather than committing to the first match.
	for i, line := range lines {
		if !sourceLine.MatchString(strings.TrimSpace(line)) {
			continue
		}
		for _, next := range lines[i+1:] {
			trimmed := strings.TrimSpace(next)
			if strings.HasPrefix(trimmed, "template =") {
				tmplLine = trimmed
				break
			}
			if trimmed == "}" { // end of the stage block — no template line
				break
			}
		}
		if tmplLine != "" {
			break
		}
	}
	if tmplLine == "" {
		t.Fatalf("could not find the stage.template block for source %q in log-shipper.yaml", name)
	}
	first := strings.IndexByte(tmplLine, '"')
	last := strings.LastIndexByte(tmplLine, '"')
	if first < 0 || last <= first {
		t.Fatalf("could not extract the quoted template from: %s", tmplLine)
	}
	text := tmplLine[first+1 : last]
	// Helm tpl: `{{ "{{" }}` / `{{ "}}" }}` render to the literal braces.
	text = strings.ReplaceAll(text, `{{ "{{" }}`, "{{")
	text = strings.ReplaceAll(text, `{{ "}}" }}`, "}}")
	if strings.Contains(text, "{{ \"") {
		t.Fatalf("template for %q carries an unrecognized Helm escape after un-escaping: %s", name, text)
	}
	// River unquoting: `\"` inside the double-quoted string is a `"`.
	text = strings.ReplaceAll(text, `\"`, `"`)
	tmpl, err := template.New(name).Parse(text)
	if err != nil {
		t.Fatalf("template for %q does not parse as a Go template (%q): %v", name, text, err)
	}
	return tmpl
}

// renderStage executes a pipeline template against the extracted-field map the
// way Alloy's stage.template does: `.Value` is the source field's current
// value, every other key is a previously extracted field. map[string]string
// models Alloy's extracted map for these stages (all string-typed); a missing
// key reads as "", i.e. "not extracted".
func renderStage(t *testing.T, tmpl *template.Template, data map[string]string) string {
	t.Helper()
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("execute %s template over %v: %v", tmpl.Name(), data, err)
	}
	return out.String()
}

func TestShipperHostAllowlistRetainsPlatformEdgeHosts(t *testing.T) {
	shouldDrop := shipperPipelineTemplate(t, "should_drop")
	namespace := shipperPipelineTemplate(t, "namespace")
	service := shipperPipelineTemplate(t, "platform_service")

	// The full w4/m88 + w5/053 allowlist: each host is retained under a FIXED
	// bounded namespace/service pair — the pods actually serving it — so the
	// obs availability dashboard's 5xx log panel can explain a platform-API
	// outage (ADR088 §6). `host` itself stays line-only (see
	// TestPathAndHostNeverBecomeLabels for the query side).
	cases := []struct {
		host          string
		wantNamespace string
		wantService   string
	}{
		{"dashboard.bex.co", "dashboard", "dashboard"}, // w4/m88
		{"api.bex.co", "bex-system", "bex-api"},        // w5/053
		{"oauth.bex.co", "auth", "hydra"},              // w5/053
		{"auth.bex.co", "auth", "kratos"},              // w5/053
		{"obs.bex.co", "monitoring", "grafana"},        // w5/053
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			// A platform edge line: the ServiceName regex missed, so `app`
			// normalized to "" and only the host allowlist can retain it.
			line := map[string]string{"app": "", "host": tc.host}
			if got := renderStage(t, shouldDrop, line); got != "no" {
				t.Fatalf("should_drop(%s) = %q, want \"no\" — the line would be dropped and the availability dashboard's log panel stays empty during an outage", tc.host, got)
			}
			line["Value"] = "" // the namespace stage's source is empty on a regex miss
			if got := renderStage(t, namespace, line); got != tc.wantNamespace {
				t.Errorf("namespace(%s) = %q, want %q", tc.host, got, tc.wantNamespace)
			}
			if got := renderStage(t, service, line); got != tc.wantService {
				t.Errorf("service(%s) = %q, want %q", tc.host, got, tc.wantService)
			}
		})
	}
}

func TestShipperHostAllowlistDropsUnknownHosts(t *testing.T) {
	shouldDrop := shipperPipelineTemplate(t, "should_drop")

	// Drop-not-guess: any host NOT on the explicit allowlist must still be
	// dropped when the line has no App attribution — never retained under a
	// guessed label. Includes near misses (tool-named grafana.bex.co, a
	// tenant host whose ServiceName regex failed) and the empty host.
	for _, host := range []string{
		"grafana.bex.co",  // the tool name; the role-named host is obs.bex.co
		"ssh.bex.co",      // real platform host, deliberately not allowlisted
		"web.onbex.co",    // tenant host — retention comes from ServiceName, not host
		"api.bex.co.evil", // prefix-collision probe
		"example.com",
		"",
	} {
		got := renderStage(t, shouldDrop, map[string]string{"app": "", "host": host})
		if got != "yes" {
			t.Errorf("should_drop(host=%q, no app) = %q, want \"yes\" (drop) — an unlisted host must never mint a request stream", host, got)
		}
	}
}

func TestShipperHostAllowlistKeepsTenantAttributionFirst(t *testing.T) {
	shouldDrop := shipperPipelineTemplate(t, "should_drop")
	namespace := shipperPipelineTemplate(t, "namespace")
	service := shipperPipelineTemplate(t, "platform_service")

	// A tenant-attributed line (regex matched, `app` non-empty) keeps its
	// regex-derived namespace/app and gets NO service label (empty values are
	// omitted by Alloy's stage.labels) — regardless of what host it was
	// served on.
	line := map[string]string{
		"app":   "tea-d98210cbbpdc73dcrkvg-web",
		"host":  "api.bex.co", // even a host on the allowlist must not override attribution
		"Value": "tea-d98210cbbpdc73dcrkvg",
	}
	if got := renderStage(t, shouldDrop, line); got != "no" {
		t.Fatalf("should_drop(tenant line) = %q, want \"no\"", got)
	}
	if got := renderStage(t, namespace, line); got != "tea-d98210cbbpdc73dcrkvg" {
		t.Errorf("namespace(tenant line) = %q, want the regex-derived namespace", got)
	}
	if got := renderStage(t, service, line); got != "" {
		t.Errorf("service(tenant line) = %q, want \"\" (no service label on tenant streams)", got)
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

// TestShipperBuildPodsKeepPredeploy pins w5/m100: the Alloy build_pods pipeline
// must keep component=predeploy as well as component=build so migration stdout
// lands in Loki as type=build (Render-compatible CLI reachability). Reading the
// live ConfigMap source prevents the keep regex from silently narrowing again.
func TestShipperBuildPodsKeepPredeploy(t *testing.T) {
	path := findRepoFile(t, filepath.Join("deploy", "gitops", "base", "log-shipper.yaml"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(data)
	idx := strings.Index(body, `discovery.relabel "build_pods"`)
	if idx < 0 {
		t.Fatal(`missing discovery.relabel "build_pods" in log-shipper.yaml`)
	}
	// Bound the block loosely to the next discovery.relabel so we assert the
	// keep rule that belongs to build_pods, not a later pipeline.
	rest := body[idx:]
	if next := strings.Index(rest[1:], `discovery.relabel "`); next >= 0 {
		rest = rest[:next+1]
	}
	if !strings.Contains(rest, `regex         = "build|predeploy"`) {
		t.Fatalf("build_pods keep regex must be build|predeploy (w5/m100); block:\n%s", rest)
	}
	if !strings.Contains(rest, `__meta_kubernetes_pod_label_app_bex_co_predeploy`) {
		t.Fatal("build_pods must derive app from app.bex.co/predeploy when build label is absent")
	}
}

// --- w4/m113: a static site's request lines reach its own App ---
//
// The two JSON objects below are verbatim production Traefik access lines,
// captured 2026-09-19 from the deployed traefik pods (fields trimmed to the
// ones the pipeline reads). They are the ground truth the w6/m131 post-mortem
// says this guard must be written against, for both shapes at once — the whole
// class of bug here is a reconstruction that drifts from what Traefik actually
// emits.
//
// Static: ADR029 points every static host's Ingress at the SHARED static
// server through a per-App ExternalName alias named `bex-static-<app>`, so the
// ServiceName regex recovers an app name no App has. `type=request` was
// therefore empty for every static site while router-attributed bandwidth for
// the same traffic was correct.
const (
	prodStaticServiceName  = "default-bex-static-hello-static-8080@kubernetes"
	prodStaticIngressName  = "hello-static"
	prodComputeServiceName = "tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-eden-cms-v2-3000@kubernetes"
	prodComputeIngressName = "tea-d98210cbbpdc73dcrkvg-eden-cms-v2"
)

// shipperAppLabel runs the real pipeline over a captured line: the ServiceName
// regex, then the `app` stage.template that overrides it — exactly the two
// stages in log-shipper.yaml, in order.
func shipperAppLabel(t *testing.T, serviceName, ingressName string) (namespace, app string) {
	t.Helper()
	re := shipperServiceNameRegex(t)
	appTmpl := shipperPipelineTemplate(t, "app")
	namespace, regexApp, _ := attribute(re, serviceName)
	return namespace, renderStage(t, appTmpl, map[string]string{
		"Value": regexApp, "ingress_name": ingressName,
	})
}

func TestShipperAttributesStaticSiteRequestLines(t *testing.T) {
	ns, app := shipperAppLabel(t, prodStaticServiceName, prodStaticIngressName)
	if ns != "default" {
		t.Errorf("namespace = %q, want default (app.Namespace)", ns)
	}
	// The value bex-api's LogQL selector pins — app.Name, NOT the alias
	// Service's `bex-static-` name.
	if app != prodStaticIngressName {
		t.Fatalf("app = %q, want %q — a static site's request lines would be labeled with a name no App has, so type=request stays empty for it", app, prodStaticIngressName)
	}
	// The control: the pre-fix pipeline (ServiceName regex alone) produced the
	// wrong name, which is why this test fails against it.
	re := shipperServiceNameRegex(t)
	if _, legacy, _ := attribute(re, prodStaticServiceName); legacy != "bex-static-"+prodStaticIngressName {
		t.Errorf("control: ServiceName regex alone = %q, want the bex-static-prefixed alias name", legacy)
	}
}

func TestShipperComputeAttributionIsUnchanged(t *testing.T) {
	ns, app := shipperAppLabel(t, prodComputeServiceName, prodComputeIngressName)
	if ns != "tea-d98210cbbpdc73dcrkvg" || app != prodComputeIngressName {
		t.Fatalf("compute attribution = (%q, %q), want (tea-d98210cbbpdc73dcrkvg, %q)", ns, app, prodComputeIngressName)
	}
	// On a compute line the two sources agree exactly, so the override is a
	// no-op there — the property that makes it safe to apply unconditionally.
	re := shipperServiceNameRegex(t)
	if _, regexApp, _ := attribute(re, prodComputeServiceName); regexApp != prodComputeIngressName {
		t.Errorf("ServiceName regex (%q) and KubernetesIngressName (%q) must agree for compute", regexApp, prodComputeIngressName)
	}
}

// A platform edge line carries a KubernetesIngressName too. It must NOT become
// an `app`: doing so would route the line down the tenant branch of the
// namespace/platform_service templates and lose its fixed platform pair, which
// is what the w4/m88 + w5/053 allowlist exists to preserve.
func TestShipperIngressNameNeverAttributesAPlatformEdgeLine(t *testing.T) {
	// bex-system is neither tea-<xid> nor default, so the regex misses and
	// leaves `app` empty — the override's gate.
	ns, app := shipperAppLabel(t, "bex-system-bex-api-80@kubernetes", "bex-api")
	if ns != "" || app != "" {
		t.Fatalf("platform edge attribution = (%q, %q), want both empty so the host allowlist owns the line", ns, app)
	}
}

// A line with no Ingress fields at all (Traefik's own 404 at the edge, or a
// non-Ingress router) falls back to the ServiceName regex unchanged.
func TestShipperFallsBackToServiceNameWithoutAnIngressName(t *testing.T) {
	_, app := shipperAppLabel(t, "default-web-8080@kubernetes", "")
	if app != "web" {
		t.Fatalf("app = %q, want web from the ServiceName regex fallback", app)
	}
}
