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
	"cmp"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// OriginMetrics is bex-api's own request telemetry — the ORIGIN half of the
// picture whose only half used to be Traefik's edge counters (docs/ADR088 §6
// recorded that as a known gap; w3/m84 closes it). With both halves, an API
// incident can be attributed: an origin-side 2xx under an edge-side error is
// the edge/LB, an origin-side 5xx is bex-api.
//
// Every label is bounded by construction, because a metric label is the one
// place a tenant string turns into permanent server memory:
//
//   - surface: rest | graphql | mcp | auth | internal (closed set below).
//   - route: the REGISTERED mux PATTERN the request matched ("GET
//     /v1/services/{serviceId}"), never the raw path — so no srv-…/dpg-…/tea-…
//     id can ever become a series. Nothing matched => "unmatched", which is also
//     what a path scanner collapses into.
//   - method: bounded to the HTTP methods bex registers; anything else folds to
//     "other" (a client may send any valid token).
//   - status: the response code as written.
//   - GraphQL operation: the document's first top-level field, and only when the
//     scope-matrix table (classifiedOps — CI-generated from the schema) knows it;
//     everything else, including an anonymous or unparseable document, is
//     "other". type is query/mutation/subscription, outcome the closed
//     classification below.
//   - MCP tool: the REGISTERED tool name only — an unknown tool creates no
//     series at all.
//
// Streaming responses (SSE log tails, MCP SSE, hijacked upgrades) are counted in
// http_requests_total but deliberately NOT observed into the duration histogram:
// a tail that is healthy for an hour would otherwise dominate every bucket and
// make the p95 of ordinary unary requests unreadable. Their latency question is
// "did it start and stay up", answered by the in-flight gauge and the counter.
type OriginMetrics struct {
	duration *prometheus.HistogramVec
	requests *prometheus.CounterVec
	inFlight *prometheus.GaugeVec
	graphql  *prometheus.HistogramVec
	mcpTools *prometheus.HistogramVec
}

// Request surfaces — the coarse "which product surface" axis every panel and
// both origin alert rules group by.
const (
	surfaceREST     = "rest"
	surfaceGraphQL  = "graphql"
	surfaceMCP      = "mcp"
	surfaceAuth     = "auth"
	surfaceInternal = "internal"
)

// routeUnmatched is the single series every unrouted request (404s, scanners)
// collapses into, so probing cannot inflate cardinality one path at a time.
const routeUnmatched = "unmatched"

// methodOther bounds the method axis: net/http accepts any RFC-token method, so
// an unregistered one folds here instead of minting a series.
const methodOther = "other"

// GraphQL outcome classes. GraphQL answers 200 with an `errors` array, so the
// HTTP status says nothing about whether the operation worked. The vocabulary is
// the sentinel classes core.WriteErr maps to statuses (extensions.code itself is
// open-ended — every new *_LIMIT code would be a new label value), plus "ok",
// "rejected" for a coded error carrying no sentinel, and "internal" for an
// unclassified resolver failure.
const (
	gqlOutcomeOK          = "ok"
	gqlOutcomeInvalid     = "invalid"
	gqlOutcomeDenied      = "denied"
	gqlOutcomeNotFound    = "not_found"
	gqlOutcomeConflict    = "conflict"
	gqlOutcomePayment     = "payment_required"
	gqlOutcomeUnavailable = "unavailable"
	gqlOutcomeRejected    = "rejected"
	gqlOutcomeInternal    = "internal"
)

// GraphQL operation dimensions: the label for a document whose top-level field
// isn't in the schema's operation table, and the three operation types.
const (
	gqlOperationOther   = "other"
	gqlTypeQuery        = "query"
	gqlTypeMutation     = "mutation"
	gqlTypeSubscription = "subscription"
)

// MCP tool outcomes: a denial (workspace/scope/authz) is separated from a fault
// because they page different people.
const (
	mcpOutcomeOK     = "ok"
	mcpOutcomeError  = "error"
	mcpOutcomeDenied = "denied"
)

// originDurationBuckets is API-shaped: dense where a REST/GraphQL call should
// live (5ms–1s), thinning to the 30s GraphQL execution bound so a pathological
// document still lands in a real bucket rather than +Inf.
var originDurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

