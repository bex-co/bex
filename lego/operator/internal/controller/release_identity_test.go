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
	"reflect"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestAppSpecIdentityClassificationIsExhaustive(t *testing.T) {
	typeOfSpec := reflect.TypeFor[appv1alpha1.AppSpec]()
	want := make(map[string]bool, typeOfSpec.NumField())
	for field := range typeOfSpec.Fields() {
		want[field.Name] = true
	}

	var missing, extra []string
	for field := range want {
		if _, ok := appSpecIdentityClasses[field]; !ok {
			missing = append(missing, field)
		}
	}
	for field := range appSpecIdentityClasses {
		if !want[field] {
			extra = append(extra, field)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("AppSpec identity classification drift: missing=%v extra=%v", missing, extra)
	}
}

func TestEveryAppSpecFieldMatchesItsIdentityClass(t *testing.T) {
	base := appv1alpha1.AppSpec{
		Repo:         "https://github.com/bex-co/bex.git",
		Branch:       "main",
		Runtime:      "go",
		Builder:      "native",
		BuildCommand: "go build -o app .",
		StartCommand: "./app",
		Port:         3000,
	}
	fields := make([]string, 0, len(appSpecIdentityClasses))
	for field := range appSpecIdentityClasses {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		class := appSpecIdentityClasses[field]
		t.Run(field, func(t *testing.T) {
			baseline := base
			// An explicit Render runtime owns builder selection; exercise the legacy
			// Builder field in the mode where it is authoritative.
			if field == "Builder" {
				baseline.Runtime = ""
			}
			baseIdentity := desiredAppReleaseIdentity(baseline)
			changed := baseline
			mutateIdentityTestField(reflect.ValueOf(&changed).Elem().FieldByName(field))
			got := desiredAppReleaseIdentity(changed)

			artifactChanged := got.artifact != baseIdentity.artifact
			releaseChanged := got.release != baseIdentity.release
			if artifactChanged != (class&identityArtifact != 0) {
				t.Errorf("artifact changed=%v, class=%d", artifactChanged, class)
			}
			if releaseChanged != (class&(identityArtifact|identityRelease) != 0) {
				t.Errorf("release changed=%v, class=%d", releaseChanged, class)
			}
		})
	}
}

func TestMissingReleaseIdentityForcesCanonicalRelease(t *testing.T) {
	app := &appv1alpha1.App{
		Spec: appv1alpha1.AppSpec{
			Repo: "https://github.com/bex-co/bex.git", Runtime: "go",
			BuildCommand: "go build -o app .", StartCommand: "./app", Replicas: 2,
		},
	}
	app.Generation = 2
	app.Status.Image = "zot.bex-registry.svc:5000/app:gen-1"
	app.Status.ActiveRevision = "rev-1"
	app.Status.ObservedGeneration = 1

	decision := prepareAppReleaseDecision(app)
	if !decision.artifactChanged || !decision.releaseChanged {
		t.Fatalf("decision = %+v, want normal artifact and release reconciliation", decision)
	}
	if app.Status.ReleaseGeneration != 2 {
		t.Fatalf("releaseGeneration = %d, want current generation 2", app.Status.ReleaseGeneration)
	}
	if app.Status.ArtifactFingerprint != "" || app.Status.ReleaseFingerprint == "" {
		t.Fatal("decision must not backfill a missing artifact identity without producing it")
	}
	if app.Status.ArtifactImage != "" {
		t.Fatalf("artifactImage = %q, want a fresh artifact", app.Status.ArtifactImage)
	}
}

func TestResolvedArtifactSurvivesPreDeployReconcile(t *testing.T) {
	spec := appv1alpha1.AppSpec{
		Repo: "https://github.com/bex-co/bex.git", Runtime: "go",
		BuildCommand: "go build -o app .", StartCommand: "./app",
		PreDeployCommand: "echo migrate",
	}
	identity := desiredAppReleaseIdentity(spec)
	app := &appv1alpha1.App{
		Spec: spec,
		Status: appv1alpha1.AppStatus{
			Image:               "registry.example/app:gen-1",
			ArtifactImage:       "registry.example/app:gen-2",
			ArtifactFingerprint: identity.artifact,
			ReleaseFingerprint:  identity.release,
			ReleaseGeneration:   2,
			ActiveRevision:      "rev-1",
			ObservedGeneration:  1,
		},
	}
	app.Generation = 2

	decision := prepareAppReleaseDecision(app)
	image, resolved := reusableArtifactImage(app, decision)
	if !resolved || image != "registry.example/app:gen-2" {
		t.Fatalf("reusable artifact = %q resolved=%v, want candidate gen-2", image, resolved)
	}
	if app.Status.Image != "registry.example/app:gen-1" {
		t.Fatalf("active image changed to %q before pre-deploy", app.Status.Image)
	}
}

func TestReleaseGenerationSurvivesOperationalMutationBeforeReconcile(t *testing.T) {
	previousSpec := appv1alpha1.AppSpec{Image: "example:v1", Port: 3000}
	previous := desiredAppReleaseIdentity(previousSpec)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Generation:  3,
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"},
		},
		Spec: appv1alpha1.AppSpec{Image: "example:v2", Port: 3000, Replicas: 2},
		Status: appv1alpha1.AppStatus{
			ArtifactFingerprint: previous.artifact,
			ReleaseFingerprint:  previous.release,
			ReleaseGeneration:   1,
			ActiveRevision:      "rev-1",
		},
	}

	decision := prepareAppReleaseDecision(app)
	if !decision.releaseChanged || app.Status.ReleaseGeneration != 2 {
		t.Fatalf("decision=%+v releaseGeneration=%d, want changed release 2", decision, app.Status.ReleaseGeneration)
	}

	// Once release 2 is active, its annotation is stale and cannot mask a
	// direct release edit made at metadata generation 4.
	app.Status.ArtifactFingerprint = decision.desired.artifact
	app.Status.ReleaseFingerprint = decision.desired.release
	app.Status.ReleaseGeneration = 2
	app.Generation = 4
	app.Spec.Image = "example:v3"
	decision = prepareAppReleaseDecision(app)
	if !decision.releaseChanged || app.Status.ReleaseGeneration != 4 {
		t.Fatalf("stale annotation decision=%+v releaseGeneration=%d, want direct generation 4", decision, app.Status.ReleaseGeneration)
	}
}

