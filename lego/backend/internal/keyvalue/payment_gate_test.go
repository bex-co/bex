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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type rejectingPaymentGate struct{ calls []string }

func (g *rejectingPaymentGate) RequirePaymentMethod(_ context.Context, workspaceID string) error {
	g.calls = append(g.calls, workspaceID)
	return core.NewPaymentRequiredError()
}

func TestPaidIntentGuardCoversKeyValueCreateAndBothPlanUpdatePaths(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		svc, _ := newService()
		svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
		gate := &rejectingPaymentGate{}
		svc.Payment = gate
		_, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "cache", Plan: "starter"})
		if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
			t.Fatalf("paid create err=%v calls=%v", err, gate.calls)
		}
		gate.calls = nil
		if _, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "free-kv", Plan: "free"}); err != nil {
			t.Fatalf("free create: %v", err)
		}
		if len(gate.calls) != 0 {
			t.Fatalf("free create consulted paid gate: %v", gate.calls)
		}
	})

	keyValue := func() *appv1alpha1.KeyValue {
		return &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: "red-test", Namespace: "default", Labels: map[string]string{core.LabelTenant: "tea-a"}},
			Spec:       appv1alpha1.KeyValueSpec{Name: "cache", Plan: "free"},
		}
	}
	for _, tc := range []struct {
		name string
		run  func(*Service) error
	}{
		{name: "dedicated SetPlan", run: func(s *Service) error { _, err := s.SetPlan(ctxAs("user-a"), "red-test", "starter"); return err }},
		{name: "REST-compatible PATCH plan", run: func(s *Service) error {
			plan := "starter"
			_, err := s.UpdateKeyValue(ctxAs("user-a"), "red-test", KeyValuePatch{Plan: &plan})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(keyValue())
			svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
			gate := &rejectingPaymentGate{}
			svc.Payment = gate
			if err := tc.run(svc); !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 {
				t.Fatalf("paid update err=%v calls=%v", err, gate.calls)
			}
		})
	}
}
