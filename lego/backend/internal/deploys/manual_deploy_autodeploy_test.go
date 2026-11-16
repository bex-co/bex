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
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render (render.com/docs/deploys § Manual deploys): "Deploying a specific
// commit through the Dashboard or a deploy hook disables auto-deploys. The CLI
// and API don't." (w4/m431)

func autoDeployingRepoApp(trigger string) *appv1alpha1.App {
	a := repoApp("web", "srv-1", "main")
	a.Spec.AutoDeploy, a.Spec.AutoDeployTrigger = true, trigger
	return a
}

// autoDeployEvents is the auto-deploy rows sink recorded.
func autoDeployEvents(sink *deployHookAuditSink) []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbSetAutoDeploy {
			out = append(out, ev)
		}
	}
	return out
}

func assertManualDeployDisable(t *testing.T, sink *deployHookAuditSink, fromTrigger string) {
	t.Helper()
	evs := autoDeployEvents(sink)
	if len(evs) != 1 || evs[0].AutoDeployEnabled == nil || *evs[0].AutoDeployEnabled ||
		evs[0].AutoDeployReason == nil || *evs[0].AutoDeployReason != core.AutoDeployReasonManualDeploy ||
		evs[0].AutoDeployTrigger == nil || *evs[0].AutoDeployTrigger != fromTrigger {
		t.Fatalf("auto-deploy events = %+v, want one disable with reason manual_deploy from %s", evs, fromTrigger)
	}
}

func TestSpecificCommitDeployTurnsAutoDeployOffInTheSameWrite(t *testing.T) {
	for _, trigger := range []string{"", core.AutoDeployTriggerChecksPass} {
		ds := newFakeStore()
		app := autoDeployingRepoApp(trigger)
		app.Generation = 1
		cl := generationBumpingClient(app)
		svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: ds}
		sink := &deployHookAuditSink{}
		svc.Audit = sink
		d, err := svc.TriggerManual(context.Background(), "web", TriggerParams{CommitID: "deadbeef", DisableAutoDeploy: true})
		if err != nil {
			t.Fatalf("TriggerManual: %v", err)
		}
		got := getApp(t, cl, "web")
		if got.Spec.AutoDeploy || got.Spec.BuildCommit != "deadbeef" {
			t.Fatalf("after the pin: autoDeploy %v buildCommit %q, want off and deadbeef", got.Spec.AutoDeploy, got.Spec.BuildCommit)
		}
		// One write: the release the deploy row is filed under is the
		// generation that also turned auto-deploy off.
		row, err := ds.GetDeploy(context.Background(), "srv-1", d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if row.Generation != got.Generation {
			t.Fatalf("deploy generation %d, App generation %d: the pin and the switch-off were separate writes", row.Generation, got.Generation)
		}
		want := core.AutoDeployTriggerCommit
		if trigger != "" {
			want = trigger
		}
		assertManualDeployDisable(t, sink, want)
	}
}

func TestSpecificCommitDeployOnAServiceAlreadyOffRecordsNothing(t *testing.T) {
	svc, cl := newService(newFakeStore(), repoApp("web", "srv-1", "main"))
	sink := &deployHookAuditSink{}
	svc.Audit = sink
	if _, err := svc.TriggerManual(context.Background(), "web", TriggerParams{CommitID: "deadbeef", DisableAutoDeploy: true}); err != nil {
		t.Fatal(err)
	}
	if getApp(t, cl, "web").Spec.AutoDeploy {
		t.Fatal("auto-deploy came on")
	}
	if evs := autoDeployEvents(sink); len(evs) != 0 {
		t.Fatalf("auto-deploy events = %+v, want none for a service already off", evs)
	}
}

// Render's API and CLI keep auto-deploy: a commitId alone leaves it on.
func TestAPICommitDeployKeepsAutoDeploy(t *testing.T) {
	svc, cl := newService(newFakeStore(), autoDeployingRepoApp(""))
	sink := &deployHookAuditSink{}
	svc.Audit = sink
	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{CommitID: "deadbeef"}); err != nil {
		t.Fatal(err)
	}
	if !getApp(t, cl, "web").Spec.AutoDeploy {
		t.Fatal("an API deploy of a commit turned auto-deploy off")
	}
	if evs := autoDeployEvents(sink); len(evs) != 0 {
		t.Fatalf("auto-deploy events = %+v, want none", evs)
	}
}

func TestDisableAutoDeployWithoutACommitIsRefused(t *testing.T) {
	svc, cl := newService(newFakeStore(), autoDeployingRepoApp(""))
	before := getApp(t, cl, "web")
	_, err := svc.TriggerManual(context.Background(), "web", TriggerParams{DisableAutoDeploy: true})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("err = %v, want ErrBadRequest", err)
	}
	if got := getApp(t, cl, "web"); got.ResourceVersion != before.ResourceVersion {
		t.Fatal("a refused trigger changed the App")
	}
}

func callDeployHook(t *testing.T, svc *Service, query string) *httptest.ResponseRecorder {
	t.Helper()
	if svc.DeployHookLimiter == nil {
		svc.DeployHookLimiter = NewDeployHookRateLimiter(6000, 10)
	}
	hook, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	svc.DeployHookHandler().ServeHTTP(w, httptest.NewRequest(http.MethodPost,
		"/v1/deploy-hooks?key="+url.QueryEscape(deployHookTokenFromURL(t, hook.URL))+query, nil))
	return w
}

