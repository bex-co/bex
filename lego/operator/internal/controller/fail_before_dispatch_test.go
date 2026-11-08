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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestImageRefusedBeforeDispatchKeepsThePriorReleaseServing is w4/533: a
// deploy of a missing tag over a serving image service failed before any pod
// was dispatched, yet the service read Failed and bex-api recorded a phantom
// server_failed. Over a served release the phase and Ready keep describing the
// release that serves, and the Rollout verdict carries the failure to the
// deploy row; with nothing served it stays an ordinary failure.
func TestImageRefusedBeforeDispatchKeepsThePriorReleaseServing(t *testing.T) {
	ctx := context.Background()
	refused := errors.New("image metadata request returned status 404")
	setup := func(t *testing.T, served bool) (*AppReconciler, *appv1alpha1.App) {
		t.Helper()
		one := int32(1)
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", Generation: 2,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}},
			Spec:   appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Image: "docker.io/traefik/whoami:qa-no-such-tag", Replicas: 1},
			Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, ReleaseGeneration: 2},
		}
		if served {
			app.Status.ActiveRevision = "rev-1"
		}
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"}, Spec: appsv1.DeploymentSpec{Replicas: &one}}
		scheme := wakeScheme()
		cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(app).WithObjects(app, dep).Build()
		return wakeReconciler(cl, scheme), app
	}

	t.Run("over a served release", func(t *testing.T) {
		r, app := setup(t, true)
		if _, err := r.failBeforeDispatch(ctx, app, "ImageMetadataUnavailable", refused); !errors.Is(err, refused) {
			t.Fatalf("err = %v, want the refusal for a retry", err)
		}
		got := &appv1alpha1.App{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(app), got); err != nil {
			t.Fatal(err)
		}
		ready := meta.FindStatusCondition(got.Status.Conditions, appv1alpha1.ConditionReady)
		rollout := meta.FindStatusCondition(got.Status.Conditions, appv1alpha1.ConditionRollout)
		if got.Status.Phase != appv1alpha1.PhaseRunning || ready == nil || ready.Status != metav1.ConditionTrue ||
			ready.Reason != reasonPriorReleaseServing {
			t.Fatalf("phase %q ready %+v, want Running and the prior release serving", got.Status.Phase, ready)
		}
		if rollout == nil || rollout.Status != metav1.ConditionFalse || rollout.ObservedGeneration != 2 || rollout.Message != refused.Error() {
			t.Fatalf("rollout verdict = %+v, want the refusal for release 2", rollout)
		}
	})

	t.Run("with nothing served", func(t *testing.T) {
		r, app := setup(t, false)
		_, _ = r.failBeforeDispatch(ctx, app, "ImageMetadataUnavailable", refused)
		got := &appv1alpha1.App{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(app), got); err != nil {
			t.Fatal(err)
		}
		if got.Status.Phase != appv1alpha1.PhaseFailed {
			t.Fatalf("phase = %q, want Failed for a first release", got.Status.Phase)
		}
	})
}

// w4/534: a missing tag failed with the registry client's "image metadata
// request returned status 404", naming neither the image nor the fix.
func TestImageRefusalNamesTheImageAndTheFix(t *testing.T) {
	const image = "docker.io/traefik/whoami:qa-no-such-tag"
	for _, tc := range []struct {
		name   string
		err    error
		want   []string
		absent string
	}{
		{"missing tag", &registry.RefusedError{Status: 404}, []string{image, "was not found", "repository or tag does not exist"}, "returned status"},
		{"refused access", &registry.RefusedError{Status: 401}, []string{image, "refused access", "registry credential for docker.io"}, "returned status"},
		{"forbidden on another host", &registry.RefusedError{Status: 403}, []string{"registry credential for ghcr.io"}, ""},
		{"other refusal", &registry.RefusedError{Status: 400}, []string{image, "HTTP 400"}, ""},
	} {
		ref := image
		if tc.name == "forbidden on another host" {
			ref = "ghcr.io/acme/private:1"
		}
		got := imageRefusalError(ref, fmt.Errorf("wrapped: %w", tc.err)).Error()
		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s: %q, want it to contain %q", tc.name, got, want)
			}
		}
		if tc.absent != "" && strings.Contains(got, tc.absent) {
			t.Errorf("%s: %q still carries %q", tc.name, got, tc.absent)
		}
	}
	other := errors.New("image port discovery cannot parse image")
	if got := imageRefusalError(image, other); got != other {
		t.Errorf("a non-refusal = %v, want it unchanged", got)
	}
}

// w2/072: Render reports a missing image or tag, and a refused credential, as
// an Image Pull Failed service event and email. The registry's refusal of the
// image metadata read is that case caught before any pod is dispatched, so it
// fails the deploy under ReasonImagePullFailed, which bex-api records as
// image_pull_failed; any other metadata failure keeps ImageMetadataUnavailable.
func TestImageRefusalFailsUnderImagePullFailed(t *testing.T) {
	// The registry's real answer for a tag that does not exist.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}`))
	}))
	defer srv.Close()
	_, missing := registry.ReadImageConfig(context.Background(), srv.Client(), strings.TrimPrefix(srv.URL, "http://"), "library/busybox", "no-such-tag", "")
	if missing == nil {
		t.Fatal("a 404 manifest read succeeded")
	}

	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"missing tag (MANIFEST_UNKNOWN)", missing, appv1alpha1.ReasonImagePullFailed},
		{"refused credential", fmt.Errorf("wrapped: %w", &registry.RefusedError{Status: http.StatusUnauthorized}), appv1alpha1.ReasonImagePullFailed},
		{"forbidden", &registry.RefusedError{Status: http.StatusForbidden}, appv1alpha1.ReasonImagePullFailed},
		{"other refusal", &registry.RefusedError{Status: http.StatusBadRequest}, appv1alpha1.ReasonImagePullFailed},
		{"unparseable image", errors.New("image port discovery cannot parse image"), "ImageMetadataUnavailable"},
	} {
		if got := imageMetadataFailureReason(tc.err); got != tc.want {
			t.Errorf("%s: reason = %q, want %q", tc.name, got, tc.want)
		}
	}

	// The reason reaches the condition bex-api reads: Rollout over a served
	// release, Ready with nothing served.
	for _, served := range []bool{true, false} {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", Generation: 2,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}},
			Spec:   appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Image: "docker.io/library/busybox:no-such-tag", Replicas: 1},
			Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying, ReleaseGeneration: 2},
		}
		condition := appv1alpha1.ConditionReady
		if served {
			app.Status.ActiveRevision = "rev-1"
			condition = appv1alpha1.ConditionRollout
		}
		scheme := wakeScheme()
		cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(app).WithObjects(app).Build()
		r := wakeReconciler(cl, scheme)
		_, _ = r.failBeforeDispatch(context.Background(), app, imageMetadataFailureReason(missing), imageRefusalError(app.Spec.Image, missing))
		got := &appv1alpha1.App{}
		if err := r.Get(context.Background(), client.ObjectKeyFromObject(app), got); err != nil {
			t.Fatal(err)
		}
		c := meta.FindStatusCondition(got.Status.Conditions, condition)
		if c == nil || c.Status != metav1.ConditionFalse || c.Reason != appv1alpha1.ReasonImagePullFailed ||
			!strings.Contains(c.Message, "was not found in its registry") {
			t.Errorf("served=%v: %s = %+v, want False/%s naming the missing image", served, condition, c, appv1alpha1.ReasonImagePullFailed)
		}
	}
}
