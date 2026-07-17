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

package keyvalue

import (
	"bufio"
	"context"
	"sort"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// KeyValueLogQuery is the filter set for QueryKeyValueLogs. It mirrors the
// managed Key Value subset of logs.LogQuery; HTTP request-log filters do not
// apply to a Valkey process stream.
type KeyValueLogQuery struct {
	Search    string
	Since     time.Time
	End       time.Time
	Limit     int64
	Direction string
	Instance  []string // restrict to these pod names (empty = all pods)
}

// KeyValueLogEntry is one Valkey log line in Render's log shape.
type KeyValueLogEntry struct {
	Timestamp string            `json:"timestamp,omitempty"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// KeyValueLogQuerySource is the narrow cross-feature seam that lets the
// dedicated Key Value compatibility adapters reuse the generic logs core
// without importing the logs package here.
type KeyValueLogQuerySource func(context.Context, string, KeyValueLogQuery) ([]KeyValueLogEntry, error)

const (
	defaultKVLogLimit = 20
	maxKVLogLimit     = 100
)

// QueryKeyValueLogs is the dedicated Key Value adapters' compatibility verb.
// Typed red- ids delegate to the generic durable logs core in production;
// isolated tests retain the direct Valkey pod path.
// Results are oldest-first and capped at q.Limit. ErrLogsUnavailable is
// returned when the selected path has no source; unknown instances return
// ErrNotFound.
func (s *Service) QueryKeyValueLogs(ctx context.Context, name string, q KeyValueLogQuery) ([]KeyValueLogEntry, error) {
	if kind, ok := ids.KindOf(name); ok && kind == ids.KeyValue && s.KeyValueLogs != nil {
		return s.KeyValueLogs(ctx, name, q)
	}
	kv, err := s.fetchKeyValue(ctx, core.RelCanViewLogs, name)
	if err != nil {
		return nil, err
	}
	if s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	if q.Limit <= 0 {
		q.Limit = defaultKVLogLimit
	}
	if q.Limit > maxKVLogLimit {
		q.Limit = maxKVLogLimit
	}
	pods, err := s.KeyValuePods(ctx, kv.Name)
	if err != nil {
		return nil, err
	}
	searchLower := strings.ToLower(q.Search)
	var out []KeyValueLogEntry
	for i := range pods {
		pod := pods[i].Name
		if len(q.Instance) > 0 {
			found := false
			for _, inst := range q.Instance {
				if inst == pod {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		entries, err := s.readKVPodLogs(ctx, kv.Name, pod, q.Limit)
		if err != nil {
			// A pod that has restarted or been reaped just goes missing.
			continue
		}
		for _, e := range entries {
			if searchLower != "" && !strings.Contains(strings.ToLower(e.Message), searchLower) {
				continue
			}
			if !q.Since.IsZero() || !q.End.IsZero() {
				if t, err2 := time.Parse(time.RFC3339Nano, e.Timestamp); err2 == nil {
					if !q.Since.IsZero() && t.Before(q.Since) {
						continue
					}
					if !q.End.IsZero() && t.After(q.End) {
						continue
					}
				}
			}
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	lim := q.Limit
	if int64(len(out)) > lim {
		if q.Direction == core.DirectionForward {
			out = out[:lim]
		} else {
			out = out[int64(len(out))-lim:]
		}
	}
	return out, nil
}

func (s *Service) readKVPodLogs(ctx context.Context, kv, pod string, tail int64) ([]KeyValueLogEntry, error) {
	rc, err := s.PodLogs(ctx, s.Namespace, pod, core.ValkeyContainer, tail)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	var entries []KeyValueLogEntry
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		entries = append(entries, parseKVLogLine(kv, pod, sc.Text()))
	}
	return entries, sc.Err()
}

func parseKVLogLine(kv, pod, line string) KeyValueLogEntry {
	ts, msg := "", line
	if i := strings.IndexByte(line, ' '); i > 0 {
		if t, err := time.Parse(time.RFC3339Nano, line[:i]); err == nil {
			ts = t.UTC().Format(time.RFC3339Nano)
			msg = line[i+1:]
		}
	}
	return KeyValueLogEntry{
		Timestamp: ts,
		Message:   msg,
		Labels: map[string]string{
			"service":  kv,
			"instance": pod,
			"type":     "keyvalue",
		},
	}
}
