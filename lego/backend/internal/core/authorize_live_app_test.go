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

// w4/503: a by-id read of a service whose deletion is accepted is as absent as
// a service that never existed; an active one reads normally.
func TestAuthorizeLiveAppHidesADeletingService(t *testing.T) {
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	live := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "live", Namespace: "default"}}
	gone := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: "gone", Namespace: "default", DeletionTimestamp: &now, Finalizers: []string{"app.bex.co/finalizer"},
	}}
	b := &Base{Client: fakeAppClient(live, gone), Namespace: "default"}
	ctx := context.Background()

	if a, err := b.AuthorizeLiveApp(ctx, RelCanView, "live"); err != nil || a.Name != "live" {
		t.Fatalf("live = %v, %v; want the app", a, err)
	}
	if _, err := b.AuthorizeLiveApp(ctx, RelCanView, "gone"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting = %v, want ErrNotFound", err)
	}
	if _, err := b.AuthorizeLiveApp(ctx, RelCanView, "never-existed"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown = %v, want ErrNotFound", err)
	}
	// Teardown keeps AuthorizeApp, which still reaches the CR being removed.
	if a, err := b.AuthorizeApp(ctx, RelCanView, "gone"); err != nil || a.Name != "gone" {
		t.Fatalf("AuthorizeApp on a deleting service = %v, %v; want the app", a, err)
	}
}
