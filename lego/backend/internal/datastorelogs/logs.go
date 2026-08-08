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

// Package datastorelogs holds the direct-pod log read path shared by the
// managed datastore features. Postgres (CNPG) and Key Value (Valkey) both
// expose a Render-compatible "logs for one managed instance" verb over the same
// mechanism — read each pod's container stream, filter, merge, cap — and differ
// only in which container carries the process log and which type label the
// entries are stamped with.
//
// This is the fallback path. In production both features delegate to the
// generic durable logs core (Loki); this is what serves isolated tests and any
// deployment without a durable store.
package datastorelogs

import (
	"bufio"
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Resource type labels stamped onto every entry, shared with the generic logs
// feature so both paths report a managed instance the same way.
const (
	KindPostgres = "postgres"
	KindKeyValue = "keyvalue"
)

const (
	// DefaultLimit applies when a query does not ask for a size.
	DefaultLimit = 20
	// MaxLimit caps a caller's request: this path reads whole pod streams, so
	// the ceiling bounds both the API response and the read itself.
	MaxLimit = 100
)

// Query is the filter set for a managed datastore log read. It mirrors the
// managed subset of the generic logs query; HTTP request-log filters do not
// apply to a database process stream.
type Query struct {
	Search    string
	Since     time.Time
	End       time.Time
	Limit     int64
	Direction string
	Instance  []string // restrict to these pod names (empty = all pods)
}

// Entry is one datastore log line in Render's log shape.
type Entry struct {
	Timestamp string            `json:"timestamp,omitempty"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// Source is the narrow cross-feature seam that lets the dedicated datastore
// compatibility adapters reuse the generic logs core without importing the logs
// package.
type Source func(context.Context, string, Query) ([]Entry, error)

// Instance identifies one managed datastore's pods and how to read them.
type Instance struct {
	Namespace string
	Name      string // the CR name, stamped as the "service" label
	Kind      string // KindPostgres or KindKeyValue, stamped as the "type" label
	Container string // the container within each pod carrying the process log
	Pods      []string
	PodLogs   core.PodLogSource
}

// Collect reads the instance's pod logs and returns them oldest-first, capped
// at the query's limit. A pod whose stream cannot be read is skipped rather
// than failing the whole read: a pod that has restarted or been reaped just
// goes missing.
func Collect(ctx context.Context, in Instance, q Query) ([]Entry, error) {
	if q.Limit <= 0 {
		q.Limit = DefaultLimit
	}
	if q.Limit > MaxLimit {
		q.Limit = MaxLimit
	}

	searchLower := strings.ToLower(q.Search)
	var out []Entry
	for _, pod := range in.Pods {
		if len(q.Instance) > 0 && !slices.Contains(q.Instance, pod) {
			continue
		}
		entries, err := in.readPod(ctx, pod, q.Limit)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if searchLower != "" && !strings.Contains(strings.ToLower(e.Message), searchLower) {
				continue
			}
			if !q.within(e.Timestamp) {
				continue
			}
			out = append(out, e)
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	if lim := q.Limit; int64(len(out)) > lim {
		if q.Direction == core.DirectionForward {
			out = out[:lim]
		} else {
			out = out[int64(len(out))-lim:]
		}
	}
	return out, nil
}

// within reports whether an entry's timestamp falls inside the query window.
// An unparseable timestamp is kept: dropping it would silently hide a line the
// datastore emitted without a leading RFC3339 stamp.
func (q Query) within(timestamp string) bool {
	if q.Since.IsZero() && q.End.IsZero() {
		return true
	}
	t, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return true
	}
	if !q.Since.IsZero() && t.Before(q.Since) {
		return false
	}
	return q.End.IsZero() || !t.After(q.End)
}

func (in Instance) readPod(ctx context.Context, pod string, tail int64) ([]Entry, error) {
	rc, err := in.PodLogs(ctx, in.Namespace, pod, in.Container, tail)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	var entries []Entry
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		entries = append(entries, in.parseLine(pod, sc.Text()))
	}
	return entries, sc.Err()
}

// parseLine splits kubelet's "<RFC3339Nano> <message>" prefix off a raw line.
// A line with no parseable leading stamp keeps its full text as the message.
func (in Instance) parseLine(pod, line string) Entry {
	ts, msg := "", line
	if i := strings.IndexByte(line, ' '); i > 0 {
		if t, err := time.Parse(time.RFC3339Nano, line[:i]); err == nil {
			ts = t.UTC().Format(time.RFC3339Nano)
			msg = line[i+1:]
		}
	}
	return Entry{
		Timestamp: ts,
		Message:   msg,
		Labels: map[string]string{
			"service":  in.Name,
			"instance": pod,
			"type":     in.Kind,
		},
	}
}