func TestDeployHookRefTurnsAutoDeployOff(t *testing.T) {
	svc, cl := newService(newFakeStore(), autoDeployingRepoApp(""))
	sink := &deployHookAuditSink{}
	svc.Audit = sink
	if w := callDeployHook(t, svc, "&ref=deadbeef"); w.Code != http.StatusOK {
		t.Fatalf("hook: code=%d body=%s", w.Code, w.Body.String())
	}
	if got := getApp(t, cl, "web"); got.Spec.AutoDeploy || got.Spec.BuildCommit != "deadbeef" {
		t.Fatalf("after the hook: autoDeploy %v buildCommit %q, want off and deadbeef", got.Spec.AutoDeploy, got.Spec.BuildCommit)
	}
	assertManualDeployDisable(t, sink, core.AutoDeployTriggerCommit)
}

func TestDeployHookWithoutRefKeepsAutoDeploy(t *testing.T) {
	svc, cl := newService(newFakeStore(), autoDeployingRepoApp(""))
	sink := &deployHookAuditSink{}
	svc.Audit = sink
	if w := callDeployHook(t, svc, ""); w.Code != http.StatusOK {
		t.Fatalf("hook: code=%d body=%s", w.Code, w.Body.String())
	}
	if !getApp(t, cl, "web").Spec.AutoDeploy {
		t.Fatal("a hook call without ref turned auto-deploy off")
	}
	if evs := autoDeployEvents(sink); len(evs) != 0 {
		t.Fatalf("auto-deploy events = %+v, want none", evs)
	}
}

// Behind a deploy in progress the hook answers Render's 202. When the queued
// release is written through, the switch-off rides that write at once.
func TestDeployHookRefQueuedBehindADeployTurnsAutoDeployOff(t *testing.T) {
	ds := newFakeStore()
	app := autoDeployingRepoApp("")
	app.Generation = 1
	svc := &Service{Base: &core.Base{Client: generationBumpingClient(app), Namespace: "default", Clock: fixedNow}, Store: ds}
	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); err != nil {
		t.Fatal(err)
	}
	if w := callDeployHook(t, svc, "&ref=deadbeef"); w.Code != http.StatusAccepted {
		t.Fatalf("hook behind a deploy: code=%d body=%s, want 202", w.Code, w.Body.String())
	}
	if getApp(t, svc.Client, "web").Spec.AutoDeploy {
		t.Fatal("a queued hook deploy with ref left auto-deploy on")
	}
}

// Behind a selected release in flight the hook's deploy waits without
// changing the App (w4/661), and turns auto-deploy off when it opens.
func TestDeferredDeployHookRefTurnsAutoDeployOffWhenItOpens(t *testing.T) {
	ds := newDeferringStore()
	ctx := context.Background()
	flight, _ := ds.CreateDeploy(ctx, "srv-1", store.TriggerRollback, "web:v1", 2, store.CommitInfo{}, "")
	ds.set(flight.ID, func(d *store.Deploy) { d.ID, d.Status = "dep-flight", store.DeployUpdateInProgress })
	app := autoDeployingRepoApp("")
	app.Generation = 2
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 2, Image: "web:v1"}
	app.Status.ReleaseGeneration, app.Status.SelectedReleaseGeneration = 2, 2
	svc := &Service{Base: &core.Base{Client: generationBumpingClient(app), Namespace: "default", Clock: fixedNow}, Store: ds}
	sink := &deployHookAuditSink{}
	svc.Audit = sink

	if w := callDeployHook(t, svc, "&ref=deadbeef"); w.Code != http.StatusAccepted {
		t.Fatalf("hook behind a selected release: code=%d body=%s, want 202", w.Code, w.Body.String())
	}
	if !getApp(t, svc.Client, "web").Spec.AutoDeploy {
		t.Fatal("a waiting hook deploy changed the App before it opened")
	}
	if evs := autoDeployEvents(sink); len(evs) != 0 {
		t.Fatalf("auto-deploy events before the deploy opened = %+v", evs)
	}

	settle(t, ds, svc, "dep-flight")
	if got := getApp(t, svc.Client, "web"); got.Spec.AutoDeploy || got.Spec.BuildCommit != "deadbeef" {
		t.Fatalf("after the deploy opened: autoDeploy %v buildCommit %q, want off and deadbeef", got.Spec.AutoDeploy, got.Spec.BuildCommit)
	}
	assertManualDeployDisable(t, sink, core.AutoDeployTriggerCommit)
}

// GraphQL triggerDeploy mirrors rollbackService: disableAutoDeploy is the
// dashboard's, and every other caller keeps auto-deploy by default.
func TestGraphQLTriggerDeployDisablesAutoDeployOnlyWhenAsked(t *testing.T) {
	for _, tc := range []struct {
		name string
		arg  string
		want bool
	}{
		{"default leaves it on", "", true},
		{"dashboard turns it off", ", disableAutoDeploy: true", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(newFakeStore(), autoDeployingRepoApp(""))
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
				Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
			})
			if err != nil {
				t.Fatal(err)
			}
			res := graphql.Do(graphql.Params{
				Schema:        schema,
				Context:       context.Background(),
				RequestString: `mutation { triggerDeploy(serviceId: "web", commitId: "deadbeef"` + tc.arg + `) { id } }`,
			})
			if len(res.Errors) > 0 {
				t.Fatalf("errors: %v", res.Errors)
			}
			if got := getApp(t, cl, "web"); got.Spec.AutoDeploy != tc.want || got.Spec.BuildCommit != "deadbeef" {
				t.Errorf("autoDeploy = %v buildCommit %q, want %v and deadbeef", got.Spec.AutoDeploy, got.Spec.BuildCommit, tc.want)
			}
		})
	}
}
