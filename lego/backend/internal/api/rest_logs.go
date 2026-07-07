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
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// rest_logs.go is the REST logs adapter — Render logs-API compatible. It maps
// the query string (resource/type/text/startTime/endTime/limit) and the
// {hasMore, next*Time, logs} envelope onto Core.QueryLogs, and serves a live
// tail over SSE. Like the rest of the REST adapter it holds no logic beyond
// routing + Render (de)serialization; the read path lives in Core.

func (s *Server) registerLogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/logs", s.logsQuery)
	mux.HandleFunc("GET /v1/logs/subscribe", s.logsSubscribe)
}

// logsQuery serves GET /v1/logs — a historical query across an App's replicas.
func (s *Server) logsQuery(w http.ResponseWriter, r *http.Request) {
	resources, q, err := parseLogParams(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Render's `resource` is an array; merge each App's lines, then sort + cap.
	var all []LogEntry
	for _, res := range resources {
		q.App = res
		entries, err := s.Core.QueryLogs(r.Context(), q)
		if err != nil {
			writeErr(w, err)
			return
		}
		all = append(all, entries...)
	}
	if len(resources) > 1 {
		sort.SliceStable(all, func(i, j int) bool { return all[i].Timestamp < all[j].Timestamp })
		if int64(len(all)) > q.Limit {
			all = all[int64(len(all))-q.Limit:]
		}
	}
	writeJSON(w, http.StatusOK, toRenderLogList(all, q.Limit))
}

// logsSubscribe serves GET /v1/logs/subscribe — a live tail over Server-Sent
// Events (one `data: <renderLog JSON>` frame per line, following one resource).
// bex uses SSE where Render upgrades to a WebSocket: no extra dependency,
// curl-friendly, same "stream new lines live" contract.
func (s *Server) logsSubscribe(w http.ResponseWriter, r *http.Request) {
	resources, q, err := parseLogParams(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	q.App = resources[0] // subscribe follows a single App

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	// SSE is long-lived; clear the per-request write deadline so the server's
	// WriteTimeout doesn't kill the stream.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	err = s.Core.FollowLogs(r.Context(), q, func(e LogEntry) error {
		payload, mErr := json.Marshal(toRenderLog(e))
		if mErr != nil {
			return mErr
		}
		if _, wErr := fmt.Fprintf(w, "data: %s\n\n", payload); wErr != nil {
			return wErr
		}
		flusher.Flush()
		return nil
	})
	if err == ErrLogsUnavailable {
		// Headers are already sent; surface the reason as a terminal SSE event.
		_, _ = fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
		flusher.Flush()
	}
}

// parseLogParams maps Render's logs query string onto resources + a LogQuery.
// resource is required (repeatable, the App id); type is repeatable
// (app/application/request/build); text is the search; startTime/endTime are
// RFC3339; limit follows Render's 1..100 range (Core clamps).
func parseLogParams(r *http.Request) ([]string, LogQuery, error) {
	v := r.URL.Query()

	resources := v["resource"]
	if len(resources) == 0 {
		return nil, LogQuery{}, fmt.Errorf("resource is required")
	}

	q := LogQuery{Search: v.Get("text")}

	// `type` is repeatable (Render). Narrow only when a single concrete type is
	// asked for (the dashboard's "Application logs"/"Request logs" case); several
	// — or none — returns all types.
	seen := map[string]struct{}{}
	for _, t := range v["type"] {
		switch t {
		case "", "all":
		case renderLogTypeApp, LogTypeApplication:
			seen[LogTypeApplication] = struct{}{}
		case LogTypeRequest:
			seen[LogTypeRequest] = struct{}{}
		case LogTypeBuild:
			seen[LogTypeBuild] = struct{}{}
		default:
			return nil, LogQuery{}, fmt.Errorf("unknown type %q (want app|request|build)", t)
		}
	}
	if len(seen) == 1 {
		for t := range seen {
			q.Type = t
		}
	}

	if s := v.Get("startTime"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, LogQuery{}, fmt.Errorf("startTime: %w", err)
		}
		q.Since = t
	}
	if e := v.Get("endTime"); e != "" {
		t, err := time.Parse(time.RFC3339, e)
		if err != nil {
			return nil, LogQuery{}, fmt.Errorf("endTime: %w", err)
		}
		q.End = t
	}
	if l := v.Get("limit"); l != "" {
		n, err := strconv.ParseInt(l, 10, 64)
		if err != nil {
			return nil, LogQuery{}, fmt.Errorf("limit: %w", err)
		}
		q.Limit = n
	}

	return resources, q, nil
}
