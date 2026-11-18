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

package rollout

import (
	"context"
	"errors"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// liveStore answers the deploy-history reads by status, as the store does:
// ServedRelease's live row and DeployInFlight's open ones.
type liveStore struct {
	fakeDeployStore
	live []store.Deploy
	err  error
}

func (l *liveStore) ListDeploys(_ context.Context, _ string, filter store.DeployFilter) ([]store.Deploy, error) {
	var rows []store.Deploy
	for _, d := range l.live {
		if len(filter.Statuses) == 0 || slices.Contains(filter.Statuses, d.Status) {
			rows = append(rows, d)
		}
	}
	return rows, l.err
}

const (
	savedTag     = "docker.io/library/busybox:1.36"
	savedDigest  = "docker.io/library/busybox@sha256:73aaf090"
	overrideTag  = "docker.io/library/busybox:1.37"
	overrideDgst = "docker.io/library/busybox@sha256:a7dbc319"
)

func imageApp() *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Generation: 5, Labels: map[string]string{
			store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-web",
		}},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Image: savedTag},
	}
}

// w4/m418: a configuration-only release of an image-backed service reruns the
// image it serves when that is not the saved one (an imageUrl override), and
// selects nothing when it serves the saved tag, has no live release, or holds
// its change.
func TestSelectServedImage(t *testing.T) {
	overrideRow := store.Deploy{Status: store.DeployLive, Image: overrideTag, ResolvedImage: overrideDgst}
	savedRow := store.Deploy{Status: store.DeployLive, Image: savedTag, ResolvedImage: savedDigest}
	// A Restart row records the digest it selected as its image.
	restartedSaved := store.Deploy{Status: store.DeployLive, Image: savedDigest, ResolvedImage: savedDigest}
	for _, tc := range []struct {
		name   string
		live   []store.Deploy
		noHist bool
		tweak  func(*appv1alpha1.App)
		want   string
	}{
		{name: "an override is served", live: []store.Deploy{overrideRow}, want: overrideDgst},
		{name: "a Restart of the override is served", live: []store.Deploy{{Status: store.DeployLive, Image: overrideDgst, ResolvedImage: overrideDgst}}, want: overrideDgst},
		{name: "the saved tag is served", live: []store.Deploy{savedRow}},
		{name: "a configured-port service serves the tag as written", live: []store.Deploy{{Status: store.DeployLive, Image: savedTag, ResolvedImage: savedTag}}},
		{name: "a Restart of the saved tag is served", live: []store.Deploy{restartedSaved}, tweak: func(a *appv1alpha1.App) {
			a.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: savedDigest, Source: savedTag}
		}},
		{name: "no live release", live: nil},
		{name: "no deploy history", noHist: true, live: []store.Deploy{overrideRow}},
		{name: "a suspended service", live: []store.Deploy{overrideRow}, tweak: func(a *appv1alpha1.App) { a.Spec.Suspended = true }},
		{name: "a Git-backed service", live: []store.Deploy{overrideRow}, tweak: func(a *appv1alpha1.App) { a.Spec.Repo = "https://github.com/bex-co/bex" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := imageApp()
			if tc.tweak != nil {
				tc.tweak(a)
			}
			tr := &Tracker{Store: &liveStore{live: tc.live}}
			if tc.noHist {
				tr = &Tracker{Store: &fakeDeployStore{}}
			}
			if err := tr.SelectServedImage(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if a.Spec.ReleaseConfig != nil {
					t.Fatalf("selected %+v, want nothing", a.Spec.ReleaseConfig)
				}
				return
			}
			want := appv1alpha1.ReleaseConfigReference{Generation: 6, Image: tc.want}
			if a.Spec.ReleaseConfig == nil || *a.Spec.ReleaseConfig != want {
				t.Fatalf("selected %+v, want %+v", a.Spec.ReleaseConfig, want)
			}
		})
	}
}

