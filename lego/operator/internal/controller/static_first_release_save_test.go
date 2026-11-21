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

package controller

import (
	"context"
	"errors"
	"maps"
	"strconv"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// savedAfterFirstRelease is a static site whose first release (generation 1)
// published while two saves (generations 2 and 3) waited for a deploy: the
// state production's first deploy was stranded in (w8/m460).
func savedAfterFirstRelease() *appv1alpha1.App {
	spec := appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/octocat/Spoon-Knife", Branch: "main",
		BuildCommand: "mkdir -p out && echo m > out/marker.txt", PublishPath: "out",
	}
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 3, Annotations: map[string]string{
			appv1alpha1.AnnotationReleaseGeneration:       "1",
			appv1alpha1.AnnotationPendingSourceGeneration: "3",
		}},
		Spec: spec,
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseDeploying, ReleaseGeneration: 1, ActiveRevision: "rev-1",
			ReleaseFingerprint:         desiredAppReleaseIdentity(spec).release,
			ReleaseSettingsFingerprint: releaseSettingsFingerprint(spec),
			PendingReleaseGeneration:   2,
		},
	}
}

// The pending slot coalesced the first save while the release built. Once the
// release settled, that generation is only the save, which waits for a deploy:
// left in the slot it requeued the settled release every second.
func TestSettledReleaseOwesNothingForASave(t *testing.T) {
	app := savedAfterFirstRelease()
	decision := prepareAppReleaseDecision(app, nil)
	if !decision.sourcePending || decision.pinned {
		t.Fatalf("decision = %+v, want the save to wait behind the settled release", decision)
	}
	if app.Status.PendingReleaseGeneration != 0 {
		t.Fatalf("pendingReleaseGeneration = %d, want 0: a save owes no release", app.Status.PendingReleaseGeneration)
	}
	if _, owed := owedReleaseRequeue(app); owed {
		t.Fatal("a settled release behind a save must not requeue as owed")
	}
}

func TestStaticReleaseFinishing(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*appv1alpha1.App)
		want bool
	}{
		{"first release published, host not handed over", func(*appv1alpha1.App) {}, true},
		{"served and running", func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseRunning }, true},
		{"direct first publish uploading", func(a *appv1alpha1.App) { a.Status.ActiveRevision = "" }, true},
		{"direct first publish cloning", func(a *appv1alpha1.App) {
			a.Status.ActiveRevision, a.Status.Phase = "", appv1alpha1.PhaseBuilding
		}, true},
		{"newer direct release publishing over a served one", func(a *appv1alpha1.App) { a.Status.ReleaseGeneration = 2 }, true},
		{"built release whose image is not built yet", func(a *appv1alpha1.App) {
			a.Status.ActiveRevision, a.Status.Phase = "", appv1alpha1.PhaseBuilding
			a.Status.ReleaseArtifactFingerprint = "artifact-v1:building"
		}, false},
		{"failed first release", func(a *appv1alpha1.App) {
			a.Status.ActiveRevision, a.Status.Phase = "", appv1alpha1.PhaseFailed
		}, false},
		{"newer release failed over a served one", func(a *appv1alpha1.App) {
			a.Status.ReleaseGeneration, a.Status.Phase = 2, appv1alpha1.PhaseRunning
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := savedAfterFirstRelease()
			tc.edit(app)
			if got := staticReleaseFinishing(app); got != tc.want {
				t.Fatalf("staticReleaseFinishing = %v, want %v (status %+v)", got, tc.want, app.Status)
			}
		})
	}
}

