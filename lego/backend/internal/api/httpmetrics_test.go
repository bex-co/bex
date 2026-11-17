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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bex-co/bex/lego/backend/internal/clitelemetry"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// meteredServer builds the same stack cmd/api serves: the composed product
// handler with the request-telemetry middleware as the outermost wrapper.
func meteredServer(t *testing.T, base *core.Base, d Deps) (http.Handler, *prometheus.Registry, *Server) {
	t.Helper()
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	srv := NewServer(base, d)
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.OriginMetrics = metrics
	return metrics.Middleware(buildHandler(t, srv)), reg, srv
}

// sample is one observed metric: its family and its label set.
type sample struct {
	family string
	labels map[string]string
}

func (s sample) String() string {
	keys := make([]string, 0, len(s.labels))
	for k := range s.labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, s.labels[k]))
	}
	return s.family + "{" + strings.Join(parts, ",") + "}"
}

func gatherSamples(t *testing.T, reg *prometheus.Registry) []sample {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var out []sample
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, pair := range metric.GetLabel() {
				labels[pair.GetName()] = pair.GetValue()
			}
			out = append(out, sample{family: family.GetName(), labels: labels})
		}
	}
	return out
}

// labelValues returns the distinct values of one label across one family.
func labelValues(samples []sample, family, label string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range samples {
		if s.family != family {
			continue
		}
		if v, ok := s.labels[label]; ok && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func hasSample(samples []sample, family string, labels map[string]string) bool {
	for _, s := range samples {
		if s.family != family {
			continue
		}
		match := true
		for k, v := range labels {
			if s.labels[k] != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestOriginMetricsFamiliesAppearOnAScrape pins the exact family names ADR088 §6
// and the alert rules read. A rename here silently empties every origin panel,
// so the names are asserted against the real /metrics text exposition.
func TestOriginMetricsFamiliesAppearOnAScrape(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}
	h, reg, srv := meteredServer(t, base, Deps{})

	do(t, h, http.MethodGet, "/v1/services", testToken, "")
	gql(t, h, `{ services { id name } }`)
	cs := mcpSession(t, srv)
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_services"}); err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	w := httptest.NewRecorder()
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/metrics = %d", w.Code)
	}
	scrape := w.Body.String()
	for _, family := range []string{
		"bex_api_http_request_duration_seconds_bucket",
		"bex_api_http_request_duration_seconds_count",
		"bex_api_http_request_duration_seconds_sum",
		"bex_api_http_requests_total",
		"bex_api_http_in_flight_requests",
		"bex_api_graphql_operation_duration_seconds",
		"bex_api_mcp_tool_duration_seconds",
		"bex_api_cli_requests_total",
		"bex_api_cli_telemetry_events_total",
	} {
		if !strings.Contains(scrape, family) {
			t.Errorf("scrape is missing %s", family)
		}
	}
	// The in-flight gauge must return to zero: a leaked Inc would make the
	// saturation panel climb forever.
	if !strings.Contains(scrape, `bex_api_http_in_flight_requests{surface="rest"} 0`) {
		t.Errorf("in-flight gauge did not return to zero:\n%s", scrape)
	}
}

// TestOriginMetricsTelemetrySeries pins the exposition of the CLI usage
// counter (w5/m92, ADR113 D8): stored events by completion kind, output format
// and client, the names the alert rules and the CLI usage board read.
func TestOriginMetricsTelemetrySeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewOriginMetrics(reg)
	m.ObserveTelemetry("success", "json", clitelemetry.ClientBex)
	m.ObserveTelemetry("execution_error", "text", clitelemetry.ClientRenderCLI)

	scrape := scrapeText(t, reg)
	// NB: exposition sorts label names alphabetically, not in declaration order.
	for _, series := range []string{
		`bex_api_cli_telemetry_events_total{client="bex",completion_kind="success",output_format="json"} 1`,
		`bex_api_cli_telemetry_events_total{client="render-cli",completion_kind="execution_error",output_format="text"} 1`,
		`bex_api_cli_telemetry_events_total{client="bex",completion_kind="execution_error",output_format="text"} 0`,
	} {
		if !strings.Contains(scrape, series) {
			t.Errorf("scrape is missing %s", series)
		}
	}
	// Nil-safe: metrics-off ingest must not panic.
	var off *OriginMetrics
	off.ObserveTelemetry("success", "json", clitelemetry.ClientBex)
}

// TestOriginMetricsExportCLISeriesAtZero: a series that first appears at 1 is
// invisible to increase(), and on production every rollout started both
// replicas empty, so the first stored events of each new pod never reached a
// rate (w3/m438). Every series the CLI telemetry alerts read exists at 0
// before anything happens.
func TestOriginMetricsExportCLISeriesAtZero(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewOriginMetrics(reg)
	m.Middleware(http.NotFoundHandler())

	scrape := scrapeText(t, reg)
	for _, client := range clitelemetry.Clients {
		if want := `bex_api_cli_requests_total{client="` + client + `"} 0`; !strings.Contains(scrape, want) {
			t.Errorf("scrape is missing %s", want)
		}
		for _, kind := range clitelemetry.CompletionKinds {
			for _, format := range clitelemetry.OutputFormats {
				want := fmt.Sprintf(`bex_api_cli_telemetry_events_total{client=%q,completion_kind=%q,output_format=%q} 0`, client, kind, format)
				if !strings.Contains(scrape, want) {
					t.Errorf("scrape is missing %s", want)
				}
			}
		}
	}
	for _, status := range []string{"202", "429", "500", "503"} {
		want := `bex_api_http_requests_total{method="POST",route="POST /v1/cli-telemetry-events",status="` + status + `",surface="rest"} 0`
		if !strings.Contains(scrape, want) {
			t.Errorf("scrape is missing %s", want)
		}
	}
	// The internal listener does not serve the ingest route, so it creates
	// none of its series.
	internal := prometheus.NewRegistry()
	NewOriginMetrics(internal).InternalMiddleware(http.NotFoundHandler())
	if got := scrapeText(t, internal); strings.Contains(got, "cli-telemetry-events") {
		t.Errorf("the internal listener created ingest route series:\n%s", got)
	}
}

// TestOriginMetricsCountsCLIRequestsByClient pins ADR113 D6: the REST requests
// the CLI sends, split by whether the bex launcher sent them, are the
// denominator the stored telemetry is read against. Anything else, and the
// telemetry deliveries themselves, are not counted.
func TestOriginMetricsCountsCLIRequestsByClient(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}
	h, reg, _ := meteredServer(t, base, Deps{})
	const cliUA = "render-cli/2.36.0 (macOS - 15.1)"
	send := func(method, path, userAgent, bexVersion, body string) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+testToken)
		r.Header.Set("User-Agent", userAgent)
		if bexVersion != "" {
			r.Header.Set(clitelemetry.BexVersionHeader, bexVersion)
		}
		h.ServeHTTP(httptest.NewRecorder(), r)
	}

	// Counted: the launcher (prefix and release header) and an unmodified
	// Render CLI (prefix alone), whatever the answer — an unrouted path is
	// still a CLI request.
	send(http.MethodGet, "/v1/services", cliUA, "0.3.0", "")
	send(http.MethodPost, "/v1/services/web/restart", cliUA, "0.3.0", "")
	send(http.MethodGet, "/v1/services", cliUA, "", "")
	send(http.MethodGet, "/v1/does-not-exist", cliUA, "", "")

	// Not counted.
	send(http.MethodGet, "/v1/services", "Mozilla/5.0 (Macintosh)", "", "")
	send(http.MethodGet, "/v1/services", "", "0.3.0", "")
	send(http.MethodGet, "/v1/services", "curl/8.4 render-cli/2.36.0", "", "")
	send(http.MethodPost, "/graphql", cliUA, "0.3.0", `{"query":"{ services { id } }"}`)
	send(http.MethodPost, "/mcp", cliUA, "0.3.0", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	send(http.MethodPost, "/v1/device-grant", cliUA, "0.3.0", `{}`)
	send(http.MethodPost, "/v1/cli-telemetry-events", cliUA, "0.3.0", `{"command":"services"}`)
	send(http.MethodPost, "/v1/cli-telemetry-events", cliUA, "", `{"command":"services"}`)

	for client, want := range map[string]float64{clitelemetry.ClientBex: 2, clitelemetry.ClientRenderCLI: 2} {
		if got := counterValue(t, reg, "bex_api_cli_requests_total", map[string]string{"client": client}); got != want {
			t.Errorf("cli requests{client=%q} = %v, want %v", client, got, want)
		}
	}
	if got := labelValues(gatherSamples(t, reg), "bex_api_cli_requests_total", "client"); len(got) != 2 {
		t.Errorf("client label values = %v, want exactly the two clients", got)
	}
}

