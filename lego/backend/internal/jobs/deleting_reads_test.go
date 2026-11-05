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

package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/503: the one-off job reads of a service whose deletion is accepted are as
// absent as the service itself.
func TestJobReadsOfADeletingServiceAreNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	ts := metav1.NewTime(time.Unix(1_700_000_000, 0))
	gone := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: "gone", Namespace: "default", DeletionTimestamp: &ts, Finalizers: []string{"app.bex.co/finalizer"},
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(client.Object(gone)).Build()
	svc := &Service{Base: &core.Base{Authz: allowAllChecker{}, Client: cl, Namespace: "default"}, Store: &recordingJobStore{}}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})

	if _, err := svc.List(ctx, "gone", ListFilter{}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("List on a deleting service = %v, want ErrNotFound", err)
	}
	if _, err := svc.Get(ctx, "gone", "job-1"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Get on a deleting service = %v, want ErrNotFound", err)
	}
}
