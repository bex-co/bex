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

package deploys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- test harness -------------------------------------------------------------

func fakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// sampleApp is a store-managed App (carries the bex.co/app-id label the
// reconciler stamps) unless storeID is empty, in which case it's hand-applied
// — the case with no deploy history at all.
func sampleApp(name, storeID string) *appv1alpha1.App {
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Generation: 1},
		Spec:       appv1alpha1.AppSpec{Image: name + ":v1"},
	}
	if storeID != "" {
		a.Labels = map[string]string{store.LabelAppID: storeID}
	}
	return a
}

func fixedNow() time.Time { return time.Unix(1_700_000_000, 0).UTC() }

func newService(ds DeployStore, objs ...client.Object) (*Service, client.Client) {
	cl := fakeClient(objs...)
	return &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: ds}, cl
}

type startedCall struct{ tenantID, appName string }

type blockingStartedNotifier struct {
	calls   chan startedCall
	release chan struct{}
}

func newBlockingStartedNotifier() *blockingStartedNotifier {
	return &blockingStartedNotifier{calls: make(chan startedCall, 1), release: make(chan struct{})}
}

func (n *blockingStartedNotifier) NotifyDeployStarted(_ context.Context, tenantID, appName, _ string) {
	n.calls <- startedCall{tenantID: tenantID, appName: appName}
	<-n.release
}

// fakeStore is an in-memory DeployStore, newest-first like PGStore/memStore.
type fakeStore struct {
	mu       sync.Mutex
	byApp    map[string][]store.Deploy
	nextID   int
	setImage map[string]string // appID -> last SetAppImage call, for assertions
}

func newFakeStore() *fakeStore { return &fakeStore{byApp: map[string][]store.Deploy{}} }

func (f *fakeStore) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo) (store.Deploy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	status := store.DeployCreated
	for _, existing := range f.byApp[appID] {
		if store.IsOpenDeployStatus(existing.Status) && existing.Generation >= generation {
			status = store.DeployCanceled
			break
		}
	}
	if status == store.DeployCreated {
		for i, existing := range f.byApp[appID] {
			if !store.IsOpenDeployStatus(existing.Status) {
				continue
			}
			existing.Status = store.DeployCanceled
			existing.UpdatedAt = now
			existing.FinishedAt = &now
			f.byApp[appID][i] = existing
		}
	}
	f.nextID++
	d := store.Deploy{ID: fmt.Sprintf("dep-%d", f.nextID), AppID: appID, Trigger: trigger, Image: image, Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message, Status: status, CreatedAt: now, UpdatedAt: now}
	if status == store.DeployCanceled {
		d.FinishedAt = &now
	}
	f.byApp[appID] = append([]store.Deploy{d}, f.byApp[appID]...)
	return d, nil
}

func (f *fakeStore) CreateRollbackDeploy(_ context.Context, appID, image, rollbackOf string, generation int64, commit store.CommitInfo) (store.Deploy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	status := store.DeployCreated
	for _, existing := range f.byApp[appID] {
		if store.IsOpenDeployStatus(existing.Status) && existing.Generation >= generation {
			status = store.DeployCanceled
			break
		}
	}
	if status == store.DeployCreated {
		for i, existing := range f.byApp[appID] {
			if !store.IsOpenDeployStatus(existing.Status) {
				continue
			}
			existing.Status = store.DeployCanceled
			existing.UpdatedAt = now
			existing.FinishedAt = &now
			f.byApp[appID][i] = existing
		}
	}
	f.nextID++
	d := store.Deploy{
		ID: fmt.Sprintf("dep-%d", f.nextID), AppID: appID, Trigger: "rollback", Image: image, ResolvedImage: image,
		RollbackOf: rollbackOf, Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message, Status: status, CreatedAt: now, UpdatedAt: now,
	}
	if status == store.DeployCanceled {
		d.FinishedAt = &now
	}
	f.byApp[appID] = append([]store.Deploy{d}, f.byApp[appID]...)
	return d, nil
}