func TestPendingSourceWaitsForADeployGeneration(t *testing.T) {
	previousSpec := appv1alpha1.AppSpec{Repo: "https://github.com/acme/old", Branch: "main"}
	previous := desiredAppReleaseIdentity(previousSpec)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 2,
			Annotations: map[string]string{
				appv1alpha1.AnnotationPendingSourceGeneration: "2",
				appv1alpha1.AnnotationReleaseGeneration:       "1",
			},
		},
		Spec: appv1alpha1.AppSpec{Image: "nginx:stable"},
		Status: appv1alpha1.AppStatus{
			Image:               "registry.example/acme/old@sha256:active",
			ArtifactImage:       "registry.example/acme/old@sha256:active",
			ArtifactFingerprint: previous.artifact,
			ReleaseFingerprint:  previous.release,
			ReleaseGeneration:   1,
			ActiveRevision:      "rev-1",
		},
	}

	decision := prepareAppReleaseDecision(app)
	image, resolved := reusableArtifactImage(app, decision)
	if !decision.sourcePending || decision.artifactChanged || decision.releaseChanged {
		t.Fatalf("pending source decision = %+v, want no release", decision)
	}
	if !resolved || image != app.Status.Image {
		t.Fatalf("pending source image=%q resolved=%v, want active %q", image, resolved, app.Status.Image)
	}

	// A later operational spec generation must not accidentally consume the
	// pending source; only a deploy-generation annotation does.
	app.Generation = 3
	decision = prepareAppReleaseDecision(app)
	if !decision.sourcePending {
		t.Fatalf("operational generation consumed pending source: %+v", decision)
	}

	app.Generation = 4
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "4"
	decision = prepareAppReleaseDecision(app)
	if decision.sourcePending || !decision.artifactChanged || !decision.releaseChanged {
		t.Fatalf("deploy generation did not consume source: %+v", decision)
	}
	image, resolved = reusableArtifactImage(app, decision)
	if !resolved || image != "nginx:stable" {
		t.Fatalf("deployed source image=%q resolved=%v, want nginx:stable", image, resolved)
	}
}

