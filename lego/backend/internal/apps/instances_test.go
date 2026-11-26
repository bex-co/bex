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

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
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

// w4/663: the pre-deploy Job's pod carries the App's pod label while its
// command runs, but it is no instance: a first deploy in pre-deploy lists none
// (as the shell says), and a later one only the Deployment's pods.
func TestListInstancesSkipsThePreDeployPod(t *testing.T) {
	for _, serviceType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker} {
		t.Run(serviceType, func(t *testing.T) {
			app := instanceTestApp()
			app.Spec.Type = serviceType
			created := time.Date(2026, 11, 14, 13, 38, 34, 0, time.UTC)
			preDeploy := instanceTestPod("predeploy-web-gen-2-x7k2p", app.Name, "99999999-9999-9999-9999-999999999999", corev1.PodRunning, created)
			preDeploy.Labels[core.PodLabelComponent] = "predeploy"
			preDeploy.Labels[core.PodLabelPreDeploy] = app.Name

			got, err := instanceService(app, preDeploy).ListInstances(context.Background(), testServiceID)
			if err != nil || len(got) != 0 {
				t.Fatalf("first deploy in pre-deploy: instances = %+v, %v; want none", got, err)
			}

			serving := instanceTestPod("web-5ff6c9466c-l4hnb", app.Name, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", corev1.PodRunning, created.Add(-time.Hour))
			got, err = instanceService(app, preDeploy, serving).ListInstances(context.Background(), testServiceID)
			if err != nil || len(got) != 1 || !got[0].CreatedAt.Equal(serving.CreationTimestamp.Time) {
				t.Fatalf("later deploy in pre-deploy: instances = %+v, %v; want only the serving pod", got, err)
			}
		})
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
	// The pod's deploy is known (w1/m661), and REST still carries only
	// Render's two public fields.
	pod.Labels[core.PodLabelRevision] = "rev-7"
	svc := instanceService(app, pod)
	svc.Store = &recordingStore{releaseDeploys: map[releaseKey]string{{testServiceID, 7}: "dep-live"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	path := "/v1/services/" + testServiceID + "/instances"
	t.Run(path, func(t *testing.T) {
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
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
		// A Kubernetes client reads a pod's creation time in the local zone.
		want := `[{"id":"` + ids.ServiceInstanceID(testServiceID, pod.Name) + `","createdAt":"` + created.Local().Format(time.RFC3339) + `"}]`
		if got := strings.TrimSpace(rec.Body.String()); got != want {
			t.Fatalf("body = %s, want byte for byte %s", got, want)
		}
	})
}

// revisionPod is an instance pod created for the release revision names, in
// phase; a Running one is Ready unless the phase is starting, a Running pod
// that is not Ready yet.
func revisionPod(name, app, revision string, phase corev1.PodPhase, created time.Time) *corev1.Pod {
	ready := phase == corev1.PodRunning
	if phase == starting {
		phase, ready = corev1.PodRunning, false
	}
	pod := instanceTestPod(name, app, "uid-"+name, phase, created)
	if revision != "" {
		pod.Labels[core.PodLabelRevision] = revision
	}
	if phase == corev1.PodRunning {
		status := corev1.ConditionFalse
		if ready {
			status = corev1.ConditionTrue
		}
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}
	}
	return pod
}

// starting stands for a Running pod that is not Ready yet in revisionPod.
const starting corev1.PodPhase = "Starting"

// w1/m661: each instance names the deploy row of the release its pod was
// created for, and its status (instanceStatus). A release no
// deploy row records, and a pod with no release revision, name no deploy.
func TestListInstanceDeploysNamesEachInstancesDeployAndStatus(t *testing.T) {
	app := instanceTestApp()
	base := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	untracked := revisionPod("web-untracked", app.Name, "rev-5", corev1.PodUnknown, base.Add(3*time.Minute))
	surge := revisionPod("web-surge", app.Name, "rev-4", starting, base.Add(2*time.Minute))
	rolled := revisionPod("web-rolled", app.Name, "rev-4", corev1.PodRunning, base.Add(time.Minute))
	old := revisionPod("web-old", app.Name, "rev-3", corev1.PodRunning, base)
	unlabeled := revisionPod("web-unlabeled", app.Name, "", corev1.PodRunning, base.Add(-time.Minute))
	svc := instanceService(app, untracked, surge, rolled, old, unlabeled)
	svc.Store = &recordingStore{releaseDeploys: map[releaseKey]string{
		{testServiceID, 3}: "dep-old",
		{testServiceID, 4}: "dep-new",
		// Another service's row of generation 5 is not this one's.
		{"srv-other", 5}: "dep-other",
	}}

	got, err := svc.ListInstanceDeploys(context.Background(), testServiceID)
	if err != nil {
		t.Fatalf("ListInstanceDeploys: %v", err)
	}
	want := []struct {
		pod            *corev1.Pod
		deploy, status string
	}{
		{untracked, "", "unknown"},
		{surge, "dep-new", "pending"},
		{rolled, "dep-new", "running"},
		{old, "dep-old", "running"},
		{unlabeled, "", "running"},
	}
	if len(got) != len(want) {
		t.Fatalf("instances = %+v, want %d", got, len(want))
	}
	public, err := svc.ListInstances(context.Background(), testServiceID)
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	for i, w := range want {
		if got[i].ID != ids.ServiceInstanceID(testServiceID, w.pod.Name) || got[i].DeployID != w.deploy || got[i].Status != w.status {
			t.Errorf("instance %d = %s deploy %q status %q, want %s deploy %q status %q", i,
				got[i].ID, got[i].DeployID, got[i].Status, w.pod.Name, w.deploy, w.status)
		}
		if got[i].ID != public[i].ID || !got[i].CreatedAt.Equal(public[i].CreatedAt) {
			t.Errorf("instance %d = %s %s, REST's = %+v: the two lists disagree", i, got[i].ID, got[i].CreatedAt, public[i])
		}
	}
}

// A store-less bex-api records no deploys, so its instances name none; a
// store that cannot answer fails GraphQL's read and never REST's, which does
// not read deploys.
func TestListInstanceDeploysWithoutAStore(t *testing.T) {
	app := instanceTestApp()
	pod := revisionPod("web-a", app.Name, "rev-2", corev1.PodRunning, time.Now())

	got, err := instanceService(app, pod).ListInstanceDeploys(context.Background(), testServiceID)
	if err != nil || len(got) != 1 || got[0].DeployID != "" || got[0].Status != "running" {
		t.Fatalf("store-less instances = %+v, %v; want one running instance with no deploy", got, err)
	}

	failing := instanceService(app, pod)
	failing.Store = &recordingStore{err: errors.New("deploys unreadable")}
	if _, err := failing.ListInstanceDeploys(context.Background(), testServiceID); err == nil {
		t.Fatal("ListInstanceDeploys answered over a store that cannot read deploys")
	}
	if public, err := failing.ListInstances(context.Background(), testServiceID); err != nil || len(public) != 1 {
		t.Fatalf("ListInstances over an unreadable store = %+v, %v; want the instance", public, err)
	}
}

// An instance's status is its pod's phase, but a Running pod runs only once
// it is Ready: starting, failing its health check or crash-looping, it is
// still pending, so it does not count toward a deploy's Scaling.
func TestInstanceStatusIsThePodPhaseOnceReady(t *testing.T) {
	created := time.Now()
	crashLooping := revisionPod("crash", "web", "rev-1", starting, created)
	crashLooping.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: core.AppContainer, RestartCount: 4,
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}}}
	for _, tc := range []struct {
		pod  *corev1.Pod
		want string
	}{
		{revisionPod("pending", "web", "rev-1", corev1.PodPending, created), "pending"},
		{revisionPod("ready", "web", "rev-1", corev1.PodRunning, created), "running"},
		{revisionPod("starting", "web", "rev-1", starting, created), "pending"},
		{crashLooping, "pending"},
		{instanceTestPod("no-conditions", "web", "u", corev1.PodRunning, created), "pending"},
		{revisionPod("succeeded", "web", "rev-1", corev1.PodSucceeded, created), "succeeded"},
		{revisionPod("failed", "web", "rev-1", corev1.PodFailed, created), "failed"},
		{revisionPod("unknown", "web", "rev-1", corev1.PodUnknown, created), "unknown"},
		{revisionPod("unreported", "web", "rev-1", "", created), "unknown"},
	} {
		if got := instanceStatus(tc.pod); got != tc.want {
			t.Errorf("instanceStatus(%s) = %q, want %q", tc.pod.Name, got, tc.want)
		}
	}
}

