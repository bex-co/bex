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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEnvGroupConcurrentSameNameRenameAdmitsOne(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, _ := envGroupFixture(t, st)
	rows := make([]EnvGroup, 2)
	for i, name := range []string{"first", "second"} {
		var err error
		rows[i], err = st.InsertEnvGroup(ctx, newEnvGroupRow(tenant, name))
		if err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	type outcome struct {
		index int
		err   error
	}
	outcomes := make(chan outcome, len(rows))
	for i, row := range rows {
		go func() {
			<-start
			row.Name = "shared"
			_, err := st.UpdateEnvGroupMeta(ctx, row)
			outcomes <- outcome{index: i, err: err}
		}()
	}
	close(start)
	winners := 0
	for range rows {
		result := <-outcomes
		got, err := st.GetEnvGroup(ctx, rows[result.index].ID)
		if err != nil {
			t.Fatal(err)
		}
		if result.err == nil {
			winners++
			if got.Name != "shared" {
				t.Fatalf("winning rename left name %q", got.Name)
			}
		} else {
			if !errors.Is(result.err, ErrEnvGroupNameTaken) {
				t.Fatalf("losing rename = %v, want ErrEnvGroupNameTaken", result.err)
			}
			if got.Name != rows[result.index].Name || got.MetaVersion != rows[result.index].MetaVersion {
				t.Fatalf("losing rename changed its row: %+v", got)
			}
		}
	}
	if winners != 1 {
		t.Fatalf("%d renames won, want exactly one", winners)
	}
}

func TestEnvGroupContentLockRejectsSameGroupAndAllowsDifferentGroups(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	err := st.WithEnvGroupContentLock(ctx, "evg-first", func(lockedCtx context.Context) error {
		if err := st.WithEnvGroupContentLock(ctx, "evg-first", func(context.Context) error {
			t.Fatal("a second replica entered the same group's lock")
			return nil
		}); !errors.Is(err, ErrEnvGroupChanged) {
			t.Fatalf("same-group contender = %v, want ErrEnvGroupChanged", err)
		}
		if err := st.WithEnvGroupContentLock(ctx, "evg-second", func(context.Context) error { return nil }); err != nil {
			t.Fatalf("different-group contender = %v", err)
		}
		return st.WithEnvGroupContentLock(lockedCtx, "evg-first", func(context.Context) error { return nil })
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.WithEnvGroupContentLock(ctx, "evg-first", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("released lock was not reusable: %v", err)
	}
}

func TestEnvGroupContentLockUsesOneConnectionAndCommitsCrashEvidence(t *testing.T) {
	seed := newReplayTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config := seed.Pool.Config()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	tenant, _ := envGroupFixture(t, st)
	g, err := st.InsertEnvGroup(ctx, newEnvGroupRow(tenant, "crashed"))
	if err != nil {
		t.Fatal(err)
	}
	interrupted := errors.New("simulated process interruption")
	err = st.WithEnvGroupContentLock(ctx, g.ID, func(ctx context.Context) error {
		row, err := st.GetEnvGroup(ctx, g.ID)
		if err != nil {
			return err
		}
		_, err = st.ClaimEnvGroupContent(context.WithoutCancel(ctx), row.ID, row.Content.Version,
			EnvGroupOp{ID: "ego-crashed", Kind: "patch", Phase: "admitted", LeaseUntil: time.Now().Add(time.Minute)})
		if err != nil {
			return err
		}
		return interrupted
	})
	if !errors.Is(err, interrupted) {
		t.Fatalf("locked callback = %v, want simulated interruption", err)
	}
	row, err := st.GetEnvGroup(ctx, g.ID)
	if err != nil || row.Content.State != EnvGroupBusy || row.Content.Op == nil || row.Content.Op.ID != "ego-crashed" {
		t.Fatalf("callback failure rolled back recovery evidence: %+v, %v", row.Content, err)
	}
}

func TestEnvGroupContentLockDropsASessionAfterConnectionLoss(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	err := st.WithEnvGroupContentLock(ctx, "evg-lost-session", func(ctx context.Context) error {
		var pid int
		if err := st.envGroupQuery(ctx).QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			return err
		}
		_, err := st.Pool.Exec(ctx, `SELECT pg_terminate_backend($1)`, pid)
		return err
	})
	if err == nil {
		t.Fatal("lost advisory-lock session reported success")
	}
	if err := st.WithEnvGroupContentLock(ctx, "evg-lost-session", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("a failed unlock left the group locked or a broken pooled connection: %v", err)
	}
}

func TestEnvGroupCleanupEvidenceSurvivesCommitWithoutRecommittingGeneration(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, _ := envGroupFixture(t, st)
	g, err := st.InsertEnvGroup(ctx, newEnvGroupRow(tenant, "cleanup"))
	if err != nil {
		t.Fatal(err)
	}
	op := EnvGroupOp{ID: "ego-cleanup", Kind: "patch", Phase: "files_written", LeaseUntil: time.Now().Add(time.Minute), EnvChanged: true}
	claim, err := st.ClaimEnvGroupContent(ctx, g.ID, g.Content.Version, op)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReleaseEnvGroupContent(ctx, g.ID, claim, EnvGroupRelease{
		RetainOperation: true, Commit: &EnvGroupCommit{EnvKeys: []string{"COMMITTED"}, UpdatedAt: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	idle, err := st.GetEnvGroup(ctx, g.ID)
	if err != nil || idle.Content.State != EnvGroupIdle || idle.Content.Generation != 2 || idle.Content.Op == nil || idle.Content.Op.ID != op.ID {
		t.Fatalf("committed cleanup evidence = %+v, %v", idle.Content, err)
	}
	if _, err := st.ClaimEnvGroupContent(ctx, g.ID, idle.Content.Version, op); !errors.Is(err, ErrEnvGroupChanged) {
		t.Fatalf("claim replaced uncleared operation = %v, want ErrEnvGroupChanged", err)
	}
	if err := st.ReleaseEnvGroupContent(ctx, g.ID, idle.Content.Version, EnvGroupRelease{}); err != nil {
		t.Fatal(err)
	}
	clean, err := st.GetEnvGroup(ctx, g.ID)
	if err != nil || clean.Content.Op != nil || clean.Content.Generation != 2 || len(clean.EnvKeys) != 1 || clean.EnvKeys[0] != "COMMITTED" {
		t.Fatalf("cleanup lost committed content or recommitted generation: %+v, %v", clean, err)
	}
}

func TestEnvGroupDeletedRowRejectsEveryDelayedContentWrite(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, _ := envGroupFixture(t, st)
	g, err := st.InsertEnvGroup(ctx, newEnvGroupRow(tenant, "deleted"))
	if err != nil {
		t.Fatal(err)
	}
	op := EnvGroupOp{ID: "ego-delayed", Kind: "patch", Phase: "admitted", LeaseUntil: time.Now().Add(time.Minute)}
	claim, err := st.ClaimEnvGroupContent(ctx, g.ID, g.Content.Version, op)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DeleteEnvGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	for name, write := range map[string]func() error{
		"metadata": func() error { _, err := st.UpdateEnvGroupMeta(ctx, g); return err },
		"claim": func() error {
			_, err := st.ClaimEnvGroupContent(ctx, g.ID, g.Content.Version, op)
			return err
		},
		"advance": func() error { return st.AdvanceEnvGroupOp(ctx, g.ID, op.ID, "env_written", op.LeaseUntil) },
		"release": func() error {
			return st.ReleaseEnvGroupContent(ctx, g.ID, claim, EnvGroupRelease{Commit: &EnvGroupCommit{EnvKeys: []string{"ZOMBIE"}, UpdatedAt: time.Now()}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := write(); !errors.Is(err, ErrNotFound) {
				t.Fatalf("delayed write = %v, want ErrNotFound", err)
			}
			if _, err := st.GetEnvGroup(ctx, g.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("deleted group after write = %v, want ErrNotFound", err)
			}
		})
	}
}
