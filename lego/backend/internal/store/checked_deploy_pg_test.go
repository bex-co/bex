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
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func checkedDeployApp(t *testing.T, st *PGStore, name string) App {
	t.Helper()
	ctx := context.Background()
	tenant := lifecycleTenant(t, st, "checked-"+name)
	a, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: name, Repo: "https://github.com/acme/app", Branch: "main", Port: 8080, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListDeploys(ctx, a.ID, DeployFilter{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("initial deploys = %+v, err=%v", rows, err)
	}
	if won, err := st.CloseDeploy(ctx, rows[0].ID, DeployLive, "img:initial"); err != nil || !won {
		t.Fatalf("initial release: won=%v err=%v", won, err)
	}
	return a
}

func TestPGEnsureCheckedDeployConcurrentReceipts(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	a := checkedDeployApp(t, st, "concurrent")
	authorAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	commit := CommitInfo{Hash: strings.Repeat("a", 40), Message: "checked change", AuthorAt: &authorAt}
	type result struct {
		deploy Deploy
		err    error
	}
	const workers = 16
	results := make(chan result, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			<-start
			d, err := st.EnsureCheckedDeploy(ctx, a.ID, 2, commit)
			results <- result{d, err}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	var first Deploy
	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		if first.ID == "" {
			first = r.deploy
		}
		if r.deploy.ID != first.ID || r.deploy.Status != first.Status || r.deploy.Generation != 2 || r.deploy.Commit != commit.Hash || r.deploy.CommitAuthorAt == nil || !r.deploy.CommitAuthorAt.Equal(authorAt) {
			t.Errorf("same receipt returned different deploys: %+v / %+v", first, r.deploy)
		}
	}
	if first.Trigger != TriggerNewCommit || first.Commit != commit.Hash || first.CommitMessage != commit.Message || first.CommitAuthorAt == nil || !first.CommitAuthorAt.Equal(authorAt) || first.Image != "" || first.TriggeredBy != "" {
		t.Fatalf("checked deploy lost source provenance: %+v", first)
	}
	rows, err := st.ListDeploys(ctx, a.ID, DeployFilter{})
	if err != nil || len(rows) != 2 || rows[0].ID != first.ID {
		t.Fatalf("receipt retries created extra history: %+v err=%v", rows, err)
	}
}

func TestPGEnsureCheckedDeployKeepsTerminalReceipt(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	for _, status := range []string{DeployLive, DeployBuildFailed, DeployCanceled} {
		t.Run(status, func(t *testing.T) {
			a := checkedDeployApp(t, st, status)
			commit := CommitInfo{Hash: strings.Repeat("b", 40), Message: "original metadata"}
			d, err := st.EnsureCheckedDeploy(ctx, a.ID, 2, commit)
			if err != nil {
				t.Fatal(err)
			}
			if won, err := st.CloseDeploy(ctx, d.ID, status, "img:checked"); err != nil || !won {
				t.Fatalf("close: won=%v err=%v", won, err)
			}
			terminal, err := st.GetDeploy(ctx, a.ID, d.ID)
			if err != nil {
				t.Fatal(err)
			}
			commit.Message = "metadata from a later retry"
			again, err := st.EnsureCheckedDeploy(ctx, a.ID, 2, commit)
			if err != nil || !reflect.DeepEqual(again, terminal) {
				t.Fatalf("terminal receipt mutated: before=%+v after=%+v err=%v", terminal, again, err)
			}
			rows, err := st.ListDeploys(ctx, a.ID, DeployFilter{})
			if err != nil || len(rows) != 2 {
				t.Fatalf("terminal retry created history: %+v err=%v", rows, err)
			}
		})
	}
}

func TestPGEnsureCheckedDeployLateReceiptPreservesNewerRelease(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	a := checkedDeployApp(t, st, "late")
	newer, err := st.CreateDeploy(ctx, a.ID, TriggerNewCommit, "", 3, CommitInfo{Hash: strings.Repeat("c", 40)}, "")
	if err != nil {
		t.Fatal(err)
	}
	oldCommit := CommitInfo{Hash: strings.Repeat("b", 40)}
	old, err := st.EnsureCheckedDeploy(ctx, a.ID, 2, oldCommit)
	if err != nil || old.Status != DeployCanceled || old.CancelReason != supersededByReason(newer.ID) || old.FinishedAt == nil {
		t.Fatalf("late receipt = %+v err=%v, want canceled by %s", old, err, newer.ID)
	}
	again, err := st.EnsureCheckedDeploy(ctx, a.ID, 2, oldCommit)
	if err != nil || !reflect.DeepEqual(again, old) {
		t.Fatalf("late receipt retry = %+v err=%v, want %+v", again, err, old)
	}
	current, err := st.GetDeploy(ctx, a.ID, newer.ID)
	if err != nil || !reflect.DeepEqual(current, newer) {
		t.Fatalf("late receipt changed newer release: before=%+v after=%+v err=%v", newer, current, err)
	}
}

func TestPGEnsureCheckedDeployRefusesDifferentProvenance(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	commit := CommitInfo{Hash: strings.Repeat("d", 40)}
	for _, tc := range []struct {
		name, trigger, image, actor, hash string
	}{
		{name: "commit", trigger: TriggerNewCommit, hash: strings.Repeat("e", 40)},
		{name: "trigger", trigger: TriggerAPI, hash: commit.Hash},
		{name: "image", trigger: TriggerNewCommit, image: "img:override", hash: commit.Hash},
		{name: "actor", trigger: TriggerNewCommit, actor: "different-actor", hash: commit.Hash},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := checkedDeployApp(t, st, tc.name)
			existing, err := st.CreateDeploy(ctx, a.ID, tc.trigger, tc.image, 2, CommitInfo{Hash: tc.hash}, tc.actor)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.EnsureCheckedDeploy(ctx, a.ID, 2, commit); !errors.Is(err, ErrConflict) {
				t.Fatalf("mismatched %s = %v, want conflict", tc.name, err)
			}
			current, err := st.GetDeploy(ctx, a.ID, existing.ID)
			if err != nil || !reflect.DeepEqual(current, existing) {
				t.Fatalf("refused receipt changed existing deploy: %+v err=%v", current, err)
			}
			rows, err := st.ListDeploys(ctx, a.ID, DeployFilter{})
			if err != nil || len(rows) != 2 {
				t.Fatalf("refused receipt created history: %+v err=%v", rows, err)
			}
		})
	}
}

