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

package core

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/503, w8/366: every verb on a service whose deletion is accepted finds it as
// absent as a service that never existed, through either seam; an active one
// resolves normally.
func TestAuthorizeAppHidesADeletingService(t *testing.T) {
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	live := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "live", Namespace: "default"}}
	gone := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: "gone", Namespace: "default", DeletionTimestamp: &now, Finalizers: []string{"app.bex.co/finalizer"},
	}}
	b := &Base{Client: fakeAppClient(live, gone), Namespace: "default"}
	ctx := context.Background()

	for _, relation := range []string{RelCanView, RelCanViewLogs, RelCanViewSensitive, RelCanOperate, RelCanCreate} {
		if a, err := b.AuthorizeApp(ctx, relation, "live"); err != nil || a.Name != "live" {
			t.Fatalf("%s live = %v, %v; want the app", relation, a, err)
		}
		if a, err := b.AuthorizeApp(ctx, relation, "gone"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s deleting = %v, %v; want ErrNotFound", relation, a, err)
		}
		if _, err := b.AuthorizeAppAs(ctx, "feature.Named", relation, "gone"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s deleting through AuthorizeAppAs = %v; want ErrNotFound", relation, err)
		}
		if _, err := b.AuthorizeApp(ctx, relation, "never-existed"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s unknown = %v; want ErrNotFound", relation, err)
		}
	}
}

// A caller who may not see a deleting service is refused as before: the
// deletion check runs after authorization, so it is no existence oracle.
func TestAuthorizeAppRefusesBeforeHidingADeletingService(t *testing.T) {
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	gone := sampleApp("gone", "tea-a")
	gone.DeletionTimestamp, gone.Finalizers = &now, []string{"app.bex.co/finalizer"}
	b := resolvingBase(gone)
	if _, err := b.AuthorizeApp(actingCtx(), RelCanView, "gone"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("member on a deleting service = %v, want ErrNotFound", err)
	}
	b.Authz = fakeDenyChecker{}
	if _, err := b.AuthorizeApp(actingCtx(), RelCanView, "gone"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("denied caller on a deleting service = %v, want ErrForbidden", err)
	}
}