// TestCLITelemetryCounterHoldsOnlyClosedSeries is ADR113 D8's acceptance test:
// fifty deliveries, each with a different garbage command, version, kind and
// format, are all stored, and the counter still holds only the series its
// closed label sets allow — no value from a body became a label.
func TestCLITelemetryCounterHoldsOnlyClosedSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	st := &recordingTelemetryStore{}
	svc := &clitelemetry.Service{Base: &core.Base{}, Store: st, Metrics: metrics}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for i := range 50 {
		body, _ := json.Marshal(map[string]any{
			"command":         fmt.Sprintf("services list --garbage-%d", i),
			"cli_version":     fmt.Sprintf("9.9.%d-evil", i),
			"completion_kind": fmt.Sprintf("kind_%d", i),
			"output_format":   fmt.Sprintf("format_%d", i),
		})
		r := httptest.NewRequest(http.MethodPost, "/v1/cli-telemetry-events", strings.NewReader(string(body)))
		if i%2 == 0 {
			r.Header.Set(clitelemetry.BexVersionHeader, fmt.Sprintf("0.%d.0", i))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusAccepted {
			t.Fatalf("delivery %d = %d: %s", i, w.Code, w.Body)
		}
	}

	if len(st.rows) != 50 {
		t.Fatalf("stored rows = %d, want 50", len(st.rows))
	}
	const family = "bex_api_cli_telemetry_events_total"
	samples := gatherSamples(t, reg)
	var series int
	for _, s := range samples {
		if s.family != family {
			continue
		}
		series++
		if len(s.labels) != 3 {
			t.Errorf("series %v has labels beyond completion_kind, output_format and client", s)
		}
		if !slices.Contains(clitelemetry.CompletionKinds, s.labels["completion_kind"]) ||
			!slices.Contains(clitelemetry.OutputFormats, s.labels["output_format"]) ||
			!slices.Contains(clitelemetry.Clients, s.labels["client"]) {
			t.Errorf("series %v carries a value outside the closed sets", s)
		}
	}
	if want := len(clitelemetry.CompletionKinds) * len(clitelemetry.OutputFormats) * len(clitelemetry.Clients); series != want {
		t.Errorf("counter holds %d series, want the %d the closed sets allow", series, want)
	}
	for client, want := range map[string]float64{clitelemetry.ClientBex: 25, clitelemetry.ClientRenderCLI: 25} {
		labels := map[string]string{"completion_kind": clitelemetry.CompletionKindOther, "output_format": clitelemetry.OutputFormatUnknown, "client": client}
		if got := counterValue(t, reg, family, labels); got != want {
			t.Errorf("%s%v = %v, want %v", family, labels, got, want)
		}
	}
}

