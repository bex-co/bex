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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/503: every other by-id read under /v1/services/{id} answers the same
// ErrNotFound as Get once the service's deletion is accepted. Add a read verb
// here when one is added.
func TestDeletingServiceIsAbsentFromTheRemainingByIDReads(t *testing.T) {
	cron := deletingApp("gone")
	cron.Spec.Type = appv1alpha1.TypeCronJob
	svc, _ := newService(nil, cron)
	ctx := context.Background()

	reads := map[string]func() error{
		"ListDomains":    func() error { _, err := svc.ListDomains(ctx, "gone"); return err },
		"GetDomain":      func() error { _, err := svc.GetDomain(ctx, "gone", "example.com"); return err },
		"ListInstances":  func() error { _, err := svc.ListInstances(ctx, "gone"); return err },
		"GetAutoscaling": func() error { _, err := svc.GetAutoscaling(ctx, "gone"); return err },
		"OutboundIPs":    func() error { _, err := svc.OutboundIPs(ctx, "gone"); return err },
		"ListDisks":      func() error { _, err := svc.ListDisks(ctx, "gone"); return err },
		"ListCronRuns":   func() error { _, err := svc.ListCronRuns(ctx, "gone", "", 10); return err },
		"GetCronRun":     func() error { _, err := svc.GetCronRun(ctx, "gone", "run-1"); return err },
	}
	for name, read := range reads {
		if err := read(); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("%s on a deleting service = %v, want core.ErrNotFound", name, err)
		}
	}

	// Control: an active service still reads.
	active, _ := newService(nil, sampleApp("live"))
	if _, err := active.ListInstances(ctx, "live"); err != nil {
		t.Errorf("ListInstances on an active service = %v", err)
	}
	if _, err := active.GetAutoscaling(ctx, "live"); err != nil {
		t.Errorf("GetAutoscaling on an active service = %v", err)
	}
	if _, err := active.ListDomains(ctx, "live"); err != nil {
		t.Errorf("ListDomains on an active service = %v", err)
	}
}
