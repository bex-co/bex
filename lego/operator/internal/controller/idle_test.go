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
			"no TTL configured: never sleeps",
			mkIdleApp("free", 0, now.Add(-10*time.Minute), false),
			false,
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
			"no last-active annotation: never auto-hibernates (operator stamps it)",
			mkIdleApp("free", 300, time.Time{}, false),
			false,
		},
		{
			"maintenance mode enabled: auto-hibernate defers to it (pods must stay up)",
			func() *appv1alpha1.App {
				app := mkIdleApp("free", 300, now.Add(-10*time.Minute), false)
				app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
				return app
			}(),
			false,
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
