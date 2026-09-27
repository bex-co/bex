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
	"strconv"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m46: two `deploys create --image` 14 ms apart both closed canceled, and
// the second row recorded the other call's image; in another pair the OLDER
// trigger won. Each trigger wrote the row image, derived its release
// generation from its own pre-trigger read and merge-patched the CR with no
// resourceVersion, so the writes interleaved.

// generationBumpingClient behaves like the API server where the fake does
// not: a Patch that changes spec bumps metadata.generation. A small delay
// inside the patch widens the window two unserialized triggers overlap in.
func generationBumpingClient(objs ...client.Object) client.Client {
	return interceptor.NewClient(fakeClient(objs...).(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			before := &appv1alpha1.App{}
			if err := c.Get(ctx, client.ObjectKeyFromObject(obj), before); err != nil {
				return err
			}
			time.Sleep(2 * time.Millisecond)
			if err := c.Patch(ctx, obj, patch, opts...); err != nil {
				return err
			}
			after := obj.(*appv1alpha1.App)
			if after.Spec.RestartedAt != before.Spec.RestartedAt || after.Spec.Image != before.Spec.Image {
				after.Generation = before.Generation + 1
				return c.Update(ctx, after)
			}
			return nil
		},
	})
}

// triggerPair fires two image triggers (or an image trigger and a restart) at
// the same service concurrently and returns the rows, with the CR and the row
// image the pair left behind.
func triggerPair(t *testing.T, a, b TriggerParams) ([2]DeployView, *appv1alpha1.App, string, map[string]int64) {
	t.Helper()
	ds := newFakeStore()
	app := sampleApp("svc", "srv-11")
	app.Spec.Image = "echo:33"
	cl := generationBumpingClient(app)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: time.Now}, Store: ds}
	var rows [2]DeployView
	var errs [2]error
	var wg sync.WaitGroup
	for i, p := range []TriggerParams{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows[i], errs[i] = svc.Trigger(context.Background(), "svc", p)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("trigger %d: %v", i, err)
		}
	}
	generations := map[string]int64{}
	for _, d := range ds.byApp["srv-11"] {
		generations[d.ID] = d.Generation
	}
	return rows, getApp(t, cl, "svc"), ds.setImage["srv-11"], generations
}

func TestConcurrentTriggersResolveNewestWins(t *testing.T) {
	for name, pair := range map[string][2]TriggerParams{
		"image vs image":   {{ImageURL: "echo:34"}, {ImageURL: "echo:35"}},
		"image vs restart": {{ImageURL: "echo:34"}, {}},
	} {
		t.Run(name, func(t *testing.T) {
			for range 10 {
				rows, cr, rowImage, generations := triggerPair(t, pair[0], pair[1])
				gen := func(v DeployView) int64 { return generations[v.ID] }
				for i, p := range pair {
					if p.ImageURL != "" && rows[i].Image != p.ImageURL {
						t.Fatalf("row %d image = %q, want its own request %q", i, rows[i].Image, p.ImageURL)
					}
				}
				older, newer := rows[0], rows[1]
				if gen(older) > gen(newer) {
					older, newer = newer, older
				}
				// Distinct, ordered releases; the CR names the newest row's release
				// and runs its image, and the row image agrees.
				release, _ := strconv.ParseInt(cr.Annotations[appv1alpha1.AnnotationReleaseGeneration], 10, 64)
				if gen(older) >= gen(newer) || release != gen(newer) {
					t.Fatalf("releases: older %d, newer %d, CR %d — want older < newer == CR", gen(older), gen(newer), release)
				}
				if cr.Spec.Image != newer.Image || rowImage != newer.Image {
					t.Fatalf("CR image %q / row image %q, want the newest row's %q", cr.Spec.Image, rowImage, newer.Image)
				}
				if newer.Status == store.DeployCanceled {
					t.Fatalf("the newest row opened canceled: %+v", newer)
				}
			}
		})
	}
}
