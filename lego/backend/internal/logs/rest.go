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
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
)

// wsUpgrader upgrades the subscribe connection to WebSocket when the Render
// CLI (v2+) requests it. Leaving CheckOrigin nil uses Gorilla's safe default:
// requests without an Origin (the official CLI) are accepted, while browser
// handshakes must have an Origin whose host matches the API host. The HTTP auth
// gate remains the authorization boundary; Origin is an independent browser
// request-integrity check.
var wsUpgrader = websocket.Upgrader{}

// rest.go is the REST logs adapter — Render logs-API compatible. It maps the
// query string (resource/type/text/startTime/endTime/limit) and the {hasMore,
// next*Time, logs} envelope onto QueryLogs, and serves a live tail over SSE.

// RegisterREST mounts the logs query, label-value discovery, and live-tail routes.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/logs", s.logsQuery)
	mux.HandleFunc("GET /v1/logs/values", s.logsValues)
	mux.HandleFunc("GET /v1/logs/subscribe", s.logsSubscribe)
}

// renderLookback is how far back a read on Render's logs surfaces reaches when
// it names no startTime: Render's List logs and List log label values document
// startTime as "Defaults to `now() - 1 hour`", and its MCP server's list_logs
// and list_log_label_values as "Defaults to 1 hour ago" (w5/197).
const renderLookback = time.Hour

// renderWindow resolves a read's window on Render's logs surfaces and bounds
// it by BEX_MAX_QUERY_HOURS. Every REST log read goes through it — the
// historical query AND label discovery — and so do GraphQL's
// `logs`/`logLabelValues` (graphql.go, w9/m1/t002) and MCP's
// `list_logs`/`list_log_label_values` (mcp.go, w9/004).
//
// An omitted startTime reads the hour before the earlier of endTime and now:
// Render's `now() - 1 hour` whenever the window ends now or later, and a
// window that is never inverted when an endTime alone lies further back
// (w8/120). Setting it here, before the read, is what makes the store, the
// pod-log fallback and the page cursors all name that one hour.
//
// A start more than the cap before the part of the window that can hold
// lines, which ends at now (w8/123), is clamped to the cap, never refused:
// Render's List logs documents no range limit, so `--start 30d` or a start a
// year back reads what is kept (w8/374). The default cap, 720 hours, is the
// longest plan window and the store's retention, so the clamp hides no kept
// line; the plan's own floor narrows the read further (floorAt). Only an
// inverted window is refused, and one that has not begun stays as asked: it
// holds no lines yet (w8/120).
func (s *Service) renderWindow(q *LogQuery) error {
	end := time.Now()
	if !q.End.IsZero() {
		end = earlier(q.End, end)
	}
	if q.Since.IsZero() {
		q.Since = end.Add(-renderLookback)
	}
	if err := core.ValidateInclusiveQueryRange(q.Since, q.End); err != nil {
		return err
	}
	if s.MaxQueryHours > 0 {
		q.Since = later(q.Since, end.Add(-time.Duration(s.MaxQueryHours)*time.Hour))
	}
	return nil
}

// logsValues serves GET /v1/logs/values — Render's filter-value discovery: the
// values one label takes among the logs the same filter set selects. Render
// returns a bare JSON string array, and so does bex.
func (s *Service) logsValues(w http.ResponseWriter, r *http.Request) {
	scopeReq, q, err := parseLogParams(r)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	if err := s.renderWindow(&q); err != nil {
		core.WriteErr(w, err)
		return
	}
	v := r.URL.Query()
	label := v.Get("label")
	if label == "" {
		core.WriteErr(w, fmt.Errorf("%w: label is required", core.ErrBadRequest))
		return
	}

	ctx := core.WithRequestMemo(core.WithWorkspace(core.WithAuditVerb(r.Context(), AuditVerbLogLabelValues), v.Get("ownerId")))
	resolved, err := s.resolveLogScope(ctx, scopeReq, &q, nil)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	if resolved.Scoped {
		if err := checkLabel(label, DiscoverableLabels); err != nil {
			core.WriteErr(w, err)
			return
		}
		if err := q.validate(); err != nil {
			core.WriteErr(w, err)
			return
		}
	}
	all := []string{}
	for _, res := range resolved.IDs {
		if resolved.Scoped && core.IsDatastoreResource(res) && !slices.Contains(datastoreLogLabels, label) {
			continue
		}
		values, err := s.LogLabelValues(ctx, label, q.forResource(res))
		if err != nil {
			if resolved.Scoped && (errors.Is(err, core.ErrForbidden) || errors.Is(err, core.ErrNotFound)) {
				continue
			}
			core.WriteErr(w, err)
			return
		}
		all = append(all, values...)
	}
	slices.Sort(all)
	all = slices.Compact(all)
	// Render's limitParam on a bare array: at most `limit` values (20 when
	// absent or below one, as /v1/logs reads it), the first in sorted order;
	// Render does not document its order. GraphQL and MCP, which call
	// LogLabelValues directly, keep every value.
	if limit := core.PageLimitOrAbsent(int(min(q.Limit, core.MaxPageLimit))); len(all) > limit {
		all = all[:limit]
	}
	core.WriteJSON(w, http.StatusOK, all)
}

