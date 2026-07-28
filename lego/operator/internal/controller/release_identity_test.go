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
	"reflect"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestAppSpecIdentityClassificationIsExhaustive(t *testing.T) {
	typeOfSpec := reflect.TypeFor[appv1alpha1.AppSpec]()
	want := make(map[string]bool, typeOfSpec.NumField())
	for i := range typeOfSpec.NumField() {
		want[typeOfSpec.Field(i).Name] = true
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

func TestLegacyReleaseIdentityAdoptsActiveRevision(t *testing.T) {
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
	if !decision.legacyBackfill || decision.artifactChanged || decision.releaseChanged {
		t.Fatalf("decision = %+v, want legacy adoption with no build/release", decision)
	}
	if app.Status.ReleaseGeneration != 1 {
		t.Fatalf("releaseGeneration = %d, want 1", app.Status.ReleaseGeneration)
	}
	if app.Status.ArtifactFingerprint == "" || app.Status.ReleaseFingerprint == "" {
		t.Fatal("legacy adoption must persist both identities")
	}
	if app.Status.ArtifactImage != app.Status.Image {
		t.Fatalf("artifactImage = %q, want active image %q", app.Status.ArtifactImage, app.Status.Image)
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
	case reflect.Ptr:
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
		for i := range value.NumField() {
			field := value.Field(i)
			if field.CanSet() {
				mutateIdentityTestField(field)
				return
			}
		}
	default:
		panic("unsupported AppSpec test field kind: " + value.Kind().String())
	}
}