type recordingTelemetryStore struct{ rows []store.CLITelemetryEvent }

func (r *recordingTelemetryStore) InsertCLITelemetryEvent(_ context.Context, ev store.CLITelemetryEvent) error {
	r.rows = append(r.rows, ev)
	return nil
}

func scrapeText(t *testing.T, reg *prometheus.Registry) string {
	t.Helper()
	w := httptest.NewRecorder()
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/metrics = %d", w.Code)
	}
	return w.Body.String()
}

// TestOriginMetricsRouteLabelIsAPatternNeverAnId is the cardinality guard: ids
// are the one thing that turns a bounded histogram into an unbounded memory
// leak, so every route label must be a registered mux pattern.
func TestOriginMetricsRouteLabelIsAPatternNeverAnId(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}
	h, reg, _ := meteredServer(t, base, Deps{})

	for _, path := range []string{
		"/v1/services/srv-d1n0abcdefgh",
		"/v1/services/srv-d1n0abcdefgh/deploys",
		"/v1/services/srv-d1n0abcdefgh/env-vars",
		"/v1/postgres/dpg-d1n0abcdefgh",
		"/v1/key-value/red-d1n0abcdefgh",
		"/v1/owners/tea-d1n0abcdefgh",
		"/v1/webhooks/endpoints/whk-d1n0abcdefgh",
		"/v1/agent-sessions/ags-d1n0abcdefgh",
	} {
		do(t, h, http.MethodGet, path, testToken, "")
	}

	samples := gatherSamples(t, reg)
	idShaped := regexp.MustCompile(`(^|[/_-])(srv|dpg|red|tea|ags|whk|dep|key|env|job|prj)-[a-z0-9]{6,}`)
	for _, s := range samples {
		if !strings.HasPrefix(s.family, "bex_api_") {
			continue
		}
		for label, value := range s.labels {
			if idShaped.MatchString(value) {
				t.Errorf("id-shaped label value on %s: %s=%q", s.family, label, value)
			}
		}
	}
	routes := labelValues(samples, "bex_api_http_requests_total", "route")
	if len(routes) == 0 {
		t.Fatal("no route labels recorded")
	}
	for _, route := range routes {
		if route == routeUnmatched {
			continue
		}
		// A registered pattern is "[METHOD ]/literal/{wildcard}/…": every
		// non-literal segment is braced, so a bare id segment cannot appear.
		for _, segment := range strings.Split(strings.TrimPrefix(route, "GET "), "/") {
			if strings.ContainsAny(segment, "{}") || segment == "" {
				continue
			}
			if strings.Contains(segment, "-") && strings.ContainsAny(segment, "0123456789") {
				t.Errorf("route %q carries what looks like a value segment %q", route, segment)
			}
		}
	}
	t.Logf("route labels: %v", routes)
}