// logsQuery serves GET /v1/logs — a historical query across App, managed
// Postgres, Key Value, or a whole project/environment, matching Render's
// resource/project/environment/region contract.
func (s *Service) logsQuery(w http.ResponseWriter, r *http.Request) {
	scopeReq, q, err := parseLogParams(r)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	if err := s.renderWindow(&q); err != nil {
		core.WriteErr(w, err)
		return
	}

	// Render's `resource` / `project` resolve to a bounded set of resource ids;
	// queryLogPage merges each resource's lines, then sorts and caps. QueryLogs
	// gets the query as parsed: normalizing here first would coerce an invalid
	// `direction` to the default before the verb could refuse it.
	ctx := core.WithRequestMemo(core.WithWorkspace(core.WithAuditVerb(r.Context(), AuditVerbQueryLogs), r.URL.Query().Get("ownerId")))
	resolved, err := s.resolveLogScope(ctx, scopeReq, &q, nil)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	if len(resolved.IDs) == 0 {
		if err := q.validate(); err != nil {
			core.WriteErr(w, err)
			return
		}
		n := q.normalized()
		page := logPage{Entries: []LogEntry{}}
		page.NextStartTime, page.NextEndTime = pageCursors(nil, n.Since, n.End, n.Direction)
		core.WriteJSON(w, http.StatusOK, toRenderLogList(page))
		return
	}
	page, err := s.queryLogPage(ctx, resolved.IDs, q)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, toRenderLogList(page))
}

// logsSubscribe serves GET /v1/logs/subscribe — a live tail of every
// requested resource (or a project/environment scope), merged onto one stream
// in arrival order (w8/024, w4/m596). Three wire formats are supported:
//
//   - WebSocket upgrade → WS text frames, one per log line; the Render CLI
//     v2+ sends Upgrade: websocket and expects this protocol.
//   - Accept: text/event-stream → SSE (`data: <JSON>\n\n` per line); for
//     browser EventSource and curl -N.
//   - Anything else → NDJSON (`<JSON>\n` per line).
func (s *Service) logsSubscribe(w http.ResponseWriter, r *http.Request) {
	// Mounted as a method value, this has no verb method beneath it: the tail's
	// authorizations name FollowLogs, as the other surfaces' tail does (w21/093).
	ctx := core.WithRequestMemo(core.WithWorkspace(core.WithAuditVerb(r.Context(), AuditVerbFollowLogs), r.URL.Query().Get("ownerId")))
	r = r.WithContext(ctx)
	v := r.URL.Query()
	if v.Has("project") && v.Has("filterDocument") {
		s.projectSubscribe(w, r)
		return
	}
	scopeReq, q, err := parseLogParams(r)
	if err != nil {
		core.WriteErr(w, err)
		return
	}

	// Resume where this client's previous connection stopped. A browser
	// EventSource reconnects on its own — invisibly to the page — and replays
	// the last `id:` we emitted as Last-Event-ID; honoring it is what keeps a
	// reconnect from re-reading the pod's whole log from offset 0 (w6/m93).
	//
	// An explicit startTime is the window's LOWER bound, not a competing cursor:
	// the browser now sends it on every connect (w6/m111), so it rides along on
	// the invisible reconnects too. Take the LATER of the two — the resume never
	// re-reads below the caller's window, and a window never re-reads what the
	// tail already delivered. With no startTime (a CLI/NDJSON client) the resume
	// is the only bound, and on the WebSocket transport with neither the tail
	// begins when it opens (followLogs, w8/288); with a startTime and no resume
	// (the first connect) the window is the bound.
	resumeTail(r, &q)
	// Refuse invalid bounds before writing streaming headers or upgrading to
	// WebSocket; FollowLogs' validation would otherwise run after HTTP 200/101.
	if err := q.validate(); err != nil {
		core.WriteErr(w, err)
		return
	}

	resolved, err := s.resolveLogScope(ctx, scopeReq, &q, nil)
	if err != nil {
		core.WriteErr(w, err)
		return
	}

	queries := make([]LogQuery, len(resolved.IDs))
	for i, resource := range resolved.IDs {
		queries[i] = q.forResource(resource)
	}
	if err := s.checkLiveSubscribable(queries); err != nil {
		core.WriteErr(w, err)
		return
	}

	if resolved.Scoped {
		if err := s.checkRequestLogsPlan(ctx, resolved.WorkspaceID, q); err != nil {
			core.WriteErr(w, err)
			return
		}
		release, err := s.acquireSlot(r.Context(), func() (string, error) { return resolved.WorkspaceID, nil })
		if err != nil {
			writeSubscriptionRefusal(w, err)
			return
		}
		defer release()
		isWS := websocket.IsWebSocketUpgrade(r)
		if isWS {
			q.renderStream = true
		}
		follow := func(ctx context.Context, emit func(LogEntry) error, heartbeat func() error) error {
			return s.followProjectScope(ctx, scopeReq, q, resolved.Resources, emit, heartbeat)
		}
		if isWS {
			s.subscribeWebSocket(w, r, follow)
			return
		}
		s.subscribeStream(w, r, follow)
		return
	}

	// Admission retains a final process ceiling and adds caller/workspace shares
	// after resource authorization, leaving capacity for unrelated tenants —
	// one slot per followed resource, so a multi-resource tail costs what that
	// many single tails would. Check the workspace plan's request-log gate right
	// after resource authorization so a Hobby refusal answers HTTP 400 before
	// any streaming header or WebSocket 101 (w32/102).
	for i := range queries {
		workspace, release, err := s.acquireSubscriptionWorkspace(r.Context(), queries[i])
		if err != nil {
			writeSubscriptionRefusal(w, err)
			return
		}
		defer release()
		if err := s.checkRequestLogsPlan(ctx, workspace, queries[i]); err != nil {
			core.WriteErr(w, err)
			return
		}
	}

	follow := func(ctx context.Context, emit func(LogEntry) error, heartbeat func() error) error {
		return s.followAll(ctx, queries, emit, heartbeat)
	}
	// Render CLI v2+ sends an upgrade request; everyone else gets SSE or NDJSON.
	if websocket.IsWebSocketUpgrade(r) {
		// Render's own transport: Render's subscription semantics (renderStream).
		for i := range queries {
			queries[i].renderStream = true
		}
		s.subscribeWebSocket(w, r, follow)
		return
	}
	s.subscribeStream(w, r, follow)
}

