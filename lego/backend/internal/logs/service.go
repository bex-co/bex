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
	"regexp"
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
	// than from a Loki `predeploy` stream, so it is requested alone (validate()
	// rejects mixing it with app/request/build). The same Job's stdout is also
	// shipped into Loki as LogTypeBuild by the Alloy build_pods pipeline
	// (w5/m100) for Render-compatible clients.
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

// DefaultTailRelistInterval is how often an app tail looks for new pods: the
// build tail's pod-wait cadence, so a new cron run's first line is at most ~2 s
// behind.
const DefaultTailRelistInterval = 2 * time.Second

// DefaultRequestPollInterval is how often the subscribe stream's store poll
// looks for new request lines (w4/m237): a request line is at most ~2 s of
// poll cadence (plus shipper ingest) behind the request it records.
const DefaultRequestPollInterval = 2 * time.Second

// DefaultTailHeartbeatInterval keeps a quiet tail well inside the shortest idle
// cutoff between bex-api and a client: Cloudflare, in front of api.bex.co, ends
// a proxied response that sends nothing for 100 seconds.
const DefaultTailHeartbeatInterval = 25 * time.Second

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
	// FollowLogs reports ErrLogsUnavailable. The app/build producers always read
	// pod logs — real-time, zero ingest lag, and they survive Loki being down;
	// request lines poll History instead (w4/m237).
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
	// TailRelistInterval is how often an app tail re-lists the App's pods to
	// attach to new ones (w1/m146). 0 = DefaultTailRelistInterval; tests shrink
	// it.
	TailRelistInterval time.Duration
	// TailHeartbeatInterval is how often an app tail writes a transport
	// keepalive, so a quiet stream is not cut by a proxy on the way to the
	// client (w1/m146). 0 = DefaultTailHeartbeatInterval; tests shrink it.
	TailHeartbeatInterval time.Duration
	// RequestPollInterval is how often the subscribe stream's store poll
	// looks for new request lines (w4/m237). 0 = DefaultRequestPollInterval;
	// tests shrink it.
	RequestPollInterval time.Duration
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
	Types    []string // empty == all types; values: app | request | build
	Search   []string // case-insensitive literal substrings, OR within Render's `text`
	// TextMode picks what Search compares against: TextModeRaw ("" — the
	// default) the stored message bytes, TextModeDisplay the terminal-display
	// text a log row shows (displayText). A Bex extension; see display.go.
	TextMode string
	Since    time.Time // zero == no lower bound
	End      time.Time // zero == no upper bound
	Limit    int64     // max lines
	// Direction picks which end of the window Limit keeps: DirectionBackward
	// (default, newest) or DirectionForward (oldest).
	Direction string

	// Structured filters. Each is a value set (OR within a filter, AND across
	// filters — Render's semantics); a `*` wildcard is supported per value.
	// Level applies to app and datastore logs; Host/StatusCode/Method/Path to request logs;
	// Instance to app and datastore logs (request logs originate at the edge).
	Level      []string
	Instance   []string
	Host       []string
	StatusCode []string
	Method     []string
	Path       []string

	// appID and notBefore scope an App read to its own incarnation (w8/m124):
	// a service recreated under a deleted one's name has a new id, and nothing
	// it owns predates its App. Set by scopeToApp, never by a caller.
	appID     string
	notBefore time.Time

	levelFilter *regexp.Regexp // datastore pod severity matcher, compiled once per query
	// probe lets a page read one line past the caller's (clamped) Limit, so the
	// page can tell whether any line lies beyond it (queryLogPage).
	probe bool
}

