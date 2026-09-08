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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bex-co/bex/lego/backend/internal/core"
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

// checkWindow enforces BEX_MAX_QUERY_HOURS on a query's time range. Every REST
// log read goes through it — the historical query AND label discovery — and so
// do GraphQL's `logs`/`logLabelValues` (graphql.go, w9/m1/t002) and MCP's
// `list_logs`/`list_log_label_values` (mcp.go, w9/004) — all accept the same
// startTime/endTime and would otherwise let a caller scan the store unbounded.
func (s *Service) checkWindow(q LogQuery) error {
	return core.CheckQueryWindow(s.MaxQueryHours, time.Now, q.Since, q.End)
}

// logsValues serves GET /v1/logs/values — Render's filter-value discovery: the
// values one label takes among the logs the same filter set selects. Render
// returns a bare JSON string array, and so does bex.
func (s *Service) logsValues(w http.ResponseWriter, r *http.Request) {
	resources, q, err := parseLogParams(r)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	if err := s.checkWindow(q); err != nil {
		core.WriteErr(w, err)
		return
	}
	label := r.URL.Query().Get("label")
	if label == "" {
		core.WriteErr(w, fmt.Errorf("%w: label is required", core.ErrBadRequest))
		return
	}

	all := []string{}
	for _, res := range resources {
		q.App = res
		values, err := s.LogLabelValues(r.Context(), label, q)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		all = append(all, values...)
	}
	slices.Sort(all)
	core.WriteJSON(w, http.StatusOK, slices.Compact(all))
}

// logsQuery serves GET /v1/logs — a historical query across App or managed
// Postgres instances, matching Render's generic resource-array contract.
func (s *Service) logsQuery(w http.ResponseWriter, r *http.Request) {
	resources, q, err := parseLogParams(r)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	if err := s.checkWindow(q); err != nil {
		core.WriteErr(w, err)
		return
	}

	// Render's `resource` is an array; merge each resource's lines, then sort + cap.
	// QueryLogs gets the query as parsed — normalizing here first would coerce an
	// invalid `direction` to the default before the verb could refuse it.
	var all []LogEntry
	for _, res := range resources {
		q.App = res
		entries, err := s.QueryLogs(r.Context(), q)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		all = append(all, entries...)
	}
	merged := q.normalized() // the limit/direction the merge across resources applies
	if len(resources) > 1 {
		sort.SliceStable(all, func(i, j int) bool { return all[i].Timestamp < all[j].Timestamp })
		all = merged.capToLimit(all) // the limit is a total across resources, not per-App
	}
	core.WriteJSON(w, http.StatusOK, toRenderLogList(all, merged.Limit, merged.Since, merged.End))
}