// TestOriginMetricsFoldsUnroutedAndUnknownMethods keeps a path scanner from
// minting one series per probe.
func TestOriginMetricsFoldsUnroutedAndUnknownMethods(t *testing.T) {
	h, reg, _ := meteredServer(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})

	for _, path := range []string{"/nope", "/wp-login.php", "/v1/does-not-exist"} {
		do(t, h, http.MethodGet, path, testToken, "")
	}
	r := httptest.NewRequest("PROPFIND", "/v1/services", nil)
	r.Header.Set("Authorization", "Bearer "+testToken)
	h.ServeHTTP(httptest.NewRecorder(), r)

	samples := gatherSamples(t, reg)
	routes := labelValues(samples, "bex_api_http_requests_total", "route")
	for _, route := range routes {
		if strings.Contains(route, "nope") || strings.Contains(route, "wp-login") {
			t.Errorf("unrouted probe minted its own route label %q", route)
		}
	}
	if !hasSample(samples, "bex_api_http_requests_total", map[string]string{"route": routeUnmatched}) {
		t.Errorf("unrouted requests were not folded into %q (routes: %v)", routeUnmatched, routes)
	}
	methods := labelValues(samples, "bex_api_http_requests_total", "method")
	for _, method := range methods {
		if method == "PROPFIND" {
			t.Errorf("unregistered method became a label value (methods: %v)", methods)
		}
	}
	if !hasSample(samples, "bex_api_http_requests_total", map[string]string{"method": methodOther}) {
		t.Errorf("unregistered method was not folded into %q (methods: %v)", methodOther, methods)
	}
}