// A static site keeps no build-input records, so its undeployed changes read
// the saved settings against the release's own settings fingerprint while a
// save waits.
func TestStaticSaveWaits(t *testing.T) {
	// restamped is bex-api's root or publish directory save, which stamps a
	// rebuild (SetRootDir, SetPublishPath).
	restamped := func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-11-19T09:00:00Z" }
	for _, tc := range []struct {
		name string
		edit func(*appv1alpha1.App)
		want bool
	}{
		{"saves that net out to the release's own settings", func(*appv1alpha1.App) {}, false},
		// w8/339: a root directory set and cleared left only bex-api's stamp.
		{"a set-and-clear that left only a rebuild stamp", restamped, false},
		{"a saved publish directory", func(a *appv1alpha1.App) { a.Spec.PublishPath = "out2"; restamped(a) }, true},
		{"a saved root directory", func(a *appv1alpha1.App) { a.Spec.RootDir = "sub"; restamped(a) }, true},
		{"an operational change", func(a *appv1alpha1.App) { a.Spec.DisplayName = "renamed" }, false},
		{"a release an older operator dispatched compares its whole identity", func(a *appv1alpha1.App) {
			a.Status.ReleaseSettingsFingerprint = ""
			restamped(a)
		}, true},
		{"an older operator's release whose settings net out", func(a *appv1alpha1.App) {
			a.Status.ReleaseSettingsFingerprint = ""
		}, false},
		{"a deploy consumed the save", func(a *appv1alpha1.App) {
			a.Spec.PublishPath = "out2"
			a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "3"
		}, false},
		{"nothing served yet", func(a *appv1alpha1.App) {
			a.Spec.PublishPath = "out2"
			a.Status.ActiveRevision = ""
		}, false},
		{"a release-v1 fingerprint cannot be compared", func(a *appv1alpha1.App) {
			a.Status.ReleaseFingerprint, a.Status.ReleaseSettingsFingerprint = legacyReleaseFingerprint(a.Spec), ""
			a.Spec.PublishPath = "out2"
		}, false},
		{"a settings fingerprint of another identity version cannot be compared", func(a *appv1alpha1.App) {
			a.Status.ReleaseSettingsFingerprint = "release-v3:0"
			a.Spec.PublishPath = "out2"
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := savedAfterFirstRelease()
			tc.edit(app)
			if got := staticSaveWaits(app); got != tc.want {
				t.Fatalf("staticSaveWaits = %v, want %v", got, tc.want)
			}
		})
	}
}

// w32/049: a static Restart restores the publication serving now (bex-api
// selects its retained prefix). Its request consumes the pending-save marker,
// but the restored release published none of the saved settings, so it keeps
// the served settings fingerprint and the save keeps waiting until a deploy
// publishes it.
func TestStaticRestartKeepsTheServedSettings(t *testing.T) {
	spec := appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/bex", Branch: "main",
		RootDir: "examples/static-site", BuildCommand: "mkdir -p dist && echo QA128F-PUB > dist/marker.txt", PublishPath: "dist",
	}
	served := releaseSettingsFingerprint(spec)
	// Serving release 2 from site/rev-2/; release 3 is a save of a bad root
	// directory, waiting for a deploy.
	saved := func() *appv1alpha1.App {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "site", Generation: 3, Annotations: map[string]string{
				appv1alpha1.AnnotationReleaseGeneration:       "2",
				appv1alpha1.AnnotationPendingSourceGeneration: "3",
			}},
			Spec: spec,
			Status: appv1alpha1.AppStatus{
				Phase: appv1alpha1.PhaseRunning, ReleaseGeneration: 2, ActiveRevision: "rev-2", StaticPrefix: "site/rev-2/",
				ReleaseFingerprint: desiredAppReleaseIdentity(spec).release, ReleaseSettingsFingerprint: served,
			},
		}
		app.Spec.RootDir, app.Spec.RestartedAt = "examples/nope-128f", "2026-11-20T00:49:00Z"
		return app
	}
	restore := func(app *appv1alpha1.App, prefix string) {
		app.Generation = 4
		app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "4"
		app.Spec.RestartedAt = "2026-11-20T00:51:44Z"
		app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 4, Image: "zot/site:gen-2@sha256:bb", StaticPrefix: prefix}
		if d := prepareAppReleaseDecision(app, nil); !d.releaseChanged || app.Status.ReleaseGeneration != 4 {
			t.Fatalf("decision = %+v (release %d), want the restore adopted as release 4", d, app.Status.ReleaseGeneration)
		}
	}

	app := saved()
	if !staticSaveWaits(app) {
		t.Fatal("the save must wait before the restart")
	}
	restore(app, "site/rev-2/")
	if app.Status.ReleaseSettingsFingerprint != served {
		t.Fatal("a restart that re-serves the served publication must keep the served settings")
	}
	if !staticSaveWaits(app) {
		t.Fatal("the save must still wait while the restart copies the served publication")
	}
	app.Status.ActiveRevision, app.Status.StaticPrefix = "rev-4", "site/rev-4/"
	if !staticSaveWaits(app) || undeployedChangesReason(app, true, false) != appv1alpha1.UndeployedReasonSaved {
		t.Fatal("the save must still read as a saved, undeployed change once the restart serves")
	}

	// The next deploy publishes the save.
	app.Generation = 5
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "5"
	app.Spec.ReleaseConfig, app.Spec.RestartedAt = nil, "2026-11-20T00:58:00Z"
	prepareAppReleaseDecision(app, nil)
	if staticSaveWaits(app) || app.Status.ReleaseSettingsFingerprint != releaseSettingsFingerprint(app.Spec) {
		t.Fatal("a deploy that publishes the save must record its settings and end the wait")
	}

	// Control: a rollback to another release's publication is not a restart.
	// The decision records the saved settings, and the reconciler then
	// carries the restored publication's from its record
	// (TestStaticRollbackCarriesTheRestoredSettings, w32/055).
	app = saved()
	restore(app, "site/rev-1/")
	if app.Status.ReleaseSettingsFingerprint != releaseSettingsFingerprint(app.Spec) {
		t.Fatal("a rollback to another publication must record the saved settings")
	}
}

