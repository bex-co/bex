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
	"errors"
	"fmt"
	"reflect"
	"slices"
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

func TestSourceablePostgresWebhookEffectsRecordOnlyAfterSuccess(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-test", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "orders", Plan: "free",
		},
		Status: appv1alpha1.DatabaseStatus{BackupsEnabled: true},
	}
	svc, _ := newService(db)
	sink := &webhookAuditSink{}
	svc.Audit = sink
	ctx := context.Background()

	if _, err := svc.Restart(ctx, db.Name); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if _, err := svc.SetPlan(ctx, db.Name, "basic-256mb"); err != nil {
		t.Fatalf("SetPlan: %v", err)
	}
	if _, err := svc.CreateUser(ctx, db.Name, "reporter"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := svc.DeleteUser(ctx, db.Name, "reporter"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := svc.CreateExport(ctx, db.Name); err != nil {
		t.Fatalf("CreateExport: %v", err)
	}

	want := []string{
		core.AuditVerbPostgresRestarted,
		core.AuditVerbPostgresPlanChanged,
		core.AuditVerbPostgresCredentialsCreated,
		core.AuditVerbPostgresCredentialsDeleted,
		core.AuditVerbPostgresBackupStarted,
	}
	got := make([]string, 0, len(sink.events))
	for _, event := range sink.events {
		got = append(got, event.Verb)
		if event.Target != core.DatabaseTarget(db.Name) || event.TargetName != "orders" {
			t.Errorf("event identity = target %q name %q", event.Target, event.TargetName)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("effects = %v, want %v", got, want)
	}

	before := len(sink.events)
	if _, err := svc.SetPlan(ctx, db.Name, "not-a-plan"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("invalid SetPlan = %v, want ErrBadRequest", err)
	}
	if len(sink.events) != before {
		t.Fatalf("failed validation emitted %d events", len(sink.events)-before)
	}
}

// TestPostgresSetPlanAuditVerbAndPlanPair is w10/m5: SetPlan always records
// the plan verb with the typed from/to pair — an idempotent same-plan set no
// longer masquerades as UpdatePostgres (the pair is simply equal).
func TestPostgresSetPlanAuditVerbAndPlanPair(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-test", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "free"},
	}
	svc, _ := newService(db)
	sink := &webhookAuditSink{}
	svc.Audit = sink
	ctx := context.Background()

	if _, err := svc.SetPlan(ctx, db.Name, "basic-256mb"); err != nil {
		t.Fatalf("SetPlan: %v", err)
	}
	if _, err := svc.SetPlan(ctx, db.Name, "basic-256mb"); err != nil {
		t.Fatalf("idempotent SetPlan: %v", err)
	}
	if len(sink.events) != 2 {
		t.Fatalf("events = %d, want 2", len(sink.events))
	}
	change, noop := sink.events[0], sink.events[1]
	if change.Verb != core.AuditVerbPostgresPlanChanged ||
		change.PlanFrom == nil || *change.PlanFrom != "free" ||
		change.PlanTo == nil || *change.PlanTo != "basic-256mb" {
		t.Fatalf("plan change event = %+v", change)
	}
	if noop.Verb != core.AuditVerbPostgresPlanChanged ||
		noop.PlanFrom == nil || *noop.PlanFrom != "basic-256mb" ||
		noop.PlanTo == nil || *noop.PlanTo != "basic-256mb" {
		t.Fatalf("idempotent set event = %+v (want SetPlan verb with equal pair, never %s)",
			noop, core.AuditVerbPostgresUpdated)
	}
}

// TestPostgresConfigurationEffectsProjectRenderNames is w3/m82 t003: a PATCH
// that changes high availability, the connection pooler, or the disk size now
// records a fixed verb carrying the value it set, so Render's
// postgres_ha_status_changed / postgres_connection_pool_enabled_changed /
// postgres_disk_size_changed each have a producer instead of landing as one
// undifferentiated postgres.UpdatePostgres row.
func TestPostgresConfigurationEffectsProjectRenderNames(t *testing.T) {
	enabled, disabled := true, false
	grown := int32(40)
	cases := []struct {
		name      string
		patch     PostgresPatch
		wantVerb  string
		wantEvent string
		check     func(core.AuditEvent) error
	}{
		{
			name: "high availability on", patch: PostgresPatch{EnableHighAvailability: &enabled},
			wantVerb: core.AuditVerbPostgresHAChanged, wantEvent: eventvocab.TypePostgresHAStatusChanged,
			check: wantBoolDetail("highAvailabilityEnabled", func(e core.AuditEvent) *bool { return e.HighAvailabilityEnabled }, true),
		},
		{
			name: "connection pool on", patch: PostgresPatch{Pooler: &enabled},
			wantVerb: core.AuditVerbPostgresPoolerChanged, wantEvent: eventvocab.TypePostgresConnectionPoolEnabledChanged,
			check: wantBoolDetail("connectionPoolEnabled", func(e core.AuditEvent) *bool { return e.ConnectionPoolEnabled }, true),
		},
		{
			name: "disk grown", patch: PostgresPatch{DiskSizeGB: &grown},
			wantVerb: core.AuditVerbPostgresDiskSizeChanged, wantEvent: eventvocab.TypePostgresDiskSizeChanged,
			check: func(e core.AuditEvent) error {
				if e.DiskSizeGB == nil || *e.DiskSizeGB != grown {
					return fmt.Errorf("diskSizeGB = %v, want %d", e.DiskSizeGB, grown)
				}
				return nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := &appv1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "dpg-cfg", Namespace: "default"},
				Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "basic-1gb", StorageGB: 20},
			}
			svc, _ := newService(db)
			sink := &webhookAuditSink{}
			svc.Audit = sink

			if _, err := svc.UpdatePostgres(context.Background(), db.Name, tc.patch); err != nil {
				t.Fatalf("UpdatePostgres: %v", err)
			}
			if len(sink.events) != 1 {
				t.Fatalf("events = %d, want 1: %+v", len(sink.events), sink.events)
			}
			event := sink.events[0]
			if event.Verb != tc.wantVerb {
				t.Fatalf("verb = %q, want %q (never the generic %s)", event.Verb, tc.wantVerb, core.AuditVerbPostgresUpdated)
			}
			if event.Target != core.DatabaseTarget(db.Name) || event.TargetName != "orders" {
				t.Errorf("event identity = target %q name %q", event.Target, event.TargetName)
			}
			if err := tc.check(event); err != nil {
				t.Error(err)
			}
			if got := eventvocab.DatastoreAuditTypes()[tc.wantVerb]; got != tc.wantEvent {
				t.Errorf("%s projects to %q, want %q", tc.wantVerb, got, tc.wantEvent)
			}
		})
	}

	// Turning a setting back off is the same verb with the opposite value —
	// the auto_deploy_enabled shape, not a second event name.
	t.Run("high availability off", func(t *testing.T) {
		db := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-ha", Namespace: "default"},
			Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "basic-1gb", HighAvailability: true},
		}
		svc, _ := newService(db)
		sink := &webhookAuditSink{}
		svc.Audit = sink

		if _, err := svc.UpdatePostgres(context.Background(), db.Name, PostgresPatch{EnableHighAvailability: &disabled}); err != nil {
			t.Fatalf("UpdatePostgres: %v", err)
		}
		event := sink.events[len(sink.events)-1]
		if event.Verb != core.AuditVerbPostgresHAChanged ||
			event.HighAvailabilityEnabled == nil || *event.HighAvailabilityEnabled {
			t.Fatalf("HA disable event = %+v", event)
		}
	})

	// One atomic PATCH that changes two named fields records one row per field
	// effect — the maintenance-mode shape.
	t.Run("two fields in one patch record two effects", func(t *testing.T) {
		db := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-both", Namespace: "default"},
			Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "basic-1gb", StorageGB: 20},
		}
		svc, _ := newService(db)
		sink := &webhookAuditSink{}
		svc.Audit = sink

		if _, err := svc.UpdatePostgres(context.Background(), db.Name,
			PostgresPatch{EnableHighAvailability: &enabled, DiskSizeGB: &grown}); err != nil {
			t.Fatalf("UpdatePostgres: %v", err)
		}
		got := make([]string, 0, len(sink.events))
		for _, event := range sink.events {
			got = append(got, event.Verb)
		}
		want := []string{core.AuditVerbPostgresHAChanged, core.AuditVerbPostgresDiskSizeChanged}
		if !slices.Equal(got, want) {
			t.Fatalf("effects = %v, want %v", got, want)
		}
	})
}

