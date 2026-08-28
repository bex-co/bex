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

package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func mkIdleApp(tier string, ttl int32, lastActive time.Time, suspended bool) *appv1alpha1.App {
	ann := map[string]string{}
	if !lastActive.IsZero() {
		ann[annotLastActive] = lastActive.UTC().Format(time.RFC3339)
	}
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Annotations: ann},
		Spec: appv1alpha1.AppSpec{
			Tier:           tier,
			IdleTTLSeconds: ttl,
			Suspended:      suspended,
		},
	}
}

func TestIsFreeApp(t *testing.T) {
	cases := []struct {
		tier string
		want bool
	}{
		{"", true}, // empty = defaults to free
		{"free", true},
		{"starter", false},
		{"standard", false},
		{"pro", false},
		{"pro-plus", false},
	}
	for _, tc := range cases {
		app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Tier: tc.tier}}
		if got := isFreeApp(app); got != tc.want {
			t.Errorf("isFreeApp(%q) = %v, want %v", tc.tier, got, tc.want)
		}
	}
}

func TestAutoSleepWindow(t *testing.T) {
	cases := []struct {
		name string
		tier string
		ttl  int32
		want time.Duration
	}{
		{"free, no explicit ttl: platform default", "free", 0, defaultIdleTTL},
		{"empty tier (free default), no ttl: platform default", "", 0, defaultIdleTTL},
		{"free, explicit ttl overrides the default", "free", 600, 600 * time.Second},
		{"paid tier: never auto-sleeps, window 0", "starter", 0, 0},
		{"paid tier with a ttl set: still 0 (never sleeps)", "pro", 600, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := mkIdleApp(tc.tier, tc.ttl, time.Time{}, false)
			if got := autoSleepWindow(app); got != tc.want {
				t.Fatalf("autoSleepWindow = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestAutoSleepEligibleCreatedDefault pins the predicate all three callers gate
// on for the created-default (idleTTLSeconds:0) free web App — the shape that
// shipped never-sleeping. It must be eligible now (fails under the pre-w6/m116
// `IdleTTLSeconds > 0` predicate).
func TestAutoSleepEligibleCreatedDefault(t *testing.T) {
	app := mkIdleApp("free", 0, time.Time{}, false)
	app.Spec.Type = appv1alpha1.TypeWebService
	if !autoSleepEligible(app) {
		t.Fatal("a free web App with idleTTLSeconds:0 must be auto-sleep eligible (0 = platform default, not never)")
	}
	// Suspension and paid tier still exclude it.
	suspended := mkIdleApp("free", 0, time.Time{}, true)
	if autoSleepEligible(suspended) {
		t.Fatal("a suspended App must never be auto-sleep eligible")
	}
	paid := mkIdleApp("starter", 0, time.Time{}, false)
	if autoSleepEligible(paid) {
		t.Fatal("a paid App must never be auto-sleep eligible regardless of idleTTLSeconds")
	}
}

func TestAutoSleepEligibleByServiceType(t *testing.T) {
	cases := []struct {
		name  string
		type_ string
		want  bool
	}{
		{name: "legacy empty type", want: true},
		{name: "web service", type_: appv1alpha1.TypeWebService, want: true},
		{name: "private service", type_: appv1alpha1.TypePrivateService, want: false},
		{name: "background worker", type_: appv1alpha1.TypeBackgroundWorker, want: false},
		{name: "cron job", type_: appv1alpha1.TypeCronJob, want: false},
		{name: "static site", type_: appv1alpha1.TypeStaticSite, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := mkIdleApp("free", 300, time.Now().Add(-10*time.Minute), false)
			app.Spec.Type = tc.type_
			if got := autoSleepEligible(app); got != tc.want {
				t.Fatalf("autoSleepEligible = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLastActiveTime(t *testing.T) {
	t.Run("missing annotation returns zero", func(t *testing.T) {
		app := &appv1alpha1.App{}
		if !lastActiveTime(app).IsZero() {
			t.Fatal("expected zero time")
		}
	})
	t.Run("invalid annotation returns zero", func(t *testing.T) {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{annotLastActive: "not-a-time"},
			},
		}
		if !lastActiveTime(app).IsZero() {
			t.Fatal("expected zero time on bad parse")
		}
	})
	t.Run("valid RFC3339 annotation is parsed correctly", func(t *testing.T) {
		stamp := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{annotLastActive: stamp.Format(time.RFC3339)},
			},
		}
		got := lastActiveTime(app)
		if !got.Equal(stamp) {
			t.Fatalf("got %v, want %v", got, stamp)
		}
	})
}

func TestShouldAutoHibernate(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		app       *appv1alpha1.App
		wantSleep bool
	}{
		{
			// idleTTLSeconds:0 is the shipped create default. It now means the
			// platform default window (defaultIdleTTL = 15 min), not "never":
			// 10 min idle is within that window, so not yet — but it IS now
			// eligible, where before w6/m116 a 0 App never slept at all.
			"free, idleTTLSeconds:0 (default), within default window: not yet",
			mkIdleApp("free", 0, now.Add(-10*time.Minute), false),
			false,
		},
		{
			// The exact bug: a created-default free App idle past the default
			// window must hibernate. Before w6/m116 this returned false forever.
			"free, idleTTLSeconds:0 (default), past default window: hibernates",
			mkIdleApp("free", 0, now.Add(-20*time.Minute), false),
			true,
		},
		{
			"paid tier: always-on",
			mkIdleApp("starter", 300, now.Add(-10*time.Minute), false),
			false,
		},
		{
			"pro tier: always-on",
			mkIdleApp("pro", 300, now.Add(-24*time.Hour), false),
			false,
		},
		{
			"manually suspended: auto-hibernate defers to spec.suspended",
			mkIdleApp("free", 300, now.Add(-10*time.Minute), true),
			false,
		},
		{
			"free, TTL not yet elapsed: stays running",
			mkIdleApp("free", 300, now.Add(-4*time.Minute), false),
			false,
		},
		{
			"free, TTL elapsed: should hibernate",
			mkIdleApp("free", 300, now.Add(-6*time.Minute), false),
			true,
		},
		{
			"empty tier (free default), TTL elapsed: should hibernate",
			mkIdleApp("", 300, now.Add(-10*time.Minute), false),
			true,
		},
		{
			"private service never auto-hibernates without a public wake path",
			func() *appv1alpha1.App {
				app := mkIdleApp("free", 300, now.Add(-10*time.Minute), false)
				app.Spec.Type = appv1alpha1.TypePrivateService
				return app
			}(),
			false,
		},
		{
			"no last-active annotation: never auto-hibernates (operator stamps it)",
			mkIdleApp("free", 300, time.Time{}, false),
			false,
		},
		{
			"legacy free maintenance still auto-hibernates its workload",
			func() *appv1alpha1.App {
				app := mkIdleApp("free", 300, now.Add(-10*time.Minute), false)
				app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
				return app
			}(),
			true,
		},
		{
			"maintenance mode struct present but disabled: auto-hibernate unaffected",
			func() *appv1alpha1.App {
				app := mkIdleApp("free", 300, now.Add(-6*time.Minute), false)
				app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: false}
				return app
			}(),
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldAutoHibernate(tc.app); got != tc.wantSleep {
				t.Fatalf("shouldAutoHibernate = %v, want %v", got, tc.wantSleep)
			}
		})
	}
}

func TestIdleRequeueAfter(t *testing.T) {
	now := time.Now()
	ttl := 300 * time.Second

	t.Run("no annotation: full TTL returned", func(t *testing.T) {
		app := mkIdleApp("free", 300, time.Time{}, false)
		got := idleRequeueAfter(app, now)
		if got != ttl {
			t.Fatalf("got %v, want %v", got, ttl)
		}
	})

	t.Run("idleTTLSeconds:0 (default), no annotation: full default window", func(t *testing.T) {
		app := mkIdleApp("free", 0, time.Time{}, false)
		got := idleRequeueAfter(app, now)
		if got != defaultIdleTTL {
			t.Fatalf("got %v, want the platform default %v (0 must not compute a zero-length window)", got, defaultIdleTTL)
		}
	})

	t.Run("midway through TTL: remaining time returned", func(t *testing.T) {
		last := now.Add(-100 * time.Second)
		app := mkIdleApp("free", 300, last, false)
		got := idleRequeueAfter(app, now)
		want := 200 * time.Second
		// Allow 2s tolerance for test timing.
		if got < want-2*time.Second || got > want+2*time.Second {
			t.Fatalf("got %v, want ~%v", got, want)
		}
	})

	t.Run("past TTL: minimum 5s floor", func(t *testing.T) {
		last := now.Add(-10 * time.Minute)
		app := mkIdleApp("free", 300, last, false)
		got := idleRequeueAfter(app, now)
		if got != 5*time.Second {
			t.Fatalf("got %v, want 5s floor", got)
		}
	})
}
