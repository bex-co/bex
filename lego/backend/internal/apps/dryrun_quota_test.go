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

package apps

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCreateDryRunRefusesPastPlanCap is w8/046: a workspace at its service
// count cap gets the real create's cap refusal from a dry run, and nothing is
// written.
func TestCreateDryRunRefusesPastPlanCap(t *testing.T) {
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	if err := cl.Create(context.Background(), atCapQuota(store.AppsQuotaCountKey)); err != nil {
		t.Fatalf("seed quota: %v", err)
	}
	_, err := svc.Create(paidGateContext(), CreateRequest{Name: "extra", Image: "nginx:alpine", DryRun: true})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "limited to 1 services") {
		t.Fatalf("dry-run past cap = %v, want the cap refusal", err)
	}
	var apps appv1alpha1.AppList
	if err := cl.List(context.Background(), &apps); err != nil || len(apps.Items) != 0 {
		t.Fatalf("dry-run refusal wrote Apps: %+v err=%v", apps.Items, err)
	}
}

func atCapQuota(key string) *corev1.ResourceQuota {
	name := corev1.ResourceName(key)
	one := *resource.NewQuantity(1, resource.DecimalSI)
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Namespace: "tea-a", Name: core.TenantQuotaName},
		Spec:       corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{name: one}},
		Status:     corev1.ResourceQuotaStatus{Used: corev1.ResourceList{name: one}},
	}
}
