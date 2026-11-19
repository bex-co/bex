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

package apps

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w10/m490: a GitHub App-signed push follows a renamed or transferred
// repository by GitHub's repository id. `tianpan-v4-web` stored
// puncsky/tianpan-v3 and missed every push after the repository became
// puncsky/tpkb, because the webhook matched by URL alone.

const (
	renamedFrom  = "https://github.com/octo/old-name"
	renamedTo    = "https://github.com/octo/new-name"
	renamedID    = int64(42)
	renameSecret = "app-secret"
)

// repoIDRow is one apps row's repository half in repoIDStore.
type repoIDRow struct {
	repo string
	id   int64
}

// repoIDStore is recordingStore plus the apps rows' repository id, with the
// compare-and-set semantics of store.PGStore's github_repo_identity.go.
type repoIDStore struct {
	*recordingStore
	rows  map[string]repoIDRow
	gaps  []store.RepoIdentityGap
	moves int
}

func newRepoIDStore(rows map[string]repoIDRow) *repoIDStore {
	return &repoIDStore{recordingStore: &recordingStore{}, rows: rows}
}

func (r *repoIDStore) SetAppGitHubRepoID(_ context.Context, id, repo string, repoID int64) (bool, error) {
	row, ok := r.rows[id]
	if !ok || row.repo != repo || row.id == repoID {
		return false, nil
	}
	r.rows[id] = repoIDRow{repo: repo, id: repoID}
	return true, nil
}

func (r *repoIDStore) MoveAppRepo(_ context.Context, id string, repoID int64, from, to string) (bool, error) {
	row, ok := r.rows[id]
	if !ok || repoID == 0 || row.id != repoID || row.repo != from {
		return false, nil
	}
	r.rows[id] = repoIDRow{repo: to, id: repoID}
	r.moves++
	return true, nil
}

func (r *repoIDStore) SetBlueprintGitHubRepoID(context.Context, string, string, int64) (bool, error) {
	return false, nil
}

func (r *repoIDStore) ListRepoIdentityGaps(context.Context) ([]store.RepoIdentityGap, error) {
	return r.gaps, nil
}

// repoIDBlueprints is fakeBlueprintStore plus the Blueprint rows' repository
// id, with the same compare-and-set semantics.
type repoIDBlueprints struct{ *fakeBlueprintStore }

func (f repoIDBlueprints) SetBlueprintGitHubRepoID(_ context.Context, id, repo string, repoID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.blueprints[id]
	if !ok || b.Repo != repo || b.GitHubRepoID == repoID {
		return false, nil
	}
	b.GitHubRepoID = repoID
	f.blueprints[id] = b
	return true, nil
}

func (f repoIDBlueprints) MoveBlueprintRepo(_ context.Context, id string, repoID int64, from, to string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.blueprints[id]
	if !ok || repoID == 0 || b.GitHubRepoID != repoID || b.Repo != from {
		return false, nil
	}
	b.Repo = to
	f.blueprints[id] = b
	return true, nil
}

func (f repoIDBlueprints) blueprint(id string) store.Blueprint {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.blueprints[id]
}

// renamedRepoApp is a store-managed service in tenant on repo, carrying
// repoID as its GitHub repository id (0: none).
func renamedRepoApp(name, appID, tenant, repo string, repoID int64) *appv1alpha1.App {
	a := autoDeployApp(name, repo)
	a.Labels = map[string]string{
		store.LabelManagedBy: store.ManagedByValue, core.LabelAppID: appID, core.LabelTenant: tenant,
	}
	core.SetGitHubRepoID(a, repoID)
	return a
}

