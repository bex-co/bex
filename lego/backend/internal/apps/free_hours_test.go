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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func freeHoursSuspendedApp(name, month string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Tier = "free"
	a.Spec.Type = appv1alpha1.TypeWebService
	a.Spec.Suspended = true
	a.Annotations = map[string]string{core.AnnotFreeHoursSuspended: month}
	return a
}

// w8/m139: a service suspended by the monthly Free instance hours reports the
// billing suspender, cannot be resumed by hand that month, and runs again when
// it moves to a paid plan or the month ends.
func TestFreeHoursSuspensionRules(t *testing.T) {
	ctx := context.Background()
	month := FreeHoursMonth(time.Now())

	svc, _ := newService(nil, freeHoursSuspendedApp("capped", month))
	v, err := svc.Get(ctx, "capped")
	if err != nil || !v.Suspended || strings.Join(core.SuspendersFor(v.Suspended, v.FreeHoursSuspended), ",") != "billing" {
		t.Fatalf("view = %+v err=%v, want suspended by billing", v, err)
	}
	if _, err := svc.Resume(ctx, "capped"); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "Free instance hours") {
		t.Fatalf("manual resume: want the exhausted-hours refusal, got %v", err)
	}

	if _, err := svc.SetPlan(ctx, "capped", "starter"); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	after, err := svc.Get(ctx, "capped")
	if err != nil || after.Suspended || after.FreeHoursSuspended {
		t.Fatalf("an upgraded service must run at once: %+v err=%v", after, err)
	}

	stale, _ := newService(nil, freeHoursSuspendedApp("lastmonth", "2020-01"))
	if _, err := stale.Resume(ctx, "lastmonth"); err != nil {
		t.Fatalf("a previous month's marker must not block resume: %v", err)
	}
}