func TestPendingSourceWithoutImageHaltsBeforeBuildOrStaticPublish(t *testing.T) {
	app := &appv1alpha1.App{
		Spec: appv1alpha1.AppSpec{Repo: "https://example.invalid/new.git"},
	}
	_, _, halted, err := (&AppReconciler{}).resolveDeployImage(
		context.Background(), app, appReleaseDecision{sourcePending: true}, 10000,
	)
	if err != nil || !halted {
		t.Fatalf("pending source resolve: halted=%v err=%v, want halted without error", halted, err)
	}
}

func TestCanceledReleaseKeepsLastSuccessfulGeneration(t *testing.T) {
	previousSpec := appv1alpha1.AppSpec{Repo: "https://example.invalid/repo.git", RestartedAt: "first"}
	previous := desiredAppReleaseIdentity(previousSpec)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 2,
			Annotations: map[string]string{
				appv1alpha1.AnnotationReleaseGeneration:         "2",
				appv1alpha1.AnnotationCanceledReleaseGeneration: "2",
			},
		},
		Spec: appv1alpha1.AppSpec{Repo: previousSpec.Repo, RestartedAt: "second"},
		Status: appv1alpha1.AppStatus{
			Image:               "registry.example/app@sha256:old",
			ArtifactImage:       "registry.example/app@sha256:old",
			ArtifactFingerprint: previous.artifact,
			ReleaseFingerprint:  previous.release,
			ReleaseGeneration:   2,
			ActiveRevision:      "rev-1",
			ObservedGeneration:  1,
		},
	}

	decision := prepareAppReleaseDecision(app)
	if !decision.canceled {
		t.Fatalf("decision = %+v, want canceled", decision)
	}
	if app.Status.ReleaseGeneration != 1 {
		t.Fatalf("releaseGeneration = %d, want last successful generation 1", app.Status.ReleaseGeneration)
	}
}

func TestNewReleaseSupersedesCanceledGeneration(t *testing.T) {
	previous := desiredAppReleaseIdentity(appv1alpha1.AppSpec{Repo: "https://example.invalid/repo.git", RestartedAt: "first"})
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 3,
			Annotations: map[string]string{
				appv1alpha1.AnnotationReleaseGeneration:         "3",
				appv1alpha1.AnnotationCanceledReleaseGeneration: "2",
			},
		},
		Spec: appv1alpha1.AppSpec{Repo: "https://example.invalid/repo.git", RestartedAt: "third"},
		Status: appv1alpha1.AppStatus{
			Image:               "registry.example/app@sha256:old",
			ArtifactFingerprint: previous.artifact,
			ReleaseFingerprint:  previous.release,
			ReleaseGeneration:   1,
			ActiveRevision:      "rev-1",
		},
	}

	decision := prepareAppReleaseDecision(app)
	if decision.canceled || !decision.releaseChanged || app.Status.ReleaseGeneration != 3 {
		t.Fatalf("decision=%+v releaseGeneration=%d, want normal release 3", decision, app.Status.ReleaseGeneration)
	}
}

func mutateIdentityTestField(value reflect.Value) {
	switch value.Kind() {
	case reflect.String:
		value.SetString(value.String() + "changed")
	case reflect.Bool:
		value.SetBool(!value.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(value.Int() + 1)
	case reflect.Pointer:
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		mutateIdentityTestField(value.Elem())
	case reflect.Slice:
		slice := reflect.MakeSlice(value.Type(), 1, 1)
		mutateIdentityTestField(slice.Index(0))
		value.Set(slice)
	case reflect.Map:
		m := reflect.MakeMap(value.Type())
		key := reflect.New(value.Type().Key()).Elem()
		val := reflect.New(value.Type().Elem()).Elem()
		mutateIdentityTestField(key)
		mutateIdentityTestField(val)
		m.SetMapIndex(key, val)
		value.Set(m)
	case reflect.Struct:
		for _, field := range value.Fields() {
			if field.CanSet() {
				mutateIdentityTestField(field)
				return
			}
		}
	default:
		panic("unsupported AppSpec test field kind: " + value.Kind().String())
	}
}

// TestSpecRollsReleaseMatchesReleaseFingerprint is the cross-plane guard for
// w6/m51: bex-api decides whether a spec patch owes the user a deploy-history
// row by calling appv1alpha1.SpecRollsRelease, while the operator decides
// whether to actually roll by comparing release fingerprints. If those two ever
// disagree, a real rollout goes unrecorded (the original bug) or a no-op patch
// mints a phantom deploy. Assert they agree field by field.
func TestSpecRollsReleaseMatchesReleaseFingerprint(t *testing.T) {
	base := appv1alpha1.AppSpec{
		Repo:         "https://github.com/bex-co/bex.git",
		Branch:       "main",
		Runtime:      "go",
		Builder:      "native",
		BuildCommand: "go build -o app .",
		StartCommand: "./app",
		Port:         3000,
	}
	fields := make([]string, 0, len(appSpecIdentityClasses))
	for field := range appSpecIdentityClasses {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			baseline := base
			if field == "Builder" {
				baseline.Runtime = ""
			}
			changed := baseline
			mutateIdentityTestField(reflect.ValueOf(&changed).Elem().FieldByName(field))

			operatorRolls := desiredAppReleaseIdentity(changed).release != desiredAppReleaseIdentity(baseline).release
			backendRolls := appv1alpha1.SpecRollsRelease(baseline, changed)
			if operatorRolls != backendRolls {
				t.Fatalf("operator rolls=%v but SpecRollsRelease=%v for %s", operatorRolls, backendRolls, field)
			}
		})
	}
}