// githubPush is a GitHub push delivery's body as GitHub sends it: the
// repository block names the repository's current URLs and its id, and the
// installation that delivered it.
func githubPush(t *testing.T, repoID int64, repoURL, after string, installation int64, changed ...string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"ref":   "refs/heads/main",
		"after": after,
		"repository": map[string]any{
			"id":        repoID,
			"full_name": repoURL[len("https://github.com/"):],
			"html_url":  repoURL,
			"clone_url": repoURL + ".git",
			"ssh_url":   "git@github.com:" + repoURL[len("https://github.com/"):] + ".git",
			"url":       repoURL,
		},
		"installation": map[string]any{"id": installation},
		"head_commit":  map[string]any{"id": after, "message": "update", "timestamp": "2026-11-18T03:51:46Z"},
		"commits":      []map[string]any{{"modified": changed}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

const (
	pushOne = "7dd47f81e8aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	pushTwo = "7dd47f81e8bbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// TestAppSignedPushFollowsARenamedRepository is the milestone's definition of
// done 1–2 at the webhook: a push whose payload names owner/new-name reaches
// the service and the Blueprint that store owner/old-name with the same id,
// deploys the service, syncs the Blueprint, and moves both stored URLs, each
// recorded once as the system's. Before w10/m490 the push matched nothing
// and answered 200.
func TestAppSignedPushFollowsARenamedRepository(t *testing.T) {
	app := renamedRepoApp("web", "srv-web", "tea-a", renamedFrom, renamedID)
	st := newRepoIDStore(map[string]repoIDRow{"srv-web": {repo: renamedFrom, id: renamedID}})
	svc, cl := newService(st, app)
	sink := &captureAuditSink{}
	svc.Audit = sink
	bps := repoIDBlueprints{newFakeBlueprintStore(store.Blueprint{
		ID: "blp-web", TenantID: "tea-a", Name: "web", Repo: renamedFrom, GitHubRepoID: renamedID,
		Branch: "main", Path: "render.yaml", AutoSync: true, Status: store.BlueprintStatusInSync,
	})}
	svc.Blueprints = bps
	h := &GitWebhook{Svc: svc, GitHubSecret: renameSecret, Replays: &fakeReplayGuard{},
		Installations: fakeInstallations{7: {"tea-a"}}}

	body := githubPush(t, renamedID, renamedTo, pushOne, 7, "render.yaml", "main.go")
	if rec := postPush(t, h, renameSecret, "push", body); rec.Code != http.StatusOK {
		t.Fatalf("push => %d %s", rec.Code, rec.Body)
	}

	got := getApp(t, cl, "web")
	if got.Spec.RestartedAt == "" {
		t.Fatal("the push to the renamed repository must deploy the service that stores its old name")
	}
	if got.Spec.Repo != renamedTo || st.rows["srv-web"].repo != renamedTo {
		t.Errorf("service repo = %q (row %q), want both %q", got.Spec.Repo, st.rows["srv-web"].repo, renamedTo)
	}
	// REST GET /v1/services/{id}, GraphQL service(id) and MCP get_service all
	// read the service through Get.
	if v, err := svc.Get(context.Background(), "web"); err != nil || v.Repo != renamedTo {
		t.Errorf("Get repo = %q, %v; want %q", v.Repo, err, renamedTo)
	}
	if got.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] == "" {
		t.Error("the moved repo is a source change: it must be marked pending until the deploy consumes it")
	}
	if b := bps.blueprint("blp-web"); b.Repo != renamedTo {
		t.Errorf("blueprint repo = %q, want %q", b.Repo, renamedTo)
	}
	if n := len(bps.autoSyncIntents); n != 1 {
		t.Errorf("blueprint auto-sync intents = %d, want 1", n)
	}
	moves := repoMoveRows(sink)
	if len(moves) != 2 {
		t.Fatalf("repository moves recorded = %d (%+v), want one for the service and one for the Blueprint", len(moves), moves)
	}
	for _, ev := range moves {
		if ev.RepoFrom != renamedFrom || ev.RepoTo != renamedTo || ev.Caller != core.RepoMoveCaller || ev.CallerMethod != "internal" {
			t.Errorf("move row = %+v, want the system's, from %s to %s", ev, renamedFrom, renamedTo)
		}
		if ev.Resource != core.WorkspaceObject("tea-a") {
			t.Errorf("move row resource = %q, want the service's own workspace", ev.Resource)
		}
	}

	// A later push from the new name matches by URL as well as id: nothing
	// moves again, and nothing is recorded twice.
	if rec := postPush(t, h, renameSecret, "push", githubPush(t, renamedID, renamedTo, pushTwo, 7, "main.go")); rec.Code != http.StatusOK {
		t.Fatalf("second push => %d %s", rec.Code, rec.Body)
	}
	if n := len(st.deployCalls); n != 2 {
		t.Errorf("deploy rows = %d, want one per push", n)
	}
	if n := len(repoMoveRows(sink)); n != 2 || st.moves != 1 {
		t.Errorf("after a second push: %d move rows, %d row moves; want 2 and 1", n, st.moves)
	}
}

func repoMoveRows(sink *captureAuditSink) []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range sink.events {
		if ev.RepoTo != "" {
			out = append(out, ev)
		}
	}
	return out
}

// TestRepositoryIDMatchingStaysInTheInstallationsWorkspace is codex #7 for id
// matching: a service in another workspace that carries the same repository
// id is not reached by a delivery from this installation, and neither is a
// service whose stored id names another repository, even at the pushed URL (a
// new repository created under an old name).
func TestRepositoryIDMatchingStaysInTheInstallationsWorkspace(t *testing.T) {
	mine := renamedRepoApp("web", "srv-web", "tea-a", renamedFrom, renamedID)
	theirs := renamedRepoApp("api", "srv-api", "tea-b", renamedFrom, renamedID)
	impostor := renamedRepoApp("old", "srv-old", "tea-a", renamedTo, 99)
	st := newRepoIDStore(map[string]repoIDRow{
		"srv-web": {repo: renamedFrom, id: renamedID},
		"srv-api": {repo: renamedFrom, id: renamedID},
		"srv-old": {repo: renamedTo, id: 99},
	})
	svc, cl := newService(st, mine, theirs, impostor)
	h := &GitWebhook{Svc: svc, GitHubSecret: renameSecret, Installations: fakeInstallations{7: {"tea-a"}}}

	if rec := postPush(t, h, renameSecret, "push", githubPush(t, renamedID, renamedTo, pushOne, 7, "main.go")); rec.Code != http.StatusOK {
		t.Fatalf("push => %d %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "web").Spec.RestartedAt == "" {
		t.Error("the installation's own workspace's service must deploy")
	}
	if a := getApp(t, cl, "api"); a.Spec.RestartedAt != "" || a.Spec.Repo != renamedFrom {
		t.Errorf("another workspace's service with the same id was reached: restartedAt %q, repo %q", a.Spec.RestartedAt, a.Spec.Repo)
	}
	if a := getApp(t, cl, "old"); a.Spec.RestartedAt != "" {
		t.Error("a service whose stored id names another repository must not deploy for a push at its URL")
	}
}

// TestManualWebhookPushMatchesByURLOnly: the manual key carries no installation
// and Gitea sends ids of its own, so a manual push never matches by id, and a
// service with an id still deploys for a manual push to its stored URL.
func TestManualWebhookPushMatchesByURLOnly(t *testing.T) {
	app := renamedRepoApp("web", "srv-web", "tea-a", renamedFrom, renamedID)
	st := newRepoIDStore(map[string]repoIDRow{"srv-web": {repo: renamedFrom, id: renamedID}})
	svc, cl := newService(st, app)
	h := &GitWebhook{Svc: svc, Secret: "manual"}

	if rec := postPush(t, h, "manual", "", githubPush(t, renamedID, renamedTo, pushOne, 0, "main.go")); rec.Code != http.StatusOK {
		t.Fatalf("manual push => %d %s", rec.Code, rec.Body)
	}
	if a := getApp(t, cl, "web"); a.Spec.RestartedAt != "" || a.Spec.Repo != renamedFrom {
		t.Fatalf("a manual push named by id reached the service: restartedAt %q, repo %q", a.Spec.RestartedAt, a.Spec.Repo)
	}
	if rec := postPush(t, h, "manual", "", githubPush(t, 7777, renamedFrom, pushTwo, 0, "main.go")); rec.Code != http.StatusOK {
		t.Fatalf("manual push => %d %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "web").Spec.RestartedAt == "" {
		t.Error("a manual push to the stored URL must deploy, whatever id the payload carries")
	}
	if st.rows["srv-web"].id != renamedID {
		t.Errorf("a manual push changed the stored id to %d", st.rows["srv-web"].id)
	}
}

// TestAppSignedPushTeachesAURLMatchedServiceItsRepositoryID: a service with no
// id yet matches by URL, and learns the id the push carries, so a later
// rename keeps it deploying. A manual push teaches nothing.
func TestAppSignedPushTeachesAURLMatchedServiceItsRepositoryID(t *testing.T) {
	app := renamedRepoApp("web", "srv-web", "tea-a", renamedFrom, 0)
	st := newRepoIDStore(map[string]repoIDRow{"srv-web": {repo: renamedFrom}})
	svc, cl := newService(st, app)
	h := &GitWebhook{Svc: svc, GitHubSecret: renameSecret, Secret: "manual", Installations: fakeInstallations{7: {"tea-a"}}}

	if rec := postPush(t, h, "manual", "", githubPush(t, 7777, renamedFrom, pushOne, 0, "main.go")); rec.Code != http.StatusOK {
		t.Fatalf("manual push => %d", rec.Code)
	}
	if st.rows["srv-web"].id != 0 {
		t.Fatalf("a manual push recorded id %d", st.rows["srv-web"].id)
	}
	if rec := postPush(t, h, renameSecret, "push", githubPush(t, renamedID, renamedFrom, pushTwo, 7, "main.go")); rec.Code != http.StatusOK {
		t.Fatalf("app push => %d", rec.Code)
	}
	if getApp(t, cl, "web").Spec.RestartedAt == "" {
		t.Fatal("a service with no id must still deploy for a push to its URL")
	}
	if got := st.rows["srv-web"]; got.id != renamedID || got.repo != renamedFrom {
		t.Errorf("row after the app push = %+v, want id %d at the unchanged URL", got, renamedID)
	}
}

// TestGitRepositoryTracks is the match rule's truth table.
func TestGitRepositoryTracks(t *testing.T) {
	pushed := gitRepository{ID: renamedID, CloneURL: renamedTo + ".git", HTMLURL: renamedTo}
	for _, tc := range []struct {
		name     string
		stored   string
		storedID int64
		byID     bool
		want     bool
	}{
		{"same id, old URL, signed", renamedFrom, renamedID, true, true},
		{"same id, same URL, signed", renamedTo, renamedID, true, true},
		{"other id at the pushed URL, signed", renamedTo, 99, true, false},
		{"no id, same URL, signed", renamedTo, 0, true, true},
		{"no id, old URL, signed", renamedFrom, 0, true, false},
		{"same id, old URL, manual", renamedFrom, renamedID, false, false},
		{"other id at the pushed URL, manual", renamedTo, 99, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pushed.tracks(tc.stored, tc.storedID, tc.byID); got != tc.want {
				t.Errorf("tracks(%q, %d, %v) = %v, want %v", tc.stored, tc.storedID, tc.byID, got, tc.want)
			}
		})
	}
	// A payload with no id (Gitea's push without one, an old capture)
	// matches by URL even when signed.
	if !(gitRepository{CloneURL: renamedFrom}).tracks(renamedFrom, renamedID, true) {
		t.Error("a payload with no id must fall back to the URL")
	}
}

// TestCreateStoresTheGrantedRepositoryID: a service created from a repository
// the workspace's installation grants carries its id on the row and the App
// from the start; one read anonymously carries none.
func TestCreateStoresTheGrantedRepositoryID(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   int64
	}{{"granted", renamedID}, {"public, not granted", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			st := &recordingStore{}
			svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
			svc.GitHub = &fakeCloneTokens{defaultBranch: "main", repoID: tc.id}
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
			if _, err := svc.create(ctx, CreateRequest{Name: "web", Repo: renamedFrom, Branch: "main"}); err != nil {
				t.Fatalf("create: %v", err)
			}
			if len(st.appCreates) != 1 || st.appCreates[0].GitHubRepoID != tc.id {
				t.Fatalf("row = %+v, want GitHubRepoID %d", st.appCreates, tc.id)
			}
			var list appv1alpha1.AppList
			if err := cl.List(context.Background(), &list); err != nil || len(list.Items) != 1 {
				t.Fatalf("list: %v (%d apps)", err, len(list.Items))
			}
			if got := core.GitHubRepoID(&list.Items[0]); got != tc.id {
				t.Errorf("App annotation id = %d, want %d", got, tc.id)
			}
		})
	}
}