// logsSubscribe serves GET /v1/logs/subscribe — a live tail, following one
// resource. Three wire formats are supported:
//
//   - WebSocket upgrade → WS text frames, one per log line; the Render CLI
//     v2+ sends Upgrade: websocket and expects this protocol.
//   - Accept: text/event-stream → SSE (`data: <JSON>\n\n` per line); for
//     browser EventSource and curl -N.
//   - Anything else → NDJSON (`<JSON>\n` per line).
func (s *Service) logsSubscribe(w http.ResponseWriter, r *http.Request) {
	resources, q, err := parseLogParams(r)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	q.App = resources[0] // subscribe follows a single App

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
	// is the only bound, exactly as before; with a startTime and no resume (the
	// first connect) the window is the bound.
	if resume, ok := resumeFrom(r.Header.Get("Last-Event-ID")); ok && resume.After(q.Since) {
		q.Since = resume
	}
	// Refuse invalid bounds before writing streaming headers or upgrading to
	// WebSocket; FollowLogs' validation would otherwise run after HTTP 200/101.
	if err := q.validate(); err != nil {
		core.WriteErr(w, err)
		return
	}

	// SECURITY (codex #3): reject a subscription with no live producer BEFORE
	// acquiring a slot — a type=predeploy (or other producerless) request would
	// otherwise park on an idle stream and hold one of the process-global SSE
	// slots until it chose to disconnect, letting one tenant exhaust the pool.
	if err := q.liveSubscribable(); err != nil {
		core.WriteErr(w, err)
		return
	}

	// Admission retains a final process ceiling and adds caller/workspace shares
	// after resource authorization, leaving capacity for unrelated tenants.
	release, err := s.acquireSubscription(r.Context(), q)
	if err != nil {
		if errors.Is(err, errSubscriptionLimit) {
			// Render's one error dialect ({error,message,id}) on every branch, so a
			// Render client reads .message here too, not a bare {error} (w9/m39).
			core.WriteErrStatus(w, http.StatusTooManyRequests, "too many active log subscriptions")
		} else {
			core.WriteErr(w, err)
		}
		return
	}
	defer release()

	// Render CLI v2+ sends an upgrade request; everyone else gets SSE or NDJSON.
	if websocket.IsWebSocketUpgrade(r) {
		s.subscribeWebSocket(w, r, q)
		return
	}
	s.subscribeStream(w, r, q)
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

var errSubscriptionLimit = errors.New("log subscription capacity reached")

func (s *Service) acquireSubscription(ctx context.Context, q LogQuery) (func(), error) {
	global := false
	if s.MaxSSEConns > 0 {
		if s.sseConns.Add(1) > s.MaxSSEConns {
			s.sseConns.Add(-1)
			return nil, errSubscriptionLimit
		}
		global = true
	}
	releaseGlobal := func() {
		if global {
			s.sseConns.Add(-1)
		}
	}

	workspace, err := s.subscriptionWorkspace(ctx, q.App)
	if err != nil {
		releaseGlobal()
		return nil, err
	}
	identity, _ := core.IdentityFrom(ctx)
	subject := identity.Subject

	s.sseMu.Lock()
	if s.sseBySubject == nil {
		s.sseBySubject = make(map[string]int)
		s.sseByWorkspace = make(map[string]int)
	}
	if s.MaxSSEConnsPerSubject > 0 && subject != "" && s.sseBySubject[subject] >= s.MaxSSEConnsPerSubject {
		s.sseMu.Unlock()
		releaseGlobal()
		return nil, errSubscriptionLimit
	}
	if s.MaxSSEConnsPerWorkspace > 0 && s.sseByWorkspace[workspace] >= s.MaxSSEConnsPerWorkspace {
		s.sseMu.Unlock()
		releaseGlobal()
		return nil, errSubscriptionLimit
	}
	if s.MaxSSEConnsPerSubject > 0 && subject != "" {
		s.sseBySubject[subject]++
	}
	if s.MaxSSEConnsPerWorkspace > 0 {
		s.sseByWorkspace[workspace]++
	}
	s.sseMu.Unlock()

	var once sync.Once
	return func() {
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
	var labels map[string]string
	var namespace string
	switch {
	case isPostgresResource(resource):
		database, err := s.AuthorizeDatabase(ctx, core.RelCanViewLogs, resource)
		if err != nil {
			return "", err
		}
		labels, namespace = database.Labels, database.Namespace
	case isKeyValueResource(resource):
		kv, err := s.AuthorizeKeyValue(ctx, core.RelCanViewLogs, resource)
		if err != nil {
			return "", err
		}
		labels, namespace = kv.Labels, kv.Namespace
	default:
		app, err := s.AuthorizeApp(ctx, core.RelCanViewLogs, resource)
		if err != nil {
			return "", err
		}
		labels, namespace = app.Labels, app.Namespace
	}
	if workspace := labels[core.LabelTenant]; workspace != "" {
		return workspace, nil
	}
	if namespace != "" {
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

// subscribeWebSocket streams the follow as WS text frames, one per log line.
func (s *Service) subscribeWebSocket(w http.ResponseWriter, r *http.Request, q LogQuery) {
	conn, wsErr := wsUpgrader.Upgrade(w, r, nil)
	if wsErr != nil {
		return // upgrader already wrote the error response
	}
	defer conn.Close()
	// Read pump: keep-alive + detect client disconnect.
	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				conn.Close()
				return
			}
		}
	}()
	followErr := s.FollowLogs(r.Context(), q, func(e LogEntry) error {
		payload, mErr := json.Marshal(toRenderLog(e))
		if mErr != nil {
			return mErr
		}
		return conn.WriteMessage(websocket.TextMessage, payload)
	})
	if errors.Is(followErr, core.ErrLogsUnavailable) || errors.Is(followErr, core.ErrLogStoreUnavailable) {
		msg, _ := json.Marshal(map[string]string{"error": followErr.Error()})
		_ = conn.WriteMessage(websocket.TextMessage, msg)
	}
}

// subscribeStream streams the follow as SSE (Accept: text/event-stream) or
// NDJSON. The two differ only in content type and per-line framing.
func (s *Service) subscribeStream(w http.ResponseWriter, r *http.Request, q LogQuery) {
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

	err := s.FollowLogs(r.Context(), q, func(e LogEntry) error {
		payload, mErr := json.Marshal(toRenderLog(e))
		if mErr != nil {
			return mErr
		}
		if _, wErr := io.WriteString(w, frame(payload, e.Timestamp)); wErr != nil {
			return wErr
		}
		flusher.Flush()
		return nil
	})
	// Headers are already sent, so a refusal can't be an HTTP status. For SSE
	// callers surface it as a terminal error event; for NDJSON callers it is
	// a best-effort final line (the client should treat a non-JSON tail as EOF).
	// Cancellation/disconnect is normal and silent; named refusals such as
	// ErrBuildNotRunning must reach the client instead of leaving an empty stream.
	if err != nil && !errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintf(w, errorFormat, err.Error())
		flusher.Flush()
	}
}

