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

package postgres

import (
	"bufio"
	"context"
	"sort"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// DatabaseLogQuery is the filter set for QueryDatabaseLogs. It mirrors the
// managed-Postgres subset of logs.LogQuery; HTTP request-log filters do not
// apply to a database process stream.
type DatabaseLogQuery struct {
	Search    string
	Since     time.Time
	End       time.Time
	Limit     int64
	Direction string
	Instance  []string // restrict to these pod names (empty = all pods)
}

// DatabaseLogEntry is one CNPG log line in Render's log shape.
type DatabaseLogEntry struct {
	Timestamp string            `json:"timestamp,omitempty"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// DatabaseLogQuerySource is the narrow cross-feature seam that lets the
// dedicated Postgres compatibility adapters reuse the generic logs core
// without importing the logs package here.
type DatabaseLogQuerySource func(context.Context, string, DatabaseLogQuery) ([]DatabaseLogEntry, error)

const (
	defaultDBLogLimit = 20
	maxDBLogLimit     = 100
)

// QueryDatabaseLogs is the dedicated Postgres adapters' compatibility verb.
// Typed dpg- ids delegate to the generic durable logs core in production;
// legacy name-shaped CRs and isolated tests retain the direct CNPG pod path.
// Results are oldest-first and capped at q.Limit. ErrLogsUnavailable is
// returned when the selected path has no source; unknown instances return
// ErrNotFound.
func (s *Service) QueryDatabaseLogs(ctx context.Context, name string, q DatabaseLogQuery) ([]DatabaseLogEntry, error) {
	if kind, ok := ids.KindOf(name); ok && kind == ids.Postgres && s.DatabaseLogs != nil {
		return s.DatabaseLogs(ctx, name, q)
	}
	d, err := s.fetchDatabase(ctx, core.RelCanViewLogs, name)
	if err != nil {
		return nil, err
	}
	if s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	if q.Limit <= 0 {
		q.Limit = defaultDBLogLimit
	}
	if q.Limit > maxDBLogLimit {
		q.Limit = maxDBLogLimit
	}
	pods, err := s.DatabasePods(ctx, d.Name)
	if err != nil {
		return nil, err
	}
	searchLower := strings.ToLower(q.Search)
	var out []DatabaseLogEntry
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
		entries, err := s.readDBPodLogs(ctx, d.Name, pod, q.Limit)
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

func (s *Service) readDBPodLogs(ctx context.Context, db, pod string, tail int64) ([]DatabaseLogEntry, error) {
	rc, err := s.PodLogs(ctx, s.Namespace, pod, core.CNPGPostgresContainer, tail)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	var entries []DatabaseLogEntry
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		entries = append(entries, parseDBLogLine(db, pod, sc.Text()))
	}
	return entries, sc.Err()
}

func parseDBLogLine(db, pod, line string) DatabaseLogEntry {
	ts, msg := "", line
	if i := strings.IndexByte(line, ' '); i > 0 {
		if t, err := time.Parse(time.RFC3339Nano, line[:i]); err == nil {
			ts = t.UTC().Format(time.RFC3339Nano)
			msg = line[i+1:]
		}
	}
	return DatabaseLogEntry{
		Timestamp: ts,
		Message:   msg,
		Labels: map[string]string{
			"service":  db,
			"instance": pod,
			"type":     "postgres",
		},
	}
}
