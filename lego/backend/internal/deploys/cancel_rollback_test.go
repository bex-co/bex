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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- Cancel ------------------------------------------------------------------

func TestCancelClosesOpenDeployAndIsIdempotentConflict(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	got, err := svc.Cancel(context.Background(), "web", first.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Status != store.DeployCanceled || got.FinishedAt == nil {
		t.Fatalf("canceled deploy = %+v, want status canceled with finished_at set", got)
	}
	stored, err := ds.GetDeploy(context.Background(), "srv-1", first.ID)
	if err != nil || stored.Status != store.DeployCanceled {
		t.Fatalf("stored deploy = %+v (err %v), want canceled", stored, err)
	}

	// Already terminal: a second Cancel is Render's 409, never a silent no-op.
	if _, err := svc.Cancel(context.Background(), "web", first.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("re-cancel: want core.ErrConflict, got %v", err)
	}
}

func TestCancelRefusesAlreadyLiveDeploy(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close: won=%v err=%v", won, err)
	}
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	if _, err := svc.Cancel(context.Background(), "web", first.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("cancel of a live deploy: want core.ErrConflict, got %v", err)
	}
}

func TestCancelUnknownDeployIsNotFound(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	if _, err := svc.Cancel(context.Background(), "web", "dep-nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cancel unknown deploy: want core.ErrNotFound, got %v", err)
	}
}

// TestCancelDeletesInFlightBuildJob covers the repo-backed path (t002): a
// build Job named per the operator's own convention (buildJobName mirrors
// lego/operator/internal/build.JobName) is deleted when Cancel runs against
// a repo-backed App with an open deploy.
func TestCancelDeletesInFlightBuildJob(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "", 3, store.CommitInfo{})
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Repo = "https://example.invalid/acme/web.git"
	app.Generation = 5 // deliberately different from the deploy's own Generation (3) — Cancel must use the row's, not this
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: buildJobName("web", 3), Namespace: "default"}}
	svc, cl := newService(ds, app, job)

	if _, err := svc.Cancel(context.Background(), "web", first.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	var gone batchv1.Job
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: buildJobName("web", 3)}, &gone)
	if !apierrors.IsNotFound(err) {
		t.Errorf("build job after cancel: want deleted (NotFound), got %v", err)
	}
}

func TestCancelDeletesInFlightKpackImage(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "", 3, store.CommitInfo{})
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Repo = "https://example.invalid/acme/web.git"
	image := &unstructured.Unstructured{}
	image.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Image"})
	image.SetName(buildJobName("web", 3))
	image.SetNamespace("default")
	svc, cl := newService(ds, app, image)

	if _, err := svc.Cancel(context.Background(), "web", first.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	gone := &unstructured.Unstructured{}
	gone.SetGroupVersionKind(image.GroupVersionKind())
	err := cl.Get(context.Background(), client.ObjectKeyFromObject(image), gone)
	if !apierrors.IsNotFound(err) {
		t.Errorf("kpack Image after cancel: want deleted (NotFound), got %v", err)
	}
}

// TestCancelImageBackedAppIsNoJobNoop covers the image-backed case: there is
// no build Job to begin with, so Cancel's delete is a harmless not-found
// no-op and the row still closes canceled.
func TestCancelImageBackedAppIsNoJobNoop(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	svc, _ := newService(ds, sampleApp("web", "srv-1")) // Repo == "", Image set — no Job ever existed

	got, err := svc.Cancel(context.Background(), "web", first.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Status != store.DeployCanceled {
		t.Errorf("canceled deploy = %+v", got)
	}
}

// --- Rollback ----------------------------------------------------------------

func TestRollbackRestoresPreviousLiveImage(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close first live: won=%v err=%v", won, err)
	}
	bad, _ := ds.CreateDeploy(context.Background(), "srv-1", "api", "web:bad", 2, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), bad.ID, store.DeployUpdateFailed, ""); err != nil || !won {
		t.Fatalf("close bad failed: won=%v err=%v", won, err)
	}
	app := sampleApp("web", "srv-1")
	app.Spec.Image = "web:bad"
	svc, cl := newService(ds, app)

	rolled, err := svc.Rollback(context.Background(), "web", first.ID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolled.Trigger != "rollback" || rolled.Image != "web:v1" || rolled.RollbackOf != first.ID {
		t.Fatalf("rolled-back deploy = %+v", rolled)
	}
	if ds.setImage["srv-1"] != "web:v1" {
		t.Errorf("row-first write: SetAppImage(srv-1, ...) = %q, want web:v1", ds.setImage["srv-1"])
	}
	got := getApp(t, cl, "web")
	if got.Spec.Image != "web:v1" {
		t.Errorf("app spec.image after rollback = %q, want web:v1", got.Spec.Image)
	}
	if got.Spec.RestartedAt == "" {
		t.Error("Rollback must bump spec.restartedAt so the CR converges immediately")
	}
	if annotation := got.Annotations[appv1alpha1.AnnotationReleaseGeneration]; annotation != "2" {
		t.Errorf("release-generation annotation = %q, want 2", annotation)
	}

	list, err := svc.List(context.Background(), "web", ListFilter{})
	if err != nil || len(list) != 3 || list[0].ID != rolled.ID {
		t.Fatalf("List after rollback (want newest-first, 3 entries) = %+v (err %v)", list, err)
	}
}