// checkLiveSubscribable refuses, before a slot or a header, a subscription
// with no live producer for one of its resources.
//
// SECURITY (codex #3): such a request (type=predeploy, say) would otherwise
// park on an idle stream and hold one of the process-global SSE slots until
// it chose to disconnect, letting one tenant exhaust the pool. Every resource
// answers to its own contract — a datastore's filters (w5/m139), an App's
// routed to the producers whose lines carry them (w8/m147) — and the check
// reads only the id's kind and the query, so it confirms nothing about
// existence. The request producer polls the log store: with none wired, a
// request-only subscription (request filters alone narrow to one) fails
// closed too, after every resource's own 400. Mixed streams still serve their
// pod lines without it.
func (s *Service) checkLiveSubscribable(queries []LogQuery) error {
	requestOnly := false
	for _, q := range queries {
		routed, err := q.liveSubscribable()
		if err != nil {
			return err
		}
		requestOnly = requestOnly || routed.onlyRequest()
	}
	if requestOnly && s.History == nil {
		return core.ErrLogStoreUnavailable
	}
	return nil
}

// serializedWriters wraps a tail's emit and heartbeat so the followers of a
// multi-resource tail, each in its own goroutine, write to the one transport
// one at a time. A nil heartbeat stays a no-op.
func serializedWriters(emit func(LogEntry) error, heartbeat func() error) (func(LogEntry) error, func() error) {
	var mu sync.Mutex
	serialized := func(write func() error) error {
		mu.Lock()
		defer mu.Unlock()
		return write()
	}
	return func(e LogEntry) error { return serialized(func() error { return emit(e) }) },
		func() error {
			if heartbeat == nil {
				return nil
			}
			return serialized(heartbeat)
		}
}

// tailFollow is what a subscription transport streams: a follow that calls
// emit per line and heartbeat on its keepalive, until the context ends or the
// tail fails.
type tailFollow func(ctx context.Context, emit func(LogEntry) error, heartbeat func() error) error

// resumeTail lowers q's bound to where this client's previous connection
// stopped (the SSE Last-Event-ID), never below the caller's own window.
func resumeTail(r *http.Request, q *LogQuery) {
	if resume, ok := resumeFrom(r.Header.Get("Last-Event-ID")); ok && resume.After(q.Since) {
		q.Since = resume
	}
}