// w32/055: a static Rollback restores another release's publication, built
// with that release's settings. The reconciler carries the settings its
// record holds, so saved settings that differ from the target's wait for the
// next deploy, which Render runs with the current configuration, whether or
// not a save was pending. A record written before w32/055 holds none: the
// rollback then carries the settings serving when it was requested.
func TestStaticRollbackCarriesTheRestoredSettings(t *testing.T) {
	ctx := context.Background()
	spec := func(marker string) appv1alpha1.AppSpec {
		return appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/bex", Branch: "main", RootDir: "examples/static-site",
			BuildCommand: "mkdir -p dist && echo " + marker + " > dist/marker.txt", PublishPath: "dist",
		}
	}
	// Release 1 published PUB1, release 2 serves PUB2, and PUB3 is a save
	// waiting for a deploy.
	target, served, pending := spec("PUB1"), spec("PUB2"), spec("PUB3")
	const uid = types.UID("site-uid")
	record := func(owner types.UID, prefix, settings string) *corev1.Secret {
		rec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.StaticPublicationRecordName("site", 1), Namespace: snapshotTestNS,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "App", Name: "site", UID: owner, Controller: new(true)}}},
			Data: map[string][]byte{appv1alpha1.StaticPublicationPrefixKey: []byte(prefix)},
		}
		if settings != "" {
			rec.Data[staticPublicationSettingsKey] = []byte(settings)
		}
		return rec
	}
	recorded := record(uid, "site/rev-1/", releaseSettingsFingerprint(target))
	// rollback adopts release 4, which restores prefix, over the saved spec.
	rollback := func(t *testing.T, saved appv1alpha1.AppSpec, prefix string, records ...client.Object) (*appv1alpha1.App, error) {
		t.Helper()
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: snapshotTestNS, UID: uid, Generation: 4, Annotations: map[string]string{
				appv1alpha1.AnnotationReleaseGeneration:       "4",
				appv1alpha1.AnnotationPendingSourceGeneration: "3",
			}},
			Spec: saved,
			Status: appv1alpha1.AppStatus{
				Phase: appv1alpha1.PhaseRunning, ReleaseGeneration: 2, ActiveRevision: "rev-2", StaticPrefix: "site/rev-2/",
				ReleaseFingerprint:         desiredAppReleaseIdentity(served).release,
				ReleaseSettingsFingerprint: releaseSettingsFingerprint(served), ServedSettingsFingerprint: releaseSettingsFingerprint(served),
			},
		}
		app.Spec.RestartedAt = "2026-11-20T05:43:18Z"
		app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 4, Image: "zot/site:gen-1@sha256:aa", StaticPrefix: prefix}
		scheme := deletionScheme(t)
		r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(records...).Build(), Scheme: scheme}
		if d := prepareAppReleaseDecision(app, nil); !d.releaseChanged || app.Status.ReleaseGeneration != 4 {
			t.Fatalf("decision = %+v (release %d), want the rollback adopted as release 4", d, app.Status.ReleaseGeneration)
		}
		return app, r.carryRestoredPublicationSettings(ctx, app)
	}
	// waits reads the saved-change status while the copy runs and once it serves.
	waits := func(t *testing.T, app *appv1alpha1.App) bool {
		t.Helper()
		during := staticSaveWaits(app)
		recordStaticRevision(app, "rev-4", true)
		if after := staticSaveWaits(app); after != during {
			t.Fatalf("the save waits %v while the copy runs but %v once it serves", during, after)
		}
		if during && undeployedChangesReason(app, true, false) != appv1alpha1.UndeployedReasonSaved {
			t.Fatalf("reason = %q, want saved", undeployedChangesReason(app, true, false))
		}
		return during
	}

	for _, tc := range []struct {
		name    string
		saved   appv1alpha1.AppSpec
		records []client.Object
		carries appv1alpha1.AppSpec
		waits   bool
	}{
		{"a pending save", pending, []client.Object{recorded}, target, true},
		{"no pending save, the target released with other settings", served, []client.Object{recorded}, target, true},
		{"no pending save, the target released with the saved settings", target, []client.Object{recorded}, target, false},
		{"a pending save, the target's record from before w32/055", pending, []client.Object{record(uid, "site/rev-1/", "")}, served, true},
		{"no pending save, the target's record from before w32/055", served, []client.Object{record(uid, "site/rev-1/", "")}, served, false},
		{"a pending save, a record another App left", pending, []client.Object{record("other-uid", "site/rev-1/", releaseSettingsFingerprint(target))}, served, true},
		{"a pending save, a record of another prefix", pending, []client.Object{record(uid, "site/rev-9/", releaseSettingsFingerprint(target))}, served, true},
		{"no pending save, no record", served, nil, served, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, err := rollback(t, tc.saved, "site/rev-1/", tc.records...)
			if err != nil {
				t.Fatal(err)
			}
			if app.Status.ReleaseSettingsFingerprint != releaseSettingsFingerprint(tc.carries) {
				t.Fatalf("the rollback carries %s, want %s", app.Status.ReleaseSettingsFingerprint, releaseSettingsFingerprint(tc.carries))
			}
			if got := waits(t, app); got != tc.waits {
				t.Fatalf("staticSaveWaits = %v, want %v", got, tc.waits)
			}
			if app.Status.ServedSettingsFingerprint != releaseSettingsFingerprint(tc.carries) {
				t.Fatal("the restored publication must serve with the settings the rollback carried")
			}
		})
	}

	t.Run("the next deploy publishes the saved settings", func(t *testing.T) {
		app, err := rollback(t, pending, "site/rev-1/", recorded)
		if err != nil || !waits(t, app) {
			t.Fatalf("precondition: the save waits after the rollback (err %v)", err)
		}
		// bex-api's deploy drops the selection (deploys.Service.Trigger).
		app.Generation = 5
		app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "5"
		app.Spec.ReleaseConfig, app.Spec.RestartedAt = nil, "2026-11-20T05:50:00Z"
		prepareAppReleaseDecision(app, nil)
		recordStaticRevision(app, "rev-5", true)
		if staticSaveWaits(app) {
			t.Fatal("a deploy that publishes the save must end the wait")
		}
	})

	t.Run("a restart keeps the served settings without reading a record", func(t *testing.T) {
		app, err := rollback(t, pending, "site/rev-2/", record(uid, "site/rev-2/", releaseSettingsFingerprint(target)))
		if err != nil {
			t.Fatal(err)
		}
		if app.Status.ReleaseSettingsFingerprint != releaseSettingsFingerprint(served) {
			t.Fatal("a restart of the served publication must carry the served settings")
		}
	})

	t.Run("a failed read fails the pass", func(t *testing.T) {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: snapshotTestNS, UID: uid},
			Spec:       pending,
			Status:     appv1alpha1.AppStatus{ReleaseGeneration: 4, SelectedReleaseGeneration: 4, ActiveRevision: "rev-2", StaticPrefix: "site/rev-2/"},
		}
		app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 4, StaticPrefix: "site/rev-1/"}
		scheme := deletionScheme(t)
		cl := interceptor.NewClient(fake.NewClientBuilder().WithScheme(scheme).Build(), interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return errors.New("apiserver unavailable")
			},
		})
		r := &AppReconciler{Client: cl, Scheme: scheme}
		if err := r.carryRestoredPublicationSettings(ctx, app); err == nil {
			t.Fatal("an unreadable record must not be taken for one with no settings")
		}
	})
}

