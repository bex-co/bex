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
// Collect is the adapters' own fallback: in production both features delegate
// to the generic logs core, which reads Loki or, without it, the same pods
// through ParseLine. Collect serves isolated tests and a Key Value addressed
// by a name rather than its id.
package datastorelogs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
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
	// Empty-but-non-nil: a datastore with no matching log lines answers a
	// declared array, so it must be [] on every surface that nests this result
	// rather than `null` (w4/m116/t006).
	out := []Entry{}
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

	sort.SliceStable(out, func(i, j int) bool { return core.TimestampLess(out[i].Timestamp, out[j].Timestamp) })
	if lim := q.Limit; int64(len(out)) > lim {
		if q.Direction == core.DirectionForward {
			out = out[:lim]
		} else {
			out = out[int64(len(out))-lim:]
		}
	}
	return out, nil
}

// within reports whether an entry's timestamp, which parseLine parsed, falls
// inside the query window.
func (q Query) within(timestamp string) bool {
	if q.Since.IsZero() && q.End.IsZero() {
		return true
	}
	t, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return false
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
		if e, keep := in.parseLine(pod, sc.Text()); keep {
			entries = append(entries, e)
		}
	}
	return entries, sc.Err()
}

// ParseLine reads one raw line of a datastore pod's log, the rule both direct
// reads apply, this package's and the logs service's (w5/m122): its stamp and
// message, unwrapped by CNPGLine for Postgres with its level. keep is false for
// a line without the kubelet's stamp, which is the kubelet's own answer, and
// for CNPG's operational chatter.
func ParseLine(kind, line string) (ts, msg, level string, keep bool) {
	ts, msg, ok := core.SplitPodLogLine(line)
	if !ok {
		return "", "", "", false
	}
	if kind == KindPostgres {
		if msg, level, keep = CNPGLine(msg); !keep {
			return "", "", "", false
		}
	}
	return ts, msg, level, true
}

func (in Instance) parseLine(pod, line string) (Entry, bool) {
	ts, msg, level, keep := ParseLine(in.Kind, line)
	if !keep {
		return Entry{}, false
	}
	labels := map[string]string{
		"service":  in.Name,
		"instance": pod,
		"type":     in.Kind,
	}
	if level != "" {
		labels["level"] = level
	}
	return Entry{Timestamp: ts, Message: msg, Labels: labels}, true
}

// CNPGLine unwraps one line of a CNPG postgres container (w8/030), by the
// allow-list the log shipper's type=postgres pipeline applies (w5/m122; one
// fixture file tests both). That container's PID 1 is CNPG's instance
// manager: it logs its own JSON and re-emits PostgreSQL's csvlog as
// logger=postgres msg=record with the line under record. A record becomes
// PostgreSQL's own stderr shape with its severity as the level; a line that is
// not a JSON object passes unchanged; any other JSON is the manager's and is
// dropped (keep=false). It reads the line as the shipper's JSON stage does:
// exact keys, any value types.
func CNPGLine(line string) (message, level string, keep bool) {
	var entry map[string]any
	if json.Unmarshal([]byte(line), &entry) != nil {
		return line, "", true
	}
	record, isRecord := entry["record"].(map[string]any)
	if entry["logger"] != "postgres" || entry["msg"] != "record" || !isRecord {
		return "", "", false
	}
	severity := field(record["error_severity"])
	message = fmt.Sprintf("%s [%s] %s:  %s", field(record["log_time"]), field(record["process_id"]), severity, field(record["message"]))
	if detail := field(record["detail"]); detail != "" {
		message += " DETAIL:  " + detail
	}
	if hint := field(record["hint"]); hint != "" {
		message += " HINT:  " + hint
	}
	return message, postgresLevel(severity), true
}

// field renders one record value as the shipper's template does: absent or
// null as nothing, a string as itself, a number in plain decimal, and any
// other value as its JSON.
func field(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// postgresLevel maps a PostgreSQL error_severity onto the shipper's level
// vocabulary.
func postgresLevel(severity string) string {
	switch severity {
	case "ERROR", "FATAL", "PANIC":
		return "error"
	case "WARNING":
		return "warning"
	case "LOG", "INFO", "NOTICE":
		return "info"
	default:
		return "debug"
	}
}