// validate rejects a filter value outside the accepted vocabulary — an unknown
// `direction` or log `type`. It runs inside the verbs (before normalized(), which
// coerces), so the refusal is the service's, not each adapter's: a fourth caller
// cannot forget it, and all three surfaces refuse identically. core.ErrBadRequest
// maps to 400 (adapters name the offending value in the message).
func (q LogQuery) validate() error {
	if err := core.ValidateInclusiveQueryRange(q.Since, q.End); err != nil {
		return err
	}
	if _, err := core.ParseDirection(q.Direction); err != nil {
		return err
	}
	if q.TextMode != "" && q.TextMode != TextModeRaw && q.TextMode != TextModeDisplay {
		return fmt.Errorf("%w: unknown textMode %q (want %s|%s)", core.ErrBadRequest, q.TextMode, TextModeRaw, TextModeDisplay)
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
// normalizes the text terms without mutating caller-owned slices.
func (q LogQuery) normalized() LogQuery {
	if q.Limit <= 0 {
		q.Limit = defaultLogLimit
	}
	limit := int64(maxLogLimit)
	if q.probe {
		limit++ // the one line past the caller's maximum
	}
	if q.Limit > limit {
		q.Limit = limit
	}
	if q.Direction != DirectionForward {
		q.Direction = DirectionBackward
	}
	q.Search = normalizeLogText(q.Search)
	return q
}

func normalizeLogText(values []string) []string {
	terms := make([]string, 0, len(values))
	for _, term := range values {
		if term != "" {
			terms = append(terms, strings.ToLower(term))
		}
	}
	slices.Sort(terms)
	return slices.Compact(terms)
}

// hasFilters reports whether any line-level filter (search/time/level) is set — the
// ones the pod-log path applies in Go.
func (q LogQuery) hasFilters() bool {
	return len(q.Search) > 0 || !q.Since.IsZero() || !q.End.IsZero() || q.levelFilter != nil
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
		q.hasStoreOnlyFilters()
}

// hasStoreOnlyFilters reports whether the query carries a structured filter
// only the log store can honor: the labels those filters select on are
// attached by the shipper on the way into the store, and are simply not
// present on a stream read straight off the kubelet.
func (q LogQuery) hasStoreOnlyFilters() bool {
	return len(q.Level) > 0 || len(q.Host) > 0 || len(q.StatusCode) > 0 ||
		len(q.Method) > 0 || len(q.Path) > 0
}

// onlyRequest reports whether the query asks for request logs and nothing a
// pod producer would also have to answer for.
func (q LogQuery) onlyRequest() bool {
	return len(q.Types) == 1 && q.Types[0] == LogTypeRequest
}

// liveSubscribable reports (as an error) when a subscription has no live
// producer for some part of what it asks. The stream has three producers —
// app pod stdout, build Job stdout, and a short-interval store poll for
// request logs (w4/m237) — so what stays refused is: a producerless type
// (e.g. type=predeploy), and store-only filters over anything the pod tail
// must also answer (the poll honors those filters only for request lines).
// Refusing up front keeps such a subscription from being accepted into an
// idle stream that holds a process-global SSE slot until the client
// disconnects (codex #3). Note: build logs CAN be tailed live (the build
// Job's pod stdout, w3/m14).
func (q LogQuery) liveSubscribable() error {
	// A datastore tail reads its pods' process log and applies the level
	// filter per line (compileLevelFilter), so its own contract decides.
	if isDatastoreResource(q.App) {
		return q.datastoreFilters(q.App)
	}
	if q.hasStoreOnlyFilters() && !q.onlyRequest() {
		return fmt.Errorf("%w: the live tail reads pod logs, so it cannot filter app or build lines by level/statusCode/method/path/host — query the logs API for those, or subscribe to request logs alone", core.ErrBadRequest)
	}
	if len(q.Types) > 0 && !q.wants(LogTypeApp) && !q.wants(LogTypeBuild) && !q.wants(LogTypeRequest) {
		return fmt.Errorf("%w: the live tail streams only app, build and request logs; type=%v has no live producer — use the historical logs query", core.ErrBadRequest, q.Types)
	}
	return nil
}

// keepPod reports whether a pod satisfies the `instance` filter.
// scopeToApp points q at app's own streams and pods: its CR name (the label
// the shipper and the pods carry), its id, and nothing before it existed.
func (q *LogQuery) scopeToApp(app *appv1alpha1.App) {
	q.App = app.Name
	q.appID = core.AppIncarnationID(app)
	q.notBefore = core.AppNotBefore(app)
}

// ownPods is the App's pods without an earlier incarnation's (core.OwnAppPods).
func (s *Service) ownPods(ctx context.Context, namespace string, q LogQuery) ([]corev1.Pod, error) {
	pods, err := s.AppPodsIn(ctx, namespace, q.App)
	if q.appID == "" {
		return pods, err
	}
	return core.DropOtherIncarnations(pods, q.appID, q.notBefore), err
}

func (q LogQuery) keepPod(pod string) bool {
	if len(q.Instance) == 0 {
		return true
	}
	return slices.Contains(q.Instance, pod)
}

// capToLimit keeps q.Limit entries from the end the direction asks for: the
// newest (backward, the default) or the oldest (forward). entries are oldest-first
// and stay that way — direction chooses which lines, not how they're ordered.
//
// When the cut lands inside a shared-timestamp group (fan-out across instances),
// the group is kept whole so page cursors (which are timestamps) cannot drop a
// sibling that shares the boundary instant (w4/m107).
func (q LogQuery) capToLimit(entries []LogEntry) []LogEntry {
	lim := lokiLimit(q)
	if int64(len(entries)) <= lim {
		return entries
	}
	if q.Direction == DirectionForward {
		end := lim
		t := entries[end-1].Timestamp
		for end < int64(len(entries)) && entries[end].Timestamp == t {
			end++
		}
		return entries[:end]
	}
	start := int64(len(entries)) - lim
	t := entries[start].Timestamp
	for start > 0 && entries[start-1].Timestamp == t {
		start--
	}
	return entries[start:]
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

// localTextSearch reports whether Loki must return raw candidate lines and let
// Go apply the final text matcher. Loki's regexp simplifier can turn some
// case-insensitive Unicode literals into byte contains filters that are not a
// superset of Go's strings.ToLower + literal contains contract (for example
// Greek iota folds through U+0345). Running the same matcher for raw and display
// history keeps the REST/GraphQL/MCP/CLI contract exact; scanLokiWindow still
// bounds the work and returns an honest partial page when it cannot cover the
// whole window.
func (q LogQuery) localTextSearch() bool {
	return len(q.Search) > 0
}

// matchesText reports whether message contains any normalized Search term, in
// the query's TextMode. No terms matches everything.
func (q LogQuery) matchesText(message string) bool {
	if len(q.Search) == 0 {
		return true
	}
	if q.TextMode == TextModeDisplay {
		message = displayText(message)
	}
	message = strings.ToLower(message)
	return slices.ContainsFunc(q.Search, func(term string) bool { return strings.Contains(message, term) })
}

// keep reports whether an entry satisfies the search/time/level filters. Assumes
// normalized() has run.
func (q LogQuery) keep(e LogEntry) bool {
	if q.levelFilter != nil && !q.levelFilter.MatchString(e.Labels[LabelLevel]) {
		return false
	}
	if !q.matchesText(e.Message) {
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
		q := LogQuery{Limit: tail}
		q.scopeToApp(app)
		entries, err := s.History(ctx, app.Namespace, q.normalized())
		// A tail is best-effort: the newest lines a budget-limited scan found
		// are the answer, not a failure.
		var partial *ScanIncompleteError
		if errors.As(err, &partial) {
			entries, err = partial.Entries, nil
		}
		return setLogResource(entries, resource), err
	}
	var q LogQuery
	q.scopeToApp(app)
	entries, err := s.collectPodLogs(ctx, app.Namespace, q, tail)
	return setLogResource(entries, resource), err
}

// QueryLogs returns an App's log lines matching the filter set, oldest-first and
// capped at Limit. With the durable store wired it serves all log types (app +
// request + build) and every structured filter; in pod-log fallback mode it
// serves app logs and refuses what it cannot honor (ErrLogStoreUnavailable)
// rather than returning unfiltered lines. `type=build` without the store is a
// 503, not a silent empty — the same honesty rule as request logs.
func (s *Service) QueryLogs(ctx context.Context, q LogQuery) ([]LogEntry, error) {
	if isDatastoreResource(q.App) {
		return s.queryDatastoreLogs(ctx, q)
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
	q.scopeToApp(app)
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
	// Pre-deploy step logs (w1/m33): a distinct LIVE source (the migration Job's
	// pod), read directly from the App's own namespace — the Job is co-located
	// with the App (ADR043 D8), not in the build namespace — never the durable
	// store, which has no predeploy stream — like the SSE tail always reads pod
	// logs. validate() guarantees predeploy is requested alone, so it owns the
	// whole response.
	if slices.Contains(q.Types, LogTypePreDeploy) {
		entries, err := s.collectPreDeployLogs(ctx, appNS, resource, q)
		return setLogResource(entries, resource), err
	}
	pods, err := s.ownPods(ctx, appNS, q)
	if err != nil {
		return nil, err
	}
	s.translateInstanceFilter(ctx, resource, appNS, &q, candidatesFromPods(pods))
	if s.History != nil {
		// The store applies every filter (labels + line) server-side and returns
		// oldest-first, capped at q.Limit.
		entries, err := s.History(ctx, appNS, q)
		if err != nil {
			return labelHistory(nil, err, resource)
		}
		entries = s.synthesizeProgress(ctx, q, resource, newProgressContext(app), entries)
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
	sort.SliceStable(out, func(i, j int) bool { return core.TimestampLess(out[i].Timestamp, out[j].Timestamp) })
	return setLogResource(q.filterAndCap(out), resource), nil
}

// queryDatastoreLogs is the one datastore-scoped core read behind the REST,
// GraphQL, and MCP log adapters, for a managed Postgres or Key Value alike.
// Render addresses every log-producing resource through the generic `resource`
// filter, so no parallel public API is needed. The datastore's authorization
// gate runs before either a Loki selector or a pod selector is constructed.
func (s *Service) queryDatastoreLogs(ctx context.Context, q LogQuery) ([]LogEntry, error) {
	target, err := s.datastoreTarget(ctx, &q)
	if err != nil {
		return nil, err
	}
	if err := q.compileLevelFilter(); err != nil {
		return nil, err
	}
	if s.History == nil && s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	q = q.normalized()
	pods, err := target.list(ctx)
	if err != nil {
		return nil, err
	}
	s.translateInstanceFilter(ctx, target.resource, target.namespace, &q, candidatesFromPods(pods))
	if s.History != nil {
		entries, err := s.History(ctx, target.namespace, q)
		return labelHistory(entries, err, target.resource)
	}
	entries, err := s.collectDatastorePodLogs(ctx, q, pods, target)
	if err != nil {
		return nil, err
	}
	return setLogResource(q.filterAndCap(entries), target.resource), nil
}

// datastoreTarget is the opening every managed-datastore read shares —
// history, label discovery and the live tail (w5/m139): authorize the `dpg-` or
// `red-` resource for can_view_logs, point q at the datastore's own streams,
// and validate q against the datastore contract. The target it answers says
// where the datastore's process log lives and how to follow it.
func (s *Service) datastoreTarget(ctx context.Context, q *LogQuery) (tailTarget, error) {
	requested := q.App
	var target tailTarget
	if isPostgresResource(requested) {
		database, err := s.AuthorizeDatabase(ctx, core.RelCanViewLogs, requested)
		if err != nil {
			return tailTarget{}, err
		}
		q.App, q.Database = "", database.Name
		target = tailTarget{
			namespace: database.Namespace, name: database.Name,
			container: core.CNPGPostgresContainer, logType: datastorelogs.KindPostgres,
			list: func(ctx context.Context) ([]corev1.Pod, error) {
				return s.DatabasePods(ctx, database.Namespace, database.Name)
			},
			revalidate: func(ctx context.Context) error {
				var fresh appv1alpha1.Database
				if err := s.Client.Get(ctx, client.ObjectKeyFromObject(database), &fresh); err != nil {
					return err
				}
				return s.AuthorizeDatabaseFresh(ctx, core.RelCanViewLogs, &fresh)
			},
		}
	} else {
		kv, err := s.AuthorizeKeyValue(ctx, core.RelCanViewLogs, requested)
		if err != nil {
			return tailTarget{}, err
		}
		q.App, q.KeyValue = "", kv.Name
		target = tailTarget{
			namespace: kv.Namespace, name: kv.Name,
			container: core.ValkeyContainer, logType: datastorelogs.KindKeyValue,
			list: func(ctx context.Context) ([]corev1.Pod, error) {
				return s.KeyValuePods(ctx, kv.Namespace, kv.Name)
			},
			revalidate: func(ctx context.Context) error {
				var fresh appv1alpha1.KeyValue
				if err := s.Client.Get(ctx, client.ObjectKeyFromObject(kv), &fresh); err != nil {
					return err
				}
				return s.AuthorizeKeyValueFresh(ctx, core.RelCanViewLogs, &fresh)
			},
		}
	}
	target.resource = requested
	// CNPG and Valkey pods change rarely, and a slower re-list only delays
	// re-attaching to a restarted container; each is an uncached pod LIST.
	target.relist = 5 * s.tailRelistInterval()
	if err := q.validate(); err != nil {
		return tailTarget{}, err
	}
	return target, q.datastoreFilters(requested)
}

// labelHistory attributes a durable-history read to the requested resource,
// including the covered part of a scan that ran out of time, which travels as
// a *ScanIncompleteError for the adapters to page on from (w4/m140).
func labelHistory(entries []LogEntry, err error, resource string) ([]LogEntry, error) {
	var partial *ScanIncompleteError
	if errors.As(err, &partial) {
		partial.Entries = setLogResource(partial.Entries, resource)
		return nil, partial
	}
	return setLogResource(entries, resource), err
}

func isPostgresResource(resource string) bool {
	kind, ok := ids.KindOf(resource)
	return ok && kind == ids.Postgres
}

func isKeyValueResource(resource string) bool {
	kind, ok := ids.KindOf(resource)
	return ok && kind == ids.KeyValue
}

func isDatastoreResource(resource string) bool {
	return isPostgresResource(resource) || isKeyValueResource(resource)
}

// datastoreFilters pins the supported managed-datastore contract: time range,
// direction, text search, level, limit, and instance. HTTP request/build
// filters are service concepts; refusing them is safer than silently dropping
// them. resource names the datastore family in the refusal.
func (q LogQuery) datastoreFilters(resource string) error {
	if len(q.Types) > 0 || len(q.Host) > 0 || len(q.StatusCode) > 0 || len(q.Method) > 0 || len(q.Path) > 0 {
		family := "Key Value"
		if isPostgresResource(resource) {
			family = "Postgres"
		}
		return fmt.Errorf("%w: managed %s logs support text, time range, direction, level, limit, and instance filters", core.ErrBadRequest, family)
	}
	return nil
}

// compileLevelFilter turns a datastore query's level filter into the
// line-level matcher its pod reads apply (the store applies it as a label
// matcher instead). Postgres and Key Value lines carry the shipper's level
// vocabulary, so storedLevels maps Render's names the same way for both.
func (q *LogQuery) compileLevelFilter() error {
	if len(q.Level) == 0 {
		return nil
	}
	filter, err := regexp.Compile(labelValuePattern(storedLevels(q.Level)))
	if err != nil {
		return fmt.Errorf("%w: invalid level filter: %v", core.ErrBadRequest, err)
	}
	q.levelFilter = filter
	return nil
}

// filterAndCap applies the line-level search/time/level filters (in place — skipping
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
	if isDatastoreResource(q.App) {
		return s.datastoreLogLabelValues(ctx, label, q)
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
	q.scopeToApp(app)
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

// datastoreLogLabelValues discovers a managed datastore's instance and level
// values: both datastores' lines carry a level (a Postgres record's severity,
// a Valkey line's marker — w5/m139), which only the store indexes.
func (s *Service) datastoreLogLabelValues(ctx context.Context, label string, q LogQuery) ([]string, error) {
	target, err := s.datastoreTarget(ctx, &q)
	if err != nil {
		return nil, err
	}
	if label != LabelInstance && label != LabelLevel {
		return nil, fmt.Errorf("%w: managed datastore log label %q is unsupported (want %s|%s)", core.ErrBadRequest, label, LabelInstance, LabelLevel)
	}
	if s.LabelValues != nil {
		values, err := s.LabelValues(ctx, target.namespace, label, q.normalized())
		if err != nil {
			return nil, err
		}
		if label == LabelInstance {
			return projectLogLabelValues(target.resource, values), nil
		}
		return slices.Compact(slices.Sorted(slices.Values(values))), nil
	}
	if label == LabelLevel {
		return nil, core.ErrLogStoreUnavailable
	}
	pods, err := target.list(ctx)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(pods))
	for i := range pods {
		values = append(values, pods[i].Name)
	}
	return projectLogLabelValues(target.resource, values), nil
}

// FollowLogs streams a resource's new log lines to emit until ctx is cancelled
// or emit errors. The tail always reads live pod logs (never the store —
// real-time, zero ingest lag). It serves app logs from App pods and type=build
// from the active build pod in BuildNamespace, with the text/time/instance
// filters, refusing store-only filters over those; and a managed datastore's
// process log with its instance/text/level filters (w5/m139). Requires a PodLogStream (nil =>
// core.ErrLogsUnavailable).
//
// An app tail stays open for as long as it is authorized and the App exists:
// it attaches to each app pod once that pod's container has started — the next
// cron run, a wake from hibernation, a new deploy, a scale-up — and never ends
// merely because every pod it was following has exited (w1/m146). Before, a
// cron job between runs closed the stream ~300 ms after replaying its last run,
// and the dashboard reconnected every ~3 s behind a "disconnected" banner.
func (s *Service) FollowLogs(ctx context.Context, q LogQuery, emit func(LogEntry) error) error {
	return s.followLogs(ctx, q, emit, nil)
}

// followLogs is FollowLogs with a transport keepalive. A non-nil heartbeat is
// called every TailHeartbeatInterval on an app tail, from the same goroutine
// that calls emit, so a transport can write its keepalive without racing a log
// frame.
func (s *Service) followLogs(ctx context.Context, q LogQuery, emit func(LogEntry) error, heartbeat func() error) error {
	if isDatastoreResource(q.App) {
		return s.followDatastore(ctx, q, emit, heartbeat)
	}
	requested := q.App
	app, err := s.AuthorizeApp(ctx, core.RelCanViewLogs, requested)
	if err != nil {
		return err
	}
	resource := appLogResource(app.Labels, requested)
	q.scopeToApp(app)
	if err := q.validate(); err != nil {
		return err
	}
	// Each producer needs its own source: pod lines (and the build tail) read
	// pod stdout, request lines poll the log store. A request-only stream on
	// a store-less deployment fails closed here, before any streaming header;
	// a mixed or all-types stream without a store still serves its pod lines
	// (the pre-poll behavior), just with no request producer.
	if q.onlyRequest() && s.History == nil {
		return core.ErrLogStoreUnavailable
	}
	if (q.wants(LogTypeApp) || q.wants(LogTypeBuild)) && s.PodLogsFollow == nil {
		return core.ErrLogsUnavailable
	}
	q = q.normalized()
	podsLive, err := s.ownPods(ctx, app.Namespace, q)
	if err != nil {
		return err
	}
	// The caller's instance selectors, before translation: a pod that starts
	// after subscribe is matched against these, not against the names that
	// resolved from the pods present now.
	selectors := slices.Clone(q.Instance)
	s.translateInstanceFilter(ctx, resource, app.Namespace, &q, candidatesFromPods(podsLive))
	// The refusal is about the PRODUCERS, not the deployment: store-only
	// filters over pod lines are something this stream structurally cannot
	// do — a bad request (400), not "the store is missing" (503), which
	// would be an outright lie on a cluster that has one.
	if err := q.liveSubscribable(); err != nil {
		return err
	}
	// Authorization lifetime (w4/034): admission authorized once above, but an
	// established tail must not outlive the grant. The watchdog re-checks FRESH
	// (uncached) on the interval; a deny, a checker failure, or the App's
	// deletion cancels the stream ctx, which ends every producer goroutine and
	// the emit loops below — and with them the caller's subscription cap slots.
	// An allowed re-check touches nothing: the tail is not interrupted.
	target := tailTarget{
		resource: resource, namespace: app.Namespace, name: app.Name,
		container: core.AppContainer, logType: LogTypeApp,
		list: func(ctx context.Context) ([]corev1.Pod, error) { return s.ownPods(ctx, app.Namespace, q) },
		revalidate: func(ctx context.Context) error {
			return s.revalidateTailTarget(ctx, app.Namespace, app.Name)
		},
		relist:       s.tailRelistInterval(),
		pollRequests: true,
	}
	ctx, stopWatchdog := withRevalidation(ctx, s.revalidateInterval(), target.revalidate)
	defer stopWatchdog()
	if len(q.Types) == 1 && q.Types[0] == LogTypeBuild {
		return s.followBuildLogs(ctx, q, resource, newProgressContext(app), emit)
	}
	if !q.wants(LogTypeApp) && !q.wants(LogTypeRequest) {
		// SECURITY (codex #3): a query wanting no produced type (e.g.
		// type=predeploy) has nothing to stream; it must NOT park on
		// ctx.Done() holding one of the process-global SSE slots until the
		// client disconnects. Refuse immediately so the slot is released.
		// (logsSubscribe also rejects this pre-slot via liveSubscribable;
		// this guards any other FollowLogs caller.)
		return fmt.Errorf("%w: the live tail streams only app, build and request logs; type=%v has no live producer — use the historical logs query", core.ErrBadRequest, q.Types)
	}

	// app.Namespace: the pods stream from their `<ws>` namespace under ADR043,
	// the same namespace translateInstanceFilter selected them in.
	return s.followPods(ctx, q, selectors, target, podsLive, emit, heartbeat)
}

// followDatastore is the live tail of a managed Postgres or Key Value
// (w5/m139): its pods' process log, followed and parsed as the historical pod
// read parses it, under the same authorization, filters and watchdog as an
// app tail. Render's /logs/subscribe takes a postgres or redis resource as it
// takes a service.
func (s *Service) followDatastore(ctx context.Context, q LogQuery, emit func(LogEntry) error, heartbeat func() error) error {
	target, err := s.datastoreTarget(ctx, &q)
	if err != nil {
		return err
	}
	if err := q.compileLevelFilter(); err != nil {
		return err
	}
	if s.PodLogsFollow == nil {
		return core.ErrLogsUnavailable
	}
	q = q.normalized()
	pods, err := target.list(ctx)
	if err != nil {
		return err
	}
	selectors := slices.Clone(q.Instance)
	s.translateInstanceFilter(ctx, target.resource, target.namespace, &q, candidatesFromPods(pods))
	ctx, stopWatchdog := withRevalidation(ctx, s.revalidateInterval(), target.revalidate)
	defer stopWatchdog()
	return s.followPods(ctx, q, selectors, target, pods, emit, heartbeat)
}

// podFollow is a pod tail's record of one pod it has attached to.
type podFollow struct {
	active   bool  // a follow goroutine is still reading the pod's log
	opened   bool  // the last follow opened a log (false: kubelet refused it)
	restarts int32 // the app container's restart count when it was attached
}

type followEnd struct {
	pod    string
	opened bool
}

// tailTarget is what a pod tail follows: one App's app container, or one
// managed datastore's process container (w5/m139).
type tailTarget struct {
	resource  string // the requested id, stamped on every emitted entry
	namespace string
	name      string // the CR name: the entries' service label
	container string
	logType   string
	list      func(context.Context) ([]corev1.Pod, error)
	// revalidate is the tail watchdog's fresh check: the target still exists
	// and the caller may still read its logs.
	revalidate func(context.Context) error
	relist     time.Duration
	// pollRequests adds the store poll's request lines: an App's edge traffic.
	pollRequests bool
}

// followPods is the pod tail's producer loop. It attaches a follow to every
// admitted pod whose target container has started, re-lists the target's pods
// every TailRelistInterval to attach to new ones, and ends only on ctx
// cancellation or an emit/heartbeat error.
//
// A pod is attached once per container life: a Succeeded cron run replays once
// per subscription, not on every re-list. A container that restarted after its
// follow ended is attached again — kubelet follows the current container, so
// the new follow reads only the new life's log — which is what a crash-looping
// service's tail used to get from the client reconnecting when its stream ended.
func (s *Service) followPods(ctx context.Context, q LogQuery, selectors []string, target tailTarget, pods []corev1.Pod, emit func(LogEntry) error, heartbeat func() error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan LogEntry, 64)
	ended := make(chan followEnd, 8)
	follows := make(map[string]*podFollow, len(pods))

	attach := func(pods []corev1.Pod) {
		for i := range pods {
			pod := &pods[i]
			if !q.keepPod(pod.Name) || !containerStarted(pod, target.container) {
				continue
			}
			restarts := containerRestarts(pod, target.container)
			if f := follows[pod.Name]; f != nil {
				restarted := f.opened && restarts > f.restarts
				// A follow kubelet refused is retried while the pod can still
				// produce output; a finished pod's unreadable log is not retried
				// every tick for the life of the subscription.
				refused := !f.opened && !podFinished(pod)
				if f.active || !(restarted || refused) {
					continue
				}
			}
			follows[pod.Name] = &podFollow{active: true, restarts: restarts}
			go func(name string) {
				var opened atomic.Bool
				s.streamContainerLogs(ctx, target.namespace, target.name, name, target.container, target.logType, q.Since, ch, &opened)
				select {
				case ended <- followEnd{pod: name, opened: opened.Load()}:
				case <-ctx.Done():
				}
			}(pod.Name)
		}
	}
	// A request-only subscription follows no pods: its only producer is the
	// store poll below.
	tailPods := !target.pollRequests || q.wants(LogTypeApp)
	if tailPods {
		attach(pods)
	}
	if target.pollRequests && q.wants(LogTypeRequest) && s.History != nil {
		go s.pollRequestLogs(ctx, q, target.namespace, ch)
	}

	relist := time.NewTicker(target.relist)
	defer relist.Stop()
	var beat <-chan time.Time
	if heartbeat != nil {
		ticker := time.NewTicker(s.tailHeartbeatInterval())
		defer ticker.Stop()
		beat = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-relist.C:
			if !tailPods {
				continue
			}
			current, err := target.list(ctx)
			if err != nil {
				continue // a failed list is retried next tick; the tail stays up
			}
			if len(selectors) > 0 {
				resolved := ids.ResolveInstanceSelectors(selectors, target.resource, candidatesFromPods(current))
				for _, name := range resolved {
					if !slices.Contains(q.Instance, name) {
						q.Instance = append(q.Instance, name)
					}
				}
			}
			attach(current)
		case end := <-ended:
			if f := follows[end.pod]; f != nil {
				f.active, f.opened = false, end.opened
			}
		case <-beat:
			if err := heartbeat(); err != nil {
				return err
			}
		case e := <-ch:
			if !q.keep(e) {
				continue
			}
			setLogEntryResource(&e, target.resource)
			if err := emit(e); err != nil {
				return err
			}
		}
	}
}

// requestCursor is the store poll's exactly-once cursor: the newest emitted
// instant plus the identities emitted at that instant. The store query's
// start is inclusive, so the boundary instant is re-read every poll; the
// seen set is what keeps its siblings from re-emitting (w4/m107's
// group-whole rule, from the emitting side). A same-instant group larger
// than one poll page can still lose its tail past the page — the store
// stamps at nanosecond precision, so a page-sized group at one instant is
// not a shape real traffic produces.
type requestCursor struct {
	at   time.Time
	seen map[string]struct{}
}

// logEntryKey is one log line's identity for exactly-once bookkeeping —
// shared with the store scan's boundary dedupe in loki.go.
func logEntryKey(e LogEntry) string {
	return e.Timestamp + "\x00" + e.Labels[LabelInstance] + "\x00" + e.Message
}

func (c *requestCursor) admit(e LogEntry) bool {
	ts, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	if err != nil {
		return false
	}
	if ts.Before(c.at) {
		return false
	}
	key := logEntryKey(e)
	if ts.After(c.at) {
		c.at = ts
		c.seen = map[string]struct{}{key: {}}
		return true
	}
	if _, dup := c.seen[key]; dup {
		return false
	}
	c.seen[key] = struct{}{}
	return true
}

// pollRequestLogs is the subscribe stream's store producer (w4/m237): every
// RequestPollInterval it reads the request lines that have reached the log
// store since the cursor and feeds the same merge channel the pod follows
// feed, so app and request lines interleave on one stream. It runs on the
// tail's revalidated ctx, so a revoked grant ends it with the pods. A store
// error retries next tick — a Loki blip must not tear down an established
// tail, exactly like a failed pod re-list.
func (s *Service) pollRequestLogs(ctx context.Context, q LogQuery, namespace string, ch chan<- LogEntry) {
	pq := q
	pq.Types = []string{LogTypeRequest}
	pq.Direction = DirectionForward
	pq.Limit = maxLogLimit
	pq.End = time.Time{}
	cursor := &requestCursor{at: q.Since, seen: map[string]struct{}{}}
	tick := func() {
		// Drain a burst within the tick: a full page with fresh lines means
		// the next page is already in the store.
		for range 10 {
			// Bound the scan window on a quiet subscription: the emitted
			// cursor barely moves while the store query's end is now, so an
			// unfloored Since would re-scan the subscription's whole life
			// every tick (and a path/host/text filter takes the sliced scan
			// path). One minute of overlap is far beyond shipper ingest lag;
			// the dedupe cursor still guarantees exactly-once inside it.
			pq.Since = cursor.at
			if floor := time.Now().Add(-time.Minute); pq.Since.Before(floor) {
				pq.Since = floor
			}
			entries, err := s.History(ctx, namespace, pq)
			if err != nil {
				return
			}
			fresh := 0
			for _, e := range entries {
				if !cursor.admit(e) {
					continue
				}
				fresh++
				select {
				case ch <- e:
				case <-ctx.Done():
					return
				}
			}
			if len(entries) < int(pq.Limit) || fresh == 0 {
				return
			}
		}
	}
	tick()
	ticker := time.NewTicker(s.requestPollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}

func (s *Service) requestPollInterval() time.Duration {
	if s.RequestPollInterval > 0 {
		return s.RequestPollInterval
	}
	return DefaultRequestPollInterval
}

// appContainerStarted reports whether a pod's app container has a log to
// follow: it is running or has run. A container still waiting (scheduling, an
// image pull) has none yet, and the pod is attached on a later re-list once it
// starts. A pod with no reported app container status is attempted unless it
// is Pending.
func containerStarted(pod *corev1.Pod, container string) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == container {
			return status.State.Waiting == nil
		}
	}
	return pod.Status.Phase != corev1.PodPending
}

func containerRestarts(pod *corev1.Pod, container string) int32 {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == container {
			return status.RestartCount
		}
	}
	return 0
}

func podFinished(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}

func (s *Service) tailRelistInterval() time.Duration {
	if s.TailRelistInterval > 0 {
		return s.TailRelistInterval
	}
	return DefaultTailRelistInterval
}

func (s *Service) tailHeartbeatInterval() time.Duration {
	if s.TailHeartbeatInterval > 0 {
		return s.TailHeartbeatInterval
	}
	return DefaultTailHeartbeatInterval
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
func (s *Service) followBuildLogs(ctx context.Context, q LogQuery, resource string, pc progressContext, emit func(LogEntry) error) error {
	prog := s.newProgressFollower(q, resource, pc)
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
// speaks. With no pod at all it keeps waiting while the deploy row is still
// open before or during its build (w8/020: a tail started while the deploy was
// queued used to end in a second). ErrBuildNotRunning is returned only when
// nothing is pending or running and the row is not building — the
// honestly-terminal case (no build, or it already completed and its history
// belongs to Loki). Each wait tick also re-reads the deploy row
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
		if err := prog.emitReached(ctx, emit); err != nil {
			return corev1.Pod{}, nil, err
		}
		if !waiting && !prog.buildOpen() {
			return corev1.Pod{}, nil, ErrBuildNotRunning
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
	sort.SliceStable(out, func(i, j int) bool { return core.TimestampLess(out[i].Timestamp, out[j].Timestamp) })
	return out, nil
}

// datastore names everything the managed Postgres and Key Value pod-log reads
// do differently: which instance, which container carries the process log, and
// which type label the entries are stamped with.

// collectDatastorePodLogs reads one managed datastore's logs from its pods.
// namespace is the datastore CR's own namespace (ADR043 D8) — reading from the
// shared one instead returns no error and no logs, which is how the App-side
// version of this bug stayed invisible.
func (s *Service) collectDatastorePodLogs(ctx context.Context, q LogQuery, pods []corev1.Pod, ds tailTarget) ([]LogEntry, error) {
	var out []LogEntry
	for i := range pods {
		if !q.keepPod(pods[i].Name) {
			continue
		}
		entries, err := s.readContainerLogs(ctx, ds.namespace, ds.name, pods[i].Name, ds.container, ds.logType, q.Limit)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	sort.SliceStable(out, func(i, j int) bool { return core.TimestampLess(out[i].Timestamp, out[j].Timestamp) })
	return out, nil
}

// appPodNames lists the App's replica names the query's `instance` filter admits
// — the pod-log path's honoring of that filter (a pod name is a pod name, so this
// one structured filter needs no store).
func (s *Service) appPodNames(ctx context.Context, namespace string, q LogQuery) ([]string, error) {
	pods, err := s.ownPods(ctx, namespace, q)
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
	return s.readContainerLogs(ctx, namespace, service, pod, core.AppContainer, LogTypeApp, tail)
}

// readContainerLogs reads up to tail lines from one pod's container, tagged with
// service+pod and labelled with that container and logType. Generalizes
// readPodLogs so the pre-deploy and datastore paths read their containers with
// the same parsing and say what they are.
func (s *Service) readContainerLogs(ctx context.Context, namespace, service, pod, container, logType string, tail int64) ([]LogEntry, error) {
	rc, err := s.PodLogs(ctx, namespace, pod, container, tail)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	parse := func(line string) (LogEntry, bool) {
		return parseContainerLogLine(service, pod, container, logType, line), true
	}
	if isDatastoreLog(logType) {
		parse = func(line string) (LogEntry, bool) { return datastoreLogEntry(service, pod, container, logType, line) }
	}
	var entries []LogEntry
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // allow long lines
	for sc.Scan() {
		if entry, keep := parse(sc.Text()); keep {
			entries = append(entries, entry)
		}
	}
	return entries, sc.Err()
}

func isDatastoreLog(logType string) bool {
	return logType == datastorelogs.KindPostgres || logType == datastorelogs.KindKeyValue
}

// datastoreLogEntry parses one datastore container line by the rule its own
// fallback reader applies: the kubelet's answers and CNPG's chatter are
// dropped, and a Postgres record is unwrapped, as the shipper does for history
// (w8/030), with its level — or a Valkey line's (w5/m139) — as a label.
func datastoreLogEntry(service, pod, container, kind, line string) (LogEntry, bool) {
	ts, msg, level, keep := datastorelogs.ParseLine(kind, line)
	if !keep {
		return LogEntry{}, false
	}
	entry := newLogEntry(service, pod, container, kind, ts, msg)
	if level != "" {
		entry.Labels[LabelLevel] = level
	}
	return entry, true
}

// collectPreDeployLogs reads the pre-deploy step's Job-pod logs (w1/m33) for an
// App, oldest-first and capped at q.Limit, applying the same text/time filters
// the app pod-log path does. appNS is the App CR's own namespace — the Job is
// co-located with the App (ADR043 D8), so its pods live there, NOT in the build
// namespace. Live-only: a Job pod that has been TTL-reaped is simply gone (an
// empty read), never an error — the same ephemerality as build logs. Requires
// PodLogs to be wired (ErrLogsUnavailable otherwise).
func (s *Service) collectPreDeployLogs(ctx context.Context, appNS, resource string, q LogQuery) ([]LogEntry, error) {
	if s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	pods, err := s.PreDeployPods(ctx, q.App, appNS)
	if err != nil {
		return nil, err
	}
	if len(q.Instance) > 0 {
		// This source reads only extant migration pods, so historical App
		// instance discovery cannot supply its candidates.
		q.Instance = ids.ResolveInstanceSelectors(q.Instance, resource, candidatesFromPods(pods))
		if len(q.Instance) == 0 {
			return nil, nil
		}
	}
	var out []LogEntry
	for i := range pods {
		pod := pods[i].Name
		if !q.keepPod(pod) {
			continue
		}
		entries, err := s.readContainerLogs(ctx, appNS, q.App, pod, core.PreDeployContainer, LogTypePreDeploy, q.Limit)
		if err != nil {
			// A reaped pod (or a container that never produced logs) drops out of
			// the read rather than failing the whole query.
			continue
		}
		out = append(out, entries...)
	}
	sort.SliceStable(out, func(i, j int) bool { return core.TimestampLess(out[i].Timestamp, out[j].Timestamp) })
	return q.filterAndCap(out), nil
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

	send := func(entry LogEntry) bool {
		select {
		case <-ctx.Done():
			return false
		case ch <- entry:
			return true
		}
	}
	// A follow starts at its since second, and the kubelet stamps nothing on
	// the rest of a split line whose first chunk it skipped as older. So an
	// unstamped first line is held: the kubelet's own answer when the stream
	// ends there, the tenant's text when more output follows.
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	// A datastore follow drops what its historical read drops, the kubelet's
	// unstamped answers included, so it has no first line to hold.
	if isDatastoreLog(logType) {
		for sc.Scan() {
			if entry, keep := datastoreLogEntry(service, pod, container, logType, sc.Text()); keep && !send(entry) {
				return
			}
		}
		return
	}
	var held *string
	for first := true; sc.Scan(); first = false {
		line := sc.Text()
		entry := parseContainerLogLine(service, pod, container, logType, line)
		if first && entry.Timestamp == "" {
			held = &line
			continue
		}
		if held != nil {
			if !send(newLogEntry(service, pod, container, logType, "", *held)) {
				return
			}
			held = nil
		}
		if !send(entry) {
			return
		}
	}
	if held != nil {
		send(parseContainerLogLine(service, pod, container, logType, *held))
	}
}

// maxLogMessageBytes caps one log record's message (round-5 finding 17). The
// stream/query scanners accept lines up to ~1 MiB; a tenant workload emitting
// repeated near-limit lines otherwise flows unbounded into every viewer (the
// dashboard/mobile clients cap by record COUNT, not bytes). 64 KiB is generous
// (containerd already splits raw lines at 16 KiB) and bounds every downstream
// consumer at the source. Truncation trims to a valid-UTF-8 boundary so the
// marker never rides a split rune.
const maxLogMessageBytes = 64 * 1024

// logsGoneMessage is the platform line that stands in for the kubelet's own
// answer: the untimestamped body, naming the internal containerd id or the
// /var/log/pods path, it returns once a container's log file no longer exists
// because the container was evicted or collected (w8/025, w8/050).
const logsGoneMessage = "==> logs for this instance are no longer available: its container was removed"

func parseContainerLogLine(service, pod, container, logType, line string) LogEntry {
	ts, msg, stamped := core.SplitPodLogLine(line)
	// Every line a container wrote carries the Timestamps:true prefix, so a
	// line without one is the kubelet's own answer, never the tenant's output.
	// It is re-typed as a platform line and never shows the containerd id or
	// the node's paths (w8/025). The rule is the line's source, not its words
	// (w5/m122): the shipper reads containers' files, which hold only what
	// they wrote.
	if !stamped {
		container, msg = progressContainer, logsGoneMessage
	}
	return newLogEntry(service, pod, container, logType, ts, msg)
}

// capLogMessage applies maxLogMessageBytes. Every reader — live pod stream,
// fallback, and durable history — returns messages through it, so one emitted
// record has one message shape (and one dashboard key) on either path (w4/256).
func capLogMessage(msg string) string {
	if len(msg) <= maxLogMessageBytes {
		return msg
	}
	return strings.ToValidUTF8(msg[:maxLogMessageBytes], "") + " …[truncated]"
}

// newLogEntry is one container record, its message capped at
// maxLogMessageBytes.
func newLogEntry(service, pod, container, logType, ts, msg string) LogEntry {
	return LogEntry{
		Timestamp: ts,
		Message:   capLogMessage(msg),
		Labels: map[string]string{
			"service":   service,
			"instance":  pod,
			"container": container,
			LabelType:   logType,
		},
	}
}