// TestSpecRollsReleaseIgnoresNoOpChanges pins the two cases a naive field-wise
// comparison gets wrong: an unchanged spec, and a list rewritten from nil to
// empty (what removing the last linked env group produces).
func TestSpecRollsReleaseIgnoresNoOpChanges(t *testing.T) {
	spec := appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/bex.git", StartCommand: "./app"}
	if appv1alpha1.SpecRollsRelease(spec, spec) {
		t.Fatal("an unchanged spec must not roll a release")
	}
	emptied := spec
	emptied.EnvFromSecrets = []string{}
	emptied.FilesFromSecrets = []string{}
	if appv1alpha1.SpecRollsRelease(spec, emptied) {
		t.Fatal("nil -> empty list must not roll a release")
	}
	grown := spec
	grown.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 10}
	resized := spec
	resized.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 20}
	if !appv1alpha1.SpecRollsRelease(spec, grown) {
		t.Fatal("attaching a disk must roll a release")
	}
	if appv1alpha1.SpecRollsRelease(grown, resized) {
		t.Fatal("growing a disk online must not roll a release")
	}
}

func TestClearCacheAppliesOnlyMatchingRelease(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 3,
			Annotations: map[string]string{
				appv1alpha1.AnnotationReleaseGeneration:           "3",
				appv1alpha1.AnnotationClearCacheReleaseGeneration: "3",
			},
		},
	}
	if !clearCacheApplies(app) {
		t.Fatal("matching clear-cache generation must apply")
	}
	app.Annotations[appv1alpha1.AnnotationClearCacheReleaseGeneration] = "2"
	if clearCacheApplies(app) {
		t.Fatal("stale clear-cache marker must not apply to a newer release")
	}
	delete(app.Annotations, appv1alpha1.AnnotationClearCacheReleaseGeneration)
	if clearCacheApplies(app) {
		t.Fatal("absent clear-cache marker must not apply")
	}
	app.Annotations[appv1alpha1.AnnotationClearCacheReleaseGeneration] = "not-a-number"
	if clearCacheApplies(app) {
		t.Fatal("garbage clear-cache marker must not apply")
	}
}

