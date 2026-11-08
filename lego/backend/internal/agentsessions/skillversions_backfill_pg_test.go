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

package agentsessions

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// The startup pass against the real store: versions that hold 0210's SQL
// rendering of the six pinned descriptions become the Go rendering, keeping
// version, message, author and time with a recomputed short id; a second pass,
// and rows saved by the service, change nothing.
func TestBackfillSkillVersionContentPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	ws, err := pg.CreateTenant(ctx, fmt.Sprintf("skill-backfill-%d", time.Now().UnixNano()), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(ctx, ws.ID) })

	lim := store.AgentSkillLimits{Count: 50, EnabledBytes: 1 << 20}
	for i, c := range []struct{ description, sql string }{
		{"How to deploy", "How to deploy"},
		{"Deploy (prod), v2.0", "Deploy (prod), v2.0"},
		{"n/a", "n/a"},
		{"Deploy: staging", `"Deploy: staging"`},
		{"Fix #12", `"Fix #12"`},
		{"yes", `"yes"`},
	} {
		name := fmt.Sprintf("skill-%d", i)
		row := store.AgentSkill{ID: ids.New(ids.AgentSkill), WorkspaceID: ws.ID, Name: name, Description: c.description,
			Triggers: []string{"user", "model"}, Enabled: true, Body: "x\n", UpdatedBy: "admin"}
		sqlContent := "---\nname: " + name + "\ndescription: " + c.sql + "\n---\nx\n"
		row.VersionContent, row.VersionMessage = sqlContent, "Add skills/"+name+"/SKILL.md"
		saved, err := pg.CreateAgentSkill(ctx, row, lim)
		if err != nil {
			t.Fatal(err)
		}
		before, err := pg.GetAgentSkillVersion(ctx, saved.ID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := BackfillSkillVersionContent(ctx, pg); err != nil {
			t.Fatal(err)
		}
		want := renderSkill(saved, "")
		after, err := pg.GetAgentSkillVersion(ctx, saved.ID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if after.Content != want || after.ShortID != store.AgentSkillVersionShortID(saved.ID, 1, want) ||
			after.Version != 1 || after.Message != before.Message || after.CreatedBy != before.CreatedBy || !after.CreatedAt.Equal(before.CreatedAt) {
			t.Fatalf("description %q: version after the pass = %+v, want content %q (before %+v)", c.description, after, want, before)
		}
		if n, err := BackfillSkillVersionContent(ctx, pg); err != nil || n != 0 {
			t.Fatalf("description %q: second pass rewrote %d rows, err=%v", c.description, n, err)
		}
	}
}
