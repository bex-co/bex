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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w2/m253: a crash-loop suspension reports Render's stuck_crashlooping
// suspender on every read, and an owner's resume — unlike a Free-hours one —
// is allowed and clears it.
func TestCrashLoopSuspensionReportsAndResumes(t *testing.T) {
	ctx := context.Background()
	a := sampleApp("looping")
	a.Spec.Suspended = true
	a.Annotations = map[string]string{core.AnnotCrashLoopSuspended: "2026-11-03T12:00:00Z"}
	svc, cl := newService(nil, a)

	v, err := svc.Get(ctx, "looping")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(core.SuspendersFor(v.Suspended, v.FreeHoursSuspended, v.CrashLoopSuspended), ","); got != "stuck_crashlooping" {
		t.Fatalf("suspenders = %q, want stuck_crashlooping", got)
	}
	if got := toRenderService(v).Suspenders; strings.Join(got, ",") != "stuck_crashlooping" {
		t.Errorf("REST suspenders = %v", got)
	}

	if _, err := svc.Resume(ctx, "looping"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	after := getApp(t, cl, "looping")
	if after.Spec.Suspended || after.Annotations[core.AnnotCrashLoopSuspended] != "" {
		t.Errorf("resume left suspended=%v annotation=%q", after.Spec.Suspended, after.Annotations[core.AnnotCrashLoopSuspended])
	}

	// A user suspension stays "user".
	if got := core.SuspendersFor(true, false, false); strings.Join(got, ",") != "user" {
		t.Errorf("user suspenders = %v", got)
	}
}