// A failed history read is not "nothing serving": the caller must not roll
// the saved tag on a guess.
func TestSelectServedImageReportsAFailedRead(t *testing.T) {
	boom := errors.New("db down")
	tr := &Tracker{Store: &liveStore{err: boom}}
	if err := tr.SelectServedImage(context.Background(), imageApp()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

const servedBuild = "zot.bex-registry.svc:5000/t/web:gen-2@sha256:4d8c"

func sourceApp() *appv1alpha1.App {
	a := imageApp()
	a.Spec.Image, a.Spec.Repo = "", "https://github.com/bex-co/bex"
	return a
}

// w4/m180, w4/708: a configuration-only release reruns the build a
// source-built service serves; a prebuilt image keeps SelectServedImage's
// rule. A static site, unknown history and a held (suspended) change select
// nothing, and a source-built service with no live build has nothing to rerun.
func TestSelectServedBuild(t *testing.T) {
	built := store.Deploy{Status: store.DeployLive, ResolvedImage: servedBuild, Commit: "936b61bab203"}
	overrideRow := store.Deploy{Status: store.DeployLive, Image: overrideTag, ResolvedImage: overrideDgst}
	for _, tc := range []struct {
		name    string
		app     func() *appv1alpha1.App
		live    []store.Deploy
		noHist  bool
		tweak   func(*appv1alpha1.App)
		want    string
		wantErr error
	}{
		{name: "a source-built service reruns its served build", app: sourceApp, live: []store.Deploy{built}, want: servedBuild},
		{name: "a source-built service with no live build", app: sourceApp, wantErr: ErrNoLiveBuild},
		{name: "a static site rebuilds", app: sourceApp, live: []store.Deploy{built}, tweak: func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeStaticSite }},
		{name: "a suspended source-built service", app: sourceApp, live: []store.Deploy{built}, tweak: func(a *appv1alpha1.App) { a.Spec.Suspended = true }},
		{name: "unknown history", app: sourceApp, noHist: true},
		{name: "an image service reruns its override", app: imageApp, live: []store.Deploy{overrideRow}, want: overrideDgst},
		{name: "an image service with no live release deploys its saved tag", app: imageApp},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := tc.app()
			if tc.tweak != nil {
				tc.tweak(a)
			}
			tr := &Tracker{Store: &liveStore{live: tc.live}}
			if tc.noHist {
				tr = &Tracker{Store: &fakeDeployStore{}}
			}
			if err := tr.SelectServedBuild(context.Background(), a); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.want == "" {
				if a.Spec.ReleaseConfig != nil {
					t.Fatalf("selected %+v, want nothing", a.Spec.ReleaseConfig)
				}
				return
			}
			want := appv1alpha1.ReleaseConfigReference{Generation: 6, Image: tc.want}
			if a.Spec.ReleaseConfig == nil || *a.Spec.ReleaseConfig != want {
				t.Fatalf("selected %+v, want %+v", a.Spec.ReleaseConfig, want)
			}
		})
	}
}

// A deploy executing or queued is in flight; a live or finished one is not,
// and with no history nothing is known to be.
func TestDeployInFlight(t *testing.T) {
	live := store.Deploy{Status: store.DeployLive, ResolvedImage: servedBuild}
	for _, tc := range []struct {
		name string
		rows []store.Deploy
		want bool
	}{
		{name: "only a live deploy", rows: []store.Deploy{live, {Status: store.DeployCanceled}}},
		{name: "a build in progress", rows: []store.Deploy{live, {Status: store.DeployBuildInProgress}}, want: true},
		{name: "a queued deploy", rows: []store.Deploy{live, {Status: store.DeployQueued}}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := &Tracker{Store: &liveStore{live: tc.rows}}
			if got, err := tr.DeployInFlight(context.Background(), sourceApp()); err != nil || got != tc.want {
				t.Fatalf("DeployInFlight = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	if got, err := (&Tracker{Store: &fakeDeployStore{}}).DeployInFlight(context.Background(), sourceApp()); err != nil || got {
		t.Fatalf("no history: DeployInFlight = %v, %v; want false", got, err)
	}
	boom := errors.New("db down")
	if _, err := (&Tracker{Store: &liveStore{err: boom}}).DeployInFlight(context.Background(), sourceApp()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}
