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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const testServiceID = "srv-c185th5c2rvvnhbfiltg"

func instanceTestApp() *appv1alpha1.App {
	a := managedApp(core.CRName("tea-a", "web"), testServiceID)
	a.Labels[core.LabelServiceName] = "web"
	a.Spec.Type = appv1alpha1.TypeWebService
	a.Spec.Replicas = 2
	return a
}

func instanceTestPod(name, app, uid string, phase corev1.PodPhase, created time.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			UID:               types.UID(uid),
			Labels:            map[string]string{core.PodLabelApp: app},
			CreationTimestamp: metav1.NewTime(created),
		},
		Status: corev1.PodStatus{Phase: phase},
	}
}

func instanceService(objects ...client.Object) *Service {
	return &Service{Base: &core.Base{Client: fakeClient(objects...), Namespace: "default"}}
}

func TestListInstancesProjectsLiveRolloutPodsOnly(t *testing.T) {
	app := instanceTestApp()
	oldTime := time.Date(2026, 7, 14, 20, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Minute)
	oldPod := instanceTestPod("internal-old-name", app.Name, "11111111-1111-1111-1111-111111111111", corev1.PodRunning, oldTime)
	newPod := instanceTestPod("internal-new-name", app.Name, "22222222-2222-2222-2222-222222222222", corev1.PodPending, newTime)
	unknownPod := instanceTestPod("internal-unknown-name", app.Name, "33333333-3333-3333-3333-333333333333", corev1.PodUnknown, newTime.Add(-time.Second))
	terminating := instanceTestPod("terminating", app.Name, "44444444-4444-4444-4444-444444444444", corev1.PodRunning, newTime)
	now := metav1.NewTime(newTime)
	terminating.DeletionTimestamp = &now
	terminating.Finalizers = []string{"test.bex.co/hold"}
	succeeded := instanceTestPod("cron-success", app.Name, "55555555-5555-5555-5555-555555555555", corev1.PodSucceeded, oldTime)
	failed := instanceTestPod("cron-failed", app.Name, "66666666-6666-6666-6666-666666666666", corev1.PodFailed, oldTime)
	unrelated := instanceTestPod("other-workspace", "same-public-name", "77777777-7777-7777-7777-777777777777", corev1.PodRunning, oldTime)

	svc := instanceService(app, oldPod, newPod, unknownPod, terminating, succeeded, failed, unrelated)
	got, err := svc.ListInstances(context.Background(), testServiceID)
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("instances = %+v, want the old/new/unknown non-terminal rollout Pods only", got)
	}
	if !got[0].CreatedAt.Equal(newTime) || !got[1].CreatedAt.Equal(newTime.Add(-time.Second)) || !got[2].CreatedAt.Equal(oldTime) {
		t.Fatalf("instances are not newest-first from Pod creation timestamps: %+v", got)
	}
	for _, instance := range got {
		if !strings.HasPrefix(instance.ID, testServiceID+"-") {
			t.Errorf("instance id %q does not extend public service id", instance.ID)
		}
		if strings.Contains(instance.ID, "internal-") {
			t.Errorf("instance id leaked a Kubernetes Pod name: %q", instance.ID)
		}
	}
	again, err := svc.ListInstances(context.Background(), testServiceID)
	if err != nil || len(again) != len(got) {
		t.Fatalf("second ListInstances: %v %+v", err, again)
	}
	for i := range got {
		if again[i] != got[i] {
			t.Fatalf("instance identity changed across reads: first=%+v second=%+v", got, again)
		}
	}
}

func TestListInstancesReturnsAllocatedEmptyArrayForNoReplicaKinds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*appv1alpha1.App)
	}{
		{name: "suspended", mutate: func(a *appv1alpha1.App) { a.Spec.Suspended = true }},
		{name: "auto-hibernated-scale-to-zero", mutate: func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseHibernated }},
		{name: "cron-job", mutate: func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeCronJob }},
		{name: "static-site", mutate: func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeStaticSite }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := instanceTestApp()
			tc.mutate(app)
			leftover := instanceTestPod("leftover", app.Name, "88888888-8888-8888-8888-888888888888", corev1.PodRunning, time.Now())
			got, err := instanceService(app, leftover).ListInstances(context.Background(), testServiceID)
			if err != nil {
				t.Fatalf("ListInstances: %v", err)
			}
			if got == nil || len(got) != 0 {
				t.Fatalf("instances = %#v, want allocated empty slice", got)
			}
		})
	}
}

