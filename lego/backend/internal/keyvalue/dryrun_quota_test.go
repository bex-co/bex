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

package keyvalue

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

// TestDryRunCreateKeyValueRefusesPastPlanCap is w8/046: a Hobby workspace
// already holding its one Key Value gets the real create's cap refusal from a
// dry run, and nothing is written.
func TestDryRunCreateKeyValueRefusesPastPlanCap(t *testing.T) {
	svc, cl := newService(atCapQuota(store.KeyValuesQuotaCountKey))
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	_, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "cache", Plan: "free", DryRun: true})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "limited to 1 key-value stores") {
		t.Fatalf("dry-run past cap = %v, want the cap refusal", err)
	}
	var list appv1alpha1.KeyValueList
	if err := cl.List(context.Background(), &list); err != nil || len(list.Items) != 0 {
		t.Fatalf("dry-run refusal wrote KeyValues: %+v err=%v", list.Items, err)
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
