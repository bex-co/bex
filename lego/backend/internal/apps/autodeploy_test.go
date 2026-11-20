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
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
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
		if got := core.RenderAutoDeployTrigger(c.autoDeploy, c.specTrigger); got != c.trigger {
			t.Errorf("RenderAutoDeployTrigger(%v, %q) = %q, want %q", c.autoDeploy, c.specTrigger, got, c.trigger)
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

// TestParseAutoDeployRefusesBoth (w8/347): Render's API refuses a request
// carrying both Auto-Deploy fields ("You can't set both `autoDeploy` and
// `autoDeployTrigger` in the same request.", pinned OpenAPI), whatever the
// values; either one alone maps as before, and the legacy enum alone leaves
// the checks gate as it is, on every surface.
func TestParseAutoDeployRefusesBoth(t *testing.T) {
	for _, c := range []struct {
		legacy, trigger     string
		enabled, checksPass *bool
	}{
		{"", "commit", boolPtr(true), boolPtr(false)},
		{"", "off", boolPtr(false), boolPtr(false)},
		{"", "checksPass", boolPtr(true), boolPtr(true)},
		{"yes", "", boolPtr(true), nil},
		{"no", "", boolPtr(false), nil},
		{"", "", nil, nil},
	} {
		enabled, checksPass, err := parseAutoDeploy(c.legacy, c.trigger)
		if err != nil || deref(enabled) != deref(c.enabled) || deref(checksPass) != deref(c.checksPass) {
			t.Errorf("parseAutoDeploy(%q, %q) = %v/%v, %v; want %v/%v", c.legacy, c.trigger, deref(enabled), deref(checksPass), err, deref(c.enabled), deref(c.checksPass))
		}
	}
	for _, both := range [][2]string{{"no", "commit"}, {"yes", "off"}, {"no", "checksPass"}, {"yes", "commit"}, {"maybe", "sometimes"}} {
		enabled, checksPass, err := parseAutoDeploy(both[0], both[1])
		if !errors.Is(err, errAutoDeployBoth) || enabled != nil || checksPass != nil {
			t.Errorf("parseAutoDeploy(%q, %q) = %v/%v, %v; want errAutoDeployBoth", both[0], both[1], deref(enabled), deref(checksPass), err)
		}
	}
	if !errors.Is(errAutoDeployBoth, core.ErrBadRequest) {
		t.Error("errAutoDeployBoth must be a 400 (core.ErrBadRequest)")
	}
	if _, _, err := parseAutoDeploy("maybe", ""); err == nil {
		t.Error("parseAutoDeploy(maybe) should reject an invalid legacy value")
	}
}

// TestAutoDeployBothFieldsRefusedOnEverySurface (w8/347): REST create (with
// and without dryRun) and PATCH, GraphQL createService and MCP update_service
// answer a request carrying both autoDeploy and autoDeployTrigger with a 400
// naming both, and write nothing; either field alone still lands.
func TestAutoDeployBothFieldsRefusedOnEverySurface(t *testing.T) {
	ctx := context.Background()
	gitApp := func() *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/x/app", AutoDeploy: true},
		}
	}
	refusedBoth := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"id":"bad_request"`) ||
			!strings.Contains(rec.Body.String(), "autoDeploy and autoDeployTrigger") {
			t.Errorf("both fields => 400 bad_request naming both, got %d: %s", rec.Code, rec.Body)
		}
	}

	t.Run("REST PATCH", func(t *testing.T) {
		svc, cl := newService(nil, gitApp())
		before := getApp(t, cl, "web").ResourceVersion
		for _, body := range []string{`{"autoDeploy":"no","autoDeployTrigger":"commit"}`, `{"autoDeploy":"no","autoDeployTrigger":"commit","dryRun":true}`} {
			refusedBoth(t, serveRESTPatch(t, svc, "web", body))
		}
		if a := getApp(t, cl, "web"); a.ResourceVersion != before || !a.Spec.AutoDeploy {
			t.Errorf("a refused PATCH wrote: resourceVersion %s -> %s, autoDeploy %v", before, a.ResourceVersion, a.Spec.AutoDeploy)
		}
		for _, c := range []struct {
			body string
			want bool
		}{{`{"autoDeploy":"no"}`, false}, {`{"autoDeployTrigger":"commit"}`, true}, {`{"autoDeployTrigger":"off"}`, false}, {`{"autoDeploy":"yes"}`, true}} {
			if rec := serveRESTPatch(t, svc, "web", c.body); rec.Code != http.StatusOK {
				t.Fatalf("PATCH %s => 200, got %d: %s", c.body, rec.Code, rec.Body)
			}
			if got := getApp(t, cl, "web").Spec.AutoDeploy; got != c.want {
				t.Errorf("after PATCH %s autoDeploy = %v, want %v", c.body, got, c.want)
			}
		}
	})

	t.Run("REST create", func(t *testing.T) {
		svc, cl := newService(nil)
		const both = `"autoDeploy":"no","autoDeployTrigger":"commit"`
		for _, body := range []string{
			`{"name":"web","repo":"https://github.com/x/app",` + both + `}`,
			`{"name":"web","repo":"https://github.com/x/app",` + both + `,"dryRun":true}`,
		} {
			refusedBoth(t, serveRESTStrict(t, svc, http.MethodPost, "/v1/services", body))
		}
		var list appv1alpha1.AppList
		if err := cl.List(ctx, &list); err != nil || len(list.Items) != 0 {
			t.Fatalf("a refused create left %d services (%v)", len(list.Items), err)
		}
		for _, c := range []struct {
			name, field string
			want        bool
		}{{"legacy", `"autoDeploy":"no"`, false}, {"trigger", `"autoDeployTrigger":"off"`, false}, {"commit", `"autoDeployTrigger":"commit"`, true}} {
			body := `{"name":"` + c.name + `","repo":"https://github.com/x/app",` + c.field + `}`
			if rec := serveRESTStrict(t, svc, http.MethodPost, "/v1/services", body); rec.Code != http.StatusCreated {
				t.Fatalf("create %s => 201, got %d: %s", c.field, rec.Code, rec.Body)
			}
			if got := getApp(t, cl, c.name).Spec.AutoDeploy; got != c.want {
				t.Errorf("create %s: autoDeploy = %v, want %v", c.field, got, c.want)
			}
		}
	})

	t.Run("GraphQL createService", func(t *testing.T) {
		base := func() map[string]any { return map[string]any{"name": "web", "repo": "https://github.com/x/app"} }
		in := base()
		in["autoDeploy"], in["autoDeployTrigger"] = false, "commit"
		if _, err := gqlCreateRequest(in); !errors.Is(err, errAutoDeployBoth) {
			t.Errorf("createService(autoDeploy, autoDeployTrigger) = %v, want errAutoDeployBoth", err)
		}
		in = base()
		in["autoDeploy"] = false
		if req, err := gqlCreateRequest(in); err != nil || deref(req.AutoDeploy) != false {
			t.Errorf("createService(autoDeploy: false) = %v, %v; want false", deref(req.AutoDeploy), err)
		}
		in = base()
		in["autoDeployTrigger"] = "off"
		if req, err := gqlCreateRequest(in); err != nil || deref(req.AutoDeploy) != false {
			t.Errorf("createService(autoDeployTrigger: off) = %v, %v; want false", deref(req.AutoDeploy), err)
		}
	})

	t.Run("MCP update_service", func(t *testing.T) {
		svc, cl := newService(nil, gitApp())
		before := getApp(t, cl, "web").ResourceVersion
		_, err := svc.applyServicePatch(ctx, updateServiceArgs{ServiceID: "web", AutoDeploy: boolPtr(false), AutoDeployTrigger: sp("commit")})
		if !errors.Is(err, errAutoDeployBoth) {
			t.Errorf("update_service(autoDeploy, autoDeployTrigger) = %v, want errAutoDeployBoth", err)
		}
		if a := getApp(t, cl, "web"); a.ResourceVersion != before || !a.Spec.AutoDeploy {
			t.Errorf("a refused update_service wrote: resourceVersion %s -> %s, autoDeploy %v", before, a.ResourceVersion, a.Spec.AutoDeploy)
		}
		if _, err := svc.applyServicePatch(ctx, updateServiceArgs{ServiceID: "web", AutoDeploy: boolPtr(false)}); err != nil || getApp(t, cl, "web").Spec.AutoDeploy {
			t.Errorf("update_service(autoDeploy: false) = %v; autoDeploy %v, want false", err, getApp(t, cl, "web").Spec.AutoDeploy)
		}
		if _, err := svc.applyServicePatch(ctx, updateServiceArgs{ServiceID: "web", AutoDeployTrigger: sp("commit")}); err != nil || !getApp(t, cl, "web").Spec.AutoDeploy {
			t.Errorf("update_service(autoDeployTrigger: commit) = %v; autoDeploy %v, want true", err, getApp(t, cl, "web").Spec.AutoDeploy)
		}
	})
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

// TestAutoDeployAuditCarriesRendersTriggerAndReason pins w3/m317: turning
// auto-deploy off in settings records Render's reason setting_change and the
// trigger it turned off (fromTrigger); turning it on records the trigger now
// in force (newTrigger) and no reason; an unchanged re-save records nothing.
func TestAutoDeployAuditCarriesRendersTriggerAndReason(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/x/app", AutoDeploy: true, AutoDeployTrigger: checksPassTrigger},
	}
	svc, _ := newService(nil, app)
	sink := &captureAuditSink{}
	svc.Audit = sink
	ctx := context.Background()
	if _, err := svc.SetAutoDeploy(ctx, "web", false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetAutoDeployTrigger(ctx, "web", "commit"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetAutoDeployTrigger(ctx, "web", "commit"); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbSetAutoDeploy && ev.Outcome == core.AuditAllowed {
			got = append(got, fmt.Sprintf("%v %v %v", ptrValue(ev.AutoDeployEnabled), ptrValue(ev.AutoDeployTrigger), ptrValue(ev.AutoDeployReason)))
		}
	}
	// The repeated "commit" changed nothing, so it announces nothing.
	want := []string{"false checksPass setting_change", "true commit <nil>"}
	if !slices.Equal(got, want) {
		t.Fatalf("auto-deploy audit rows = %q, want %q", got, want)
	}
}

// w9/m449: a change between two triggers that both deploy is Render's
// UpdateAutoDeployTriggerEvent in the workspace audit log, while the service
// feed and webhooks keep reading it as auto-deploy enabled with the new
// trigger from a feed_only apps.SetAutoDeploy row. Turning it off or on stays
// one visible apps.SetAutoDeploy row (UpdateAutoDeployEvent).
func TestAutoDeployTriggerChangeIsRendersTriggerEvent(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/x/app", AutoDeploy: true, AutoDeployTrigger: checksPassTrigger},
	}
	svc, _ := newService(nil, app)
	sink := &captureAuditSink{}
	svc.Audit = sink
	ctx := context.Background()
	for _, trigger := range []string{"commit", "off", "commit"} {
		if _, err := svc.SetAutoDeployTrigger(ctx, "web", trigger); err != nil {
			t.Fatalf("set %s: %v", trigger, err)
		}
	}
	var got []string
	for _, ev := range sink.events {
		if ev.Outcome == core.AuditAllowed {
			got = append(got, fmt.Sprintf("%s %v %v feedOnly=%v", ev.Verb, ptrValue(ev.AutoDeployEnabled), ptrValue(ev.AutoDeployTrigger), ev.FeedOnly))
		}
	}
	want := []string{
		"apps.SetAutoDeploy true commit feedOnly=true",
		"apps.SetAutoDeployTrigger <nil> commit feedOnly=false",
		"apps.SetAutoDeploy false commit feedOnly=false",
		"apps.SetAutoDeploy true commit feedOnly=false",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("auto-deploy audit rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
