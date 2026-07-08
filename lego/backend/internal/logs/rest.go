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
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the REST logs adapter — Render logs-API compatible. It maps the
// query string (resource/type/text/startTime/endTime/limit) and the {hasMore,
// next*Time, logs} envelope onto QueryLogs, and serves a live tail over SSE.

// RegisterREST mounts the logs query + live-tail routes.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/logs", s.logsQuery)
	mux.HandleFunc("GET /v1/logs/subscribe", s.logsSubscribe)
}

// logsQuery serves GET /v1/logs — a historical query across an App's replicas.
func (s *Service) logsQuery(w http.ResponseWriter, r *http.Request) {
	resources, q, err := parseLogParams(r)
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Render's `resource` is an array; merge each App's lines, then sort + cap.
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
	if len(resources) > 1 {
		sort.SliceStable(all, func(i, j int) bool { return all[i].Timestamp < all[j].Timestamp })
		if int64(len(all)) > q.Limit {
			all = all[int64(len(all))-q.Limit:]
		}
	}
	core.WriteJSON(w, http.StatusOK, toRenderLogList(all, q.Limit))
}

// logsSubscribe serves GET /v1/logs/subscribe — a live tail over Server-Sent
// Events (one `data: <renderLog JSON>` frame per line, following one resource).
func (s *Service) logsSubscribe(w http.ResponseWriter, r *http.Request) {
	resources, q, err := parseLogParams(r)
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	q.App = resources[0] // subscribe follows a single App

	flusher, ok := w.(http.Flusher)
	if !ok {
		core.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
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

	err = s.FollowLogs(r.Context(), q, func(e LogEntry) error {
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
	if err == core.ErrLogsUnavailable {
		// Headers are already sent; surface the reason as a terminal SSE event.
		_, _ = fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
		flusher.Flush()
	}
}

// narrowLogType maps Render's repeatable `type` filter onto Core's single Type.
// bex narrows only when exactly one concrete type is requested; empty / "all" /
// several => "" (all types). ok is false when a value isn't a recognized type —
// the REST adapter turns that into a 400, while the MCP tool tolerates it (Render
// clients pass filters through). `application` is the accepted alias for `app`
// (renderLogTypeApp). Shared by the REST and MCP fragments so the two agree.
func narrowLogType(types []string) (typ string, ok bool) {
	ok = true
	seen := map[string]struct{}{}
	for _, t := range types {
		switch t {
		case "", "all":
		case renderLogTypeApp, LogTypeApplication:
			seen[LogTypeApplication] = struct{}{}
		case LogTypeRequest:
			seen[LogTypeRequest] = struct{}{}
		case LogTypeBuild:
			seen[LogTypeBuild] = struct{}{}
		default:
			ok = false
		}
	}
	if len(seen) == 1 {
		for t := range seen {
			return t, ok
		}
	}
	return "", ok
}

// parseLogParams maps Render's logs query string onto resources + a LogQuery.
func parseLogParams(r *http.Request) ([]string, LogQuery, error) {
	v := r.URL.Query()

	resources := v["resource"]
	if len(resources) == 0 {
		return nil, LogQuery{}, fmt.Errorf("resource is required")
	}

	q := LogQuery{Search: v.Get("text")}

	// `type` is repeatable (Render). Narrow only when a single concrete type is
	// asked for; several — or none — returns all types.
	typ, ok := narrowLogType(v["type"])
	if !ok {
		return nil, LogQuery{}, fmt.Errorf("unknown type (want app|request|build)")
	}
	q.Type = typ

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
