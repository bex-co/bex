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
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/signins"
)

// Two bex-api replicas seeing one new Kratos session at the same moment record
// one LoginEvent per workspace the member belongs to, and no row in a
// workspace they do not belong to; a second session is a second sign-in, and
// a sign-out is one LogoutEvent per workspace (w9/m452).
func TestPGSignInsRecordOnceAcrossReplicas(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	subject := "kratos-" + ids.New(ids.Workspace)
	workspaces := []string{ids.New(ids.Workspace), ids.New(ids.Workspace), ids.New(ids.Workspace)}
	t.Cleanup(func() {
		for _, w := range workspaces {
			_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, w)
			_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, w)
		}
	})
	for _, w := range workspaces {
		if _, err := pool.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $1)`, w); err != nil {
			t.Fatal(err)
		}
	}
	for _, w := range workspaces[:2] {
		if _, err := pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'member')`, w, subject); err != nil {
			t.Fatal(err)
		}
	}
	session := signins.Session{
		ID: "sess-" + ids.New(ids.Workspace), AuthenticatedAt: time.Now().Add(-time.Minute).UTC().Truncate(time.Microsecond),
		SourceIP: "198.51.100.7", UserAgent: "Mozilla/5.0",
	}
	count := func(verb string) map[string]int {
		t.Helper()
		got := map[string]int{}
		for _, w := range workspaces {
			var n int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE workspace_id = $1 AND verb = $2 AND caller = $3`, w, verb, subject).Scan(&n); err != nil {
				t.Fatal(err)
			}
			got[w] = n
		}
		return got
	}

	var wg sync.WaitGroup
	for range 2 {
		replica := &signins.Recorder{Workspaces: st, Audit: st}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 3 {
				replica.ObserveSession(ctx, subject, session)
			}
		}()
	}
	wg.Wait()
	deadline := time.Now().Add(10 * time.Second)
	for got := count(core.AuditVerbLogin); got[workspaces[0]] < 1 || got[workspaces[1]] < 1; got = count(core.AuditVerbLogin) {
		if time.Now().After(deadline) {
			t.Fatalf("sign-in rows = %v, want one in each member workspace", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Let any write still in flight land before counting duplicates.
	time.Sleep(200 * time.Millisecond)
	if got := count(core.AuditVerbLogin); got[workspaces[0]] != 1 || got[workspaces[1]] != 1 || got[workspaces[2]] != 0 {
		t.Errorf("sign-in rows = %v, want 1, 1 and none outside the membership", got)
	}
	rows, err := st.ListAuditEvents(ctx, workspaces[0], AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].At.Equal(session.AuthenticatedAt) || rows[0].SourceIP != "198.51.100.7" ||
		rows[0].UserAgent != "Mozilla/5.0" || rows[0].CallerMethod != core.MethodSession {
		t.Errorf("sign-in row = %+v, want dated at authentication, from the session's origin", rows)
	}

	recorder := &signins.Recorder{Workspaces: st, Audit: st}
	if err := recorder.RecordSignOut(ctx, subject, core.MethodSession); err != nil {
		t.Fatal(err)
	}
	if got := count(core.AuditVerbLogout); got[workspaces[0]] != 1 || got[workspaces[1]] != 1 || got[workspaces[2]] != 0 {
		t.Errorf("sign-out rows = %v, want 1, 1 and none outside the membership", got)
	}
}
