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

package postgres

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestSetPlanAcceptsRenderSpecAlias is w8/011: Render CLI help's 0.1c-256mb
// maps onto bex basic-256mb rather than 400ing.
func TestSetPlanAcceptsRenderSpecAlias(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alias-db", Namespace: "default",
			Labels: map[string]string{core.LabelTenant: core.DefaultTenant},
		},
		Spec: appv1alpha1.DatabaseSpec{Name: "alias-db", Plan: "free"},
	}
	svc, cl := newService(db)
	view, err := svc.SetPlan(context.Background(), "alias-db", "0.1c-256mb")
	if err != nil {
		t.Fatalf("SetPlan(0.1c-256mb): %v", err)
	}
	if view.Plan != "basic-256mb" {
		t.Fatalf("view.Plan = %q, want basic-256mb", view.Plan)
	}
	var got appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "alias-db"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.Plan != "basic-256mb" {
		t.Fatalf("spec.plan = %q, want basic-256mb", got.Spec.Plan)
	}
}
