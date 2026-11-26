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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w3/m643: Render's resume opens a deploy (its deploy trigger enum carries
// service_resumed, and its dashboard opens the deploy its resumeService
// answers). bex's resume re-releases what the service was serving, and the
// resume itself rides that release write.

// resumeFixture is a suspended service whose deploy dep-live serves web:v1.
func resumeFixture(t *testing.T, kind string) (*Service, *fakeStore, *appv1alpha1.App) {
	t.Helper()
	ds := newFakeStore()
	seedDeploys(ds, store.Deploy{ID: "dep-live", Status: store.DeployLive, Image: "web:v1", ResolvedImage: "web@sha256:live", Generation: 1})
	app := sampleApp("web", "srv-1")
	app.Spec.Type = kind
	app.Spec.Suspended = true
	svc, _ := newService(ds, app)
	return svc, ds, app
}

// resumeCalls counts the resume's own row write and App change, which the
// deploy must carry, and applies the change as apps.setSuspended does.
type resumeCalls struct{ rows, mutates int }

func (c *resumeCalls) write() error { c.rows++; return nil }

func (c *resumeCalls) resume(a *appv1alpha1.App) {
	c.mutates++
	a.Spec.Suspended = false
}

func TestResumeReleaseOpensAServiceResumedDeployOfTheServingRelease(t *testing.T) {
	for _, kind := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeCronJob} {
		t.Run(kind, func(t *testing.T) {
			svc, ds, app := resumeFixture(t, kind)
			calls := &resumeCalls{}
			id, err := svc.ResumeRelease(context.Background(), "web", app, calls.write, calls.resume)
			if err != nil {
				t.Fatalf("ResumeRelease: %v", err)
			}
			if id == "" {
				t.Fatal("ResumeRelease opened no deploy for a suspended service with a serving release")
			}
			d, err := ds.GetDeploy(context.Background(), "srv-1", id)
			if err != nil {
				t.Fatal(err)
			}
			if d.Trigger != store.TriggerServiceResumed || d.Image != "web@sha256:live" {
				t.Errorf("deploy = trigger %q image %q, want service_resumed of the serving image", d.Trigger, d.Image)
			}
			if got := renderTrigger(d.Trigger); got != "service_resumed" {
				t.Errorf("REST trigger = %q, want Render's service_resumed", got)
			}
			if calls.rows != 1 || calls.mutates == 0 {
				t.Errorf("the resume's row write ran %d times and its App change %d, want both in the release write", calls.rows, calls.mutates)
			}
			got := getApp(t, svc.Client, "web")
			if got.Spec.Suspended {
				t.Error("the App is still suspended: the resume did not ride the release write")
			}
			if got.Spec.ReleaseConfig == nil || got.Spec.ReleaseConfig.Image != "web@sha256:live" {
				t.Errorf("release selection = %+v, want the serving release's image", got.Spec.ReleaseConfig)
			}
		})
	}
}

