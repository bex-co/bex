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
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func diskInvariantService(t *testing.T) (*Service, *recordingStore) {
	t.Helper()
	app := diskEligibleApp("disk-web")
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/data", SizeGB: 1}
	svc, _, rec := newDiskService(app)
	return svc, rec
}

func TestDiskInvariantRefusalsPrecedeIntentAndBilling(t *testing.T) {
	cpu := int32(60)
	for _, operation := range []struct {
		name string
		run  func(*Service) error
	}{
		{"free plan", func(s *Service) error { _, err := s.SetPlan(context.Background(), "disk-web", "free"); return err }},
		{"preview free plan", func(s *Service) error {
			_, err := s.PreviewSetPlan(context.Background(), "disk-web", "free")
			return err
		}},
		{"multiple instances", func(s *Service) error { _, err := s.Scale(context.Background(), "disk-web", 2); return err }},
		{"autoscaling", func(s *Service) error {
			_, err := s.SetAutoscaling(context.Background(), "disk-web", SetAutoscalingRequest{MinInstances: 1, MaxInstances: 1, TargetCPUPercent: &cpu})
			return err
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			svc, rec := diskInvariantService(t)
			before := getApp(t, svc.Client, "disk-web")
			gate := &enforcingBillingGate{}
			svc.Billing = gate
			err := operation.run(svc)
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "disk") {
				t.Fatalf("refusal = %v", err)
			}
			if len(rec.tierCalls)+len(rec.replicasCalls)+len(rec.deployCalls) != 0 {
				t.Fatalf("refusal wrote intent: %+v", rec)
			}
			if len(gate.calls) != 0 {
				t.Fatalf("refusal reached billing: %v", gate.calls)
			}
			after := getApp(t, svc.Client, "disk-web")
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal changed CR: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestDiskInvariantAllowedTransitions(t *testing.T) {
	t.Run("paid plan and preview", func(t *testing.T) {
		svc, rec := diskInvariantService(t)
		before := getApp(t, svc.Client, "disk-web")
		if _, err := svc.PreviewSetPlan(context.Background(), "disk-web", "standard"); err != nil {
			t.Fatal(err)
		}
		if len(rec.tierCalls) != 0 || !reflect.DeepEqual(before, getApp(t, svc.Client, "disk-web")) {
			t.Fatal("preview wrote intent")
		}
		if _, err := svc.SetPlan(context.Background(), "disk-web", "standard"); err != nil {
			t.Fatal(err)
		}
		after := getApp(t, svc.Client, "disk-web")
		if len(rec.tierCalls) != 1 || rec.tierCalls[0].tier != after.Spec.Tier || after.Spec.Tier == before.Spec.Tier || !reflect.DeepEqual(before.Spec.Disk, after.Spec.Disk) {
			t.Fatalf("paid change: CR=%+v writes=%+v", after.Spec, rec.tierCalls)
		}
	})
	t.Run("one instance", func(t *testing.T) {
		svc, rec := diskInvariantService(t)
		if _, err := svc.Scale(context.Background(), "disk-web", 1); err != nil {
			t.Fatal(err)
		}
		if len(rec.replicasCalls) != 1 || rec.replicasCalls[0].replicas != 1 || getApp(t, svc.Client, "disk-web").Spec.Replicas != 1 {
			t.Fatalf("scale1 writes=%+v", rec.replicasCalls)
		}
	})
	t.Run("disable stale autoscaling", func(t *testing.T) {
		svc, _ := diskInvariantService(t)
		app := getApp(t, svc.Client, "disk-web")
		app.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 1, MaxReplicas: 2}
		if err := svc.Client.Update(context.Background(), app); err != nil {
			t.Fatal(err)
		}
		if err := svc.DeleteAutoscaling(context.Background(), "disk-web"); err != nil {
			t.Fatal(err)
		}
		after := getApp(t, svc.Client, "disk-web")
		if after.Spec.Autoscaling != nil || after.Spec.Disk == nil {
			t.Fatalf("disable did not preserve disk: %+v", after.Spec)
		}
	})
}