// registeredMethods is the closed method axis (every method bex-api registers a
// route for).
var registeredMethods = map[string]struct{}{
	http.MethodGet: {}, http.MethodHead: {}, http.MethodPost: {}, http.MethodPut: {},
	http.MethodPatch: {}, http.MethodDelete: {}, http.MethodOptions: {},
}

// NewOriginMetrics registers the families on reg. One instance is shared by the
// public :8090 listener, the internal :8091 listener, and the GraphQL/MCP
// observation points, so all of bex-api's request telemetry lands on the one
// registry /metrics serves.
func NewOriginMetrics(reg prometheus.Registerer) *OriginMetrics {
	m := &OriginMetrics{
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "api", Name: "http_request_duration_seconds",
			Help:    "bex-api origin request duration by surface, registered route pattern, method, and status. Streaming responses are excluded (counted in bex_api_http_requests_total).",
			Buckets: originDurationBuckets,
		}, []string{"surface", "route", "method", "status"}),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "api", Name: "http_requests_total",
			Help: "bex-api origin requests completed by surface, registered route pattern, method, and status (includes streaming responses).",
		}, []string{"surface", "route", "method", "status"}),
		inFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "api", Name: "http_in_flight_requests",
			Help: "bex-api requests currently being served, by surface.",
		}, []string{"surface"}),
		graphql: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "api", Name: "graphql_operation_duration_seconds",
			Help:    "GraphQL document handling duration by schema operation, operation type, and outcome class.",
			Buckets: originDurationBuckets,
		}, []string{"operation", "type", "outcome"}),
		mcpTools: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "api", Name: "mcp_tool_duration_seconds",
			Help:    "MCP tool call duration by registered tool name and outcome (ok/error/denied).",
			Buckets: originDurationBuckets,
		}, []string{"tool", "outcome"}),
	}
	reg.MustRegister(m.duration, m.requests, m.inFlight, m.graphql, m.mcpTools)
	return m
}

// Middleware meters the public listener, deriving the surface from the request
// path. It is the OUTERMOST wrapper on that listener so a request shed by the
// auth gate, a rate limiter, or the body cap is still counted — those rejections
// are exactly what an origin error-ratio panel must show.
func (m *OriginMetrics) Middleware(next http.Handler) http.Handler {
	return m.middleware(publicSurface, next)
}

// InternalMiddleware meters the cluster-internal control-plane listener (:8091).
// Its whole mux is one surface: nothing there is a product API, and the
// projection/mint/ops-role verbs must never be averaged into the public SLIs.
func (m *OriginMetrics) InternalMiddleware(next http.Handler) http.Handler {
	return m.middleware(func(*http.Request) string { return surfaceInternal }, next)
}

func (m *OriginMetrics) middleware(surfaceOf func(*http.Request) string, next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		surface := surfaceOf(r)
		// The recorder travels in the CONTEXT, not on the request struct: the
		// auth gate and several handlers clone the request (r.WithContext /
		// r.Clone), so an inner mux sets r.Pattern on a copy this middleware
		// never sees. A pointer in the context survives every clone, and
		// recordRoutePattern writes the pattern of each mux the request
		// descends through — innermost last, which is the most specific.
		route := &routeRecorder{}
		r = r.WithContext(withRouteRecorder(r.Context(), route))
		sw := &statusWriter{ResponseWriter: w}
		m.inFlight.WithLabelValues(surface).Inc()
		start := time.Now()
		next.ServeHTTP(sw, r)
		elapsed := time.Since(start)
		m.inFlight.WithLabelValues(surface).Dec()
		// Nothing recorded means no inner mux was descended: fall back to this
		// middleware's OWN request pattern (the listener's top-level mux mutates
		// the very request we hold) for routes mounted there directly, e.g.
		// GET /readyz.
		pattern := cmp.Or(route.pattern, r.Pattern, routeUnmatched)
		status := strconv.Itoa(sw.statusCode())
		method := requestMethod(r.Method)
		m.requests.WithLabelValues(surface, pattern, method, status).Inc()
		if !sw.streamed {
			m.duration.WithLabelValues(surface, pattern, method, status).Observe(elapsed.Seconds())
		}
	})
}

// observeGraphQL records one document's handling. A nil receiver (metrics not
// wired — tests, stdio mode) is a no-op.
func (m *OriginMetrics) observeGraphQL(operation, opType, outcome string, d time.Duration) {
	if m == nil {
		return
	}
	m.graphql.WithLabelValues(operation, opType, outcome).Observe(d.Seconds())
}