// Each publication record keeps the settings its publication was released
// with (w32/055). The release that just published states its own; a backfill
// adds the served settings only to this App's record of the same prefix that
// was written without them, and replaces nothing.
func TestStaticPublicationRecordsKeepTheirSettings(t *testing.T) {
	ctx := context.Background()
	scheme := deletionScheme(t)
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: snapshotTestNS, UID: "site-uid"}}
	owned := func(gen int64, data map[string]string, owner types.UID) *corev1.Secret {
		rec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.StaticPublicationRecordName("site", gen), Namespace: snapshotTestNS,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "App", Name: "site", UID: owner, Controller: new(true)}}},
			Data: map[string][]byte{}}
		for k, v := range data {
			rec.Data[k] = []byte(v)
		}
		return rec
	}
	prefixKey, settingsKey := appv1alpha1.StaticPublicationPrefixKey, staticPublicationSettingsKey
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		owned(1, map[string]string{prefixKey: "site/rev-1/"}, app.UID),
		owned(2, map[string]string{prefixKey: "site/rev-2/", settingsKey: "release-v2:old"}, app.UID),
		owned(3, map[string]string{prefixKey: "site/rev-3/"}, "other-uid"),
		owned(4, map[string]string{prefixKey: "site/rev-9/"}, app.UID),
	).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}
	for _, w := range []struct {
		gen       int64
		settings  string
		overwrite bool
	}{
		{1, "release-v2:s1", false}, {2, "release-v2:s2", false}, {3, "release-v2:s3", false}, {4, "release-v2:s4", false},
		{5, "release-v2:s5", false}, {6, "", false}, {7, "release-v2:s7", true},
	} {
		if err := r.writeStaticPublicationRecord(ctx, app, w.gen, "site/rev-"+strconv.FormatInt(w.gen, 10)+"/", w.settings, w.overwrite); err != nil {
			t.Fatal(err)
		}
	}
	for gen, want := range map[int64]map[string]string{
		1: {prefixKey: "site/rev-1/", settingsKey: "release-v2:s1"},  // an older record gains its settings
		2: {prefixKey: "site/rev-2/", settingsKey: "release-v2:old"}, // a recorded setting is never replaced
		3: {prefixKey: "site/rev-3/"},                                // another App's record is left alone
		4: {prefixKey: "site/rev-9/"},                                // so is one naming another prefix
		5: {prefixKey: "site/rev-5/", settingsKey: "release-v2:s5"},  // a missing record is created with them
		6: {prefixKey: "site/rev-6/"},                                // unknown settings are not recorded
		7: {prefixKey: "site/rev-7/", settingsKey: "release-v2:s7"},  // the release that just published
	} {
		rec := &corev1.Secret{}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: snapshotTestNS, Name: appv1alpha1.StaticPublicationRecordName("site", gen)}, rec); err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for k, v := range rec.Data {
			got[k] = string(v)
		}
		if !maps.Equal(got, want) {
			t.Errorf("record %d = %v, want %v", gen, got, want)
		}
	}
}

