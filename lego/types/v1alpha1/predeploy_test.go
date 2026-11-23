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

package v1alpha1

import (
	"strings"
	"testing"
)

// w33/m435: the pre-deploy step and the one-time initialDeployHook are two
// selections. The step is the release's preDeployCommand on every release, the
// first included; the hook is owed until one run succeeds and is never the
// step.
func TestPreDeployStepAndInitialDeployHookAreSeparateSelections(t *testing.T) {
	service := func(kind string, mutate func(*App)) *App {
		app := &App{Spec: AppSpec{Type: kind, PreDeployCommand: "migrate", InitialDeployHook: "seed"}}
		if mutate != nil {
			mutate(app)
		}
		return app
	}
	for name, tc := range map[string]struct {
		app  *App
		step string
		hook string // the hook still owed, "" when none is
	}{
		"first release":       {service(TypeWebService, nil), "migrate", "seed"},
		"private service":     {service(TypePrivateService, nil), "migrate", "seed"},
		"background worker":   {service(TypeBackgroundWorker, nil), "migrate", "seed"},
		"hook only":           {service(TypeWebService, func(a *App) { a.Spec.PreDeployCommand = "" }), "", "seed"},
		"blank hook":          {service(TypeWebService, func(a *App) { a.Spec.InitialDeployHook = "  " }), "migrate", ""},
		"completed by status": {service(TypeWebService, func(a *App) { a.Status.InitialDeployHookCompleted = true }), "migrate", ""},
		"completed by the older annotation": {service(TypeWebService, func(a *App) {
			a.Annotations = map[string]string{AnnotationInitialDeployHookRan: "true"}
		}), "migrate", ""},
		"cron job":     {service(TypeCronJob, nil), "", ""},
		"static site":  {service(TypeStaticSite, nil), "", ""},
		"build source": {service(TypeBuildSource, nil), "", ""},
	} {
		if got := tc.app.PreDeployStepCommand(); got != tc.step {
			t.Errorf("%s: pre-deploy step = %q, want %q", name, got, tc.step)
		}
		if got := tc.app.OwedInitialDeployHook(); got != tc.hook {
			t.Errorf("%s: owed hook = %q, want %q", name, got, tc.hook)
		}
	}
}

// The hook's Job keeps the pre-deploy plane's prefix (the admission policy
// names it) and never shares a name with a pre-deploy Job, whose name ends in
// its generation.
func TestInitialDeployHookJobNameNeverCollidesWithAPreDeployJob(t *testing.T) {
	long := strings.Repeat("a", 70)
	for _, name := range []string{"web", "web-hook", "web-gen-1", long} {
		for _, rev := range []string{"gen-1", "gen-12", ""} {
			hook, step := InitialDeployHookJobName(name, rev), PreDeployJobName(name, rev)
			if hook == step || !strings.HasPrefix(hook, "predeploy-") || len(hook) > 63 {
				t.Errorf("hook Job %q for %s/%q: want a distinct predeploy- name of at most 63 characters (step %q)", hook, name, rev, step)
			}
		}
	}
	if got := InitialDeployHookJobName("web", "gen-3"); got != "predeploy-web-gen-3-hook" {
		t.Errorf("InitialDeployHookJobName = %q, want predeploy-web-gen-3-hook", got)
	}
	if got := InitialDeployHookJobName("web", ""); got != "predeploy-web-latest-hook" {
		t.Errorf("InitialDeployHookJobName with no revision = %q, want predeploy-web-latest-hook", got)
	}
	// Another service whose name ends like the suffix cannot produce it: a
	// pre-deploy Job name always ends in a generation.
	if PreDeployJobName("web-gen-3-hook", "gen-1") == InitialDeployHookJobName("web", "gen-3") {
		t.Error("a pre-deploy Job name equals another service's hook Job name")
	}
}

// w32/091: bex-api recognizes the operator's 30-minute deadline and StartError
// pre-deploy verdicts so the deploy log closes them as Render's does, while
// leaving the control-plane backstop and every other failure on bex's line.
func TestPreDeployFailureReasonClassifiers(t *testing.T) {
	window := FormatDeployReason(DeployReasonPreDeployWindowExceeded, DeployReasonParam{Name: "minutes", Value: "30"})
	if !IsPreDeployWindowExceeded(window) || IsPreDeployCouldNotLaunch(window) {
		t.Errorf("window reason %q: window=%v launch=%v, want true/false", window, IsPreDeployWindowExceeded(window), IsPreDeployCouldNotLaunch(window))
	}
	for _, msg := range []string{
		FormatDeployReason(DeployReasonPreDeployCouldNotLaunch),
		PreDeployCouldNotStartPrefix + `the program "release" does not exist in this image` + PreDeployNeverRanSuffix,
		PreDeployCouldNotStartPrefix + `the program "/app/migrate" is not executable in this image` + PreDeployNeverRanSuffix,
		PreDeployCouldNotStartPrefix + `the container runtime could not execute "release"` + PreDeployNeverRanSuffix,
	} {
		if !IsPreDeployCouldNotLaunch(msg) || IsPreDeployWindowExceeded(msg) {
			t.Errorf("launch reason %q: launch=%v window=%v, want true/false", msg, IsPreDeployCouldNotLaunch(msg), IsPreDeployWindowExceeded(msg))
		}
	}
	for _, other := range []string{
		"",
		FormatDeployReason(DeployReasonPreDeployTimedOut),
		FormatDeployReason(DeployReasonPreDeployOutOfMemory, DeployReasonParam{Name: "code", Value: "137"}),
		FormatDeployReason(DeployReasonPreDeployExited, DeployReasonParam{Name: "code", Value: "1"}),
		FormatDeployReason(DeployReasonPreDeployFailed),
		"the pre-deploy command could not be started: the platform failed",
		"image pull is failing: not found; the pre-deploy command never ran",
	} {
		if IsPreDeployWindowExceeded(other) || IsPreDeployCouldNotLaunch(other) {
			t.Errorf("other reason %q: window=%v launch=%v, want false/false", other, IsPreDeployWindowExceeded(other), IsPreDeployCouldNotLaunch(other))
		}
	}
}