// TestSourceChangeRecordsTheNewRepositoryID: Update Source to a granted
// repository records its id on the row and the App; to a public one clears it.
func TestSourceChangeRecordsTheNewRepositoryID(t *testing.T) {
	app := renamedRepoApp("web", "srv-web", "", renamedFrom, renamedID)
	app.Labels[core.LabelTenant] = ""
	delete(app.Labels, core.LabelTenant)
	st := newRepoIDStore(map[string]repoIDRow{})
	svc, cl := newService(st, app)
	tokens := &fakeCloneTokens{repoID: 77}
	svc.GitHub = tokens
	// The row as SetAppSource leaves it: the new repo, the old id forgotten.
	st.rows["srv-web"] = repoIDRow{repo: renamedTo}

	repo := renamedTo
	if _, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", sourcePatch{Repo: &repo}); err != nil {
		t.Fatalf("set source: %v", err)
	}
	if got := st.rows["srv-web"]; got.id != 77 {
		t.Errorf("row id = %d, want the new repository's 77", got.id)
	}
	if got := core.GitHubRepoID(getApp(t, cl, "web")); got != 77 {
		t.Errorf("App annotation id = %d, want 77", got)
	}

	tokens.repoID = 0
	public := "https://github.com/stranger/public"
	if _, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", sourcePatch{Repo: &public}); err != nil {
		t.Fatalf("set source: %v", err)
	}
	if got := core.GitHubRepoID(getApp(t, cl, "web")); got != 0 {
		t.Errorf("a public repository's App kept id %d", got)
	}
}

