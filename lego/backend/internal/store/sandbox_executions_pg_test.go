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

package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
)

// w32/m298: Render's execution history lists newest first, pages by
// execution id, stops a record once, and is pruned with its sandbox.
func TestSandboxExecutionsPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-execs").ID
	other := lifecycleTenant(t, st, "sandbox-execs-other").ID
	stamp := time.Now().UnixNano()
	box, gone := fmt.Sprintf("sbx-x%d", stamp), fmt.Sprintf("sbx-y%d", stamp)
	base := time.Now().Add(-time.Minute).UTC().Truncate(time.Microsecond)
	ids := make([]string, 3)
	for i := range ids {
		ids[i] = fmt.Sprintf("exe-%d-%d", stamp, i)
		e := SandboxExecution{ID: ids[i], WorkspaceID: ws, SandboxID: box, Type: "runs", Operation: "stream",
			Command: fmt.Sprintf("echo %d", i), Subject: "subject-a", StartedAt: base.Add(time.Duration(i) * time.Second)}
		if err := st.RecordSandboxExecution(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.RecordSandboxExecution(ctx, SandboxExecution{ID: fmt.Sprintf("exe-%d-o", stamp), WorkspaceID: other, SandboxID: box,
		Type: "files", Operation: "upload", Command: "/a", Subject: "s", StartedAt: base}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordSandboxExecution(ctx, SandboxExecution{ID: fmt.Sprintf("exe-%d-bad", stamp), WorkspaceID: ws, SandboxID: box,
		Type: "jobs", Operation: "stream", Subject: "s", StartedAt: base}); err == nil {
		t.Error("an execution type outside Render's runs|files was stored")
	}

	page, err := st.SandboxExecutions(ctx, ws, box, "", 2)
	if err != nil || len(page) != 2 || page[0].ID != ids[2] || page[1].ID != ids[1] {
		t.Fatalf("first page = %+v (err %v), want the two newest, newest first", page, err)
	}
	page, err = st.SandboxExecutions(ctx, ws, box, page[1].ID, 2)
	if err != nil || len(page) != 1 || page[0].ID != ids[0] {
		t.Fatalf("second page = %+v (err %v), want the oldest only", page, err)
	}
	if page, _ = st.SandboxExecutions(ctx, ws, box, "exe-unknown", 10); len(page) != 0 {
		t.Errorf("an unknown cursor returned %d rows", len(page))
	}

	three := 3
	if err := st.StopSandboxExecution(ctx, ws, ids[2], base.Add(time.Hour), &three); err != nil {
		t.Fatal(err)
	}
	// A later stop (the sandbox's finalization) leaves the first one standing.
	if err := st.StopOpenSandboxExecutions(ctx, ws, box, base.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := st.SandboxExecution(ctx, ws, box, ids[2])
	if err != nil || got.ExitCode == nil || *got.ExitCode != 3 || !got.StoppedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("stopped record = %+v (err %v), want exit 3 at the first stop", got, err)
	}
	if got, _ = st.SandboxExecution(ctx, ws, box, ids[0]); got.StoppedAt == nil || got.ExitCode != nil {
		t.Fatalf("finalized record = %+v, want stopped with no exit code", got)
	}
	if _, err := st.SandboxExecution(ctx, other, box, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Errorf("another workspace read the execution: %v", err)
	}

	// Pruned with its sandbox, and only once the rows are not fresh.
	if err := st.RecordSandboxExecution(ctx, SandboxExecution{ID: fmt.Sprintf("exe-%d-g", stamp), WorkspaceID: ws, SandboxID: gone,
		Type: "runs", Operation: "sync", Subject: "s", StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := st.PruneSandboxExecutions(ctx, ws, []string{box}); err != nil {
		t.Fatal(err)
	}
	if page, _ = st.SandboxExecutions(ctx, ws, gone, "", 10); len(page) != 1 {
		t.Fatalf("a fresh row of an unlisted sandbox was pruned")
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE sandbox_executions SET started_at = now() - interval '1 hour' WHERE workspace_id = $1`, ws); err != nil {
		t.Fatal(err)
	}
	if err := st.PruneSandboxExecutions(ctx, ws, []string{box}); err != nil {
		t.Fatal(err)
	}
	if page, _ = st.SandboxExecutions(ctx, ws, gone, "", 10); len(page) != 0 {
		t.Errorf("a gone sandbox kept %d executions", len(page))
	}
	if page, _ = st.SandboxExecutions(ctx, ws, box, "", 10); len(page) != 3 {
		t.Errorf("a live sandbox lost executions: %d left", len(page))
	}
	// Past the retention, even a live sandbox's oldest record goes.
	if _, err := st.Pool.Exec(ctx, `UPDATE sandbox_executions SET started_at = now() - interval '31 days' WHERE id = $1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	if err := st.PruneSandboxExecutions(ctx, ws, []string{box}); err != nil {
		t.Fatal(err)
	}
	if page, _ = st.SandboxExecutions(ctx, ws, box, "", 10); len(page) != 2 || slices.ContainsFunc(page, func(e SandboxExecution) bool { return e.ID == ids[0] }) {
		t.Errorf("after the retention prune = %+v, want the two recent records", page)
	}
}

// w8/m151: an open execution refreshed by its stream stays open; one whose
// stream went away is stopped at its last heartbeat, or at its start when it
// was never seen; a stopped record is left alone.
func TestSandboxExecutionHeartbeatPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-exec-heartbeat").ID
	stamp := time.Now().UnixNano()
	box := fmt.Sprintf("sbx-h%d", stamp)
	start := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	live, seen, never := fmt.Sprintf("exe-%d-live", stamp), fmt.Sprintf("exe-%d-seen", stamp), fmt.Sprintf("exe-%d-never", stamp)
	for _, id := range []string{live, seen, never} {
		if err := st.RecordSandboxExecution(ctx, SandboxExecution{ID: id, WorkspaceID: ws, SandboxID: box, Type: "runs", Operation: "stream", Subject: "s", StartedAt: start}); err != nil {
			t.Fatal(err)
		}
	}
	now := start.Add(time.Hour)
	if err := st.TouchSandboxExecution(ctx, ws, live, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := st.TouchSandboxExecution(ctx, ws, seen, start.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := st.StopStaleSandboxExecutions(ctx, ws, box, now.Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]*time.Time{live: nil, seen: ptrTime(start.Add(10 * time.Minute)), never: ptrTime(start)} {
		e, err := st.SandboxExecution(ctx, ws, box, id)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case want == nil && e.StoppedAt != nil:
			t.Errorf("%s: stopped at %v, want open (refreshed a minute ago)", id, e.StoppedAt)
		case want != nil && (e.StoppedAt == nil || !e.StoppedAt.Equal(*want)):
			t.Errorf("%s: stopped at %v, want %v", id, e.StoppedAt, *want)
		}
	}
	// A touch after a stop changes nothing, and a stopped record keeps its end.
	if err := st.TouchSandboxExecution(ctx, ws, never, now); err != nil {
		t.Fatal(err)
	}
	if err := st.StopStaleSandboxExecutions(ctx, ws, box, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if e, _ := st.SandboxExecution(ctx, ws, box, never); e.StoppedAt == nil || !e.StoppedAt.Equal(start) {
		t.Errorf("a stopped record moved: %v, want %v", e.StoppedAt, start)
	}
}
