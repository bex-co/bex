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
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Log types — Render's `type` vocabulary. `app` is the App container's own
// output; `request` is Traefik's access log, shipped and labelled by the
// log-shipper (deploy/gitops/base/log-shipper.yaml) and served from the durable
// store. `build` has no backend in bex (builds run in a separate plane,
// docs/ADR001-go-and-gitops.md), so it resolves to an honest empty result — the
// one type that is empty by design, and documented as such.
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
type PodLogSource func(ctx context.Context, namespace, pod, container string, tail int64) (io.ReadCloser, error)

// PodLogStream streams a pod container's logs live (Follow:true) until ctx is
// cancelled. PodLogSource's tail-follow sibling — kept separate so the domain
// stays clientset-free. nil => FollowLogs reports core.ErrLogsUnavailable.
type PodLogStream func(ctx context.Context, namespace, pod, container string) (io.ReadCloser, error)

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
	// BuildNamespace is BEX_BUILD_NAMESPACE — where the operator runs pre-deploy
	// (and build) Job pods, so `type=predeploy` reads a migration's logs from the
	// right namespace (w1/m33). Empty falls back to the API's own namespace, the
	// operator's default when the env is unset.
	BuildNamespace string

	sseConns atomic.Int64
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
	App    string
	Types  []string  // empty == all types; values: app | request | build
	Search string    // case-insensitive substring on the message (Render's `text`)
	Since  time.Time // zero == no lower bound
	End    time.Time // zero == no upper bound
	Limit  int64     // max lines
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

// parseTimeWindow parses the optional startTime/endTime RFC3339 bounds shared
// by REST (`?startTime=&endTime=`) and GraphQL (`logs(startTime:, endTime:)`)
// — one parser so the two surfaces cannot drift on the accepted format or the
// error text naming the offending field (MCP's `list_logs` keeps its own copy:
// its error type, core.Err, differs from REST/GraphQL's core.ErrBadRequest
// wrap, so unifying it here would change its error shape). Empty stays the
// zero time (bound unset, LogQuery.Since/.End's own contract).
func parseTimeWindow(startTime, endTime string) (since, end time.Time, err error) {
	if startTime != "" {
		if since, err = time.Parse(time.RFC3339, startTime); err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: startTime: %s", core.ErrBadRequest, err)
		}
	}
	if endTime != "" {
		if end, err = time.Parse(time.RFC3339, endTime); err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: endTime: %s", core.ErrBadRequest, err)
		}
	}
	return since, end, nil
}