// TestSettingsCommandEditsDemandAFreshArtifact is w4/m110 t001's contract
// pinned against a real production App.
//
// Live on 2026-09-17, a native-runtime web service created from a Public Git
// URL took three Settings saves (env var, start command, build command). Each
// opened a Config Change deploy that went live in 13-30s on the rev-1 image,
// with no Build events and pods still running the previously baked command,
// while a deploy-hook trigger of the same spec rebuilt and picked the edits up.
//
// The spec below is `qa-20260919-p13-web`'s, read verbatim from the Hetzner app
// cluster (`tea-d98210cbbpdc73dcrkvg`) while it served. Its fingerprints below
// are the ones that App's status carried, so this test also proves the operator
// binary in production computes identity from exactly this code: if either
// constant stops matching, the identity inputs moved and the deployed
// fingerprints can no longer be reasoned about from this file.
func TestSettingsCommandEditsDemandAFreshArtifact(t *testing.T) {
	const (
		liveArtifact = "artifact-v1:737b9bce64c8652cae5f0d19239c6f471a82f7ea5174a9f09e68c9a42476e5e0"
		liveRelease  = "release-v1:f9698cfe48fa9c208d0ee5b80979ece4b59eb56c9388a7a48fcae9769964a6b0"
		liveImage    = "zot.bex-registry.svc:5000/tea-d98210cbbpdc73dcrkvg/" +
			"tea-d98210cbbpdc73dcrkvg-qa-20260919-p13-web:gen-1@sha256:8ec7ee9d98f2d1960abf21bafea50f1c8402978ca49ee92048b5002d0abcf73c"
	)
	live := appv1alpha1.AppSpec{
		AutoDeploy: true, Branch: "main", BuildCommand: "npm install", Builder: "native",
		Port: 3000, Repo: "https://github.com/render-examples/express-hello-world",
		Runtime: "node", StartCommand: "npm start", Subdomain: "qa-20260919-p13-web",
		Tier: "free", Type: "web_service",
	}
	if got := desiredAppReleaseIdentity(live); got.artifact != liveArtifact || got.release != liveRelease {
		t.Fatalf("identity of the live prod spec = %+v, want the fingerprints that App's status carried", got)
	}

	// The two Settings edits, each applied to the live spec on its own.
	for _, tc := range []struct {
		name string
		edit func(*appv1alpha1.AppSpec)
	}{
		{"start command", func(s *appv1alpha1.AppSpec) {
			s.StartCommand = `sh -c "echo QAENV:$QA_TEST_VAR && npm start"`
		}},
		{"build command", func(s *appv1alpha1.AppSpec) {
			s.BuildCommand = "npm install && echo QA_BUILD_MARKER"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &appv1alpha1.App{Spec: *live.DeepCopy()}
			app.Generation = 6
			app.Annotations = map[string]string{appv1alpha1.AnnotationReleaseGeneration: "6"}
			app.Status.ArtifactFingerprint = liveArtifact
			app.Status.ReleaseFingerprint = liveRelease
			app.Status.ArtifactImage = liveImage
			app.Status.Image = liveImage
			app.Status.ActiveRevision = "rev-1"
			app.Status.ReleaseGeneration = 1
			app.Status.ObservedGeneration = 1
			app.Status.Phase = appv1alpha1.PhaseRunning
			tc.edit(&app.Spec)

			decision := prepareAppReleaseDecision(app)
			if !decision.artifactChanged {
				t.Fatalf("%s edit left the artifact unchanged — the deploy would roll the stale image", tc.name)
			}
			if !decision.releaseChanged {
				t.Fatalf("%s edit left the release unchanged", tc.name)
			}
			// The reuse gate is what actually decides "build or roll the old
			// image", so assert it directly rather than only the fingerprints.
			if image, resolved := reusableArtifactImage(app, decision); resolved || image != "" {
				t.Fatalf("%s edit reused %q (resolved=%v); a rebuild is owed", tc.name, image, resolved)
			}
			if app.Status.ReleaseGeneration != 6 {
				t.Fatalf("releaseGeneration = %d, want the requested generation 6 (a fresh gen-N build tag)",
					app.Status.ReleaseGeneration)
			}
		})
	}
}

func TestSuccessfulReleaseGenerationLegacyBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                                       string
		active, release, artifact, releaseArtifact string
		want                                       int64
	}{
		{name: "legacy image", want: 4},
		{name: "modern release attempt", release: "release", want: 0},
		{name: "modern artifact attempt", artifact: "artifact", want: 0},
		{name: "modern release artifact", releaseArtifact: "artifact", want: 0},
		{name: "legacy revision name", active: "old-revision", release: "release", want: 4},
		{name: "served revision wins", active: "rev-2", release: "release", want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &appv1alpha1.App{Status: appv1alpha1.AppStatus{
				Image: "image", ObservedGeneration: 4, ActiveRevision: tc.active,
				ReleaseFingerprint: tc.release, ArtifactFingerprint: tc.artifact,
				ReleaseArtifactFingerprint: tc.releaseArtifact,
			}}
			if got := successfulReleaseGeneration(app); got != tc.want {
				t.Fatalf("successful generation = %d, want %d", got, tc.want)
			}
		})
	}
}
