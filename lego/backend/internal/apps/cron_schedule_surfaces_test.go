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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestCronScheduleContractAcrossSurfaces is w1/m145 t004's parity evidence:
// the two schedules the dashboard drifted on — "0 0 * * 7" (robfig bounds
// day-of-week to 0-6) and "0 0 ? * *" (? is robfig's *) — go through every
// write surface, and each must agree with the shared vector table the
// dashboard form is tested against. A refusal must carry the one sentence the
// dashboard relays to the user (a 400 / bad request, never a 5xx).
func TestCronScheduleContractAcrossSurfaces(t *testing.T) {
	const refusal = "schedule must be a valid 5-field cron expression"
	ctx := context.Background()

	restRefusal := func(rec *httptest.ResponseRecorder) string {
		if rec.Code == http.StatusBadRequest {
			return rec.Body.String()
		}
		return fmt.Sprintf("HTTP %d: %s", rec.Code, rec.Body)
	}
	graphQLOutcome := func(t *testing.T, svc *Service, request string) (bool, string) {
		res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: ctx, RequestString: request})
		return len(res.Errors) == 0, fmt.Sprint(res.Errors)
	}
	verbOutcome := func(err error) (bool, string) {
		if err != nil && !errors.Is(err, core.ErrBadRequest) {
			return false, "not a bad request: " + err.Error()
		}
		return err == nil, fmt.Sprint(err)
	}

	// The MCP tools' handlers are one line each — create_cron_job runs
	// s.Create(in.toCreateRequest()) and update_service runs
	// s.applyServicePatch(in) — so their rows drive exactly that code.
	surfaces := []struct {
		name string
		run  func(t *testing.T, schedule string) (accepted bool, detail string)
	}{
		{"REST POST /v1/services", func(t *testing.T, schedule string) (bool, string) {
			svc, _ := newService(nil)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			body := fmt.Sprintf(`{"name":"nightly","type":"cron_job","schedule":%q,"image":{"imagePath":"job:v1"}}`, schedule)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
			return rec.Code == http.StatusCreated, restRefusal(rec)
		}},
		{"REST PATCH /v1/services/{id}", func(t *testing.T, schedule string) (bool, string) {
			svc, _ := newService(nil, cronApp("nightly"))
			rec := serveRESTPatch(t, svc, "nightly", fmt.Sprintf(`{"schedule":%q}`, schedule))
			return rec.Code == http.StatusOK, restRefusal(rec)
		}},
		{"GraphQL createService", func(t *testing.T, schedule string) (bool, string) {
			svc, _ := newService(nil)
			return graphQLOutcome(t, svc, fmt.Sprintf(`mutation { createService(name:"nightly", type:"cron_job", image:"job:v1", schedule:%q) { id } }`, schedule))
		}},
		{"GraphQL updateCronJob", func(t *testing.T, schedule string) (bool, string) {
			svc, _ := newService(nil, cronApp("nightly"))
			return graphQLOutcome(t, svc, fmt.Sprintf(`mutation { updateCronJob(id:"nightly", schedule:%q) { schedule } }`, schedule))
		}},
		{"MCP create_cron_job", func(t *testing.T, schedule string) (bool, string) {
			svc, _ := newService(nil)
			_, err := svc.Create(ctx, createCronJobArgs{Name: "nightly", Schedule: schedule, Image: "job:v1"}.toCreateRequest())
			return verbOutcome(err)
		}},
		{"MCP update_service", func(t *testing.T, schedule string) (bool, string) {
			svc, _ := newService(nil, cronApp("nightly"))
			_, err := svc.applyServicePatch(ctx, updateServiceArgs{ServiceID: "nightly", Schedule: sp(schedule)})
			return verbOutcome(err)
		}},
		{"Blueprint validate", func(t *testing.T, schedule string) (bool, string) {
			svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
			manifest := fmt.Sprintf("services:\n  - name: nightly\n    type: cron\n    runtime: image\n    image:\n      url: docker.io/library/alpine:3\n    schedule: %q\n", schedule)
			v, err := svc.ValidateBlueprint(ctx, "", manifest, "")
			if err != nil {
				t.Fatalf("ValidateBlueprint: %v", err)
			}
			return v.Valid, fmt.Sprintf("%+v", v.Errors)
		}},
	}

	table := loadCronScheduleVectors(t)
	for _, schedule := range []string{"0 0 * * 7", "0 0 ? * *"} {
		want, ok := table[schedule]
		if !ok {
			t.Fatalf("%q is missing from %s", schedule, cronScheduleVectorsPath)
		}
		for _, surface := range surfaces {
			t.Run(schedule+"/"+surface.name, func(t *testing.T) {
				accepted, detail := surface.run(t, schedule)
				if accepted != want {
					t.Fatalf("accepted = %v, table says %v (%s)", accepted, want, detail)
				}
				if !accepted && !strings.Contains(detail, refusal) {
					t.Errorf("refusal must say %q, got %s", refusal, detail)
				}
			})
		}
	}
}
