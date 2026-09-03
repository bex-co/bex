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

// TestDeletingServiceHidesDeployHistory pins the w3/m81 read contract for the
// deploy children: once a service's deletion is accepted, its .../deploys list
// and by-id read return the same core.ErrNotFound the service read does — even
// when real deploy rows still exist in the store. A deleting service must not
// keep serving the history of a resource that is being torn down.
func TestDeletingServiceHidesDeployHistory(t *testing.T) {
	ds := newFakeStore()
	app := sampleApp("web", "srv-1")
	now := metav1.NewTime(fixedNow())
	app.DeletionTimestamp = &now
	app.Finalizers = []string{"app.bex.co/finalizer"}
	svc, _ := newService(ds, app)

	// Seed a real deploy so the assertion proves the deletion gate rather than an
	// incidentally empty store.
	d, err := ds.CreateDeploy(context.Background(), "srv-1", "manual", "web:v1", 1, store.CommitInfo{})
	if err != nil {
		t.Fatalf("seed deploy: %v", err)
	}

	if _, err := svc.List(context.Background(), "web", ListFilter{}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("List deploys for a deleting service = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.Get(context.Background(), "web", d.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Get an existing deploy for a deleting service = %v, want core.ErrNotFound (the gate, not a missing id)", err)
	}
}

// TestActiveServiceDeployHistoryUnaffected guards the regression: a live
// service still lists its deploy history.
func TestActiveServiceDeployHistoryUnaffected(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	if _, err := ds.CreateDeploy(context.Background(), "srv-1", "manual", "web:v1", 1, store.CommitInfo{}); err != nil {
		t.Fatalf("seed deploy: %v", err)
	}
	list, err := svc.List(context.Background(), "web", ListFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("List deploys for a live service = %v len=%d, want the one row", err, len(list))
	}
}
