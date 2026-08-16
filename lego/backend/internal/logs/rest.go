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
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// wsUpgrader upgrades the subscribe connection to WebSocket when the Render
// CLI (v2+) requests it. Origin checking is skipped — auth is already enforced
// at the HTTP auth-gate that wraps this handler.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

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

	// SECURITY (codex #3): reject a subscription with no live producer BEFORE
	// acquiring a slot — a type=predeploy (or other producerless) request would
	// otherwise park on an idle stream and hold one of the process-global SSE
	// slots until it chose to disconnect, letting one tenant exhaust the pool.
	if err := q.liveSubscribable(); err != nil {
		core.WriteErr(w, err)
		return
	}

	// The concurrency cap is acquired only after the request parses and is
	// confirmed streamable, so malformed or producerless requests never take a slot.
	if s.MaxSSEConns > 0 {
		if s.sseConns.Add(1) > s.MaxSSEConns {
			s.sseConns.Add(-1)
			// Render's one error dialect ({error,message,id}) on every branch, so a
			// Render client reads .message here too, not a bare {error} (w9/m39).
			core.WriteErrStatus(w, http.StatusTooManyRequests, "too many active log subscriptions")
			return
		}
		defer s.sseConns.Add(-1)
	}

	// Render CLI v2+ sends an upgrade request; everyone else gets SSE or NDJSON.
	if websocket.IsWebSocketUpgrade(r) {
		s.subscribeWebSocket(w, r, q)
		return
	}
	s.subscribeStream(w, r, q)
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
	lineFormat, errorFormat := "%s\n", "{\"error\":%q}\n"
	contentType := "application/x-ndjson"
	if useSSE {
		lineFormat, errorFormat = "data: %s\n\n", "event: error\ndata: %q\n\n"
		contentType = "text/event-stream"
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
		if _, wErr := fmt.Fprintf(w, lineFormat, payload); wErr != nil {
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
