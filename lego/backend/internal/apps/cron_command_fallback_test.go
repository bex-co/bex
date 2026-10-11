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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m194: an image/Dockerfile cron's startCommand fallback is its Command on
// read, and an explicit clear removes it so the image default runs.

const legacyCronScript = "sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'"

// legacyImageCron is the observed fixture: runtime image, command stored only
// as the startCommand fallback.
func legacyImageCron(name string) *appv1alpha1.App {
	app := imageCron(name, "")
	app.Spec.Runtime = "image"
	app.Spec.Schedule = "0 0 1 1 *"
	app.Spec.StartCommand = legacyCronScript
	return app
}

// legacyDockerCron is the Dockerfile sibling: the same fallback branch.
func legacyDockerCron(name string) *appv1alpha1.App {
	app := legacyImageCron(name)
	app.Spec.Image = ""
	app.Spec.Repo = "https://github.com/acme/jobs"
	app.Spec.Runtime = "docker"
	return app
}

// nativeCron bakes its start command into the built image; it is not a
// runtime override and must survive a Command clear.
func nativeCron(name string) *appv1alpha1.App {
	app := legacyDockerCron(name)
	app.Spec.Runtime = "node"
	app.Spec.StartCommand = "node job.js"
	return app
}

func cronGraphQL(t *testing.T, svc *Service, query string) map[string]any {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
	if len(res.Errors) > 0 {
		t.Fatalf("%s: %v", query, res.Errors)
	}
	return res.Data.(map[string]any)
}

func assertNoRuntimeOverride(t *testing.T, spec appv1alpha1.AppSpec) {
	t.Helper()
	if spec.Command != "" || spec.StartCommand != "" {
		t.Errorf("command=%q startCommand=%q, want both empty so the image default runs", spec.Command, spec.StartCommand)
	}
}

// The configured fallback is the cron's Command on every read surface.
func TestLegacyCronFallbackReadsAsCommand(t *testing.T) {
	for name, app := range map[string]*appv1alpha1.App{
		"image":      legacyImageCron("qa-cron"),
		"dockerfile": legacyDockerCron("qa-cron"),
	} {
		t.Run(name, func(t *testing.T) {
			svc, _ := newService(nil, app.DeepCopy())
			got := cronGraphQL(t, svc, `{ server(id: "qa-cron") { command } }`)["server"].(map[string]any)
			if got["command"] != legacyCronScript {
				t.Errorf("GraphQL command = %v, want the stored fallback", got["command"])
			}

			var read struct {
				ServiceDetails struct {
					Command            string         `json:"command"`
					EnvSpecificDetails map[string]any `json:"envSpecificDetails"`
				} `json:"serviceDetails"`
			}
			if err := json.Unmarshal(getServiceBody(t, app.DeepCopy()), &read); err != nil {
				t.Fatal(err)
			}
			if read.ServiceDetails.Command != legacyCronScript {
				t.Errorf("REST serviceDetails.command = %q", read.ServiceDetails.Command)
			}
			if read.ServiceDetails.EnvSpecificDetails["startCommand"] != legacyCronScript {
				t.Errorf("REST envSpecificDetails = %v, want startCommand = the fallback", read.ServiceDetails.EnvSpecificDetails)
			}
		})
	}
}

// Native crons keep their baked start command out of the runtime Command.
func TestNativeCronStartCommandIsNotACommandOverride(t *testing.T) {
	svc, _ := newService(nil, nativeCron("qa-cron"))
	got := cronGraphQL(t, svc, `{ server(id: "qa-cron") { command startCommand } }`)["server"].(map[string]any)
	if got["command"] != "" || got["startCommand"] != "node job.js" {
		t.Errorf("native cron read = %v, want empty command beside its start command", got)
	}
}