func TestRollbackRefusesNonLiveTarget(t *testing.T) {
	ds := newFakeStore()
	stillOpen, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	if _, err := svc.Rollback(context.Background(), "web", stillOpen.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("rollback to a never-live deploy: want core.ErrConflict, got %v", err)
	}
}

func TestRollbackUnknownDeployIsNotFound(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	if _, err := svc.Rollback(context.Background(), "web", "dep-nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("rollback unknown deploy: want core.ErrNotFound, got %v", err)
	}
}

func TestRollbackRefusesSuspendedService(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	_, _ = ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1")
	app := sampleApp("web", "srv-1")
	app.Spec.Suspended = true
	svc, _ := newService(ds, app)

	if _, err := svc.Rollback(context.Background(), "web", first.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("rollback of suspended service: want core.ErrConflict, got %v", err)
	}
}

func TestRollbackRequiresStoreManagedApp(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("manual", ""))

	if _, err := svc.Rollback(context.Background(), "manual", "dep-1"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("hand-applied rollback: want core.ErrBadRequest, got %v", err)
	}
}

// --- Store-off 503 ------------------------------------------------------------

func TestCancelRollbackUnavailableWithoutStore(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web", "srv-1"))
	ctx := context.Background()

	if _, err := svc.Cancel(ctx, "web", "dep-1"); !errors.Is(err, core.ErrDeploysUnavailable) {
		t.Errorf("Cancel: want ErrDeploysUnavailable, got %v", err)
	}
	if _, err := svc.Rollback(ctx, "web", "dep-1"); !errors.Is(err, core.ErrDeploysUnavailable) {
		t.Errorf("Rollback: want ErrDeploysUnavailable, got %v", err)
	}
}

// --- REST fragment -------------------------------------------------------------

func TestRESTCancelAndRollback(t *testing.T) {
	ds := newFakeStore()
	open, _ := ds.CreateDeploy(context.Background(), "srv-1", "api", "web:v2", 2, store.CommitInfo{})
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/deploys/"+open.ID+"/cancel", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel: code=%d body=%s", rec.Code, rec.Body)
	}
	var canceled renderDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &canceled); err != nil || canceled.Status != store.DeployCanceled {
		t.Fatalf("cancel body = %s (err %v)", rec.Body, err)
	}

	// Cancel again: past the cancelable window, Render's 409.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/deploys/"+open.ID+"/cancel", nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("re-cancel: code=%d, want 409", rec.Code)
	}

	// Rollback needs a live target.
	live, _ := ds.CreateDeploy(context.Background(), "srv-1", "api", "web:v1", 2, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), live.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close live: won=%v err=%v", won, err)
	}

	rec = httptest.NewRecorder()
	body := strings.NewReader(`{"deployId":"` + live.ID + `"}`)
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/rollback", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("rollback: code=%d body=%s", rec.Code, rec.Body)
	}
	var rolled renderDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &rolled); err != nil || rolled.Trigger != "rollback" || rolled.RollbackOf != live.ID {
		t.Fatalf("rollback body = %s (err %v)", rec.Body, err)
	}
}

func TestREST503CancelRollbackWithoutStore(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web", "srv-1"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/deploys/dep-1/cancel", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("cancel: code=%d, want 503", rec.Code)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/rollback", strings.NewReader(`{"deployId":"dep-1"}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("rollback: code=%d, want 503", rec.Code)
	}
}

// --- MCP parity ---------------------------------------------------------------

// TestMCPRegistersCancelAndRollback covers three-adapter parity for w2/m10:
// cancel_deploy/rollback_deploy are registered alongside list_deploys/get_deploy.
func TestMCPRegistersCancelAndRollback(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	_, _ = ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1")
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	have := map[string]bool{}
	for _, tl := range tools.Tools {
		have[tl.Name] = true
	}
	for _, want := range []string{"list_deploys", "get_deploy", "cancel_deploy", "rollback_deploy"} {
		if !have[want] {
			t.Errorf("tool %q not registered", want)
		}
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "cancel_deploy", Arguments: map[string]any{"serviceId": "web", "deployId": first.ID}})
	if err != nil {
		t.Fatalf("cancel_deploy transport error: %v", err)
	}
	// first is already live (closed above) — past the cancelable window, a tool error.
	if !res.IsError {
		t.Errorf("cancel_deploy on a live deploy: want a tool error, got %+v", res)
	}

	second, _ := ds.CreateDeploy(context.Background(), "srv-1", "api", "web:v2", 2, store.CommitInfo{})
	_, _ = ds.CloseDeploy(context.Background(), second.ID, store.DeployLive, "web:v2")
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "rollback_deploy", Arguments: map[string]any{"serviceId": "web", "deployId": first.ID}})
	if err != nil || res.IsError {
		t.Fatalf("rollback_deploy: %v isErr=%v", err, res.IsError)
	}
	var got renderDeploy
	if err := decodeStructured(res.StructuredContent, &got); err != nil {
		t.Fatalf("decode rollback_deploy result: %v", err)
	}
	if got.Trigger != "rollback" || got.RollbackOf != first.ID {
		t.Errorf("rollback_deploy result = %+v", got)
	}
}