func wantBoolDetail(field string, read func(core.AuditEvent) *bool, want bool) func(core.AuditEvent) error {
	return func(event core.AuditEvent) error {
		got := read(event)
		if got == nil || *got != want {
			return fmt.Errorf("%s = %v, want %t", field, got, want)
		}
		return nil
	}
}

// A PATCH that touches only fields Render's datastore vocabulary has no name
// for must keep recording the generic update row — the pre-t003 behavior for
// rename, ip-allow-list, and parameter overrides.
func TestPostgresUnnamedFieldsStillRecordTheGenericUpdate(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-rename", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "basic-1gb", StorageGB: 20, HighAvailability: true},
	}
	svc, _ := newService(db)
	sink := &webhookAuditSink{}
	svc.Audit = sink
	ctx := context.Background()

	renamed := "orders-eu"
	if _, err := svc.UpdatePostgres(ctx, db.Name, PostgresPatch{Name: &renamed}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	// An idempotent re-assert of an already-enabled setting is not a change.
	stillEnabled := true
	if _, err := svc.UpdatePostgres(ctx, db.Name, PostgresPatch{EnableHighAvailability: &stillEnabled}); err != nil {
		t.Fatalf("idempotent HA set: %v", err)
	}
	got := make([]string, 0, len(sink.events))
	for _, event := range sink.events {
		got = append(got, event.Verb)
	}
	want := []string{core.AuditVerbPostgresUpdated, core.AuditVerbPostgresUpdated}
	if !slices.Equal(got, want) {
		t.Fatalf("effects = %v, want %v", got, want)
	}
}

