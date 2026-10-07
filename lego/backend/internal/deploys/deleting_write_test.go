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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/023: `bex deploys create` ~2 s after `bex services delete` answered 500
// while every read of the same service answered 404 — the trigger reached a
// terminating App's secrets, CR and store row. Every deploy write now answers
// a deleting service the way reads do, and writes nothing.
func TestDeployWritesOnADeletingServiceAre404(t *testing.T) {
	for verb, call := range map[string]func(*Service) error{
		"trigger image": func(s *Service) error {
			_, err := s.Trigger(context.Background(), "svc", TriggerParams{ImageURL: "nginx:1.27"})
			return err
		},
		"restart": func(s *Service) error {
			_, err := s.Trigger(context.Background(), "svc", TriggerParams{})
			return err
		},
		"rollback": func(s *Service) error { _, err := s.Rollback(context.Background(), "svc", "dep-1"); return err },
		"cancel":   func(s *Service) error { _, err := s.Cancel(context.Background(), "svc", "dep-2"); return err },
	} {
		t.Run(verb, func(t *testing.T) {
			ds := newFakeStore()
			app := sampleApp("svc", "srv-11")
			now := metav1.Now()
			app.DeletionTimestamp = &now
			app.Finalizers = []string{"app.bex.co/finalizer"}
			svc, cl := newService(ds, app)
			// A rollback target and an open deploy, so rollback and cancel reach
			// their writes rather than 404 on a missing row.
			ds.byApp["srv-11"] = []store.Deploy{
				{ID: "dep-2", AppID: "srv-11", Status: store.DeployBuildInProgress, Image: "svc:v1", Generation: 2},
				{ID: "dep-1", AppID: "srv-11", Status: store.DeployLive, Image: "svc:v0", ResolvedImage: "svc:v0", Generation: 1},
			}
			seeded := len(ds.byApp["srv-11"])
			before := getApp(t, cl, "svc").ResourceVersion

			if err := call(svc); !errors.Is(err, core.ErrNotFound) {
				t.Fatalf("%s on a deleting service = %v, want ErrNotFound (404)", verb, err)
			}
			if len(ds.byApp["srv-11"]) != seeded || ds.byApp["srv-11"][0].Status != store.DeployBuildInProgress || len(ds.setImage) != 0 {
				t.Errorf("%s wrote store state for a deleting service: rows %v, image %v", verb, ds.byApp["srv-11"], ds.setImage)
			}
			if after := getApp(t, cl, "svc").ResourceVersion; after != before {
				t.Errorf("%s patched the deleting App (resourceVersion %s → %s)", verb, before, after)
			}
		})
	}
}

// lockWaitStore is fakeStore with the trigger lock, whose whileWaiting runs
// once before the lock is granted: what lands while a trigger waits for it.
type lockWaitStore struct {
	*fakeStore
	whileWaiting func()
}

func (l *lockWaitStore) WithAppAdvisoryLock(_ context.Context, _ string, fn func() error) error {
	if hook := l.whileWaiting; hook != nil {
		l.whileWaiting = nil
		hook()
	}
	return fn()
}

// TestADeployForARecreatedServiceLeavesItsNamesakeAlone (w5/157): a trigger
// whose service was deleted and recreated under its name while it waited for
// the trigger lock re-read the App by name and released the namesake, filing
// the release under the deleted service's history. It now refuses as the
// service having changed and writes nothing.
func TestADeployForARecreatedServiceLeavesItsNamesakeAlone(t *testing.T) {
	ds := &lockWaitStore{fakeStore: newFakeStore()}
	svc, cl := newService(ds, sampleApp("svc", "srv-11"))
	ctx := context.Background()
	var recreated error
	ds.whileWaiting = func() {
		recreated = func() error {
			if err := cl.Delete(ctx, sampleApp("svc", "srv-11")); err != nil {
				return err
			}
			namesake := sampleApp("svc", "srv-22")
			namesake.UID = "uid-namesake"
			return cl.Create(ctx, namesake)
		}()
	}

	_, err := svc.Trigger(ctx, "svc", TriggerParams{ImageURL: "svc:v2"})
	if recreated != nil {
		t.Fatalf("recreate: %v", recreated)
	}
	if !errors.Is(err, core.ErrServiceReplaced) {
		t.Fatalf("the trigger = %v, want the conflict that the service changed", err)
	}
	namesake := getApp(t, cl, "svc")
	if len(ds.byApp) != 0 || len(ds.setImage) != 0 || namesake.UID != "uid-namesake" || namesake.Spec.Image != "svc:v1" {
		t.Fatalf("the trigger wrote rows %v and images %v, and left the namesake %s at %q; want nothing written", ds.byApp, ds.setImage, namesake.UID, namesake.Spec.Image)
	}
}