type podListTrapClient struct {
	client.Client
	podListCalled bool
}

func (c *podListTrapClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*corev1.PodList); ok {
		c.podListCalled = true
		return errors.New("Pod list must not run before authorization")
	}
	return c.Client.List(ctx, list, opts...)
}

func TestListInstancesAuthorizesBeforeObservingPods(t *testing.T) {
	app := instanceTestApp()
	trap := &podListTrapClient{Client: fakeClient(app)}
	svc := &Service{Base: &core.Base{
		Client: trap, Namespace: "default", Authz: &fakeChecker{allow: false},
	}}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "intruder", Method: "oauth2"})
	if _, err := svc.ListInstances(ctx, testServiceID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("ListInstances error = %v, want ErrForbidden", err)
	}
	if trap.podListCalled {
		t.Fatal("ListInstances observed Pods before App authorization")
	}
}

func TestRESTServiceInstancesExactWireShape(t *testing.T) {
	app := instanceTestApp()
	created := time.Date(2026, 7, 14, 20, 30, 0, 0, time.UTC)
	pod := instanceTestPod("web-abc-123", app.Name, "99999999-9999-9999-9999-999999999999", corev1.PodRunning, created)
	svc := instanceService(app, pod)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	path := "/v1/services/" + testServiceID + "/instances"
	t.Run(path, func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
		}
		var renderCompatible []struct {
			ID        string    `json:"id"`
			CreatedAt time.Time `json:"createdAt"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &renderCompatible); err != nil || len(renderCompatible) != 1 {
			t.Fatalf("Render-compatible decode: %v %#v body=%s", err, renderCompatible, rec.Body.String())
		}
		if renderCompatible[0].ID == "" || !renderCompatible[0].CreatedAt.Equal(created) {
			t.Fatalf("decoded instance = %+v", renderCompatible[0])
		}
		var raw []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil || len(raw[0]) != 2 {
			t.Fatalf("wire shape contains fields beyond id/createdAt: %v %#v", err, raw)
		}
	})
}

func TestRESTServiceInstancesEmptyMissingAndForeignWorkspace(t *testing.T) {
	t.Run("empty is array not null", func(t *testing.T) {
		app := instanceTestApp()
		app.Spec.Suspended = true
		mux := http.NewServeMux()
		instanceService(app).RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/"+testServiceID+"/instances", nil))
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("empty instances = %d %q, want 200 []", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing uses shared error envelope", func(t *testing.T) {
		mux := http.NewServeMux()
		instanceService().RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/srv-missing/instances", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("missing = %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["id"] != "not_found" || body["message"] == nil || body["error"] == nil {
			t.Fatalf("missing error envelope = %#v, err=%v", body, err)
		}
	})

	t.Run("foreign workspace is forbidden before Pod list", func(t *testing.T) {
		app := instanceTestApp()
		app.Labels[core.LabelTenant] = "tea-b"
		trap := &podListTrapClient{Client: fakeClient(app)}
		svc := &Service{Base: &core.Base{
			Client: trap, Namespace: "default", Workspace: fakeWorkspace{"alice": "tea-a"},
		}}
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		req := httptest.NewRequest(http.MethodGet, "/v1/services/"+testServiceID+"/instances", nil)
		req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "alice", Method: "session"}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || trap.podListCalled {
			t.Fatalf("foreign workspace = %d podList=%v body=%s", rec.Code, trap.podListCalled, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "99999999") {
			t.Fatalf("foreign response leaked Pod data: %s", rec.Body.String())
		}
	})
}