func (m *OriginMetrics) observeMCPTool(tool, outcome string, d time.Duration) {
	if m == nil {
		return
	}
	m.mcpTools.WithLabelValues(tool, outcome).Observe(d.Seconds())
}

// publicSurface classifies a request on the public listener. Path-based rather
// than pattern-based so a 404 still lands on the surface it was aimed at.
func publicSurface(r *http.Request) string {
	path := r.URL.Path
	switch {
	// OAuth 2.1 discovery + the Render CLI's credential-less device-flow
	// protocol routes: token plumbing, not product traffic, so an auth-server
	// outage doesn't read as a REST regression.
	case strings.HasPrefix(path, "/.well-known/"),
		path == "/v1/device-grant", path == "/v1/device-token",
		strings.HasPrefix(path, "/v1/token/"):
		return surfaceAuth
	case path == "/graphql":
		return surfaceGraphQL
	case path == "/mcp" || strings.HasPrefix(path, "/mcp/"):
		return surfaceMCP
	case strings.HasPrefix(path, restMountPattern):
		return surfaceREST
	default:
		// /healthz, /readyz — lifecycle endpoints, not the product surface.
		return surfaceInternal
	}
}

func requestMethod(method string) string {
	if _, ok := registeredMethods[method]; ok {
		return method
	}
	return methodOther
}

// routeRecorder carries the matched pattern from whichever mux matched it out to
// the metrics middleware. Written once per mux the request descends through
// (last write wins — the innermost mux is the most specific), read after the
// handler returns; single-goroutine per request.
type routeRecorder struct{ pattern string }

type routeRecorderKey struct{}

func withRouteRecorder(ctx context.Context, rec *routeRecorder) context.Context {
	return context.WithValue(ctx, routeRecorderKey{}, rec)
}

func routeRecorderFrom(ctx context.Context) *routeRecorder {
	rec, _ := ctx.Value(routeRecorderKey{}).(*routeRecorder)
	return rec
}

func (r *routeRecorder) record(pattern string) {
	if r != nil && pattern != "" {
		r.pattern = pattern
	}
}

// recordRoutePattern notes which of mux's registered patterns a request matches
// before handing it on, so the metrics middleware labels the request with the
// pattern rather than its raw path. mux.Handler is used instead of reading
// r.Pattern afterwards because the wrappers in between clone the request.
// Costs one extra match, and only while a request is actually being metered.
func recordRoutePattern(mux *http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rec := routeRecorderFrom(r.Context()); rec != nil {
			_, pattern := mux.Handler(r)
			rec.record(cmp.Or(pattern, routeUnmatched))
		}
		next.ServeHTTP(w, r)
	})
}

// statusWriter captures the status code and whether the response streamed, and
// forwards every optional ResponseWriter capability the handlers below it rely
// on — Flusher (SSE per-event flush), Hijacker (WebSocket upgrade), ReaderFrom
// (sendfile), and Unwrap (http.ResponseController). Same contract as the gzip
// wrapper in gzip.go; losing any of them would silently buffer a live log tail.
type statusWriter struct {
	http.ResponseWriter
	code     int
	streamed bool
	hijacked bool
}

