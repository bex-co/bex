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

// w3/m318: a sandbox record outlives the OpenSandbox object. The first end
// wins and keeps the view written at create, an ended record is never
// rewritten as live, the sweeper ends only old
// absent records, and a record ended more than 30 days ago is deleted, along
// with the execution history it kept.
func TestSandboxRecordsPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-records").ID
	other := lifecycleTenant(t, st, "sandbox-records-other").ID
	stamp := time.Now().UnixNano()
	ended, absent, fresh := fmt.Sprintf("sbx-e%d", stamp), fmt.Sprintf("sbx-a%d", stamp), fmt.Sprintf("sbx-f%d", stamp)
	view := func(v string) []byte { return []byte(`{"id":"` + v + `"}`) }
	for _, id := range []string{ended, absent, fresh} {
		if err := st.RecordSandbox(ctx, SandboxRecord{WorkspaceID: ws, SandboxID: id, Owner: "usr-1", View: view("created")}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.EndedSandbox(ctx, ws, ended); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a live record read as ended: %v", err)
	}

	first := time.Now().UTC().Truncate(time.Second)
	if err := st.RecordSandbox(ctx, SandboxRecord{WorkspaceID: ws, SandboxID: ended, Owner: "usr-1", View: view("at-end"), TerminatedAt: &first}); err != nil {
		t.Fatal(err)
	}
	later := first.Add(time.Hour)
	if err := st.RecordSandbox(ctx, SandboxRecord{WorkspaceID: ws, SandboxID: ended, Owner: "usr-1", View: view("again"), TerminatedAt: &later}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordSandbox(ctx, SandboxRecord{WorkspaceID: ws, SandboxID: ended, Owner: "usr-1", View: view("revived")}); err != nil {
		t.Fatal(err)
	}
	got, err := st.EndedSandbox(ctx, ws, ended)
	if err != nil || got.TerminatedAt == nil || !got.TerminatedAt.Equal(first) || string(got.View) != `{"id": "created"}` || got.Owner != "usr-1" {
		t.Fatalf("ended record = %+v %s (err %v), want the first end and the view written at create", got, got.View, err)
	}
	if _, err := st.EndedSandbox(ctx, other, ended); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another workspace read the record: %v", err)
	}
	// A sandbox from before records existed is recorded at its end.
	legacy := fmt.Sprintf("sbx-l%d", stamp)
	if err := st.RecordSandbox(ctx, SandboxRecord{WorkspaceID: ws, SandboxID: legacy, Owner: "usr-2", View: view("legacy"), TerminatedAt: &first}); err != nil {
		t.Fatal(err)
	}

	// The sweeper ends only absent records older than ten minutes.
	if _, err := st.Pool.Exec(ctx, `UPDATE sandbox_records SET created_at = now() - interval '1 hour' WHERE workspace_id = $1 AND sandbox_id <> $2`, ws, fresh); err != nil {
		t.Fatal(err)
	}
	swept := first.Add(2 * time.Minute)
	if err := st.EndAbsentSandboxes(ctx, ws, nil, swept); err != nil {
		t.Fatal(err)
	}
	list, err := st.EndedSandboxes(ctx, ws)
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, r := range list {
		listed = append(listed, r.SandboxID)
	}
	if want := []string{absent, ended, legacy}; !slices.Equal(listed, want) {
		t.Fatalf("ended records = %v, want %v newest first and not the fresh %s", listed, want, fresh)
	}
	if workspaces, err := st.EndedSandboxWorkspaces(ctx, absent); err != nil || !slices.Equal(workspaces, []string{ws}) {
		t.Fatalf("ended workspaces of %s = %v (err %v), want [%s]", absent, workspaces, err, ws)
	}

	// An ended sandbox's executions survive the prune of absent sandboxes.
	if err := st.RecordSandboxExecution(ctx, SandboxExecution{ID: "sbe-" + ended, WorkspaceID: ws, SandboxID: ended, Type: "runs", Operation: "sync", Command: "ls", StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE sandbox_executions SET started_at = now() - interval '1 hour' WHERE workspace_id = $1`, ws); err != nil {
		t.Fatal(err)
	}
	if err := st.PruneSandboxExecutions(ctx, ws, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SandboxExecution(ctx, ws, ended, "sbe-"+ended); err != nil {
		t.Fatalf("an ended sandbox's execution was pruned: %v", err)
	}

	// Thirty days after its end a record is deleted.
	if err := st.ExpireSandboxRecords(ctx, first.Add(SandboxRecordRetention+time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EndedSandbox(ctx, ws, ended); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a record ended 30 days ago is still readable: %v", err)
	}
	if _, err := st.EndedSandbox(ctx, ws, absent); err != nil {
		t.Fatalf("a record ended within 30 days expired: %v", err)
	}
}
