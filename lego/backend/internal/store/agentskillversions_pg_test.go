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
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// backfillDescriptions pins how the 0210 SQL renders a description. sql is the
// text after "description: " in the backfilled content. agentsessions'
// TestRenderedDescriptionsMatchTheMigrationBackfill pins the Go renderer to the
// same table: where the two renderers agree the text is the same; where they differ the
// comments give the Go output.
var backfillDescriptions = []struct{ description, sql string }{
	{"How to deploy", "How to deploy"},
	{"Deploy (prod), v2.0", "Deploy (prod), v2.0"},
	{"n/a", "n/a"},
	{"Deploy: staging", `"Deploy: staging"`}, // Go: 'Deploy: staging'
	{"Fix #12", `"Fix #12"`},                 // Go: 'Fix #12'
	{"yes", `"yes"`},                         // Go: yes
}

func versioned(s AgentSkill, content, message string) AgentSkill {
	s.VersionContent, s.VersionMessage = content, message
	return s
}

func listVersionNumbers(t *testing.T, s *PGStore, skillID string, before, limit int) []int {
	t.Helper()
	rows, err := s.ListAgentSkillVersions(context.Background(), skillID, before, limit)
	if err != nil {
		t.Fatal(err)
	}
	out := []int{}
	for _, v := range rows {
		if v.Content != "" {
			t.Fatalf("a version list carries content: %+v", v)
		}
		out = append(out, v.Version)
	}
	return out
}