func TestPGClearCheckedDeployImageFencesSourceAndOverride(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	a := checkedDeployApp(t, st, "clear-image")
	const override = "img:rollback"
	if err := st.SetAppImage(ctx, a.ID, override); err != nil {
		t.Fatal(err)
	}
	before, err := st.GetApp(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	version, matched, err := st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, override)
	if err != nil || !matched || version < 0 {
		t.Fatalf("source snapshot: version=%d matched=%v err=%v", version, matched, err)
	}
	for _, tc := range []struct{ name, repo, branch, image string }{
		{name: "repository", repo: "https://github.com/other/app", branch: a.Branch, image: override},
		{name: "branch", repo: a.Repo, branch: "other", image: override},
		{name: "override", repo: a.Repo, branch: a.Branch, image: "img:older-rollback"},
		{name: "empty-image", repo: a.Repo, branch: a.Branch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, matched, err := st.CheckedDeploySourceVersion(ctx, a.ID, tc.repo, tc.branch, tc.image); err != nil || matched {
				t.Fatalf("stale source snapshot: matched=%v err=%v", matched, err)
			}
			matched, err := st.ClearCheckedDeployImage(ctx, a.ID, tc.repo, tc.branch, tc.image, version)
			if err != nil || matched {
				t.Fatalf("stale source matched=%v err=%v", matched, err)
			}
			current, err := st.GetApp(ctx, a.ID)
			if err != nil || !reflect.DeepEqual(current, before) {
				t.Fatalf("stale dispatch changed source: %+v err=%v", current, err)
			}
		})
	}
	matched, err = st.ClearCheckedDeployImage(ctx, a.ID, a.Repo, a.Branch, override, version)
	if err != nil || !matched {
		t.Fatalf("matching source: matched=%v err=%v", matched, err)
	}
	cleared, err := st.GetApp(ctx, a.ID)
	if err != nil || cleared.Image != "" {
		t.Fatalf("image not cleared: %+v err=%v", cleared, err)
	}
	version, matched, err = st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, "")
	if err != nil || !matched {
		t.Fatalf("empty source snapshot: matched=%v err=%v", matched, err)
	}
	var beforeClock time.Time
	if err := st.Pool.QueryRow(ctx, `SELECT updated_at FROM apps WHERE id = $1`, a.ID).Scan(&beforeClock); err != nil {
		t.Fatal(err)
	}
	matched, err = st.ClearCheckedDeployImage(ctx, a.ID, a.Repo, a.Branch, "", version)
	if err != nil || !matched {
		t.Fatalf("already empty: matched=%v err=%v", matched, err)
	}
	unchanged, err := st.GetApp(ctx, a.ID)
	if err != nil || !reflect.DeepEqual(unchanged, cleared) {
		t.Fatalf("empty-image check changed app: %+v err=%v", unchanged, err)
	}
	afterVersion, matched, err := st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, "")
	if err != nil || !matched || afterVersion != version {
		t.Fatalf("empty-image check changed source revision: before=%d after=%d matched=%v err=%v", version, afterVersion, matched, err)
	}
	var afterClock time.Time
	if err := st.Pool.QueryRow(ctx, `SELECT updated_at FROM apps WHERE id = $1`, a.ID).Scan(&afterClock); err != nil || !afterClock.Equal(beforeClock) {
		t.Fatalf("empty-image check changed update clock: before=%s after=%s err=%v", beforeClock, afterClock, err)
	}
	if _, matched, err := st.CheckedDeploySourceVersion(ctx, "srv-missing", a.Repo, a.Branch, ""); err != nil || matched {
		t.Fatalf("missing source snapshot: matched=%v err=%v", matched, err)
	}
	if matched, err := st.ClearCheckedDeployImage(ctx, "srv-missing", a.Repo, a.Branch, "", version); err != nil || matched {
		t.Fatalf("missing source: matched=%v err=%v", matched, err)
	}
	// A nullable repo compares as empty too. Keep image non-null to preserve
	// the apps source constraint; the earlier no-op covered a null image.
	if _, err := st.Pool.Exec(ctx, `UPDATE apps SET repo = NULL, image = '' WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	beforeNullCheck, err := st.GetApp(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	version, matched, err = st.CheckedDeploySourceVersion(ctx, a.ID, "", a.Branch, "")
	if err != nil || !matched {
		t.Fatalf("nullable source snapshot: matched=%v err=%v", matched, err)
	}
	matched, err = st.ClearCheckedDeployImage(ctx, a.ID, "", a.Branch, "", version)
	if err != nil || !matched {
		t.Fatalf("nullable source: matched=%v err=%v", matched, err)
	}
	afterNullCheck, err := st.GetApp(ctx, a.ID)
	if err != nil || !reflect.DeepEqual(afterNullCheck, beforeNullCheck) {
		t.Fatalf("nullable source check wrote app: %+v err=%v", afterNullCheck, err)
	}
}

func TestPGClearCheckedDeployImagePreservesRepeatedRollback(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	a := checkedDeployApp(t, st, "same-image-rollback")
	const override = "img:same-rollback"
	if err := st.SetAppImage(ctx, a.ID, override); err != nil {
		t.Fatal(err)
	}
	version, matched, err := st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, override)
	if err != nil || !matched {
		t.Fatalf("accepted source snapshot: matched=%v err=%v", matched, err)
	}
	// A newer rollback chooses exactly the same source tuple. Only the row
	// version distinguishes it from the override this dispatch was replacing.
	if err := st.SetAppImage(ctx, a.ID, override); err != nil {
		t.Fatal(err)
	}
	newerVersion, matched, err := st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, override)
	if err != nil || !matched || newerVersion == version {
		t.Fatalf("newer rollback snapshot: old=%d new=%d matched=%v err=%v", version, newerVersion, matched, err)
	}
	matched, err = st.ClearCheckedDeployImage(ctx, a.ID, a.Repo, a.Branch, override, version)
	if err != nil || matched {
		t.Fatalf("old accepted receipt cleared newer rollback: matched=%v err=%v", matched, err)
	}
	after, matched, err := st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, override)
	if err != nil || !matched || after != newerVersion {
		t.Fatalf("newer rollback changed: before=%d after=%d matched=%v err=%v", newerVersion, after, matched, err)
	}
}

func TestPGCheckedDeploySourceRevisionIgnoresUnrelatedSettings(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	a := checkedDeployApp(t, st, "source-revision")
	version, matched, err := st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, "")
	if err != nil || !matched || version != 0 {
		t.Fatalf("new source revision: version=%d matched=%v err=%v", version, matched, err)
	}
	const override = "img:rollback"
	if err := st.SetAppImage(ctx, a.ID, override); err != nil {
		t.Fatal(err)
	}
	version, matched, err = st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, override)
	if err != nil || !matched || version != 1 {
		t.Fatalf("rollback source revision: version=%d matched=%v err=%v", version, matched, err)
	}
	if err := st.SetAppReplicas(ctx, a.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppDisplayName(ctx, a.ID, "renamed while dispatching"); err != nil {
		t.Fatal(err)
	}
	afterSettings, matched, err := st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, override)
	if err != nil || !matched || afterSettings != version {
		t.Fatalf("unrelated settings superseded source: before=%d after=%d matched=%v err=%v", version, afterSettings, matched, err)
	}
	matched, err = st.ClearCheckedDeployImage(ctx, a.ID, a.Repo, a.Branch, override, version)
	if err != nil || !matched {
		t.Fatalf("unrelated settings prevented image cleanup: matched=%v err=%v", matched, err)
	}
	version, matched, err = st.CheckedDeploySourceVersion(ctx, a.ID, a.Repo, a.Branch, "")
	if err != nil || !matched || version != 2 {
		t.Fatalf("cleanup source revision: version=%d matched=%v err=%v", version, matched, err)
	}
	for _, source := range []struct{ repo, branch string }{
		{repo: a.Repo, branch: "next"},
		{repo: "https://github.com/acme/other", branch: "next"},
	} {
		if err := st.SetAppSource(ctx, a.ID, source.repo, "", source.branch, nil); err != nil {
			t.Fatal(err)
		}
		nextVersion, matched, err := st.CheckedDeploySourceVersion(ctx, a.ID, source.repo, source.branch, "")
		if err != nil || !matched || nextVersion != version+1 {
			t.Fatalf("source update revision: before=%d after=%d matched=%v err=%v", version, nextVersion, matched, err)
		}
		version = nextVersion
	}
	desired, err := st.ListDesiredApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range desired {
		if d.ID == a.ID {
			if d.SourceRevision != version || d.Replicas != 2 || d.Image != "" || d.DisplayName != "renamed while dispatching" {
				t.Fatalf("projection lost source revision or unrelated settings: %+v", d)
			}
			return
		}
	}
	t.Fatal("updated source missing from projection")
}