// parseLogParams maps Render's logs query string onto resources + a LogQuery.
// Every filter Render documents (level/instance/host/statusCode/method/path,
// repeatable) is parsed into the query and honored downstream; a value the
// vocabulary doesn't contain — an unknown `type` or `direction` — is a 400 naming
// it, never a silently widened query. Shared by /v1/logs, /v1/logs/values and
// /v1/logs/subscribe so the three agree on what a filter means.
func parseLogParams(r *http.Request) ([]string, LogQuery, error) {
	v := r.URL.Query()

	resources, err := boundedResources(v["resource"])
	if err != nil {
		return nil, LogQuery{}, err
	}

	q := LogQuery{
		Search:     v.Get("text"),
		Level:      v["level"],
		Instance:   v["instance"],
		Host:       v["host"],
		StatusCode: v["statusCode"],
		Method:     v["method"],
		Path:       v["path"],
	}

	types, err := NormalizeTypes(v["type"])
	if err != nil {
		return nil, LogQuery{}, err
	}
	q.Types = types

	// `direction` and `type` values are vetted by the verb (LogQuery.validate), so
	// the three surfaces refuse identically; NormalizeTypes only maps the `app`
	// alias and the "all" widening, which is adapter-shaped work.
	q.Direction = v.Get("direction")
	q.Since, q.End, err = core.ParseTimeWindow(v.Get("startTime"), v.Get("endTime"))
	if err != nil {
		return nil, LogQuery{}, err
	}
	if q.Limit, err = core.QueryLimit64(v, "limit"); err != nil {
		return nil, LogQuery{}, err
	}

	return resources, q, nil
}

// maxLogResources bounds the per-request authorization + Loki/Kubernetes fan-out
// of Render's repeatable resource filter. Twenty is deliberately generous for a
// human dashboard/CLI query while keeping one request's upstream work finite.
const maxLogResources = 20

// boundedResources rejects oversized raw arrays before doing any upstream work,
// then de-duplicates in first-seen order. Checking the raw length first matters:
// repeating one valid resource thousands of times must be refused, not accepted
// as a cheap-looking one-element set after compaction.
func boundedResources(resources []string) ([]string, error) {
	if len(resources) == 0 {
		return nil, fmt.Errorf("%w: resource is required", core.ErrBadRequest)
	}
	if len(resources) > maxLogResources {
		return nil, fmt.Errorf("%w: resource accepts at most %d entries", core.ErrBadRequest, maxLogResources)
	}
	seen := make(map[string]struct{}, len(resources))
	out := make([]string, 0, len(resources))
	for _, resource := range resources {
		if _, ok := seen[resource]; ok {
			continue
		}
		seen[resource] = struct{}{}
		out = append(out, resource)
	}
	return out, nil
}