// A resume opens no deploy, and writes nothing, where it has no release to
// re-release or a deploy in flight would bring the service back itself.
func TestResumeReleaseOpensNothingWithoutAReleaseToResume(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []store.Deploy
	}{
		{"never live", []store.Deploy{{ID: "dep-failed", Status: store.DeployBuildFailed, Generation: 1}}},
		{"a deploy in flight", []store.Deploy{
			{ID: "dep-building", Status: store.DeployBuildInProgress, Generation: 2},
			{ID: "dep-live", Status: store.DeployLive, ResolvedImage: "web@sha256:live", Generation: 1},
		}},
		{"a deploy queued", []store.Deploy{
			{ID: "dep-queued", Status: store.DeployQueued, Generation: 2},
			{ID: "dep-live", Status: store.DeployLive, ResolvedImage: "web@sha256:live", Generation: 1},
		}},
		{"a live release with no recorded artifact", []store.Deploy{{ID: "dep-live", Status: store.DeployLive, Generation: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ds := newFakeStore()
			seedDeploys(ds, tc.rows...)
			app := sampleApp("web", "srv-1")
			app.Spec.Suspended = true
			svc, cl := newService(ds, app)
			calls := &resumeCalls{}
			id, err := svc.ResumeRelease(context.Background(), "web", app, calls.write, calls.resume)
			if err != nil || id != "" {
				t.Fatalf("ResumeRelease = %q, %v; want no deploy and no error", id, err)
			}
			if calls.rows != 0 || calls.mutates != 0 {
				t.Errorf("the resume was written (%d row writes, %d App changes); the caller resumes without a deploy", calls.rows, calls.mutates)
			}
			if n := len(ds.byApp["srv-1"]); n != len(tc.rows) {
				t.Errorf("%d deploy rows, want the %d seeded", n, len(tc.rows))
			}
			if got := getApp(t, cl, "web"); !got.Spec.Suspended || got.Spec.ReleaseConfig != nil {
				t.Errorf("the App changed: suspended %v, release %+v", got.Spec.Suspended, got.Spec.ReleaseConfig)
			}
		})
	}

	// A serving release that cannot be read (status names a release no
	// deploy records) leaves the resume to the caller, as a Restart refuses.
	ds := newFakeStore()
	seedDeploys(ds, store.Deploy{ID: "dep-live", Status: store.DeployLive, ResolvedImage: "web@sha256:live", Generation: 1})
	untracked := sampleApp("web", "srv-1")
	untracked.Spec.Suspended = true
	untracked.Status.ActiveRevision, untracked.Status.ReleaseGeneration = "rev-5", 5
	svc, cl := newService(ds, untracked)
	calls := &resumeCalls{}
	if id, err := svc.ResumeRelease(context.Background(), "web", untracked, calls.write, calls.resume); err != nil || id != "" || calls.rows != 0 {
		t.Errorf("ResumeRelease of an unreadable serving release = %q, %v after %d row writes; want none", id, err, calls.rows)
	}
	if got := getApp(t, cl, "web"); !got.Spec.Suspended {
		t.Error("the App was resumed without its caller")
	}

	// A hand-applied App has no row, so no deploy history.
	svc, _, app := resumeFixture(t, appv1alpha1.TypeWebService)
	delete(app.Labels, store.LabelAppID)
	if id, err := svc.ResumeRelease(context.Background(), "web", app, (&resumeCalls{}).write, (&resumeCalls{}).resume); err != nil || id != "" {
		t.Errorf("ResumeRelease of an unmanaged App = %q, %v; want none", id, err)
	}
}

// losingLiveReads answers ListDeploys from the wrapped store until the
// serving (live-filtered) read that the locked selection makes, then either
// fails it or answers empty. The unlocked selectServing and the rowWrite
// re-check both pass, so the row write runs; only the selection the release
// write would use is lost — a transient read failure, or the operator moving
// the serving release on between the two locked reads.
type losingLiveReads struct {
	DeployStore
	live int
	fail error
}

func (s *losingLiveReads) ListDeploys(ctx context.Context, appID string, filter store.DeployFilter) ([]store.Deploy, error) {
	if len(filter.Statuses) == 1 && filter.Statuses[0] == store.DeployLive {
		s.live++
		if s.live == 4 {
			if s.fail != nil {
				return nil, s.fail
			}
			return nil, nil
		}
	}
	return s.DeployStore.ListDeploys(ctx, appID, filter)
}

// A serving release that stops being selectable after the resume's row write
// leaves the resume to the caller — "", nil with the row written, so the
// caller resumes without a deploy — instead of failing the resume after its
// row already flipped to resumed while the App still reads suspended.
func TestResumeReleaseFallsBackWhenSelectionIsLostAfterTheRowWrite(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail error
	}{
		{"read fails under the lock", errors.New("boom")},
		{"release moved on under the lock", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ds := newFakeStore()
			seedDeploys(ds, store.Deploy{ID: "dep-live", Status: store.DeployLive, Image: "web:v1", ResolvedImage: "web@sha256:live", Generation: 1})
			app := sampleApp("web", "srv-1")
			app.Spec.Suspended = true
			svc, cl := newService(&losingLiveReads{DeployStore: ds, fail: tc.fail}, app)
			calls := &resumeCalls{}
			id, err := svc.ResumeRelease(context.Background(), "web", app, calls.write, calls.resume)
			if err != nil || id != "" {
				t.Fatalf("ResumeRelease = %q, %v; want no deploy and no error", id, err)
			}
			if calls.rows != 1 {
				t.Errorf("the resume's row write ran %d times, want 1: the caller resumes on the written row", calls.rows)
			}
			if n := len(ds.byApp["srv-1"]); n != 1 {
				t.Errorf("%d deploy rows, want the 1 seeded: no service_resumed row opened", n)
			}
			if got := getApp(t, cl, "web"); !got.Spec.Suspended || got.Spec.ReleaseConfig != nil {
				t.Errorf("the App changed: suspended %v, release %+v; the caller applies the resume", got.Spec.Suspended, got.Spec.ReleaseConfig)
			}
		})
	}
}