// TestOriginMetricsSurfaceClassification proves the axis both origin alert rules
// group by: an auth-server or MCP fault must not read as a REST regression.
func TestOriginMetricsSurfaceClassification(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	srv := NewServer(base, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.OAuthIssuer = "https://auth.example.test"
	srv.OAuthResource = "https://api.example.test"
	srv.OriginMetrics = metrics
	h := metrics.Middleware(buildHandler(t, srv))

	do(t, h, http.MethodGet, "/v1/services", testToken, "")
	do(t, h, http.MethodPost, "/graphql", testToken, `{"query":"{ services { id } }"}`)
	do(t, h, http.MethodPost, "/mcp", testToken, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	do(t, h, http.MethodGet, "/.well-known/oauth-protected-resource", "", "")
	do(t, h, http.MethodGet, "/healthz", "", "")

	samples := gatherSamples(t, reg)
	for surface, route := range map[string]string{
		surfaceREST:     "GET /v1/services",
		surfaceGraphQL:  "POST /graphql",
		surfaceMCP:      "/mcp",
		surfaceAuth:     "GET /.well-known/oauth-protected-resource",
		surfaceInternal: "GET /healthz",
	} {
		if !hasSample(samples, "bex_api_http_requests_total", map[string]string{"surface": surface}) {
			t.Errorf("no %q surface series (samples: %v)", surface, samples)
			continue
		}
		if !hasSample(samples, "bex_api_http_requests_total", map[string]string{"surface": surface, "route": route}) {
			t.Errorf("surface %q did not record route %q (routes: %v)",
				surface, route, labelValues(samples, "bex_api_http_requests_total", "route"))
		}
	}
	if got := labelValues(samples, "bex_api_http_requests_total", "surface"); len(got) != 5 {
		t.Errorf("surface label values = %v, want exactly the five closed values", got)
	}
}

// TestOriginMetricsInternalListenerIsOneSurface — the :8091 mux is metered whole
// so a projection or mint verb never lands in the public SLIs.
func TestOriginMetricsInternalListenerIsOneSurface(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	internalRoot := http.NewServeMux()
	internalRoot.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	internalRoot.HandleFunc("/v1/apps/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := metrics.InternalMiddleware(internalRoot)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/apps/app-1", nil))

	samples := gatherSamples(t, reg)
	if got := labelValues(samples, "bex_api_http_requests_total", "surface"); len(got) != 1 || got[0] != surfaceInternal {
		t.Errorf("internal listener surfaces = %v, want [%s]", got, surfaceInternal)
	}
	if !hasSample(samples, "bex_api_http_requests_total", map[string]string{"route": "/v1/apps/{id}"}) {
		t.Errorf("internal route was not labelled by its pattern (routes: %v)",
			labelValues(samples, "bex_api_http_requests_total", "route"))
	}
}

// TestOriginMetricsGraphQLOperationDimensions: GraphQL always answers 200, so
// the operation histogram is the only place a broken mutation is visible.
func TestOriginMetricsGraphQLOperationDimensions(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}
	h, reg, _ := meteredServer(t, base, Deps{})

	gql(t, h, `{ services { id name } }`)
	// An operation the schema does not have: the label folds to "other" instead
	// of letting a client name its own series.
	do(t, h, http.MethodPost, "/graphql", testToken, `{"query":"query Nope { notAField }"}`)
	// A named mutation on a service that does not exist — a real error class.
	do(t, h, http.MethodPost, "/graphql", testToken,
		`{"query":"mutation R { restartServer(id: \"srv-missing\") { id } }","operationName":"R"}`)

	samples := gatherSamples(t, reg)
	const family = "bex_api_graphql_operation_duration_seconds"
	if !hasSample(samples, family, map[string]string{"operation": "services", "type": gqlTypeQuery, "outcome": gqlOutcomeOK}) {
		t.Errorf("no ok series for the services query (samples: %v)", samples)
	}
	if !hasSample(samples, family, map[string]string{"operation": gqlOperationOther}) {
		t.Errorf("unknown operation did not fold to %q (operations: %v)",
			gqlOperationOther, labelValues(samples, family, "operation"))
	}
	if !hasSample(samples, family, map[string]string{"operation": "restartServer", "type": gqlTypeMutation}) {
		t.Errorf("mutation dimensions missing (operations: %v, types: %v)",
			labelValues(samples, family, "operation"), labelValues(samples, family, "type"))
	}
	for _, outcome := range labelValues(samples, family, "outcome") {
		switch outcome {
		case gqlOutcomeOK, gqlOutcomeInvalid, gqlOutcomeDenied, gqlOutcomeNotFound, gqlOutcomeConflict,
			gqlOutcomePayment, gqlOutcomeUnavailable, gqlOutcomeRejected, gqlOutcomeInternal:
		default:
			t.Errorf("outcome %q is outside the closed vocabulary", outcome)
		}
	}
}