// projectSubscribe is /v1/logs/subscribe for a project view (w3/m505, a Bex
// extension: Render's public API has no project tail): `project`, the view's
// `filterDocument` (JSON, Render's empty default when absent) and `type`
// (app or request). The scope is resolved and every resource's filters
// checked before any streaming header, and the whole tail holds one
// subscription slot; followProject re-resolves the scope while it runs.
func (s *Service) projectSubscribe(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	if v.Has("resource") {
		core.WriteErr(w, fmt.Errorf("%w: a subscription takes exactly one of resource and project", core.ErrBadRequest))
		return
	}
	if err := refuseUnsupportedLogFilters(v); err != nil {
		core.WriteErr(w, err)
		return
	}
	q, err := parseLogQuery(v)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	var rawDoc any
	if raw := v.Get("filterDocument"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &rawDoc); err != nil {
			core.WriteErr(w, fmt.Errorf("%w: filterDocument is not JSON: %v", core.ErrBadRequest, err))
			return
		}
	}
	ctx := core.WithRequestMemo(r.Context())
	view, err := newProjectView(v.Get("project"), rawDoc, &q)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	resumeTail(r, &q)
	if err := q.validate(); err != nil {
		core.WriteErr(w, err)
		return
	}
	workspace, resources, err := s.resolveProjectView(ctx, view, nil)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	queries := make([]LogQuery, len(resources))
	for i, res := range resources {
		queries[i] = q.forResource(res.ID)
	}
	if err := s.checkLiveSubscribable(queries); err != nil {
		core.WriteErr(w, err)
		return
	}
	release, err := s.acquireSlot(r.Context(), func() (string, error) { return workspace, nil })
	if err != nil {
		writeSubscriptionRefusal(w, err)
		return
	}
	defer release()
	follow := func(ctx context.Context, emit func(LogEntry) error, heartbeat func() error) error {
		return s.followProject(ctx, view, q, resources, emit, heartbeat)
	}
	if websocket.IsWebSocketUpgrade(r) {
		s.subscribeWebSocket(w, r, follow)
		return
	}
	s.subscribeStream(w, r, follow)
}

