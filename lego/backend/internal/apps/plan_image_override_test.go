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
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// historyStore is recordingStore with the deploy history the served-image
// selection reads, filtered by status as the store filters it.
type historyStore struct {
	*recordingStore
	rows []store.Deploy
}

func (s *historyStore) ListDeploys(_ context.Context, _ string, filter store.DeployFilter) ([]store.Deploy, error) {
	var rows []store.Deploy
	for _, d := range s.rows {
		if len(filter.Statuses) == 0 || slices.Contains(filter.Statuses, d.Status) {
			rows = append(rows, d)
		}
	}
	return rows, nil
}

const (
	savedWhoami    = "docker.io/traefik/whoami:v1.10.1"
	overrideWhoami = "docker.io/traefik/whoami:v1.11.0"
	overrideDigest = "docker.io/traefik/whoami@sha256:200689790a0a0ea48ca45992e0450bc26ccab5307375b41c84dfc4f2475937ab"
)

// overriddenWhoami is w6/159's fixture: a Starter image service created on
// whoami v1.10.1 whose release 6, an imageUrl deploy of v1.11.0, is live.
func overriddenWhoami() (*appv1alpha1.App, *historyStore) {
	a := manage(&appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "default", Generation: 6,
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "6"},
		},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Image: savedWhoami, Tier: "starter", Replicas: 1,
			ReleaseConfig: &appv1alpha1.ReleaseConfigReference{Generation: 6, Image: overrideWhoami},
		},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning, ReleaseGeneration: 6, SelectedReleaseGeneration: 6, ActiveRevision: "rev-6",
		},
	}, "srv-test")
	return a, &historyStore{recordingStore: &recordingStore{}, rows: []store.Deploy{
		{ID: "dep-override", Trigger: store.TriggerAPI, Status: store.DeployLive, Generation: 6, Image: overrideWhoami, ResolvedImage: overrideDigest},
		{ID: "dep-create", Trigger: store.TriggerCreate, Status: store.DeployDeactivated, Generation: 1, Image: savedWhoami},
	}}
}

// w6/159: a Starter → Standard change after an imageUrl deploy rolled the
// service back to the tag it was created with, because the settings release
// ran spec.image. Through the dashboard (updateServicePlan), which deploys a
// plan change at once, the release the plan change opens now runs the digest
// the override shipped, and its deploy row says so. The saved image stays the
// one the service was created with, as Render's settings do.
func TestAPlanChangeAfterAnImageOverrideKeepsItsImage(t *testing.T) {
	a, st := overriddenWhoami()
	svc, cl := newService(st, a)
	if _, err := svc.SetPlan(context.Background(), "web", "standard"); err != nil {
		t.Fatal(err)
	}
	got := getApp(t, cl, "web")
	want := appv1alpha1.ReleaseConfigReference{Generation: 7, Image: overrideDigest}
	if got.Spec.Tier != "standard" || got.Spec.ReleaseConfig == nil || *got.Spec.ReleaseConfig != want {
		t.Fatalf("tier %q, release config %+v; want standard on %+v", got.Spec.Tier, got.Spec.ReleaseConfig, want)
	}
	if got.Spec.Image != savedWhoami || got.Annotations[appv1alpha1.AnnotationReleaseGeneration] != "7" {
		t.Fatalf("saved image %q, release %q; want %s kept and release 7", got.Spec.Image, got.Annotations[appv1alpha1.AnnotationReleaseGeneration], savedWhoami)
	}
	if len(st.deployCalls) != 1 || st.deployCalls[0].Trigger != store.TriggerSettingsChange || st.deployCalls[0].Image != overrideDigest {
		t.Fatalf("deploy rows = %+v, want one settings_change row running %s", st.deployCalls, overrideDigest)
	}
}

// The same change through Render's Update Service is saved for the next
// deploy (w6/m609): the override keeps serving, its selection is left alone,
// and no deploy is opened.
func TestASavedPlanAfterAnImageOverrideLeavesItServing(t *testing.T) {
	a, st := overriddenWhoami()
	svc, cl := newService(st, a)
	plan := "standard"
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{Plan: &plan}); err != nil {
		t.Fatal(err)
	}
	got := getApp(t, cl, "web")
	want := appv1alpha1.ReleaseConfigReference{Generation: 6, Image: overrideWhoami}
	if got.Spec.Tier != "standard" || got.Spec.ReleaseConfig == nil || *got.Spec.ReleaseConfig != want {
		t.Fatalf("tier %q, release config %+v; want standard saved beside the override's %+v", got.Spec.Tier, got.Spec.ReleaseConfig, want)
	}
	if got.Spec.Image != savedWhoami || got.Annotations[appv1alpha1.AnnotationReleaseGeneration] != "6" ||
		got.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != "7" {
		t.Fatalf("saved image %q, annotations %v; want %s kept, release 6 and the plan pending for generation 7",
			got.Spec.Image, got.Annotations, savedWhoami)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %+v, want none", st.deployCalls)
	}
}

// A deploy that lands between a compound PATCH's read and its publish keeps
// its own image selection: the saved plan neither overwrites the deploy's
// image nor its release.
func TestAPlanChangeRacedByADeployKeepsTheDeploysImage(t *testing.T) {
	a, st := overriddenWhoami()
	svc, cl := newService(st, a)
	writes := observeAppWrites(svc, cl)
	racer := appv1alpha1.ReleaseConfigReference{Generation: 7, Image: "docker.io/traefik/whoami:v1.11.1"}
	raced := false
	writes.beforePatch = func(ctx context.Context, inner client.WithWatch) error {
		if raced {
			return nil
		}
		raced = true
		var live appv1alpha1.App
		if err := inner.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &live); err != nil {
			return err
		}
		base := client.MergeFrom(live.DeepCopy())
		live.Spec.ReleaseConfig = racer.DeepCopy()
		live.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "7"
		live.Generation = 7
		return inner.Patch(ctx, &live, base)
	}
	plan := "standard"
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{Plan: &plan}); err != nil {
		t.Fatal(err)
	}
	got := getApp(t, cl, "web")
	if got.Spec.Tier != "standard" || got.Spec.ReleaseConfig == nil || *got.Spec.ReleaseConfig != racer {
		t.Fatalf("tier %q, release config %+v; want standard with the racing deploy's %+v", got.Spec.Tier, got.Spec.ReleaseConfig, racer)
	}
	// The plan is saved behind the racing deploy (w6/m609), which keeps its
	// own release stamp; the PATCH opens no deploy of its own.
	if got.Annotations[appv1alpha1.AnnotationReleaseGeneration] != "7" || got.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != "8" {
		t.Fatalf("annotations %v, want the racing release 7 kept and the plan pending for generation 8", got.Annotations)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %+v, want none", st.deployCalls)
	}
}