// TestGetReportsLostRepositoryAccess is t004's server half: a service bex's
// GitHub App connected reads repoAccessLost once the installation cannot read
// its repository, and stops as soon as it can again. A public-URL service that
// was never connected never reads it.
func TestGetReportsLostRepositoryAccess(t *testing.T) {
	connected := renamedRepoApp("web", "srv-web", "", renamedFrom, renamedID)
	delete(connected.Labels, core.LabelTenant)
	public := renamedRepoApp("pub", "srv-pub", "", "https://github.com/stranger/public", 0)
	delete(public.Labels, core.LabelTenant)
	tokens := &fakeCloneTokens{ok: false}
	svc, _ := ghService(tokens, connected, public)
	svc.ManualWebhookDisabled = true

	read := func(name string) AppView {
		t.Helper()
		svc.pushDelivery = nil // a fresh grant answer per read, past the memo
		v, err := svc.Get(context.Background(), name)
		if err != nil {
			t.Fatalf("get %s: %v", name, err)
		}
		return v
	}
	if v := read("web"); !v.RepoAccessLost || v.PushDeliveryMethod != PushDeliveryUnavailable {
		t.Errorf("connected, unreadable: repoAccessLost=%v method=%s, want true unavailable", v.RepoAccessLost, v.PushDeliveryMethod)
	}
	if v := read("pub"); v.RepoAccessLost {
		t.Error("a service never connected through the GitHub App must not read repoAccessLost")
	}
	tokens.ok = true
	if v := read("web"); v.RepoAccessLost {
		t.Error("repoAccessLost must clear once the installation reads the repository again")
	}
}