// TestOriginMetricsMCPToolSeriesAreRegisteredNamesOnly: agent traffic is
// observable per tool, and a made-up tool name cannot create a series.
func TestOriginMetricsMCPToolSeriesAreRegisteredNamesOnly(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	srv := NewServer(base, Deps{})
	srv.OriginMetrics = metrics
	cs := mcpSession(t, srv)
	ctx := context.Background()

	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_services"}); err != nil {
		t.Fatalf("list_services: %v", err)
	}
	// A tool nobody registered: the SDK answers "unknown tool" and no series
	// may appear — otherwise the label set is whatever a caller types.
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "definitely_not_a_tool"}); err == nil {
		t.Fatal("unknown tool call succeeded, want an SDK error")
	}
	// A workspace the caller cannot reach is a DENIAL, not a fault.
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_service", Arguments: map[string]any{"serviceId": "web", "workspaceId": "tea-nonmember"},
	}); err != nil {
		t.Fatalf("get_service: %v", err)
	}

	samples := gatherSamples(t, reg)
	const family = "bex_api_mcp_tool_duration_seconds"
	if !hasSample(samples, family, map[string]string{"tool": "list_services", "outcome": mcpOutcomeOK}) {
		t.Errorf("no ok series for list_services (tools: %v)", labelValues(samples, family, "tool"))
	}
	for _, tool := range labelValues(samples, family, "tool") {
		if !registeredMCPTool(tool) {
			t.Errorf("series for unregistered tool %q", tool)
		}
	}
	for _, outcome := range labelValues(samples, family, "outcome") {
		switch outcome {
		case mcpOutcomeOK, mcpOutcomeError, mcpOutcomeDenied:
		default:
			t.Errorf("outcome %q is outside the closed vocabulary", outcome)
		}
	}
}

// TestOriginMetricsMarksDeniedToolCalls — the workspace/scope gate renders its
// refusal as an error RESULT (SetError keeps only text), so the class has to be
// marked where it is known or every denial would read as a tool fault.
func TestOriginMetricsMarksDeniedToolCalls(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	middleware := mcpMetricsMiddleware(metrics)
	handler := middleware(func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return mcpCallError(ctx, fmt.Errorf("cannot access workspace: %w", core.ErrForbidden)), nil
	})
	call := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "list_services"}}
	if _, err := handler(context.Background(), mcpCallToolMethod, call); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !hasSample(gatherSamples(t, reg), "bex_api_mcp_tool_duration_seconds",
		map[string]string{"tool": "list_services", "outcome": mcpOutcomeDenied}) {
		t.Errorf("a forbidden workspace was not recorded as %q", mcpOutcomeDenied)
	}
}

// TestOriginMetricsExcludesStreamsFromTheHistogram: an SSE tail that is healthy
// for an hour would otherwise own every latency bucket and hide the p95 of the
// unary requests the SLI is about. It is still counted.
func TestOriginMetricsExcludesStreamsFromTheHistogram(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stream", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("streaming handler lost http.Flusher through the metrics wrapper")
			return
		}
		_, _ = w.Write([]byte("data: one\n\n"))
		flusher.Flush()
	})
	mux.HandleFunc("GET /unary", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	h := metrics.InternalMiddleware(mux)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/stream", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/unary", nil))

	samples := gatherSamples(t, reg)
	if !hasSample(samples, "bex_api_http_requests_total", map[string]string{"route": "GET /stream"}) {
		t.Error("the stream was not counted in bex_api_http_requests_total")
	}
	if hasSample(samples, "bex_api_http_request_duration_seconds", map[string]string{"route": "GET /stream"}) {
		t.Error("the stream was observed into the duration histogram")
	}
	if !hasSample(samples, "bex_api_http_request_duration_seconds", map[string]string{"route": "GET /unary"}) {
		t.Error("the unary request was not observed into the duration histogram")
	}
}

// TestStatusWriterKeepsHijackerAndReaderFrom — the Web Shell/agent upgrade paths
// and sendfile responses must survive the wrapper, exactly as they do through
// gzip.go's.
func TestStatusWriterKeepsHijackerAndReaderFrom(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /upgrade", func(w http.ResponseWriter, _ *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("handler lost http.Hijacker through the metrics wrapper")
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("Hijack: %v", err)
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n\r\n"))
		_ = conn.Close()
	})
	// The middleware records a request only after its handler returns, so wait
	// for the middleware, not the hijack: reading samples once the handler had
	// closed the connection flaked a deploy (w4/210).
	metered := metrics.InternalMiddleware(mux)
	recorded := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metered.ServeHTTP(w, r)
		recorded <- struct{}{}
	}))
	defer server.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if _, err := fmt.Fprintf(conn, "GET /upgrade HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_ = conn.Close()
	if !strings.Contains(status, "101") {
		t.Fatalf("upgrade response = %q, want 101", status)
	}
	<-recorded

	samples := gatherSamples(t, reg)
	if !hasSample(samples, "bex_api_http_requests_total",
		map[string]string{"route": "GET /upgrade", "status": "101"}) {
		t.Errorf("hijacked upgrade was not counted as 101 (samples: %v)", samples)
	}
	if hasSample(samples, "bex_api_http_request_duration_seconds", map[string]string{"route": "GET /upgrade"}) {
		t.Error("a hijacked upgrade was observed into the duration histogram")
	}
}

