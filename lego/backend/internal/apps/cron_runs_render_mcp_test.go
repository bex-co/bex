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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w3/068: Render's MCP server ships run_cron_job and cancel_cron_job_run
// (render-mcp-server #268). An agent written to it calls run_cron_job with
// cancelActiveRun and cancel_cron_job_run with only a serviceId; bex's tools
// of the same names take that contract, on the service verbs REST's
// POST/DELETE /v1/cron-jobs/{id}/runs use.

var cronMCPNow = time.Date(2026, 7, 14, 12, 5, 0, 0, time.UTC)

// cronMCPService is a cron job whose newest run (nightly-run-live) is in
// progress, with a fixed clock and a patch counter.
func cronMCPService(app *appv1alpha1.App) (*Service, *cronPatchCountingClient) {
	svc, _ := newService(nil, app)
	cl := &cronPatchCountingClient{Client: svc.Client}
	svc.Client = cl
	svc.Clock = func() time.Time { return cronMCPNow }
	return svc, cl
}

func TestRenderMCPRunCronJobHonorsCancelActiveRun(t *testing.T) {
	liveID := ids.Derive(ids.CronRun, "nightly-run-live")
	newID := ids.Derive(ids.CronRun, appv1alpha1.ManualCronRunJobName("nightly", cronMCPNow.Format(time.RFC3339Nano)))

	t.Run("refuses while a run is in progress", func(t *testing.T) {
		svc, cl := cronMCPService(cronWithRuns("nightly"))
		res := callMCPTool(t, svc, "run_cron_job", map[string]any{"serviceId": "nightly"})
		text := mcpResultText(res)
		if !res.IsError || !strings.Contains(text, CronErrorRunActive) || !strings.Contains(text, liveID) || !strings.Contains(text, "cancelActiveRun") {
			t.Fatalf("run_cron_job over an active run = %q (isError %v), want %s naming %s and cancelActiveRun", text, res.IsError, CronErrorRunActive, liveID)
		}
		app := getApp(t, cl, "nightly")
		if cl.patches != 0 || app.Spec.RunAt != "" || app.Spec.CancelRun != nil {
			t.Fatalf("refused run wrote intent: %d patches, runAt %q, cancelRun %+v", cl.patches, app.Spec.RunAt, app.Spec.CancelRun)
		}
	})

	t.Run("cancelActiveRun replaces it as REST's POST does", func(t *testing.T) {
		svc, cl := cronMCPService(cronWithRuns("nightly"))
		res := callMCPTool(t, svc, "run_cron_job", map[string]any{"serviceId": "nightly", "cancelActiveRun": true})
		rendertest.AssertRenderMCPResult(t, "run_cron_job", res)
		out := decodeMap(t, []byte(mcpResultText(res)))
		run, _ := out["run"].(map[string]any)
		canceled, _ := out["canceledRun"].(map[string]any)
		if run["id"] != newID || run["status"] != "pending" {
			t.Fatalf("run = %v, want %s pending", run, newID)
		}
		if canceled["id"] != liveID || canceled["startedAt"] != "2026-07-14T12:03:00Z" {
			t.Fatalf("canceledRun = %v, want %s started 2026-07-14T12:03:00Z", canceled, liveID)
		}
		mcpIntent := getApp(t, cl, "nightly").Spec

		rest, restCl := cronMCPService(cronWithRuns("nightly"))
		mux := http.NewServeMux()
		rest.RegisterREST(mux)
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/cron-jobs/nightly/runs", nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), newID) {
			t.Fatalf("REST trigger status=%d body=%s, want the same run %s", rec.Code, rec.Body.String(), newID)
		}
		restIntent := getApp(t, restCl, "nightly").Spec
		if mcpIntent.RunAt != restIntent.RunAt || mcpIntent.CancelRun == nil || restIntent.CancelRun == nil || *mcpIntent.CancelRun != *restIntent.CancelRun {
			t.Fatalf("MCP intent runAt %q cancel %+v, REST runAt %q cancel %+v", mcpIntent.RunAt, mcpIntent.CancelRun, restIntent.RunAt, restIntent.CancelRun)
		}
	})

	t.Run("no run in progress runs without the flag", func(t *testing.T) {
		svc, cl := cronMCPService(cronApp("nightly"))
		res := callMCPTool(t, svc, "run_cron_job", map[string]any{"serviceId": "nightly"})
		rendertest.AssertRenderMCPResult(t, "run_cron_job", res)
		out := decodeMap(t, []byte(mcpResultText(res)))
		if run, _ := out["run"].(map[string]any); run["id"] != newID {
			t.Fatalf("run = %v, want %s", out, newID)
		}
		if _, ok := out["canceledRun"]; ok {
			t.Fatalf("canceledRun = %v with no run in progress", out["canceledRun"])
		}
		if app := getApp(t, cl, "nightly"); app.Spec.CancelRun != nil {
			t.Fatalf("cancel intent %+v with no run in progress", app.Spec.CancelRun)
		}
	})
}

func TestRenderMCPCancelCronJobRunCancelsTheRunInProgress(t *testing.T) {
	liveID := ids.Derive(ids.CronRun, "nightly-run-live")

	t.Run("serviceId alone cancels the active run as REST's DELETE does", func(t *testing.T) {
		svc, cl := cronMCPService(cronWithRuns("nightly"))
		res := callMCPTool(t, svc, "cancel_cron_job_run", map[string]any{"serviceId": "nightly"})
		rendertest.AssertRenderMCPResult(t, "cancel_cron_job_run", res)
		out := decodeMap(t, []byte(mcpResultText(res)))
		run, _ := out["run"].(map[string]any)
		if out["serviceId"] != "nightly" || out["canceled"] != true || run["id"] != liveID || run["status"] != "canceled" {
			t.Fatalf("cancel_cron_job_run = %v, want Render's {serviceId, canceled: true} and run %s canceled", out, liveID)
		}
		mcpIntent := getApp(t, cl, "nightly").Spec.CancelRun

		rest, restCl := cronMCPService(cronWithRuns("nightly"))
		mux := http.NewServeMux()
		rest.RegisterREST(mux)
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v1/cron-jobs/nightly/runs", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("REST cancel status=%d body=%s", rec.Code, rec.Body.String())
		}
		restIntent := getApp(t, restCl, "nightly").Spec.CancelRun
		if mcpIntent == nil || restIntent == nil || mcpIntent.Name != "nightly-run-live" || *mcpIntent != *restIntent {
			t.Fatalf("MCP cancel intent %+v, REST %+v, want both on nightly-run-live", mcpIntent, restIntent)
		}
	})

	t.Run("no run in progress is REST's conflict", func(t *testing.T) {
		svc, cl := cronMCPService(cronApp("nightly"))
		res := callMCPTool(t, svc, "cancel_cron_job_run", map[string]any{"serviceId": "nightly"})
		if text := mcpResultText(res); !res.IsError || !strings.Contains(text, "no active run") {
			t.Fatalf("cancel with no run in progress = %q (isError %v), want the conflict", text, res.IsError)
		}
		if cl.patches != 0 {
			t.Fatalf("cancel with no run in progress used %d patches", cl.patches)
		}
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v1/cron-jobs/nightly/runs", nil))
		if rec.Code != http.StatusConflict {
			t.Fatalf("REST cancel with no run in progress status=%d, want 409", rec.Code)
		}
	})
}
