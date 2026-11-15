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

package objectgw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// ReportInterval is how often the gateway tells bex-api what it sent. A gateway
// that dies loses at most this much of a shadow meter, which is shown at $0
// (ADR112 D8).
const ReportInterval = time.Minute

// Reporter delivers a gateway's byte counts to bex-api.
type Reporter interface {
	Report(ctx context.Context, bytes map[string]int64) error
}

// HTTPReporter posts the counts to bex-api's internal listener (verbs.go).
type HTTPReporter struct{ APIClient }

func (r *HTTPReporter) Report(ctx context.Context, counts map[string]int64) error {
	body, err := json.Marshal(EgressReport{Bytes: counts})
	if err != nil {
		return err
	}
	resp, err := r.do(ctx, http.MethodPost, EgressPath, body)
	if err != nil {
		return err
	}
	defer drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bex-api answered the report with status %d", resp.StatusCode)
	}
	return nil
}

// Meter counts the bytes the gateway sends, per workspace, and reports them.
// Counting costs one map write on the request path and never fails or blocks a
// request. A report that fails is kept and sent again with the next one.
type Meter struct {
	reporter Reporter

	mu      sync.Mutex
	pending map[string]int64
}

// NewMeter returns a meter that reports through r.
func NewMeter(r Reporter) *Meter {
	return &Meter{reporter: r, pending: map[string]int64{}}
}

// Add counts n bytes sent for a workspace. Past MaxReportWorkspaces distinct
// workspaces held (bex-api unreachable for a long time) a new one is dropped:
// the shadow meter undercounts, a request never fails.
func (m *Meter) Add(workspaceID string, n int64) {
	if m == nil || n <= 0 || workspaceID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, held := m.pending[workspaceID]; !held && len(m.pending) >= MaxReportWorkspaces {
		return
	}
	m.pending[workspaceID] += n
}

// Flush reports what has been counted since the last successful flush. A report
// whose answer was lost is sent again with the next, so it can count twice: the
// meter is a shadow meter, shown at $0.
func (m *Meter) Flush(ctx context.Context) error {
	m.mu.Lock()
	batch := m.pending
	m.pending = map[string]int64{}
	m.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	if err := m.reporter.Report(ctx, batch); err != nil {
		m.mu.Lock()
		for ws, n := range batch {
			if _, held := m.pending[ws]; held || len(m.pending) < MaxReportWorkspaces {
				m.pending[ws] += n
			}
		}
		m.mu.Unlock()
		return err
	}
	return nil
}

// Run flushes every interval until ctx ends. It does not flush on the way out:
// the process does that once its server has drained (cmd/objectgw), when the
// last responses have been counted.
func (m *Meter) Run(ctx context.Context, interval time.Duration) {
	core.Poll(ctx, "object gateway egress report", interval, m.Flush)
}
