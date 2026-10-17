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

// TestAutoDeployTriggerMapping verifies spec.autoDeploy and its checks gate map
// onto Render's two wire representations: the legacy autoDeploy enum
// ("yes"/"no", which knows no gate) and autoDeployTrigger (off, commit,
// checksPass — w5/m53, w1/m181).
func TestAutoDeployTriggerMapping(t *testing.T) {
	for _, c := range []struct {
		autoDeploy     bool
		specTrigger    string
		yesNo, trigger string
	}{
		{true, "", "yes", "commit"},
		{true, "commit", "yes", "commit"},
		{true, checksPassTrigger, "yes", "checksPass"},
		{false, "", "no", "off"},
		{false, checksPassTrigger, "no", "off"},
	} {
		if got := yesNoEnum(c.autoDeploy); got != c.yesNo {
			t.Errorf("yesNoEnum(%v) = %q, want %q", c.autoDeploy, got, c.yesNo)
		}
		if got := triggerEnum(c.autoDeploy, c.specTrigger); got != c.trigger {
			t.Errorf("triggerEnum(%v, %q) = %q, want %q", c.autoDeploy, c.specTrigger, got, c.trigger)
		}
	}
}

// TestParseTrigger covers the accept side: each Render trigger maps to
// whether a push deploys and whether it waits for checks; anything else is a
// Render-shaped ErrBadRequest naming the three values.
func TestParseTrigger(t *testing.T) {
	for _, c := range []struct {
		in                  string
		enabled, checksPass *bool
	}{
		{"", nil, nil},
		{"commit", boolPtr(true), boolPtr(false)},
		{"off", boolPtr(false), boolPtr(false)},
		{"checksPass", boolPtr(true), boolPtr(true)},
	} {
		enabled, checksPass, err := parseTrigger(c.in)
		if err != nil {
			t.Fatalf("parseTrigger(%q) unexpected error: %v", c.in, err)
		}
		if deref(enabled) != deref(c.enabled) || deref(checksPass) != deref(c.checksPass) {
			t.Errorf("parseTrigger(%q) = %v/%v, want %v/%v", c.in, deref(enabled), deref(checksPass), deref(c.enabled), deref(c.checksPass))
		}
	}
	_, _, err := parseTrigger("garbage")
	if err == nil || !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "checksPass") {
		t.Errorf("parseTrigger(garbage) = %v, want ErrBadRequest naming the values", err)
	}
}

// TestParseAutoDeployPrecedence: autoDeployTrigger wins over the legacy
// autoDeploy enum when both are sent (Render's precedence); the legacy enum
// alone leaves the checks gate as it is, on every surface.
func TestParseAutoDeployPrecedence(t *testing.T) {
	for _, c := range []struct {
		legacy, trigger     string
		enabled, checksPass *bool
	}{
		{"no", "commit", boolPtr(true), boolPtr(false)},
		{"yes", "off", boolPtr(false), boolPtr(false)},
		{"no", "checksPass", boolPtr(true), boolPtr(true)},
		{"yes", "", boolPtr(true), nil},
		{"no", "", boolPtr(false), nil},
		{"", "", nil, nil},
	} {
		enabled, checksPass, err := parseAutoDeploy(c.legacy, c.trigger)
		if err != nil || deref(enabled) != deref(c.enabled) || deref(checksPass) != deref(c.checksPass) {
			t.Errorf("parseAutoDeploy(%q, %q) = %v/%v, %v; want %v/%v", c.legacy, c.trigger, deref(enabled), deref(checksPass), err, deref(c.enabled), deref(c.checksPass))
		}
	}
	if _, _, err := parseAutoDeploy("maybe", ""); err == nil {
		t.Error("parseAutoDeploy(maybe) should reject an invalid legacy value")
	}
}

func boolPtr(b bool) *bool { return &b }

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