// validate rejects a filter value outside the accepted vocabulary — an unknown
// `direction` or log `type`. It runs inside the verbs (before normalized(), which
// coerces), so the refusal is the service's, not each adapter's: a fourth caller
// cannot forget it, and all three surfaces refuse identically. core.ErrBadRequest
// maps to 400 (adapters name the offending value in the message).
func (q LogQuery) validate() error {
	if _, err := core.ParseDirection(q.Direction); err != nil {
		return err
	}
	for _, t := range q.Types {
		if t != LogTypeApp && t != LogTypeRequest && t != LogTypeBuild && t != LogTypePreDeploy {
			return fmt.Errorf("%w: unknown log type %q (want %s|%s|%s|%s)", core.ErrBadRequest, t, LogTypeApp, LogTypeRequest, LogTypeBuild, LogTypePreDeploy)
		}
	}
	// Pre-deploy logs are a distinct live source (the migration Job's pod, w1/m33),
	// not merged with app/request/build — so it must be requested alone rather than
	// silently ignored inside a mixed query.
	if slices.Contains(q.Types, LogTypePreDeploy) && len(q.Types) > 1 {
		return fmt.Errorf("%w: log type %q must be requested on its own", core.ErrBadRequest, LogTypePreDeploy)
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
func (q LogQuery) tailSupports() error {
	if q.needsStore() {
		return fmt.Errorf("%w: the live tail reads pod logs, so it cannot stream request or build logs or filter by level/statusCode/method/path/host — query the logs API for those", core.ErrBadRequest)
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
		entries, err := s.History(ctx, s.Namespace, LogQuery{App: app.Name, Limit: tail}.normalized())
		return setLogResource(entries, resource), err
	}
	entries, err := s.collectPodLogs(ctx, LogQuery{App: app.Name}, tail)
	return setLogResource(entries, resource), err
}

// QueryLogs returns an App's log lines matching the filter set, oldest-first and
// capped at Limit. With the durable store wired it serves all log types (app +
// request + build) and every structured filter; in pod-log fallback mode it
// serves app logs and refuses what it cannot honor (ErrLogStoreUnavailable)
// rather than returning unfiltered lines. `type=build` without the store is a
// 503, not a silent empty — the same honesty rule as request logs.
func (s *Service) QueryLogs(ctx context.Context, q LogQuery) ([]LogEntry, error) {
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
	if err := q.validate(); err != nil {
		return nil, err
	}
	if s.History == nil && s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	q = q.normalized()
	// Pre-deploy step logs (w1/m33): a distinct LIVE source (the migration Job's
	// pod), read directly from the build namespace — never the durable store, which
	// has no predeploy stream — like the SSE tail always reads pod logs. validate()
	// guarantees predeploy is requested alone, so it owns the whole response.
	if slices.Contains(q.Types, LogTypePreDeploy) {
		entries, err := s.collectPreDeployLogs(ctx, q)
		return setLogResource(entries, resource), err
	}
	if s.History != nil {
		// The store applies every filter (labels + line) server-side and returns
		// oldest-first, capped at q.Limit.
		entries, err := s.History(ctx, s.Namespace, q)
		return setLogResource(entries, resource), err
	}
	if q.needsStore() {
		return nil, core.ErrLogStoreUnavailable
	}
	entries, err := s.collectPodLogs(ctx, q, q.Limit)
	if err != nil {
		return nil, err
	}
	return setLogResource(q.filterAndCap(entries), resource), nil
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
	app, err := s.AuthorizeApp(ctx, core.RelCanViewLogs, q.App)
	if err != nil {
		return nil, err
	}
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
	values, err := s.LabelValues(ctx, s.Namespace, label, q.normalized())
	if err != nil {
		return nil, err
	}
	return slices.Compact(slices.Sorted(slices.Values(values))), nil
}

// FollowLogs streams an App's new log lines to emit until ctx is cancelled or
// emit errors. The tail always reads live pod logs (never the store — real-time,
// zero ingest lag), so it serves app logs with the text/time/instance filters and
// refuses the store-only ones (ErrLogStoreUnavailable), exactly as the fallback
// query path does. Requires a PodLogStream (nil => core.ErrLogsUnavailable).
func (s *Service) FollowLogs(ctx context.Context, q LogQuery, emit func(LogEntry) error) error {
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
	// The tail's refusal is about the TRANSPORT, not the deployment: it reads pod
	// logs even when Loki is wired, so a store-only filter is something this stream
	// structurally cannot do — a bad request (400), not "the store is missing"
	// (503), which would be an outright lie on a cluster that has one.
	if err := q.tailSupports(); err != nil {
		return err
	}
	if !q.wants(LogTypeApp) {
		<-ctx.Done() // nothing to stream; hold until the client disconnects
		return ctx.Err()
	}

	pods, err := s.appPodNames(ctx, q)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan LogEntry, 64)
	var wg sync.WaitGroup
	for i := range pods {
		wg.Add(1)
		go func(pod string) {
			defer wg.Done()
			s.streamPodLogs(ctx, q.App, pod, ch)
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

// appLogResource is the public resource identity each returned log line carries.
// Store-managed and API-created Apps use their Render-shaped srv- id; legacy
// hand-applied CRs retain the identifier the caller used.
func appLogResource(labels map[string]string, requested string) string {
	if id := labels[core.LabelAppID]; id != "" {
		return id
	}
	return requested
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
}

// collectPodLogs reads up to tail lines from every replica of an App the query's
// `instance` filter admits, tagged and timestamp-sorted. Shared by Logs (MCP) and
// QueryLogs (REST/GraphQL).
func (s *Service) collectPodLogs(ctx context.Context, q LogQuery, tail int64) ([]LogEntry, error) {
	pods, err := s.appPodNames(ctx, q)
	if err != nil {
		return nil, err
	}
	var out []LogEntry
	for _, pod := range pods {
		entries, err := s.readPodLogs(ctx, q.App, pod, tail)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out, nil
}

// appPodNames lists the App's replica names the query's `instance` filter admits
// — the pod-log path's honoring of that filter (a pod name is a pod name, so this
// one structured filter needs no store).
func (s *Service) appPodNames(ctx context.Context, q LogQuery) ([]string, error) {
	pods, err := s.AppPods(ctx, q.App)
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

func (s *Service) readPodLogs(ctx context.Context, service, pod string, tail int64) ([]LogEntry, error) {
	return s.readContainerLogs(ctx, s.Namespace, service, pod, core.AppContainer, tail)
}

// readContainerLogs reads up to tail lines from one pod's container, tagged with
// service+pod. Generalizes readPodLogs so the pre-deploy path can read the
// "predeploy" container in the build namespace with the same parsing.
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
// the app pod-log path does. Live-only: a Job pod that has been TTL-reaped is
// simply gone (an empty read), never an error — the same ephemerality as build
// logs. Requires PodLogs to be wired (ErrLogsUnavailable otherwise).
func (s *Service) collectPreDeployLogs(ctx context.Context, q LogQuery) ([]LogEntry, error) {
	if s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	pods, err := s.PreDeployPods(ctx, q.App, s.BuildNamespace)
	if err != nil {
		return nil, err
	}
	ns := s.BuildNamespace
	if ns == "" {
		ns = s.Namespace
	}
	var out []LogEntry
	for i := range pods {
		pod := pods[i].Name
		entries, err := s.readContainerLogs(ctx, ns, q.App, pod, core.PreDeployContainer, q.Limit)
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
func (s *Service) streamPodLogs(ctx context.Context, service, pod string, ch chan<- LogEntry) {
	rc, err := s.PodLogsFollow(ctx, s.Namespace, pod, core.AppContainer)
	if err != nil {
		return
	}
	defer func() { _ = rc.Close() }()

	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return
		case ch <- parseLogLine(service, pod, sc.Text()):
		}
	}
}

// parseLogLine splits kubelet's "Timestamps: true" prefix (an RFC3339Nano stamp,
// a space, then the message) off a log line, tagging it with Render-shaped
// labels. A line without a parseable stamp is kept whole as the message.
func parseLogLine(service, pod, line string) LogEntry {
	ts, msg := "", line
	if i := strings.IndexByte(line, ' '); i > 0 {
		if t, err := time.Parse(time.RFC3339Nano, line[:i]); err == nil {
			ts = t.UTC().Format(time.RFC3339Nano)
			msg = line[i+1:]
		}
	}
	return LogEntry{
		Timestamp: ts,
		Message:   msg,
		Labels: map[string]string{
			"service":   service,
			"instance":  pod,
			"container": core.AppContainer,
			LabelType:   LogTypeApp, // the pod-log path reads the App container only
		},
	}
}
