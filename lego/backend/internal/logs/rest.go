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
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

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
// log read goes through it — the historical query AND label discovery — and
// so does GraphQL's `logs`/`logLabelValues` (graphql.go, w9/m1/t002) — both
// accept the same startTime/endTime and would otherwise let a caller scan the
// store unbounded.
func (s *Service) checkWindow(q LogQuery) error {
	if s.MaxQueryHours <= 0 || q.Since.IsZero() {
		return nil
	}
	end := q.End
	if end.IsZero() {
		end = time.Now()
	}
	if end.Sub(q.Since) > time.Duration(s.MaxQueryHours)*time.Hour {
		return fmt.Errorf("%w: query range exceeds %d hours", core.ErrBadRequest, s.MaxQueryHours)
	}
	return nil
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

// logsQuery serves GET /v1/logs — a historical query across an App's replicas.
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

	// Render's `resource` is an array; merge each App's lines, then sort + cap.
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
	core.WriteJSON(w, http.StatusOK, toRenderLogList(all, merged.Limit))
}

// logsSubscribe serves GET /v1/logs/subscribe — a live tail over Server-Sent
// Events (one `data: <renderLog JSON>` frame per line, following one resource).
func (s *Service) logsSubscribe(w http.ResponseWriter, r *http.Request) {
	if s.MaxSSEConns > 0 {
		if s.sseConns.Add(1) > s.MaxSSEConns {
			s.sseConns.Add(-1)
			core.WriteJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "too many active log subscriptions",
			})
			return
		}
		defer s.sseConns.Add(-1)
	}
	resources, q, err := parseLogParams(r)
	if err != nil {
		core.WriteErr(w, err)
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
	// Headers are already sent, so a refusal can't be an HTTP status: surface the
	// reason as a terminal SSE event. ErrLogStoreUnavailable lands here when the
	// tail is asked for a store-only filter (level/statusCode/…) — the tail reads
	// pod logs by design, so it says so rather than streaming unfiltered lines.
	if errors.Is(err, core.ErrLogsUnavailable) || errors.Is(err, core.ErrLogStoreUnavailable) {
		_, _ = fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
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

	resources := v["resource"]
	if len(resources) == 0 {
		return nil, LogQuery{}, fmt.Errorf("%w: resource is required", core.ErrBadRequest)
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
	q.Since, q.End, err = parseTimeWindow(v.Get("startTime"), v.Get("endTime"))
	if err != nil {
		return nil, LogQuery{}, err
	}
	if l := v.Get("limit"); l != "" {
		n, err := strconv.ParseInt(l, 10, 64)
		if err != nil {
			return nil, LogQuery{}, fmt.Errorf("%w: limit: %s", core.ErrBadRequest, err)
		}
		q.Limit = n
	}

	return resources, q, nil
}
