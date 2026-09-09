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

// Package logs is the logs feature: the aggregated read (MCP list_logs), the
// richer filtered query (REST/GraphQL), and the live tail (SSE). One Service
// implementation over an injected PodLogSource/PodLogStream, so Core stays
// clientset-free and the three surfaces read pod logs the same way.
package logs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/datastorelogs"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Log types — Render's `type` vocabulary. `app` is the App container's own
// output; `request` is Traefik's access log, shipped and labelled by the
// log-shipper (deploy/gitops/base/log-shipper.yaml) and served from the durable
// store. `build` history is store-backed, while the SSE tail follows the active
// build pod directly in BEX_BUILD_NAMESPACE.
// `application` is accepted as an input alias for `app`.
const (
	LogTypeApp     = "app"
	LogTypeRequest = "request"
	LogTypeBuild   = "build"
	// LogTypePreDeploy selects the pre-deploy step's Job-pod logs (w1/m33) — a
	// distinct LIVE source (the migration's own container), read directly rather
	// than from the durable store, so it is requested alone (validate() rejects
	// mixing it with app/request/build).
	LogTypePreDeploy = "predeploy"

	// logTypeApplicationAlias is the long spelling of LogTypeApp that bex's own
	// surfaces have always accepted.
	logTypeApplicationAlias = "application"
)

// ErrBuildNotRunning terminates a type=build SSE subscription when there is no
// active build pod — none pending, none running; a build that hasn't started
// yet is waited for, not refused (followBuildLogs).
// A named terminal event is honest and lets clients fall back
// to history instead of holding an empty stream forever.
var ErrBuildNotRunning = errors.New("no running build is available to follow")

// DefaultRevalidateInterval is how often an established live log tail re-runs
// its FRESH authorization check (w4/034): admission authorized once, but
// without the watchdog a successful SSE/WebSocket/NDJSON tail would outlive a
// membership or key revocation indefinitely — the checker's positive cache
// hides the revocation for its TTL, and nothing re-checks after that. The
// watchdog closes the window to one interval. The same cadence the SSH
// gateway's stream watchdog uses (sshgateway.DefaultRevalidateInterval).
const DefaultRevalidateInterval = core.DefaultStreamRevalidateInterval

// The log labels Render's clients filter and discover by (`list_log_label_values`'s
// enum). Each maps onto a stream label the shipper attaches — except LabelHost,
// which is deliberately NOT a label (unbounded per request; it lives in the access
// line) and is answered from the App's own hostnames.
const (
	LabelType       = "type"
	LabelLevel      = "level"
	LabelInstance   = "instance"
	LabelMethod     = "method"
	LabelStatusCode = "statusCode"
	LabelHost       = "host"
)

// DiscoverableLabels is the label enum LogLabelValues accepts — Render's exact six
// (render-oss/render-mcp-server's `list_log_label_values`), sorted so the error
// message and the MCP tool description read the same. The domain owns this
// vocabulary; translating it to a particular store's label names is the store
// adapter's job (see NewLokiLabelValuesSource).
var DiscoverableLabels = []string{LabelHost, LabelInstance, LabelLevel, LabelMethod, LabelStatusCode, LabelType}

// Render defaults the logs `limit` to 20 and caps it at 100; bex matches so a
// Render client sees identical paging. defaultLogTail caps the MCP list_logs
// read when the caller gives no limit.
const (
	defaultLogLimit = 20
	maxLogLimit     = 100
	defaultLogTail  = 100
)

// PodLogSource fetches the raw (timestamped) log stream for one pod container.
// The Service depends on this narrow function instead of a full clientset so the
// domain layer stays apiserver-thin and is trivial to fake; production wires it
// via NewPodLogSource. nil => the read verbs report core.ErrLogsUnavailable.
// Type alias of core.PodLogSource so the postgres logs feature (w3/m28) can
// inject the same source without importing the logs package.
type PodLogSource = core.PodLogSource

// PodLogStream streams a pod container's logs live (Follow:true) until ctx is
// cancelled. PodLogSource's tail-follow sibling — kept separate so the domain
// stays clientset-free. nil => FollowLogs reports core.ErrLogsUnavailable.
// since bounds where the follow starts (zero == from the beginning); it is the
// query's lower bound pushed down to kubelet instead of being applied only
// after the whole log has already crossed the wire.
type PodLogStream func(ctx context.Context, namespace, pod, container string, since time.Time) (io.ReadCloser, error)

// LogHistorySource reads durable log history for one App from a log store (Loki),
// applying the resolved LogQuery filters (label selector, time range, text, limit)
// server-side and returning entries oldest-first, capped at q.Limit — the same
// shape parseLogLine yields, so the adapters render either backend identically.
// It supersedes the live pod-log read for QueryLogs/Logs when wired (production
// keeps PodLogsFollow on pod logs for the tail; see docs/ADR010-observability.md). nil =>
// those verbs read live pod logs (the byte-identical default). Injected from
// BEX_LOKI_URL via NewLokiSource, like PodLogSource keeps the clientset out of the
// domain.
type LogHistorySource func(ctx context.Context, namespace string, q LogQuery) ([]LogEntry, error)

// LogLabelValuesSource discovers the values one stream label takes among the logs
// a LogQuery selects — the backend of `list_log_label_values`. label is the store's
// label name (already mapped from Render's vocabulary by lokiLabelFor), and the
// query is scoped to one App, so a caller can never enumerate another tenant's
// pods or hostnames. nil => the discovery verb reports core.ErrLogStoreUnavailable
// for the store-backed labels. Injected from BEX_LOKI_URL via
// NewLokiLabelValuesSource, the logs sibling of MetricsFilterValuesSource.
type LogLabelValuesSource func(ctx context.Context, namespace, label string, q LogQuery) ([]string, error)