// Every explicit clear entrypoint removes both runtime slots.
func TestCronCommandClearRemovesTheFallback(t *testing.T) {
	ctx := context.Background()
	clears := map[string]func(t *testing.T, svc *Service){
		"SetCronJob": func(t *testing.T, svc *Service) {
			if _, err := svc.SetCronJob(ctx, "qa-cron", sp("0 0 1 1 *"), sp("")); err != nil {
				t.Fatal(err)
			}
		},
		"SetCommands whitespace": func(t *testing.T, svc *Service) {
			if _, err := svc.SetCommands(ctx, "qa-cron", nil, sp("   ")); err != nil {
				t.Fatal(err)
			}
		},
		"GraphQL updateCronJob": func(t *testing.T, svc *Service) {
			got := cronGraphQL(t, svc, `mutation { updateCronJob(id: "qa-cron", schedule: "0 0 1 1 *", command: "") { command } }`)
			if c := got["updateCronJob"].(map[string]any)["command"]; c != "" {
				t.Errorf("updateCronJob returned command %v, want empty", c)
			}
		},
		"GraphQL setStartCommand": func(t *testing.T, svc *Service) {
			cronGraphQL(t, svc, `mutation { setStartCommand(id: "qa-cron", command: "") { command } }`)
		},
	}
	for _, body := range []string{
		`{"command":""}`,
		`{"serviceDetails":{"envSpecificDetails":{"startCommand":""}}}`,
		`{"serviceDetails":{"envSpecificDetails":{"dockerCommand":" "}}}`,
	} {
		clears["REST PATCH "+body] = func(t *testing.T, svc *Service) {
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/qa-cron", strings.NewReader(body)))
			if rec.Code != http.StatusOK {
				t.Fatalf("PATCH %s = %d: %s", body, rec.Code, rec.Body)
			}
		}
	}
	for name, clear := range clears {
		for kind, app := range map[string]*appv1alpha1.App{
			"image":      legacyImageCron("qa-cron"),
			"dockerfile": legacyDockerCron("qa-cron"),
		} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				svc, cl := newService(nil, app)
				clear(t, svc)
				a := getApp(t, cl, "qa-cron")
				assertNoRuntimeOverride(t, a.Spec)
				if a.Spec.Schedule != "0 0 1 1 *" {
					t.Errorf("schedule = %q, want preserved", a.Spec.Schedule)
				}
			})
		}
	}
}

// A nonempty replacement wins over the fallback; omitting the command keeps
// both slots; a native cron's clear keeps its baked start command.
func TestCronCommandFallbackPreservedWhereItMust(t *testing.T) {
	ctx := context.Background()

	svc, cl := newService(nil, legacyImageCron("qa-cron"))
	v, err := svc.SetCronJob(ctx, "qa-cron", nil, sp("sh -c 'echo qa-a48-replacement-control'"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Command != "sh -c 'echo qa-a48-replacement-control'" {
		t.Errorf("replacement view command = %q", v.Command)
	}
	if got := getApp(t, cl, "qa-cron").Spec.Command; got != "sh -c 'echo qa-a48-replacement-control'" {
		t.Errorf("replacement spec.command = %q", got)
	}

	svc, cl = newService(nil, legacyImageCron("qa-cron"))
	if _, err := svc.SetCronJob(ctx, "qa-cron", sp("0 6 * * *"), nil); err != nil {
		t.Fatal(err)
	}
	if a := getApp(t, cl, "qa-cron"); a.Spec.StartCommand != legacyCronScript || a.Spec.Schedule != "0 6 * * *" {
		t.Errorf("schedule-only save: schedule=%q startCommand=%q, want the fallback kept", a.Spec.Schedule, a.Spec.StartCommand)
	}

	svc, cl = newService(nil, nativeCron("qa-cron"))
	if _, err := svc.SetCronJob(ctx, "qa-cron", nil, sp("")); err != nil {
		t.Fatal(err)
	}
	if a := getApp(t, cl, "qa-cron"); a.Spec.StartCommand != "node job.js" {
		t.Errorf("native clear dropped the baked start command: %q", a.Spec.StartCommand)
	}

	web := sampleApp("qa-web")
	web.Spec.StartCommand = "nginx"
	svc, cl = newService(nil, web)
	if _, err := svc.SetCommands(ctx, "qa-web", nil, sp("")); err != nil {
		t.Fatal(err)
	}
	if a := getApp(t, cl, "qa-web"); a.Spec.StartCommand != "" || a.Spec.Command != "" {
		t.Errorf("web start-command clear: %+v", a.Spec)
	}
}