// ListDeploys mirrors PGStore's filter semantics (w2/m31) — status set,
// exclusive created_at bounds, keyset cursor off the cursor row's own
// (CreatedAt, ID), clamped limit — so adapter tests exercise the same paging
// behavior the real store has. f.byApp is already newest-first.
func (f *fakeStore) ListDeploys(_ context.Context, appID string, filter store.DeployFilter) ([]store.Deploy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var cursorRow *store.Deploy
	for _, d := range f.byApp[appID] {
		if d.ID == filter.Cursor {
			cursorRow = &d
			break
		}
	}
	var out []store.Deploy
	for _, d := range f.byApp[appID] {
		if len(filter.Statuses) > 0 && !slices.Contains(filter.Statuses, d.Status) {
			continue
		}
		if !filter.CreatedAfter.IsZero() && !d.CreatedAt.After(filter.CreatedAfter) {
			continue
		}
		if !filter.CreatedBefore.IsZero() && !d.CreatedAt.Before(filter.CreatedBefore) {
			continue
		}
		if !filter.UpdatedAfter.IsZero() && !d.UpdatedAt.After(filter.UpdatedAfter) {
			continue
		}
		if !filter.UpdatedBefore.IsZero() && !d.UpdatedAt.Before(filter.UpdatedBefore) {
			continue
		}
		if !filter.FinishedAfter.IsZero() && (d.FinishedAt == nil || !d.FinishedAt.After(filter.FinishedAfter)) {
			continue
		}
		if !filter.FinishedBefore.IsZero() && (d.FinishedAt == nil || !d.FinishedAt.Before(filter.FinishedBefore)) {
			continue
		}
		if filter.Cursor != "" && (cursorRow == nil || d.CreatedAt.After(cursorRow.CreatedAt) ||
			(d.CreatedAt.Equal(cursorRow.CreatedAt) && d.ID >= cursorRow.ID)) {
			continue
		}
		out = append(out, d)
	}
	if limit := min(filter.Limit, core.MaxPageLimit); limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) GetDeploy(_ context.Context, appID, deployID string) (store.Deploy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.byApp[appID] {
		if d.ID == deployID {
			return d, nil
		}
	}
	return store.Deploy{}, store.ErrNotFound
}

// CloseDeploy mirrors store.PGStore's CAS guard (WHERE finished_at IS NULL)
// so the fake exercises the same race-safety Cancel and the reconciler's
// write-back both rely on.
func (f *fakeStore) CloseDeploy(_ context.Context, id, status, resolvedImage string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for appID, deploys := range f.byApp {
		for i, d := range deploys {
			if d.ID != id {
				continue
			}
			if !store.IsTerminalDeployStatus(status) || status == store.DeployDeactivated || !store.CanTransitionDeploy(d.Status, status) {
				return false, nil
			}
			now := time.Now()
			d.Status = status
			d.UpdatedAt = now
			if resolvedImage != "" {
				d.ResolvedImage = resolvedImage
			}
			if status != store.DeployCanceled && d.StartedAt == nil {
				d.StartedAt = &now
			}
			d.FinishedAt = &now
			f.byApp[appID][i] = d
			if status == store.DeployLive {
				for j, other := range f.byApp[appID] {
					if j != i && other.Status == store.DeployLive {
						other.Status = store.DeployDeactivated
						other.UpdatedAt = now
						f.byApp[appID][j] = other
					}
				}
			}
			return true, nil
		}
	}
	return false, nil
}

// SetAppImage records the image write-through for assertion — the fake has
// no backing apps table, so it just remembers the last call per app id.
func (f *fakeStore) SetAppImage(_ context.Context, id string, image string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setImage == nil {
		f.setImage = map[string]string{}
	}
	f.setImage[id] = image
	return nil
}

// --- List / Get -----------------------------------------------------------

func TestListEmptyForHandAppliedApp(t *testing.T) {
	ds := newFakeStore()
	ds.byApp["srv-other"] = []store.Deploy{{ID: "dep-1", AppID: "srv-other", Status: store.DeployLive}}
	svc, _ := newService(ds, sampleApp("manual", ""))

	got, err := svc.List(context.Background(), "manual", ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("hand-applied App must have empty history, got %+v", got)
	}
}