// Render's postgres_read_replicas_changed has no bex producer: read replicas
// are create-time only (PostgresPatch has no ReadReplicas field), so the name
// must stay out of the advertised vocabulary rather than be advertised empty.
func TestPostgresReadReplicasChangedHasNoProducerAndIsNotAdvertised(t *testing.T) {
	for _, name := range eventvocab.DatastoreAuditTypes() {
		if name == "postgres_read_replicas_changed" {
			t.Fatal("postgres_read_replicas_changed is advertised, but no verb mutates read replicas after create")
		}
	}
	if _, ok := reflect.TypeOf(PostgresPatch{}).FieldByName("ReadReplicas"); ok {
		t.Fatal("PostgresPatch gained a ReadReplicas field — record the resulting replica set and advertise postgres_read_replicas_changed")
	}
}

func TestPostgresCreatedEffectCarriesMintedIDAndDisplayName(t *testing.T) {
	svc, _ := newService()
	sink := &webhookAuditSink{}
	svc.Audit = sink

	view, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "orders", Plan: "free"})
	if err != nil {
		t.Fatalf("CreatePostgres: %v", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("create events = %d, want 1", len(sink.events))
	}
	event := sink.events[0]
	if event.Verb != core.AuditVerbPostgresCreated || event.Target != core.DatabaseTarget(view.ID) || event.TargetName != "orders" {
		t.Fatalf("created event = %+v", event)
	}
}
