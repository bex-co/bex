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

package events

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/503: the activity feed of a service whose deletion is accepted is as
// absent as the service itself.
func TestEventsListOfADeletingServiceIsNotFound(t *testing.T) {
	gone := sampleApp("gone", "srv-gone", "")
	ts := metav1.NewTime(time.Unix(1_700_000_000, 0))
	gone.DeletionTimestamp = &ts
	gone.Finalizers = []string{"app.bex.co/finalizer"}
	svc := newService(&fakeStore{}, gone, sampleApp("live", "srv-live", ""))

	if _, err := svc.List(context.Background(), "gone", Filter{}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("List on a deleting service = %v, want ErrNotFound", err)
	}
	if _, err := svc.List(context.Background(), "live", Filter{}); err != nil {
		t.Fatalf("List on an active service = %v", err)
	}
}
