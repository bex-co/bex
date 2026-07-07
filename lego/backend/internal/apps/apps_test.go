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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func fakeClient(apps ...*appv1alpha1.App) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func sampleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: name + ":v1", Replicas: 2},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, URL: "https://" + name + ".onbex.co"},
	}
}

func newService(st IntentStore, apps ...*appv1alpha1.App) (*Service, client.Client) {
	cl := fakeClient(apps...)
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: st}, cl
}

func getApp(t *testing.T, cl client.Client, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &a); err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return &a
}

// --- Read + write verbs ---

func TestServiceListGetVerbs(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"), sampleApp("api"))

	list, err := svc.List(context.Background())
	if err != nil || len(list) != 2 {
		t.Fatalf("List: %v len=%d", err, len(list))
	}

	v, err := svc.Get(context.Background(), "web")
	if err != nil || v.Name != "web" || v.Replicas != 2 {
		t.Fatalf("Get: %v %+v", err, v)
	}
	if _, err := svc.Get(context.Background(), "nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown => ErrNotFound, got %v", err)
	}

	// Suspend keeps replicas; Resume clears; Restart stamps restartedAt.
	if _, err := svc.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if a := getApp(t, cl, "web"); !a.Spec.Suspended || a.Spec.Replicas != 2 {
		t.Errorf("suspend must set suspended and keep replicas: %+v", a.Spec)
	}
	if _, err := svc.Resume(context.Background(), "web"); err != nil || getApp(t, cl, "web").Spec.Suspended {
		t.Errorf("resume should clear suspended: %v", err)
	}
	if _, err := svc.Restart(context.Background(), "web"); err != nil || getApp(t, cl, "web").Spec.RestartedAt == "" {
		t.Errorf("restart should stamp restartedAt: %v", err)
	}
}

// --- Single writer of intent (store-managed vs hand-applied) ---

type recordingStore struct {
	calls []struct {
		id        string
		suspended bool
	}
	err error
}

func (r *recordingStore) SetAppSuspended(_ context.Context, id string, suspended bool) error {
	if r.err != nil {
		return r.err
	}
	r.calls = append(r.calls, struct {
		id        string
		suspended bool
	}{id, suspended})
	return nil
}

func managedApp(name, appID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: appID}
	return a
}

func TestSuspendManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if _, err := svc.Resume(context.Background(), "web"); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if len(rec.calls) != 2 || rec.calls[0].id != "srv-1" || !rec.calls[0].suspended ||
		rec.calls[1].id != "srv-1" || rec.calls[1].suspended {
		t.Fatalf("want row writes [srv-1 true, srv-1 false], got %v", rec.calls)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("CR should be resumed after the fast-path patch")
	}
}

func TestSuspendUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, sampleApp("hand"))

	if _, err := svc.Suspend(context.Background(), "hand"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("unmanaged app must not touch the store, got %v", rec.calls)
	}
	if !getApp(t, cl, "hand").Spec.Suspended {
		t.Fatal("CR should be suspended")
	}
}

func TestSuspendRowWriteFailureLeavesCRUntouched(t *testing.T) {
	rec := &recordingStore{err: errors.New("db down")}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.Suspend(context.Background(), "web"); err == nil {
		t.Fatal("want error when the row write fails")
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("CR must not be patched when the row write failed")
	}
}

func TestSuspendWithoutStorePatchesCR(t *testing.T) {
	svc, cl := newService(nil, managedApp("web", "srv-1"))
	if _, err := svc.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if !getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("CR should be suspended")
	}
}

// --- REST + GraphQL fragments (Render shape), without the auth gate ---

func TestRESTFragmentRenderShape(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// list is the {service, cursor} envelope.
	var list []serviceWithCursor
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("list: %v %+v", err, list)
	}
	if list[0].Service.Type != renderWebService || list[0].Service.Suspended != core.RenderNotSuspended {
		t.Errorf("render service shape wrong: %+v", list[0].Service)
	}
	// suspend => 202, restart => 200 (Render status codes).
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/suspend", nil))
	if rec.Code != http.StatusAccepted {
		t.Errorf("suspend => 202, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/restart", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("restart => 200, got %d", rec.Code)
	}
	// unknown => 404.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown => 404, got %d", rec.Code)
	}
}