// w32/053: a deploy whose build or publication failed, or that was canceled,
// published none of the settings it was dispatched with, and its request
// consumed the pending-save marker. The saved settings are then compared with
// the publication still serving: a save that never went live waits, and one
// put back to the live settings waits for nothing.
func TestStaticSaveAfterAnUnpublishedRelease(t *testing.T) {
	spec := appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/bex", Branch: "main",
		RootDir: "examples/static-site", BuildCommand: "mkdir -p out2 && echo QA4AA5-S2 > out2/index.html", PublishPath: "out2",
	}
	served := releaseSettingsFingerprint(spec)
	badBranch := func(s *appv1alpha1.AppSpec) { s.Branch = "qa-no-such-branch-4aa5" }
	// failedDeploy is the repro's steps 1-3: release 1 serves rev-1; the bad
	// branch is saved (generation 2) and deployed (generation 3), and release
	// 3 is adopted and dispatched. fail records its verdict; nil leaves it
	// building.
	failedDeploy := func(t *testing.T, fail func(*appv1alpha1.App)) *appv1alpha1.App {
		t.Helper()
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "site", Generation: 3, Annotations: map[string]string{
				appv1alpha1.AnnotationReleaseGeneration:       "3",
				appv1alpha1.AnnotationPendingSourceGeneration: "2",
			}},
			Spec: spec,
			Status: appv1alpha1.AppStatus{
				Phase: appv1alpha1.PhaseRunning, ReleaseGeneration: 1, ActiveRevision: "rev-1", StaticPrefix: "site/rev-1/",
				ReleaseFingerprint: desiredAppReleaseIdentity(spec).release, ReleaseSettingsFingerprint: served,
				ServedSettingsFingerprint: served,
			},
		}
		badBranch(&app.Spec)
		app.Spec.RestartedAt = "2026-11-20T02:41:00Z"
		if d := prepareAppReleaseDecision(app, nil); !d.releaseChanged || app.Status.ReleaseGeneration != 3 {
			t.Fatalf("decision = %+v (release %d), want release 3 dispatched", d, app.Status.ReleaseGeneration)
		}
		app.Status.Phase = appv1alpha1.PhaseBuilding
		if fail != nil {
			app.Status.Phase = appv1alpha1.PhaseRunning
			fail(app)
		}
		return app
	}
	buildFailed := func(app *appv1alpha1.App) {
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionBuild, Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonBuildFailedUserError,
			ObservedGeneration: app.Status.ReleaseGeneration,
		})
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: reasonPriorReleaseServing,
			ObservedGeneration: app.Generation,
		})
	}
	// saveAgain is a later save without a deploy (bex services update).
	saveAgain := func(app *appv1alpha1.App, mutate func(*appv1alpha1.AppSpec)) {
		app.Generation++
		app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] = strconv.FormatInt(app.Generation, 10)
		mutate(&app.Spec)
		prepareAppReleaseDecision(app, nil)
	}
	live := func(s *appv1alpha1.AppSpec) { s.Branch, s.BuildCommand = spec.Branch, spec.BuildCommand }

	for _, tc := range []struct {
		name string
		fail func(*appv1alpha1.App)
	}{
		{"its build failed", buildFailed},
		{"its publication failed", func(app *appv1alpha1.App) {
			meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
				Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: reasonPublishFailed,
				ObservedGeneration: app.Status.ReleaseGeneration,
			})
		}},
		{"it failed outright", func(app *appv1alpha1.App) { app.Status.Phase = appv1alpha1.PhaseFailed }},
		{"its deploy was canceled", func(app *appv1alpha1.App) {
			app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "3"
			prepareAppReleaseDecision(app, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := failedDeploy(t, tc.fail)
			if !staticSaveWaits(app) || undeployedChangesReason(app, true, false) != appv1alpha1.UndeployedReasonSaved {
				t.Fatal("C: the saved branch never served, so it still waits as a saved change")
			}
			saveAgain(app, live)
			if staticSaveWaits(app) {
				t.Fatal("D: every saved setting is the live release's again, so nothing waits")
			}
			saveAgain(app, func(s *appv1alpha1.AppSpec) { s.BuildCommand = "echo QA4AA5 fail; exit 3" })
			if !staticSaveWaits(app) {
				t.Fatal("a later save that differs from the live release waits")
			}
		})
	}

	t.Run("the deploy in flight carries the saved settings", func(t *testing.T) {
		if staticSaveWaits(failedDeploy(t, nil)) {
			t.Fatal("a building release of the saved settings leaves nothing waiting")
		}
	})
	t.Run("an earlier release's failure does not condemn the one building", func(t *testing.T) {
		app := failedDeploy(t, buildFailed)
		app.Generation, app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = 4, "4"
		app.Spec.RestartedAt = "2026-11-20T02:50:00Z"
		prepareAppReleaseDecision(app, nil)
		app.Status.Phase = appv1alpha1.PhaseBuilding
		if staticReleaseEndedUnpublished(app) || staticSaveWaits(app) {
			t.Fatal("release 4 is building the saved settings; release 3's verdict is not its own")
		}
	})
	t.Run("a newer deploy requested is on its way", func(t *testing.T) {
		app := failedDeploy(t, buildFailed)
		app.Generation, app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = 4, "4"
		if staticReleaseEndedUnpublished(app) || staticSaveWaits(app) {
			t.Fatal("a deploy not yet adopted carries the saved settings")
		}
	})
	t.Run("a publication an older operator recorded keeps today's comparison", func(t *testing.T) {
		app := failedDeploy(t, buildFailed)
		app.Status.ServedSettingsFingerprint = ""
		if staticSaveWaits(app) {
			t.Fatal("with no served settings, the consumed marker leaves nothing to compare")
		}
	})
	// w32/049 builds on this: a Restart after the failed deploy re-serves the
	// served publication, so it carries the served settings, not the failed
	// release's, and the saved branch keeps waiting once it serves.
	t.Run("a restart after it keeps the served settings", func(t *testing.T) {
		app := failedDeploy(t, buildFailed)
		app.Generation, app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = 4, "4"
		app.Spec.RestartedAt = "2026-11-20T02:52:00Z"
		app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 4, StaticPrefix: "site/rev-1/"}
		if d := prepareAppReleaseDecision(app, nil); !d.releaseChanged || app.Status.ReleaseSettingsFingerprint != served {
			t.Fatalf("decision = %+v, want the restart adopted with the served settings", d)
		}
		recordStaticRevision(app, "rev-4", true)
		app.Status.Phase = appv1alpha1.PhaseRunning
		if app.Status.ServedSettingsFingerprint != served || !staticSaveWaits(app) {
			t.Fatal("the restart serves the live settings; the saved branch still waits")
		}
	})
}