func (s *statusWriter) WriteHeader(code int) {
	if s.code == 0 {
		s.code = code
		// An event stream is a long-lived response whose duration measures the
		// client's attention span, not the server's work.
		s.streamed = strings.HasPrefix(s.Header().Get("Content-Type"), "text/event-stream")
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.code == 0 {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	conn, buf, err := h.Hijack()
	if err == nil {
		s.hijacked = true
		s.streamed = true
	}
	return conn, buf, err
}

func (s *statusWriter) ReadFrom(src io.Reader) (int64, error) {
	if s.code == 0 {
		s.WriteHeader(http.StatusOK)
	}
	if rf, ok := s.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	return io.Copy(s.ResponseWriter, src)
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// statusCode reports what the client saw: the explicit code, 101 for a hijacked
// upgrade (the handler owns the wire, no code passed through here), or the 200
// net/http writes for a handler that returned without touching the response.
func (s *statusWriter) statusCode() int {
	switch {
	case s.code != 0:
		return s.code
	case s.hijacked:
		return http.StatusSwitchingProtocols
	default:
		return http.StatusOK
	}
}

// graphqlOutcome folds a GraphQL response's first error into the closed outcome
// vocabulary, using the same sentinel classification core.WriteErr applies on
// REST so the two surfaces agree about what "denied" or "not found" means.
func graphqlOutcome(err error) string {
	switch {
	case err == nil:
		return gqlOutcomeOK
	case errors.Is(err, core.ErrForbidden):
		return gqlOutcomeDenied
	case errors.Is(err, core.ErrNotFound):
		return gqlOutcomeNotFound
	case errors.Is(err, core.ErrBadRequest):
		return gqlOutcomeInvalid
	case errors.Is(err, core.ErrConflict), errors.Is(err, core.ErrBillingEnforced):
		return gqlOutcomeConflict
	case errors.Is(err, core.ErrPaymentRequired):
		return gqlOutcomePayment
	case errors.Is(err, core.ErrUnavailable):
		return gqlOutcomeUnavailable
	case core.IsPublicError(err):
		// A coded rejection (a quota/limit code) carrying no sentinel class.
		return gqlOutcomeRejected
	default:
		return gqlOutcomeInternal
	}
}

// mcpMetricsMiddleware times tool calls. It is added LAST in MCPServer, which
// makes it the outermost receiving middleware, so a call the workspace/scope
// gate refuses is observed as denied rather than vanishing.
//
// An unregistered tool name is passed through unobserved: the SDK answers it
// with an "unknown tool" JSON-RPC error, and minting a series per made-up name
// would hand any caller an unbounded label.
func mcpMetricsMiddleware(m *OriginMetrics) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			call, ok := req.(*mcp.CallToolRequest)
			if method != mcpCallToolMethod || !ok || call.Params == nil || !registeredMCPTool(call.Params.Name) {
				return next(ctx, method, req)
			}
			outcome := &toolOutcomeRecorder{}
			ctx = withToolOutcomeRecorder(ctx, outcome)
			start := time.Now()
			result, err := next(ctx, method, req)
			m.observeMCPTool(call.Params.Name, outcome.classify(result, err), time.Since(start))
			return result, err
		}
	}
}

const mcpCallToolMethod = "tools/call"

// registeredMCPTool reports whether name is a tool bex-api actually registers.
// classifiedOps is the scope matrix — regenerated from the live registry and
// swept by TestScopeMatrixCoversLiveOperations — so it is the one closed list of
// tool names that cannot drift from what is mounted.
func registeredMCPTool(name string) bool {
	_, ok := lookupScopeClass("MCP " + name)
	return ok
}

// toolOutcomeRecorder lets the gates INSIDE the tool-call chain hand their
// classification out to the metrics middleware. mcp.CallToolResult.SetError
// keeps only the message, so a workspace/scope refusal is indistinguishable
// from a fault by the time the middleware sees the result — the gate marks it
// instead of the middleware guessing from text.
type toolOutcomeRecorder struct{ outcome string }

type toolOutcomeRecorderKey struct{}

func withToolOutcomeRecorder(ctx context.Context, rec *toolOutcomeRecorder) context.Context {
	return context.WithValue(ctx, toolOutcomeRecorderKey{}, rec)
}

// markToolOutcome records the class of an error a tool-call gate is about to
// render as an error result. No-op when nothing is metering the call.
func markToolOutcome(ctx context.Context, err error) {
	rec, _ := ctx.Value(toolOutcomeRecorderKey{}).(*toolOutcomeRecorder)
	if rec == nil {
		return
	}
	rec.outcome = mcpErrorOutcome(err)
}

// classify prefers a gate's own marking; otherwise it reads the tool's answer —
// a returned error (the shared mcputil.AddTool seam preserves core sentinels
// through it) or an isError result.
func (t *toolOutcomeRecorder) classify(result mcp.Result, err error) string {
	if t.outcome != "" {
		return t.outcome
	}
	if err != nil {
		return mcpErrorOutcome(err)
	}
	if call, ok := result.(*mcp.CallToolResult); ok && call.IsError {
		return mcpOutcomeError
	}
	return mcpOutcomeOK
}

func mcpErrorOutcome(err error) string {
	if errors.Is(err, core.ErrForbidden) {
		return mcpOutcomeDenied
	}
	return mcpOutcomeError
}