// TestOriginMetricsCountsShedRequests — the auth gate's 401s and the body cap's
// 413s ARE the origin's error picture; a middleware mounted inside them would
// report a healthy API while every client was being refused.
func TestOriginMetricsCountsShedRequests(t *testing.T) {
	base := &core.Base{Client: fakeClient(), Namespace: "default"}
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	srv := NewServer(base, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.OriginMetrics = metrics
	h := metrics.Middleware(buildHandler(t, srv))

	do(t, h, http.MethodGet, "/v1/services", "", "")

	if !hasSample(gatherSamples(t, reg), "bex_api_http_requests_total",
		map[string]string{"surface": surfaceREST, "status": "401"}) {
		t.Error("an unauthenticated request was not counted")
	}
}

// TestGraphQLOperationDimsAreBounded exercises the label derivation directly,
// including the shapes a hostile client would try.
func TestGraphQLOperationDimsAreBounded(t *testing.T) {
	for _, tc := range []struct {
		name, query, operationName string
		wantOp, wantType           string
	}{
		{name: "anonymous query", query: `{ services { id } }`, wantOp: "services", wantType: gqlTypeQuery},
		{name: "named mutation", query: `mutation M { restartServer(id: "x") { id } }`, operationName: "M",
			wantOp: "restartServer", wantType: gqlTypeMutation},
		{name: "unknown field", query: `{ whateverIWant }`, wantOp: gqlOperationOther, wantType: gqlTypeQuery},
		{name: "unparseable", query: `{{{`, wantOp: gqlOperationOther, wantType: gqlTypeQuery},
		{name: "introspection only", query: `{ __schema { types { name } } }`, wantOp: gqlOperationOther, wantType: gqlTypeQuery},
		{name: "empty", query: ``, wantOp: gqlOperationOther, wantType: gqlTypeQuery},
		{name: "names an absent operation", query: `query A { services { id } }`, operationName: "B",
			wantOp: gqlOperationOther, wantType: gqlTypeQuery},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotOp, gotType := graphqlOperationDims(tc.query, tc.operationName)
			if gotOp != tc.wantOp || gotType != tc.wantType {
				t.Errorf("graphqlOperationDims = (%q, %q), want (%q, %q)", gotOp, gotType, tc.wantOp, tc.wantType)
			}
		})
	}
}

// TestGraphQLOutcomeClassesMatchRESTClassification keeps the two surfaces
// telling the same story about the same failure.
func TestGraphQLOutcomeClassesMatchRESTClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{err: nil, want: gqlOutcomeOK},
		{err: core.ErrForbidden, want: gqlOutcomeDenied},
		{err: fmt.Errorf("wrapped: %w", core.ErrNotFound), want: gqlOutcomeNotFound},
		{err: core.ErrBadRequest, want: gqlOutcomeInvalid},
		{err: core.ErrConflict, want: gqlOutcomeConflict},
		{err: core.ErrPaymentRequired, want: gqlOutcomePayment},
		{err: core.ErrUnavailable, want: gqlOutcomeUnavailable},
		{err: fmt.Errorf("pq: relation \"apps\" does not exist"), want: gqlOutcomeInternal},
	} {
		if got := graphqlOutcome(tc.err); got != tc.want {
			t.Errorf("graphqlOutcome(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

// TestOriginMetricsAreOptional — every observation point tolerates an unwired
// registry, so stdio mode and the existing tests keep running untouched.
func TestOriginMetricsAreOptional(t *testing.T) {
	var metrics *OriginMetrics
	metrics.observeGraphQL("services", gqlTypeQuery, gqlOutcomeOK, 0)
	metrics.observeMCPTool("list_services", mcpOutcomeOK, 0)
	handler := metrics.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("unmetered handler = %d", w.Code)
	}
}