// A publication records the settings its release was dispatched with, and a
// served release an older operator published takes them on its next pass,
// except while a cancel left the canceled release's settings in status.
func TestRecordStaticRevisionRecordsServedSettings(t *testing.T) {
	served := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "site", Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}},
		Status:     appv1alpha1.AppStatus{ReleaseGeneration: 2, ReleaseSettingsFingerprint: "release-v2:two"},
	}
	for _, tc := range []struct {
		name      string
		edit      func(*appv1alpha1.App)
		published bool
		want      string
	}{
		{"a publication", func(a *appv1alpha1.App) { a.Status.ServedSettingsFingerprint = "release-v2:one" }, true, "release-v2:two"},
		{"an older operator's served release", func(*appv1alpha1.App) {}, false, "release-v2:two"},
		{"a recorded publication stays as recorded", func(a *appv1alpha1.App) { a.Status.ServedSettingsFingerprint = "release-v2:one" }, false, "release-v2:one"},
		{"a cancel left the canceled release's settings", func(a *appv1alpha1.App) {
			a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "2"
		}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := served.DeepCopy()
			tc.edit(app)
			recordStaticRevision(app, "rev-2", tc.published)
			if got := app.Status.ServedSettingsFingerprint; got != tc.want {
				t.Fatalf("servedSettingsFingerprint = %q, want %q", got, tc.want)
			}
		})
	}
}

