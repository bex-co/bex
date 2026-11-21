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

package envgroups

import (
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/366: a service whose delete was accepted cannot be linked to a group, on
// create or afterwards: it is absent, as it is to every verb that names it,
// and no group is created around it.
func TestADeletingServiceCannotJoinAGroup(t *testing.T) {
	ctx := testCtx()
	gone := sampleApp("gone")
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	gone.DeletionTimestamp, gone.Finalizers = &now, []string{"app.bex.co/finalizer"}
	svc := newService(newFakeStore(), gone, sampleApp("web"))

	if _, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "with-gone", ServiceIDs: []string{"gone"}}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("create a group linking a deleting service = %v, want core.ErrNotFound", err)
	}
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", ServiceIDs: []string{"web"}})
	if err != nil {
		t.Fatalf("create a group linking a live service: %v", err)
	}
	if err := svc.LinkService(ctx, g.ID, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("link a deleting service = %v, want core.ErrNotFound", err)
	}
	if got, err := svc.GetEnvGroup(ctx, g.ID); err != nil || len(got.ServiceLinks) != 1 {
		t.Fatalf("group after the refused link = %+v, %v; want only web", got, err)
	}
}