// followAll is followLogs over every subscribed resource, merged: one follower
// per resource, their frames and keepalives serialized onto the single
// transport writer. The first follower to fail ends the whole tail with its
// error (the transports turn it into their refusal frame); a client
// disconnect ends every follower quietly.
func (s *Service) followAll(ctx context.Context, queries []LogQuery, emit func(LogEntry) error, heartbeat func() error) error {
	// Each follower runs in a goroutine of its own, with no verb method beneath it.
	ctx = core.WithAuditVerb(ctx, AuditVerbFollowLogs)
	if len(queries) == 1 {
		return s.followLogs(ctx, queries[0], emit, heartbeat)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	emitOne, beat := serializedWriters(emit, heartbeat)
	done := make(chan error, len(queries))
	for _, q := range queries {
		go func() {
			done <- s.followLogs(ctx, q, emitOne, beat)
		}()
	}
	var first error
	for range queries {
		if err := <-done; err != nil && first == nil && !errors.Is(err, context.Canceled) {
			first = err
			cancel()
		}
	}
	return first
}

// resumeFrom parses an SSE Last-Event-ID back into the lower bound it stands
// for. The id is the log line's own RFC3339Nano timestamp, so a resume is just
// a query window — no server-side cursor state to keep or expire. An
// unparseable value (a client that invented one, or a frame from a build that
// carried no stamp) is ignored rather than refused: the worst case is the
// unbounded read this exists to avoid, never a failed subscription.
func resumeFrom(lastEventID string) (time.Time, bool) {
	if lastEventID == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, lastEventID)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

var errSubscriptionLimit = core.ErrLogSubscriptionLimit

const (
	subscriptionStoreTimeout = 2 * time.Second
	subscriptionWarnInterval = 30 * time.Second
)

var lastSubscriptionStoreWarn atomic.Int64

func warnSubscriptionStoreFallback(op string, err error) {
	now := time.Now().UnixNano()
	prev := lastSubscriptionStoreWarn.Load()
	if now-prev >= int64(subscriptionWarnInterval) && lastSubscriptionStoreWarn.CompareAndSwap(prev, now) {
		log.Printf("bex-api: shared log-subscription store %s failed (falling back to local caps): %v", op, err)
	}
}

func newSubscriptionSlotID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("slot-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func (s *Service) subscriptionNow() time.Time {
	if s.SubscriptionNow != nil {
		return s.SubscriptionNow()
	}
	return time.Now()
}

func (s *Service) subscriptionLeaseTimings() (ttl, renew time.Duration) {
	ttl = s.SubscriptionLeaseTTL
	if ttl <= 0 {
		ttl = DefaultSubscriptionLeaseTTL
	}
	renew = s.SubscriptionRenewInterval
	if renew <= 0 {
		renew = DefaultSubscriptionRenewInterval
	}
	if renew >= ttl {
		renew = max(time.Millisecond, ttl/3)
	}
	return ttl, renew
}

// writeSubscriptionRefusal answers a subscription admission refused: 429 at
// capacity, the error's own status otherwise.
func writeSubscriptionRefusal(w http.ResponseWriter, err error) {
	if errors.Is(err, errSubscriptionLimit) {
		// Render's one error dialect ({error,message,id}) on every branch, so a
		// Render client reads .message here too, not a bare {error} (w9/m39).
		core.WriteErrStatus(w, http.StatusTooManyRequests, "too many active log subscriptions")
		return
	}
	core.WriteErr(w, err)
}

func (s *Service) checkRequestLogsPlan(ctx context.Context, workspaceID string, q LogQuery) error {
	if !slices.Contains(q.Types, LogTypeRequest) && len(q.requestFilters()) == 0 {
		return nil
	}
	wp, err := s.workspacePlanOf(ctx, workspaceID)
	if err != nil || !wp.ok {
		return err
	}
	if !core.PlanAllows(wp.plan, core.FeatureRequestLogs) {
		return core.NewPlanFeatureError("type", wp.plan, core.FeatureRequestLogs)
	}
	return nil
}

func (s *Service) acquireSubscription(ctx context.Context, q LogQuery) (func(), error) {
	_, release, err := s.acquireSubscriptionWorkspace(ctx, q)
	return release, err
}

func (s *Service) acquireSubscriptionWorkspace(ctx context.Context, q LogQuery) (string, func(), error) {
	ctx = core.WithAuditVerb(ctx, AuditVerbFollowLogs)
	return s.acquireSlotWithWorkspace(ctx, func() (string, error) { return s.subscriptionWorkspace(ctx, q.App) })
}

// acquireSlot takes one subscription slot for the caller in the workspace
// workspaceOf answers: global, per-caller, and per-workspace caps. The local
// global cap is checked first so a full process refuses before authorization
// runs; when SubscriptionStore is wired (w8/m579), all three caps are enforced
// across replicas via Postgres TTL leases with heartbeat renewal, falling back
// fail-open to local counters on store outage.
func (s *Service) acquireSlot(ctx context.Context, workspaceOf func() (string, error)) (func(), error) {
	_, release, err := s.acquireSlotWithWorkspace(ctx, workspaceOf)
	return release, err
}

func (s *Service) acquireSlotWithWorkspace(ctx context.Context, workspaceOf func() (string, error)) (string, func(), error) {
	global := false
	if s.MaxSSEConns > 0 {
		if s.sseConns.Add(1) > s.MaxSSEConns {
			s.sseConns.Add(-1)
			return "", nil, errSubscriptionLimit
		}
		global = true
	}
	releaseGlobal := func() {
		if global {
			s.sseConns.Add(-1)
		}
	}
	workspace, err := workspaceOf()
	if err != nil {
		releaseGlobal()
		return "", nil, err
	}
	identity, _ := core.IdentityFrom(ctx)
	subject := identity.Subject

	if s.SubscriptionStore != nil && (s.MaxSSEConns > 0 || s.MaxSSEConnsPerSubject > 0 || s.MaxSSEConnsPerWorkspace > 0) {
		ttl, renewInterval := s.subscriptionLeaseTimings()
		slotID := newSubscriptionSlotID()
		storeCtx, cancel := context.WithTimeout(ctx, subscriptionStoreTimeout)
		acqErr := s.SubscriptionStore.AcquireLogSubscriptionSlot(
			storeCtx, s.subscriptionNow(), slotID, subject, workspace,
			ttl, s.MaxSSEConns, s.MaxSSEConnsPerSubject, s.MaxSSEConnsPerWorkspace,
		)
		cancel()
		switch {
		case acqErr == nil:
			renewCtx, cancelRenew := context.WithCancel(context.Background())
			doneCh := make(chan struct{})
			go func() {
				defer close(doneCh)
				ticker := time.NewTicker(renewInterval)
				defer ticker.Stop()
				for {
					select {
					case <-renewCtx.Done():
						return
					case <-ctx.Done():
						return
					case <-ticker.C:
						tCtx, tCancel := context.WithTimeout(renewCtx, subscriptionStoreTimeout)
						if rErr := s.SubscriptionStore.RenewLogSubscriptionSlot(tCtx, slotID, s.subscriptionNow().Add(ttl)); rErr != nil && !errors.Is(rErr, context.Canceled) {
							warnSubscriptionStoreFallback("renew", rErr)
						}
						tCancel()
					}
				}
			}()
			var once sync.Once
			return workspace, func() {
				once.Do(func() {
					cancelRenew()
					<-doneCh
					relCtx, relCancel := context.WithTimeout(context.Background(), subscriptionStoreTimeout)
					if rErr := s.SubscriptionStore.ReleaseLogSubscriptionSlot(relCtx, slotID); rErr != nil {
						warnSubscriptionStoreFallback("release", rErr)
					}
					relCancel()
					releaseGlobal()
				})
			}, nil
		case errors.Is(acqErr, core.ErrLogSubscriptionLimit):
			releaseGlobal()
			return "", nil, errSubscriptionLimit
		default:
			warnSubscriptionStoreFallback("acquire", acqErr)
			// Fall through fail-open to local per-subject/per-workspace counters.
		}
	}

	s.sseMu.Lock()
	if s.sseBySubject == nil {
		s.sseBySubject = make(map[string]int)
		s.sseByWorkspace = make(map[string]int)
	}
	if s.MaxSSEConnsPerSubject > 0 && subject != "" && s.sseBySubject[subject] >= s.MaxSSEConnsPerSubject {
		s.sseMu.Unlock()
		releaseGlobal()
		return "", nil, errSubscriptionLimit
	}
	if s.MaxSSEConnsPerWorkspace > 0 && s.sseByWorkspace[workspace] >= s.MaxSSEConnsPerWorkspace {
		s.sseMu.Unlock()
		releaseGlobal()
		return "", nil, errSubscriptionLimit
	}
	if s.MaxSSEConnsPerSubject > 0 && subject != "" {
		s.sseBySubject[subject]++
	}
	if s.MaxSSEConnsPerWorkspace > 0 {
		s.sseByWorkspace[workspace]++
	}
	s.sseMu.Unlock()

	var once sync.Once
	return workspace, func() {
		once.Do(func() {
			s.sseMu.Lock()
			if s.MaxSSEConnsPerSubject > 0 && subject != "" {
				decrementSubscriptionKey(s.sseBySubject, subject)
			}
			if s.MaxSSEConnsPerWorkspace > 0 {
				decrementSubscriptionKey(s.sseByWorkspace, workspace)
			}
			s.sseMu.Unlock()
			releaseGlobal()
		})
	}, nil
}

func (s *Service) subscriptionWorkspace(ctx context.Context, resource string) (string, error) {
	owner, err := s.authorizeLogOwner(ctx, resource)
	if err != nil {
		return "", err
	}
	if workspace := owner.GetLabels()[core.LabelTenant]; workspace != "" {
		return workspace, nil
	}
	if namespace := owner.GetNamespace(); namespace != "" {
		return namespace, nil
	}
	return core.DefaultTenant, nil
}

func decrementSubscriptionKey(counts map[string]int, key string) {
	if counts[key] <= 1 {
		delete(counts, key)
		return
	}
	counts[key]--
}

// errWSWrite marks a write failure on the WebSocket connection itself (peer
// disconnected or broken pipe), so subscribeWebSocket does not try to send a
// refusal frame back onto a dead socket.
var (
	errWSWrite      = errors.New("websocket write failed")
	errTailInternal = errors.New("internal error")
)

// tailRefusalError returns err when its text is safe to expose to a client, or
// logs the raw failure server-side and returns a generic error matching
// core.WriteErr's redaction boundary.
func tailRefusalError(err error) error {
	if errors.Is(err, ErrBuildNotRunning) || core.IsPublicError(err) {
		return err
	}
	log.Printf("bex-api: log tail error: %v", err)
	return errTailInternal
}

// subscribeWebSocket streams the follow as WS text frames, one per log line.
func (s *Service) subscribeWebSocket(w http.ResponseWriter, r *http.Request, follow tailFollow) {
	conn, wsErr := wsUpgrader.Upgrade(w, r, nil)
	if wsErr != nil {
		return // upgrader already wrote the error response
	}
	defer conn.Close()
	wsCtx, cancelWS := context.WithCancel(r.Context())
	defer cancelWS()
	var peerClosed atomic.Bool
	// Read pump: keep-alive + detect client disconnect.
	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				peerClosed.Store(true)
				cancelWS()
				conn.Close()
				return
			}
		}
	}()
	followErr := follow(wsCtx, func(e LogEntry) error {
		payload, mErr := json.Marshal(toRenderLog(e))
		if mErr != nil {
			return mErr
		}
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return fmt.Errorf("%w: %w", errWSWrite, err)
		}
		return nil
	}, func() error {
		// A ping keeps an idle tail open through the proxies in front of
		// bex-api; the client's library answers it without any frame reaching
		// the caller (w1/m146).
		if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
			return fmt.Errorf("%w: %w", errWSWrite, err)
		}
		return nil
	})
	// Every post-upgrade server refusal goes out as one final Log-shaped line
	// followed by CloseTailRefused (4000). The pinned Render CLI v2.29+ retries
	// forever on 1000/1006 EOF, whereas 4000 is terminal (w8/m67, w32/102); the
	// `==>` line carries the reason for clients that decode every frame as a
	// Log (w8/020) and matches the SSE `event: error`. Normal client
	// disconnects (context cancellation, peer close, or a write to a dead
	// socket) end silently.
	if followErr != nil && !errors.Is(followErr, context.Canceled) && !errors.Is(followErr, errWSWrite) && !peerClosed.Load() && wsCtx.Err() == nil {
		publicErr := tailRefusalError(followErr)
		if payload, err := json.Marshal(toRenderLog(refusalLogEntry(publicErr))); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, payload)
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(CloseTailRefused, closeReason(publicErr)), time.Now().Add(time.Second))
	}
}

