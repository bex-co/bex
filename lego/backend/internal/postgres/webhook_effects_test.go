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
	"slices"
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