// LogEntry is one log line in Render's log shape: a timestamp, the message, and
// a label set (service/instance/container). Adapters render it verbatim.
type LogEntry struct {
	Timestamp string            `json:"timestamp,omitempty"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// Service reads an App's pod logs over the injected sources.
type Service struct {
	*core.Base
	// PodLogs fetches pod logs for the read verbs; the live default when History
	// is unset. nil AND History nil => ErrLogsUnavailable.
	PodLogs PodLogSource
	// PodLogsFollow streams live pod logs for FollowLogs (the SSE tail); nil =>
	// FollowLogs reports ErrLogsUnavailable. The tail always reads pod logs, never
	// History — real-time, zero ingest lag, and it survives Loki being down.
	PodLogsFollow PodLogStream
	// History, when wired (BEX_LOKI_URL), backs QueryLogs/Logs with durable
	// history that survives pod restarts; nil => those verbs read live pod logs.
	History LogHistorySource
	// LabelValues, when wired (BEX_LOKI_URL), backs LogLabelValues — filter-value
	// discovery over the store's labels; nil => only the `host` label (answered
	// from the App itself) resolves, the rest report ErrLogStoreUnavailable.
	LabelValues LogLabelValuesSource
	// MaxQueryHours, when positive, caps the startTime–endTime window accepted by
	// REST log queries. 0 = unlimited.
	MaxQueryHours int
	// MaxSSEConns, when positive, caps concurrent GET /v1/logs/subscribe SSE
	// connections. Excess connections receive 429. 0 = unlimited.
	MaxSSEConns int64
	// MaxSSEConnsPerSubject and MaxSSEConnsPerWorkspace partition the global
	// stream pool so one caller or tenant cannot consume every slot. 0 disables
	// the corresponding dimension.
	MaxSSEConnsPerSubject   int
	MaxSSEConnsPerWorkspace int
	// RevalidateInterval is the live tail's authorization watchdog cadence
	// (w4/034): an established FollowLogs stream re-runs a FRESH
	// AuthorizeApp(can_view_logs) — bypassing the checker's positive cache — on
	// this interval, so a membership/key revocation or the App's deletion ends
	// the stream within one interval instead of at the next admission. 0 = the
	// DefaultRevalidateInterval default; negative disables the watchdog
	// (admission-only, the pre-watchdog behavior).
	RevalidateInterval time.Duration
	// BuildNamespace is BEX_BUILD_NAMESPACE — where the operator runs build Job
	// pods, so `type=build` reads/tails them from the right namespace. Empty
	// falls back to the API's own namespace, the operator's default when the env
	// is unset. (Pre-deploy Jobs are co-located with their App, ADR043 D8 — the
	// predeploy path uses the App CR's namespace, never this one.)
	BuildNamespace string
	// BuildPodWaitInterval is the re-list cadence while a type=build tail waits
	// for a Pending build pod to start (scheduling + image pull take minutes on
	// a cold node). 0 = the 2s default; tests shrink it.
	BuildPodWaitInterval time.Duration
	// DeployProgress, when wired (the control-plane store is configured), backs
	// platform progress-line synthesis (w1/m48) — Render-style `==>` narration
	// derived from deploy rows, merged into explicit type=build reads and the
	// live build tail. nil => no synthesis (byte-identical prior behavior).
	DeployProgress DeployProgressSource

	sseConns       atomic.Int64
	sseMu          sync.Mutex
	sseBySubject   map[string]int
	sseByWorkspace map[string]int
}

// Query directions — Render's `direction`: which end of the time window the
// `limit` lines are taken from. Backward (the default) keeps the most recent;
// forward keeps the oldest. Either way the returned slice stays oldest-first.
// The vocabulary itself is core's (shared with the audit log), aliased here
// for the adapters' convenience.
const (
	DirectionBackward = core.DirectionBackward
	DirectionForward  = core.DirectionForward
)

// LogQuery is the resolved filter set for QueryLogs / FollowLogs — Render's logs
// filter vocabulary (api-docs.render.com/reference/list-logs). Every filter here
// is honored; a filter bex cannot honor is refused at the adapter or with
// core.ErrLogStoreUnavailable, never silently dropped.
type LogQuery struct {
	App string
	// Database is resolved internally after a dpg- resource has passed
	// AuthorizeDatabase. Adapters continue to use Render's single `resource`
	// field (stored in App on input); callers cannot set this selector directly.
	Database string
	// KeyValue is resolved internally after a red- resource has passed
	// AuthorizeKeyValue. Adapters use Render's single `resource` field (stored
	// in App on input); callers cannot set this selector directly.
	KeyValue string
	Types    []string  // empty == all types; values: app | request | build
	Search   string    // case-insensitive substring on the message (Render's `text`)
	Since    time.Time // zero == no lower bound
	End      time.Time // zero == no upper bound
	Limit    int64     // max lines
	// Direction picks which end of the window Limit keeps: DirectionBackward
	// (default, newest) or DirectionForward (oldest).
	Direction string

	// Structured filters. Each is a value set (OR within a filter, AND across
	// filters — Render's semantics); a `*` wildcard is supported per value.
	// Level applies to app logs; Host/StatusCode/Method/Path to request logs;
	// Instance to app logs (a request line's origin is the edge, not a replica).
	Level      []string
	Instance   []string
	Host       []string
	StatusCode []string
	Method     []string
	Path       []string

	searchLower string // Search lowercased once by normalized(); read by keep()
}

// validate rejects a filter value outside the accepted vocabulary — an unknown
// `direction` or log `type`. It runs inside the verbs (before normalized(), which
// coerces), so the refusal is the service's, not each adapter's: a fourth caller
// cannot forget it, and all three surfaces refuse identically. core.ErrBadRequest
// maps to 400 (adapters name the offending value in the message).
func (q LogQuery) validate() error {
	if err := core.ValidateQueryRange(q.Since, q.End); err != nil {
		return err
	}
	if _, err := core.ParseDirection(q.Direction); err != nil {
		return err
	}
	for _, t := range q.Types {
		if t != LogTypeApp && t != LogTypeRequest && t != LogTypeBuild && t != LogTypePreDeploy {
			return fmt.Errorf("%w: unknown log type %q (want %s|%s|%s|%s)", core.ErrBadRequest, t, LogTypeApp, LogTypeRequest, LogTypeBuild, LogTypePreDeploy)
		}
	}
	// Pre-deploy and build logs are distinct live sources (Job pods, w1/m33 + w3/m14),
	// not merged with app/request — so each must be requested alone rather than
	// silently ignored inside a mixed query.
	if slices.Contains(q.Types, LogTypePreDeploy) && len(q.Types) > 1 {
		return fmt.Errorf("%w: log type %q must be requested on its own", core.ErrBadRequest, LogTypePreDeploy)
	}
	if slices.Contains(q.Types, LogTypeBuild) && len(q.Types) > 1 {
		return fmt.Errorf("%w: log type %q must be requested on its own", core.ErrBadRequest, LogTypeBuild)
	}
	return nil
}

// normalized clamps Limit to Render's paging range, defaults the direction, and
// precomputes searchLower.
func (q LogQuery) normalized() LogQuery {
	if q.Limit <= 0 {
		q.Limit = defaultLogLimit
	}
	if q.Limit > maxLogLimit {
		q.Limit = maxLogLimit
	}
	if q.Direction != DirectionForward {
		q.Direction = DirectionBackward
	}
	q.searchLower = strings.ToLower(q.Search)
	return q
}

// hasFilters reports whether any line-level filter (search/time) is set — the
// ones the pod-log path applies in Go.
func (q LogQuery) hasFilters() bool {
	return q.Search != "" || !q.Since.IsZero() || !q.End.IsZero()
}

// wants reports whether the query asks for a log type. An empty Types means "all
// types", so it wants every one of them.
func (q LogQuery) wants(t string) bool {
	if len(q.Types) == 0 {
		return true
	}
	return slices.Contains(q.Types, t)
}

// needsStore reports whether the query asks for something only the durable log
// store can answer: request logs, build logs, or a structured filter over labels
// the shipper attaches. The pod-log fallback has neither — it reads one
// container's stdout — so these must be refused there rather than quietly ignored.
func (q LogQuery) needsStore() bool {
	return slices.Contains(q.Types, LogTypeRequest) || // asked for request logs explicitly
		slices.Contains(q.Types, LogTypeBuild) || // build logs live only in the store
		len(q.Level) > 0 || len(q.Host) > 0 || len(q.StatusCode) > 0 ||
		len(q.Method) > 0 || len(q.Path) > 0
}

// tailSupports rejects the filters the live tail cannot honor. The tail follows
// pod stdout by design (docs/ADR010-observability.md § The tail reads pod logs) —
// the labels those filters select on are attached by the shipper on the way into
// the store, and are simply not present on a stream read straight off the kubelet.
// So this is a permanent property of the transport, not a missing dependency: the
// query API answers these, the tail says so, and neither silently ignores them.
// Note: build logs CAN be tailed live (the build Job's pod stdout, w3/m14) — so
// this check is more targeted than needsStore(), which still gates the QueryLogs
// historical path (Job pods are ephemeral; history requires the store).
func (q LogQuery) tailSupports() error {
	if slices.Contains(q.Types, LogTypeRequest) ||
		len(q.Level) > 0 || len(q.Host) > 0 || len(q.StatusCode) > 0 ||
		len(q.Method) > 0 || len(q.Path) > 0 {
		return fmt.Errorf("%w: the live tail reads pod logs, so it cannot stream request logs or filter by level/statusCode/method/path/host — query the logs API for those", core.ErrBadRequest)
	}
	return nil
}

// liveSubscribable reports (as an error) when a subscription has no live producer
// to stream. The live tail has exactly two producers — app pod stdout and build
// Job stdout — so a query the tail cannot honor (request logs, store-only
// filters) or one that wants neither app nor build (e.g. type=predeploy) must be
// refused up front rather than accepted into an idle stream that holds a
// process-global SSE slot until the client disconnects (codex #3).
func (q LogQuery) liveSubscribable() error {
	if err := q.tailSupports(); err != nil {
		return err
	}
	if len(q.Types) > 0 && !q.wants(LogTypeApp) && !q.wants(LogTypeBuild) {
		return fmt.Errorf("%w: the live tail streams only app and build logs; type=%v has no live producer — use the historical logs query", core.ErrBadRequest, q.Types)
	}
	return nil
}

// keepPod reports whether a pod satisfies the `instance` filter — the one
// structured filter the pod-log fallback CAN honor (a pod name is a pod name).
func (q LogQuery) keepPod(pod string) bool {
	if len(q.Instance) == 0 {
		return true
	}
	return slices.Contains(q.Instance, pod)
}

// capToLimit keeps q.Limit entries from the end the direction asks for: the
// newest (backward, the default) or the oldest (forward). entries are oldest-first
// and stay that way — direction chooses which lines, not how they're ordered.
func (q LogQuery) capToLimit(entries []LogEntry) []LogEntry {
	lim := lokiLimit(q)
	if int64(len(entries)) <= lim {
		return entries
	}
	if q.Direction == DirectionForward {
		return entries[:lim]
	}
	return entries[int64(len(entries))-lim:]
}

// NormalizeTypes maps Render's repeatable `type` filter onto the canonical set
// (app/request/build), accepting the `application` alias and treating ""/"all" as
// "every type". An unrecognized value is reported so the adapter can refuse it
// (REST 400) instead of silently widening the query. Shared by all three surfaces.
func NormalizeTypes(types []string) ([]string, error) {
	var out []string
	for _, t := range types {
		switch t {
		case "", "all":
			return nil, nil // one "all" means all — no narrowing
		case LogTypeApp, logTypeApplicationAlias:
			out = append(out, LogTypeApp)
		case LogTypeRequest:
			out = append(out, LogTypeRequest)
		case LogTypeBuild:
			out = append(out, LogTypeBuild)
		case LogTypePreDeploy:
			out = append(out, LogTypePreDeploy)
		default:
			return nil, fmt.Errorf("%w: unknown log type %q (want app|request|build|predeploy)", core.ErrBadRequest, t)
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(out))), nil
}

// keep reports whether an entry satisfies the search/time filters. Assumes
// normalized() has run (searchLower populated).
func (q LogQuery) keep(e LogEntry) bool {
	if q.Search != "" && !strings.Contains(strings.ToLower(e.Message), q.searchLower) {
		return false
	}
	if !q.Since.IsZero() || !q.End.IsZero() {
		if t, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
			if !q.Since.IsZero() && t.Before(q.Since) {
				return false
			}
			if !q.End.IsZero() && t.After(q.End) {
				return false
			}
		}
	}
	return true
}

// Logs returns recent log lines for an App, aggregated across its pods and
// sorted by timestamp. tail caps lines per instance (<=0 => defaultLogTail). It
// fails with core.ErrNotFound for an unknown App and core.ErrLogsUnavailable
// when no source is wired. It's the unfiltered convenience read; the REST and
// MCP adapters go through QueryLogs (Render's type/text/time filters).
func (s *Service) Logs(ctx context.Context, name string, tail int64) ([]LogEntry, error) {
	app, err := s.AuthorizeApp(ctx, core.RelCanViewLogs, name)
	if err != nil {
		return nil, err // ErrNotFound for unknown apps, exactly like Get
	}
	resource := appLogResource(app.Labels, name)
	if s.History == nil && s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	if tail <= 0 {
		tail = defaultLogTail
	}
	if s.History != nil {
		// Durable history: the unfiltered tail-N read is a limit-only query.
		entries, err := s.History(ctx, app.Namespace, LogQuery{App: app.Name, Limit: tail}.normalized())
		return setLogResource(entries, resource), err
	}
	entries, err := s.collectPodLogs(ctx, app.Namespace, LogQuery{App: app.Name}, tail)
	return setLogResource(entries, resource), err
}

// QueryLogs returns an App's log lines matching the filter set, oldest-first and
// capped at Limit. With the durable store wired it serves all log types (app +
// request + build) and every structured filter; in pod-log fallback mode it
// serves app logs and refuses what it cannot honor (ErrLogStoreUnavailable)
// rather than returning unfiltered lines. `type=build` without the store is a
// 503, not a silent empty — the same honesty rule as request logs.
func (s *Service) QueryLogs(ctx context.Context, q LogQuery) ([]LogEntry, error) {
	if isPostgresResource(q.App) {
		return s.queryPostgresLogs(ctx, q)
	}
	if isKeyValueResource(q.App) {
		return s.queryKeyValueLogs(ctx, q)
	}
	requested := q.App
	app, err := s.AuthorizeApp(ctx, core.RelCanViewLogs, requested)
	if err != nil {
		return nil, err
	}
	resource := appLogResource(app.Labels, requested)
	// Logs are indexed and pod-selected by the App CR's actual metadata.name,
	// which may be tenant-prefixed. The public name/srv-id is only an API
	// address and must not leak into the Kubernetes label selector.
	q.App = app.Name
	// The App CR is in hand — its namespace is the per-tenant `<ws>` namespace
	// (ADR043), where its pods and Loki streams also live; use it directly
	// rather than the shared s.Namespace, exactly as the metrics feature does.
	appNS := app.Namespace
	if err := q.validate(); err != nil {
		return nil, err
	}
	if s.History == nil && s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	q = q.normalized()
	pods, err := s.AppPodsIn(ctx, appNS, app.Name)
	if err != nil {
		return nil, err
	}
	s.translateInstanceFilter(ctx, resource, appNS, &q, candidatesFromPods(pods))
	// Pre-deploy step logs (w1/m33): a distinct LIVE source (the migration Job's
	// pod), read directly from the App's own namespace — the Job is co-located
	// with the App (ADR043 D8), not in the build namespace — never the durable
	// store, which has no predeploy stream — like the SSE tail always reads pod
	// logs. validate() guarantees predeploy is requested alone, so it owns the
	// whole response.
	if slices.Contains(q.Types, LogTypePreDeploy) {
		entries, err := s.collectPreDeployLogs(ctx, appNS, q)
		return setLogResource(entries, resource), err
	}
	if s.History != nil {
		// The store applies every filter (labels + line) server-side and returns
		// oldest-first, capped at q.Limit.
		entries, err := s.History(ctx, appNS, q)
		if err != nil {
			return nil, err
		}
		entries = s.synthesizeProgress(ctx, q, resource, app.Spec.Repo, app.Spec.Branch, app.Spec.Type, entries)
		return setLogResource(entries, resource), nil
	}
	if q.needsStore() {
		return nil, core.ErrLogStoreUnavailable
	}
	var out []LogEntry
	for i := range pods {
		if !q.keepPod(pods[i].Name) {
			continue
		}
		entries, err := s.readPodLogs(ctx, appNS, q.App, pods[i].Name, q.Limit)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return setLogResource(q.filterAndCap(out), resource), nil
}

// queryPostgresLogs is the one datastore-scoped core read behind the existing
// REST, GraphQL, and MCP log adapters. Render addresses every log-producing
// resource through the generic `resource` filter, so no parallel public API is
// needed. The Database authorization gate runs before either a Loki selector or
// a pod selector is constructed.
func (s *Service) queryPostgresLogs(ctx context.Context, q LogQuery) ([]LogEntry, error) {
	requested := q.App
	database, err := s.AuthorizeDatabase(ctx, core.RelCanViewLogs, requested)
	if err != nil {
		return nil, err
	}
	q.App = ""
	q.Database = database.Name
	if err := q.validatePostgres(); err != nil {
		return nil, err
	}
	if s.History == nil && s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	q = q.normalized()
	pods, err := s.DatabasePods(ctx, database.Namespace, database.Name)
	if err != nil {
		return nil, err
	}
	s.translateInstanceFilter(ctx, requested, database.Namespace, &q, candidatesFromPods(pods))
	if s.History != nil {
		entries, err := s.History(ctx, database.Namespace, q)
		return setLogResource(entries, requested), err
	}
	entries, err := s.collectDatastorePodLogs(ctx, database.Namespace, q, pods, datastore{
		name:      q.Database,
		container: core.CNPGPostgresContainer,
		kind:      datastorelogs.KindPostgres,
	})
	if err != nil {
		return nil, err
	}
	return setLogResource(q.filterAndCap(entries), requested), nil
}

func isPostgresResource(resource string) bool {
	kind, ok := ids.KindOf(resource)
	return ok && kind == ids.Postgres
}

// validatePostgres pins the supported managed-Postgres contract: time range,
// direction, text search, limit, and instance. HTTP request/build filters are
// service concepts; refusing them is safer than silently dropping them.
func (q LogQuery) validatePostgres() error {
	if err := q.validate(); err != nil {
		return err
	}
	if len(q.Types) > 0 || len(q.Level) > 0 || len(q.Host) > 0 || len(q.StatusCode) > 0 || len(q.Method) > 0 || len(q.Path) > 0 {
		return fmt.Errorf("%w: managed Postgres logs support text, time range, direction, limit, and instance filters", core.ErrBadRequest)
	}
	return nil
}

// queryKeyValueLogs is the one datastore-scoped core read behind the existing
// REST, GraphQL, and MCP key-value log adapters. Mirrors queryPostgresLogs with
// AuthorizeKeyValue and the Loki `keyvalue` stream label.
func (s *Service) queryKeyValueLogs(ctx context.Context, q LogQuery) ([]LogEntry, error) {
	requested := q.App
	kv, err := s.AuthorizeKeyValue(ctx, core.RelCanViewLogs, requested)
	if err != nil {
		return nil, err
	}
	q.App = ""
	q.KeyValue = kv.Name
	if err := q.validateKeyValue(); err != nil {
		return nil, err
	}
	if s.History == nil && s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	q = q.normalized()
	pods, err := s.KeyValuePods(ctx, kv.Namespace, kv.Name)
	if err != nil {
		return nil, err
	}
	s.translateInstanceFilter(ctx, requested, kv.Namespace, &q, candidatesFromPods(pods))
	if s.History != nil {
		entries, err := s.History(ctx, kv.Namespace, q)
		return setLogResource(entries, requested), err
	}
	entries, err := s.collectDatastorePodLogs(ctx, kv.Namespace, q, pods, datastore{
		name:      q.KeyValue,
		container: core.ValkeyContainer,
		kind:      datastorelogs.KindKeyValue,
	})
	if err != nil {
		return nil, err
	}
	return setLogResource(q.filterAndCap(entries), requested), nil
}

func isKeyValueResource(resource string) bool {
	kind, ok := ids.KindOf(resource)
	return ok && kind == ids.KeyValue
}

// validateKeyValue pins the supported managed Key Value contract: time range,
// direction, text search, limit, and instance. HTTP request/build filters are
// service concepts; refusing them is safer than silently dropping them.
func (q LogQuery) validateKeyValue() error {
	if err := q.validate(); err != nil {
		return err
	}
	if len(q.Types) > 0 || len(q.Level) > 0 || len(q.Host) > 0 || len(q.StatusCode) > 0 || len(q.Method) > 0 || len(q.Path) > 0 {
		return fmt.Errorf("%w: managed Key Value logs support text, time range, direction, limit, and instance filters", core.ErrBadRequest)
	}
	return nil
}

// filterAndCap applies the line-level search/time filters (in place — skipping
// the pass entirely on the common no-filter query) and clamps to q.Limit. The
// pod-log path's shared tail, used by both the app and pre-deploy reads.
func (q LogQuery) filterAndCap(entries []LogEntry) []LogEntry {
	if q.hasFilters() {
		kept := entries[:0]
		for _, e := range entries {
			if q.keep(e) {
				kept = append(kept, e)
			}
		}
		entries = kept
	}
	return q.capToLimit(entries)
}

// LogLabelValues lists the values a log label takes for one App — Render's
// `list_log_label_values` / `GET /v1/logs/values`, so a client discovers which
// levels, statuses, methods, instances or types actually occur instead of guessing.
// Values are always scoped to the requested App's streams: no caller can enumerate
// another tenant's pods. `host` is answered from the App's own URLs (it is not a
// stream label — the cardinality budget keeps it in the line), so it resolves even
// without the store.
func (s *Service) LogLabelValues(ctx context.Context, label string, q LogQuery) ([]string, error) {
	if isPostgresResource(q.App) {
		return s.postgresLogLabelValues(ctx, label, q)
	}
	if isKeyValueResource(q.App) {
		return s.keyValueLogLabelValues(ctx, label, q)
	}
	requested := q.App
	app, err := s.AuthorizeApp(ctx, core.RelCanViewLogs, requested)
	if err != nil {
		return nil, err
	}
	resource := appLogResource(app.Labels, requested)
	if err := q.validate(); err != nil {
		return nil, err
	}
	q.App = app.Name
	if !slices.Contains(DiscoverableLabels, label) {
		// Naming the offending label beats an empty list, which a client would read
		// as "this service has no such values".
		return nil, fmt.Errorf("%w: unknown log label %q (want %s)", core.ErrBadRequest, label, strings.Join(DiscoverableLabels, "|"))
	}
	if label == LabelHost {
		// `host` is not a stream label (the cardinality budget keeps it in the line),
		// so its values come from the App itself — which is why it resolves even with
		// no store wired.
		return core.HostsFromURLs(app.Status.URLs), nil
	}
	if s.LabelValues == nil {
		return nil, core.ErrLogStoreUnavailable
	}
	// app.Namespace is the App's `<ws>` namespace under ADR043 (see QueryLogs);
	// the discoverable stream labels live there, not in the shared s.Namespace.
	values, err := s.LabelValues(ctx, app.Namespace, label, q.normalized())
	if err != nil {
		return nil, err
	}
	if label == LabelInstance {
		return projectLogLabelValues(resource, values), nil
	}
	return slices.Compact(slices.Sorted(slices.Values(values))), nil
}

func (s *Service) postgresLogLabelValues(ctx context.Context, label string, q LogQuery) ([]string, error) {
	requested := q.App
	database, err := s.AuthorizeDatabase(ctx, core.RelCanViewLogs, requested)
	if err != nil {
		return nil, err
	}
	q.App = ""
	q.Database = database.Name
	if err := q.validatePostgres(); err != nil {
		return nil, err
	}
	if label != LabelInstance {
		return nil, fmt.Errorf("%w: managed Postgres log label %q is unsupported (want %s)", core.ErrBadRequest, label, LabelInstance)
	}
	if s.LabelValues != nil {
		values, err := s.LabelValues(ctx, database.Namespace, label, q.normalized())
		if err != nil {
			return nil, err
		}
		return projectLogLabelValues(requested, values), nil
	}
	pods, err := s.DatabasePods(ctx, database.Namespace, database.Name)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(pods))
	for i := range pods {
		values = append(values, pods[i].Name)
	}
	return projectLogLabelValues(requested, values), nil
}

func (s *Service) keyValueLogLabelValues(ctx context.Context, label string, q LogQuery) ([]string, error) {
	requested := q.App
	kv, err := s.AuthorizeKeyValue(ctx, core.RelCanViewLogs, requested)
	if err != nil {
		return nil, err
	}
	q.App = ""
	q.KeyValue = kv.Name
	if err := q.validateKeyValue(); err != nil {
		return nil, err
	}
	if label != LabelInstance {
		return nil, fmt.Errorf("%w: managed Key Value log label %q is unsupported (want %s)", core.ErrBadRequest, label, LabelInstance)
	}
	if s.LabelValues != nil {
		values, err := s.LabelValues(ctx, kv.Namespace, label, q.normalized())
		if err != nil {
			return nil, err
		}
		return projectLogLabelValues(requested, values), nil
	}
	pods, err := s.KeyValuePods(ctx, kv.Namespace, kv.Name)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(pods))
	for i := range pods {
		values = append(values, pods[i].Name)
	}
	return projectLogLabelValues(requested, values), nil
}

// FollowLogs streams an App's new log lines to emit until ctx is cancelled or
// emit errors. The tail always reads live pod logs (never the store — real-time,
// zero ingest lag). It serves app logs from App pods and type=build from the
// active build pod in BuildNamespace, with the text/time/instance filters; it
// refuses store-only filters. Requires a PodLogStream (nil =>
// core.ErrLogsUnavailable).
func (s *Service) FollowLogs(ctx context.Context, q LogQuery, emit func(LogEntry) error) error {
	if isPostgresResource(q.App) {
		if _, err := s.AuthorizeDatabase(ctx, core.RelCanViewLogs, q.App); err != nil {
			return err
		}
		return fmt.Errorf("%w: managed Postgres live log subscription is not supported; use the historical logs query", core.ErrBadRequest)
	}
	if isKeyValueResource(q.App) {
		if _, err := s.AuthorizeKeyValue(ctx, core.RelCanViewLogs, q.App); err != nil {
			return err
		}
		return fmt.Errorf("%w: managed Key Value live log subscription is not supported; use the historical logs query", core.ErrBadRequest)
	}
	requested := q.App
	app, err := s.AuthorizeApp(ctx, core.RelCanViewLogs, requested)
	if err != nil {
		return err
	}
	resource := appLogResource(app.Labels, requested)
	q.App = app.Name
	if err := q.validate(); err != nil {
		return err
	}
	if s.PodLogsFollow == nil {
		return core.ErrLogsUnavailable
	}
	q = q.normalized()
	podsLive, err := s.AppPodsIn(ctx, app.Namespace, app.Name)
	if err != nil {
		return err
	}
	s.translateInstanceFilter(ctx, resource, app.Namespace, &q, candidatesFromPods(podsLive))
	// The tail's refusal is about the TRANSPORT, not the deployment: it reads pod
	// logs even when Loki is wired, so a store-only filter is something this stream
	// structurally cannot do — a bad request (400), not "the store is missing"
	// (503), which would be an outright lie on a cluster that has one.
	if err := q.tailSupports(); err != nil {
		return err
	}
	// Authorization lifetime (w4/034): admission authorized once above, but an
	// established tail must not outlive the grant. The watchdog re-checks FRESH
	// (uncached) on the interval; a deny, a checker failure, or the App's
	// deletion cancels the stream ctx, which ends every producer goroutine and
	// the emit loops below — and with them the caller's subscription cap slots.
	// An allowed re-check touches nothing: the tail is not interrupted.
	ctx, stopWatchdog := withRevalidation(ctx, s.revalidateInterval(), func(checkCtx context.Context) error {
		return s.revalidateTailTarget(checkCtx, app.Namespace, app.Name)
	})
	defer stopWatchdog()
	if len(q.Types) == 1 && q.Types[0] == LogTypeBuild {
		return s.followBuildLogs(ctx, q, resource, app.Spec.Repo, app.Spec.Branch, app.Spec.Type, emit)
	}
	if !q.wants(LogTypeApp) {
		// SECURITY (codex #3): the live tail has exactly two producers — app pod
		// stdout and build Job stdout. A query wanting neither (e.g. type=predeploy)
		// has nothing to stream; it must NOT park on ctx.Done() holding one of the
		// process-global SSE slots until the client disconnects. Refuse immediately
		// so the slot is released. (logsSubscribe also rejects this pre-slot via
		// liveSubscribable; this guards any other FollowLogs caller.)
		return fmt.Errorf("%w: the live tail streams only app and build logs; type=%v has no live producer — use the historical logs query", core.ErrBadRequest, q.Types)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan LogEntry, 64)
	var wg sync.WaitGroup

	pods := make([]string, 0, len(podsLive))
	for i := range podsLive {
		if q.keepPod(podsLive[i].Name) {
			pods = append(pods, podsLive[i].Name)
		}
	}
	for i := range pods {
		wg.Add(1)
		go func(pod string) {
			defer wg.Done()
			// app.Namespace: the pods stream from their `<ws>` namespace under
			// ADR043, the same namespace translateInstanceFilter selected them in.
			s.streamPodLogs(ctx, app.Namespace, q.App, pod, q.Since, ch)
		}(pods[i])
	}
	go func() { wg.Wait(); close(ch) }()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e, ok := <-ch:
			if !ok {
				return nil
			}
			if !q.keep(e) {
				continue
			}
			setLogEntryResource(&e, resource)
			if err := emit(e); err != nil {
				return err
			}
		}
	}
}

// revalidateInterval resolves the watchdog cadence: 0 means "not configured"
// and falls back to DefaultRevalidateInterval; a negative value disables the
// watchdog outright (admission-only, the pre-w4/034 behavior).
func (s *Service) revalidateInterval() time.Duration {
	if s.RevalidateInterval == 0 {
		return DefaultRevalidateInterval
	}
	return s.RevalidateInterval
}

// revalidateTailTarget is the watchdog check for an established tail: re-fetch
// the App (its deletion ends the stream too — a tail of a gone resource has
// nothing left to be authorized FOR) and reassert can_view_logs against its
// OWN workspace through the Fresh seam, bypassing the checker's positive cache
// so a revocation takes effect within one interval, not at the next admission.
func (s *Service) revalidateTailTarget(ctx context.Context, namespace, name string) error {
	var fresh appv1alpha1.App
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &fresh); err != nil {
		return err
	}
	return s.AuthorizeAppFresh(ctx, core.RelCanViewLogs, &fresh)
}

// withRevalidation is the logs-facing name of core.WithRevalidation, shared
// with the SSH/agent transports (round-9 #6 / round 15). logs must not import
// the gateway package just for it.
func withRevalidation(parent context.Context, interval time.Duration, check func(context.Context) error) (context.Context, context.CancelFunc) {
	return core.WithRevalidation(parent, interval, check)
}

// followBuildLogs streams every currently-running container from the newest
// active build pod. Container discovery covers unsigned BuildKit (buildkit),
// signed BuildKit (buildkit init then sign), and kpack's generated step names.
// Completed pods are deliberately excluded: their history comes from Loki and
// following them would replay an older deploy under the current subscription.
//
// Platform progress lines (w1/m48): before and while waiting for the pod, the
// tail speaks the deploy's own narration — queued/building lines at subscribe,
// new phases as the wait loop observes them — so a subscriber never stares at
// a silent stream while the build image pulls. After the pod's stream ends,
// one final check emits the closing line if the deploy row already turned
// terminal; otherwise it rides the history read's post-deploy grace poll.
func (s *Service) followBuildLogs(ctx context.Context, q LogQuery, resource, repo, branch, serviceType string, emit func(LogEntry) error) error {
	prog := s.newProgressFollower(q, resource, repo, branch, serviceType)
	if err := prog.emitReached(ctx, emit); err != nil {
		return err
	}
	pod, containers, err := s.awaitBuildPod(ctx, q, prog, emit)
	if err != nil {
		return err
	}

	ns := s.BuildNamespace
	if ns == "" {
		ns = s.Namespace
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan LogEntry, 64)
	var opened atomic.Bool
	var wg sync.WaitGroup
	startStream := func(container string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.streamContainerLogs(ctx, ns, q.App, pod.Name, container, LogTypeBuild, q.Since, ch, &opened)
		}()
	}
	started := make(map[string]bool, len(containers))
	for _, container := range containers {
		started[container] = true
		startStream(container)
	}
	// The build's phases are sequential init containers, so the set running at
	// subscribe time is not the set that will run: a subscriber connected while
	// the clone streams would otherwise never see BuildKit's output at all
	// (w6/m123). Keep watching the pod and attach to each phase as it starts —
	// until every container the spec declares has been attached (nothing more
	// can begin) or the pod turns terminal. The stream then ends the way it
	// always has: when the attached streams finish.
	allAttached := func(p *corev1.Pod) bool {
		return len(started) >= len(p.Spec.InitContainers)+len(p.Spec.Containers)
	}
	if !allAttached(&pod) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait := s.BuildPodWaitInterval
			if wait <= 0 {
				wait = 2 * time.Second
			}
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait):
				}
				pods, err := s.BuildPods(ctx, q.App, s.BuildNamespace)
				if err != nil {
					continue
				}
				var cur *corev1.Pod
				for i := range pods {
					if pods[i].Name == pod.Name {
						cur = &pods[i]
						break
					}
				}
				if cur == nil {
					return
				}
				for _, container := range runningContainers(*cur) {
					if !started[container] {
						started[container] = true
						startStream(container)
					}
				}
				if allAttached(cur) ||
					cur.Status.Phase == corev1.PodSucceeded || cur.Status.Phase == corev1.PodFailed {
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); close(ch) }()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e, ok := <-ch:
			if !ok {
				if !opened.Load() {
					return ErrBuildNotRunning
				}
				// The build's stdout is done — if the deploy row has already
				// closed, speak the terminal line before ending the stream.
				return prog.emitReached(ctx, emit)
			}
			if !q.keep(e) {
				continue
			}
			setLogEntryResource(&e, resource)
			if err := emit(e); err != nil {
				return err
			}
		}
	}
}

// awaitBuildPod resolves the newest build pod with at least one running
// container. A Pending pod (or a Running one whose containers haven't started)
// is a build in flight, not a missing one — scheduling plus a cold image pull
// keep the pod silent for minutes in production, and a subscriber connecting
// the moment its deploy opens must wait for the first line rather than receive
// the terminal "no running build" event and give up right before the build
// speaks. ErrBuildNotRunning is returned only when nothing is pending or
// running — the honestly-terminal case (no build, or it already completed and
// its history belongs to Loki). Each wait tick also re-reads the deploy row
// through prog (nil-safe) so phase transitions narrate live while the pod is
// still silent.
func (s *Service) awaitBuildPod(ctx context.Context, q LogQuery, prog *progressFollower, emit func(LogEntry) error) (corev1.Pod, []string, error) {
	wait := s.BuildPodWaitInterval
	if wait <= 0 {
		wait = 2 * time.Second
	}
	for {
		pods, err := s.BuildPods(ctx, q.App, s.BuildNamespace)
		if err != nil {
			return corev1.Pod{}, nil, err
		}
		var active []corev1.Pod
		waiting := false
		for i := range pods {
			if !q.keepPod(pods[i].Name) {
				continue
			}
			switch pods[i].Status.Phase {
			case corev1.PodRunning:
				active = append(active, pods[i])
			case corev1.PodPending:
				waiting = true
			}
		}
		if len(active) > 0 {
			sort.SliceStable(active, func(i, j int) bool {
				if active[i].CreationTimestamp.Time.Equal(active[j].CreationTimestamp.Time) {
					return active[i].Name < active[j].Name
				}
				return active[i].CreationTimestamp.Time.Before(active[j].CreationTimestamp.Time)
			})
			pod := active[len(active)-1]
			if containers := runningContainers(pod); len(containers) > 0 {
				return pod, containers, nil
			}
			// Running phase with no container up resolves within seconds —
			// either a container starts or the phase turns terminal.
			waiting = true
		}
		if !waiting {
			return corev1.Pod{}, nil, ErrBuildNotRunning
		}
		if err := prog.emitReached(ctx, emit); err != nil {
			return corev1.Pod{}, nil, err
		}
		select {
		case <-ctx.Done():
			return corev1.Pod{}, nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

func runningContainers(pod corev1.Pod) []string {
	var out []string
	for i := range pod.Status.InitContainerStatuses {
		if pod.Status.InitContainerStatuses[i].State.Running != nil {
			out = append(out, pod.Status.InitContainerStatuses[i].Name)
		}
	}
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].State.Running != nil {
			out = append(out, pod.Status.ContainerStatuses[i].Name)
		}
	}
	return out
}

// appLogResource is the public resource identity each returned log line carries.
// Store-managed and API-created Apps use their Render-shaped srv- id; legacy
// hand-applied CRs retain the identifier the caller used.
func appLogResource(labels map[string]string, requested string) string {
	if id := labels[core.LabelAppID]; id != "" {
		return id
	}
	return requested
}

// noSuchInstance is an internal pod selector that can never match a real name.
// Used when every public/legacy instance filter fails to resolve so the query
// stays narrow instead of dropping the filter and returning every line.
const noSuchInstance = "__bex_no_such_instance__"

func projectLogInstance(resourceID, podName string) string {
	if resourceID == "" || podName == "" {
		return podName
	}
	return ids.ServiceInstanceID(resourceID, podName)
}

func projectLogLabelValues(resourceID string, values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, projectLogInstance(resourceID, v))
	}
	return slices.Compact(slices.Sorted(slices.Values(out)))
}

func candidatesFromPods(pods []corev1.Pod) []ids.InstanceCandidate {
	out := make([]ids.InstanceCandidate, 0, len(pods))
	for i := range pods {
		out = append(out, ids.InstanceCandidate{Name: pods[i].Name, UID: string(pods[i].UID)})
	}
	return out
}

// translateInstanceFilter rewrites public instance ids (and legacy raw-name /
// UID-derived selectors) into the internal pod names Loki and the live tail
// match on. Candidates are the authorized live pods plus, when the store is
// wired, historical pod names from label discovery — never a cross-resource
// enumeration.
func (s *Service) translateInstanceFilter(ctx context.Context, resourceID, namespace string, q *LogQuery, live []ids.InstanceCandidate) {
	if len(q.Instance) == 0 {
		return
	}
	cands := append([]ids.InstanceCandidate(nil), live...)
	if s.LabelValues != nil {
		disc := *q
		disc.Instance = nil
		if vals, err := s.LabelValues(ctx, namespace, LabelInstance, disc.normalized()); err == nil {
			seen := make(map[string]struct{}, len(cands))
			for _, c := range cands {
				seen[c.Name] = struct{}{}
			}
			for _, name := range vals {
				if name == "" {
					continue
				}
				if _, ok := seen[name]; ok {
					continue
				}
				cands = append(cands, ids.InstanceCandidate{Name: name})
			}
		}
	}
	resolved := ids.ResolveInstanceSelectors(q.Instance, resourceID, cands)
	if len(resolved) == 0 {
		q.Instance = []string{noSuchInstance}
		return
	}
	q.Instance = resolved
}

func setLogResource(entries []LogEntry, resource string) []LogEntry {
	for i := range entries {
		setLogEntryResource(&entries[i], resource)
	}
	return entries
}

func setLogEntryResource(entry *LogEntry, resource string) {
	if entry.Labels == nil {
		entry.Labels = map[string]string{}
	}
	entry.Labels["service"] = resource
	if pod := entry.Labels[LabelInstance]; pod != "" {
		entry.Labels[LabelInstance] = projectLogInstance(resource, pod)
	}
}

// collectPodLogs reads up to tail lines from every replica of an App the query's
// `instance` filter admits, tagged and timestamp-sorted. Shared by Logs (MCP) and
// QueryLogs (REST/GraphQL).
func (s *Service) collectPodLogs(ctx context.Context, namespace string, q LogQuery, tail int64) ([]LogEntry, error) {
	pods, err := s.appPodNames(ctx, namespace, q)
	if err != nil {
		return nil, err
	}
	var out []LogEntry
	for _, pod := range pods {
		entries, err := s.readPodLogs(ctx, namespace, q.App, pod, tail)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out, nil
}

// datastore names everything the managed Postgres and Key Value pod-log reads
// do differently: which instance, which container carries the process log, and
// which type label the entries are stamped with.
type datastore struct {
	name      string
	container string
	kind      string
}

// collectDatastorePodLogs reads one managed datastore's logs from its pods.
// namespace is the datastore CR's own namespace (ADR043 D8) — reading from the
// shared one instead returns no error and no logs, which is how the App-side
// version of this bug stayed invisible.
func (s *Service) collectDatastorePodLogs(ctx context.Context, namespace string, q LogQuery, pods []corev1.Pod, ds datastore) ([]LogEntry, error) {
	var out []LogEntry
	for i := range pods {
		if !q.keepPod(pods[i].Name) {
			continue
		}
		entries, err := s.readContainerLogs(ctx, namespace, ds.name, pods[i].Name, ds.container, q.Limit)
		if err != nil {
			return nil, err
		}
		for j := range entries {
			entries[j].Labels[LabelType] = ds.kind
		}
		out = append(out, entries...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out, nil
}

func (s *Service) collectDatabasePodLogs(ctx context.Context, namespace string, q LogQuery) ([]LogEntry, error) {
	pods, err := s.DatabasePods(ctx, namespace, q.Database)
	if err != nil {
		return nil, err
	}
	return s.collectDatastorePodLogs(ctx, namespace, q, pods, datastore{
		name:      q.Database,
		container: core.CNPGPostgresContainer,
		kind:      datastorelogs.KindPostgres,
	})
}

func (s *Service) collectKeyValuePodLogs(ctx context.Context, namespace string, q LogQuery) ([]LogEntry, error) {
	pods, err := s.KeyValuePods(ctx, namespace, q.KeyValue)
	if err != nil {
		return nil, err
	}
	return s.collectDatastorePodLogs(ctx, namespace, q, pods, datastore{
		name:      q.KeyValue,
		container: core.ValkeyContainer,
		kind:      datastorelogs.KindKeyValue,
	})
}

// appPodNames lists the App's replica names the query's `instance` filter admits
// — the pod-log path's honoring of that filter (a pod name is a pod name, so this
// one structured filter needs no store).
func (s *Service) appPodNames(ctx context.Context, namespace string, q LogQuery) ([]string, error) {
	pods, err := s.AppPodsIn(ctx, namespace, q.App)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(pods))
	for i := range pods {
		if q.keepPod(pods[i].Name) {
			out = append(out, pods[i].Name)
		}
	}
	return out, nil
}

func (s *Service) readPodLogs(ctx context.Context, namespace, service, pod string, tail int64) ([]LogEntry, error) {
	return s.readContainerLogs(ctx, namespace, service, pod, core.AppContainer, tail)
}

// readContainerLogs reads up to tail lines from one pod's container, tagged with
// service+pod. Generalizes readPodLogs so the pre-deploy path can read the
// "predeploy" container (in the App's own namespace) with the same parsing.
func (s *Service) readContainerLogs(ctx context.Context, namespace, service, pod, container string, tail int64) ([]LogEntry, error) {
	rc, err := s.PodLogs(ctx, namespace, pod, container, tail)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	var entries []LogEntry
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // allow long lines
	for sc.Scan() {
		entries = append(entries, parseLogLine(service, pod, sc.Text()))
	}
	return entries, sc.Err()
}

// collectPreDeployLogs reads the pre-deploy step's Job-pod logs (w1/m33) for an
// App, oldest-first and capped at q.Limit, applying the same text/time filters
// the app pod-log path does. appNS is the App CR's own namespace — the Job is
// co-located with the App (ADR043 D8), so its pods live there, NOT in the build
// namespace. Live-only: a Job pod that has been TTL-reaped is simply gone (an
// empty read), never an error — the same ephemerality as build logs. Requires
// PodLogs to be wired (ErrLogsUnavailable otherwise).
func (s *Service) collectPreDeployLogs(ctx context.Context, appNS string, q LogQuery) ([]LogEntry, error) {
	if s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	pods, err := s.PreDeployPods(ctx, q.App, appNS)
	if err != nil {
		return nil, err
	}
	var out []LogEntry
	for i := range pods {
		pod := pods[i].Name
		entries, err := s.readContainerLogs(ctx, appNS, q.App, pod, core.PreDeployContainer, q.Limit)
		if err != nil {
			// A reaped pod (or a container that never produced logs) drops out of
			// the read rather than failing the whole query.
			continue
		}
		out = append(out, entries...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return q.filterAndCap(out), nil
}

// streamPodLogs follows one pod's log into ch until ctx ends or the stream
// closes. A replica going away ends its stream without failing the subscription.
func (s *Service) streamPodLogs(ctx context.Context, namespace, service, pod string, since time.Time, ch chan<- LogEntry) {
	s.streamContainerLogs(ctx, namespace, service, pod, core.AppContainer, LogTypeApp, since, ch, nil)
}

func (s *Service) streamContainerLogs(ctx context.Context, namespace, service, pod, container, logType string, since time.Time, ch chan<- LogEntry, opened *atomic.Bool) {
	rc, err := s.PodLogsFollow(ctx, namespace, pod, container, since)
	if err != nil {
		return
	}
	defer func() { _ = rc.Close() }()
	if opened != nil {
		opened.Store(true)
	}

	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return
		case ch <- parseContainerLogLine(service, pod, container, logType, sc.Text()):
		}
	}
}

// parseLogLine splits kubelet's "Timestamps: true" prefix (an RFC3339Nano stamp,
// a space, then the message) off a log line, tagging it with Render-shaped
// labels. A line without a parseable stamp is kept whole as the message.
func parseLogLine(service, pod, line string) LogEntry {
	return parseContainerLogLine(service, pod, core.AppContainer, LogTypeApp, line)
}

// maxLogMessageBytes caps one log record's message (round-5 finding 17). The
// stream/query scanners accept lines up to ~1 MiB; a tenant workload emitting
// repeated near-limit lines otherwise flows unbounded into every viewer (the
// dashboard/mobile clients cap by record COUNT, not bytes). 64 KiB is generous
// (containerd already splits raw lines at 16 KiB) and bounds every downstream
// consumer at the source. Truncation trims to a valid-UTF-8 boundary so the
// marker never rides a split rune.
const maxLogMessageBytes = 64 * 1024

func parseContainerLogLine(service, pod, container, logType, line string) LogEntry {
	ts, msg := "", line
	if i := strings.IndexByte(line, ' '); i > 0 {
		if t, err := time.Parse(time.RFC3339Nano, line[:i]); err == nil {
			ts = t.UTC().Format(time.RFC3339Nano)
			msg = line[i+1:]
		}
	}
	if len(msg) > maxLogMessageBytes {
		msg = strings.ToValidUTF8(msg[:maxLogMessageBytes], "") + " …[truncated]"
	}
	return LogEntry{
		Timestamp: ts,
		Message:   msg,
		Labels: map[string]string{
			"service":   service,
			"instance":  pod,
			"container": container,
			LabelType:   logType,
		},
	}
}
