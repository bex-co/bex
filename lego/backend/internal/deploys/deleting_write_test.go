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
