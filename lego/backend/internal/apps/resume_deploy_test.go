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
	"errors"
	"testing"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w3/m643: with the deploys verb wired, Resume resumes through the deploy it
// opens (Render's service_resumed) and answers it as LatestDeployID, which
// GraphQL resumeService reads as latestDeployId.

// resumeHook stands in for deploys.Service.ResumeRelease: it applies the
// resume's own row write and App change, as the release write does, and
// answers deployID.
type resumeHook struct {
	cl       client.Client
	deployID string
	err      error
	calls    int
}

func (h *resumeHook) run(ctx context.Context, _ string, a *appv1alpha1.App, writeRow func() error, resume func(*appv1alpha1.App)) (string, error) {
	h.calls++
	if h.err != nil || h.deployID == "" {
		return "", h.err
	}
	if err := writeRow(); err != nil {
		return "", err
	}
	base := client.MergeFrom(a.DeepCopy())
	resume(a)
	if err := h.cl.Patch(ctx, a, base); err != nil {
		return "", err
	}
	return h.deployID, nil
}

func suspendedManagedApp() *appv1alpha1.App {
	a := managedApp("web", "srv-1")
	a.Spec.Suspended = true
	a.Annotations = map[string]string{core.AnnotCrashLoopSuspended: "true"}
	return a
}

func TestResumeAnswersTheDeployItOpened(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, suspendedManagedApp())
	hook := &resumeHook{cl: cl, deployID: "dep-resumed"}
	svc.ResumeDeploy = hook.run

	view, err := svc.Resume(context.Background(), "web")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if view.LatestDeployID != "dep-resumed" || hook.calls != 1 {
		t.Errorf("LatestDeployID = %q after %d hook calls, want the deploy the resume opened", view.LatestDeployID, hook.calls)
	}
	if view.Suspended {
		t.Error("the answer still reads suspended")
	}
	// The row is written once, by the deploy's release write.
	if len(rec.calls) != 1 || rec.calls[0].suspended {
		t.Errorf("row writes = %v, want one resume", rec.calls)
	}
	if got := getApp(t, cl, "web"); got.Spec.Suspended {
		t.Error("the App is still suspended")
	}

	// GraphQL resumeService answers it as latestDeployId.
	if _, err := svc.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	hook.deployID = "dep-resumed-2"
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{"ok": &graphql.Field{Type: graphql.Boolean}}}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	out := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { resumeService(id: "web") { suspended latestDeployId } }`})
	if len(out.Errors) > 0 {
		t.Fatalf("resumeService: %v", out.Errors)
	}
	got := out.Data.(map[string]any)["resumeService"].(map[string]any)
	if got["latestDeployId"] != "dep-resumed-2" || got["suspended"] != "not_suspended" {
		t.Errorf("resumeService = %v, want latestDeployId dep-resumed-2, not_suspended", got)
	}
}

// Where the deploys verb opens nothing, Resume resumes without a deploy, as it
// always has; a failure of its writes is the resume's. Suspend never asks it.
func TestResumeWithoutADeployStillResumes(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, suspendedManagedApp())
	svc.ResumeDeploy = (&resumeHook{cl: cl}).run

	view, err := svc.Resume(context.Background(), "web")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if view.LatestDeployID != "" {
		t.Errorf("LatestDeployID = %q, want none", view.LatestDeployID)
	}
	got := getApp(t, cl, "web")
	if got.Spec.Suspended || got.Annotations[core.AnnotCrashLoopSuspended] != "" {
		t.Errorf("not resumed: suspended %v, annotations %v", got.Spec.Suspended, got.Annotations)
	}
	if len(rec.calls) != 1 || rec.calls[0].suspended {
		t.Errorf("row writes = %v, want one resume", rec.calls)
	}

	failing := &recordingStore{}
	svc, cl = newService(failing, suspendedManagedApp())
	failed := errors.New("patch refused")
	svc.ResumeDeploy = (&resumeHook{cl: cl, err: failed}).run
	if _, err := svc.Resume(context.Background(), "web"); !errors.Is(err, failed) {
		t.Errorf("Resume = %v, want the deploy's write failure", err)
	}
	if len(failing.calls) != 0 || !getApp(t, cl, "web").Spec.Suspended {
		t.Errorf("a failed resume was written again without its deploy: %v", failing.calls)
	}

	hook := &resumeHook{deployID: "dep-x"}
	svc, cl = newService(&recordingStore{}, managedApp("web", "srv-1"))
	hook.cl = cl
	svc.ResumeDeploy = hook.run
	if _, err := svc.Suspend(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	if hook.calls != 0 {
		t.Errorf("Suspend asked the resume deploy verb %d times", hook.calls)
	}
}

// A resume its guards refuse opens no deploy.
func TestARefusedResumeOpensNoDeploy(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	stopped := inProtectedEnvironment(suspendedManagedApp())
	svc, cl := newService(rec, stopped)
	asNonAdmin(svc)
	hook := &resumeHook{cl: cl, deployID: "dep-x"}
	svc.ResumeDeploy = hook.run

	if _, err := svc.Resume(ctxAs("user-a"), "web"); !adminOnlyRefusal(err) {
		t.Fatalf("Resume = %v, want PROTECTED_ENVIRONMENT_ADMIN_REQUIRED", err)
	}
	if hook.calls != 0 || len(rec.calls) != 0 || !getApp(t, cl, "web").Spec.Suspended {
		t.Errorf("a refused resume reached the deploy verb %d times and wrote %v", hook.calls, rec.calls)
	}
}

// Only a member's Resume opens a deploy. The month's allowance lifting its own
// suspension (Free instance hours, included bandwidth) brings the service back
// without one, and an idle wake never reaches bex-api: the activator scales the
// Deployment up itself.
func TestAPlatformResumeOpensNoDeploy(t *testing.T) {
	a := suspendedManagedApp()
	a.Annotations = map[string]string{core.AnnotFreeHoursSuspended: "2026-10"}
	svc, cl := newService(&recordingStore{}, a)
	hook := &resumeHook{cl: cl, deployID: "dep-x"}
	svc.ResumeDeploy = hook.run

	worker := BillingSuspensionWorker{Svc: svc, Annotation: core.AnnotFreeHoursSuspended}
	if err := worker.Resume(context.Background(), getApp(t, cl, "web")); err != nil {
		t.Fatalf("worker Resume: %v", err)
	}
	if hook.calls != 0 {
		t.Errorf("the allowance's resume asked the resume deploy verb %d times", hook.calls)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Error("the allowance's resume did not resume")
	}
}