// TestDeployRecordSurfacesPreDeployStatus covers w1/m33: a deploy that failed
// its migration carries pre_deploy_status "failed" through DeployView and onto
// the REST JSON — the field that tells it apart from a health-check failure.
func TestDeployRecordSurfacesPreDeployStatus(t *testing.T) {
	ds := newFakeStore()
	ds.byApp["srv-1"] = []store.Deploy{{
		ID: "dep-1", AppID: "srv-1", Status: store.DeployUpdateFailed,
		PreDeployStatus: store.PreDeployFailed,
	}}
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	got, err := svc.List(context.Background(), "web", ListFilter{})
	if err != nil || len(got) != 1 {
		t.Fatalf("List = %+v (err %v), want one deploy", got, err)
	}
	if got[0].PreDeployStatus != store.PreDeployFailed {
		t.Errorf("DeployView.PreDeployStatus = %q, want failed", got[0].PreDeployStatus)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/deploys", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list deploys => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out []struct {
		Deploy struct {
			Status          string `json:"status"`
			PreDeployStatus string `json:"preDeployStatus"`
		} `json:"deploy"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out) != 1 {
		t.Fatalf("decode = %v, body %s", err, rec.Body)
	}
	if out[0].Deploy.Status != store.DeployUpdateFailed || out[0].Deploy.PreDeployStatus != store.PreDeployFailed {
		t.Errorf("REST deploy = %+v, want update_failed with preDeployStatus failed", out[0].Deploy)
	}
}

func TestListGetTriggerLifecycle(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	svc, cl := newService(ds, sampleApp("web", "srv-1"))

	list, err := svc.List(context.Background(), "web", ListFilter{})
	if err != nil || len(list) != 1 || list[0].ID != first.ID || list[0].Trigger != "create" {
		t.Fatalf("List = %+v (err %v)", list, err)
	}

	triggered, err := svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if triggered.Trigger != "api" || triggered.Status != store.DeployCreated {
		t.Errorf("triggered deploy = %+v", triggered)
	}
	app := getApp(t, cl, "web")
	if app.Spec.RestartedAt == "" {
		t.Error("Trigger must bump spec.restartedAt (a re-pull/restart now)")
	}
	if got := app.Annotations[appv1alpha1.AnnotationReleaseGeneration]; got != "2" {
		t.Errorf("release-generation annotation = %q, want 2", got)
	}
	if got := ds.byApp["srv-1"][0].Generation; got != 2 {
		t.Errorf("deploy generation = %d, want annotated release generation 2", got)
	}

	list, err = svc.List(context.Background(), "web", ListFilter{})
	if err != nil || len(list) != 2 || list[0].ID != triggered.ID {
		t.Fatalf("List after trigger (want newest first) = %+v (err %v)", list, err)
	}

	got, err := svc.Get(context.Background(), "web", triggered.ID)
	if err != nil || got.ID != triggered.ID {
		t.Fatalf("Get: %+v (err %v)", got, err)
	}
	if _, err := svc.Get(context.Background(), "web", "dep-doesnotexist"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown deploy: want core.ErrNotFound, got %v", err)
	}
}

func TestTriggerNotifiesDeployStartedOffRequestPath(t *testing.T) {
	ds := newFakeStore()
	a := sampleApp("web", "srv-1")
	a.Labels[core.LabelTenant] = "tea-a"
	svc, _ := newService(ds, a)
	notifier := newBlockingStartedNotifier()
	defer close(notifier.release)
	svc.StartedNotifier = notifier

	result := make(chan error, 1)
	go func() {
		_, err := svc.Trigger(context.Background(), "web", TriggerParams{})
		result <- err
	}()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Trigger: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Trigger blocked on deploy-start notification delivery")
	}
	select {
	case got := <-notifier.calls:
		if got != (startedCall{tenantID: "tea-a", appName: "web"}) {
			t.Errorf("started notification = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("Trigger returned without issuing a deploy-start notification")
	}
}

func getApp(t *testing.T, cl client.Client, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &a); err != nil {
		t.Fatalf("get app %s: %v", name, err)
	}
	return &a
}

// --- Trigger refusals -------------------------------------------------------

func TestTriggerRefusesSuspendedService(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("web", "srv-1")
	app.Spec.Suspended = true
	svc, _ := newService(ds, app)

	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); !errors.Is(err, core.ErrConflict) {
		t.Errorf("suspended trigger: want core.ErrConflict, got %v", err)
	}
}

func TestTriggerRequiresStoreManagedApp(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("manual", ""))

	if _, err := svc.Trigger(context.Background(), "manual", TriggerParams{}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("hand-applied trigger: want core.ErrBadRequest, got %v", err)
	}
}

// --- commitId checkout override (t009) ----------------------------------------

func TestTriggerSetsCommitIDInSpec(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, sampleApp("web", "srv-1"))

	_, err := svc.Trigger(context.Background(), "web", TriggerParams{CommitID: "abc123"})
	if err != nil {
		t.Fatalf("Trigger with commitId: %v", err)
	}
	app := getApp(t, cl, "web")
	if app.Spec.BuildCommit != "abc123" {
		t.Errorf("spec.buildCommit = %q, want %q", app.Spec.BuildCommit, "abc123")
	}
}

func TestTriggerResetsCommitIDWhenOmitted(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("web", "srv-1")
	app.Spec.BuildCommit = "stale-commit" // was set by a previous pinned deploy
	svc, cl := newService(ds, app)

	_, err := svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil {
		t.Fatalf("Trigger without commitId: %v", err)
	}
	got := getApp(t, cl, "web")
	if got.Spec.BuildCommit != "" {
		t.Errorf("spec.buildCommit should be reset to empty on a non-commitId trigger; got %q", got.Spec.BuildCommit)
	}
}

func TestTriggerAfterRepoRollbackClearsImageOverride(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("web", "srv-1")
	app.Spec.Repo = "https://github.com/bex-co/web.git"
	app.Spec.Image = "zot.test/web:gen-4" // exact-image override left by Rollback
	svc, cl := newService(ds, app)

	deploy, err := svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil {
		t.Fatalf("Trigger after rollback: %v", err)
	}
	if got := ds.setImage["srv-1"]; got != "" {
		t.Errorf("stored image override = %q, want cleared", got)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "" {
		t.Errorf("spec.image after source trigger = %q, want cleared", got)
	}
	if deploy.Image != "" {
		t.Errorf("source deploy row image = %q, want empty until the build resolves", deploy.Image)
	}
}

func TestTriggerRejectsCommitIDForCronJob(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("cron", "srv-2")
	app.Spec.Type = appv1alpha1.TypeCronJob
	svc, _ := newService(ds, app)

	_, err := svc.Trigger(context.Background(), "cron", TriggerParams{CommitID: "abc123"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("commitId for cron_job: want core.ErrBadRequest, got %v", err)
	}
}

// --- deployMode reject/accept paths (t009) ------------------------------------

func TestTriggerDeployOnlyRejectsRepoBacked(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("svc", "srv-3")
	app.Spec.Image = "" // repo-backed: no prebuilt image
	app.Spec.Repo = "https://github.com/bex-co/hello.git"
	svc, _ := newService(ds, app)

	_, err := svc.Trigger(context.Background(), "svc", TriggerParams{DeployMode: "deploy_only"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("deploy_only for repo-backed: want core.ErrBadRequest, got %v", err)
	}
}

func TestTriggerDeployOnlyAcceptsImageBacked(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("svc", "srv-4"))
	// sampleApp uses Image: "svc:v1" (image-backed), so deploy_only is fine.

	_, err := svc.Trigger(context.Background(), "svc", TriggerParams{DeployMode: "deploy_only"})
	if err != nil {
		t.Errorf("deploy_only for image-backed: want nil, got %v", err)
	}
}

// --- imageUrl accept/reject paths (w2/m44) ------------------------------------

func TestTriggerImageURLRejectsRepoBacked(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("svc", "srv-10")
	app.Spec.Image = ""
	app.Spec.Repo = "https://github.com/bex-co/hello.git"
	svc, _ := newService(ds, app)

	_, err := svc.Trigger(context.Background(), "svc", TriggerParams{ImageURL: "nginx:1.27"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("imageUrl for repo-backed: want core.ErrBadRequest, got %v", err)
	}
}

func TestTriggerImageURLAcceptsImageBacked(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("svc", "srv-11")
	svc, cl := newService(ds, app)

	_, err := svc.Trigger(context.Background(), "svc", TriggerParams{ImageURL: "nginx:1.27"})
	if err != nil {
		t.Fatalf("imageUrl for image-backed: want nil, got %v", err)
	}
	// The App spec must carry the new image so the operator pulls the override.
	if got := getApp(t, cl, "svc").Spec.Image; got != "nginx:1.27" {
		t.Errorf("spec.image after imageUrl trigger = %q, want nginx:1.27", got)
	}
}

// --- Restart opens a deploy row (t009) -----------------------------------------

func TestRestartOpensDeploy(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, sampleApp("web", "srv-5"))

	d, err := svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil {
		t.Fatalf("Trigger (restart): %v", err)
	}
	if d.Trigger != "api" || d.Status != store.DeployCreated {
		t.Errorf("Restart deploy = %+v, want trigger=api, status=created", d)
	}
	// RestartedAt must be bumped so the operator rolls the pods.
	app := getApp(t, cl, "web")
	if app.Spec.RestartedAt == "" {
		t.Error("Restart must bump spec.restartedAt")
	}
	// BuildCommit must remain empty (Restart is not a commitId-pinned deploy).
	if app.Spec.BuildCommit != "" {
		t.Errorf("Restart must not set spec.buildCommit; got %q", app.Spec.BuildCommit)
	}
}

// --- Store-off 503 ----------------------------------------------------------

func TestVerbsUnavailableWithoutStore(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web", "srv-1"))
	ctx := context.Background()

	if _, err := svc.List(ctx, "web", ListFilter{}); !errors.Is(err, core.ErrDeploysUnavailable) {
		t.Errorf("List: want ErrDeploysUnavailable, got %v", err)
	}
	if _, err := svc.Get(ctx, "web", "dep-1"); !errors.Is(err, core.ErrDeploysUnavailable) {
		t.Errorf("Get: want ErrDeploysUnavailable, got %v", err)
	}
	if _, err := svc.Trigger(ctx, "web", TriggerParams{}); !errors.Is(err, core.ErrDeploysUnavailable) {
		t.Errorf("Trigger: want ErrDeploysUnavailable, got %v", err)
	}
}

// --- REST fragment ------------------------------------------------------------

func TestRESTListGetTrigger(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	do := newRESTHarness(t, svc)

	rec := do("GET", "/v1/services/web/deploys")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: code=%d body=%s", rec.Code, rec.Body)
	}
	if list := decodeList(t, rec); len(list) != 1 || list[0].Deploy.ID != first.ID {
		t.Fatalf("list envelope = %s", rec.Body)
	}

	rec = do("POST", "/v1/services/web/deploys")
	if rec.Code != http.StatusCreated {
		t.Fatalf("trigger: code=%d body=%s", rec.Code, rec.Body)
	}
	var created renderDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Trigger != "api" {
		t.Fatalf("trigger body = %s (err %v)", rec.Body, err)
	}

	rec = do("GET", "/v1/services/web/deploys/"+created.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: code=%d body=%s", rec.Code, rec.Body)
	}
	rec = do("GET", "/v1/services/web/deploys/dep-nope")
	if rec.Code != http.StatusNotFound {
		t.Errorf("get unknown: code=%d, want 404", rec.Code)
	}
}

func TestRESTTriggerWithCommitID(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, sampleApp("web", "srv-1"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest("POST", "/v1/services/web/deploys", strings.NewReader(`{"commitId":"deadbeef"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("trigger with commitId: code=%d body=%s", rec.Code, rec.Body)
	}
	app := getApp(t, cl, "web")
	if app.Spec.BuildCommit != "deadbeef" {
		t.Errorf("spec.buildCommit = %q, want %q", app.Spec.BuildCommit, "deadbeef")
	}
}

// TestRESTTriggerClearCacheEnum pins Render's real wire type for clearCache:
// the string enum "clear"/"do_not_clear" (cli/pkg/client/types_gen.go), not a
// bool — the official CLI always sends one of these two values on every
// deploys-create call. bex builds are already always cache-free, so both
// recognized values are accepted as no-ops; only an unrecognized value 400s.
func TestRESTTriggerClearCacheEnum(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	trigger := func(body string) int {
		req := httptest.NewRequest("POST", "/v1/services/web/deploys", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := trigger(`{"clearCache":"do_not_clear"}`); code != http.StatusCreated {
		t.Errorf(`clearCache="do_not_clear": want 201, got %d`, code)
	}
	if code := trigger(`{"clearCache":"clear"}`); code != http.StatusCreated {
		t.Errorf(`clearCache="clear": want 201, got %d`, code)
	}
	if code := trigger(`{}`); code != http.StatusCreated {
		t.Errorf("omitted clearCache: want 201, got %d", code)
	}
	if code := trigger(`{"clearCache":"purge_everything"}`); code != http.StatusBadRequest {
		t.Errorf("unknown clearCache value: want 400, got %d", code)
	}
	if code := trigger(`{"clearCache":true}`); code != http.StatusBadRequest {
		t.Errorf("bool clearCache (the old, wrong wire type): want 400 (JSON decode failure), got %d", code)
	}
}

func TestRESTTriggerDeployOnlyRejectsRepoBacked(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("svc", "srv-2")
	app.Spec.Image = ""
	app.Spec.Repo = "https://github.com/bex-co/hello.git"
	svc, _ := newService(ds, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"deployMode":"deploy_only"}`
	req := httptest.NewRequest("POST", "/v1/services/svc/deploys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("deploy_only for repo-backed: want 400, got %d; body=%s", rec.Code, rec.Body)
	}
}

func TestREST503WithoutStore(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web", "srv-1"))
	do := newRESTHarness(t, svc)

	if rec := do("GET", "/v1/services/web/deploys"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code=%d, want 503", rec.Code)
	}
}

// --- MCP parity ---------------------------------------------------------------

// TestMCPMatchesREST drives list_deploys/get_deploy over an in-memory MCP
// session and asserts the structured results are identical to what List/Get
// (the same Service the REST fragment calls) return — three-adapter parity,
// not a second implementation.
func TestMCPMatchesREST(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	ctx := context.Background()
	cs := newMCPSession(t, svc)

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	have := map[string]bool{}
	for _, tl := range tools.Tools {
		have[tl.Name] = true
	}
	for _, want := range []string{"list_deploys", "get_deploy"} {
		if !have[want] {
			t.Errorf("tool %q not registered", want)
		}
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_deploys", Arguments: map[string]any{"serviceId": "web"}})
	if err != nil || res.IsError {
		t.Fatalf("list_deploys: %v isErr=%v", err, res.IsError)
	}
	var got listDeploysResult
	if err := decodeStructured(res.StructuredContent, &got); err != nil {
		t.Fatalf("decode list_deploys result: %v", err)
	}
	restList, err := svc.List(ctx, "web", ListFilter{})
	if err != nil || len(got.Deploys) != len(restList) || got.Deploys[0].ID != first.ID {
		t.Fatalf("list_deploys = %+v, want to match List() = %+v (err %v)", got, restList, err)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_deploy", Arguments: map[string]any{"serviceId": "web", "deployId": first.ID}})
	if err != nil || res.IsError {
		t.Fatalf("get_deploy: %v isErr=%v", err, res.IsError)
	}
	var oneGot renderDeploy
	if err := decodeStructured(res.StructuredContent, &oneGot); err != nil {
		t.Fatalf("decode get_deploy result: %v", err)
	}
	if oneGot.ID != first.ID || oneGot.Status != store.DeployCreated {
		t.Errorf("get_deploy = %+v", oneGot)
	}

	// An unknown deploy id is a tool error, not a silent empty result.
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_deploy", Arguments: map[string]any{"serviceId": "web", "deployId": "dep-nope"}})
	if err != nil {
		t.Fatalf("get_deploy unknown transport error: %v", err)
	}
	if !res.IsError {
		t.Errorf("get_deploy with unknown id: want a tool error, got %+v", res)
	}
}

func decodeStructured(v any, out any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
