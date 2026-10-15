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
	"fmt"
	"net/http"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// bashCronCommand is sweep285's native cron command: Bash syntax that fails
// under the /bin/sh a command override runs in (w8/m69).
const bashCronCommand = "[[ 1 = 1 ]] && printf '%s' QA285_BASH_OK"

// assertNativeCronStart checks a native cron runs want as its baked start
// command, with no /bin/sh override in front of it.
func assertNativeCronStart(t *testing.T, spec appv1alpha1.AppSpec, want string) {
	t.Helper()
	if spec.StartCommand != want || spec.Command != "" {
		t.Errorf("start command %q / override %q, want %q / no override", spec.StartCommand, spec.Command, want)
	}
}

// The pinned CLI's native cron create keeps its command in the native launch
// role for every native runtime, and reads it back as startCommand, which a
// clone re-sends.
func TestNativeCronCreateKeepsTheStartCommand(t *testing.T) {
	for _, runtime := range []string{"node", "python", "go", "ruby", "rust", "elixir"} {
		t.Run(runtime, func(t *testing.T) {
			svc, cl := newService(nil)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := serveJSON(t, mux, http.MethodPost, "/v1/services", map[string]any{
				"name": "nightly", "type": "cron_job", "repo": "https://github.com/acme/jobs",
				"serviceDetails": map[string]any{"runtime": runtime, "plan": "free", "schedule": "0 0 1 1 *",
					"envSpecificDetails": map[string]any{"buildCommand": "printf control", "startCommand": bashCronCommand}},
			})
			if rec.Code != http.StatusCreated {
				t.Fatalf("create: %d %s", rec.Code, rec.Body)
			}
			assertNativeCronStart(t, getApp(t, cl, "nightly").Spec, bashCronCommand)
			if got := nativeStartCommandFromRead(t, rec.Body.Bytes()); got != bashCronCommand {
				t.Errorf("read startCommand = %q, want %q", got, bashCronCommand)
			}
		})
	}
}

// A native cron's start command update replaces what runs, whether the cron
// holds the native role or a legacy duplicate override; a blank one drops only
// the override. REST, GraphQL and MCP share the setter.
func TestNativeCronStartCommandUpdateKeepsTheNativeRole(t *testing.T) {
	fixtures := map[string]func() *appv1alpha1.App{
		"native role": func() *appv1alpha1.App { return nativeCron("nightly") },
		"legacy duplicate": func() *appv1alpha1.App {
			app := nativeCron("nightly")
			app.Spec.Command = app.Spec.StartCommand
			return app
		},
	}
	writes := map[string]func(t *testing.T, svc *Service, command string){
		"REST PATCH": func(t *testing.T, svc *Service, command string) {
			body := fmt.Sprintf(`{"serviceDetails":{"envSpecificDetails":{"startCommand":%q}}}`, command)
			if rec := serveRESTPatch(t, svc, "nightly", body); rec.Code != http.StatusOK {
				t.Fatalf("PATCH: %d %s", rec.Code, rec.Body)
			}
		},
		"GraphQL setStartCommand": func(t *testing.T, svc *Service, command string) {
			cronGraphQL(t, svc, fmt.Sprintf(`mutation { setStartCommand(id: "nightly", command: %q) { id } }`, command))
		},
		"MCP update_service": func(t *testing.T, svc *Service, command string) {
			call, cleanup := appsMCPClient(t, svc)
			defer cleanup()
			call("update_service", map[string]any{"serviceId": "nightly", "startCommand": command})
		},
	}
	for fixture, app := range fixtures {
		for write, apply := range writes {
			t.Run(fixture+"/"+write, func(t *testing.T) {
				svc, cl := newService(nil, app())
				apply(t, svc, bashCronCommand)
				assertNativeCronStart(t, getApp(t, cl, "nightly").Spec, bashCronCommand)

				svc, cl = newService(nil, app())
				apply(t, svc, "")
				assertNativeCronStart(t, getApp(t, cl, "nightly").Spec, "node job.js")
			})
		}
	}
}

// Bex's own command field stays the explicit override it documents, and is
// what a read then reports the cron runs.
func TestNativeCronExplicitOverrideStaysAnOverride(t *testing.T) {
	svc, cl := newService(nil, nativeCron("nightly"))
	if _, err := svc.SetCronJob(context.Background(), "nightly", nil, sp("node override.js")); err != nil {
		t.Fatal(err)
	}
	app := getApp(t, cl, "nightly")
	if app.Spec.Command != "node override.js" || app.Spec.StartCommand != "node job.js" {
		t.Errorf("explicit override = %q beside start %q", app.Spec.Command, app.Spec.StartCommand)
	}
	if got := nativeStartCommandFromRead(t, getServiceBody(t, app)); got != "node override.js" {
		t.Errorf("read startCommand with an override = %q, want the override", got)
	}
}

// A native cron created in the native role reads back its start command, so
// the CLI's clone (which re-sends startCommand) keeps it.
func TestNativeCronReadsBackItsStartCommand(t *testing.T) {
	if got := nativeStartCommandFromRead(t, getServiceBody(t, nativeCron("nightly"))); got != "node job.js" {
		t.Errorf("read startCommand = %q, want the baked start command", got)
	}
}

// A genuine Blueprint edit of a native cron's Bash command is its new start
// command, including the apply that switches a Dockerfile cron to a native
// runtime; an unchanged reapply stays a no-op.
func TestBlueprintNativeCronEditKeepsTheNativeRole(t *testing.T) {
	manifest := func(runtime, command string) string {
		build := ""
		if runtime != "docker" {
			build = "    buildCommand: npm ci\n"
		}
		return fmt.Sprintf("services:\n  - name: nightly\n    type: cron\n    runtime: %s\n    repo: https://github.com/acme/jobs\n    schedule: '0 0 1 1 *'\n%s    startCommand: %q\n", runtime, build, command)
	}
	for _, from := range []string{"node", "docker"} {
		t.Run("from "+from, func(t *testing.T) {
			svc, cl := newService(nil)
			ctx := context.Background()
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest(from, "echo old")}); err != nil {
				t.Fatal(err)
			}
			var edited string
			for attempt := range 2 {
				if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest("node", bashCronCommand)}); err != nil {
					t.Fatal(err)
				}
				app := getApp(t, cl, "nightly")
				assertNativeCronStart(t, app.Spec, bashCronCommand)
				if attempt == 1 && app.ResourceVersion != edited {
					t.Errorf("unchanged reapply wrote the cron: %s -> %s", edited, app.ResourceVersion)
				}
				edited = app.ResourceVersion
			}
		})
	}
}

// A legacy duplicate override already running the declared command is left
// in place by an unchanged Blueprint apply, so the fix opens no deploy on its
// own (w8/m69).
func TestBlueprintLeavesAMatchingNativeCronOverride(t *testing.T) {
	spec := nativeCron("nightly").Spec
	spec.Command = spec.StartCommand
	want := spec
	want.Command = ""
	if !nativeBuild(buildStrategy(spec)) {
		t.Fatalf("fixture is not native: %q", buildStrategy(spec))
	}
	applyBlueprintCommand(&spec, want)
	if spec.Command != "node job.js" || spec.StartCommand != "node job.js" {
		t.Errorf("matching override changed: command %q start %q", spec.Command, spec.StartCommand)
	}
}
