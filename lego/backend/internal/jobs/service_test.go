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

package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type recordingFactWriter struct{ facts []store.ServiceEventFact }

func (w *recordingFactWriter) InsertServiceEventFact(_ context.Context, f store.ServiceEventFact) (bool, error) {
	for _, existing := range w.facts {
		if existing.SourceKey == f.SourceKey {
			return false, nil // idempotent, like the real store's ON CONFLICT DO NOTHING
		}
	}
	w.facts = append(w.facts, f)
	return true, nil
}

// TestRecordJobRunEnded proves job_run_ended (w7/m66) fires only for a finished
// job (succeeded/failed) and never for a cancel (which keeps its job_canceled
// audit event) or a still-running/pending one.
func TestRecordJobRunEnded(t *testing.T) {
	finished := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		status string
		want   string // expected fact status, or "" for no fact
	}{
		{"succeeded", store.JobSucceeded, store.EventStatusSucceeded},
		{"failed", store.JobFailed, store.EventStatusFailed},
		{"canceled emits nothing (job_canceled covers it)", store.JobCanceled, ""},
		{"running emits nothing", store.JobRunning, ""},
		{"pending emits nothing", store.JobPending, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			writer := &recordingFactWriter{}
			svc := &Service{Base: &core.Base{}, EventFacts: writer}
			svc.recordJobRunEnded(context.Background(), "srv-1",
				store.Job{ID: "job-1", Status: c.status, FinishedAt: &finished})
			if c.want == "" {
				if len(writer.facts) != 0 {
					t.Fatalf("status %q recorded %d facts, want 0", c.status, len(writer.facts))
				}
				return
			}
			if len(writer.facts) != 1 {
				t.Fatalf("status %q recorded %d facts, want 1", c.status, len(writer.facts))
			}
			f := writer.facts[0]
			if f.Type != store.EventFactJobRunEnded || f.Status != c.want ||
				f.SourceKey != "job:job-1:run_ended" || f.AppID != "srv-1" || !f.At.Equal(finished) {
				t.Fatalf("job_run_ended fact = %+v", f)
			}
		})
	}
}

// TestRecordJobRunEndedNoOpWithoutWriterOrApp proves the two degrade paths:
// no EventFacts writer (store off) and a hand-applied service (no app id) both
// no-op silently rather than panic.
func TestRecordJobRunEndedNoOpWithoutWriterOrApp(t *testing.T) {
	// nil EventFacts: no panic, no-op.
	svc := &Service{Base: &core.Base{}}
	svc.recordJobRunEnded(context.Background(), "srv-1", store.Job{ID: "job-1", Status: store.JobSucceeded})

	// empty appID (hand-applied service, no Postgres row to key a fact on): no fact.
	writer := &recordingFactWriter{}
	svc2 := &Service{Base: &core.Base{}, EventFacts: writer}
	svc2.recordJobRunEnded(context.Background(), "", store.Job{ID: "job-1", Status: store.JobSucceeded})
	if len(writer.facts) != 0 {
		t.Fatalf("empty appID recorded %d facts, want 0", len(writer.facts))
	}
}