// ADR099 D2: every save writes exactly one version in its own transaction,
// a refused save writes none, and a version is immutable.
func TestPGAgentSkillVersionsOnePerSave(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-skill-versions")
	ctx := context.Background()
	lim := AgentSkillLimits{Count: 10, EnabledBytes: 1 << 20}
	created, err := s.CreateAgentSkill(ctx, versioned(newSkill(tenant.ID, "deploy"), "v1 content\n", "Add skills/deploy/SKILL.md"), lim)
	if err != nil {
		t.Fatal(err)
	}
	next := versioned(created, "v2 content\n", "Update skills/deploy/SKILL.md")
	next.Body, next.UpdatedBy = "second", "dev"
	if _, err := s.UpdateAgentSkill(ctx, next, 1, lim); err != nil {
		t.Fatal(err)
	}

	// Refused saves leave the history alone: a stale version, a missing row,
	// a taken name and a cap breach.
	if _, err := s.UpdateAgentSkill(ctx, versioned(next, "stale\n", "x"), 1, lim); !errors.Is(err, ErrAgentSkillVersion) {
		t.Fatalf("stale update = %v", err)
	}
	if _, err := s.CreateAgentSkill(ctx, versioned(newSkill(tenant.ID, "deploy"), "dup\n", "x"), lim); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate create = %v", err)
	}
	other, err := s.CreateAgentSkill(ctx, versioned(newSkill(tenant.ID, "other"), "o1\n", "Add skills/other/SKILL.md"), lim)
	if err != nil {
		t.Fatal(err)
	}
	clash := versioned(other, "clash\n", "x")
	clash.Name = "deploy"
	if _, err := s.UpdateAgentSkill(ctx, clash, 1, lim); !errors.Is(err, ErrConflict) {
		t.Fatalf("rename onto a taken name = %v", err)
	}
	big := versioned(created, "big\n", "x")
	big.Body = "0123456789"
	if _, err := s.UpdateAgentSkill(ctx, big, 2, AgentSkillLimits{Count: 10, EnabledBytes: 1}); !errors.Is(err, ErrAgentSkillBytes) {
		t.Fatalf("over the byte cap = %v", err)
	}
	if _, err := s.CreateAgentSkill(ctx, versioned(newSkill(tenant.ID, "third"), "t\n", "x"), AgentSkillLimits{Count: 2, EnabledBytes: 1 << 20}); !errors.Is(err, ErrAgentSkillLimit) {
		t.Fatalf("over the count cap = %v", err)
	}
	if _, err := s.UpdateAgentSkill(ctx, versioned(newSkill(tenant.ID, "ghost"), "g\n", "x"), 1, lim); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing skill = %v", err)
	}
	if got := listVersionNumbers(t, s, created.ID, 0, 10); fmt.Sprint(got) != "[2 1]" {
		t.Fatalf("versions after refused saves = %v, want [2 1]", got)
	}
	if got := listVersionNumbers(t, s, other.ID, 0, 10); fmt.Sprint(got) != "[1]" {
		t.Fatalf("versions of the other skill = %v", got)
	}
	if _, err := s.GetAgentSkillVersion(ctx, "ghost", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("version of a missing skill = %v", err)
	}

	v1, err := s.GetAgentSkillVersion(ctx, created.ID, 1)
	if err != nil || v1.Content != "v1 content\n" || v1.Message != "Add skills/deploy/SKILL.md" || v1.CreatedBy != "admin" || v1.CreatedAt.IsZero() {
		t.Fatalf("version 1 = %+v err=%v", v1, err)
	}
	v2, err := s.GetAgentSkillVersion(ctx, created.ID, 2)
	if err != nil || v2.Content != "v2 content\n" || v2.CreatedBy != "dev" || v2.Message != "Update skills/deploy/SKILL.md" {
		t.Fatalf("version 2 = %+v err=%v", v2, err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{7}$`).MatchString(v1.ShortID) || v1.ShortID == v2.ShortID ||
		v1.ShortID != AgentSkillVersionShortID(created.ID, 1, "v1 content\n") {
		t.Fatalf("short ids %q %q", v1.ShortID, v2.ShortID)
	}
	if _, err := s.GetAgentSkillVersion(ctx, created.ID, 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("version 3 = %v", err)
	}
}

func TestPGAgentSkillVersionPagingRetentionAndDelete(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-skill-version-retention")
	ctx := context.Background()
	lim := AgentSkillLimits{Count: 10, EnabledBytes: 1 << 20}
	cur, err := s.CreateAgentSkill(ctx, versioned(newSkill(tenant.ID, "deploy"), "c1\n", "Add"), lim)
	if err != nil {
		t.Fatal(err)
	}
	saves := AgentSkillVersionRetention + 5
	for v := 2; v <= saves; v++ {
		cur, err = s.UpdateAgentSkill(ctx, versioned(cur, fmt.Sprintf("c%d\n", v), "Update"), v-1, lim)
		if err != nil || cur.Version != v {
			t.Fatalf("save %d = %+v err=%v", v, cur, err)
		}
	}
	// The newest 200 stay; the oldest were dropped in the saving transactions.
	var n, oldest int
	if err := pool.QueryRow(ctx, `SELECT count(*), min(version) FROM agent_skill_versions WHERE skill_id=$1`, cur.ID).Scan(&n, &oldest); err != nil {
		t.Fatal(err)
	}
	if n != AgentSkillVersionRetention || oldest != saves-AgentSkillVersionRetention+1 {
		t.Fatalf("kept %d versions from %d, want %d from %d", n, oldest, AgentSkillVersionRetention, saves-AgentSkillVersionRetention+1)
	}
	if _, err := s.GetAgentSkillVersion(ctx, cur.ID, 5); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a dropped version = %v", err)
	}
	if _, err := s.GetAgentSkillVersion(ctx, cur.ID, saves); err != nil {
		t.Fatalf("the newest version = %v", err)
	}

	// Stable keyset paging, newest first, covering every kept version once.
	var all []int
	before := 0
	for {
		page := listVersionNumbers(t, s, cur.ID, before, 60)
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		before = page[len(page)-1]
	}
	if len(all) != AgentSkillVersionRetention || all[0] != saves || all[len(all)-1] != oldest {
		t.Fatalf("paged %d versions %d..%d", len(all), all[0], all[len(all)-1])
	}
	for i := 1; i < len(all); i++ {
		if all[i] != all[i-1]-1 {
			t.Fatalf("paging skipped or repeated near %d: %d then %d", i, all[i-1], all[i])
		}
	}

	// Deleting the skill, or its workspace, deletes its versions.
	other, err := s.CreateAgentSkill(ctx, versioned(newSkill(tenant.ID, "other"), "o\n", "Add"), lim)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAgentSkill(ctx, cur.ID); err != nil {
		t.Fatal(err)
	}
	if got := listVersionNumbers(t, s, cur.ID, 0, 10); len(got) != 0 {
		t.Fatalf("%d versions outlived their skill", len(got))
	}
	if got := listVersionNumbers(t, s, other.ID, 0, 10); len(got) != 1 {
		t.Fatalf("the other skill's versions = %v", got)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE id=$1`, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if got := listVersionNumbers(t, s, other.ID, 0, 10); len(got) != 0 {
		t.Fatalf("%d versions outlived their workspace", len(got))
	}
}

// 0210 gives every existing skill a version row for its current version, with
// the content rendered in SQL, and rolls back by dropping the table.
func TestAgentSkillVersionsMigrationBackfillsExistingSkills(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	read := func(name string) string {
		t.Helper()
		raw, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(label, sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}
	// The 0204 shape, without its workspace foreign key.
	exec("scratch schema", `
		CREATE SCHEMA migration_0210_skill_versions;
		SET LOCAL search_path TO migration_0210_skill_versions;
		CREATE TABLE agent_skills (
			id text PRIMARY KEY, workspace_id text NOT NULL, name text NOT NULL, description text NOT NULL,
			triggers text[] NOT NULL DEFAULT '{user,model}', enabled boolean NOT NULL DEFAULT true,
			body text NOT NULL, extra text NOT NULL DEFAULT '', version int NOT NULL DEFAULT 1,
			updated_by text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now());
		INSERT INTO agent_skills (id, workspace_id, name, description, body, version, updated_by)
			VALUES ('skl-plain', 'tea-a', 'deploy', 'How to deploy', E'Run it.\n', 1, 'admin');
		INSERT INTO agent_skills (id, workspace_id, name, description, triggers, body, extra, version, updated_by)
			VALUES ('skl-user', 'tea-a', 'release', 'Cut: a release', '{user}', E'Steps.\n', E'argument-hint: <tag>\n', 4, 'dev');`)
	for i, c := range backfillDescriptions {
		exec("description case", `INSERT INTO migration_0210_skill_versions.agent_skills (id, workspace_id, name, description, body, version)
			VALUES ($1, 'tea-a', 'd', $2, E'x\n', 1)`, fmt.Sprintf("skl-desc-%d", i), c.description)
	}
	exec("apply 0210", read("0210_agent_skill_versions.up.sql"))

	type row struct {
		version                              int
		content, message, createdBy, shortID string
	}
	get := func(id string) (r row) {
		t.Helper()
		if err := tx.QueryRow(ctx, `SELECT version, content, message, created_by, short_id FROM agent_skill_versions WHERE skill_id=$1`, id).
			Scan(&r.version, &r.content, &r.message, &r.createdBy, &r.shortID); err != nil {
			t.Fatalf("version row of %s: %v", id, err)
		}
		return r
	}
	plain := get("skl-plain")
	wantPlain := "---\nname: deploy\ndescription: How to deploy\n---\nRun it.\n"
	if plain.version != 1 || plain.content != wantPlain || plain.message != "Add skills/deploy/SKILL.md" || plain.createdBy != "admin" ||
		plain.shortID != AgentSkillVersionShortID("skl-plain", 1, wantPlain) {
		t.Fatalf("backfilled plain skill = %+v", plain)
	}
	user := get("skl-user")
	wantUser := "---\nname: release\ndescription: \"Cut: a release\"\ntriggers: [user]\nargument-hint: <tag>\n---\nSteps.\n"
	if user.version != 4 || user.content != wantUser || user.message != "Update skills/release/SKILL.md" || user.createdBy != "dev" ||
		user.shortID != AgentSkillVersionShortID("skl-user", 4, wantUser) {
		t.Fatalf("backfilled user-only skill = %+v", user)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM agent_skill_versions`).Scan(&n); err != nil || n != 2+len(backfillDescriptions) {
		t.Fatalf("%d version rows for %d skills, err=%v", n, 2+len(backfillDescriptions), err)
	}
	// The SQL render against the Go renderer's pinned output (see
	// TestRenderedDescriptionsMatchTheMigrationBackfill in agentsessions).
	for i, c := range backfillDescriptions {
		got := get(fmt.Sprintf("skl-desc-%d", i)).content
		want := "---\nname: d\ndescription: " + c.sql + "\n---\nx\n"
		if got != want {
			t.Errorf("backfilled description %q = %q, want %q", c.description, got, want)
		}
	}

	exec("roll back 0210", read("0210_agent_skill_versions.down.sql"))
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('migration_0210_skill_versions.agent_skill_versions') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("agent_skill_versions survived the down migration (err=%v)", err)
	}
}

// saveSkillN saves the skill count times, each against the version it read.
func saveSkillN(t *testing.T, s *PGStore, cur AgentSkill, count int, lim AgentSkillLimits) AgentSkill {
	t.Helper()
	for i := 0; i < count; i++ {
		var err error
		cur, err = s.UpdateAgentSkill(context.Background(), versioned(cur, fmt.Sprintf("c%d\n", cur.Version+1), "Update"), cur.Version, lim)
		if err != nil {
			t.Fatal(err)
		}
	}
	return cur
}

// The retention keeps exactly the newest 200: at 200 versions nothing is
// dropped, the 201st save drops version 1 and only version 1.
func TestPGAgentSkillVersionRetentionBoundary(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-skill-version-boundary")
	ctx := context.Background()
	lim := AgentSkillLimits{Count: 10, EnabledBytes: 1 << 20}
	cur, err := s.CreateAgentSkill(ctx, versioned(newSkill(tenant.ID, "deploy"), "c1\n", "Add"), lim)
	if err != nil {
		t.Fatal(err)
	}
	cur = saveSkillN(t, s, cur, AgentSkillVersionRetention-1, lim)
	count := func() (n, oldest int) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT count(*), min(version) FROM agent_skill_versions WHERE skill_id=$1`, cur.ID).Scan(&n, &oldest); err != nil {
			t.Fatal(err)
		}
		return n, oldest
	}
	if n, oldest := count(); cur.Version != 200 || n != 200 || oldest != 1 {
		t.Fatalf("at version %d: %d kept from %d, want 200 from 1", cur.Version, n, oldest)
	}
	cur = saveSkillN(t, s, cur, 1, lim)
	if n, oldest := count(); cur.Version != 201 || n != 200 || oldest != 2 {
		t.Fatalf("at version %d: %d kept from %d, want 200 from 2", cur.Version, n, oldest)
	}
	if _, err := s.GetAgentSkillVersion(ctx, cur.ID, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("version 1 after the 201st save = %v", err)
	}
	if _, err := s.GetAgentSkillVersion(ctx, cur.ID, 2); err != nil {
		t.Fatalf("version 2 after the 201st save = %v", err)
	}
}

// Two saves racing on the same ifVersion: one wins, the other is a stale
// version, and exactly one version row is written for the contested number.
func TestPGAgentSkillConcurrentSavesWriteOneVersion(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-skill-version-race")
	ctx := context.Background()
	lim := AgentSkillLimits{Count: 10, EnabledBytes: 1 << 20}
	created, err := s.CreateAgentSkill(ctx, versioned(newSkill(tenant.ID, "deploy"), "c1\n", "Add"), lim)
	if err != nil {
		t.Fatal(err)
	}
	const racers = 6
	errs := make([]error, racers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			next := versioned(created, fmt.Sprintf("racer %d\n", i), "Update")
			next.Body = fmt.Sprintf("racer %d", i)
			_, errs[i] = s.UpdateAgentSkill(ctx, next, 1, lim)
		}()
	}
	close(start)
	wg.Wait()
	wins := 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case !errors.Is(err, ErrAgentSkillVersion):
			t.Errorf("a losing save failed with %v, want ErrAgentSkillVersion", err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d saves won, want 1 (%v)", wins, errs)
	}
	if got := listVersionNumbers(t, s, created.ID, 0, 10); fmt.Sprint(got) != "[2 1]" {
		t.Fatalf("versions after the race = %v, want [2 1]", got)
	}
	row, err := s.GetAgentSkill(ctx, created.ID)
	v2, verr := s.GetAgentSkillVersion(ctx, created.ID, 2)
	if err != nil || verr != nil || row.Version != 2 || v2.Content != row.Body+"\n" {
		t.Fatalf("row %+v err=%v, version 2 %+v err=%v: the stored version is not the winner's", row, err, v2, verr)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_skill_versions WHERE skill_id=$1`, created.ID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("%d version rows, err=%v", n, err)
	}
}