// GraphQL serviceInstances answers deployId (null for no deploy) and status
// beside id and createdAt, under the same authorization as REST.
func TestGraphQLServiceInstancesDeployAndStatus(t *testing.T) {
	app := instanceTestApp()
	created := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	live := revisionPod("web-live", app.Name, "rev-9", corev1.PodRunning, created)
	untracked := revisionPod("web-untracked", app.Name, "rev-10", starting, created.Add(time.Minute))
	svc := instanceService(app, live, untracked)
	svc.Store = &recordingStore{releaseDeploys: map[releaseKey]string{{testServiceID, 9}: "dep-live"}}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	query := `{ serviceInstances(id: "` + testServiceID + `") { id createdAt deployId status } }`
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	got, _ := json.Marshal(res.Data)
	want := `{"serviceInstances":[` +
		`{"createdAt":"2026-11-26T09:01:00Z","deployId":null,"id":"` + ids.ServiceInstanceID(testServiceID, untracked.Name) + `","status":"pending"},` +
		`{"createdAt":"2026-11-26T09:00:00Z","deployId":"dep-live","id":"` + ids.ServiceInstanceID(testServiceID, live.Name) + `","status":"running"}]}`
	if string(got) != want {
		t.Fatalf("serviceInstances =\n%s\nwant\n%s", got, want)
	}

	svc.Authz = &fakeChecker{allow: false}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "intruder", Method: "oauth2"})
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: query})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "forbidden") {
		t.Fatalf("an unauthorized read = %v, %v; want forbidden", res.Data, res.Errors)
	}
}

func TestRESTServiceInstancesEmptyMissingAndForeignWorkspace(t *testing.T) {
	t.Run("empty is array not null", func(t *testing.T) {
		app := instanceTestApp()
		app.Spec.Suspended = true
		mux := http.NewServeMux()
		instanceService(app).RegisterREST(mux)
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/"+testServiceID+"/instances", nil))
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("empty instances = %d %q, want 200 []", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing uses shared error envelope", func(t *testing.T) {
		mux := http.NewServeMux()
		instanceService().RegisterREST(mux)
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/srv-missing/instances", nil))
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
		validated(t, mux).ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || trap.podListCalled {
			t.Fatalf("foreign workspace = %d podList=%v body=%s", rec.Code, trap.podListCalled, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "99999999") {
			t.Fatalf("foreign response leaked Pod data: %s", rec.Body.String())
		}
	})
}