// TestRepoIdentityBackfillResolvesAStaleName is t001's backfill: a service
// that stores a renamed repository's old name gets the id GitHub answers for
// that name, so its next push matches; it stores the old name until that push
// moves it. A row no installation grants keeps no id.
func TestRepoIdentityBackfillResolvesAStaleName(t *testing.T) {
	st := newRepoIDStore(map[string]repoIDRow{
		"srv-web": {repo: renamedFrom},
		"srv-pub": {repo: "https://github.com/stranger/public"},
	})
	st.gaps = []store.RepoIdentityGap{
		{Kind: store.RepoIdentityService, ID: "srv-web", TenantID: "tea-a", Repo: renamedFrom},
		{Kind: store.RepoIdentityService, ID: "srv-pub", TenantID: "tea-b", Repo: "https://github.com/stranger/public"},
	}
	lookups := &backfillLookups{granted: map[string]github.Repo{
		"tea-a\n" + renamedFrom: {ID: renamedID, FullName: "octo/new-name", HTMLURL: renamedTo, CloneURL: renamedTo + ".git"},
	}}
	svc := &Service{Base: &core.Base{Namespace: "default"}, Store: st, GitHub: lookups}
	b := &RepoIdentityBackfill{Svc: svc}

	if err := b.pass(context.Background(), st); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if got := st.rows["srv-web"]; got.id != renamedID || got.repo != renamedFrom {
		t.Errorf("renamed row = %+v, want id %d at the stored old name", got, renamedID)
	}
	if got := st.rows["srv-pub"]; got.id != 0 {
		t.Errorf("ungranted row got id %d", got.id)
	}
	if lookups.reads != 2 {
		t.Errorf("lookups = %d, want one per gap", lookups.reads)
	}
}

