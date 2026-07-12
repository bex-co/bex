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
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Log types — Render's `type` vocabulary (app / request / build). bex only
// sources application (`app`) logs today: request/build logs have no backend
// here, so those types resolve to an empty result. `application` is accepted as
// an input alias for `app`.
const (
	LogTypeApplication = "application"
	LogTypeRequest     = "request"
	LogTypeBuild       = "build"
)

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
	// MaxQueryHours, when positive, caps the startTime–endTime window accepted by
	// REST log queries. 0 = unlimited.
	MaxQueryHours int
	// MaxSSEConns, when positive, caps concurrent GET /v1/logs/subscribe SSE
	// connections. Excess connections receive 429. 0 = unlimited.
	MaxSSEConns int64

	sseConns atomic.Int64
}

// LogQuery is the resolved filter set for QueryLogs / FollowLogs.
type LogQuery struct {
	App    string
	Type   string    // "" == all; application | request | build
	Search string    // case-insensitive substring on the message
	Since  time.Time // zero == no lower bound
	End    time.Time // zero == no upper bound
	Limit  int64     // max lines (most recent kept)

	searchLower string // Search lowercased once by normalized(); read by keep()
}

// normalized clamps Limit to Render's paging range and precomputes searchLower.
func (q LogQuery) normalized() LogQuery {
	if q.Limit <= 0 {
		q.Limit = defaultLogLimit
	}
	if q.Limit > maxLogLimit {
		q.Limit = maxLogLimit
	}
	q.searchLower = strings.ToLower(q.Search)
	return q
}

// hasFilters reports whether any line-level filter (search/time) is set.
func (q LogQuery) hasFilters() bool {
	return q.Search != "" || !q.Since.IsZero() || !q.End.IsZero()
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

// appliesToApplication reports whether the query could match application logs —
// bex's only log source. A request/build-only query never can.
func (q LogQuery) appliesToApplication() bool {
	return q.Type == "" || q.Type == LogTypeApplication
}

// Logs returns recent log lines for an App, aggregated across its pods and
// sorted by timestamp. tail caps lines per instance (<=0 => defaultLogTail). It
// fails with core.ErrNotFound for an unknown App and core.ErrLogsUnavailable
// when no source is wired. It's the unfiltered convenience read; the REST and
// MCP adapters go through QueryLogs (Render's type/text/time filters).
func (s *Service) Logs(ctx context.Context, name string, tail int64) ([]LogEntry, error) {
	if err := s.Authorize(ctx, core.RelCanViewLogs); err != nil {
		return nil, err
	}
	if s.History == nil && s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	if _, err := s.GetApp(ctx, name); err != nil {
		return nil, err // ErrNotFound for unknown apps, exactly like Get
	}
	if tail <= 0 {
		tail = defaultLogTail
	}
	if s.History != nil {
		// Durable history: the unfiltered tail-N read is a limit-only query.
		return s.History(ctx, s.Namespace, LogQuery{App: name, Limit: tail})
	}
	return s.collectPodLogs(ctx, name, tail)
}

// QueryLogs returns an App's log lines filtered by type/text/time, newest-last,
// capped at Limit. It reuses the same pod-log collection as Logs; request/build
// types resolve to an empty slice (no backend).
func (s *Service) QueryLogs(ctx context.Context, q LogQuery) ([]LogEntry, error) {
	if err := s.Authorize(ctx, core.RelCanViewLogs); err != nil {
		return nil, err
	}
	if s.History == nil && s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	if _, err := s.GetApp(ctx, q.App); err != nil {
		return nil, err
	}
	q = q.normalized()
	if !q.appliesToApplication() {
		return []LogEntry{}, nil // request/build have no source in bex
	}
	if s.History != nil {
		// Durable history (Loki) applies the type/text/time/limit filters in the
		// store; it already returns oldest-first, capped at q.Limit.
		return s.History(ctx, s.Namespace, q)
	}
	entries, err := s.collectPodLogs(ctx, q.App, q.Limit)
	if err != nil {
		return nil, err
	}
	// Filter in place; skip the pass entirely on the common no-filter query.
	if q.hasFilters() {
		kept := entries[:0]
		for _, e := range entries {
			if q.keep(e) {
				kept = append(kept, e)
			}
		}
		entries = kept
	}
	if int64(len(entries)) > q.Limit {
		entries = entries[int64(len(entries))-q.Limit:] // keep the newest Limit
	}
	return entries, nil
}

// FollowLogs streams an App's new log lines to emit until ctx is cancelled or
// emit errors. The same type/text filters as QueryLogs apply. Requires a
// PodLogStream (nil => core.ErrLogsUnavailable).
func (s *Service) FollowLogs(ctx context.Context, q LogQuery, emit func(LogEntry) error) error {
	if err := s.Authorize(ctx, core.RelCanViewLogs); err != nil {
		return err
	}
	if s.PodLogsFollow == nil {
		return core.ErrLogsUnavailable
	}
	if _, err := s.GetApp(ctx, q.App); err != nil {
		return err
	}
	q = q.normalized()
	if !q.appliesToApplication() {
		<-ctx.Done() // nothing to stream; hold until the client disconnects
		return ctx.Err()
	}

	pods, err := s.AppPods(ctx, q.App)
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
		}(pods[i].Name)
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
			if err := emit(e); err != nil {
				return err
			}
		}
	}
}

// collectPodLogs reads up to tail lines from every replica of an App, tagged and
// timestamp-sorted. Shared by Logs (MCP) and QueryLogs (REST/GraphQL).
func (s *Service) collectPodLogs(ctx context.Context, name string, tail int64) ([]LogEntry, error) {
	pods, err := s.AppPods(ctx, name)
	if err != nil {
		return nil, err
	}
	var out []LogEntry
	for i := range pods {
		entries, err := s.readPodLogs(ctx, name, pods[i].Name, tail)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out, nil
}

func (s *Service) readPodLogs(ctx context.Context, service, pod string, tail int64) ([]LogEntry, error) {
	rc, err := s.PodLogs(ctx, s.Namespace, pod, core.AppContainer, tail)
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
		},
	}
}
