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
	"github.com/bex-co/bex/lego/backend/internal/eventvocab"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type webhookAuditSink struct{ events []core.AuditEvent }

func (s *webhookAuditSink) Record(_ context.Context, event core.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

// w6/031: CreateKeyValue audited at authorize time, so a create that then
// failed the plan gate, the billing gate, or the API-server write still left a
// successful "keyvalue.CreateKeyValue" row behind — an audit log naming a store
// that never existed. The row is now deferred to after the CR actually lands,
// matching CreatePostgres.
func TestKeyValueCreateAuditsOnlyAfterTheStoreExists(t *testing.T) {
	t.Run("successful create records one typed row", func(t *testing.T) {
		svc, _ := newService()
		svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
		sink := &webhookAuditSink{}
		svc.Audit = sink

		view, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "cache", Plan: "free"})
		if err != nil {
			t.Fatalf("CreateKeyValue: %v", err)
		}
		if len(sink.events) != 1 {
			t.Fatalf("events = %d, want 1: %+v", len(sink.events), sink.events)
		}
		event := sink.events[0]
		if event.Verb != core.AuditVerbKeyValueCreated ||
			event.Target != core.KeyValueTarget(view.ID) ||
			event.TargetName != "cache" {
			t.Fatalf("create event = %+v", event)
		}
	})

	t.Run("refused create records nothing", func(t *testing.T) {
		svc, _ := newService()
		svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
		svc.Payment = &rejectingPaymentGate{}
		sink := &webhookAuditSink{}
		svc.Audit = sink

		if _, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "cache", Plan: "starter"}); !errors.Is(err, core.ErrPaymentRequired) {
			t.Fatalf("err = %v, want ErrPaymentRequired", err)
		}
		if len(sink.events) != 0 {
			t.Fatalf("a refused create recorded %d event(s): %+v", len(sink.events), sink.events)
		}
	})

	t.Run("dry run records nothing", func(t *testing.T) {
		svc, _ := newService()
		svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
		sink := &webhookAuditSink{}
		svc.Audit = sink

		if _, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "cache", Plan: "free", DryRun: true}); err != nil {
			t.Fatalf("dry-run CreateKeyValue: %v", err)
		}
		if len(sink.events) != 0 {
			t.Fatalf("a dry run recorded %d event(s): %+v", len(sink.events), sink.events)
		}
	})
}

