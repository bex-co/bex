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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestAutoDeployTriggerMapping verifies bex's boolean spec.autoDeploy maps
// consistently onto Render's two wire representations: the legacy autoDeploy
// enum ("yes"/"no") and autoDeployTrigger ("commit"/"off") — never "checksPass"
// (a documented divergence, w5/m53).
func TestAutoDeployTriggerMapping(t *testing.T) {
	for _, c := range []struct {
		autoDeploy     bool
		yesNo, trigger string
	}{
		{true, "yes", "commit"},
		{false, "no", "off"},
	} {
		if got := yesNoEnum(c.autoDeploy); got != c.yesNo {
			t.Errorf("yesNoEnum(%v) = %q, want %q", c.autoDeploy, got, c.yesNo)
		}
		if got := triggerEnum(c.autoDeploy); got != c.trigger {
			t.Errorf("triggerEnum(%v) = %q, want %q", c.autoDeploy, got, c.trigger)
		}
	}
}

// TestParseTriggerRejectsChecksPass covers the accept side: the trigger enum
// maps to a tri-state *bool, and "checksPass" is rejected with a Render-shaped
// ErrBadRequest whose message names the divergence (w5/m53).
func TestParseTriggerRejectsChecksPass(t *testing.T) {
	ptr := func(b bool) *bool { return &b }
	for _, c := range []struct {
		in   string
		want *bool
	}{
		{"", nil},
		{"commit", ptr(true)},
		{"off", ptr(false)},
	} {
		got, err := parseTrigger(c.in)
		if err != nil {
			t.Fatalf("parseTrigger(%q) unexpected error: %v", c.in, err)
		}
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("parseTrigger(%q) = %v, want %v", c.in, deref(got), deref(c.want))
		}
	}

	_, err := parseTrigger("checksPass")
	if err == nil || !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("parseTrigger(checksPass) = %v, want ErrBadRequest", err)
	}
	if !strings.Contains(err.Error(), "checksPass") || !strings.Contains(err.Error(), "commit") {
		t.Errorf("checksPass error should name the divergence, got %q", err.Error())
	}
	if _, err := parseTrigger("garbage"); err == nil || !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("parseTrigger(garbage) = %v, want ErrBadRequest", err)
	}
}

// TestParseAutoDeployPrecedence: autoDeployTrigger wins over the legacy
// autoDeploy enum when both are sent (Render's precedence), and the checks-pass
// rejection propagates through even alongside a valid autoDeploy (w5/m53).
func TestParseAutoDeployPrecedence(t *testing.T) {
	// Trigger wins over a conflicting legacy value.
	if got, err := parseAutoDeploy("no", "commit"); err != nil || got == nil || !*got {
		t.Errorf("parseAutoDeploy(no, commit) = %v, %v; want true", deref(got), err)
	}
	if got, err := parseAutoDeploy("yes", "off"); err != nil || got == nil || *got {
		t.Errorf("parseAutoDeploy(yes, off) = %v, %v; want false", deref(got), err)
	}
	// Falls back to the legacy enum when no trigger is sent.
	if got, err := parseAutoDeploy("yes", ""); err != nil || got == nil || !*got {
		t.Errorf("parseAutoDeploy(yes, \"\") = %v, %v; want true", deref(got), err)
	}
	// Neither field present => nil (leave unchanged / platform default).
	if got, err := parseAutoDeploy("", ""); err != nil || got != nil {
		t.Errorf("parseAutoDeploy(\"\", \"\") = %v, %v; want nil", deref(got), err)
	}
	// checksPass is rejected even alongside a valid legacy value.
	if _, err := parseAutoDeploy("yes", "checksPass"); err == nil {
		t.Error("parseAutoDeploy(yes, checksPass) should reject checksPass")
	}
}

func deref(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}

func TestSetAutoDeployFlipsFlagWithoutRebuild(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/x/app", AutoDeploy: true},
	}
	svc, cl := newService(nil, app)

	// Turn it off — the flag flips, and no restartedAt bump (a toggle isn't a deploy).
	if _, err := svc.SetAutoDeploy(context.Background(), "web", false); err != nil {
		t.Fatal(err)
	}
	got := getApp(t, cl, "web")
	if got.Spec.AutoDeploy {
		t.Error("autoDeploy should be false after SetAutoDeploy(false)")
	}
	if got.Spec.RestartedAt != "" {
		t.Error("flipping autoDeploy must not trigger a redeploy (no restartedAt)")
	}

	// Turn it back on — survives the round-trip.
	if _, err := svc.SetAutoDeploy(context.Background(), "web", true); err != nil {
		t.Fatal(err)
	}
	if !getApp(t, cl, "web").Spec.AutoDeploy {
		t.Error("autoDeploy should be true after SetAutoDeploy(true)")
	}
}