// CloseTailRefused ends a tail the server will not continue (no running build,
// logs unavailable, bad request, plan gate, access loss). Render CLI v2.29+
// reconnects after a 1000/1001/1006/1011/1012/1013 close, so a normal or bare
// close would make a refused tail retry forever; an application code is
// terminal, and the CLI exits non-zero with the reason. Older clients end on
// any close (w8/m67, user decision D2, w32/102).
const CloseTailRefused = 4000

// closeReason fits err's text in a close frame's 123-byte reason.
func closeReason(err error) string {
	reason := err.Error()
	for len(reason) > 123 {
		_, size := utf8.DecodeLastRuneInString(reason)
		reason = reason[:len(reason)-size]
	}
	return reason
}

// refusalLogEntry is the platform `==>` line announcing why a tail ended.
func refusalLogEntry(err error) LogEntry {
	labels := map[string]string{"container": progressContainer}
	if errors.Is(err, ErrBuildNotRunning) {
		labels[LabelType] = LogTypeBuild
	}
	return LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Message:   "==> " + err.Error(),
		Labels:    labels,
	}
}

// subscribeStream streams the follow as SSE (Accept: text/event-stream) or
// NDJSON. The two differ only in content type and per-line framing.
func (s *Service) subscribeStream(w http.ResponseWriter, r *http.Request, follow tailFollow) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		core.WriteErrStatus(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	// Stream is long-lived; clear the per-request write deadline so the
	// server's WriteTimeout doesn't kill it.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	useSSE := r.Header.Get("Accept") == "text/event-stream"
	errorFormat := "{\"error\":%q}\n"
	contentType := "application/x-ndjson"
	if useSSE {
		errorFormat = "event: error\ndata: %q\n\n"
		contentType = "text/event-stream"
	}
	// frame renders one log record for the negotiated transport. SSE carries an
	// `id:` — that is what makes the browser's OWN reconnect resumable, since
	// EventSource replays the last id as Last-Event-ID with no client code
	// involved, and resumeFrom below turns it back into the query's lower bound.
	// Without it every invisible reconnect re-read the pod's whole log from
	// offset 0 (w6/m93).
	//
	// A record with no parseable stamp gets NO id line, deliberately: per the SSE
	// spec an empty `id:` field SETS the last-event-id buffer to empty, and the
	// browser then stops sending the header entirely — so emitting one would let
	// a single stamp-less line silently disable resume for the rest of the tail.
	frame := func(payload []byte, id string) string {
		if !useSSE {
			return string(payload) + "\n"
		}
		if id == "" {
			return "data: " + string(payload) + "\n\n"
		}
		return "id: " + id + "\ndata: " + string(payload) + "\n\n"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// keepalive holds an idle tail — a cron job between runs — open through the
	// proxies in front of bex-api, and neither decoder sees it: an SSE comment
	// line fires no EventSource event, and a line-delimited JSON reader skips a
	// blank line (w1/m146).
	keepalive := "\n"
	if useSSE {
		keepalive = ": keepalive\n\n"
	}
	write := func(chunk string) error {
		if _, wErr := io.WriteString(w, chunk); wErr != nil {
			return wErr
		}
		flusher.Flush()
		return nil
	}
	err := follow(r.Context(), func(e LogEntry) error {
		payload, mErr := json.Marshal(toRenderLog(e))
		if mErr != nil {
			return mErr
		}
		return write(frame(payload, e.Timestamp))
	}, func() error { return write(keepalive) })
	// Headers are already sent, so a refusal can't be an HTTP status. For SSE
	// callers surface it as a terminal error event; for NDJSON callers it is
	// a best-effort final line (the client should treat a non-JSON tail as EOF).
	// Cancellation/disconnect is normal and silent; named refusals such as
	// ErrBuildNotRunning must reach the client instead of leaving an empty stream.
	if err != nil && !errors.Is(err, context.Canceled) && r.Context().Err() == nil {
		_, _ = fmt.Fprintf(w, errorFormat, tailRefusalError(err).Error())
		flusher.Flush()
	}
}

// unsupportedLogFilters are list-logs filters Render declares that select
// streams this API has none of, each with the reason it names in its refusal:
// Render Workflows task runs (an ADR018 omission), and sandboxes, whose output
// bex streams through the sandbox run stream rather than these routes.
// Honoring one could only answer an empty page, and ignoring it silently
// widened the read to every log (w8/132), so each is refused by name. A slice,
// not a map, so a request carrying several names the same one every time.
var unsupportedLogFilters = []struct{ name, reason string }{
	{"task", "bex runs no Render Workflows, so no log belongs to a task"},
	{"taskRun", "bex runs no Render Workflows, so no log belongs to a task run"},
	{"sandbox", "sandbox output is not served by the logs API; read it from the sandbox's run stream (/v1/sandboxes/{id}/runs/{executionId}/stream)"},
}

// parseLogParams maps Render's logs query string onto a ScopeRequest + a LogQuery.
// Every filter Render documents (level/instance/host/statusCode/method/path/
// build, repeatable; project/environment/region/resource) is parsed into the
// query and honored downstream; a value the vocabulary doesn't contain — an
// unknown `type` or `direction` — is a 400 naming it, and so is a filter bex
// has no streams for (unsupportedLogFilters), never a silently widened query.
// Shared by /v1/logs, /v1/logs/values and /v1/logs/subscribe so the three agree
// on what a filter means.
func parseLogParams(r *http.Request) (savedsearch.ScopeRequest, LogQuery, error) {
	v := r.URL.Query()
	if err := refuseUnsupportedLogFilters(v); err != nil {
		return savedsearch.ScopeRequest{}, LogQuery{}, err
	}
	scopeReq := savedsearch.ScopeRequest{
		Project:     v.Get("project"),
		Environment: v.Get("environment"),
		Region:      v.Get("region"),
		Resources:   v["resource"],
	}
	q, err := parseLogQuery(v)
	if err != nil {
		return savedsearch.ScopeRequest{}, LogQuery{}, err
	}
	return scopeReq, q, nil
}

func refuseUnsupportedLogFilters(v url.Values) error {
	for _, f := range unsupportedLogFilters {
		if v.Has(f.name) {
			return fmt.Errorf("%w: log filter %q is not supported: %s", core.ErrBadRequest, f.name, f.reason)
		}
	}
	return nil
}

// parseLogQuery is parseLogParams without the resources: every filter, the
// window, direction and limit. The caller refuses the unsupported filters
// first (refuseUnsupportedLogFilters).
func parseLogQuery(v url.Values) (LogQuery, error) {
	q := LogQuery{
		Search:     v["text"],
		TextMode:   v.Get("textMode"), // Bex extension (w4/m189); vetted by LogQuery.validate
		Level:      v["level"],
		Instance:   v["instance"],
		Host:       v["host"],
		StatusCode: v["statusCode"],
		Method:     v["method"],
		Path:       v["path"],
		Build:      v["build"],
	}

	types, err := NormalizeTypes(v["type"])
	if err != nil {
		return LogQuery{}, err
	}
	q.Types = types

	// `direction` and `type` values are vetted by the verb (LogQuery.validate), so
	// the three surfaces refuse identically; NormalizeTypes only maps the `app`
	// alias and the "all" widening, which is adapter-shaped work.
	q.Direction = v.Get("direction")
	q.Since, q.End, err = core.ParseTimeWindow(v.Get("startTime"), v.Get("endTime"))
	if err != nil {
		return LogQuery{}, err
	}
	if q.Limit, err = core.QueryLimit64(v, "limit"); err != nil {
		return LogQuery{}, err
	}
	return q, nil
}

// maxLogResources bounds the per-request authorization + Loki/Kubernetes fan-out
// of Render's repeatable resource filter and project-scoped log reads.
const maxLogResources = 20