// Render publishes no Key Value create event type, and bex does not invent
// names for lifecycle writes it documents as unsupported
// (docs/render-artifacts/datastore-webhook-events.md) — so the new audit verb
// must stay out of the outbound webhook vocabulary.
func TestKeyValueCreatedIsAuditOnlyAndNotAWebhookEvent(t *testing.T) {
	if _, ok := eventvocab.DatastoreAuditTypes()[core.AuditVerbKeyValueCreated]; ok {
		t.Fatal("keyvalue.CreateKeyValue projects to a webhook event; Render has no key_value_created type")
	}
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

// TestKeyValueConfigChangeProjectsConfigRestart is w3/m82 t003. Advertising
// key_value_config_restart is verify-first, and the mechanism verifies: the
// operator derives the Valkey server flags from spec.maxmemoryPolicy and
// spec.persistenceMode (valkeyArgs), those flags are the StatefulSet's
// container args, and the reconcile is a CreateOrUpdate on that StatefulSet —
// so either change rolls the single pod. This pins the API half: the change
// records a verb carrying the configuration the instance restarted into.
func TestKeyValueConfigChangeProjectsConfigRestart(t *testing.T) {
	newStore := func() *appv1alpha1.KeyValue {
		return &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: "red-cfg", Namespace: "default"},
			Spec: appv1alpha1.KeyValueSpec{
				Name: "cache", Plan: "free",
				MaxmemoryPolicy: "allkeys-lru", PersistenceMode: "off",
			},
		}
	}
	policy, mode := "noeviction", "journal_snapshot"
	cases := []struct {
		name                 string
		patch                KeyValuePatch
		wantPolicy, wantMode string
	}{
		{"eviction policy", KeyValuePatch{MaxmemoryPolicy: &policy}, "noeviction", "off"},
		{"persistence mode", KeyValuePatch{PersistenceMode: &mode}, "allkeys_lru", "journal_snapshot"},
		{"both at once", KeyValuePatch{MaxmemoryPolicy: &policy, PersistenceMode: &mode}, "noeviction", "journal_snapshot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kv := newStore()
			svc, _ := newService(kv)
			sink := &webhookAuditSink{}
			svc.Audit = sink

			if _, err := svc.UpdateKeyValue(context.Background(), kv.Name, tc.patch); err != nil {
				t.Fatalf("UpdateKeyValue: %v", err)
			}
			if len(sink.events) != 1 {
				t.Fatalf("events = %d, want 1: %+v", len(sink.events), sink.events)
			}
			event := sink.events[0]
			if event.Verb != core.AuditVerbKeyValueConfigChanged {
				t.Fatalf("verb = %q, want %q (never the generic %s)",
					event.Verb, core.AuditVerbKeyValueConfigChanged, core.AuditVerbKeyValueUpdated)
			}
			if event.Target != core.KeyValueTarget(kv.Name) || event.TargetName != "cache" {
				t.Errorf("event identity = target %q name %q", event.Target, event.TargetName)
			}
			// Both resulting values, Render-shaped: the event states the whole
			// configuration the instance restarted into, not just the delta.
			if event.MaxmemoryPolicy == nil || *event.MaxmemoryPolicy != tc.wantPolicy {
				t.Errorf("maxmemoryPolicy = %v, want %q", event.MaxmemoryPolicy, tc.wantPolicy)
			}
			if event.PersistenceMode == nil || *event.PersistenceMode != tc.wantMode {
				t.Errorf("persistenceMode = %v, want %q", event.PersistenceMode, tc.wantMode)
			}
			if got := eventvocab.DatastoreAuditTypes()[core.AuditVerbKeyValueConfigChanged]; got != eventvocab.TypeKeyValueConfigRestart {
				t.Errorf("%s projects to %q, want %q", core.AuditVerbKeyValueConfigChanged, got, eventvocab.TypeKeyValueConfigRestart)
			}
		})
	}

	t.Run("idempotent set is not a restart", func(t *testing.T) {
		kv := newStore()
		svc, _ := newService(kv)
		sink := &webhookAuditSink{}
		svc.Audit = sink

		same := "allkeys_lru"
		if _, err := svc.UpdateKeyValue(context.Background(), kv.Name, KeyValuePatch{MaxmemoryPolicy: &same}); err != nil {
			t.Fatalf("idempotent UpdateKeyValue: %v", err)
		}
		if got := sink.events[len(sink.events)-1].Verb; got != core.AuditVerbKeyValueUpdated {
			t.Fatalf("idempotent config set recorded %q, want the generic %s", got, core.AuditVerbKeyValueUpdated)
		}
	})
}

// TestKeyValueSetPlanAuditVerbAndPlanPair is w10/m5: SetPlan always records
// the plan verb with the typed from/to pair — an idempotent same-plan set no
// longer masquerades as UpdateKeyValue (the pair is simply equal).
func TestKeyValueSetPlanAuditVerbAndPlanPair(t *testing.T) {
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
	if _, err := svc.SetPlan(ctx, kv.Name, "starter"); err != nil {
		t.Fatalf("idempotent SetPlan: %v", err)
	}
	if len(sink.events) != 2 {
		t.Fatalf("events = %d, want 2", len(sink.events))
	}
	change, noop := sink.events[0], sink.events[1]
	if change.Verb != core.AuditVerbKeyValuePlanChanged ||
		change.PlanFrom == nil || *change.PlanFrom != "free" ||
		change.PlanTo == nil || *change.PlanTo != "starter" {
		t.Fatalf("plan change event = %+v", change)
	}
	if noop.Verb != core.AuditVerbKeyValuePlanChanged ||
		noop.PlanFrom == nil || *noop.PlanFrom != "starter" ||
		noop.PlanTo == nil || *noop.PlanTo != "starter" {
		t.Fatalf("idempotent set event = %+v (want SetPlan verb with equal pair, never %s)",
			noop, core.AuditVerbKeyValueUpdated)
	}
}