// backfillLookups answers GrantedRepo from a (workspace, repo) table.
type backfillLookups struct {
	fakeCloneTokens
	granted map[string]github.Repo
	reads   int
}

func (b *backfillLookups) GrantedRepo(_ context.Context, workspaceID, repo string) (github.Repo, bool, error) {
	b.reads++
	r, ok := b.granted[workspaceID+"\n"+repo]
	return r, ok, nil
}

// TestSetGitHubRepoIDAnnotation pins the annotation's round trip and that an
// unchanged id reports no change (the projector writes only on a change).
func TestSetGitHubRepoIDAnnotation(t *testing.T) {
	a := &appv1alpha1.App{}
	if core.SetGitHubRepoID(a, 0) {
		t.Error("clearing an absent id must report no change")
	}
	if !core.SetGitHubRepoID(a, renamedID) || core.GitHubRepoID(a) != renamedID {
		t.Fatalf("set => %v", a.Annotations)
	}
	if core.SetGitHubRepoID(a, renamedID) {
		t.Error("setting the same id must report no change")
	}
	if a.Annotations[core.AnnotationGitHubRepoID] != strconv.FormatInt(renamedID, 10) {
		t.Errorf("annotation = %q", a.Annotations[core.AnnotationGitHubRepoID])
	}
	if !core.SetGitHubRepoID(a, 0) || core.GitHubRepoID(a) != 0 {
		t.Error("clearing must remove the id")
	}
	a.Annotations = map[string]string{core.AnnotationGitHubRepoID: "not-a-number"}
	if core.GitHubRepoID(a) != 0 {
		t.Error("an unreadable annotation is no id")
	}
}
