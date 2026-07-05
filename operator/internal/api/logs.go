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
	"bufio"
	"context"
	"io"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// logs.go extends the Core logs verb (core.go / MCP list_logs) with the richer
// read the REST + GraphQL adapters need: Render-logs-API filters (type / text /
// time) and a live tail. It builds on the same PodLogSource + LogEntry the MCP
// tool uses — one domain implementation, three adapters.

// Log types — Render's `type` vocabulary (app / request / build). bex only
// sources application (`app`) logs today: request logs (Traefik access logs) and
// build logs have no backend here, so those types resolve to an empty result
// rather than an error (a Render-shaped client filtering by them sees an empty
// page). `application` is accepted as an input alias for `app`.
const (
	LogTypeApplication = "application"
	LogTypeRequest     = "request"
	LogTypeBuild       = "build"
)

// Render defaults the logs `limit` to 20 and caps it at 100; bex matches so a
// Render client sees identical paging.
const (
	defaultLogLimit = 20
	maxLogLimit     = 100
)

// PodLogStream streams a pod container's logs live (Follow:true) until ctx is
// cancelled. It is PodLogSource's tail-follow sibling — kept separate so Core
// stays clientset-free; production wires it via NewPodLogStream, tests fake it.
// nil => FollowLogs reports ErrLogsUnavailable.
type PodLogStream func(ctx context.Context, namespace, pod, container string) (io.ReadCloser, error)

// LogQuery is the resolved filter set for QueryLogs / FollowLogs.
type LogQuery struct {
	App    string
	Type   string    // "" == all; application | request | build
	Search string    // case-insensitive substring on the message
	Since  time.Time // zero == no lower bound
	End    time.Time // zero == no upper bound
	Limit  int64     // max lines (most recent kept)
}

func (q LogQuery) normalized() LogQuery {
	if q.Limit <= 0 {
		q.Limit = defaultLogLimit
	}
	if q.Limit > maxLogLimit {
		q.Limit = maxLogLimit
	}
	return q
}

// keep reports whether an entry satisfies the search/time filters (type scoping
// is handled at the source level in QueryLogs).
func (q LogQuery) keep(e LogEntry) bool {
	if q.Search != "" && !strings.Contains(strings.ToLower(e.Message), strings.ToLower(q.Search)) {
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

// QueryLogs returns an App's log lines filtered by type/text/time, newest-last,
// capped at Limit. It reuses the same pod-log collection as the MCP Logs verb;
// request/build types resolve to an empty slice (no backend). Fails with
// ErrNotFound for an unknown App and ErrLogsUnavailable when no source is wired.
func (c *Core) QueryLogs(ctx context.Context, q LogQuery) ([]LogEntry, error) {
	if c.PodLogs == nil {
		return nil, ErrLogsUnavailable
	}
	if _, err := c.fetch(ctx, q.App); err != nil {
		return nil, err
	}
	q = q.normalized()
	if !q.appliesToApplication() {
		return []LogEntry{}, nil // request/build have no source in bex
	}
	entries, err := c.collectPodLogs(ctx, q.App, q.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]LogEntry, 0, len(entries))
	for _, e := range entries {
		if q.keep(e) {
			out = append(out, e)
		}
	}
	if int64(len(out)) > q.Limit {
		out = out[int64(len(out))-q.Limit:] // keep the newest Limit
	}
	return out, nil
}

// FollowLogs streams an App's new log lines to emit until ctx is cancelled or
// emit errors. The same type/text filters as QueryLogs apply. Requires a
// PodLogStream (nil => ErrLogsUnavailable).
func (c *Core) FollowLogs(ctx context.Context, q LogQuery, emit func(LogEntry) error) error {
	if c.PodLogsFollow == nil {
		return ErrLogsUnavailable
	}
	if _, err := c.fetch(ctx, q.App); err != nil {
		return err
	}
	q = q.normalized()
	if !q.appliesToApplication() {
		<-ctx.Done() // nothing to stream; hold until the client disconnects
		return ctx.Err()
	}

	var pods corev1.PodList
	if err := c.Client.List(ctx, &pods,
		client.InNamespace(c.Namespace),
		client.MatchingLabels{podLabelApp: q.App}); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan LogEntry, 64)
	var wg sync.WaitGroup
	for i := range pods.Items {
		wg.Add(1)
		go func(pod string) {
			defer wg.Done()
			c.streamPodLogs(ctx, q.App, pod, ch)
		}(pods.Items[i].Name)
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

// streamPodLogs follows one pod's log into ch until ctx ends or the stream
// closes. A replica going away ends its stream without failing the subscription.
func (c *Core) streamPodLogs(ctx context.Context, service, pod string, ch chan<- LogEntry) {
	rc, err := c.PodLogsFollow(ctx, c.Namespace, pod, appContainer)
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