// A dispatch records its release's saved settings next to its identity, and a
// deploy that only restamps them records the same settings (w8/339).
func TestDispatchRecordsReleaseSettings(t *testing.T) {
	spec := appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/octocat/Spoon-Knife", Branch: "main",
		BuildCommand: "mkdir -p out", PublishPath: "out",
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 1, Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
		Spec:       spec,
	}
	if d := prepareAppReleaseDecision(app, nil); !d.releaseChanged {
		t.Fatalf("decision = %+v, want the first release dispatched", d)
	}
	settings := app.Status.ReleaseSettingsFingerprint
	if settings != releaseSettingsFingerprint(spec) {
		t.Fatalf("releaseSettingsFingerprint = %q, want the dispatched spec's", settings)
	}

	app.Generation = 2
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	app.Spec.RestartedAt = "2026-11-19T09:00:00Z"
	first := app.Status.ReleaseFingerprint
	if d := prepareAppReleaseDecision(app, nil); !d.releaseChanged || app.Status.ReleaseFingerprint == first {
		t.Fatalf("decision = %+v, want the restamped deploy dispatched as a new release", d)
	}
	if app.Status.ReleaseSettingsFingerprint != settings {
		t.Fatal("a deploy that only restamps the release must record the same settings")
	}

	app.Generation = 3
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "3"
	app.Spec.PublishPath = "out2"
	prepareAppReleaseDecision(app, nil)
	if app.Status.ReleaseSettingsFingerprint == settings {
		t.Fatal("a deploy of a new publish directory must record its settings")
	}
}
