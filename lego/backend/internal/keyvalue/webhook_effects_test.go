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

type webhookAuditSink struct{ events []core.AuditEvent }

func (s *webhookAuditSink) Record(_ context.Context, event core.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

func TestKeyValuePlanEffectIsSuccessfulAndTyped(t *testing.T) {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-test", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Name: "cache", Plan: "free"},
	}
	svc, _ := newService(kv)
	sink := &webhookAuditSink{}
	svc.Audit = sink
	ctx := context.Background()

	if _, err := svc.SetPlan(ctx, kv.Name, "starter"); err != nil {
		t.Fatalf("SetPlan: %v", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %d, want 1", len(sink.events))
	}
	event := sink.events[0]
	if event.Verb != core.AuditVerbKeyValuePlanChanged || event.Target != core.KeyValueTarget(kv.Name) || event.TargetName != "cache" {
		t.Fatalf("plan event = %+v", event)
	}

	if _, err := svc.SetPlan(ctx, kv.Name, "not-a-plan"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("invalid SetPlan = %v, want ErrBadRequest", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("failed validation emitted an event: %+v", sink.events)
	}

	name := "renamed-cache"
	if _, err := svc.UpdateKeyValue(ctx, kv.Name, KeyValuePatch{Name: &name}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if got := sink.events[len(sink.events)-1].Verb; got != core.AuditVerbKeyValueUpdated {
		t.Fatalf("unrelated update verb = %q", got)
	}
}
